# Sprint 7 — Logging & Audit Trail

Abgeschlossen: 2026-06-04

### Tasks

| ID | Task | Typ | Ergebnis |
|----|------|-----|----------|
| LOG-01 | `pkg/logger/` — slog-Wrapper + event_types.go | M | ✅ `logger.New(service)`, `Event(eventType, msg, ...args)`, `Fatal()`, Level via `LOG_LEVEL` ENV, JSON auf stdout |
| LOG-02 | Control Server Migration | M | ✅ Alle `log.Printf` → `svcLog.Info/Event/Warn/Error`; statemachine, safety/detector, command/engine, transport/websocket, session/handover, vehicleconnection migriert |
| LOG-03 | Auth Service Migration | S | ✅ `cmd/auth-service/main.go` — `log.Printf` → `svcLog.Info/Fatal` |
| LOG-04 | Safety Service Migration | S | ✅ `cmd/safety-service/main.go` — Bus-Subscriber loggt via `svcLog.Event` |
| LOG-05 | Telemetry Service Migration | S | ✅ `internal/telemetryservice/client.go` + `cmd/` — MQTT-Events strukturiert |
| LOG-06 | WebRTC SFU Migration | S | ✅ `internal/webrtcsfu/sfu.go` + `cmd/` — alle ICE/Session-Events strukturiert |
| LOG-07 | `POST /log` Endpoint | M | ✅ `cmd/control-server/main.go` — Frontend-Logs mit `service="frontend"` in Loki |
| LOG-08 | Frontend `logger.ts` + Integration | M | ✅ `frontend/src/lib/logger.ts` — fire-and-forget POST /api/log; E-Stop, Operator-Ack, WebRTC-State integriert |
| LOG-09 | Loki + Grafana + Promtail | M | ✅ `infrastructure/loki/`, `infrastructure/promtail/`, `infrastructure/grafana/` + docker-compose Erweiterung; Ports 3100/3001; AVOC Session Dashboard |
| LOG-10 | `pkg/audit/` | M | ✅ `AuditWriter` Interface + `SQLiteAuditWriter` (WAL + fsync, modernc.org/sqlite) + `NoopWriter` |
| LOG-11 | Control Server Safety-Event-Integration | M | ✅ `WithAuditWriter()` auf DeadmanWatchdog, ACKTimeoutWatcher, Engine, WSHandler; `WriteSync()` vor jeder SAFE_MODE-Transition; `GET /audit/events` Endpoint |

### Neue Dateien

- `pkg/logger/logger.go` + `pkg/logger/event_types.go`
- `pkg/audit/writer.go` + `pkg/audit/sqlite_writer.go` + `pkg/audit/noop_writer.go`
- `infrastructure/loki/loki.yml`
- `infrastructure/promtail/promtail.yml`
- `infrastructure/grafana/provisioning/datasources/loki.yml`
- `infrastructure/grafana/provisioning/dashboards/dashboards.yml` + `avoc.json`
- `frontend/src/lib/logger.ts`

### Geänderte Dateien

- `internal/controlserver/statemachine/state.go` — `log.Printf` → slog
- `internal/controlserver/safety/detector.go` — slog + `WithAuditWriter()` + `WriteSync()` vor SAFE_MODE
- `internal/controlserver/command/engine.go` — slog + `WithAuditWriter()` + EMERGENCY_STOP Audit
- `internal/controlserver/transport/websocket.go` — slog + `WithAuditWriter()` + WS_DISCONNECT Audit
- `internal/controlserver/session/handover.go` — `log.Printf` → slog
- `internal/vehicleconnection/handler.go` — `log.Printf` → slog
- `internal/telemetryservice/client.go` — `log.Printf` → slog
- `internal/webrtcsfu/sfu.go` — `log.Printf` → slog
- `cmd/control-server/main.go` — slog + AuditWriter Init + POST /log + GET /audit/events
- `cmd/auth-service/main.go` — `log.Printf` → slog
- `cmd/safety-service/main.go` — `log.Printf` → slog
- `cmd/telemetry-service/main.go` — `log.Printf` → slog
- `cmd/webrtc-sfu/main.go` — `log.Printf` → slog
- `infrastructure/compose/docker-compose.yml` — Loki/Grafana/Promtail + audit-data Volume
- `frontend/src/components/SafetyPanel.tsx` — E-Stop logEvent Integration
- `frontend/src/components/SafeModeOverlay.tsx` — Operator-Ack logEvent Integration
- `frontend/src/hooks/useWebRTC.ts` — WebRTC State logEvent Integration
- `go.mod` — `modernc.org/sqlite v1.34.5` ergänzt

### Testprotokoll (2026-06-04)

| Test-ID | Test | Erwartung | Ergebnis |
|---------|------|-----------|----------|
| T01 | `go mod tidy` — `modernc.org/sqlite v1.34.5` laden | go.sum aktualisiert, kein Fehler | ✅ |
| T02 | `go build ./...` — alle Packages (inkl. pkg/logger, pkg/audit) | `BUILD_OK` | ✅ |
| T03 | Safety Regression (19/19) | Alle grün, JSON-Output sichtbar | ✅ 19/19 |
| T04 | Integration Tests (9/9) | Alle grün | ✅ 9/9 in 0.817s |
| T05 | pkg/logger Smoke-Test | JSON mit `service`, `level`, `event_type`-Feldern | ✅ |
| T06 | pkg/audit NoopWriter | `WriteSync()` returns nil | ✅ |
| T06b | pkg/audit SQLiteAuditWriter | `WriteSync()` + `QueryBySession()` lesen 1 Event | ✅ event_type=DEADMAN_TIMEOUT |
| T07 | Docker Build (alle 5 Go-Services + Frontend) | Alle Images gebaut | ✅ 6 Images |
| T08a | Health Checks (5 Services) | HTTP 200 auf :8080–:8084 | ✅ alle 5 |
| T08b | Structured JSON log output | `{"service":"control-server","event_type":"..."}` auf stdout | ✅ alle 5 Services |
| T08c | Audit Store ready on startup | `audit store ready` im Log mit DB-Pfad | ✅ |
| T08d | `POST /log` Frontend-Log-Ingestion | HTTP 202, Log mit `service="frontend"` in Container-Logs | ✅ |
| T08e | E2E Audit-Pipeline: WS→Session→EMERGENCY_STOP→WriteSync | 1 Event in SQLite, `event_type=EMERGENCY_STOP` | ✅ |
| T08f | `GET /audit/events?session_id=<ulid>` | JSON-Array mit 1 Safety-Event | ✅ |
| T08g | Loki ready + LogQL EMERGENCY_STOP query | 2 Streams, `session_id` als Label extrahiert | ✅ |
| T08h | Grafana API health | HTTP 200 auf Port 3001 | ✅ |
| T09 | Vitest Component Tests (31/31) | Alle grün nach logger.ts-Integration | ✅ 31/31 in 970ms |

### Messwerte

| Metrik | Wert | Ziel |
|--------|------|------|
| Safety Tests | 19/19 ✅ | 19/19 |
| Integration Tests | 9/9 ✅ | 9/9 |
| Vitest Component Tests | 31/31 ✅ | 31/31 |
| Audit WriteSync + fsync (localhost) | < 5ms | < WAL-Commit-Budget |
| LogQL Treffer `event_type=EMERGENCY_STOP` | 2 Streams ✅ | Treffer vorhanden |
| Alle 5 Go-Services Health | 200 ✅ | alle 200 |

### Fixes während der Testphase

1. **`loki.yml` compactor config**: `retention_enabled: true` erfordert `delete-request-store` (Loki v3) → Retention-Config aus Compactor entfernt.
2. **`frontend/package-lock.json`** nicht synchron mit `package.json` (neue Sprint-6-Pakete) → `npm install` regeneriert.
