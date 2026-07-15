import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import App from './App'
import { useSession } from '@/hooks/useSession'
import { useSystemState } from '@/hooks/useSystemState'
import { useTelemetry } from '@/hooks/useTelemetry'
import { useVehicleAck } from '@/hooks/useVehicleAck'
import { useActiveSessions } from '@/hooks/useActiveSessions'

// Regression guard for the DASH-06 routing decision: App.tsx has no router, so which top-level
// view renders is purely a function of session.token/session.sessionId. Everything else
// (AppContent's own cockpit internals, FleetOverview's own internals) has its own dedicated
// test coverage — this file only asserts the routing itself.

vi.mock('@/hooks/useSession')
vi.mock('@/hooks/useSystemState')
vi.mock('@/hooks/useTelemetry')
vi.mock('@/hooks/useVehicleAck')
vi.mock('@/hooks/useActiveSessions')
vi.mock('@/components/FleetOverview', () => ({
  FleetOverview: () => <div>FLEET_OVERVIEW_MARKER</div>,
}))

const OPERATOR_TOKEN = 'x.' + btoa(JSON.stringify({ role: 'ACTIVE_OPERATOR' })) + '.x'

function mockSession(overrides: Partial<ReturnType<typeof useSession>>) {
  vi.mocked(useSession).mockReturnValue({
    token: null,
    operatorId: null,
    sessionId: null,
    vehicleId: null,
    role: null,
    latency: 0,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    wsClient: {} as any,
    connect: vi.fn(),
    resume: vi.fn(),
    disconnect: vi.fn(),
    startSession: vi.fn(),
    endSession: vi.fn(),
    restoreFromServerState: vi.fn(),
    ...overrides,
  })
}

describe('App routing (DASH-06)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useSystemState).mockReturnValue({ system: 'IDLE', control: 'CONTROL_INIT', media: 'MEDIA_INIT', operator: 'NO_OPERATOR', unreachable: false })
    vi.mocked(useTelemetry).mockReturnValue(null)
    vi.mocked(useVehicleAck).mockReturnValue(null)
    vi.mocked(useActiveSessions).mockReturnValue({ activeSessions: [], hasPolled: true })
  })

  it('kein Token: rendert LoginPanel', () => {
    mockSession({ token: null })
    render(<App />)
    expect(screen.getByRole('button', { name: /anmelden|login/i })).toBeInTheDocument()
  })

  it('Token gesetzt, keine sessionId: rendert FleetOverview, nicht das Cockpit', () => {
    mockSession({ token: OPERATOR_TOKEN, sessionId: null })
    render(<App />)
    expect(screen.getByText('FLEET_OVERVIEW_MARKER')).toBeInTheDocument()
    expect(screen.queryByText(/Teleoperation Control Center/i)).not.toBeInTheDocument()
  })

  it('sessionId gesetzt: rendert das bestehende Cockpit, nicht FleetOverview', () => {
    mockSession({ token: OPERATOR_TOKEN, sessionId: 'sess-1', vehicleId: 'v1', role: 'ACTIVE_OPERATOR' })
    render(<App />)
    expect(screen.getByText(/Teleoperation Control Center/i)).toBeInTheDocument()
    expect(screen.queryByText('FLEET_OVERVIEW_MARKER')).not.toBeInTheDocument()
  })
})
