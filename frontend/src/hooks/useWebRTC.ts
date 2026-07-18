import { useCallback, useEffect, useRef, useState } from 'react'
import { reportMediaState } from '@/lib/api-client'
import { FE_WEBRTC_STATE, logEvent } from '@/lib/logger'

export type MediaState =
  | 'MEDIA_INIT'
  | 'MEDIA_NEGOTIATING'
  | 'MEDIA_CONNECTED'
  | 'MEDIA_DEGRADED'
  | 'MEDIA_FAILED'

function isValidIceServer(s: RTCIceServer): boolean {
  const urls = Array.isArray(s.urls) ? s.urls : [s.urls]
  // Reject entries where the URL contains an empty host (e.g. "stun::3478" when TURN_EXTERNAL_IP is unset)
  return urls.every(u => !/^(stun|turn):(?::|\?)/.test(u))
}

async function fetchIceServers(): Promise<RTCIceServer[]> {
  try {
    const res = await fetch('/api/ice-config')
    if (!res.ok) throw new Error(`ice-config ${res.status}`)
    const { iceServers } = await res.json()
    const valid = (iceServers as RTCIceServer[]).filter(isValidIceServer)
    if (valid.length > 0) return valid
    throw new Error('no valid ICE servers in response')
  } catch {
    // Fallback: STUN only — works on non-CGNAT networks (WiFi/DSL)
    const host = window.location.hostname || '127.0.0.1'
    return [{ urls: `stun:${host}:3478` }]
  }
}

export function useWebRTC(sessionId: string | null, vehicleId: string, token: string | null, enabled: boolean) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const streamRef = useRef<MediaStream | null>(null)
  const [mediaState, setMediaState] = useState<MediaState>('MEDIA_INIT')
  const [videoLatencyMs, setVideoLatencyMs] = useState<number | null>(null)
  const pcRef = useRef<RTCPeerConnection | null>(null)

  const updateState = useCallback((state: MediaState) => {
    setMediaState(state)
    if (token) reportMediaState(state, vehicleId, token).catch(() => {})
    logEvent(FE_WEBRTC_STATE, 'WebRTC media state changed',
      { sessionId: sessionId ?? '', data: { state } })
  }, [sessionId, vehicleId, token])

  const disconnect = useCallback(() => {
    if (pcRef.current) {
      pcRef.current.close()
      pcRef.current = null
    }
    if (videoRef.current) videoRef.current.srcObject = null
    streamRef.current = null
    setMediaState('MEDIA_INIT')
  }, [])

  const connect = useCallback(async () => {
    if (!sessionId || !vehicleId || !token) return
    disconnect()
    updateState('MEDIA_NEGOTIATING')

    const iceServers = await fetchIceServers()
    const pc = new RTCPeerConnection({ iceServers })
    pcRef.current = pc

    pc.addTransceiver('video', { direction: 'recvonly' })

    pc.ontrack = (event) => {
      if (videoRef.current && event.streams.length > 0) {
        videoRef.current.srcObject = event.streams[0]
        streamRef.current = event.streams[0]
        updateState('MEDIA_CONNECTED')
      }
    }

    pc.oniceconnectionstatechange = () => {
      if (pc !== pcRef.current) return
      if (pc.iceConnectionState === 'failed' || pc.iceConnectionState === 'disconnected') {
        updateState('MEDIA_FAILED')
      }
    }

    try {
      const offer = await pc.createOffer()
      // Guard: verhindert Race Condition durch React StrictMode double-mount.
      // Wenn eine neuere connect()-Instanz unsere PC ersetzt hat, still abbrechen.
      if (pc !== pcRef.current) return

      // WEBRTC-10 (ADR-014/020, Sprint 32): der frühere actpass→active-SDP-Zwang (Fix für einen
      // Pion-v1.19.0-DTLS-Client-Bug, Sprint 10) wurde entfernt. mediamtx:latest baut inzwischen
      // gegen pion/webrtc v4.2.x (komplett andere Codebasis als das uralte v1.19.0) — der
      // ursprüngliche Bug ist dort nicht mehr reproduzierbar, während der Zwang selbst mit
      // aktuellem Chromium bricht (`setRemoteDescription`: "Offerer must use actpass"), da der
      // Offerer laut RFC 8842 immer actpass senden muss. Standard-`createOffer()`-SDP unverändert
      // gegen den lokalen Docker-Test-Stack verifiziert (WHIP-Ingest + WHEP-Playback erfolgreich).
      await pc.setLocalDescription(offer)
      if (pc !== pcRef.current) return

      // WHEP: alle ICE-Candidates vollständig abwarten bevor der Offer gesendet wird.
      // MediaMTX erwartet alle Candidates im initialen POST (kein Trickle-ICE).
      await new Promise<void>(resolve => {
        if (pc.iceGatheringState === 'complete') { resolve(); return }
        const tid = setTimeout(resolve, 2000)
        pc.addEventListener('icegatheringstatechange', function handler() {
          if (pc.iceGatheringState === 'complete') {
            clearTimeout(tid)
            pc.removeEventListener('icegatheringstatechange', handler)
            resolve()
          }
        })
      })
      if (pc !== pcRef.current) return

      const res = await fetch(`/whep/${vehicleId}/whep`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/sdp',
          'Authorization': `Bearer ${token}`,
        },
        body: pc.localDescription!.sdp,
      })
      if (pc !== pcRef.current) return

      if (!res.ok) { updateState('MEDIA_FAILED'); return }

      const answerSdp = await res.text()
      if (pc !== pcRef.current) return
      await pc.setRemoteDescription({ type: 'answer', sdp: answerSdp })
    } catch {
      if (pc === pcRef.current) updateState('MEDIA_FAILED')
    }
  }, [sessionId, vehicleId, token, disconnect, updateState])

  useEffect(() => {
    if (enabled && sessionId && token) {
      connect()
    } else {
      disconnect()
    }
    return disconnect
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, sessionId, vehicleId, token])

  // Auto-retry when stream not yet available (MEDIA_FAILED + enabled)
  useEffect(() => {
    if (mediaState !== 'MEDIA_FAILED' || !enabled || !sessionId || !token) return
    const tid = setTimeout(connect, 3000)
    return () => clearTimeout(tid)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mediaState, enabled, sessionId, token])

  // Poll WebRTC ICE candidate-pair RTT every second while video is connected.
  // currentRoundTripTime (seconds) from the nominated candidate-pair gives
  // the network-level E2E latency for the video channel.
  useEffect(() => {
    if (mediaState !== 'MEDIA_CONNECTED') { setVideoLatencyMs(null); return }
    const pc = pcRef.current
    if (!pc) return
    const id = setInterval(async () => {
      try {
        const stats = await pc.getStats()
        stats.forEach(r => {
          // currentRoundTripTime is 0 until the first STUN consent check (~5s after ICE).
          // Ignore 0 so the UI shows "— ms" instead of a misleading "0 ms".
          if (r.type === 'candidate-pair' && r.nominated &&
              typeof r.currentRoundTripTime === 'number' && r.currentRoundTripTime > 0) {
            setVideoLatencyMs(Math.round(r.currentRoundTripTime * 1000))
          }
        })
      } catch { /* PC closed */ }
    }, 1000)
    return () => clearInterval(id)
  }, [mediaState])

  return { videoRef, streamRef, mediaState, videoLatencyMs, connect, disconnect }
}
