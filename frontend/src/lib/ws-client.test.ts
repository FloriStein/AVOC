import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import { create, toBinary } from '@bufbuild/protobuf'
import { ControlAckSchema } from '@/gen/control_pb.js'
import { WSClient } from './ws-client'
import { MockWebSocket } from '@/test/mock-websocket'

// WSClient carries the control channel's send/ACK/reconnect plumbing (ADR-010/012b). No prior
// test file existed for it (10.5% coverage — Sprint-53 vorrecherche). The Sprint-14 regression
// this file guards against: an intentional disconnect() must never fire the external onClose
// handler (which would otherwise trigger useSession.ts's reconnect-with-backoff for a close the
// operator asked for), by nulling ws.onclose BEFORE calling ws.close().

describe('WSClient', () => {
  beforeEach(() => {
    MockWebSocket.reset()
    vi.stubGlobal('WebSocket', MockWebSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  function currentSocket(): MockWebSocket {
    return MockWebSocket.instances[MockWebSocket.instances.length - 1]
  }

  it('öffnet eine WebSocket-Verbindung mit token und session_id in der URL', () => {
    const client = new WSClient()
    client.connect('tok-1', 'sess-1')

    expect(MockWebSocket.instances).toHaveLength(1)
    expect(currentSocket().url).toContain('token=tok-1')
    expect(currentSocket().url).toContain('session_id=sess-1')
    expect(currentSocket().binaryType).toBe('arraybuffer')
  })

  it('ruft onOpen auf, sobald die Verbindung öffnet', () => {
    const client = new WSClient()
    const onOpen = vi.fn()
    client.onOpen = onOpen
    client.connect('tok-1', 'sess-1')

    currentSocket().triggerOpen()

    expect(onOpen).toHaveBeenCalledTimes(1)
  })

  describe('send/ACK-Latenz', () => {
    it('sendet Bytes nur, wenn die Verbindung OPEN ist', () => {
      const client = new WSClient()
      client.connect('tok-1', 'sess-1')
      const socket = currentSocket()

      client.send(new Uint8Array([1, 2, 3]))
      expect(socket.sent).toHaveLength(0) // noch CONNECTING

      socket.triggerOpen()
      client.send(new Uint8Array([1, 2, 3]))
      expect(socket.sent).toHaveLength(1)
    })

    it('berechnet die Latenz aus send()-Zeitstempel bis zur nächsten Nachricht und ruft onAck auf', () => {
      vi.useFakeTimers()
      const client = new WSClient()
      const onAck = vi.fn()
      client.onAck = onAck
      client.connect('tok-1', 'sess-1')
      const socket = currentSocket()
      socket.triggerOpen()

      client.send(new Uint8Array([1]))
      vi.advanceTimersByTime(37)
      socket.triggerMessage(new ArrayBuffer(0))

      expect(onAck).toHaveBeenCalledWith(37)
      vi.useRealTimers()
    })

    it('ruft onAck NICHT auf für eine Nachricht ohne vorheriges send() (kein pending ACK)', () => {
      const client = new WSClient()
      const onAck = vi.fn()
      client.onAck = onAck
      client.connect('tok-1', 'sess-1')
      const socket = currentSocket()
      socket.triggerOpen()

      socket.triggerMessage(new ArrayBuffer(0))

      expect(onAck).not.toHaveBeenCalled()
    })

    it('surfacet einen Server-Fehler aus einem non-success ControlAck über onAckError', async () => {
      const client = new WSClient()
      const onAckError = vi.fn()
      client.onAckError = onAckError
      client.connect('tok-1', 'sess-1')
      const socket = currentSocket()
      socket.triggerOpen()
      client.send(new Uint8Array([1]))

      // ControlAckSchema loads via a fire-and-forget dynamic import inside ws-client.ts —
      // give it a tick to resolve before relying on real Protobuf decoding.
      await new Promise((r) => setTimeout(r, 0))

      const ack = create(ControlAckSchema, { success: false, errorMsg: 'vehicle rejected command' })
      socket.triggerMessage(toBinary(ControlAckSchema, ack).buffer)

      expect(onAckError).toHaveBeenCalledWith('vehicle rejected command')
    })

    it('ruft onAckError NICHT auf, wenn success=true', async () => {
      const client = new WSClient()
      const onAckError = vi.fn()
      client.onAckError = onAckError
      client.connect('tok-1', 'sess-1')
      const socket = currentSocket()
      socket.triggerOpen()
      client.send(new Uint8Array([1]))
      await new Promise((r) => setTimeout(r, 0))

      const ack = create(ControlAckSchema, { success: true, errorMsg: '' })
      socket.triggerMessage(toBinary(ControlAckSchema, ack).buffer)

      expect(onAckError).not.toHaveBeenCalled()
    })
  })

  describe('disconnect() — Sprint-14-Regression (onclose=null VOR close())', () => {
    it('setzt ws.onclose auf null, BEVOR close() aufgerufen wird', () => {
      const client = new WSClient()
      client.onClose = vi.fn()
      client.connect('tok-1', 'sess-1')
      const socket = currentSocket()
      socket.triggerOpen()

      client.disconnect()

      expect(socket.closeCalled).toBe(true)
      expect(socket.closeCalledWithNulledHandler).toBe(true)
    })

    it('ruft den externen onClose-Handler NICHT auf, selbst wenn der Browser das close-Event danach noch feuert', () => {
      const client = new WSClient()
      const onClose = vi.fn()
      client.onClose = onClose
      client.connect('tok-1', 'sess-1')
      const socket = currentSocket()
      socket.triggerOpen()

      client.disconnect()
      socket.triggerClose() // simulates a late native close event after the intentional disconnect

      expect(onClose).not.toHaveBeenCalled()
    })

    it('ruft onClose auf für einen ECHTEN, unerwarteten Verbindungsabbruch (kein disconnect() zuvor)', () => {
      const client = new WSClient()
      const onClose = vi.fn()
      client.onClose = onClose
      client.connect('tok-1', 'sess-1')
      const socket = currentSocket()
      socket.triggerOpen()

      socket.triggerClose() // server/network drop, not requested by the operator

      expect(onClose).toHaveBeenCalledTimes(1)
    })
  })

  describe('isOpen', () => {
    it('spiegelt den readyState der zugrunde liegenden Verbindung', () => {
      const client = new WSClient()
      expect(client.isOpen()).toBe(false)

      client.connect('tok-1', 'sess-1')
      expect(client.isOpen()).toBe(false) // noch CONNECTING

      currentSocket().triggerOpen()
      expect(client.isOpen()).toBe(true)

      client.disconnect()
      expect(client.isOpen()).toBe(false)
    })
  })
})
