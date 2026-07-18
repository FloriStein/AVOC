import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { FleetIndoorMap } from './FleetIndoorMap'
import type { Zone, Station, FleetVehicle } from '@/lib/api-client'

const validSvg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 300"><rect width="400" height="300"/></svg>'

const zone = (overrides: Partial<Zone> = {}): Zone => ({
  id: 'z1',
  name: 'Lager',
  environment: 'indoor',
  svg_geometry: validSvg,
  created_at: 't',
  ...overrides,
})

const station = (overrides: Partial<Station> = {}): Station => ({
  id: 's1',
  zone_id: 'z1',
  name: 'Regal A',
  position_x: 50,
  position_y: 60,
  created_at: 't',
  ...overrides,
})

const vehicle = (overrides: Partial<FleetVehicle> = {}): FleetVehicle => ({
  id: 'v1',
  display_name: 'V1',
  ...overrides,
})

// screen.getByTitle only matches a <title> that is a DIRECT child of <svg> (see
// @testing-library/dom's title query) — markers here nest <title> inside <circle> (the standard
// SVG pattern for a shape-level tooltip, kept for real-browser UX), so tests look the marker up
// via its <title> text content and the enclosing <circle> instead.
function marker(container: HTMLElement, name: string): Element | null {
  const title = Array.from(container.querySelectorAll('circle > title')).find((t) => t.textContent === name)
  return title?.parentElement ?? null
}

describe('FleetIndoorMap', () => {
  it('rendert nichts, wenn keine Indoor-Zone existiert', () => {
    const { container } = render(
      <FleetIndoorMap
        zones={[zone({ environment: 'outdoor' })]}
        stations={[]}
        vehicles={[]}
        selectedVehicleId={null}
        onSelectVehicle={vi.fn()}
      />,
    )
    expect(container).toBeEmptyDOMElement()
  })

  it('zeigt den Zonennamen und einen Hinweis bei fehlender Kartengeometrie', () => {
    render(
      <FleetIndoorMap
        zones={[zone({ svg_geometry: '' })]}
        stations={[]}
        vehicles={[]}
        selectedVehicleId={null}
        onSelectVehicle={vi.fn()}
      />,
    )
    expect(screen.getByText('Lager')).toBeInTheDocument()
    expect(screen.getByText(/keine kartengeometrie hinterlegt/i)).toBeInTheDocument()
  })

  it('rendert einen Marker pro Station innerhalb der Zone, keinen für eine andere Zone', () => {
    const { container } = render(
      <FleetIndoorMap
        zones={[zone()]}
        stations={[station({ id: 's1', name: 'Regal A' }), station({ id: 's2', zone_id: 'other-zone', name: 'Fremdstation' })]}
        vehicles={[]}
        selectedVehicleId={null}
        onSelectVehicle={vi.fn()}
      />,
    )
    expect(marker(container, 'Regal A')).not.toBeNull()
    expect(marker(container, 'Fremdstation')).toBeNull()
  })

  it('rendert keinen Marker für ein Fahrzeug ohne position_zone_id/position_x/y', () => {
    const { container } = render(
      <FleetIndoorMap
        zones={[zone()]}
        stations={[]}
        vehicles={[vehicle({ id: 'v1', display_name: 'Ohne Position' })]}
        selectedVehicleId={null}
        onSelectVehicle={vi.fn()}
      />,
    )
    expect(marker(container, 'Ohne Position')).toBeNull()
  })

  it('Klick auf einen Fahrzeug-Marker ruft onSelectVehicle mit der richtigen id auf', () => {
    const onSelect = vi.fn()
    const { container } = render(
      <FleetIndoorMap
        zones={[zone()]}
        stations={[]}
        vehicles={[vehicle({ id: 'v1', display_name: 'V1', position_zone_id: 'z1', position_x: 30, position_y: 40 })]}
        selectedVehicleId={null}
        onSelectVehicle={onSelect}
      />,
    )
    fireEvent.click(marker(container, 'V1')!)
    expect(onSelect).toHaveBeenCalledWith('v1')
  })

  it('markiert das ausgewählte Fahrzeug visuell anders als ein unausgewähltes', () => {
    const vehicles = [
      vehicle({ id: 'v1', display_name: 'V1', position_zone_id: 'z1', position_x: 30, position_y: 40 }),
      vehicle({ id: 'v2', display_name: 'V2', position_zone_id: 'z1', position_x: 60, position_y: 80 }),
    ]
    const { container } = render(
      <FleetIndoorMap zones={[zone()]} stations={[]} vehicles={vehicles} selectedVehicleId="v1" onSelectVehicle={vi.fn()} />,
    )
    expect(marker(container, 'V1')?.getAttribute('class')).toMatch(/stroke-white/)
    expect(marker(container, 'V2')?.getAttribute('class')).not.toMatch(/stroke-white/)
  })

  it('rendert mehrere Indoor-Zonen als eigene Karten', () => {
    render(
      <FleetIndoorMap
        zones={[zone({ id: 'z1', name: 'Lager Nord' }), zone({ id: 'z2', name: 'Lager Süd' })]}
        stations={[]}
        vehicles={[]}
        selectedVehicleId={null}
        onSelectVehicle={vi.fn()}
      />,
    )
    expect(screen.getByText('Lager Nord')).toBeInTheDocument()
    expect(screen.getByText('Lager Süd')).toBeInTheDocument()
  })
})
