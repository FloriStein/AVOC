// Minimal WebSocket stand-in shared by lib/ws-client.test.ts and lib/fleet-ws-client.test.ts.
// jsdom has no real WebSocket transport, and neither client has an existing test file/mock to
// build on (ws-client.ts's own comment notes it purposefully has no colocated unit test).
// Captures enough of the native contract (readyState, onopen/onmessage/onclose, send/close) to
// drive both clients' connect/reconnect/close logic, plus `closeCalledWithNulledHandler` to
// assert the exact ordering of the Sprint-14 race-condition fix (`ws.onclose = null` BEFORE
// `ws.close()`) rather than just its end effect.

type Listener = (() => void) | null
type MessageListener = ((event: { data: unknown }) => void) | null

export class MockWebSocket {
  static readonly CONNECTING = 0
  static readonly OPEN = 1
  static readonly CLOSING = 2
  static readonly CLOSED = 3

  static instances: MockWebSocket[] = []

  readonly url: string
  readyState = MockWebSocket.CONNECTING
  binaryType = ''
  sent: unknown[] = []
  closeCalled = false
  /** True if `onclose` was already null at the moment `close()` was invoked. */
  closeCalledWithNulledHandler = false

  onopen: Listener = null
  onmessage: MessageListener = null
  onclose: Listener = null

  constructor(url: string) {
    this.url = url
    MockWebSocket.instances.push(this)
  }

  send(data: unknown): void {
    this.sent.push(data)
  }

  close(): void {
    this.closeCalledWithNulledHandler = this.onclose === null
    this.closeCalled = true
    this.readyState = MockWebSocket.CLOSED
  }

  // Test helpers — simulate the browser dispatching events on this socket.
  triggerOpen(): void {
    this.readyState = MockWebSocket.OPEN
    this.onopen?.()
  }

  triggerMessage(data: unknown): void {
    this.onmessage?.({ data })
  }

  /** Simulates the browser eventually firing 'close', whatever handler is CURRENTLY assigned. */
  triggerClose(): void {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.()
  }

  static reset(): void {
    MockWebSocket.instances = []
  }
}
