import { describe, it, expect } from 'vitest'
import {
  computeLossRatio,
  computeBitrateBps,
  isDegradedSample,
  nextStreak,
  nextMediaStateFromStreak,
  type StreakState,
} from './useWebRTC'

// DRIFT-K3 (2026-07-16): unit tests for the pure MEDIA_DEGRADED decision logic.
// The getStats()/RTCPeerConnection plumbing around these functions has no
// existing mock infrastructure in this codebase (VideoPanel.test.tsx mocks
// useWebRTC's return value wholesale, not RTCPeerConnection internals) —
// these functions were extracted specifically so the safety-relevant
// threshold/hysteresis behavior is testable without building that mock.

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
