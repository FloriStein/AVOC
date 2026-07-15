// WebSocket client for the Fleet Dashboard live-update channel (FLEET-06, GET /fleet/ws).
// JSON, not the binary Protobuf control channel in ws-client.ts — a separate, unrelated
// connection used only to keep the Fleet Overview dashboard live.

import { parseFleetWSMessage, type FleetWSEvent } from '@/lib/fleet-ws-events'
import { logEvent, FE_WS_RECONNECT, FE_WS_CONNECTED } from '@/lib/logger'

// Same backoff schedule as useSession.ts's teleop WS reconnect (1s, 2s, 4s, 8s, 16s, max 30s).
const BACKOFF = [1000, 2000, 4000, 8000, 16000, 30000]

export type FleetWSOpenHandler = () => void
export type FleetWSCloseHandler = () => void
export type FleetWSEventHandler = (event: FleetWSEvent) => void

export class FleetWSClient {
  private ws: WebSocket | null = null
  private reconnectAttempt = 0
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private token: string | null = null
  private intentionalClose = false

  onOpen: FleetWSOpenHandler | null = null
  onClose: FleetWSCloseHandler | null = null
  onEvent: FleetWSEventHandler | null = null

  // Unlike ws-client.ts (where useSession.ts owns reconnect externally because sessionId
  // changes over the WS's lifetime), reconnect lives inside this client — its only connection
  // parameter (the token) is stable for the life of a login, so self-contained reconnect keeps
  // the calling hook simpler.
  connect(token: string): void {
    this.token = token
    this.intentionalClose = false
    this.openSocket()
  }

  private openSocket(): void {
    const t = this.token
    if (!t) return
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const url = `${proto}://${window.location.host}/fleet/ws?token=${t}`
    this.ws = new WebSocket(url)

    this.ws.onopen = () => {
      this.reconnectAttempt = 0
      logEvent(FE_WS_CONNECTED, 'Fleet WebSocket connected', { data: { channel: 'fleet' } })
      this.onOpen?.()
    }

    this.ws.onmessage = (e) => {
      if (typeof e.data !== 'string') return
      this.onEvent?.(parseFleetWSMessage(e.data))
    }

    this.ws.onclose = () => {
      this.ws = null
      this.onClose?.()
      if (this.intentionalClose) return

      const attempt = this.reconnectAttempt
      const delay = BACKOFF[Math.min(attempt, BACKOFF.length - 1)]
      this.reconnectAttempt++
      logEvent(FE_WS_RECONNECT, `Fleet WebSocket closed — reconnect in ${delay}ms`, {
        data: { channel: 'fleet', attempt, delayMs: delay },
      })
      this.reconnectTimer = setTimeout(() => this.openSocket(), delay)
    }
  }

  disconnect(): void {
    this.intentionalClose = true
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    if (this.ws) {
      this.ws.onclose = null
      this.ws.close()
      this.ws = null
    }
  }

  isOpen(): boolean {
    return this.ws?.readyState === WebSocket.OPEN
  }
}
