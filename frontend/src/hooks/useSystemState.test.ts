import { renderHook, act } from '@testing-library/react'
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import { useSystemState } from './useSystemState'
import * as apiClient from '@/lib/api-client'

// MV-07 (ADR-026): useSystemState is vehicle-scoped — GET /vehicles/{id}/state once a
// vehicleId is known, GET /sessions as a pure reachability probe before that (no more
// global GET /state). Written before the implementation (TDD) — see Sprint 17.
vi.mock('@/lib/api-client', () => ({
  getVehicleState: vi.fn(),
  listSessions: vi.fn(),
}))

const CONNECTED = { system: 'CONNECTED', control: 'CONTROL_ACTIVE', media: 'MEDIA_CONNECTED', operator: 'ACTIVE_OPERATOR' }
const INITIAL = { system: 'IDLE', control: 'CONTROL_INIT', media: 'MEDIA_INIT', operator: 'NO_OPERATOR' }

describe('useSystemState', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('vehicleId gesetzt: pollt getVehicleState und übernimmt den 4-Layer-State', async () => {
    vi.mocked(apiClient.getVehicleState).mockResolvedValue(CONNECTED)
    const { result } = renderHook(() => useSystemState('vehicle-001', 'tok'))

    await act(async () => { await vi.advanceTimersByTimeAsync(0) })

    expect(apiClient.getVehicleState).toHaveBeenCalledWith('vehicle-001')
    expect(result.current.system).toBe('CONNECTED')
  })

  it('vehicleId null: ruft getVehicleState NICHT auf, nutzt listSessions als Reachability-Probe', async () => {
    vi.mocked(apiClient.listSessions).mockResolvedValue([])
    renderHook(() => useSystemState(null, 'tok'))

    await act(async () => { await vi.advanceTimersByTimeAsync(0) })

    expect(apiClient.listSessions).toHaveBeenCalledWith('tok')
    expect(apiClient.getVehicleState).not.toHaveBeenCalled()
  })

  it('vehicleId null: State bleibt am IDLE/NO_OPERATOR-Default', async () => {
    vi.mocked(apiClient.listSessions).mockResolvedValue([])
    const { result } = renderHook(() => useSystemState(null, 'tok'))

    await act(async () => { await vi.advanceTimersByTimeAsync(0) })

    expect(result.current).toMatchObject(INITIAL)
  })

  it('pollt alle 500ms erneut', async () => {
    vi.mocked(apiClient.getVehicleState).mockResolvedValue(CONNECTED)
    renderHook(() => useSystemState('vehicle-001', 'tok'))

    await act(async () => { await vi.advanceTimersByTimeAsync(0) })
    await act(async () => { await vi.advanceTimersByTimeAsync(500) })
    await act(async () => { await vi.advanceTimersByTimeAsync(500) })

    expect(apiClient.getVehicleState).toHaveBeenCalledTimes(3)
  })

  it('3 aufeinanderfolgende Fehlschläge → unreachable wird true', async () => {
    vi.mocked(apiClient.getVehicleState).mockRejectedValue(new Error('network'))
    const { result } = renderHook(() => useSystemState('vehicle-001', 'tok'))

    await act(async () => { await vi.advanceTimersByTimeAsync(0) })
    await act(async () => { await vi.advanceTimersByTimeAsync(500) })
    expect(result.current.unreachable).toBe(false)

    await act(async () => { await vi.advanceTimersByTimeAsync(500) })
    expect(result.current.unreachable).toBe(true)
  })

  it('Erfolg vor Erreichen des Schwellwerts setzt den Fehlerzähler zurück', async () => {
    vi.mocked(apiClient.getVehicleState)
      .mockRejectedValueOnce(new Error('x'))
      .mockRejectedValueOnce(new Error('x'))
      .mockResolvedValueOnce(CONNECTED)
      .mockRejectedValueOnce(new Error('x'))
      .mockRejectedValueOnce(new Error('x'))
    const { result } = renderHook(() => useSystemState('vehicle-001', 'tok'))

    await act(async () => { await vi.advanceTimersByTimeAsync(0) })   // fail 1
    await act(async () => { await vi.advanceTimersByTimeAsync(500) }) // fail 2
    await act(async () => { await vi.advanceTimersByTimeAsync(500) }) // success → reset
    expect(result.current.unreachable).toBe(false)

    await act(async () => { await vi.advanceTimersByTimeAsync(500) }) // fail 1 again
    await act(async () => { await vi.advanceTimersByTimeAsync(500) }) // fail 2 again
    expect(result.current.unreachable).toBe(false)
  })

  it('unreachable wird nach einem erfolgreichen Poll wieder zurückgesetzt', async () => {
    vi.mocked(apiClient.getVehicleState)
      .mockRejectedValueOnce(new Error('x'))
      .mockRejectedValueOnce(new Error('x'))
      .mockRejectedValueOnce(new Error('x'))
      .mockResolvedValueOnce(CONNECTED)
    const { result } = renderHook(() => useSystemState('vehicle-001', 'tok'))

    await act(async () => { await vi.advanceTimersByTimeAsync(0) })
    await act(async () => { await vi.advanceTimersByTimeAsync(500) })
    await act(async () => { await vi.advanceTimersByTimeAsync(500) })
    expect(result.current.unreachable).toBe(true)

    await act(async () => { await vi.advanceTimersByTimeAsync(500) })
    expect(result.current.unreachable).toBe(false)
  })

  it('Wechsel der vehicleId pollt das neue Fahrzeug, nicht mehr das alte', async () => {
    vi.mocked(apiClient.getVehicleState).mockResolvedValue(CONNECTED)
    const { rerender } = renderHook(
      ({ id }: { id: string | null }) => useSystemState(id, 'tok'),
      { initialProps: { id: 'vehicle-001' } },
    )
    await act(async () => { await vi.advanceTimersByTimeAsync(0) })

    rerender({ id: 'vehicle-002' })
    await act(async () => { await vi.advanceTimersByTimeAsync(0) })

    expect(apiClient.getVehicleState).toHaveBeenLastCalledWith('vehicle-002')
  })

  it('Fehlerzähler wird beim Wechsel der vehicleId zurückgesetzt — kein "Bluten" zwischen Fahrzeugen', async () => {
    vi.mocked(apiClient.getVehicleState)
      .mockRejectedValueOnce(new Error('x')) // vehicle-001 fail 1
      .mockRejectedValueOnce(new Error('x')) // vehicle-001 fail 2
      .mockResolvedValueOnce(CONNECTED)      // vehicle-002 success — must NOT count as 3rd failure
    const { result, rerender } = renderHook(
      ({ id }: { id: string | null }) => useSystemState(id, 'tok'),
      { initialProps: { id: 'vehicle-001' } },
    )
    await act(async () => { await vi.advanceTimersByTimeAsync(0) })
    await act(async () => { await vi.advanceTimersByTimeAsync(500) })

    rerender({ id: 'vehicle-002' })
    await act(async () => { await vi.advanceTimersByTimeAsync(0) })

    expect(result.current.unreachable).toBe(false)
  })

  it('Wechsel von gesetzter vehicleId zu null setzt den State auf den Default zurück', async () => {
    vi.mocked(apiClient.getVehicleState).mockResolvedValue(CONNECTED)
    vi.mocked(apiClient.listSessions).mockResolvedValue([])
    const { result, rerender } = renderHook(
      ({ id }: { id: string | null }) => useSystemState(id, 'tok'),
      { initialProps: { id: 'vehicle-001' as string | null } },
    )
    await act(async () => { await vi.advanceTimersByTimeAsync(0) })
    expect(result.current.system).toBe('CONNECTED')

    rerender({ id: null })
    await act(async () => { await vi.advanceTimersByTimeAsync(0) })
    expect(result.current.system).toBe('IDLE')
  })

  it('beendet das Polling nach Unmount', async () => {
    vi.mocked(apiClient.getVehicleState).mockResolvedValue(CONNECTED)
    const { unmount } = renderHook(() => useSystemState('vehicle-001', 'tok'))
    await act(async () => { await vi.advanceTimersByTimeAsync(0) })
    const callsBefore = vi.mocked(apiClient.getVehicleState).mock.calls.length

    unmount()
    await act(async () => { await vi.advanceTimersByTimeAsync(2000) })

    expect(vi.mocked(apiClient.getVehicleState).mock.calls.length).toBe(callsBefore)
  })

  it('vehicleId null und listSessions schlägt 3x fehl → unreachable auch ohne Fahrzeug erkannt', async () => {
    vi.mocked(apiClient.listSessions).mockRejectedValue(new Error('down'))
    const { result } = renderHook(() => useSystemState(null, 'tok'))

    await act(async () => { await vi.advanceTimersByTimeAsync(0) })
    await act(async () => { await vi.advanceTimersByTimeAsync(500) })
    await act(async () => { await vi.advanceTimersByTimeAsync(500) })

    expect(result.current.unreachable).toBe(true)
  })
})
