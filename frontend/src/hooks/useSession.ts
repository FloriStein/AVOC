import { useCallback, useEffect, useRef, useState } from 'react'
import { login, logout as logoutAPI, startSession, endSession as endSessionAPI } from '@/lib/api-client'
import { WSClient } from '@/lib/ws-client'
import { logEvent, FE_WS_RECONNECT, FE_WS_CONNECTED } from '@/lib/logger'

// Exponential backoff delays in ms (1s, 2s, 4s, 8s, max 30s).
const BACKOFF = [1000, 2000, 4000, 8000, 16000, 30000]

export interface SessionState {
  token: string | null
  operatorId: string | null
  sessionId: string | null
  vehicleId: string | null
  role: string | null            // 'ACTIVE_OPERATOR' | 'OBSERVER' | null
  latency: number
  wsClient: WSClient
  connect: (id: string, password: string) => Promise<void>
  resume: () => Promise<void>
  disconnect: () => Promise<void>
  startSession: (vehicleId: string) => Promise<void>
  endSession: () => Promise<void>
  /** Restores session refs after page reload so resume() can recover from SAFE_MODE. */
  restoreFromServerState: (sessionId: string, vehicleId: string, role: string) => void
}

const LS_TOKEN = 'avoc-token'
const LS_OPERATOR = 'avoc-operator'

export function useSession(): SessionState {
  const storedToken = localStorage.getItem(LS_TOKEN)
  const [token, setToken] = useState<string | null>(storedToken)
  const [operatorId, setOperatorId] = useState<string | null>(localStorage.getItem(LS_OPERATOR))
  const [sessionId, setSessionId] = useState<string | null>(null)
  const [vehicleId, setVehicleId] = useState<string | null>(null)
  const [role, setRole] = useState<string | null>(null)
  const [latency, setLatency] = useState(0)

  const wsClient = useRef(new WSClient()).current
  const reconnectAttempt = useRef(0)
  const tokenRef = useRef<string | null>(storedToken)
  const sessionIdRef = useRef<string | null>(null)

  // Connects WS with session_id (ADR-025: WS connects AFTER session/start).
  // Sets up exponential backoff reconnect using the same session_id.
  const connectWS = useCallback((t: string, sid: string) => {
    wsClient.onAck = (ms) => setLatency(ms)
    wsClient.onOpen = () => {
      reconnectAttempt.current = 0
      logEvent(FE_WS_CONNECTED, 'WebSocket connected', { sessionId: sid })
    }
    wsClient.onClose = async () => {
      // Backoff reconnect after unexpected server-side close.
      // Intentional closes via wsClient.disconnect() suppress this via ws.onclose = null.
      const attempt = reconnectAttempt.current
      const delay = BACKOFF[Math.min(attempt, BACKOFF.length - 1)]
      reconnectAttempt.current++
      logEvent(FE_WS_RECONNECT, `WebSocket closed — reconnect in ${delay}ms`, {
        sessionId: sessionIdRef.current ?? sid,
        data: { attempt, delayMs: delay },
      })
      await new Promise((r) => setTimeout(r, delay))
      const currentToken = tokenRef.current
      const currentSid = sessionIdRef.current
      if (currentToken && currentSid) {
        wsClient.connect(currentToken, currentSid)
      }
    }
    wsClient.connect(t, sid)
  }, [wsClient])

  // Cross-tab sync: when another tab logs out, clear state here too.
  useEffect(() => {
    const onStorage = (e: StorageEvent) => {
      if (e.key === LS_TOKEN && !e.newValue) {
        wsClient.disconnect()
        tokenRef.current = null
        sessionIdRef.current = null
        setToken(null)
        setOperatorId(null)
        setSessionId(null)
        setVehicleId(null)
        setRole(null)
        setLatency(0)
      }
    }
    window.addEventListener('storage', onStorage)
    return () => window.removeEventListener('storage', onStorage)
  }, [wsClient])

  // Login only — WS connects after session/start (not here).
  const connect = useCallback(async (id: string, password: string) => {
    reconnectAttempt.current = 0
    const t = await login(id, password)
    localStorage.setItem(LS_TOKEN, t)
    localStorage.setItem(LS_OPERATOR, id)
    tokenRef.current = t
    setToken(t)
    setOperatorId(id)
    setSessionId(null)
    setVehicleId(null)
    setRole(null)
    sessionIdRef.current = null
  }, [])

  // Called by VehicleSelector when the operator picks a vehicle.
  // Calls POST /session/start, stores results, then connects WS with session_id.
  const startSessionFn = useCallback(async (vid: string) => {
    const t = tokenRef.current
    const opId = operatorId
    if (!t || !opId) return
    try {
      const result = await startSession(vid, opId, t)
      sessionIdRef.current = result.session_id
      setSessionId(result.session_id)
      setVehicleId(result.vehicle_id)
      setRole(result.role)
      reconnectAttempt.current = 0
      connectWS(t, result.session_id)
    } catch {
      // caller can retry
    }
  }, [operatorId, connectWS])

  // Called when operator deliberately ends a session to pick a different vehicle.
  const endSessionFn = useCallback(async () => {
    const t = tokenRef.current
    const sid = sessionIdRef.current
    wsClient.disconnect()
    if (t && sid) {
      await endSessionAPI(sid, t).catch(() => {})
    }
    sessionIdRef.current = null
    setSessionId(null)
    setVehicleId(null)
    setRole(null)
    setLatency(0)
  }, [wsClient])

  // Resume after SAFE_MODE: silently close old WS and reconnect with the same session_id.
  // The WS handler detects SAFE_MODE and transitions back to CONNECTED (ADR-025).
  const resume = useCallback(async () => {
    const t = tokenRef.current
    const sid = sessionIdRef.current
    if (!t || !sid) return
    reconnectAttempt.current = 0  // reset backoff — manual Resume should try immediately
    wsClient.disconnect()
    connectWS(t, sid)
  }, [wsClient, connectWS])

  // Restores session context after page reload so the SAFE_MODE resume path works.
  // Called from App when state polling detects SAFE_MODE + session_id but frontend
  // lost its context (token exists but sessionId is null).
  const restoreFromServerState = useCallback((sid: string, vid: string, r: string) => {
    sessionIdRef.current = sid
    setSessionId(sid)
    setVehicleId(vid)
    setRole(r)
  }, [])

  const disconnect = useCallback(async () => {
    const t = tokenRef.current
    if (t) {
      try {
        await logoutAPI(t)
      } catch (e) {
        // Server rejected logout (409 = active session) — abort silently.
        // The button is already disabled in this case; this is the backend safety net.
        if (e instanceof Error && e.message === 'active_session') return
      }
    }
    localStorage.removeItem(LS_TOKEN)
    localStorage.removeItem(LS_OPERATOR)
    wsClient.disconnect()
    tokenRef.current = null
    sessionIdRef.current = null
    setToken(null)
    setOperatorId(null)
    setSessionId(null)
    setVehicleId(null)
    setRole(null)
    setLatency(0)
  }, [wsClient])

  return {
    token, operatorId, sessionId, vehicleId, role, latency,
    wsClient, connect, resume, disconnect,
    startSession: startSessionFn, endSession: endSessionFn,
    restoreFromServerState,
  }
}
