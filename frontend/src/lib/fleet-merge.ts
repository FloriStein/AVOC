// Pure state-merge functions for the Fleet Overview dashboard — kept free of React/WebSocket so
// the merge semantics (the actual interesting logic here) are unit-testable in isolation.

import type { FleetVehicle, FleetAlert, Task } from '@/lib/api-client'
import type { FleetVehicleStatus, FleetAlertAcknowledgedEvent, FleetTaskStatusChangedEvent } from '@/lib/fleet-ws-events'

// Merges a live vehicle_status WS event into the vehicle list. No match (a status event for a
// vehicle_id not present in the initial GET /fleet/vehicles snapshot — e.g. registered between
// the REST fetch and the WS connecting) leaves the list unchanged: a conscious, documented gap —
// we don't fabricate a partial FleetVehicle row without a display_name.
//
// Non-destructive overlay: only fields present in the event overwrite the vehicle's
// corresponding field. A field omitted from the JSON payload (Go `omitempty`) leaves the
// vehicle's last known value intact rather than blanking it to undefined — a momentary MQTT
// reading missing e.g. GPS shouldn't flash the UI to "—" for data we still know from 2s ago.
export function mergeVehicleStatus(vehicles: FleetVehicle[], status: FleetVehicleStatus): FleetVehicle[] {
  const idx = vehicles.findIndex((v) => v.id === status.vehicle_id)
  if (idx === -1) return vehicles

  const existing = vehicles[idx]
  const updated: FleetVehicle = {
    ...existing,
    battery_pct: status.battery_pct ?? existing.battery_pct,
    speed: status.speed ?? existing.speed,
    position_lat: status.position_lat ?? existing.position_lat,
    position_lon: status.position_lon ?? existing.position_lon,
    position_zone_id: status.position_zone_id ?? existing.position_zone_id,
    autonomy_mode: status.autonomy_mode ?? existing.autonomy_mode,
    current_task_id: status.current_task_id ?? existing.current_task_id,
    status_updated_at: status.updated_at,
  }
  const next = vehicles.slice()
  next[idx] = updated
  return next
}

// Inserts or replaces an alert by id (idempotent against redelivery or a REST/WS race at mount).
// A genuinely new id is prepended (backend contract: newest-first); an existing id is replaced
// in place, not re-prepended.
export function upsertAlert(alerts: FleetAlert[], alert: FleetAlert): FleetAlert[] {
  const idx = alerts.findIndex((a) => a.id === alert.id)
  if (idx === -1) return [alert, ...alerts]
  const next = alerts.slice()
  next[idx] = alert
  return next
}

// Applies an alert_acknowledged event. No-op if the alert isn't in the local list (race-safe —
// e.g. acknowledged by another operator for an alert this client hasn't fetched yet).
export function applyAlertAcknowledged(alerts: FleetAlert[], ack: FleetAlertAcknowledgedEvent): FleetAlert[] {
  const idx = alerts.findIndex((a) => a.id === ack.id)
  if (idx === -1) return alerts
  const next = alerts.slice()
  next[idx] = { ...next[idx], acknowledged_by: ack.acknowledged_by, acknowledged_at: ack.acknowledged_at }
  return next
}

// Inserts or replaces a task by id (ADR-030) — same idempotent-upsert shape as upsertAlert,
// covering both the REST response from createFleetTask (called directly, before any WS event
// arrives) and the task_created WS broadcast (which then re-applies harmlessly by id).
export function upsertTask(tasks: Task[], task: Task): Task[] {
  const idx = tasks.findIndex((t) => t.id === task.id)
  if (idx === -1) return [task, ...tasks]
  const next = tasks.slice()
  next[idx] = task
  return next
}

// Applies a task_status_changed event. No-op if the task isn't in the local list (race-safe, same
// rationale as applyAlertAcknowledged) — only overlays the fields the event actually carries
// (status/completed_at/status_changed_by), leaving vehicle_id/from_station_id/etc. untouched.
export function applyTaskStatusChanged(tasks: Task[], event: FleetTaskStatusChangedEvent): Task[] {
  const idx = tasks.findIndex((t) => t.id === event.id)
  if (idx === -1) return tasks
  const next = tasks.slice()
  next[idx] = {
    ...next[idx],
    status: event.status,
    completed_at: event.completed_at,
    status_changed_by: event.status_changed_by,
  }
  return next
}
