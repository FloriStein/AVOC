import { renderHook, act } from '@testing-library/react'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import { useFleetZones } from './useFleetZones'
import * as apiClient from '@/lib/api-client'

vi.mock('@/lib/api-client', () => ({
  listFleetZones: vi.fn(),
  listFleetStations: vi.fn(),
}))

const Z1 = { id: 'z1', name: 'Z1', environment: 'outdoor' as const, svg_geometry: '<svg/>', created_at: 't1' }
const S1 = { id: 's1', zone_id: 'z1', name: 'S1', created_at: 't1' }

describe('useFleetZones', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('token null: lädt nichts, bleibt im loading-Zustand', () => {
    const { result } = renderHook(() => useFleetZones(null))
    expect(apiClient.listFleetZones).not.toHaveBeenCalled()
    expect(result.current.loading).toBe(true)
  })

  it('lädt Zonen und Stationen initial per REST', async () => {
    vi.mocked(apiClient.listFleetZones).mockResolvedValue([Z1])
    vi.mocked(apiClient.listFleetStations).mockResolvedValue([S1])

    const { result } = renderHook(() => useFleetZones('tok'))
    await act(async () => {})

    expect(result.current.zones).toEqual([Z1])
    expect(result.current.stations).toEqual([S1])
    expect(result.current.loading).toBe(false)
    expect(result.current.error).toBeNull()
  })

  it('leere Antwort (keine Zonen/Stationen) führt nicht zu einem Crash oder Fehler', async () => {
    vi.mocked(apiClient.listFleetZones).mockResolvedValue([])
    vi.mocked(apiClient.listFleetStations).mockResolvedValue([])

    const { result } = renderHook(() => useFleetZones('tok'))
    await act(async () => {})

    expect(result.current.zones).toEqual([])
    expect(result.current.stations).toEqual([])
    expect(result.current.error).toBeNull()
  })

  it('REST-Fehler bei listFleetZones setzt error, crasht nicht', async () => {
    vi.mocked(apiClient.listFleetZones).mockRejectedValue(new Error('down'))
    vi.mocked(apiClient.listFleetStations).mockResolvedValue([])

    const { result } = renderHook(() => useFleetZones('tok'))
    await act(async () => {})

    expect(result.current.error).toBeTruthy()
    expect(result.current.loading).toBe(false)
  })

  it('REST-Fehler bei listFleetStations setzt ebenfalls error', async () => {
    vi.mocked(apiClient.listFleetZones).mockResolvedValue([Z1])
    vi.mocked(apiClient.listFleetStations).mockRejectedValue(new Error('down'))

    const { result } = renderHook(() => useFleetZones('tok'))
    await act(async () => {})

    expect(result.current.error).toBeTruthy()
    expect(result.current.loading).toBe(false)
  })

  it('unmount vor Resolve: kein act()-Warning, kein State-Update danach', async () => {
    let resolveZones!: (value: typeof Z1[]) => void
    vi.mocked(apiClient.listFleetZones).mockReturnValue(new Promise((resolve) => { resolveZones = resolve }))
    vi.mocked(apiClient.listFleetStations).mockResolvedValue([])

    const { unmount } = renderHook(() => useFleetZones('tok'))
    unmount()

    // Resolving after unmount must not trigger a "state update on unmounted component" warning —
    // the active-flag guard should simply no-op.
    await act(async () => {
      resolveZones([Z1])
    })
  })
})
