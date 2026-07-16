import { renderHook, act } from '@testing-library/react'
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import { useFleetAlertSound } from './useFleetAlertSound'
import * as fleetAlertSound from '@/lib/fleet-alert-sound'
import type { FleetAlert } from '@/lib/api-client'

vi.mock('@/lib/fleet-alert-sound', async (importOriginal) => {
  const actual = await importOriginal<typeof fleetAlertSound>()
  return { ...actual, playAlertTone: vi.fn() }
})

const MUTE_KEY = 'fleet-alert-sound-muted'

const warning: FleetAlert = { id: 'a1', vehicle_id: 'v1', severity: 'warning', message: 'x', created_at: 't1' }
const critical: FleetAlert = { id: 'a2', vehicle_id: 'v1', severity: 'critical', message: 'y', created_at: 't2' }
const info: FleetAlert = { id: 'a3', vehicle_id: 'v1', severity: 'info', message: 'z', created_at: 't3' }

class FakeAudioContext {
  currentTime = 0
  destination = {}
  resume = vi.fn().mockResolvedValue(undefined)
  createGain = vi.fn(() => ({
    gain: { setValueAtTime: vi.fn(), linearRampToValueAtTime: vi.fn() },
    connect: vi.fn(),
  }))
  createOscillator = vi.fn(() => ({
    type: '',
    frequency: { setValueAtTime: vi.fn(), linearRampToValueAtTime: vi.fn() },
    connect: vi.fn(),
    start: vi.fn(),
    stop: vi.fn(),
  }))
}

describe('useFleetAlertSound', () => {
  let originalAudioContext: typeof window.AudioContext | undefined

  beforeEach(() => {
    vi.clearAllMocks()
    window.localStorage.clear()
    originalAudioContext = window.AudioContext
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    window.AudioContext = FakeAudioContext as any
  })

  afterEach(() => {
    window.AudioContext = originalAudioContext as typeof window.AudioContext
  })

  it('während loading: kein Ton, muted-Default false', () => {
    const { result } = renderHook(() => useFleetAlertSound([warning], true))
    expect(result.current.muted).toBe(false)
    expect(fleetAlertSound.playAlertTone).not.toHaveBeenCalled()
  })

  it('initialer REST-Snapshot (loading true -> false) spielt keinen Ton, auch bei bereits vorhandenen warning/critical-Alerts', () => {
    const { rerender } = renderHook(({ alerts, loading }) => useFleetAlertSound(alerts, loading), {
      initialProps: { alerts: [warning, critical] as FleetAlert[], loading: true },
    })

    rerender({ alerts: [warning, critical], loading: false })

    expect(fleetAlertSound.playAlertTone).not.toHaveBeenCalled()
  })

  it('ein tatsächlich neuer warning-Alert nach der Baseline spielt einen Ton', () => {
    const { rerender } = renderHook(({ alerts, loading }) => useFleetAlertSound(alerts, loading), {
      initialProps: { alerts: [] as FleetAlert[], loading: true },
    })
    rerender({ alerts: [], loading: false }) // Baseline: keine Alerts

    act(() => {
      rerender({ alerts: [warning], loading: false })
    })

    expect(fleetAlertSound.playAlertTone).toHaveBeenCalledTimes(1)
  })

  it('ein neuer info-Alert spielt keinen Ton', () => {
    const { rerender } = renderHook(({ alerts, loading }) => useFleetAlertSound(alerts, loading), {
      initialProps: { alerts: [] as FleetAlert[], loading: true },
    })
    rerender({ alerts: [], loading: false })

    act(() => {
      rerender({ alerts: [info], loading: false })
    })

    expect(fleetAlertSound.playAlertTone).not.toHaveBeenCalled()
  })

  it('stummgeschaltet: kein Ton trotz neuem warning-Alert', () => {
    const { result, rerender } = renderHook(({ alerts, loading }) => useFleetAlertSound(alerts, loading), {
      initialProps: { alerts: [] as FleetAlert[], loading: true },
    })
    rerender({ alerts: [], loading: false })

    act(() => result.current.toggleMuted())
    expect(result.current.muted).toBe(true)

    act(() => {
      rerender({ alerts: [warning], loading: false })
    })

    expect(fleetAlertSound.playAlertTone).not.toHaveBeenCalled()
  })

  it('toggleMuted persistiert in localStorage; ein neuer Hook-Mount liest den Wert', () => {
    const { result } = renderHook(() => useFleetAlertSound([], false))
    act(() => result.current.toggleMuted())
    expect(window.localStorage.getItem(MUTE_KEY)).toBe('1')

    const { result: result2 } = renderHook(() => useFleetAlertSound([], false))
    expect(result2.current.muted).toBe(true)
  })

  it('Sound-Storm: mehrere neue hörbare Alerts im selben Update spielen nur einen Ton', () => {
    const { rerender } = renderHook(({ alerts, loading }) => useFleetAlertSound(alerts, loading), {
      initialProps: { alerts: [] as FleetAlert[], loading: true },
    })
    rerender({ alerts: [], loading: false })

    act(() => {
      rerender({ alerts: [warning, critical], loading: false })
    })

    expect(fleetAlertSound.playAlertTone).toHaveBeenCalledTimes(1)
  })

  it('Debounce: zwei neue Alerts kurz hintereinander (getrennte Updates) spielen nur einen Ton', () => {
    const { rerender } = renderHook(({ alerts, loading }) => useFleetAlertSound(alerts, loading), {
      initialProps: { alerts: [] as FleetAlert[], loading: true },
    })
    rerender({ alerts: [], loading: false })

    act(() => {
      rerender({ alerts: [warning], loading: false })
    })
    act(() => {
      rerender({ alerts: [warning, critical], loading: false })
    })

    expect(fleetAlertSound.playAlertTone).toHaveBeenCalledTimes(1)
  })

  it('bereits gesehene Alert-ids (Resync mit gleichen Daten) lösen keinen erneuten Ton aus', () => {
    const { rerender } = renderHook(({ alerts, loading }) => useFleetAlertSound(alerts, loading), {
      initialProps: { alerts: [] as FleetAlert[], loading: true },
    })
    rerender({ alerts: [], loading: false })

    act(() => {
      rerender({ alerts: [warning], loading: false })
    })
    expect(fleetAlertSound.playAlertTone).toHaveBeenCalledTimes(1)

    act(() => {
      // Simuliert einen Resync nach WS-Reconnect mit identischem Alert (kein neuer)
      rerender({ alerts: [{ ...warning }], loading: false })
    })
    expect(fleetAlertSound.playAlertTone).toHaveBeenCalledTimes(1)
  })

  it('AudioContext nicht verfügbar (window.AudioContext undefined): kein Crash, kein Ton', () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    window.AudioContext = undefined as any

    const { rerender } = renderHook(({ alerts, loading }) => useFleetAlertSound(alerts, loading), {
      initialProps: { alerts: [] as FleetAlert[], loading: true },
    })
    rerender({ alerts: [], loading: false })

    expect(() => {
      act(() => {
        rerender({ alerts: [warning], loading: false })
      })
    }).not.toThrow()
    expect(fleetAlertSound.playAlertTone).not.toHaveBeenCalled()
  })

  it('AudioContext-Konstruktor wirft (blockiert): kein Crash, kein Ton', () => {
    window.AudioContext = vi.fn(() => {
      throw new Error('blocked')
    }) as unknown as typeof AudioContext

    const { rerender } = renderHook(({ alerts, loading }) => useFleetAlertSound(alerts, loading), {
      initialProps: { alerts: [] as FleetAlert[], loading: true },
    })
    rerender({ alerts: [], loading: false })

    expect(() => {
      act(() => {
        rerender({ alerts: [warning], loading: false })
      })
    }).not.toThrow()
    expect(fleetAlertSound.playAlertTone).not.toHaveBeenCalled()
  })

  it('playAlertTone wirft (z. B. context geschlossen): kein Crash', () => {
    vi.mocked(fleetAlertSound.playAlertTone).mockImplementationOnce(() => {
      throw new Error('closed')
    })

    const { rerender } = renderHook(({ alerts, loading }) => useFleetAlertSound(alerts, loading), {
      initialProps: { alerts: [] as FleetAlert[], loading: true },
    })
    rerender({ alerts: [], loading: false })

    expect(() => {
      act(() => {
        rerender({ alerts: [warning], loading: false })
      })
    }).not.toThrow()
  })

  it('localStorage nicht verfügbar (z. B. privater Modus wirft bei getItem): kein Crash, muted false', () => {
    const spy = vi.spyOn(window.localStorage.__proto__, 'getItem').mockImplementation(() => {
      throw new Error('blocked')
    })

    let result: ReturnType<typeof useFleetAlertSound> | undefined
    expect(() => {
      const rendered = renderHook(() => useFleetAlertSound([], false))
      result = rendered.result.current
    }).not.toThrow()
    expect(result?.muted).toBe(false)

    spy.mockRestore()
  })
})
