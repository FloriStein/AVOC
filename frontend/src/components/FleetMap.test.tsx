import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { FleetMap } from './FleetMap'
import type { Zone, Station, FleetVehicle } from '@/lib/api-client'

// MAP-09 jsdom/Leaflet spike outcome: MapContainer mounts cleanly under the existing jsdom setup
// (no ResizeObserver polyfill needed, no console warnings) — spiked separately before writing
// this suite. All cases below, including marker click, run as real component tests, not deferred
// to browser-only verification.

const geoBounds = (sw: [number, number], ne: [number, number]) =>
  JSON.stringify({ sw: { lat: sw[0], lon: sw[1] }, ne: { lat: ne[0], lon: ne[1] } })

const validSvg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10"/></svg>'

const zone = (overrides: Partial<Zone> = {}): Zone => ({
  id: 'z1',
  name: 'Zone 1',
  environment: 'outdoor',
  svg_geometry: validSvg,
  geo_bounds: geoBounds([52.1295, 11.639], [52.1312, 11.6425]),
  created_at: 't',
  ...overrides,
})

const station = (overrides: Partial<Station> = {}): Station => ({
  id: 's1',
  zone_id: 'z1',
  name: 'Station 1',
  position_lat: 52.13,
  position_lon: 11.64,
  created_at: 't',
  ...overrides,
})

const vehicle = (overrides: Partial<FleetVehicle> = {}): FleetVehicle => ({
  id: 'v1',
  display_name: 'V1',
  ...overrides,
})

let warnSpy: ReturnType<typeof vi.spyOn>

beforeEach(() => {
  warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
})

afterEach(() => {
  warnSpy.mockRestore()
})

describe('FleetMap', () => {
  it('zeigt "Keine Zonen konfiguriert" bei leerer Zonenliste, ohne Crash', () => {
    expect(() =>
      render(<FleetMap zones={[]} stations={[]} vehicles={[]} selectedVehicleId={null} onSelectVehicle={vi.fn()} />),
    ).not.toThrow()
    expect(screen.getByText(/keine zonen konfiguriert/i)).toBeInTheDocument()
  })

  it('zeigt denselben Empty State, wenn keine Zone gültige geo_bounds hat', () => {
    render(
      <FleetMap
        zones={[zone({ geo_bounds: '{not valid json' })]}
        stations={[]}
        vehicles={[]}
        selectedVehicleId={null}
        onSelectVehicle={vi.fn()}
      />,
    )
    expect(screen.getByText(/keine zonen konfiguriert/i)).toBeInTheDocument()
  })

  it('überspringt nur das Overlay einer Zone mit kaputtem geo_bounds, Rest bleibt unbeeinträchtigt', () => {
    const zones = [zone({ id: 'bad', geo_bounds: '{not valid json' }), zone({ id: 'good' })]
    expect(() =>
      render(<FleetMap zones={zones} stations={[]} vehicles={[]} selectedVehicleId={null} onSelectVehicle={vi.fn()} />),
    ).not.toThrow()
    // The valid zone still renders a map instead of the empty state.
    expect(screen.queryByText(/keine zonen konfiguriert/i)).not.toBeInTheDocument()
  })

  it('überspringt das Overlay bei leerem svg_geometry, ohne zu crashen', () => {
    expect(() =>
      render(
        <FleetMap
          zones={[zone({ svg_geometry: '' })]}
          stations={[]}
          vehicles={[]}
          selectedVehicleId={null}
          onSelectVehicle={vi.fn()}
        />,
      ),
    ).not.toThrow()
    expect(warnSpy).toHaveBeenCalledWith(expect.stringContaining('svg_geometry'))
  })

  it('rendert einen Marker pro Station', () => {
    render(
      <FleetMap
        zones={[zone()]}
        stations={[station({ id: 's1', name: 'Ladezone A' }), station({ id: 's2', name: 'Ladezone B', position_lat: 52.131, position_lon: 11.641 })]}
        vehicles={[]}
        selectedVehicleId={null}
        onSelectVehicle={vi.fn()}
      />,
    )
    expect(screen.getByTitle('Ladezone A')).toBeInTheDocument()
    expect(screen.getByTitle('Ladezone B')).toBeInTheDocument()
  })

  it('rendert keinen Marker für ein Fahrzeug ohne Position, kein Crash', () => {
    expect(() =>
      render(
        <FleetMap
          zones={[zone()]}
          stations={[]}
          vehicles={[vehicle({ id: 'vehicle-001', display_name: 'Vehicle 001' })]}
          selectedVehicleId={null}
          onSelectVehicle={vi.fn()}
        />,
      ),
    ).not.toThrow()
    expect(screen.queryByTitle('Vehicle 001')).not.toBeInTheDocument()
  })

  it('Klick auf einen Fahrzeug-Marker ruft onSelectVehicle mit der richtigen id auf', () => {
    const onSelect = vi.fn()
    render(
      <FleetMap
        zones={[zone()]}
        stations={[]}
        vehicles={[vehicle({ id: 'v1', display_name: 'V1', position_lat: 52.13, position_lon: 11.64 })]}
        selectedVehicleId={null}
        onSelectVehicle={onSelect}
      />,
    )
    fireEvent.click(screen.getByTitle('V1'))
    expect(onSelect).toHaveBeenCalledWith('v1')
  })

  it('markiert das ausgewählte Fahrzeug visuell anders als ein unausgewähltes', () => {
    const vehicles = [
      vehicle({ id: 'v1', display_name: 'V1', position_lat: 52.13, position_lon: 11.64 }),
      vehicle({ id: 'v2', display_name: 'V2', position_lat: 52.131, position_lon: 11.641 }),
    ]
    render(<FleetMap zones={[zone()]} stations={[]} vehicles={vehicles} selectedVehicleId="v1" onSelectVehicle={vi.fn()} />)
    expect(screen.getByTitle('V1').className).toMatch(/ring-2/)
    expect(screen.getByTitle('V2').className).not.toMatch(/ring-2/)
  })
})
