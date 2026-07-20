import { renderHook, act } from '@testing-library/react'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import { useVehiclePositionHistory } from './useVehiclePositionHistory'
import * as apiClient from '@/lib/api-client'

vi.mock('@/lib/api-client', () => ({
  listVehiclePositionHistory: vi.fn(),
}))

const P1 = { vehicle_id: 'v1', position_lat: 52.13, position_lon: 11.64, recorded_at: 't1' }
const P2 = { vehicle_id: 'v1', position_lat: 52.131, position_lon: 11.641, recorded_at: 't2' }

describe('useVehiclePositionHistory', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('kein Token: lädt nichts, bleibt im idle-Zustand (nicht loading)', () => {
    const { result } = renderHook(() => useVehiclePositionHistory(null, 'v1'))
    expect(apiClient.listVehiclePositionHistory).not.toHaveBeenCalled()
    expect(result.current.loading).toBe(false)
    expect(result.current.points).toEqual([])
  })

  it('kein ausgewähltes Fahrzeug: lädt nichts, leere Punkte', () => {
    const { result } = renderHook(() => useVehiclePositionHistory('tok', null))
    expect(apiClient.listVehiclePositionHistory).not.toHaveBeenCalled()
    expect(result.current.points).toEqual([])
  })

  it('lädt die Historie für das ausgewählte Fahrzeug per REST', async () => {
    vi.mocked(apiClient.listVehiclePositionHistory).mockResolvedValue([P1, P2])

    const { result } = renderHook(() => useVehiclePositionHistory('tok', 'v1'))
    await act(async () => {})

    expect(apiClient.listVehiclePositionHistory).toHaveBeenCalledWith('tok', 'v1')
    expect(result.current.points).toEqual([P1, P2])
    expect(result.current.loading).toBe(false)
    expect(result.current.error).toBeNull()
  })

  it('wechselt das Fahrzeug: fetcht erneut mit der neuen vehicleId', async () => {
    vi.mocked(apiClient.listVehiclePositionHistory).mockResolvedValue([P1])

    const { result, rerender } = renderHook(({ vehicleId }) => useVehiclePositionHistory('tok', vehicleId), {
      initialProps: { vehicleId: 'v1' },
    })
    await act(async () => {})
    expect(apiClient.listVehiclePositionHistory).toHaveBeenCalledWith('tok', 'v1')

    vi.mocked(apiClient.listVehiclePositionHistory).mockResolvedValue([P2])
    rerender({ vehicleId: 'v2' })
    await act(async () => {})

    expect(apiClient.listVehiclePositionHistory).toHaveBeenCalledWith('tok', 'v2')
    expect(result.current.points).toEqual([P2])
  })

  it('Fahrzeug auf null zurückgesetzt: leert die Punkte, kein Fetch mehr', async () => {
    vi.mocked(apiClient.listVehiclePositionHistory).mockResolvedValue([P1])

    const { result, rerender } = renderHook(({ vehicleId }) => useVehiclePositionHistory('tok', vehicleId), {
      initialProps: { vehicleId: 'v1' as string | null },
    })
    await act(async () => {})
    expect(result.current.points).toEqual([P1])

    rerender({ vehicleId: null })
    expect(result.current.points).toEqual([])
    expect(apiClient.listVehiclePositionHistory).toHaveBeenCalledTimes(1)
  })

  it('REST-Fehler setzt error, crasht nicht', async () => {
    vi.mocked(apiClient.listVehiclePositionHistory).mockRejectedValue(new Error('down'))

    const { result } = renderHook(() => useVehiclePositionHistory('tok', 'v1'))
    await act(async () => {})

    expect(result.current.error).toBeTruthy()
    expect(result.current.loading).toBe(false)
  })

  it('unmount vor Resolve: kein act()-Warning, kein State-Update danach', async () => {
    let resolvePoints!: (value: typeof P1[]) => void
    vi.mocked(apiClient.listVehiclePositionHistory).mockReturnValue(new Promise((resolve) => { resolvePoints = resolve }))

    const { unmount } = renderHook(() => useVehiclePositionHistory('tok', 'v1'))
    unmount()

    await act(async () => {
      resolvePoints([P1])
    })
  })
})
