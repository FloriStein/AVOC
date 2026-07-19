# Sprint 3 — Frontend Core

Abgeschlossen: 2026-06-03

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| FE-09 | Frontend Protobuf Adapter + Build-Pipeline | M | ✅ `@bufbuild/protobuf` + `@bufbuild/protoc-gen-es`; `common_pb.js` + `control_pb.js` im Bundle; nginx `/api/` + `/auth/` + `/vehicle/ws` Proxy-Routen |
| FE-02 | WebSocket Client + State-Polling | M | ✅ `ws-client.ts` mit Latenz-Messung; `useSystemState` (500ms Polling); `useSession` (auto-login, Reconnect mit Exponential Backoff) |
| FE-08 | SAFE MODE Overlay + Operator Ack Flow | M | ✅ Fullscreen-Overlay bei SAFE_MODE; Resume-Button triggert Recovery-Flow; DEGRADED-Banner |
| FE-04 | Safety Controls — Emergency Stop + Dead-man Switch | M | ✅ E-Stop → `POST /api/emergency-stop`; Dead-man (Spacebar/Mousedown, 400ms Interval, Protobuf DEADMAN_HOLD) |
| FE-03 | Connection Status — Live-Anzeige | S | ✅ Live Latenz (grün <50ms, gelb <100ms, rot ≥100ms), Session-ID (gekürzt), Operator-Rolle, State-Badge |

### Bugfixes während Tests

1. **`transport/websocket.go`**: Recovery-Pfad fehlte — bei neuem WS-Connect aus SAFE_MODE wurde `CONNECTING` versucht (invalid Transition). Fix: Wenn System in `SAFE_MODE`, dann `SAFE_MODE → RECOVERING → AUTHENTICATED` statt `IDLE → CONNECTING → AUTHENTICATED`.
2. **`frontend/package-lock.json`**: Neuer Dependencies (`@bufbuild/protobuf`, `@bufbuild/protoc-gen-es`, `ulidx`) nicht im Lock-File — `npm install` ausgeführt.
3. **`ConnectionPanel.tsx`**: Ungenutzte `type SystemState` Deklaration (TS6196) → entfernt.

### Testprotokoll Integration (2026-06-03)

| Test | Erwartung | Ergebnis |
|------|-----------|----------|
| Health-Checks alle 5 Services | `{"status":"ok"}` | ✅ |
| nginx `/api/state` | JSON (nicht HTML) | ✅ `{"system":"IDLE",...}` |
| nginx `/auth/operator/login` | JWT-Token | ✅ Token mit `role=OBSERVER` |
| nginx `/api/nonexistent` | `404 page not found` (nicht HTML) | ✅ |
| nginx `/auth/nonexistent` | HTTP 404 | ✅ |
| Protobuf-Bundle: `common_pb.js` + `control_pb.js` | Vorhanden | ✅ |
| Protobuf-Strings im Bundle | `CorrelationHeader`, `DEADMAN_HOLD`, `EMERGENCY_STOP` | ✅ |
| Login → WS → session/start → CONNECTED + ULID | `CONTROL_ACTIVE / ACTIVE_OPERATOR / session_id (26 Zeichen)` | ✅ |
| State-Polling (5 × 500ms) | Konsistente JSON-Antwort | ✅ |
| Emergency Stop `/api/emergency-stop` | HTTP 202, `SAFE_MODE / CONTROL_BLOCKED`, Safety Bus `EMERGENCY_STOP` | ✅ |
| Dead-man Watchdog (2s ohne Reset) | `SAFE_MODE` nach 2.5s | ✅ |
| Recovery: SAFE_MODE → Resume → CONNECTED | `RECOVERING → AUTHENTICATED → CONNECTED`, neue Session-ID ≠ alte | ✅ |
| Vehicle WS via nginx `/vehicle/ws` | Verbindung aufgebaut | ✅ |
| Vehicle WS mit Operator-Token → 401 | HTTP 401 | ✅ |
| Vehicle Disconnect → SAFE_MODE | `SAFE_MODE / CONTROL_BLOCKED` | ✅ |
| Handover Request → HANDOVER_PENDING | HTTP 202 | ✅ |
| Handover Confirm → ACTIVE_OPERATOR | HTTP 200 | ✅ |
| Session End → NO_OPERATOR → SAFE_MODE | HTTP 204, `SAFE_MODE / NO_OPERATOR` | ✅ |
| Sprint-2 Safety Tests (Regression) | 19/19 grün | ✅ |
| SAFE MODE Overlay-Text im Bundle | `"SAFE MODE"`, `"Resume — Operator Acknowledgment"` | ✅ |

### Neue Dateien

- `frontend/src/lib/api-client.ts` — HTTP-Client (login, getState, startSession, emergencyStop)
- `frontend/src/lib/ws-client.ts` — WebSocket-Client mit Latenz-Messung
- `frontend/src/hooks/useSystemState.ts` — Polling-Hook (500ms)
- `frontend/src/hooks/useSession.ts` — Session-Lifecycle (Login, Connect, Backoff, Resume)
- `frontend/src/hooks/useDeadmanSwitch.ts` — Dead-man (Spacebar/Button, Protobuf DEADMAN_HOLD)
- `frontend/src/components/SafeModeOverlay.tsx` — Fullscreen SAFE MODE Block
- `frontend/src/components/SafetyPanel.tsx` — Emergency Stop + Dead-man UI
- `frontend/src/components/ConnectionPanel.tsx` — Live State, Latenz, Session-ID, Rolle
- `frontend/src/App.tsx` (aktualisiert) — alle Komponenten verdrahtet
- `infrastructure/docker/nginx.conf` (aktualisiert) — `/api/`, `/auth/`, `/vehicle/ws` Proxy
- `infrastructure/docker/frontend.Dockerfile` (aktualisiert) — protoc + proto-gen vor Build
- `frontend/vite.config.ts` (aktualisiert) — Dev-Server Proxy
- `frontend/package.json` (aktualisiert) — `@bufbuild/protobuf`, `@bufbuild/protoc-gen-es`, `ulidx`
- `cmd/control-server/main.go` (aktualisiert) — `/emergency-stop` Proxy-Endpunkt
- `internal/controlserver/transport/websocket.go` (aktualisiert) — Recovery-Pfad SAFE_MODE→RECOVERING
