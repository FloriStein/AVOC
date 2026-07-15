// Typed parsing for the Fleet Dashboard WebSocket's JSON envelope (FLEET-06,
// internal/fleetservice/broadcast.go's WSEvent{type, data}). Kept free of any WebSocket
// dependency so the parsing logic is unit-testable without mocking a socket — unlike
// lib/ws-client.ts (binary Protobuf), which has no colocated test file for exactly that reason.

import type { FleetAlert } from '@/lib/api-client'

// Mirrors internal/fleetservice.VehicleStatus's JSON shape exactly — NOT the same as
// api-client.ts's FleetVehicle: autonomy_mode is non-optional here (Go: plain string), while
// FleetVehicle.autonomy_mode is optional (Go: *string, omitempty). This asymmetry is real and
// must be preserved, not "fixed" for consistency.
export interface FleetVehicleStatus {
  vehicle_id: string
  battery_pct?: number
  speed?: number
  position_lat?: number
  position_lon?: number
  position_zone_id?: string
  autonomy_mode: 'autonomous' | 'teleoperated' | 'manual'
  current_task_id?: string
  updated_at: string
}

export interface FleetAlertAcknowledgedEvent {
  id: string
  acknowledged_by: string
  acknowledged_at: string
}

export type FleetWSEvent =
  | { type: 'vehicle_status'; data: FleetVehicleStatus }
  | { type: 'alert_created'; data: FleetAlert }
  | { type: 'alert_acknowledged'; data: FleetAlertAcknowledgedEvent }
  | { type: 'task_created'; data: unknown }
  | { type: 'unknown'; rawType: string; data: unknown }

// Parses one raw WS text frame. Never throws — malformed JSON or an envelope missing/mistyped
// "type"/"data" collapses to the 'unknown' variant so the caller can log and skip rather than
// crash the WS message handler. 'unknown' is an explicit tag (not a widened `{type: string}`
// fallback) so `switch (event.type)` stays exhaustively narrowed for the 4 real cases downstream.
export function parseFleetWSMessage(raw: string): FleetWSEvent {
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return { type: 'unknown', rawType: '', data: raw }
  }

  if (typeof parsed !== 'object' || parsed === null) {
    return { type: 'unknown', rawType: '', data: parsed }
  }
  const envelope = parsed as Record<string, unknown>
  const rawType = envelope.type
  if (typeof rawType !== 'string') {
    return { type: 'unknown', rawType: '', data: envelope }
  }

  switch (rawType) {
    case 'vehicle_status':
      return { type: 'vehicle_status', data: envelope.data as FleetVehicleStatus }
    case 'alert_created':
      return { type: 'alert_created', data: envelope.data as FleetAlert }
    case 'alert_acknowledged':
      return { type: 'alert_acknowledged', data: envelope.data as FleetAlertAcknowledgedEvent }
    case 'task_created':
      return { type: 'task_created', data: envelope.data }
    default:
      return { type: 'unknown', rawType, data: envelope.data }
  }
}
