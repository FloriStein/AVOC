import { renderHook, act } from '@testing-library/react'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import { useSession } from './useSession'
import * as apiClient from '@/lib/api-client'

// useSession owns the login/session-start/session-end/logout state machine (ADR-025/028).
// The underlying transport (WSClient's real connect/reconnect/close behavior) has its own
// dedicated test file (lib/ws-client.test.ts) — here WSClient is mocked so these tests focus
// purely on useSession's own state transitions and wiring, not on WebSocket internals.

vi.mock('@/lib/api-client', () => ({
  login: vi.fn(),
  logout: vi.fn(),
  startSession: vi.fn(),
  endSession: vi.fn(),
}))

vi.mock('@/lib/logger', () => ({
  logEvent: vi.fn(),
  FE_WS_RECONNECT: 'FE_WS_RECONNECT',
  FE_WS_CONNECTED: 'FE_WS_CONNECTED',
}))

vi.mock('@/lib/ws-client', () => {
  class MockWSClient {
    onOpen = null
    onClose = null
    onAck = null
    onAckError = null
    connect = vi.fn()
    disconnect = vi.fn()
    send = vi.fn()
    isOpen = vi.fn().mockReturnValue(false)
  }
  return { WSClient: MockWSClient }
})

describe('useSession', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
  })

  describe('connect (login)', () => {
    it('loggt ein, speichert Token/Operator in localStorage und State', async () => {
      vi.mocked(apiClient.login).mockResolvedValue('tok-123')
      const { result } = renderHook(() => useSession())

      await act(async () => { await result.current.connect('op1', 'pw') })

      expect(apiClient.login).toHaveBeenCalledWith('op1', 'pw')
      expect(result.current.token).toBe('tok-123')
      expect(result.current.operatorId).toBe('op1')
      expect(localStorage.getItem('avoc-token')).toBe('tok-123')
      expect(localStorage.getItem('avoc-operator')).toBe('op1')
    })

    it('setzt Session-State beim Login zurück (frischer Login nach vorheriger Session)', async () => {
      vi.mocked(apiClient.login).mockResolvedValue('tok-123')
      const { result } = renderHook(() => useSession())

      await act(async () => { await result.current.connect('op1', 'pw') })

      expect(result.current.sessionId).toBeNull()
      expect(result.current.vehicleId).toBeNull()
      expect(result.current.role).toBeNull()
    })

    it('wirft weiter, wenn login() fehlschlägt (Aufrufer zeigt Fehlermeldung)', async () => {
      vi.mocked(apiClient.login).mockRejectedValue(new Error('login failed: 401'))
      const { result } = renderHook(() => useSession())

      await expect(
        act(async () => { await result.current.connect('op1', 'wrong') }),
      ).rejects.toThrow('login failed: 401')
      expect(result.current.token).toBeNull()
    })
  })

  describe('startSession', () => {
    async function loggedIn() {
      vi.mocked(apiClient.login).mockResolvedValue('tok-123')
      const hook = renderHook(() => useSession())
      await act(async () => { await hook.result.current.connect('op1', 'pw') })
      return hook
    }

    it('startet eine Session als ACTIVE_OPERATOR und verbindet den WS mit der session_id', async () => {
      const { result } = await loggedIn()
      vi.mocked(apiClient.startSession).mockResolvedValue({
        session_id: 'sess-1', vehicle_id: 'veh-1', role: 'ACTIVE_OPERATOR',
      })

      await act(async () => { await result.current.startSession('veh-1') })

      expect(apiClient.startSession).toHaveBeenCalledWith('veh-1', 'op1', 'tok-123')
      expect(result.current.sessionId).toBe('sess-1')
      expect(result.current.vehicleId).toBe('veh-1')
      expect(result.current.role).toBe('ACTIVE_OPERATOR')
      expect(result.current.wsClient.connect).toHaveBeenCalledWith('tok-123', 'sess-1')
    })

    it('ADR-028: Fahrzeug bereits belegt → Rolle OBSERVER ("Beobachten") statt ACTIVE_OPERATOR', async () => {
      const { result } = await loggedIn()
      vi.mocked(apiClient.startSession).mockResolvedValue({
        session_id: 'sess-2', vehicle_id: 'veh-1', role: 'OBSERVER',
      })

      await act(async () => { await result.current.startSession('veh-1') })

      expect(result.current.role).toBe('OBSERVER')
      // WS still connects — observers receive the same telemetry/control-ack channel read-only.
      expect(result.current.wsClient.connect).toHaveBeenCalledWith('tok-123', 'sess-2')
    })

    it('ist ein No-Op ohne Token/Operator (nicht eingeloggt)', async () => {
      const { result } = renderHook(() => useSession())

      await act(async () => { await result.current.startSession('veh-1') })

      expect(apiClient.startSession).not.toHaveBeenCalled()
      expect(result.current.sessionId).toBeNull()
    })

    it('schluckt Fehler von startSession() still, damit der Aufrufer erneut versuchen kann', async () => {
      const { result } = await loggedIn()
      vi.mocked(apiClient.startSession).mockRejectedValue(new Error('vehicle offline'))

      await act(async () => { await result.current.startSession('veh-1') })

      expect(result.current.sessionId).toBeNull()
      expect(result.current.wsClient.connect).not.toHaveBeenCalled()
    })

    it('verkabelt onAck des WSClient, sodass ein ACK die Latenz aktualisiert', async () => {
      const { result } = await loggedIn()
      vi.mocked(apiClient.startSession).mockResolvedValue({
        session_id: 'sess-1', vehicle_id: 'veh-1', role: 'ACTIVE_OPERATOR',
      })

      await act(async () => { await result.current.startSession('veh-1') })
      act(() => { result.current.wsClient.onAck?.(42) })

      expect(result.current.latency).toBe(42)
    })
  })

  describe('endSession', () => {
    it('trennt den WS, ruft endSession() auf und setzt Session-State zurück', async () => {
      vi.mocked(apiClient.login).mockResolvedValue('tok-123')
      vi.mocked(apiClient.startSession).mockResolvedValue({
        session_id: 'sess-1', vehicle_id: 'veh-1', role: 'ACTIVE_OPERATOR',
      })
      vi.mocked(apiClient.endSession).mockResolvedValue(undefined)
      const { result } = renderHook(() => useSession())
      await act(async () => { await result.current.connect('op1', 'pw') })
      await act(async () => { await result.current.startSession('veh-1') })
      const wsClient = result.current.wsClient

      await act(async () => { await result.current.endSession() })

      expect(wsClient.disconnect).toHaveBeenCalled()
      expect(apiClient.endSession).toHaveBeenCalledWith('sess-1', 'tok-123')
      expect(result.current.sessionId).toBeNull()
      expect(result.current.vehicleId).toBeNull()
      expect(result.current.role).toBeNull()
      expect(result.current.latency).toBe(0)
      // Login/token survive — endSession only ends the vehicle session, not the operator login.
      expect(result.current.token).toBe('tok-123')
    })

    it('setzt State auch zurück, wenn der endSession()-Backend-Call fehlschlägt', async () => {
      vi.mocked(apiClient.login).mockResolvedValue('tok-123')
      vi.mocked(apiClient.startSession).mockResolvedValue({
        session_id: 'sess-1', vehicle_id: 'veh-1', role: 'ACTIVE_OPERATOR',
      })
      vi.mocked(apiClient.endSession).mockRejectedValue(new Error('network down'))
      const { result } = renderHook(() => useSession())
      await act(async () => { await result.current.connect('op1', 'pw') })
      await act(async () => { await result.current.startSession('veh-1') })

      await act(async () => { await result.current.endSession() })

      expect(result.current.sessionId).toBeNull()
    })
  })

  describe('disconnect (logout)', () => {
    it('loggt aus, löscht localStorage und setzt den gesamten State zurück', async () => {
      vi.mocked(apiClient.login).mockResolvedValue('tok-123')
      vi.mocked(apiClient.logout).mockResolvedValue(undefined)
      const { result } = renderHook(() => useSession())
      await act(async () => { await result.current.connect('op1', 'pw') })
      const wsClient = result.current.wsClient

      await act(async () => { await result.current.disconnect() })

      expect(apiClient.logout).toHaveBeenCalledWith('tok-123')
      expect(wsClient.disconnect).toHaveBeenCalled()
      expect(localStorage.getItem('avoc-token')).toBeNull()
      expect(localStorage.getItem('avoc-operator')).toBeNull()
      expect(result.current.token).toBeNull()
      expect(result.current.operatorId).toBeNull()
    })

    it('bricht ab und behält den State, wenn der Server 409/active_session meldet', async () => {
      vi.mocked(apiClient.login).mockResolvedValue('tok-123')
      vi.mocked(apiClient.logout).mockRejectedValue(new Error('active_session'))
      const { result } = renderHook(() => useSession())
      await act(async () => { await result.current.connect('op1', 'pw') })

      await act(async () => { await result.current.disconnect() })

      expect(result.current.token).toBe('tok-123')
      expect(localStorage.getItem('avoc-token')).toBe('tok-123')
    })

    it('ist unproblematisch ohne aktiven Token (bereits ausgeloggt)', async () => {
      const { result } = renderHook(() => useSession())

      await act(async () => { await result.current.disconnect() })

      expect(apiClient.logout).not.toHaveBeenCalled()
      expect(result.current.token).toBeNull()
    })
  })

  describe('restoreFromServerState', () => {
    it('übernimmt Session-Kontext direkt ohne Backend-Call (Page-Reload-Recovery)', () => {
      const { result } = renderHook(() => useSession())

      act(() => { result.current.restoreFromServerState('sess-9', 'veh-9', 'ACTIVE_OPERATOR') })

      expect(result.current.sessionId).toBe('sess-9')
      expect(result.current.vehicleId).toBe('veh-9')
      expect(result.current.role).toBe('ACTIVE_OPERATOR')
    })
  })

  describe('resume', () => {
    it('ist ein No-Op ohne bestehende Session (kein Token/sessionId)', async () => {
      const { result } = renderHook(() => useSession())

      await act(async () => { await result.current.resume() })

      expect(result.current.wsClient.connect).not.toHaveBeenCalled()
    })

    it('trennt und verbindet den WS erneut mit derselben session_id (SAFE_MODE-Recovery)', async () => {
      vi.mocked(apiClient.login).mockResolvedValue('tok-123')
      vi.mocked(apiClient.startSession).mockResolvedValue({
        session_id: 'sess-1', vehicle_id: 'veh-1', role: 'ACTIVE_OPERATOR',
      })
      const { result } = renderHook(() => useSession())
      await act(async () => { await result.current.connect('op1', 'pw') })
      await act(async () => { await result.current.startSession('veh-1') })
      const wsClient = result.current.wsClient
      vi.mocked(wsClient.connect).mockClear()

      await act(async () => { await result.current.resume() })

      expect(wsClient.disconnect).toHaveBeenCalled()
      expect(wsClient.connect).toHaveBeenCalledWith('tok-123', 'sess-1')
    })
  })

  describe('Cross-Tab-Sync', () => {
    it('räumt lokalen State auf, wenn ein anderer Tab den Token per storage-Event löscht', async () => {
      vi.mocked(apiClient.login).mockResolvedValue('tok-123')
      const { result } = renderHook(() => useSession())
      await act(async () => { await result.current.connect('op1', 'pw') })
      const wsClient = result.current.wsClient

      act(() => {
        window.dispatchEvent(new StorageEvent('storage', { key: 'avoc-token', newValue: null }))
      })

      expect(wsClient.disconnect).toHaveBeenCalled()
      expect(result.current.token).toBeNull()
      expect(result.current.operatorId).toBeNull()
    })

    it('ignoriert storage-Events für andere Keys', async () => {
      vi.mocked(apiClient.login).mockResolvedValue('tok-123')
      const { result } = renderHook(() => useSession())
      await act(async () => { await result.current.connect('op1', 'pw') })

      act(() => {
        window.dispatchEvent(new StorageEvent('storage', { key: 'unrelated-key', newValue: null }))
      })

      expect(result.current.token).toBe('tok-123')
    })
  })
})
