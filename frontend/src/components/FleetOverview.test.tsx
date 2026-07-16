import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { FleetOverview } from './FleetOverview'
import { useFleetOverview } from '@/hooks/useFleetOverview'
import { useActiveSessions } from '@/hooks/useActiveSessions'
import type { SessionState } from '@/hooks/useSession'
import type { FleetVehicle } from '@/lib/api-client'

vi.mock('@/hooks/useFleetOverview')
vi.mock('@/hooks/useActiveSessions')

// Minimal valid JWT with role claim, base64url-decodable by parseTokenRole (atob(payload)).
const OBSERVER_TOKEN = 'x.' + btoa(JSON.stringify({ role: 'OBSERVER' })) + '.x'
const OPERATOR_TOKEN = 'x.' + btoa(JSON.stringify({ role: 'ACTIVE_OPERATOR' })) + '.x'

function makeSession(overrides: Partial<SessionState> = {}): SessionState {
  return {
    token: OPERATOR_TOKEN,
    operatorId: 'op1',
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
  }
}

const V1: FleetVehicle = { id: 'v1', display_name: 'V1', autonomy_mode: 'autonomous' }

describe('FleetOverview', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useActiveSessions).mockReturnValue({ activeSessions: [], hasPolled: true })
  })

  it('zeigt einen Lade-Zustand', () => {
    vi.mocked(useFleetOverview).mockReturnValue({ vehicles: [], alerts: [], tasks: [], loading: true, error: null, acknowledgeAlert: vi.fn(), createTask: vi.fn(), updateTaskStatus: vi.fn() })
    render(<FleetOverview session={makeSession()} />)
    expect(screen.getByText(/lädt flottendaten/i)).toBeInTheDocument()
  })

  it('zeigt einen Fehlerzustand', () => {
    vi.mocked(useFleetOverview).mockReturnValue({ vehicles: [], alerts: [], tasks: [], loading: false, error: 'kaputt', acknowledgeAlert: vi.fn(), createTask: vi.fn(), updateTaskStatus: vi.fn() })
    render(<FleetOverview session={makeSession()} />)
    expect(screen.getByText('kaputt')).toBeInTheDocument()
  })

  it('Fahrzeug auswählen aktualisiert das Detail-Panel', () => {
    vi.mocked(useFleetOverview).mockReturnValue({ vehicles: [V1], alerts: [], tasks: [], loading: false, error: null, acknowledgeAlert: vi.fn(), createTask: vi.fn(), updateTaskStatus: vi.fn() })
    render(<FleetOverview session={makeSession()} />)

    expect(screen.getByText(/fahrzeug auswählen/i)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /V1/i }))
    // FleetVehicleDetail now renders the vehicle's own heading instead of the placeholder.
    expect(screen.queryByText(/fahrzeug auswählen/i)).not.toBeInTheDocument()
  })

  it('leitet hasActiveOperator korrekt aus activeSessions für das ausgewählte Fahrzeug ab', () => {
    vi.mocked(useFleetOverview).mockReturnValue({ vehicles: [V1], alerts: [], tasks: [], loading: false, error: null, acknowledgeAlert: vi.fn(), createTask: vi.fn(), updateTaskStatus: vi.fn() })
    vi.mocked(useActiveSessions).mockReturnValue({
      activeSessions: [{ session_id: 's1', vehicle_id: 'v1', operator_id: 'op2', role: 'ACTIVE_OPERATOR', created_at: 't1' }],
      hasPolled: true,
    })
    render(<FleetOverview session={makeSession()} />)

    fireEvent.click(screen.getByRole('button', { name: /V1/i }))

    expect(screen.getByText(/aktiver operator: op2/i)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /teleoperate/i })).not.toBeInTheDocument()
  })

  it('leitet isObserverRole korrekt aus dem Token ab (OBSERVER sieht keinen Teleoperate-Button)', () => {
    vi.mocked(useFleetOverview).mockReturnValue({ vehicles: [V1], alerts: [], tasks: [], loading: false, error: null, acknowledgeAlert: vi.fn(), createTask: vi.fn(), updateTaskStatus: vi.fn() })
    render(<FleetOverview session={makeSession({ token: OBSERVER_TOKEN })} />)

    fireEvent.click(screen.getByRole('button', { name: /V1/i }))

    expect(screen.queryByRole('button', { name: /teleoperate/i })).not.toBeInTheDocument()
  })

  it('Abmelden-Button ruft session.disconnect auf', () => {
    vi.mocked(useFleetOverview).mockReturnValue({ vehicles: [], alerts: [], tasks: [], loading: false, error: null, acknowledgeAlert: vi.fn(), createTask: vi.fn(), updateTaskStatus: vi.fn() })
    const session = makeSession()
    render(<FleetOverview session={session} />)

    fireEvent.click(screen.getByRole('button', { name: /abmelden/i }))
    expect(session.disconnect).toHaveBeenCalled()
  })
})
