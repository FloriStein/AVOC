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
export async function reportMediaState(state: string, token: string): Promise<void> {
  await fetch('/api/media/event', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
    body: JSON.stringify({ state }),
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
