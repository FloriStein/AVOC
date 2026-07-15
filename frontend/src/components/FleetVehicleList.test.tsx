import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { FleetVehicleList } from './FleetVehicleList'
import type { FleetVehicle, ActiveSession } from '@/lib/api-client'

const V1: FleetVehicle = { id: 'v1', display_name: 'V1', vehicle_type: 'lastenrad', battery_pct: 42, autonomy_mode: 'autonomous' }
// vehicle-001-Fall: real pre-existing vehicle with all fleet-service status fields nil.
const V_NO_DATA: FleetVehicle = { id: 'vehicle-001', display_name: 'Vehicle 001' }

describe('FleetVehicleList', () => {
  it('zeigt "Keine Fahrzeuge" bei leerer Liste', () => {
    render(<FleetVehicleList vehicles={[]} activeSessions={[]} selectedVehicleId={null} onSelect={vi.fn()} />)
    expect(screen.getByText(/keine fahrzeuge/i)).toBeInTheDocument()
  })

  it('rendert ein Fahrzeug mit Batterie-Badge', () => {
    render(<FleetVehicleList vehicles={[V1]} activeSessions={[]} selectedVehicleId={null} onSelect={vi.fn()} />)
    expect(screen.getByText('V1')).toBeInTheDocument()
    expect(screen.getByText('42%')).toBeInTheDocument()
  })

  it('rendert ein Fahrzeug ohne jegliche Statusdaten (vehicle-001-Fall) ohne Crash', () => {
    expect(() =>
      render(<FleetVehicleList vehicles={[V_NO_DATA]} activeSessions={[]} selectedVehicleId={null} onSelect={vi.fn()} />),
    ).not.toThrow()
    expect(screen.getByText('Vehicle 001')).toBeInTheDocument()
    expect(screen.queryByText(/NaN/)).not.toBeInTheDocument()
  })

  it('zeigt "Aktiv: <operator_id>" wenn ein ACTIVE_OPERATOR für das Fahrzeug existiert', () => {
    const session: ActiveSession = { session_id: 's1', vehicle_id: 'v1', operator_id: 'op1', role: 'ACTIVE_OPERATOR', created_at: 't1' }
    render(<FleetVehicleList vehicles={[V1]} activeSessions={[session]} selectedVehicleId={null} onSelect={vi.fn()} />)
    expect(screen.getByText(/Aktiv: op1/)).toBeInTheDocument()
  })

  it('zeigt kein Aktiv-Badge für eine OBSERVER-Session (nur ACTIVE_OPERATOR zählt)', () => {
    const session: ActiveSession = { session_id: 's1', vehicle_id: 'v1', operator_id: 'op1', role: 'OBSERVER', created_at: 't1' }
    render(<FleetVehicleList vehicles={[V1]} activeSessions={[session]} selectedVehicleId={null} onSelect={vi.fn()} />)
    expect(screen.queryByText(/Aktiv:/)).not.toBeInTheDocument()
  })

  it('ruft onSelect mit der vehicle id auf, wenn eine Zeile geklickt wird', () => {
    const onSelect = vi.fn()
    render(<FleetVehicleList vehicles={[V1]} activeSessions={[]} selectedVehicleId={null} onSelect={onSelect} />)
    fireEvent.click(screen.getByText('V1').closest('button')!)
    expect(onSelect).toHaveBeenCalledWith('v1')
  })
})
