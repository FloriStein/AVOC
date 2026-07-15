import { describe, it, expect } from 'vitest'
import { parseFleetWSMessage } from './fleet-ws-events'

describe('parseFleetWSMessage', () => {
  it('parst vehicle_status', () => {
    const raw = JSON.stringify({ type: 'vehicle_status', data: { vehicle_id: 'v1', autonomy_mode: 'autonomous', updated_at: 't1' } })
    const event = parseFleetWSMessage(raw)
    expect(event.type).toBe('vehicle_status')
    if (event.type === 'vehicle_status') {
      expect(event.data.vehicle_id).toBe('v1')
    }
  })

  it('parst alert_created', () => {
    const raw = JSON.stringify({ type: 'alert_created', data: { id: 'a1', vehicle_id: 'v1', severity: 'critical', message: 'x', created_at: 't1' } })
    const event = parseFleetWSMessage(raw)
    expect(event.type).toBe('alert_created')
    if (event.type === 'alert_created') {
      expect(event.data.severity).toBe('critical')
    }
  })

  it('parst alert_acknowledged', () => {
    const raw = JSON.stringify({ type: 'alert_acknowledged', data: { id: 'a1', acknowledged_by: 'op1', acknowledged_at: 't1' } })
    const event = parseFleetWSMessage(raw)
    expect(event.type).toBe('alert_acknowledged')
    if (event.type === 'alert_acknowledged') {
      expect(event.data.acknowledged_by).toBe('op1')
    }
  })

  it('parst task_created ohne zu werfen (out of scope, aber muss nicht crashen)', () => {
    const raw = JSON.stringify({ type: 'task_created', data: { id: 't1' } })
    const event = parseFleetWSMessage(raw)
    expect(event.type).toBe('task_created')
  })

  it('unbekannter type-Wert wird als unknown getaggt, nicht verworfen', () => {
    const raw = JSON.stringify({ type: 'something_new', data: { foo: 'bar' } })
    const event = parseFleetWSMessage(raw)
    expect(event.type).toBe('unknown')
    if (event.type === 'unknown') {
      expect(event.rawType).toBe('something_new')
    }
  })

  it('kaputtes JSON wirft nicht, sondern liefert unknown', () => {
    expect(() => parseFleetWSMessage('{not valid json')).not.toThrow()
    const event = parseFleetWSMessage('{not valid json')
    expect(event.type).toBe('unknown')
  })

  it('fehlendes type-Feld liefert unknown', () => {
    const event = parseFleetWSMessage(JSON.stringify({ data: { foo: 'bar' } }))
    expect(event.type).toBe('unknown')
  })

  it('type ist kein String liefert unknown, ohne zu werfen', () => {
    const event = parseFleetWSMessage(JSON.stringify({ type: 42, data: {} }))
    expect(event.type).toBe('unknown')
  })

  it('JSON-Array statt Objekt liefert unknown, ohne zu werfen', () => {
    expect(() => parseFleetWSMessage('[1,2,3]')).not.toThrow()
    expect(parseFleetWSMessage('[1,2,3]').type).toBe('unknown')
  })

  it('JSON null liefert unknown, ohne zu werfen', () => {
    expect(parseFleetWSMessage('null').type).toBe('unknown')
  })

  it('leerer String liefert unknown, ohne zu werfen', () => {
    expect(() => parseFleetWSMessage('')).not.toThrow()
    expect(parseFleetWSMessage('').type).toBe('unknown')
  })
})
