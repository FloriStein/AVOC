import { render, screen } from '@testing-library/react'
import { vi, describe, it, expect } from 'vitest'
import { VehicleSelector } from './VehicleSelector'
import { useVehicles } from '@/hooks/useVehicles'
import type { VehicleInfo } from '@/lib/api-client'

vi.mock('@/hooks/useVehicles')

const vehicle = (overrides: Partial<VehicleInfo> = {}): VehicleInfo => ({
  id: 'v1', display_name: 'Vehicle-001', description: '', online: true, system_state: 'CONNECTED',
  ...overrides,
})

describe('VehicleSelector', () => {
  it('zeigt kein SAFE_MODE-Badge für ein Fahrzeug im Normalzustand', () => {
    vi.mocked(useVehicles).mockReturnValue({ vehicles: [vehicle()], loading: false })
    render(<VehicleSelector onStartSession={vi.fn()} />)

    expect(screen.getByText(/🟢 Vehicle-001/)).toBeInTheDocument()
    expect(screen.queryByText(/SAFE_MODE/)).not.toBeInTheDocument()
  })

  it('zeigt ein SAFE_MODE-Badge (MV-09) für ein Fahrzeug in SAFE_MODE', () => {
    vi.mocked(useVehicles).mockReturnValue({
      vehicles: [vehicle({ id: 'v2', display_name: 'Vehicle-002', system_state: 'SAFE_MODE' })],
      loading: false,
    })
    render(<VehicleSelector onStartSession={vi.fn()} />)

    expect(screen.getByText(/🔴 Vehicle-002 — SAFE_MODE/)).toBeInTheDocument()
  })

  it('badged nur das betroffene Fahrzeug, nicht andere online-Fahrzeuge', () => {
    vi.mocked(useVehicles).mockReturnValue({
      vehicles: [
        vehicle({ id: 'v1', display_name: 'Vehicle-001', system_state: 'CONNECTED' }),
        vehicle({ id: 'v2', display_name: 'Vehicle-002', system_state: 'SAFE_MODE' }),
      ],
      loading: false,
    })
    render(<VehicleSelector onStartSession={vi.fn()} />)

    expect(screen.getByText(/🟢 Vehicle-001/)).toBeInTheDocument()
    expect(screen.getByText(/🔴 Vehicle-002 — SAFE_MODE/)).toBeInTheDocument()
  })
})
