import { vi } from 'vitest'

// Minimal RTCPeerConnection stand-in for useWebRTC.test.ts's hook-level tests (Sprint 53,
// FETEST-05) — jsdom has no WebRTC implementation. Covers exactly the surface useWebRTC.ts's
// connect()/getStats-polling code touches: offer/answer plumbing resolves immediately (fixed
// mock SDP, iceGatheringState pre-set to 'complete' so the Trickle-ICE wait short-circuits),
// and getStats() returns whatever Map the test installs via setStats().
export class MockRTCPeerConnection {
  static instances: MockRTCPeerConnection[] = []

  ontrack: ((event: { streams: MediaStream[] }) => void) | null = null
  oniceconnectionstatechange: (() => void) | null = null
  iceConnectionState: RTCIceConnectionState = 'new'
  iceGatheringState: RTCIceGatheringState = 'complete'
  localDescription: RTCSessionDescriptionInit | null = null
  closed = false

  private statsMap = new Map<string, unknown>()

  addTransceiver = vi.fn()
  addEventListener = vi.fn()
  removeEventListener = vi.fn()

  constructor() {
    MockRTCPeerConnection.instances.push(this)
  }

  createOffer(): Promise<RTCSessionDescriptionInit> {
    return Promise.resolve({ type: 'offer', sdp: 'mock-offer-sdp' })
  }

  setLocalDescription(desc: RTCSessionDescriptionInit): Promise<void> {
    this.localDescription = desc
    return Promise.resolve()
  }

  setRemoteDescription(): Promise<void> {
    return Promise.resolve()
  }

  close(): void {
    this.closed = true
  }

  setStats(entries: [string, unknown][]): void {
    this.statsMap = new Map(entries)
  }

  getStats(): Promise<Map<string, unknown>> {
    return Promise.resolve(this.statsMap)
  }

  static reset(): void {
    MockRTCPeerConnection.instances = []
  }
}
