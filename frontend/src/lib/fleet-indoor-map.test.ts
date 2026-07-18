import { describe, it, expect } from 'vitest'
import { indoorZones, stationsInZone, vehiclesInZone, autonomyFillColor, UNKNOWN_AUTONOMY_FILL } from './fleet-indoor-map'
import type { Zone, Station, FleetVehicle } from './api-client'

const zone = (overrides: Partial<Zone> = {}): Zone => ({
  id: 'z1',
  name: 'Zone 1',
  environment: 'indoor',
  svg_geometry: '',
  created_at: 't',
  ...overrides,
})

const station = (overrides: Partial<Station> = {}): Station => ({
  id: 's1',
  zone_id: 'z1',
  name: 'Station 1',
  created_at: 't',
  ...overrides,
})

const vehicle = (overrides: Partial<FleetVehicle> = {}): FleetVehicle => ({
  id: 'v1',
  display_name: 'V1',
  ...overrides,
})

describe('indoorZones', () => {
  it('filtert nur Zonen mit environment=indoor', () => {
    const zones = [zone({ id: 'a', environment: 'indoor' }), zone({ id: 'b', environment: 'outdoor' })]
    expect(indoorZones(zones).map((z) => z.id)).toEqual(['a'])
  })

  it('gibt leeres Array zurück, wenn keine Zone indoor ist', () => {
    expect(indoorZones([zone({ environment: 'outdoor' })])).toEqual([])
  })
})

describe('stationsInZone', () => {
  it('filtert Stationen der angegebenen Zone mit gesetzten position_x/y', () => {
    const stations = [
      station({ id: 's1', zone_id: 'z1', position_x: 10, position_y: 20 }),
      station({ id: 's2', zone_id: 'z2', position_x: 10, position_y: 20 }),
    ]
    expect(stationsInZone(stations, 'z1').map((s) => s.id)).toEqual(['s1'])
  })

  it('schließt Stationen ohne position_x/y aus (z. B. reine Outdoor-Station ohne x/y)', () => {
    const stations = [
      station({ id: 's1', zone_id: 'z1', position_lat: 52.1, position_lon: 11.6 }),
      station({ id: 's2', zone_id: 'z1', position_x: 10, position_y: 20 }),
    ]
    expect(stationsInZone(stations, 'z1').map((s) => s.id)).toEqual(['s2'])
  })
})

describe('vehiclesInZone', () => {
  it('filtert Fahrzeuge mit passender position_zone_id und gesetzten position_x/y', () => {
    const vehicles = [
      vehicle({ id: 'v1', position_zone_id: 'z1', position_x: 5, position_y: 5 }),
      vehicle({ id: 'v2', position_zone_id: 'z2', position_x: 5, position_y: 5 }),
    ]
    expect(vehiclesInZone(vehicles, 'z1').map((v) => v.id)).toEqual(['v1'])
  })

  it('schließt ein Fahrzeug mit passender zone_id aber ohne position_x/y aus', () => {
    const vehicles = [vehicle({ id: 'v1', position_zone_id: 'z1' })]
    expect(vehiclesInZone(vehicles, 'z1')).toEqual([])
  })

  it('schließt ein Fahrzeug ganz ohne position_zone_id aus (heutiger Regelfall, ADR-034 Scope)', () => {
    const vehicles = [vehicle({ id: 'v1', position_x: 5, position_y: 5 })]
    expect(vehiclesInZone(vehicles, 'z1')).toEqual([])
  })
})

describe('autonomyFillColor', () => {
  it('liefert die passende fill-Klasse pro Autonomiemodus', () => {
    expect(autonomyFillColor('autonomous')).toBe('fill-green-500')
    expect(autonomyFillColor('teleoperated')).toBe('fill-blue-500')
    expect(autonomyFillColor('manual')).toBe('fill-yellow-500')
  })

  it('liefert die Unknown-Farbe bei undefined oder unbekanntem Modus', () => {
    expect(autonomyFillColor(undefined)).toBe(UNKNOWN_AUTONOMY_FILL)
    expect(autonomyFillColor('bogus')).toBe(UNKNOWN_AUTONOMY_FILL)
  })
})
