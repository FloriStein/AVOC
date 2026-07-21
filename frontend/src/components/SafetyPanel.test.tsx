import { render, screen, fireEvent } from '@testing-library/react'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import { SafetyPanel } from './SafetyPanel'
import { emergencyStop } from '@/lib/api-client'

// Mock api-client to prevent real HTTP calls
vi.mock('@/lib/api-client', () => ({
  emergencyStop: vi.fn().mockResolvedValue(undefined),
}))

// Mock useDeadmanSwitch — no WebSocket in unit tests
vi.mock('@/hooks/useDeadmanSwitch', () => ({
  useDeadmanSwitch: vi.fn(() => ({
    isActive: false,
    buttonProps: {
      onMouseDown: vi.fn(),
      onMouseUp: vi.fn(),
      onMouseLeave: vi.fn(),
    },
  })),
}))

describe('SafetyPanel', () => {
  beforeEach(() => { vi.clearAllMocks() })

  it('zeigt Emergency Stop Button', () => {
    render(<SafetyPanel systemState="CONNECTED" sessionId="sess-1" vehicleId={null} operatorId={null} wsClient={null} token={null} />)
    expect(screen.getByRole('button', { name: /emergency stop/i })).toBeInTheDocument()
  })

  it('Emergency Stop Button ist disabled wenn SAFE_MODE', () => {
    render(<SafetyPanel systemState="SAFE_MODE" sessionId="sess-1" vehicleId={null} operatorId={null} wsClient={null} token={null} />)
    expect(screen.getByRole('button', { name: /emergency stop/i })).toBeDisabled()
  })

  it('Emergency Stop Button ist disabled wenn nicht CONNECTED', () => {
    render(<SafetyPanel systemState="IDLE" sessionId={null} vehicleId={null} operatorId={null} wsClient={null} token={null} />)
    expect(screen.getByRole('button', { name: /emergency stop/i })).toBeDisabled()
  })

  it('Emergency Stop Button ist aktiv wenn CONNECTED', () => {
    render(<SafetyPanel systemState="CONNECTED" sessionId="sess-1" vehicleId={null} operatorId={null} wsClient={null} token={null} />)
    expect(screen.getByRole('button', { name: /emergency stop/i })).not.toBeDisabled()
  })

  it('Dead-man Switch Button ist sichtbar', () => {
    render(<SafetyPanel systemState="CONNECTED" sessionId="sess-1" vehicleId={null} operatorId={null} wsClient={null} token={null} />)
    expect(screen.getByText(/Hold|Spacebar|HOLD/i)).toBeInTheDocument()
  })

  it('zeigt DEGRADED-Warnung wenn DEGRADED', () => {
    render(<SafetyPanel systemState="DEGRADED" sessionId="sess-1" vehicleId={null} operatorId={null} wsClient={null} token={null} />)
    expect(screen.getByText(/DEGRADED/i)).toBeInTheDocument()
  })

  describe('Emergency Stop — echter Klick', () => {
    it('ruft emergencyStop() mit session/vehicle/token auf, wenn der Button geklickt wird', () => {
      render(
        <SafetyPanel
          systemState="CONNECTED"
          sessionId="sess-1"
          vehicleId="veh-1"
          operatorId="op-1"
          wsClient={null}
          token="tok-123"
        />,
      )

      fireEvent.click(screen.getByRole('button', { name: /emergency stop/i }))

      expect(emergencyStop).toHaveBeenCalledTimes(1)
      expect(emergencyStop).toHaveBeenCalledWith('sess-1', 'veh-1', 'tok-123')
    })

    it('ruft emergencyStop() NICHT auf, wenn kein Token vorhanden ist (Guard greift trotz aktivem Button)', () => {
      render(
        <SafetyPanel
          systemState="CONNECTED"
          sessionId="sess-1"
          vehicleId="veh-1"
          operatorId="op-1"
          wsClient={null}
          token={null}
        />,
      )

      const button = screen.getByRole('button', { name: /emergency stop/i })
      expect(button).not.toBeDisabled()
      fireEvent.click(button)

      expect(emergencyStop).not.toHaveBeenCalled()
    })

    it('Button ist nach dem Klick disabled, sobald State-Polling SAFE_MODE zurückmeldet', () => {
      const { rerender } = render(
        <SafetyPanel
          systemState="CONNECTED"
          sessionId="sess-1"
          vehicleId="veh-1"
          operatorId="op-1"
          wsClient={null}
          token="tok-123"
        />,
      )
      const button = screen.getByRole('button', { name: /emergency stop/i })
      expect(button).not.toBeDisabled()

      fireEvent.click(button)
      expect(emergencyStop).toHaveBeenCalledTimes(1)

      // Server transitions to SAFE_MODE as a result of the Emergency Stop; App polls system
      // state and passes the new systemState down as a prop re-render.
      rerender(
        <SafetyPanel
          systemState="SAFE_MODE"
          sessionId="sess-1"
          vehicleId="veh-1"
          operatorId="op-1"
          wsClient={null}
          token="tok-123"
        />,
      )

      expect(screen.getByRole('button', { name: /emergency stop/i })).toBeDisabled()
    })
  })
})
