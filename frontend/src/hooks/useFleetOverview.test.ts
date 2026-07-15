import { renderHook, act } from '@testing-library/react'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import { useFleetOverview } from './useFleetOverview'
import * as apiClient from '@/lib/api-client'

vi.mock('@/lib/api-client', () => ({
  listFleetVehicles: vi.fn(),
  listFleetAlerts: vi.fn(),
  acknowledgeFleetAlert: vi.fn(),
}))

// Mock FleetWSClient as a controllable stub — tests drive onOpen/onEvent manually instead of
// exercising a real WebSocket (same rationale as the pure fleet-ws-events.ts extraction: the
// merge/dispatch logic is what's worth testing here, not WebSocket plumbing itself).
let lastInstance: {
  connect: ReturnType<typeof vi.fn>
  disconnect: ReturnType<typeof vi.fn>
  onOpen: (() => void) | null
  onClose: (() => void) | null
  onEvent: ((event: unknown) => void) | null
} | null = null

vi.mock('@/lib/fleet-ws-client', () => {
  class MockFleetWSClient {
    connect = vi.fn()
    disconnect = vi.fn()
    onOpen: (() => void) | null = null
    onClose: (() => void) | null = null
    onEvent: ((event: unknown) => void) | null = null
    constructor() {
      // eslint-disable-next-line @typescript-eslint/no-this-alias
      lastInstance = this
    }
  }
  return { FleetWSClient: MockFleetWSClient }
})

const V1 = { id: 'v1', display_name: 'V1', battery_pct: 50 }
const A1 = { id: 'a1', vehicle_id: 'v1', severity: 'warning' as const, message: 'x', created_at: 't1' }

describe('useFleetOverview', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    lastInstance = null
  })

  it('token null: lädt nichts, bleibt im loading-Zustand', () => {
    const { result } = renderHook(() => useFleetOverview(null, null))
    expect(apiClient.listFleetVehicles).not.toHaveBeenCalled()
    expect(result.current.loading).toBe(true)
  })

  it('lädt Fahrzeuge und Alerts initial per REST', async () => {
    vi.mocked(apiClient.listFleetVehicles).mockResolvedValue([V1])
    vi.mocked(apiClient.listFleetAlerts).mockResolvedValue([A1])

    const { result } = renderHook(() => useFleetOverview('tok', 'op1'))
    await act(async () => {})

    expect(result.current.vehicles).toEqual([V1])
    expect(result.current.alerts).toEqual([A1])
    expect(result.current.loading).toBe(false)
    expect(result.current.error).toBeNull()
  })

  it('REST-Fehler beim initialen Laden setzt error, crasht nicht', async () => {
    vi.mocked(apiClient.listFleetVehicles).mockRejectedValue(new Error('down'))
    vi.mocked(apiClient.listFleetAlerts).mockResolvedValue([])

    const { result } = renderHook(() => useFleetOverview('tok', 'op1'))
    await act(async () => {})

    expect(result.current.error).toBeTruthy()
    expect(result.current.loading).toBe(false)
  })

  it('öffnet den WS-Client erst nach erfolgreichem initialen Fetch', async () => {
    vi.mocked(apiClient.listFleetVehicles).mockResolvedValue([V1])
    vi.mocked(apiClient.listFleetAlerts).mockResolvedValue([])

    renderHook(() => useFleetOverview('tok', 'op1'))
    await act(async () => {})

    expect(lastInstance?.connect).toHaveBeenCalledWith('tok')
  })

  it('vehicle_status-Event aktualisiert die Fahrzeugliste', async () => {
    vi.mocked(apiClient.listFleetVehicles).mockResolvedValue([V1])
    vi.mocked(apiClient.listFleetAlerts).mockResolvedValue([])

    const { result } = renderHook(() => useFleetOverview('tok', 'op1'))
    await act(async () => {})

    act(() => {
      lastInstance?.onEvent?.({
        type: 'vehicle_status',
        data: { vehicle_id: 'v1', battery_pct: 5, autonomy_mode: 'autonomous', updated_at: 't9' },
      })
    })

    expect(result.current.vehicles[0].battery_pct).toBe(5)
  })

  it('alert_created-Event stellt ein neues Alert voran', async () => {
    vi.mocked(apiClient.listFleetVehicles).mockResolvedValue([])
    vi.mocked(apiClient.listFleetAlerts).mockResolvedValue([A1])

    const { result } = renderHook(() => useFleetOverview('tok', 'op1'))
    await act(async () => {})

    act(() => {
      lastInstance?.onEvent?.({
        type: 'alert_created',
        data: { id: 'a2', vehicle_id: 'v2', severity: 'critical', message: 'y', created_at: 't2' },
      })
    })

    expect(result.current.alerts.map((a) => a.id)).toEqual(['a2', 'a1'])
  })

  it('unbekannter/task_created-Event wird ignoriert, crasht nicht', async () => {
    vi.mocked(apiClient.listFleetVehicles).mockResolvedValue([V1])
    vi.mocked(apiClient.listFleetAlerts).mockResolvedValue([A1])

    const { result } = renderHook(() => useFleetOverview('tok', 'op1'))
    await act(async () => {})

    expect(() => {
      act(() => {
        lastInstance?.onEvent?.({ type: 'unknown', rawType: 'x', data: {} })
      })
    }).not.toThrow()
    expect(result.current.vehicles).toEqual([V1])
    expect(result.current.alerts).toEqual([A1])
  })

  it('Reconnect (zweites onOpen) löst einen erneuten REST-Fetch aus (Resync, kein Backlog im Backend)', async () => {
    vi.mocked(apiClient.listFleetVehicles).mockResolvedValue([V1])
    vi.mocked(apiClient.listFleetAlerts).mockResolvedValue([A1])

    renderHook(() => useFleetOverview('tok', 'op1'))
    await act(async () => {})
    const callsAfterInitial = vi.mocked(apiClient.listFleetVehicles).mock.calls.length

    act(() => { lastInstance?.onOpen?.() }) // first open — must NOT trigger an extra fetch
    await act(async () => {})
    expect(vi.mocked(apiClient.listFleetVehicles).mock.calls.length).toBe(callsAfterInitial)

    act(() => { lastInstance?.onOpen?.() }) // second open (reconnect) — must resync
    await act(async () => {})
    expect(vi.mocked(apiClient.listFleetVehicles).mock.calls.length).toBe(callsAfterInitial + 1)
  })

  it('unmount trennt die WS-Verbindung', async () => {
    vi.mocked(apiClient.listFleetVehicles).mockResolvedValue([V1])
    vi.mocked(apiClient.listFleetAlerts).mockResolvedValue([])

    const { unmount } = renderHook(() => useFleetOverview('tok', 'op1'))
    await act(async () => {})
    const instance = lastInstance

    unmount()

    expect(instance?.disconnect).toHaveBeenCalled()
  })

  describe('acknowledgeAlert', () => {
    it('ruft die REST-API auf und aktualisiert optimistisch lokal', async () => {
      vi.mocked(apiClient.listFleetVehicles).mockResolvedValue([])
      vi.mocked(apiClient.listFleetAlerts).mockResolvedValue([A1])
      vi.mocked(apiClient.acknowledgeFleetAlert).mockResolvedValue(undefined)

      const { result } = renderHook(() => useFleetOverview('tok', 'op1'))
      await act(async () => {})

      await act(async () => { await result.current.acknowledgeAlert('a1') })

      expect(apiClient.acknowledgeFleetAlert).toHaveBeenCalledWith('tok', 'a1', 'op1')
      expect(result.current.alerts[0].acknowledged_by).toBe('op1')
    })

    it('wirft bei REST-Fehler weiter, statt ihn zu verschlucken (sichtbares Feedback für den Aufrufer)', async () => {
      vi.mocked(apiClient.listFleetVehicles).mockResolvedValue([])
      vi.mocked(apiClient.listFleetAlerts).mockResolvedValue([A1])
      vi.mocked(apiClient.acknowledgeFleetAlert).mockRejectedValue(new Error('failed'))

      const { result } = renderHook(() => useFleetOverview('tok', 'op1'))
      await act(async () => {})

      await expect(result.current.acknowledgeAlert('a1')).rejects.toThrow('failed')
    })

    it('ohne operatorId: no-op, ruft die REST-API nicht auf', async () => {
      vi.mocked(apiClient.listFleetVehicles).mockResolvedValue([])
      vi.mocked(apiClient.listFleetAlerts).mockResolvedValue([A1])

      const { result } = renderHook(() => useFleetOverview('tok', null))
      await act(async () => {})

      await act(async () => { await result.current.acknowledgeAlert('a1') })

      expect(apiClient.acknowledgeFleetAlert).not.toHaveBeenCalled()
    })
  })
})
