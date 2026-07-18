import { describe, it, expect } from 'vitest'
import { mergeVehicleStatus, upsertAlert, applyAlertAcknowledged, upsertTask, applyTaskStatusChanged } from './fleet-merge'
import type { FleetVehicle, FleetAlert, Task } from './api-client'
import type { FleetVehicleStatus, FleetAlertAcknowledgedEvent, FleetTaskStatusChangedEvent } from './fleet-ws-events'

const vehicle = (overrides: Partial<FleetVehicle> = {}): FleetVehicle => ({
  id: 'v1',
  display_name: 'V1',
  ...overrides,
})

describe('mergeVehicleStatus', () => {
  it('überschreibt Felder eines vorhandenen Fahrzeugs', () => {
    const vehicles = [vehicle({ battery_pct: 50 })]
    const status: FleetVehicleStatus = { vehicle_id: 'v1', battery_pct: 42, autonomy_mode: 'autonomous', updated_at: 't1' }

    const result = mergeVehicleStatus(vehicles, status)

    expect(result[0].battery_pct).toBe(42)
    expect(result[0].autonomy_mode).toBe('autonomous')
    expect(result[0].status_updated_at).toBe('t1')
  })

  it('lässt die Liste unverändert, wenn kein Fahrzeug mit der vehicle_id existiert', () => {
    const vehicles = [vehicle({ id: 'v1' })]
    const status: FleetVehicleStatus = { vehicle_id: 'unknown', autonomy_mode: 'autonomous', updated_at: 't1' }

    const result = mergeVehicleStatus(vehicles, status)

    expect(result).toBe(vehicles) // unchanged reference, not just equal content
  })

  it('nicht-destruktives Overlay: im Event fehlende Felder behalten den zuletzt bekannten Wert', () => {
    const vehicles = [vehicle({ battery_pct: 50, position_lat: 52.1, position_lon: 11.6 })]
    // Event omits position_lat/position_lon entirely (Go omitempty) — must NOT blank them.
    const status: FleetVehicleStatus = { vehicle_id: 'v1', battery_pct: 45, autonomy_mode: 'autonomous', updated_at: 't2' }

    const result = mergeVehicleStatus(vehicles, status)

    expect(result[0].battery_pct).toBe(45)
    expect(result[0].position_lat).toBe(52.1)
    expect(result[0].position_lon).toBe(11.6)
  })

  it('aktualisiert nur das passende Fahrzeug, andere bleiben unverändert', () => {
    const vehicles = [vehicle({ id: 'v1', battery_pct: 50 }), vehicle({ id: 'v2', battery_pct: 80 })]
    const status: FleetVehicleStatus = { vehicle_id: 'v1', battery_pct: 10, autonomy_mode: 'autonomous', updated_at: 't1' }

    const result = mergeVehicleStatus(vehicles, status)

    expect(result[0].battery_pct).toBe(10)
    expect(result[1].battery_pct).toBe(80)
  })

  it('leere Fahrzeugliste bleibt leer', () => {
    const status: FleetVehicleStatus = { vehicle_id: 'v1', autonomy_mode: 'autonomous', updated_at: 't1' }
    expect(mergeVehicleStatus([], status)).toEqual([])
  })
})

describe('upsertAlert', () => {
  const alert = (overrides: Partial<FleetAlert> = {}): FleetAlert => ({
    id: 'a1',
    vehicle_id: 'v1',
    severity: 'warning',
    message: 'test',
    created_at: 't1',
    ...overrides,
  })

  it('stellt ein neues Alert an den Anfang (newest-first)', () => {
    const alerts = [alert({ id: 'a1' })]
    const result = upsertAlert(alerts, alert({ id: 'a2' }))
    expect(result.map((a) => a.id)).toEqual(['a2', 'a1'])
  })

  it('ist idempotent: dieselbe id wird ersetzt, nicht erneut vorangestellt', () => {
    const alerts = [alert({ id: 'a1', message: 'first' }), alert({ id: 'a2' })]
    const result = upsertAlert(alerts, alert({ id: 'a1', message: 'updated' }))

    expect(result).toHaveLength(2)
    expect(result[0].id).toBe('a1')
    expect(result[0].message).toBe('updated')
  })

  it('leere Liste: Alert wird eingefügt', () => {
    const result = upsertAlert([], alert({ id: 'a1' }))
    expect(result).toEqual([alert({ id: 'a1' })])
  })
})

describe('applyAlertAcknowledged', () => {
  const alert = (overrides: Partial<FleetAlert> = {}): FleetAlert => ({
    id: 'a1',
    vehicle_id: 'v1',
    severity: 'warning',
    message: 'test',
    created_at: 't1',
    ...overrides,
  })

  it('setzt acknowledged_by/acknowledged_at bei passender id', () => {
    const ack: FleetAlertAcknowledgedEvent = { id: 'a1', acknowledged_by: 'op1', acknowledged_at: 't2' }
    const result = applyAlertAcknowledged([alert()], ack)

    expect(result[0].acknowledged_by).toBe('op1')
    expect(result[0].acknowledged_at).toBe('t2')
  })

  it('no-op wenn die id nicht gefunden wird (race-safe)', () => {
    const alerts = [alert({ id: 'a1' })]
    const ack: FleetAlertAcknowledgedEvent = { id: 'unknown', acknowledged_by: 'op1', acknowledged_at: 't2' }

    const result = applyAlertAcknowledged(alerts, ack)

    expect(result).toBe(alerts) // unchanged reference
  })
})

describe('upsertTask', () => {
  const task = (overrides: Partial<Task> = {}): Task => ({
    id: 't1',
    vehicle_id: 'v1',
    from_station_id: 'station-a',
    to_station_id: 'station-b',
    status: 'pending',
    priority: 0,
    created_at: 't1',
    allowed_transitions: ['in_progress', 'cancelled'],
    ...overrides,
  })

  it('stellt einen neuen Task an den Anfang (newest-first)', () => {
    const tasks = [task({ id: 't1' })]
    const result = upsertTask(tasks, task({ id: 't2' }))
    expect(result.map((t) => t.id)).toEqual(['t2', 't1'])
  })

  it('ist idempotent: dieselbe id wird ersetzt, nicht erneut vorangestellt', () => {
    const tasks = [task({ id: 't1', priority: 1 }), task({ id: 't2' })]
    const result = upsertTask(tasks, task({ id: 't1', priority: 9 }))

    expect(result).toHaveLength(2)
    expect(result[0].id).toBe('t1')
    expect(result[0].priority).toBe(9)
  })

  it('leere Liste: Task wird eingefügt', () => {
    const result = upsertTask([], task({ id: 't1' }))
    expect(result).toEqual([task({ id: 't1' })])
  })
})

describe('applyTaskStatusChanged', () => {
  const task = (overrides: Partial<Task> = {}): Task => ({
    id: 't1',
    vehicle_id: 'v1',
    from_station_id: 'station-a',
    to_station_id: 'station-b',
    status: 'pending',
    priority: 0,
    created_at: 't1',
    allowed_transitions: ['in_progress', 'cancelled'],
    ...overrides,
  })

  it('überschreibt status/completed_at/status_changed_by bei passender id', () => {
    const event: FleetTaskStatusChangedEvent = {
      id: 't1', status: 'completed', completed_at: 't2', status_changed_by: 'op1',
    }
    const result = applyTaskStatusChanged([task()], event)

    expect(result[0].status).toBe('completed')
    expect(result[0].completed_at).toBe('t2')
    expect(result[0].status_changed_by).toBe('op1')
  })

  it('lässt vehicle_id/from_station_id/to_station_id/priority unangetastet', () => {
    const original = task({ vehicle_id: 'v1', from_station_id: 'station-a', to_station_id: 'station-b', priority: 7 })
    const event: FleetTaskStatusChangedEvent = { id: 't1', status: 'in_progress', status_changed_by: 'op1' }

    const result = applyTaskStatusChanged([original], event)

    expect(result[0].vehicle_id).toBe('v1')
    expect(result[0].from_station_id).toBe('station-a')
    expect(result[0].to_station_id).toBe('station-b')
    expect(result[0].priority).toBe(7)
  })

  it('no-op wenn die id nicht gefunden wird (race-safe)', () => {
    const tasks = [task({ id: 't1' })]
    const event: FleetTaskStatusChangedEvent = { id: 'unknown', status: 'cancelled', status_changed_by: 'op1' }

    const result = applyTaskStatusChanged(tasks, event)

    expect(result).toBe(tasks) // unchanged reference
  })
})
