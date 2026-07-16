import { useCallback, useEffect, useRef, useState } from 'react'
import { reportMediaState } from '@/lib/api-client'
import { FE_WEBRTC_STATE, logEvent } from '@/lib/logger'

export type MediaState =
  | 'MEDIA_INIT'
  | 'MEDIA_NEGOTIATING'
  | 'MEDIA_CONNECTED'
  | 'MEDIA_DEGRADED'
  | 'MEDIA_FAILED'

// MEDIA_DEGRADED thresholds (DRIFT-K3, 2026-07-16) — initial values, not yet
// field-validated against a real fleet. Grill-Me decision: ship reasonable
// defaults now rather than block on data that doesn't exist yet (no live
// fleet to calibrate against); revisit once real network conditions from a
// pilot/demo are observed.
const MEDIA_DEGRADED_LOSS_RATIO = 0.05 // >5% packet loss
const MEDIA_DEGRADED_MIN_BITRATE_BPS = 100_000 // <100kbps
const MEDIA_DEGRADED_SUSTAIN_SAMPLES = 3 // consecutive 1s getStats() samples (~3s)

// Pure decision logic, exported for direct unit testing (no RTCPeerConnection
// mock needed) — the getStats() plumbing around these is thin glue code.

/** Packet-loss ratio from raw inbound-rtp counters. 0 when no packets seen yet. */
export function computeLossRatio(packetsLost: number, packetsReceived: number): number {
  const total = packetsLost + packetsReceived
  return total > 0 ? packetsLost / total : 0
}

/** Receive bitrate in bits/sec from two bytesReceived samples. null if the interval is non-positive. */
export function computeBitrateBps(bytesDelta: number, deltaSeconds: number): number | null {
  return deltaSeconds > 0 ? (bytesDelta * 8) / deltaSeconds : null
}

/** Whether a single sample counts as degraded (either threshold breached). */
export function isDegradedSample(lossRatio: number, bitrateBps: number | null): boolean {
  return lossRatio > MEDIA_DEGRADED_LOSS_RATIO ||
    (bitrateBps !== null && bitrateBps < MEDIA_DEGRADED_MIN_BITRATE_BPS)
}

export interface StreakState {
  degradedStreak: number
  healthyStreak: number
}

/** Updates the consecutive-sample streak counters for one new sample. */
export function nextStreak(streak: StreakState, degraded: boolean): StreakState {
  return degraded
    ? { degradedStreak: streak.degradedStreak + 1, healthyStreak: 0 }
    : { degradedStreak: 0, healthyStreak: streak.healthyStreak + 1 }
}

/**
 * Decides the next MediaState given the current state and updated streaks.
 * Hysteresis (MEDIA_DEGRADED_SUSTAIN_SAMPLES) prevents a single noisy sample
 * from flapping the UI in either direction. Returns null when no transition
 * is warranted (caller should keep the current state).
 */
export function nextMediaStateFromStreak(current: MediaState, streak: StreakState): MediaState | null {
  if (current === 'MEDIA_CONNECTED' && streak.degradedStreak >= MEDIA_DEGRADED_SUSTAIN_SAMPLES) {
    return 'MEDIA_DEGRADED'
  }
  if (current === 'MEDIA_DEGRADED' && streak.healthyStreak >= MEDIA_DEGRADED_SUSTAIN_SAMPLES) {
    return 'MEDIA_CONNECTED'
  }
  return null
}

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
  const streakRef = useRef<StreakState>({ degradedStreak: 0, healthyStreak: 0 })
  const lastInboundRef = useRef<{ bytes: number; time: number } | null>(null)

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

      // Pion v1.19.0 DTLS-Client-Bug: nach dem Senden von ClientHello verarbeitet Pion
      // den ServerHello des Browsers nicht → Retransmit-Loop bis Timeout.
      // Fix: Browser wird DTLS-Client (active), MediaMTX wird DTLS-Server (passive).
      // Pions DTLS-Server-Pfad ist stabil; nur der Client-Pfad hat diesen Bug.
      const fixedSdp = offer.sdp!.replace(/a=setup:actpass/g, 'a=setup:active')
      await pc.setLocalDescription({ type: 'offer', sdp: fixedSdp })
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

  // Poll WebRTC stats every second while video is connected OR degraded (the
  // latter so recovery can be detected). currentRoundTripTime (seconds) from
  // the nominated candidate-pair gives the network-level E2E latency for the
  // video channel. Packet-loss ratio and receive bitrate from the inbound-rtp
  // video report drive MEDIA_DEGRADED detection (DRIFT-K3, 2026-07-16) —
  // sustained MEDIA_DEGRADED_SUSTAIN_SAMPLES consecutive bad/good samples
  // before switching, so one noisy sample doesn't flap the UI.
  useEffect(() => {
    if (mediaState !== 'MEDIA_CONNECTED' && mediaState !== 'MEDIA_DEGRADED') {
      setVideoLatencyMs(null)
      streakRef.current = { degradedStreak: 0, healthyStreak: 0 }
      lastInboundRef.current = null
      return
    }
    const pc = pcRef.current
    if (!pc) return
    const id = setInterval(async () => {
      try {
        const stats = await pc.getStats()
        let packetsLost = 0
        let packetsReceived = 0
        let bytesReceived: number | undefined
        stats.forEach(r => {
          // currentRoundTripTime is 0 until the first STUN consent check (~5s after ICE).
          // Ignore 0 so the UI shows "— ms" instead of a misleading "0 ms".
          if (r.type === 'candidate-pair' && r.nominated &&
              typeof r.currentRoundTripTime === 'number' && r.currentRoundTripTime > 0) {
            setVideoLatencyMs(Math.round(r.currentRoundTripTime * 1000))
          }
          if (r.type === 'inbound-rtp' && r.kind === 'video') {
            packetsLost = r.packetsLost ?? 0
            packetsReceived = r.packetsReceived ?? 0
            bytesReceived = r.bytesReceived
          }
        })

        const lossRatio = computeLossRatio(packetsLost, packetsReceived)

        let bitrateBps: number | null = null
        const now = performance.now()
        if (typeof bytesReceived === 'number' && lastInboundRef.current) {
          const deltaSeconds = (now - lastInboundRef.current.time) / 1000
          bitrateBps = computeBitrateBps(bytesReceived - lastInboundRef.current.bytes, deltaSeconds)
        }
        if (typeof bytesReceived === 'number') {
          lastInboundRef.current = { bytes: bytesReceived, time: now }
        }

        streakRef.current = nextStreak(streakRef.current, isDegradedSample(lossRatio, bitrateBps))

        const next = nextMediaStateFromStreak(mediaState, streakRef.current)
        if (next !== null) updateState(next)
      } catch { /* PC closed */ }
    }, 1000)
    return () => clearInterval(id)
  }, [mediaState, updateState])

  return { videoRef, streamRef, mediaState, videoLatencyMs, connect, disconnect }
}
