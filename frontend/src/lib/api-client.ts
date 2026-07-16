// HTTP client for AVOC backend REST endpoints (via nginx proxy).
// All paths are relative — nginx routes /api/ → control-server, /auth/ → auth-service.

export interface SystemStateResponse {
  system: string
  control: string
  media: string
  operator: string
}

export async function login(id: string, password: string): Promise<string> {
  const res = await fetch('/auth/operator/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username: id, password }),
  })
  if (!res.ok) throw new Error(`login failed: ${res.status}`)
  const { token } = await res.json()
  return token as string
}

// Calls POST /auth/logout — server rejects with 409 if an ACTIVE_OPERATOR session is still live.
// Throws an Error with message "active_session" in that case so callers can react.
export async function logout(token: string): Promise<void> {
  const res = await fetch('/api/logout', {
    method: 'POST',
    headers: { 'Authorization': `Bearer ${token}` },
  })
  if (res.status === 409) throw new Error('active_session')
  if (!res.ok) throw new Error(`logout failed: ${res.status}`)
}

// Decodes the JWT payload and returns the `role` claim without an external library.
export function parseTokenRole(token: string): string {
  try {
    const payload = JSON.parse(atob(token.split('.')[1]))
    return payload.role ?? ''
  } catch {
    return ''
  }
}

// GET /vehicles/{id}/state — per-vehicle 4-layer state snapshot (ADR-026).
export async function getVehicleState(vehicleId: string): Promise<SystemStateResponse> {
  const res = await fetch(`/api/vehicles/${vehicleId}/state`)
  if (!res.ok) throw new Error(`getVehicleState failed: ${res.status}`)
  return res.json()
}

export interface StartSessionResult {
  session_id: string
  role: 'ACTIVE_OPERATOR' | 'OBSERVER'
  vehicle_id: string
}

export async function startSession(vehicleId: string, operatorId: string, token: string): Promise<StartSessionResult> {
  const res = await fetch('/api/session/start', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
    body: JSON.stringify({ vehicle_id: vehicleId, operator_id: operatorId }),
  })
  if (!res.ok) throw new Error(`startSession failed: ${res.status}`)
  return res.json()
}

export async function endSession(sessionId: string, token: string): Promise<void> {
  await fetch('/api/session/end', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
    body: JSON.stringify({ session_id: sessionId }),
  })
}

export async function emergencyStop(sessionId: string, vehicleId: string, token: string): Promise<void> {
  await fetch('/api/emergency-stop', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
    body: JSON.stringify({
      session_id: sessionId,
      vehicle_id: vehicleId,
      reason: 'operator emergency stop',
    }),
  })
}

export interface VehicleInfo {
  id: string
  display_name: string
  description: string
  online: boolean
}

export async function listVehicles(): Promise<VehicleInfo[]> {
  const res = await fetch('/api/vehicles')
  if (!res.ok) throw new Error(`listVehicles failed: ${res.status}`)
  return res.json()
}

// Reports WebRTC MEDIA STATE changes to the Control Server (ADR-009 Invariant 1).
// MEDIA_FAILED → DEGRADED on server side — never SAFE_MODE.
export async function reportMediaState(state: string, vehicleId: string, token: string): Promise<void> {
  await fetch('/api/media/event', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
    body: JSON.stringify({ state, vehicle_id: vehicleId }),
  })
}

// ─── User Management (ADR-024) ───────────────────────────────────────────────

export interface UserInfo {
  id: number
  username: string
  role: string
  is_active: boolean
  created_at: string
  last_auth_at?: string
}

export async function listUsers(token: string): Promise<UserInfo[]> {
  const res = await fetch('/auth/users', {
    headers: { 'Authorization': `Bearer ${token}` },
  })
  if (!res.ok) throw new Error(`listUsers failed: ${res.status}`)
  return res.json()
}

export async function createUser(
  token: string,
  username: string,
  password: string,
  role: string,
): Promise<void> {
  const res = await fetch('/auth/users', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
    body: JSON.stringify({ username, password, role }),
  })
  if (!res.ok) throw new Error(`createUser failed: ${res.status}`)
}

export async function deleteUser(token: string, id: number): Promise<void> {
  const res = await fetch(`/auth/users/${id}`, {
    method: 'DELETE',
    headers: { 'Authorization': `Bearer ${token}` },
  })
  if (!res.ok) throw new Error(`deleteUser failed: ${res.status}`)
}

export interface ActiveSession {
  session_id: string
  vehicle_id: string
  operator_id: string
  role: string
  created_at: string
}

export async function listSessions(token: string): Promise<ActiveSession[]> {
  const res = await fetch('/api/sessions', {
    headers: { 'Authorization': `Bearer ${token}` },
  })
  if (!res.ok) throw new Error(`listSessions failed: ${res.status}`)
  return res.json()
}

export async function updateUserRole(token: string, id: number, role: string): Promise<void> {
  const res = await fetch(`/auth/users/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
    body: JSON.stringify({ role }),
  })
  if (!res.ok) throw new Error(`updateUserRole failed: ${res.status}`)
}

// ─── Fleet (fleet-service, ADR-027/028/029) ──────────────────────────────────
// GET /fleet/vehicles: field-optionality mirrors internal/fleetservice/store.go's FleetVehicle
// exactly — all `?` fields are Go pointers with `omitempty`, nil until the vehicle has ever
// reported status (e.g. vehicle-001, the pre-existing Direct-Teleop vehicle, has none of these).

export interface FleetVehicle {
  id: string
  display_name: string
  vehicle_type?: 'lastenrad' | 'lastenzug'
  battery_pct?: number
  speed?: number
  position_lat?: number
  position_lon?: number
  position_zone_id?: string
  autonomy_mode?: 'autonomous' | 'teleoperated' | 'manual'
  current_task_id?: string
  status_updated_at?: string
}

export interface FleetAlert {
  id: string
  vehicle_id: string
  severity: 'info' | 'warning' | 'critical'
  message: string
  created_at: string
  acknowledged_at?: string
  acknowledged_by?: string
}

// GET /fleet/zones: geo_bounds mirrors internal/fleetservice/store.go's Zone.GeoBounds (*string,
// raw JSON passthrough) — callers must JSON.parse it themselves (see lib/fleet-map.ts's
// parseGeoBounds), it is never a nested object on the wire.
export interface Zone {
  id: string
  name: string
  environment: 'indoor' | 'outdoor'
  svg_geometry: string
  geo_bounds?: string
  created_at: string
}

export interface Station {
  id: string
  zone_id: string
  name: string
  position_x?: number
  position_y?: number
  position_lat?: number
  position_lon?: number
  created_at: string
}

export async function listFleetVehicles(token: string): Promise<FleetVehicle[]> {
  const res = await fetch('/fleet/vehicles', { headers: { 'Authorization': `Bearer ${token}` } })
  if (!res.ok) throw new Error(`listFleetVehicles failed: ${res.status}`)
  return res.json()
}

// Both list endpoints return JSON `null` (not `[]`) when the underlying table is empty — the Go
// store leaves the backing slice nil for zero rows (internal/fleetservice/store.go's
// ListZones/ListStations), and json.Marshal renders a nil slice as null. Normalized to `[]` here
// so callers never need a null-check.

export async function listFleetZones(token: string): Promise<Zone[]> {
  const res = await fetch('/fleet/zones', { headers: { 'Authorization': `Bearer ${token}` } })
  if (!res.ok) throw new Error(`listFleetZones failed: ${res.status}`)
  return (await res.json()) ?? []
}

export async function listFleetStations(token: string): Promise<Station[]> {
  const res = await fetch('/fleet/stations', { headers: { 'Authorization': `Bearer ${token}` } })
  if (!res.ok) throw new Error(`listFleetStations failed: ${res.status}`)
  return (await res.json()) ?? []
}

export async function listFleetAlerts(token: string): Promise<FleetAlert[]> {
  const res = await fetch('/fleet/alerts', { headers: { 'Authorization': `Bearer ${token}` } })
  if (!res.ok) throw new Error(`listFleetAlerts failed: ${res.status}`)
  return res.json()
}

export async function acknowledgeFleetAlert(token: string, id: string, acknowledgedBy: string): Promise<void> {
  const res = await fetch(`/fleet/alerts/${id}/acknowledge`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
    body: JSON.stringify({ acknowledged_by: acknowledgedBy }),
  })
  if (!res.ok) throw new Error(`acknowledgeFleetAlert failed: ${res.status}`)
}

// Sprint 24 (ADR-030) — Task-Management-UI. Field optionality mirrors
// internal/fleetservice/store.go's Task exactly (see FleetVehicle's comment above for why this
// matters: the Go struct's `omitempty`/pointer-ness is the actual contract, not a guess). `Station`
// already exists above (Sprint 23, MAP-03) — reused as-is.

export interface Task {
  id: string
  vehicle_id: string
  from_station_id: string
  to_station_id: string
  status: 'pending' | 'in_progress' | 'completed' | 'cancelled'
  priority: number
  created_at: string
  completed_at?: string
  status_changed_by?: string
}

export async function listFleetTasks(token: string): Promise<Task[]> {
  const res = await fetch('/fleet/tasks', { headers: { 'Authorization': `Bearer ${token}` } })
  if (!res.ok) throw new Error(`listFleetTasks failed: ${res.status}`)
  return res.json()
}

export interface CreateFleetTaskInput {
  vehicle_id: string
  from_station_id: string
  to_station_id: string
  priority: number
}

export async function createFleetTask(token: string, input: CreateFleetTaskInput): Promise<Task> {
  const res = await fetch('/fleet/tasks', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
    body: JSON.stringify(input),
  })
  if (!res.ok) throw new Error(`createFleetTask failed: ${res.status}`)
  return res.json()
}

// updateFleetTaskStatus can fail with a 409 (ADR-030 — the task's current status doesn't allow
// this transition) in addition to network/auth errors; callers that show inline feedback should
// inspect the thrown Error's message for "409" rather than treating every failure alike.
export async function updateFleetTaskStatus(token: string, id: string, status: string, changedBy: string): Promise<Task> {
  const res = await fetch(`/fleet/tasks/${id}/status`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
    body: JSON.stringify({ status, changed_by: changedBy }),
  })
  if (!res.ok) throw new Error(`updateFleetTaskStatus failed: ${res.status}`)
  return res.json()
}
