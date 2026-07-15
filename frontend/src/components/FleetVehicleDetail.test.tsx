import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { FleetVehicleDetail } from './FleetVehicleDetail'
import type { FleetVehicle } from '@/lib/api-client'

const V1: FleetVehicle = { id: 'v1', display_name: 'V1', vehicle_type: 'lastenrad', battery_pct: 42, speed: 5.5, autonomy_mode: 'autonomous', position_zone_id: 'zone-a' }
const V_NO_DATA: FleetVehicle = { id: 'vehicle-001', display_name: 'Vehicle 001' }

describe('FleetVehicleDetail', () => {
  it('zeigt Platzhalter, wenn kein Fahrzeug ausgewählt ist', () => {
    render(
      <FleetVehicleDetail
        vehicle={null}
        hasActiveOperator={false}
        activeOperatorId={null}
        isObserverRole={false}
        onTeleoperate={vi.fn()}
        onObserve={vi.fn()}
      />,
    )
    expect(screen.getByText(/fahrzeug auswählen/i)).toBeInTheDocument()
  })

  it('rendert Felder mit — für undefined-Werte, statt zu crashen (vehicle-001-Fall)', () => {
    expect(() =>
      render(
        <FleetVehicleDetail
          vehicle={V_NO_DATA}
          hasActiveOperator={false}
          activeOperatorId={null}
          isObserverRole={false}
          onTeleoperate={vi.fn()}
          onObserve={vi.fn()}
        />,
      ),
    ).not.toThrow()
    expect(screen.queryByText(/NaN/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Invalid Date/)).not.toBeInTheDocument()
  })

  it('Teleoperate-Button ist ABWESEND, wenn das Fahrzeug bereits einen aktiven Operator hat (ADR-028)', () => {
    render(
      <FleetVehicleDetail
        vehicle={V1}
        hasActiveOperator={true}
        activeOperatorId="op1"
        isObserverRole={false}
        onTeleoperate={vi.fn()}
        onObserve={vi.fn()}
      />,
    )
    expect(screen.queryByRole('button', { name: /teleoperate/i })).not.toBeInTheDocument()
    expect(screen.getByText(/aktiver operator: op1/i)).toBeInTheDocument()
  })

  it('zeigt stattdessen einen Beobachten-Button, wenn ein aktiver Operator existiert', () => {
    const onObserve = vi.fn()
    render(
      <FleetVehicleDetail
        vehicle={V1}
        hasActiveOperator={true}
        activeOperatorId="op1"
        isObserverRole={false}
        onTeleoperate={vi.fn()}
        onObserve={onObserve}
      />,
    )
    fireEvent.click(screen.getByRole('button', { name: /beobachten/i }))
    expect(onObserve).toHaveBeenCalledWith('v1')
  })

  it('Teleoperate-Button ist aktiv, wenn kein aktiver Operator existiert', () => {
    const onTeleoperate = vi.fn()
    render(
      <FleetVehicleDetail
        vehicle={V1}
        hasActiveOperator={false}
        activeOperatorId={null}
        isObserverRole={false}
        onTeleoperate={onTeleoperate}
        onObserve={vi.fn()}
      />,
    )
    const btn = screen.getByRole('button', { name: /teleoperate/i })
    expect(btn).not.toBeDisabled()
    fireEvent.click(btn)
    expect(onTeleoperate).toHaveBeenCalledWith('v1')
  })

  it('kein Teleoperate-Button für eine OBSERVER-Rolle, wenn kein aktiver Operator existiert', () => {
    render(
      <FleetVehicleDetail
        vehicle={V1}
        hasActiveOperator={false}
        activeOperatorId={null}
        isObserverRole={true}
        onTeleoperate={vi.fn()}
        onObserve={vi.fn()}
      />,
    )
    expect(screen.queryByRole('button', { name: /teleoperate/i })).not.toBeInTheDocument()
  })
})
