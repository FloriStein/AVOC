import { describe, it, expect } from 'vitest'
import {
  parseGeoBounds,
  parseSvgGeometry,
  autonomyMarkerColor,
  vehiclesWithPosition,
  AUTONOMY_DOT,
  UNKNOWN_AUTONOMY_DOT,
} from './fleet-map'
import type { FleetVehicle } from './api-client'

const vehicle = (overrides: Partial<FleetVehicle> = {}): FleetVehicle => ({
  id: 'v1',
  display_name: 'V1',
  ...overrides,
})

describe('parseGeoBounds', () => {
  it('gibt null zurück, wenn geo_bounds fehlt', () => {
    expect(parseGeoBounds(undefined)).toBeNull()
  })

  it('gibt null zurück bei kaputtem JSON', () => {
    expect(parseGeoBounds('{not valid json')).toBeNull()
  })

  it('gibt null zurück bei falscher Shape (fehlendes ne)', () => {
    expect(parseGeoBounds(JSON.stringify({ sw: { lat: 1, lon: 2 } }))).toBeNull()
  })

  it('gibt null zurück bei nicht-numerischen Koordinaten', () => {
    const raw = JSON.stringify({ sw: { lat: '1', lon: 2 }, ne: { lat: 3, lon: 4 } })
    expect(parseGeoBounds(raw)).toBeNull()
  })

  it('gibt null zurück bei nicht-endlichen Zahlen (z. B. Exponent-Overflow zu Infinity)', () => {
    // 1e400 is valid JSON syntax but overflows to Infinity once JSON.parse converts it to a JS
    // number — a real, reachable edge case for the Number.isFinite guard, unlike NaN/Infinity
    // literals which aren't valid JSON tokens and would fail earlier at JSON.parse itself.
    const raw = '{"sw":{"lat":1e400,"lon":2},"ne":{"lat":3,"lon":4}}'
    expect(parseGeoBounds(raw)).toBeNull()
  })

  it('parst gültige geo_bounds korrekt', () => {
    const raw = JSON.stringify({ sw: { lat: 52.1295, lon: 11.639 }, ne: { lat: 52.1312, lon: 11.6425 } })
    expect(parseGeoBounds(raw)).toEqual({
      sw: { lat: 52.1295, lon: 11.639 },
      ne: { lat: 52.1312, lon: 11.6425 },
    })
  })

  it('gibt null zurück, wenn das geparste JSON kein Objekt ist', () => {
    expect(parseGeoBounds('"just a string"')).toBeNull()
    expect(parseGeoBounds('42')).toBeNull()
    expect(parseGeoBounds('null')).toBeNull()
  })
})

describe('parseSvgGeometry', () => {
  it('gibt null zurück bei leerem String', () => {
    expect(parseSvgGeometry('')).toBeNull()
    expect(parseSvgGeometry('   ')).toBeNull()
  })

  it('gibt null zurück bei kaputtem XML', () => {
    expect(parseSvgGeometry('<svg><rect x="0"')).toBeNull()
  })

  it('gibt null zurück, wenn das Wurzelelement kein svg ist', () => {
    expect(parseSvgGeometry('<div>not an svg</div>')).toBeNull()
  })

  it('parst gültiges SVG-Markup', () => {
    const el = parseSvgGeometry('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10"/></svg>')
    expect(el).not.toBeNull()
    expect(el!.tagName.toLowerCase()).toBe('svg')
  })
})

describe('autonomyMarkerColor', () => {
  it('liefert die korrekte Farbe für alle bekannten Modi', () => {
    expect(autonomyMarkerColor('autonomous')).toBe(AUTONOMY_DOT.autonomous)
    expect(autonomyMarkerColor('teleoperated')).toBe(AUTONOMY_DOT.teleoperated)
    expect(autonomyMarkerColor('manual')).toBe(AUTONOMY_DOT.manual)
  })

  it('liefert die Unknown-Farbe für undefined', () => {
    expect(autonomyMarkerColor(undefined)).toBe(UNKNOWN_AUTONOMY_DOT)
  })

  it('liefert die Unknown-Farbe für einen unbekannten String', () => {
    expect(autonomyMarkerColor('quantum-teleport')).toBe(UNKNOWN_AUTONOMY_DOT)
  })
})

describe('vehiclesWithPosition', () => {
  it('behält Fahrzeuge mit vollständiger Position', () => {
    const vehicles = [vehicle({ position_lat: 52.1, position_lon: 11.6 })]
    expect(vehiclesWithPosition(vehicles)).toEqual(vehicles)
  })

  it('schließt Fahrzeuge ohne position_lat aus (z. B. vehicle-001)', () => {
    const vehicles = [vehicle({ id: 'vehicle-001', position_lon: 11.6 })]
    expect(vehiclesWithPosition(vehicles)).toEqual([])
  })

  it('schließt Fahrzeuge ohne position_lon aus', () => {
    const vehicles = [vehicle({ position_lat: 52.1 })]
    expect(vehiclesWithPosition(vehicles)).toEqual([])
  })

  it('schließt Fahrzeuge ganz ohne Positionsdaten aus', () => {
    const vehicles = [vehicle()]
    expect(vehiclesWithPosition(vehicles)).toEqual([])
  })

  it('behält mehrere Fahrzeuge an identischer Position (keine Deduplizierung)', () => {
    const vehicles = [
      vehicle({ id: 'v1', position_lat: 52.1, position_lon: 11.6 }),
      vehicle({ id: 'v2', position_lat: 52.1, position_lon: 11.6 }),
    ]
    expect(vehiclesWithPosition(vehicles)).toHaveLength(2)
  })
})
