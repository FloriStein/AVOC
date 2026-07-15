import { renderHook, act } from '@testing-library/react'
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import { useActiveSessions } from './useActiveSessions'
import * as apiClient from '@/lib/api-client'

vi.mock('@/lib/api-client', () => ({
  listSessions: vi.fn(),
}))

const SESSION = { session_id: 's1', vehicle_id: 'v1', operator_id: 'op1', role: 'ACTIVE_OPERATOR', created_at: 't1' }

describe('useActiveSessions', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('token null: pollt nicht', async () => {
    const { result } = renderHook(() => useActiveSessions(null))
    await act(async () => { await vi.advanceTimersByTimeAsync(3000) })

    expect(apiClient.listSessions).not.toHaveBeenCalled()
    expect(result.current.activeSessions).toEqual([])
    expect(result.current.hasPolled).toBe(false)
  })

  it('lädt die Session-Liste und setzt hasPolled', async () => {
    vi.mocked(apiClient.listSessions).mockResolvedValue([SESSION])
    const { result } = renderHook(() => useActiveSessions('tok'))

    await act(async () => { await vi.advanceTimersByTimeAsync(0) })

    expect(result.current.activeSessions).toEqual([SESSION])
    expect(result.current.hasPolled).toBe(true)
  })

  it('Fehler beim Fetch: behält die letzte bekannte Liste, hasPolled wird trotzdem true', async () => {
    vi.mocked(apiClient.listSessions).mockResolvedValueOnce([SESSION]).mockRejectedValue(new Error('down'))
    const { result } = renderHook(() => useActiveSessions('tok'))

    await act(async () => { await vi.advanceTimersByTimeAsync(0) })
    expect(result.current.activeSessions).toEqual([SESSION])

    await act(async () => { await vi.advanceTimersByTimeAsync(3000) })
    expect(result.current.activeSessions).toEqual([SESSION]) // unchanged, not cleared
    expect(result.current.hasPolled).toBe(true)
  })

  it('pollt alle 3000ms erneut', async () => {
    vi.mocked(apiClient.listSessions).mockResolvedValue([])
    renderHook(() => useActiveSessions('tok'))

    await act(async () => { await vi.advanceTimersByTimeAsync(0) })
    await act(async () => { await vi.advanceTimersByTimeAsync(3000) })
    await act(async () => { await vi.advanceTimersByTimeAsync(3000) })

    expect(apiClient.listSessions).toHaveBeenCalledTimes(3)
  })

  it('beendet das Polling nach Unmount', async () => {
    vi.mocked(apiClient.listSessions).mockResolvedValue([])
    const { unmount } = renderHook(() => useActiveSessions('tok'))
    await act(async () => { await vi.advanceTimersByTimeAsync(0) })
    const callsBefore = vi.mocked(apiClient.listSessions).mock.calls.length

    unmount()
    await act(async () => { await vi.advanceTimersByTimeAsync(9000) })

    expect(vi.mocked(apiClient.listSessions).mock.calls.length).toBe(callsBefore)
  })
})
