import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import {
  computeLossRatio,
  computeBitrateBps,
  isDegradedSample,
  nextStreak,
  nextMediaStateFromStreak,
  useWebRTC,
  type StreakState,
} from './useWebRTC'
import { reportMediaState } from '@/lib/api-client'
import { MockRTCPeerConnection } from '@/test/mock-rtc-peer-connection'

// DRIFT-K3 (2026-07-16): unit tests for the pure MEDIA_DEGRADED decision logic.
// The getStats()/RTCPeerConnection plumbing around these functions has no
// existing mock infrastructure in this codebase (VideoPanel.test.tsx mocks
// useWebRTC's return value wholesale, not RTCPeerConnection internals) —
// these functions were extracted specifically so the safety-relevant
// threshold/hysteresis behavior is testable without building that mock.

vi.mock('@/lib/api-client', () => ({
  reportMediaState: vi.fn().mockResolvedValue(undefined),
}))

vi.mock('@/lib/logger', () => ({
  logEvent: vi.fn(),
  FE_WEBRTC_STATE: 'FE_WEBRTC_STATE_CHANGE',
}))

describe('computeLossRatio', () => {
  it('gibt 0 zurück, wenn noch keine Pakete gesehen wurden', () => {
    expect(computeLossRatio(0, 0)).toBe(0)
  })

  it('berechnet das Verlust-Verhältnis aus verlorenen und empfangenen Paketen', () => {
    expect(computeLossRatio(5, 95)).toBe(0.05)
  })

  it('gibt 1 zurück, wenn alle Pakete verloren gingen', () => {
    expect(computeLossRatio(10, 0)).toBe(1)
  })
})

describe('computeBitrateBps', () => {
  it('berechnet Bits/Sekunde aus Byte-Delta und Zeit-Delta', () => {
    // 12500 bytes over 1s = 100_000 bits/s
    expect(computeBitrateBps(12_500, 1)).toBe(100_000)
  })

  it('gibt null zurück bei nicht-positivem Zeit-Delta (z. B. Uhr-Rückgang)', () => {
    expect(computeBitrateBps(1000, 0)).toBeNull()
    expect(computeBitrateBps(1000, -1)).toBeNull()
  })

  it('gibt 0 zurück, wenn sich bytesReceived nicht verändert hat', () => {
    expect(computeBitrateBps(0, 1)).toBe(0)
  })
})

describe('isDegradedSample', () => {
  it('ist false, wenn beide Schwellwerte eingehalten werden', () => {
    expect(isDegradedSample(0.01, 500_000)).toBe(false)
  })

  it('ist true, wenn die Paketverlustrate > 5% liegt', () => {
    expect(isDegradedSample(0.051, 500_000)).toBe(true)
  })

  it('ist false genau an der 5%-Grenze (Grenzwert exklusiv)', () => {
    expect(isDegradedSample(0.05, 500_000)).toBe(false)
  })

  it('ist true, wenn die Bitrate unter 100kbps fällt', () => {
    expect(isDegradedSample(0, 99_999)).toBe(true)
  })

  it('ist false genau an der 100kbps-Grenze (Grenzwert exklusiv)', () => {
    expect(isDegradedSample(0, 100_000)).toBe(false)
  })

  it('ignoriert die Bitrate-Schwelle, wenn noch kein Bitrate-Sample vorliegt (null)', () => {
    expect(isDegradedSample(0, null)).toBe(false)
  })

  it('ist true, wenn beide Schwellwerte gleichzeitig verletzt werden', () => {
    expect(isDegradedSample(0.5, 1_000)).toBe(true)
  })
})

describe('nextStreak', () => {
  const zero: StreakState = { degradedStreak: 0, healthyStreak: 0 }

  it('erhöht degradedStreak und setzt healthyStreak zurück bei einem schlechten Sample', () => {
    expect(nextStreak(zero, true)).toEqual({ degradedStreak: 1, healthyStreak: 0 })
  })

  it('erhöht healthyStreak und setzt degradedStreak zurück bei einem guten Sample', () => {
    expect(nextStreak(zero, false)).toEqual({ degradedStreak: 0, healthyStreak: 1 })
  })

  it('zählt aufeinanderfolgende schlechte Samples hoch, ein gutes Sample setzt sofort zurück', () => {
    let s = zero
    s = nextStreak(s, true)
    s = nextStreak(s, true)
    expect(s.degradedStreak).toBe(2)
    s = nextStreak(s, false)
    expect(s).toEqual({ degradedStreak: 0, healthyStreak: 1 })
  })
})

describe('nextMediaStateFromStreak', () => {
  it('bleibt bei MEDIA_CONNECTED, solange der degradedStreak unter der Sustain-Schwelle liegt', () => {
    expect(nextMediaStateFromStreak('MEDIA_CONNECTED', { degradedStreak: 2, healthyStreak: 0 })).toBeNull()
  })

  it('wechselt zu MEDIA_DEGRADED, sobald 3 aufeinanderfolgende schlechte Samples erreicht sind', () => {
    expect(nextMediaStateFromStreak('MEDIA_CONNECTED', { degradedStreak: 3, healthyStreak: 0 })).toBe('MEDIA_DEGRADED')
  })

  it('bleibt bei MEDIA_DEGRADED, solange der healthyStreak unter der Sustain-Schwelle liegt', () => {
    expect(nextMediaStateFromStreak('MEDIA_DEGRADED', { degradedStreak: 0, healthyStreak: 2 })).toBeNull()
  })

  it('erholt sich zu MEDIA_CONNECTED, sobald 3 aufeinanderfolgende gute Samples erreicht sind', () => {
    expect(nextMediaStateFromStreak('MEDIA_DEGRADED', { degradedStreak: 0, healthyStreak: 3 })).toBe('MEDIA_CONNECTED')
  })

  it('rührt MEDIA_FAILED nicht an — nur CONNECTED/DEGRADED sind Streak-getrieben', () => {
    expect(nextMediaStateFromStreak('MEDIA_FAILED', { degradedStreak: 10, healthyStreak: 0 })).toBeNull()
  })

  it('rührt MEDIA_INIT/MEDIA_NEGOTIATING nicht an', () => {
    expect(nextMediaStateFromStreak('MEDIA_INIT', { degradedStreak: 10, healthyStreak: 0 })).toBeNull()
    expect(nextMediaStateFromStreak('MEDIA_NEGOTIATING', { degradedStreak: 10, healthyStreak: 0 })).toBeNull()
  })
})

// Hook-level tests (Sprint 53, FETEST-05) — Connection-State-Übergänge, Fokus auf Zeile
// 206-265: der Stats-Polling-Loop (der die obigen reinen Funktionen tatsächlich verkabelt) und
// der Auto-Retry-Effect nach MEDIA_FAILED. RTCPeerConnection/fetch werden gemockt (siehe
// mock-rtc-peer-connection.ts) — bewusst nicht die reale SDP/ICE-Verhandlung (ADR-006-Policy,
// wie schon bei den Backend-Integrationstests in Sprint 52).

function mockFetch(url: string): Promise<{ ok: boolean; json?: () => Promise<unknown>; text?: () => Promise<string> }> {
  if (url.includes('/api/ice-config')) {
    return Promise.resolve({ ok: true, json: () => Promise.resolve({ iceServers: [{ urls: 'stun:127.0.0.1:3478' }] }) })
  }
  if (url.includes('/whep/')) {
    return Promise.resolve({ ok: true, text: () => Promise.resolve('mock-answer-sdp') })
  }
  return Promise.reject(new Error(`unexpected fetch in test: ${url}`))
}

// Advances only the microtask queue (plain Promise chains) — unlike setTimeout/setInterval,
// Promise resolution is NOT affected by vi.useFakeTimers(), so this safely drains connect()'s
// chained awaits (fetch → RTCPeerConnection → createOffer → ... → setRemoteDescription) even
// while fake timers are active for the interval-driven logic under test.
async function flushMicrotasks(ticks = 20) {
  for (let i = 0; i < ticks; i++) {
    await Promise.resolve()
  }
}

describe('useWebRTC (Hook, Connection-State-Übergänge)', () => {
  beforeEach(() => {
    MockRTCPeerConnection.reset()
    vi.stubGlobal('RTCPeerConnection', MockRTCPeerConnection)
    vi.stubGlobal('fetch', vi.fn(mockFetch))
    vi.clearAllMocks()
    // Fake timers from the start: the stats-polling/auto-retry effects under test register
    // setInterval/setTimeout as soon as MEDIA_CONNECTED is reached, during renderConnected()
    // itself — switching to fake timers only afterward would leave that interval as a real,
    // un-advanceable timer.
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  // Drives the hook through its real connect() flow up to MEDIA_CONNECTED.
  async function renderConnected() {
    const hook = renderHook(
      ({ enabled }) => useWebRTC('sess-1', 'veh-1', 'tok-1', enabled),
      { initialProps: { enabled: true } },
    )
    await act(async () => { await flushMicrotasks() })
    expect(MockRTCPeerConnection.instances).toHaveLength(1)
    const pc = MockRTCPeerConnection.instances[0]
    // videoRef is a RefObject<HTMLVideoElement> (readonly `current` in its public type) — normally
    // only React itself assigns it via the `ref={videoRef}` JSX prop. renderHook mounts no real
    // <video>, so the test attaches one directly the same way React's DOM reconciler would.
    ;(hook.result.current.videoRef as unknown as { current: HTMLVideoElement | null }).current =
      document.createElement('video')

    await act(async () => {
      pc.ontrack?.({ streams: [{} as MediaStream] })
    })
    expect(hook.result.current.mediaState).toBe('MEDIA_CONNECTED')
    return { ...hook, pc }
  }

  const HEALTHY_INBOUND: [string, unknown] = ['ib1', { type: 'inbound-rtp', kind: 'video', packetsLost: 0, packetsReceived: 100 }]
  const DEGRADED_INBOUND: [string, unknown] = ['ib1', { type: 'inbound-rtp', kind: 'video', packetsLost: 10, packetsReceived: 90 }]
  const RTT_PAIR = (rtt: number): [string, unknown] =>
    ['pair1', { type: 'candidate-pair', nominated: true, currentRoundTripTime: rtt }]

  describe('Stats-Polling (verbunden/degraded)', () => {
    it('aktualisiert videoLatencyMs aus dem nominierten candidate-pair RTT', async () => {
      const { result, pc } = await renderConnected()
      pc.setStats([RTT_PAIR(0.042), HEALTHY_INBOUND])

      await act(async () => { await vi.advanceTimersByTimeAsync(1000) })

      expect(result.current.videoLatencyMs).toBe(42)
    })

    it('wechselt zu MEDIA_DEGRADED nach 3 aufeinanderfolgenden Samples über der Verlust-Schwelle', async () => {
      const { result, pc } = await renderConnected()
      pc.setStats([RTT_PAIR(0.01), DEGRADED_INBOUND])

      await act(async () => { await vi.advanceTimersByTimeAsync(1000) })
      expect(result.current.mediaState).toBe('MEDIA_CONNECTED')
      await act(async () => { await vi.advanceTimersByTimeAsync(1000) })
      expect(result.current.mediaState).toBe('MEDIA_CONNECTED')
      await act(async () => { await vi.advanceTimersByTimeAsync(1000) })

      expect(result.current.mediaState).toBe('MEDIA_DEGRADED')
      expect(vi.mocked(reportMediaState)).toHaveBeenCalledWith('MEDIA_DEGRADED', 'veh-1', 'tok-1')
    })

    it('erholt sich zu MEDIA_CONNECTED nach 3 aufeinanderfolgenden gesunden Samples', async () => {
      const { result, pc } = await renderConnected()
      pc.setStats([RTT_PAIR(0.01), DEGRADED_INBOUND])
      await act(async () => { await vi.advanceTimersByTimeAsync(3000) })
      expect(result.current.mediaState).toBe('MEDIA_DEGRADED')

      pc.setStats([RTT_PAIR(0.01), HEALTHY_INBOUND])
      await act(async () => { await vi.advanceTimersByTimeAsync(3000) })

      expect(result.current.mediaState).toBe('MEDIA_CONNECTED')
    })

    it('stoppt das Polling und setzt videoLatencyMs zurück, sobald der State CONNECTED/DEGRADED verlässt', async () => {
      const { result, pc } = await renderConnected()
      pc.setStats([RTT_PAIR(0.05), HEALTHY_INBOUND])
      await act(async () => { await vi.advanceTimersByTimeAsync(1000) })
      expect(result.current.videoLatencyMs).toBe(50)

      await act(async () => {
        pc.iceConnectionState = 'failed'
        pc.oniceconnectionstatechange?.()
      })
      expect(result.current.mediaState).toBe('MEDIA_FAILED')
      expect(result.current.videoLatencyMs).toBeNull()

      const callsWhileFailed = vi.mocked(reportMediaState).mock.calls.length
      await act(async () => { await vi.advanceTimersByTimeAsync(2000) }) // would've been 2 more poll ticks
      expect(vi.mocked(reportMediaState).mock.calls.length).toBe(callsWhileFailed)
    })
  })

  describe('Auto-Retry nach MEDIA_FAILED', () => {
    it('verbindet nach 3000ms automatisch neu (neue RTCPeerConnection, alte wird geschlossen)', async () => {
      const { result, pc } = await renderConnected()

      await act(async () => {
        pc.iceConnectionState = 'failed'
        pc.oniceconnectionstatechange?.()
      })
      expect(result.current.mediaState).toBe('MEDIA_FAILED')

      await act(async () => { await vi.advanceTimersByTimeAsync(3000) })

      expect(pc.closed).toBe(true)
      expect(MockRTCPeerConnection.instances.length).toBeGreaterThan(1)
    })

    it('plant KEINEN Retry mehr, wenn enabled vor Ablauf der 3000ms auf false wechselt', async () => {
      const { result, pc, rerender } = await renderConnected()

      await act(async () => {
        pc.iceConnectionState = 'failed'
        pc.oniceconnectionstatechange?.()
      })
      expect(result.current.mediaState).toBe('MEDIA_FAILED')

      // Disabling (e.g. operator navigates away) tears the connection down via the
      // connect/disconnect effect — the auto-retry effect's cleanup must cancel its pending
      // setTimeout as a result, rather than reconnecting a no-longer-wanted session.
      rerender({ enabled: false })
      expect(result.current.mediaState).toBe('MEDIA_INIT')

      const instancesBefore = MockRTCPeerConnection.instances.length
      await act(async () => { await vi.advanceTimersByTimeAsync(3000) })

      expect(MockRTCPeerConnection.instances.length).toBe(instancesBefore)
    })
  })
})
