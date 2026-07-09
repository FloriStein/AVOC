# ADR-025: Multi-Operator Support

**Status:** Accepted  
**Date:** 2026-06-14  
**Sprint:** 12

## Context

The original architecture (ADR-015) assumed exactly one operator per control-server instance. A single `session.Manager.current *Session` pointer and a single global state machine made it impossible for two operators to simultaneously work with the system — even in a read-only (observer) role.

Use cases that require multi-operator:

- Instructor + student: instructor observes while student controls.
- Redundant safety: second operator can trigger E-Stop even if not controlling.
- Fleet handover: new operator joins as observer, waits for control to be released.

## Decision

### Role model

Two roles per session:

| Role | Control commands | E-Stop | Video | Telemetry |
|------|-----------------|--------|-------|-----------|
| `ACTIVE_OPERATOR` | ✅ | ✅ | ✅ | ✅ |
| `OBSERVER` | ❌ | ✅ | ✅ | ✅ |

Exactly one `ACTIVE_OPERATOR` per vehicle at any time. Additional operators who call `POST /session/start` for an already-controlled vehicle receive the `OBSERVER` role.

### Vehicle locking

`session.Manager` maintains:
- `sessions map[string]*Session` — all live sessions keyed by session_id
- `vehicleController map[string]string` — vehicleID → controller session_id

`StartSession(vehicleID, operatorID)` auto-determines the role: ACTIVE_OPERATOR if no live controller exists, OBSERVER otherwise.

### WS connects after session/start

Previously the WebSocket connected on login (before any session existed).  
Now: `POST /session/start` → `{session_id, role, vehicle_id}` → WS connects with `?session_id=<id>`.

This lets the WS handler look up the session immediately, determine the role, and apply role-based behavior without a separate auth step.

### State machine (global, ACTIVE_OPERATOR-driven)

The global `statemachine.Machine` is only advanced by `ACTIVE_OPERATOR` events:

- `POST /session/start` (ACTIVE_OPERATOR): `IDLE → CONNECTING → AUTHENTICATED → CONNECTED`
- ACTIVE_OPERATOR WS disconnect: `CONNECTED → SAFE_MODE`
- ACTIVE_OPERATOR WS reconnect after SAFE_MODE: `SAFE_MODE → RECOVERING → AUTHENTICATED → CONNECTED`
- OBSERVER connect / disconnect: **no state change**

### Observer disconnect

When an OBSERVER WS closes, `sessionMgr.ReleaseSession(sess.ID)` is called to clean up.  
No SAFE_MODE, no deadman stop, no audit event.

### When controller leaves

Observer **waits and must re-request** — no auto-promotion. The observer can call  
`POST /session/end` on their own session, then `POST /session/start` again for the  
now-free vehicle to become the ACTIVE_OPERATOR.

### E-Stop for observers

Safety takes priority: `EMERGENCY_STOP` is never blocked by the OBSERVER check in  
`command/engine.go`.

## Consequences

- `session.Manager.CreateSession()` kept for backward compatibility (handover manager, tests). Prefer `StartSession()` for new code.
- `GET /state` unchanged — returns global state (driven by ACTIVE_OPERATOR).
- Frontend: vehicle selector shown when `!sessionId` (not `systemState === 'AUTHENTICATED'`).
- Frontend: `ControlPanel` disabled when `session.role === 'OBSERVER'`.
- Frontend: `ConnectionPanel` shows a role badge (Operator / Observer).
- `DELETE /vehicles/{id}` uses `IsVehicleLocked()` instead of checking `GetCurrentSession()`.
- WHEP auth unchanged — still verifies that an ACTIVE_OPERATOR session exists for the vehicle path.
