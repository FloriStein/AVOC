# Sprint 13 — Dev-Stack Stabilisierung & Log-Korrelation

Abgeschlossen: 2026-06-13

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| DEV-01 | `nginx.dev.conf` — HTTP-only Dev-nginx; `docker-compose.yml` Volume-Mount | S | ✅ SSL-Fehler beim `make up` behoben |
| DEV-02 | Makefile: `vehicle-mock` in `build-prod` + `push` | S | ✅ 7 Images statt 6; vehicle-mock.Dockerfile |
| DEV-03 | `cmd/vehicle-mock/main.go` — `session_id` aus ControlCommand → TelemetryEvent + Ack Header | M | ✅ Vollständige Log-Korrelation über alle Kanäle |

### Neue/geänderte Dateien

- `infrastructure/docker/nginx.dev.conf` — neu (HTTP-only, kein SSL)
- `infrastructure/compose/docker-compose.yml` — frontend volumes: nginx.dev.conf mount
- `Makefile` — vehicle-mock in build-prod + push (7 Images)
- `cmd/vehicle-mock/main.go` — sessionID in state; aus ControlCommand.Header extrahiert; in TelemetryEvent.Header + VehicleCommandAck.Header propagiert

### Verification

E2E-Verification PASS — laufender Stack (alle 13 Container), Python-WS-Skripte für Session-Szenarien.

**DEV-01 nginx.dev.conf:**
- `docker compose up --build` → frontend startet ohne SSL-Fehler; Volume-Mount aktiv (`listen 80;`, 0 ssl-Treffer)
- HTTP 200 auf Port 3000; HTTPS Port 3000 → `000 refused` (kein 443-Listener)
- nginx-Proxy-Routen: `/api/health` → 200, `/auth/operator/login` → 200 + JWT, `/api/vehicles` → 200 + JSON, `/ws` → 401 (Auth-Abweisung korrekt)

**DEV-02 Makefile:**
- `vehicle-mock.Dockerfile` existiert; Makefile referenziert es korrekt in `build-prod` (7 Images) und `push`
- Dockerfile kompiliert sauber (Docker golang:1.23-alpine, verifiziert)

**DEV-03 session_id-Propagation:**
- Python-WS-Script: Operator-JWT → WS-Connect → `POST /session/start` → STEER-ControlCommand mit `session_id=01KTZWQSYKAP3AKVE40W3DNP34` → vehicle-mock forwarded → MQTT-TelemetryEvent enthält exakt dieselbe session_id → **PASS**
- VehicleCommandAck-Header ebenfalls mit session_id befüllt (Code-Pfad verifiziert)

**Sprint 12 Vehicle Registry (E2E nachverifiziert):**
- `GET /vehicles` → vehicle-001 auto-geseedet, `online: true`; vehicle-002 angelegt → `online: false`
- `DELETE /vehicles/does-not-exist` → 404 + `"vehicle not found"` ✅
- `POST` malformed JSON → 400 + `"invalid JSON"`; fehlende Felder → 400 + `"id and display_name required"` ✅
- Session-locked DELETE: `DELETE /vehicles/vehicle-001` während aktiver Session → 409 + `"vehicle is currently in active session"`; nach Session-Ende → 204 ✅
- SeedDefault nach control-server-Restart → exakt 1× vehicle-001, kein Duplikat ✅
- SQL-Injection in `id`-Feld → 201 (stored literal), Tabelle intakt (parametrisiertes SQL) ✅

**⚠️ Finding (pre-existing, nicht durch Sprint 12/13 eingeführt):**
`GET/POST/DELETE /vehicles` haben keine JWT-Pflicht — konsistent mit allen anderen REST-Endpoints des control-servers (`/state`, `/session/start` etc.). Schutz besteht nur durch Docker-Netzwerk-Isolation; Port 8080 sollte nie direkt öffentlich exponiert werden.
