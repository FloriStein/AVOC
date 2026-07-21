import { renderHook, act } from '@testing-library/react'
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import { useControls } from './useControls'
import type { WSClient } from '@/lib/ws-client'

// useControls' 20Hz command loop + keyboard wiring (lines 66-191) is the direct steering/
// throttle input path — safety-relevant because a stuck "held key" or a missing neutral-reset
// on release would keep commanding movement after the operator lets go. `@/gen/control_pb.js`
// is not generated in this test env, so ControlCommandSchema stays undefined and sendCmd always
// takes its documented fallback path (raw `Uint8Array([0x01])`) — this is exercised as-is rather
// than mocked away, since it is real production behavior before `gen/` exists.

function makeWsClient(): WSClient {
  return { send: vi.fn(), isOpen: vi.fn().mockReturnValue(true) } as unknown as WSClient
}

const FALLBACK_BYTE = new Uint8Array([0x01])

describe('useControls', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    Object.defineProperty(window.navigator, 'getGamepads', {
      value: vi.fn().mockReturnValue([]),
      configurable: true,
    })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  function pressKey(code: string) {
    window.dispatchEvent(new KeyboardEvent('keydown', { code, cancelable: true }))
  }
  function releaseKey(code: string) {
    window.dispatchEvent(new KeyboardEvent('keyup', { code }))
  }

  describe('Keyboard-Wiring', () => {
    it('sendet STEER-Kommandos, solange eine Lenktaste gehalten wird', () => {
      const wsClient = makeWsClient()
      renderHook(() => useControls(wsClient, 'sess-1', 'veh-1', 'op-1', true, 1))

      act(() => { pressKey('KeyD') })
      act(() => { vi.advanceTimersByTime(50) })

      expect(wsClient.send).toHaveBeenCalledWith(FALLBACK_BYTE)
    })

    it('setzt activeMode auf "keyboard" und steer/throttle auf die erwarteten Vorzeichen', () => {
      const wsClient = makeWsClient()
      const { result } = renderHook(() => useControls(wsClient, 'sess-1', 'veh-1', 'op-1', true, 1))

      act(() => { pressKey('KeyW') })
      act(() => { vi.advanceTimersByTime(50) })

      expect(result.current.activeMode).toBe('keyboard')
      expect(result.current.throttle).toBe(1)
      expect(result.current.steer).toBe(0)
    })

    it('skaliert Steer/Throttle mit dem speedMultiplier', () => {
      const wsClient = makeWsClient()
      const { result } = renderHook(() => useControls(wsClient, 'sess-1', 'veh-1', 'op-1', true, 0.5))

      act(() => { pressKey('KeyA') })
      act(() => { vi.advanceTimersByTime(50) })

      expect(result.current.steer).toBe(-0.5)
    })

    it('sendet nach dem Loslassen genau einmal ein Neutral-Kommando (STEER=0/THROTTLE=0), nicht auf jedem weiteren Tick', () => {
      const wsClient = makeWsClient()
      const { result } = renderHook(() => useControls(wsClient, 'sess-1', 'veh-1', 'op-1', true, 1))

      act(() => { pressKey('KeyD') })
      act(() => { vi.advanceTimersByTime(50) })
      const callsWhileHeld = vi.mocked(wsClient.send).mock.calls.length

      act(() => { releaseKey('KeyD') })
      act(() => { vi.advanceTimersByTime(50) }) // transition tick: sends neutral once
      const callsAfterRelease = vi.mocked(wsClient.send).mock.calls.length
      expect(callsAfterRelease).toBeGreaterThan(callsWhileHeld)
      expect(result.current.activeMode).toBe('none')
      expect(result.current.steer).toBe(0)

      act(() => { vi.advanceTimersByTime(50) }) // steady-state neutral: no further sends
      expect(vi.mocked(wsClient.send).mock.calls.length).toBe(callsAfterRelease)
    })

    it('ignoriert Tasteneingaben, während der Fokus in einem Eingabefeld liegt', () => {
      const input = document.createElement('input')
      document.body.appendChild(input)
      input.focus()

      const wsClient = makeWsClient()
      const { result } = renderHook(() => useControls(wsClient, 'sess-1', 'veh-1', 'op-1', true, 1))

      act(() => { pressKey('KeyW') })
      act(() => { vi.advanceTimersByTime(50) })

      expect(result.current.activeMode).toBe('none')
      document.body.removeChild(input)
    })

    it('ignoriert Tasten, die nicht in MOVE_KEYS enthalten sind', () => {
      const wsClient = makeWsClient()
      const { result } = renderHook(() => useControls(wsClient, 'sess-1', 'veh-1', 'op-1', true, 1))

      act(() => { pressKey('KeyQ') })
      act(() => { vi.advanceTimersByTime(50) })

      expect(result.current.activeMode).toBe('none')
    })

    it('entfernt die Keyboard-Listener beim Unmount', () => {
      const wsClient = makeWsClient()
      const { unmount } = renderHook(() => useControls(wsClient, 'sess-1', 'veh-1', 'op-1', true, 1))
      unmount()

      vi.mocked(wsClient.send).mockClear()
      act(() => { pressKey('KeyW') })
      act(() => { vi.advanceTimersByTime(50) })

      expect(wsClient.send).not.toHaveBeenCalled()
    })
  })

  describe('enabled=false (z.B. SAFE_MODE)', () => {
    it('sendet keine Kommandos und hält activeMode auf "none"', () => {
      const wsClient = makeWsClient()
      const { result } = renderHook(() => useControls(wsClient, 'sess-1', 'veh-1', 'op-1', false, 1))

      act(() => { pressKey('KeyW') })
      act(() => { vi.advanceTimersByTime(50) })

      expect(wsClient.send).not.toHaveBeenCalled()
      expect(result.current.activeMode).toBe('none')
      expect(result.current.steer).toBe(0)
      expect(result.current.throttle).toBe(0)
    })

    it('setzt den virtuellen Joystick zurück, wenn während aktivem Halten disabled wird', () => {
      const wsClient = makeWsClient()
      const { result, rerender } = renderHook(
        ({ enabled }) => useControls(wsClient, 'sess-1', 'veh-1', 'op-1', enabled, 1),
        { initialProps: { enabled: true } },
      )

      act(() => { result.current.setJoystick(0.5, 0.5, true) })
      expect(result.current.joyPos).toEqual({ x: 0.5, y: 0.5 })

      rerender({ enabled: false })

      expect(result.current.joyPos).toEqual({ x: 0, y: 0 })
    })
  })

  describe('Virtueller Joystick (Priorität vor Keyboard)', () => {
    it('überschreibt gehaltene Keyboard-Eingaben, solange der Joystick aktiv ist', () => {
      const wsClient = makeWsClient()
      const { result } = renderHook(() => useControls(wsClient, 'sess-1', 'veh-1', 'op-1', true, 1))

      act(() => { pressKey('KeyW') })
      act(() => { result.current.setJoystick(0.3, -0.2, true) })
      act(() => { vi.advanceTimersByTime(50) })

      expect(result.current.activeMode).toBe('joystick')
      expect(result.current.steer).toBeCloseTo(0.3)
      expect(result.current.throttle).toBeCloseTo(-0.2)
    })
  })

  describe('Gamepad (höchste Priorität)', () => {
    it('überschreibt Keyboard/Joystick, wenn ein Gamepad verbunden ist', () => {
      const gamepad = {
        connected: true,
        axes: [0.8, -0.5],
        buttons: Array.from({ length: 7 }, () => ({ value: 0 })),
      } as unknown as Gamepad
      vi.mocked(window.navigator.getGamepads).mockReturnValue([gamepad])

      const wsClient = makeWsClient()
      const { result } = renderHook(() => useControls(wsClient, 'sess-1', 'veh-1', 'op-1', true, 1))

      act(() => { pressKey('KeyW') })
      act(() => { vi.advanceTimersByTime(50) })

      expect(result.current.activeMode).toBe('gamepad')
      expect(result.current.gamepadConnected).toBe(true)
    })
  })
})
