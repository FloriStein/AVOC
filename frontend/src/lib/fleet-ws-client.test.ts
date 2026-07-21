import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import { FleetWSClient } from './fleet-ws-client'
import { MockWebSocket } from '@/test/mock-websocket'

// FleetWSClient (Fleet Dashboard live-update channel, FLEET-06) had 2.2% coverage — no test file
// existed before Sprint 53. Unlike ws-client.ts (reconnect owned externally by useSession.ts),
// this client owns its own reconnect scheduling — that self-contained reconnect logic plus the
// same Sprint-14 onclose-nulling race-condition fix are the focus here.

describe('FleetWSClient', () => {
  beforeEach(() => {
    MockWebSocket.reset()
    vi.stubGlobal('WebSocket', MockWebSocket)
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  function currentSocket(): MockWebSocket {
    return MockWebSocket.instances[MockWebSocket.instances.length - 1]
  }

  it('öffnet eine WebSocket-Verbindung mit token in der URL', () => {
    const client = new FleetWSClient()
    client.connect('tok-1')

    expect(MockWebSocket.instances).toHaveLength(1)
    expect(currentSocket().url).toContain('token=tok-1')
  })

  it('ruft onOpen auf und setzt reconnectAttempt zurück, sobald die Verbindung öffnet', () => {
    const client = new FleetWSClient()
    const onOpen = vi.fn()
    client.onOpen = onOpen
    client.connect('tok-1')

    currentSocket().triggerOpen()

    expect(onOpen).toHaveBeenCalledTimes(1)
  })

  describe('Events', () => {
    it('parst ein JSON-Text-Frame und ruft onEvent mit dem typisierten Event auf', () => {
      const client = new FleetWSClient()
      const onEvent = vi.fn()
      client.onEvent = onEvent
      client.connect('tok-1')
      currentSocket().triggerOpen()

      const payload = JSON.stringify({
        type: 'vehicle_status',
        data: { vehicle_id: 'v1', autonomy_mode: 'teleoperated', updated_at: 't1' },
      })
      currentSocket().triggerMessage(payload)

      expect(onEvent).toHaveBeenCalledWith({
        type: 'vehicle_status',
        data: { vehicle_id: 'v1', autonomy_mode: 'teleoperated', updated_at: 't1' },
      })
    })

    it('ignoriert Nachrichten, deren data kein String ist (z. B. Binärframe)', () => {
      const client = new FleetWSClient()
      const onEvent = vi.fn()
      client.onEvent = onEvent
      client.connect('tok-1')
      currentSocket().triggerOpen()

      currentSocket().triggerMessage(new ArrayBuffer(0))

      expect(onEvent).not.toHaveBeenCalled()
    })
  })

  describe('Reconnect mit exponential Backoff (unerwarteter Abbruch)', () => {
    it('verbindet nach dem ersten Abbruch nach 1000ms neu', () => {
      const client = new FleetWSClient()
      const onClose = vi.fn()
      client.onClose = onClose
      client.connect('tok-1')
      currentSocket().triggerOpen()

      currentSocket().triggerClose()
      expect(onClose).toHaveBeenCalledTimes(1)
      expect(MockWebSocket.instances).toHaveLength(1) // noch kein Reconnect-Versuch

      vi.advanceTimersByTime(1000)
      expect(MockWebSocket.instances).toHaveLength(2)
      expect(currentSocket().url).toContain('token=tok-1')
    })

    it('erhöht das Backoff-Delay bei aufeinanderfolgenden Abbrüchen (1s dann 2s)', () => {
      // Both closes below happen without an intervening triggerOpen() — a successful open
      // resets reconnectAttempt (see the next test), so growing backoff only shows up across
      // consecutive FAILURES, e.g. the socket never reaching OPEN before closing again.
      const client = new FleetWSClient()
      client.connect('tok-1')
      currentSocket().triggerClose() // 1st failure: attempt 0 → 1000ms delay, attempt becomes 1

      vi.advanceTimersByTime(1000)
      expect(MockWebSocket.instances).toHaveLength(2)

      currentSocket().triggerClose() // 2nd consecutive failure: attempt 1 → 2000ms delay
      vi.advanceTimersByTime(1999)
      expect(MockWebSocket.instances).toHaveLength(2) // noch nicht — 2s-Delay noch nicht erreicht

      vi.advanceTimersByTime(1)
      expect(MockWebSocket.instances).toHaveLength(3)
    })

    it('setzt reconnectAttempt nach einem erfolgreichen Reconnect zurück', () => {
      const client = new FleetWSClient()
      client.connect('tok-1')
      currentSocket().triggerOpen()
      currentSocket().triggerClose()
      vi.advanceTimersByTime(1000)
      currentSocket().triggerOpen() // erfolgreicher Reconnect

      currentSocket().triggerClose()
      vi.advanceTimersByTime(1000) // wieder 1s, nicht 2s, da reconnectAttempt zurückgesetzt wurde

      expect(MockWebSocket.instances).toHaveLength(3)
    })
  })

  describe('disconnect() — kein Reconnect nach intentionalem Close', () => {
    it('setzt ws.onclose auf null, BEVOR close() aufgerufen wird', () => {
      const client = new FleetWSClient()
      client.connect('tok-1')
      const socket = currentSocket()
      socket.triggerOpen()

      client.disconnect()

      expect(socket.closeCalled).toBe(true)
      expect(socket.closeCalledWithNulledHandler).toBe(true)
    })

    it('plant KEINEN Reconnect, selbst wenn der Browser danach noch ein close-Event feuert', () => {
      const client = new FleetWSClient()
      const onClose = vi.fn()
      client.onClose = onClose
      client.connect('tok-1')
      const socket = currentSocket()
      socket.triggerOpen()

      client.disconnect()
      socket.triggerClose() // late native close event

      expect(onClose).not.toHaveBeenCalled()
      vi.advanceTimersByTime(30_000)
      expect(MockWebSocket.instances).toHaveLength(1) // kein neuer Verbindungsversuch
    })

    it('räumt einen bereits laufenden Reconnect-Timer auf', () => {
      const client = new FleetWSClient()
      client.connect('tok-1')
      currentSocket().triggerOpen()
      currentSocket().triggerClose() // schedules a reconnect in 1000ms

      client.disconnect()
      vi.advanceTimersByTime(30_000)

      expect(MockWebSocket.instances).toHaveLength(1) // der geplante Reconnect wurde nicht ausgeführt
    })
  })

  describe('isOpen', () => {
    it('spiegelt den readyState der zugrunde liegenden Verbindung', () => {
      const client = new FleetWSClient()
      expect(client.isOpen()).toBe(false)

      client.connect('tok-1')
      expect(client.isOpen()).toBe(false)

      currentSocket().triggerOpen()
      expect(client.isOpen()).toBe(true)

      client.disconnect()
      expect(client.isOpen()).toBe(false)
    })
  })
})
