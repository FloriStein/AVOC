# Backlog

Lifecycle: backlog → sprint → done
Typen: S (<30 Min), M (30–180 Min), L (Architektur, ADR-pflichtig)

Stand: 2026-06-14 — aktualisiert nach Sprint 14 (Security & Observability, AUTH-01/ROB-01/UI-01 ✅)

---

## Abgeschlossen (Referenz)

| ID | Sprint | Beschreibung |
|----|--------|-------------|
| INFRA-01 | 1 | Proto Schemas + CorrelationHeader + ULID |
| FE-01 | 1 | React + TypeScript + Vite + Tailwind Setup |
| BE-01 | 1 | Auth Service JWT (Operator + Vehicle) |
| BE-02 | 1 | Control Server WebSocket + JWT Middleware |
| BE-03 | 1 | Safety Event Bus (In-Memory) |
| BE-11 | 1 | coturn STUN/TURN Setup |
| DC-01 | 1 | Dockerfile Frontend |
| DC-02 | 1 | Dockerfile Backend Services |
| DC-03 | 1 | Docker Compose Orchestrierung |
| TEST-01 | 2 | Go Test Infrastructure (testify + Mocks) |
| TEST-02 | 2 | Safety Test Suite (19/19 Szenarien) |
| BE-06 | 2 | Vehicle Connection Service |
| BE-09 | 2 | Session Manager (GSA) + State Machine Erweiterung |
| BE-10 | 2 | DeadmanWatchdog + ACKTimeoutWatcher |
| BE-12 | 2 | Operator Handover Logic |
| FE-09 | 3 | Protobuf Adapter + Build-Pipeline |
| FE-02 | 3 | WebSocket Client + State-Polling |
| FE-08 | 3 | SAFE MODE Overlay + Operator Ack Flow |
| FE-04 | 3 | Emergency Stop + Dead-man Switch |
| FE-03 | 3 | Connection Status Panel |
| INFRA-02 | 4 | Proto-Gen Fix (module=avoc, korrekte Verzeichnisstruktur) |
| BE-04 | 4 | Command Engine — Protobuf-Parsing, DEADMAN_HOLD/RELEASE, Rate Limiting, ControlAck |
| BE-05 | 4 | MQTT Telemetry Service — Paho v1.4.3, vehicle/+/telemetry Subscribe |
| BE-07 | 4 | Session Recording — Interface + MemoryRecorder, Control Server Integration |
| BE-08 | 4 | WebRTC SFU — Pion/Go, Session Event Consumer, SDP Signaling |

---

## EPIC: Frontend System

| ID | Task | Typ | Status | Notizen |
|----|------|-----|--------|---------|
| FE-05 | Control Panel UI — Joystick, Keyboard, Gamepad | M | ✅ Sprint 5 | `useControls.ts` + `ControlPanel.tsx` |
| FE-06 | Video Stream Panel — WebRTC Multi-Kamera UI | M | ✅ Sprint 5 | `useWebRTC.ts` + `VideoPanel.tsx` + SFU Track-Fix |
| FE-07 | Teleoperation Dashboard — Finales Layout & Integration | M | ✅ Sprint 5 | `App.tsx` + `useTelemetry.ts` + Dashboard integriert |

---

## EPIC: Containerization

| ID | Task | Typ | Abhängigkeiten | Notizen |
|----|------|-----|----------------|---------|
| DC-04 | Local Dev Environment — README finalisieren | S | ✅ Sprint 6 | Troubleshooting (6 Szenarien), Contributor Guide (5 Abschnitte), alle Makefile-Befehle |

---

## EPIC: Testing

| ID | Task | Typ | Status | Ergebnis |
|----|------|-----|--------|----------|
| TEST-03 | Integration Test Infrastructure — Docker Test Environment | M | ✅ Sprint 6 | docker-compose.test.yml, 9 Go Integration Tests, make test-integration |
| TEST-04 | Frontend Test Infrastructure — Vitest + RTL + Playwright | M | ✅ Sprint 6 | 31/31 Vitest Tests grün, playwright.config.ts, E2E Baseline |
| TEST-05 | Performance / Latency Tests — CI Integration (<100ms) | M | ✅ Sprint 6 | Go Benchmark p99=0ms, k6 p99=244µs, make test-latency + make test-k6 |

---

---

## EPIC: Logging (ADR-017/018)

| ID | Task | Typ | Status | Ergebnis |
|----|------|-----|--------|----------|
| LOG-01 | `pkg/logger/` — slog-Wrapper + event_types.go | M | ✅ Sprint 7 | `logger.New(service)`, `Event()`, JSON stdout, `LOG_LEVEL` ENV |
| LOG-02 | Control Server Migration | M | ✅ Sprint 7 | statemachine, safety, command, transport, session, vehicleconnection migriert |
| LOG-03 | Auth Service Migration | S | ✅ Sprint 7 | cmd/auth-service/main.go — structured JSON |
| LOG-04 | Safety Service Migration | S | ✅ Sprint 7 | cmd/safety-service/main.go — Bus-Events via Event() |
| LOG-05 | Telemetry Service Migration | S | ✅ Sprint 7 | telemetryservice/client.go — MQTT-Events strukturiert |
| LOG-06 | WebRTC SFU Migration | S | ✅ Sprint 7 | webrtcsfu/sfu.go — ICE/Session-Events strukturiert |
| LOG-07 | `POST /log` Endpoint — Frontend Log-Ingestion | M | ✅ Sprint 7 | HTTP 202, `service="frontend"` in Loki |
| LOG-08 | Frontend `logger.ts` + Integration | M | ✅ Sprint 7 | fire-and-forget; E-Stop, Operator-Ack, WebRTC integriert |
| LOG-09 | Loki + Grafana + Promtail Docker Compose | M | ✅ Sprint 7 | Ports 3100/3001; Docker-Label-Discovery; AVOC Session Dashboard |
| LOG-10 | `pkg/audit/` — AuditWriter + SQLiteAuditWriter | M | ✅ Sprint 7 | WAL+fsync, NoopWriter, QueryBySession(), modernc.org/sqlite |
| LOG-11 | Control Server Safety-Event-Integration | M | ✅ Sprint 7 | WriteSync() vor SAFE_MODE in detector.go/engine.go/websocket.go; GET /audit/events |

**Abhängigkeitspfad:**
```
LOG-01 → LOG-02..06 (parallel) → LOG-07 → LOG-08
LOG-01 → LOG-09 (parallel zu allem)
LOG-10 → LOG-11 (nach LOG-02)
```

---

## EPIC: Deployment (Sprint 8) ✅

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|----------------|
| DEPLOY-01 | ADR-019 — Deployment-Strategie (Docker Hub + EC2 + SSM) | L | ✅ Sprint 8 | — |
| DEPLOY-02 | Makefile `build-prod` + `push` — linux/amd64, Docker Hub Tags | M | ✅ Sprint 8 | DEPLOY-01 |
| DEPLOY-03 | `docker-compose.prod.yml` — `image:` statt `build:`, prod Konfiguration | M | ✅ Sprint 8 | DEPLOY-02 |
| DEPLOY-04 | `scripts/setup-ssm.sh` + `scripts/deploy.sh` — SSM-Integration | M | ✅ Sprint 8 | DEPLOY-01 |
| DEPLOY-05 | coturn EC2-Konfiguration — `external-ip` via `TURN_EXTERNAL_IP` | M | ✅ Sprint 8 | DEPLOY-01 |
| DEPLOY-06 | Grafana Security — Login-Form + Admin-Credentials aus SSM | S | ✅ Sprint 8 | DEPLOY-03 |
| DEPLOY-07 | EC2 Bootstrap Guide — Checkliste für ersten Deploy ab null | M | ✅ Sprint 8 | DEPLOY-03, DEPLOY-04, DEPLOY-05 |
| DEPLOY-08 | `fleet-service` fehlt in `docker-compose.prod.yml` (gebaut laut Makefile `GO_SERVICES`, aber nie in Prod-Compose ergänzt) | M | ✅ Sprint 36 | Service-Block ergänzt, analog `auth-service`/`telemetry-service`-Muster. Details in `tasks/sprints/36-restposten-bereinigung-iii.md` |

---

## EPIC: Video Stream (Sprint 9) ✅

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|----------------|
| STREAM-01 | ADR-020 — MediaMTX als WHIP/WHEP Router | L | ✅ Sprint 9 | — |
| STREAM-02 | `infrastructure/mediamtx/mediamtx.yml` + Docker Service | M | ✅ Sprint 9 | STREAM-01 |
| STREAM-03 | nginx: `/whep/` Proxy | S | ✅ Sprint 9 | STREAM-02 |
| STREAM-04 | `useWebRTC.ts` → WHEP-Protokoll + vehicleId-Prop | M | ✅ Sprint 9 | STREAM-02 |
| STREAM-05 | Control Server: `/internal/media/auth` + SAFE_MODE → MediaMTX API | M | ✅ Sprint 9 | STREAM-02 |
| STREAM-06 | TURN in MediaMTX ICE-Config + Compose env | S | ✅ Sprint 9 | STREAM-02 |
| STREAM-07 | CDK Port 8889 + SSM `whip-stream-key` + setup-ssm.sh | S | ✅ Sprint 9 | — |
| STREAM-08 | `docker-compose.prod.yml`: mediamtx + deploy.sh Update | S | ✅ Sprint 9 | STREAM-02 |
| STREAM-09 | Larix Setup Guide + E2E Smoke Test | S | ✅ Sprint 9 | STREAM-07, STREAM-08 |

---

## EPIC: Vehicle Registry (Sprint 12) ✅

| ID | Task | Typ | Status | Ergebnis |
|----|------|-----|--------|----------|
| VEH-REG-01 | ADR-022 — Vehicle Registry Architecture | L | ✅ Sprint 12 | `docs/adr/022-vehicle-registry.md` |
| VEH-REG-02 | `pkg/audit/sqlite_writer.go` — `DB() *sql.DB` getter | S | ✅ Sprint 12 | Shared WAL-Connection für vehicleregistry |
| VEH-REG-03 | `internal/vehicleregistry/` — VehicleStore, SQLiteVehicleStore, NoopVehicleStore | M | ✅ Sprint 12 | ErrNotFound-Sentinel; ConnectionChecker Interface |
| VEH-REG-04 | `cmd/control-server/main.go` — Store-Init + `GET/POST/DELETE /vehicles` | M | ✅ Sprint 12 | SeedDefault vehicle-001; 404 bei DELETE nicht-existent; 409 bei aktiver Session |
| VEH-REG-05 | `frontend/src/lib/api-client.ts` + `useVehicles.ts` | S | ✅ Sprint 12 | `VehicleInfo`, `listVehicles()`, 2s-Polling Hook |
| VEH-REG-06 | `frontend/src/hooks/useSession.ts` — `startSession(vehicleId)` | M | ✅ Sprint 12 | VEHICLE_ID-Hardcoding entfernt; expliziter Operator-Entscheid |
| VEH-REG-07 | `frontend/src/components/VehicleSelector.tsx` | M | ✅ Sprint 12 | Dropdown mit Online-Indikator + "Session starten"-Button |
| VEH-REG-08 | `SafetyPanel.tsx` + `ControlPanel.tsx` + `ConnectionPanel.tsx` + `App.tsx` | M | ✅ Sprint 12 | vehicleId-Prop-Chain; Auto-Start entfernt |

---

## EPIC: Vehicle Connectivity & Feedback (Sprint 11) ✅

| ID | Task | Typ | Status | Ergebnis |
|----|------|-----|--------|----------|
| VEH-01 | ADR-021 — Vehicle Connectivity & Feedback Architecture | L | ✅ Sprint 11 | `docs/adr/021-vehicle-connectivity-feedback.md` |
| VEH-02 | `proto/vehicle.proto` — VehicleCommandAck | S | ✅ Sprint 11 | Protobuf: header + command_event_id + received + received_at_ms |
| VEH-03 | `proto/telemetry.proto` — Actuation Fields 7–11 | S | ✅ Sprint 11 | steer/throttle/brake commanded + actual |
| VEH-04 | `internal/vehicleconnection/registry.go` — Registry + ForwardCommand | M | ✅ Sprint 11 | VehicleForwarder-Interface-Implementierung; thread-safe |
| VEH-05 | `internal/vehicleconnection/ackstore.go` — AckStore | S | ✅ Sprint 11 | Latest-ACK je vehicleID, sync.RWMutex |
| VEH-06 | `internal/controlserver/command/engine.go` — VehicleForwarder | M | ✅ Sprint 11 | Interface + WithVehicleForwarder(); kritischer Gap geschlossen |
| VEH-07 | `cmd/control-server/main.go` — Verdrahtung + ACK-Endpoint | M | ✅ Sprint 11 | `GET /vehicle/ack/latest/{vehicleID}` |
| VEH-08 | `cmd/vehicle-mock/main.go` — Docker-Service | L | ✅ Sprint 11 | JWT self-gen, WS, Protobuf decode, ACK send, MQTT lerp 15% |
| VEH-09 | `vehicle-mock.Dockerfile` + Compose + nginx | M | ✅ Sprint 11 | `/vehicle/` Proxy-Location, docker-compose dev+prod |
| VEH-10 | `useVehicleAck.ts` — Frontend Hook | S | ✅ Sprint 11 | 500ms-Polling `/vehicle/ack/latest/` |
| VEH-11 | `InputIndicatorPanel.tsx` — Fahrzeug-Feedback UI | M | ✅ Sprint 11 | Lenkrad-SVG (steerActual×120°), ActuationBars, AckBadge |
| VEH-12 | Tests: 7 Go + 7 TypeScript | M | ✅ Sprint 11 | 26/26 Go Unit, 41/41 Frontend Tests grün |

---

## EPIC: WebRTC ICE Migration (Sprint 10) ✅

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|----------------|
| WEBRTC-01 | CDK Security Group: 3478 TCP/UDP, 8189 UDP, 49152–65535 UDP | S | ✅ Sprint 10 | — |
| WEBRTC-02 | `mediamtx.yml`: `webrtcIPsFromInterfaces: false`, ICEServers2 entfernen, Port 8189 | S | ✅ Sprint 10 | — |
| WEBRTC-03 | coturn `network_mode: host`, `relay-ip`, `external-ip=PUBLIC/PRIVATE` | M | ✅ Sprint 10 | WEBRTC-01 |
| WEBRTC-04 | mediamtx UDP-Port 8889 → 8189 | S | ✅ Sprint 10 | WEBRTC-02 |
| WEBRTC-05 | `deploy.sh`: `TURN_PRIVATE_IP` aus EC2 IMDS (IMDSv2) | S | ✅ Sprint 10 | WEBRTC-03 |
| WEBRTC-06 | control-server: `GET /ice-config` Endpoint | M | ✅ Sprint 10 | — |
| WEBRTC-07 | control-server env: `TURN_USER`, `TURN_PASSWORD`, `TURN_EXTERNAL_IP` | S | ✅ Sprint 10 | WEBRTC-06 |
| WEBRTC-08 | `useWebRTC.ts`: DTLS-Fix, `/api/ice-config` fetch, 5s Gathering-Timeout | M | ✅ Sprint 10 | WEBRTC-06 |
| WEBRTC-09 | Deploy auf EC2 + E2E Smoke Test | M | ✅ Sprint 10 (E2E offen) | WEBRTC-01–08 |

---

## EPIC: Security & Observability (Sprint 14) 🔄

| ID | Task | Typ | Status | Ergebnis |
|----|------|-----|--------|----------|
| AUTH-01 | JWT-Pflicht REST-Endpoints (`requireJWT` Middleware, 9 Endpoints geschützt) | M | ✅ Sprint 14 | `cmd/control-server/main.go`; `api-client.ts` + `SafetyPanel.tsx` token-aware |
| ROB-01 | Backend nicht erreichbar Banner + ControlPanel-Sperre | S | ✅ Sprint 14 | `useSystemState.ts` `unreachable`; rotes Banner in `App.tsx` |
| UI-01 | Dual-Channel Status: Control (WS-ACK) + Video (ICE-RTT) in ConnectionPanel | M | ✅ Sprint 14 | `useWebRTC.ts` `getStats()`; `VideoPanel.tsx` Callback; `ConnectionPanel.tsx` 2 Zeilen |
| OBS-01 | Vehicle "zuletzt gesehen" Heartbeat-Timestamp in AckBadge (Bonus) | S | ✅ Sprint 31 | `useTelemetry.ts` liefert jetzt `timestampMs`/`ageSinceUpdateMs`; `InputIndicatorPanel.tsx`'s `AckBadge` zeigt "Zuletzt gesehen vor Xs" unabhängig vom ACK-Kommandofluss |

**Nachtrag Bugfix:** `WSClient.disconnect()` setzt `ws.onclose = null` vor `ws.close()` — E-Stop Race Condition behoben.

---

## EPIC: Multi-Vehicle State Isolation (Sprint 17)

| ID | Task | Typ | Status | Referenz |
|----|------|-----|--------|----------|
| MV-01..08 | Backend + Frontend vollständig — siehe `tasks/current-sprint.md` Sprint 17 | M | ✅ | [ADR-026](../docs/adr/026-multi-vehicle-state-isolation.md) |

**Folge-Tasks (bewusst aus ADR-026 ausgeklammert):**

| ID | Task | Typ | Status | Notizen |
|----|------|-----|--------|---------|
| MV-09 | Vehicle-Dropdown Live-State-Badge (z.B. SAFE_MODE-Indikator neben Vehicle-002) | M | ✅ Sprint 30 | `GET /vehicles` liefert jetzt `system_state`; `VehicleSelector.tsx` zeigt 🔴 SAFE_MODE-Badge |
| MV-10 | GC für `VehicleContext`-Instanzen bei großer/dynamischer Flotte | M | 🔲 Backlog | Aktuell dauerhaft behalten (kleine Flotte) — nur relevant bei deutlichem Flottenwachstum. Sprint-30-Triage 2026-07-18: bewusst zurückgestellt |
| MV-11 | Multi-Vehicle Handover — `HandoverManager` nutzt noch eine einzelne globale State Machine (`handoverSM`) für die OPERATOR-Schicht | M | ✅ bereits erledigt (Commit `f68346a`, 2026-07-15) | Bei Sprint-30-Bearbeitung 2026-07-18 als bereits behoben vorgefunden — `HandoverManager` löst seitdem pro Fahrzeug über `vehiclecontext.Registry` auf, Regressionstest `TestSafety_Handover_TwoVehicles_IndependentHandovers` in `tests/unit/safety_test.go`. Diese Zeile war nur nicht nachgepflegt worden |
| MV-12 | `GET /state` entfernen, verbleibende Konsumenten (`latency.js`, `services_test.go`) auf `GET /vehicles/{id}/state` / `GET /sessions` migrieren | S | ✅ Sprint 30 | `GET /state`/`handleState` entfernt, beide Konsumenten migriert, gegen echten Docker-Test-Stack verifiziert |
| AUTH-TEST-01 | `internal/authservice/handler_test.go` komplett veraltet (368 Zeilen, alte string-ID/`DisplayName`-API) — kompiliert nicht mit `go vet`/`go test` | M | ✅ Sprint 18 (AUTH-18-01) | 20 Tests auf neue API migriert (`User.ID int`, `Username`, neue `UserStore`-Signaturen); `go vet ./...` + alle Unit-Tests grün |

---

## EPIC: Lokaler Dev-Stack Verifikation (Sprint 19) ✅

| ID | Task | Typ | Status | Referenz |
|----|------|-----|--------|----------|
| LOCAL-01..05 | Full-Rebuild, SSL-Frage, Hot-Reload, WHIP/WHEP-DNS-Fix, Doku — siehe `tasks/current-sprint.md` Sprint 19 | S/M | ✅ | |

**Folge-Task (bewusst aus Sprint 19 ausgeklammert):**

| ID | Task | Typ | Status | Notizen |
|----|------|-----|--------|---------|
| WEBRTC-10 | SDP `a=setup:active`-Workaround (`useWebRTC.ts`, `useWHIPSender.ts`) inkompatibel mit aktuellem Chromium (`setRemoteDescription` wirft „Offerer must use actpass") | M/L | ✅ Sprint 32 | Technische Prüfung: `mediamtx:latest` (`v1.18.1`) baut gegen `pion/webrtc v4.2.12`, komplett andere Codebasis als das referenzierte `v1.19.0`. Workaround entfernt, Offer bleibt Standard-`actpass`. Lokal gegen echten `mediamtx:latest`-Container verifiziert (isolierter Pion-WHIP-Client, `docs/webrtc.md`) — DTLS/ICE-Handshake erreicht zuverlässig `Connected`. Kein eigenes ADR nötig (reiner Implementierungs-Hack, keine Architekturentscheidung berührt). Produktiv-Verifikation mit echtem Chromium auf AWS bleibt offen (`CONTEXT.MD`) |
| DOC-01 | `frontend/README.md` seit MediaMTX-WHIP/WHEP-Migration (Sprint 9/10) nicht mehr gepflegt | S | ✅ Sprint 30 | Proxy-Tabelle, `useWebRTC`-Beschreibung (WHEP statt SFU), Komponenten-/Hooks-/Lib-Liste und Funktionsumfang (Nutzerverwaltung, Fleet Dashboard) auf aktuellen Stand gebracht |
| DOC-02 | Kompilierte `control-server`-Binärdatei (~12 MB) liegt seit dem allerersten Commit im Repo-Root und ist versioniert — nicht durch `.gitignore` erfasst (nur `bin/` ist ausgeschlossen) | S | ✅ Sprint 30 | Keine Referenz im Repo gefunden. Per `git rm --cached` aus dem Tracking entfernt (lokale Datei bleibt), `.gitignore` ergänzt. Bewusst **keine** History-Rewrite |

---

## EPIC: Fleet Dashboard Planung (ADR-027/028/029, IBATOUR)

Grill-Me-Sessions 2026-07-10 bis 2026-07-14 haben den Kurswechsel Richtung IBATOUR-Leitstelle
geklärt (Autonomy-First-Betrieb, Fleet-Datenmodell, Notfall-Trigger). Details in
`docs/requirements.md`, `CONTEXT.MD`, `docs/adr/027-029`. Konkrete Sprint-Tasks (fleet-service
aufsetzen, Multi-Vehicle-Simulation, Dashboard-Komponenten) folgen in `tasks/current-sprint.md`,
sobald die Architektur final abgestimmt ist.

**Bewusst zurückgestellte Folge-Entscheidungen:**

| ID | Task | Typ | Status | Notizen |
|----|------|-----|--------|---------|
| FLEET-01 | Handshake-basierte Autonomie-Rückgabe (statt einfachem `endSession()`) | M | 🔲 Backlog | `ADR-028` — Sprint-32-Triage 2026-07-18: kein neuer Anlass zur Revision bekannt, bewusst weiter zurückgestellt; Risiko: Fahrzeug könnte Kontrolle zurückerhalten, bevor es sicher verarbeitet ist. `endSession()` reicht für den Anfang |
| FLEET-02 | Persistenzform für gefahrene Route (Historie) — Zeitreihen-DB vs. einfache Tabelle | M | ✅ Sprint 32 | [ADR-033](../docs/adr/033-vehicle-position-history.md) — einfache `vehicle_position_history`-Tabelle (kein Zeitreihen-DB-Dienst), gedrosselter Schreibpfad (min. 10s), 30-Tage-Retention; identisch mit `AP2-03` umgesetzt |
| FLEET-03 | Bestehende `POST /vehicles`/`DELETE /vehicles/{id}` in `control-server` vs. neue `fleet-service`-Admin-API — Ablösung oder Koexistenz | S | ✅ Sprint 31 | Reine Bestätigungsaufgabe, kein Code: `ADR-029` beantwortet die Frage bereits abschließend (Koexistenz, kein Bruch), Endpoints unverändert verifiziert |
| FLEET-04 | `vehicle-mock`s Fleet-Simulation nutzt hartcodierte Demo-Stationskoordinaten (`cmd/vehicle-mock/fleet_simulator.go`) statt echter Zonen/Stationen | S | ✅ Sprint 31 | Neue `cmd/vehicle-mock/fleet_stations.go`: `resolveSimulationStations()` ruft `GET /fleet/stations` ab, fällt bei Fehler/<2 geo-verorteten Stationen auf `fallbackDemoStations` zurück |
| TASKUI-01 | Doppelte Demo-Stationsanlage bereinigen (`demo-zone-taskui`/`demo-station-*-taskui` aus Sprint 24 vs. `zone-betriebshof-nord`/echte Stationen aus der parallelen Sprint-23-Session) | S | ✅ Sprint 30 | `demoStationSeed` (INSERT) durch `taskuiDemoSeedCleanup` (DELETE, FK-sicher via `NOT EXISTS`-Guard) ersetzt |
| TASKUI-02 | Task-Status-Übergangstabelle dupliziert (Go `internal/fleetservice/store.go` `taskTransitionSources` vs. TypeScript `frontend/src/components/FleetTaskPanel.tsx` `NEXT_TRANSITIONS`) | S | ✅ Sprint 30 | Server-berechnetes `Task.AllowedTransitions`-Feld ersetzt die hartcodierte Frontend-Kopie; `FleetTaskPanel.tsx` rendert Buttons jetzt aus `task.allowed_transitions` |
| TASKUI-03 | Vollständige Task-Status-Audit-Historie (mehrere Übergänge, nicht nur der letzte) | M | ✅ Sprint 31 | [ADR-032](../docs/adr/032-task-status-history.md) — neue `task_status_history`-Tabelle (additiv, `ADR-030` unverändert), `GET /fleet/tasks/{id}/history`, Backfill bestehender Tasks mit dokumentierten Näherungen |
| TASKUI-04 | Nil-Slice-→-JSON-`null`-Muster in `ListZones`/`ListStations`/`ListVehicleStatus`/`ListVehiclesWithStatus`/`ListAlerts` (`internal/fleetservice/store.go`) | S | ✅ Sprint 30 | Gleiches `var x []T` → `x := []T{}`-Muster wie bei `ListTasks` angewendet |
| TASKUI-05 | Geteiltes Docker-Compose-Projekt über alle Worktrees hinweg (`name: avoc` in `infrastructure/compose/docker-compose.yml`) | M | ✅ Sprint 30 | `COMPOSE_PROJECT_NAME` im `Makefile` aus dem Worktree-Verzeichnisnamen abgeleitet und exportiert (Datei-`name:` unverändert, env var hat Vorrang) |

---

## EPIC: AP1 — Einarbeitung & Architekturplanung (Meilenstein 1, IBATOUR)

Laut Leistungsbeschreibung umfasst AP1 zwei Teile: (1) "Einarbeitung in vorhandene
Programmierschnittstellen für ausgewählte Fahrzeuge" und (2) "Design der Softwarearchitektur und
Schnittstellen" (siehe `ADR-027` Kontext). Teil (2) ist inhaltlich weitgehend erledigt — verteilt
über `docs/vision.md`, `docs/requirements.md`, `CONTEXT.MD`, `docs/architecture.md`
(Fleet-Abschnitt nachgezogen 2026-07-16) und `ADR-027/028/029`. Teil (1) ist **nicht** erledigt und
kann laut denselben Dokumenten nicht ohne den vertraglich vorgesehenen Workshop mit der Professur
Logistik abgeschlossen werden — bis dahin läuft alles gegen eine unbestätigte Annahme
(`FleetGateway`/`MockGateway`, ADR-027). Das ist der Grund, warum die konkrete ROS2/DDS-Schnittstelle
projektweit als offener Punkt geführt wird (`DECISIONS.MD`, `CONTEXT.MD` Offene Fragen).

**Wichtiger Vorbehalt (keine Repo-Quelle, nur Rückschluss aus der Leistungsbeschreibungs-Erwähnung
in ADR-027):** Ob Meilenstein 1 formal ein eigenständiges, dem Auftraggeber zu übergebendes
Architektur-Dokument verlangt, oder ob die bereits bestehende, über mehrere Dateien verteilte
Dokumentation für die Abnahme ausreicht, ist im Repo nirgends explizit festgehalten. AP1-04 unten
ist deshalb als zu klärender Punkt markiert, nicht als sicher notwendige Aufgabe.

| ID | Task | Typ | Status | Notizen |
|----|------|-----|--------|---------|
| AP1-01 | Workshop-Termin mit der Professur Logistik vereinbaren + durchführen — Klärung der konkreten ROS2-Topics, Nachrichtenformate, DDS-vs-ROSbridge-Frage, sowie der vorhandenen Programmierschnittstellen der ausgewählten Fahrzeuge (Lastenrad/Lastenzug) | L | 🔲 Backlog | Externe Abhängigkeit (Termin mit AG), kein Code-Task; vertraglich Teil von AP1 (`ADR-027`); blockiert AP1-02/AP1-03 sowie die "Konkrete ROS2/DDS-Schnittstelle"-Zeile in `DECISIONS.MD`/`CONTEXT.MD` |
| AP1-02 | `FleetGateway`-Interface (`ADR-027`) gegen die im Workshop geklärte reale Schnittstelle abgleichen; bei signifikanter Abweichung eigenes neues ADR (ADR-027 nicht überschreiben) | M | 🔲 Backlog | Abhängigkeit: AP1-01 |
| AP1-03 | Konkreten Adapter für die reale ROS2/DDS-Schnittstelle implementieren (`MockGateway` bleibt zusätzlich als Test-Doppel erhalten, wird nicht ersetzt) | L | 🔲 Backlog | Abhängigkeit: AP1-02; betrifft ausschließlich `internal/fleetgateway`, keine Kopplung zu `control-server`/Safety-Domäne laut ADR-029 |
| AP1-04 | Klären, ob Meilenstein 1 ein eigenständiges Architektur-Liefer-Dokument für den Auftraggeber erfordert; falls ja, bestehende Docs (`docs/vision.md`, `docs/requirements.md`, `CONTEXT.MD`, `docs/architecture.md`, `ADR-027/028/029`) zu einem auftraggebertauglichen Dokument konsolidieren | S/M | ✅ Sprint 32 | Nutzerentscheidung 2026-07-18: eigenständiges Dokument gewünscht. [docs/milestones/meilenstein-1-architektur.md](../docs/milestones/meilenstein-1-architektur.md) — weist explizit darauf hin, dass Meilenstein 1 wegen des noch ausstehenden Workshops (`AP1-01`) nicht vollständig abnahmefähig ist |

**Abhängigkeitspfad:**
```
AP1-01 → AP1-02 → AP1-03
AP1-04 unabhängig, aber Klärung vor Bearbeitung nötig
```

---

## EPIC: AP2 — Web-Dashboard (Meilenstein 2, IBATOUR)

Leistungsbeschreibung AP2 umfasst vier Anforderungsbereiche (siehe `docs/requirements.md`
"Fleet-Domänenkonzepte"). Status je Bereich, Stand dieser Analyse (2026-07-16), **branchübergreifend**
recherchiert (mehrere parallele Worktrees arbeiteten laut `git worktree list` gleichzeitig an AP2):

| Bereich | Status | Referenz |
|---|---|---|
| Flottenübersicht (Echtzeit-Status, Batterie, Alert-Anzeige) | ✅ fertig, gemergt | Sprint 22 (DASH-01..08) |
| Kartenansicht — Outdoor | ✅ fertig, gemergt | Sprint 23 (MAP-01..11) |
| Kartenansicht — Indoor | ✅ fertig, Sprint 33 | `AP2-02` — [ADR-034](../docs/adr/034-indoor-vehicle-position.md), `vehicle_status.position_x/y` + `FleetIndoorMap.tsx` |
| Routenübersicht (gefahrene Historie) | ✅ fertig, Sprint 32 | `FLEET-02`/`AP2-03` — [ADR-033](../docs/adr/033-vehicle-position-history.md) |
| Task Management (Erstellen/Zuweisen/Status/Historie) | ✅ fertig, gemergt | Sprint 24 (`TASK-01..14`, `ADR-030`) |
| Alert System — Basis (Echtzeit, Priorität, Ack) | ✅ fertig, gemergt | Sprint 22 |
| Alert System — Audio-Benachrichtigung | ✅ fertig, gemergt | Sprint 25 (`AUDIO-01..06`) |

**ADR-Nummernkollision — gelöst (2026-07-16):** Dieser Branch (`feature/fleet-service-foundation-hexarch`)
hatte `docs/adr/030-hexagonal-architecture-migration.md` angelegt, unabhängig davon der `taskui`-Branch
am selben Tag `docs/adr/030-task-status-lifecycle.md` unter derselben Nummer. Beim Merge aufgelöst:
`taskui`s ADR-030 bleibt, dieses ADR wurde auf **ADR-031** verschoben
(`docs/adr/031-hexagonal-architecture-migration.md`), inkl. Anpassung von `DECISIONS.MD`/`CONTEXT.MD`.

| ID | Task | Typ | Status | Notizen |
|----|------|-----|--------|---------|
| AP2-01 | Merge-Integration: Sprint 24 (`-taskui`) und Sprint 25 (`-audio`) in `feature/fleet-service-foundation` zusammenführen (Sprint 23 ist bereits gemergt) | L | ✅ erledigt | ADR-030-Nummernkollision gelöst (siehe oben); verbleibende Merge-Konflikte (doppelte Demo-Stationsanlage `TASKUI-01`, additive `FleetOverview.tsx`-Änderungen aus Sprint 23/24/25) beim Merge selbst aufgelöst |
| AP2-02 | Indoor-Kartenrendering — Backend-Erweiterung für Fahrzeug-Punktposition innerhalb einer Indoor-Zone | L | ✅ Sprint 33 | Grill-Me 2026-07-18 + [ADR-034](../docs/adr/034-indoor-vehicle-position.md) — `vehicle_status.position_x/y`, analog `stations`. Zerlegt in 4 S/M-Teilaufgaben (siehe unten), Simulator-Befüllung bewusst ausgeklammert (Folge-Task) |
| AP2-02-01 | `vehicle_status.position_x/y`-Migration (`internal/fleetservice/store.go`) | S | ✅ Sprint 33 | ADR-034 |
| AP2-02-02 | `UpsertVehicleStatus`/`GET`-Endpunkte um `position_x/y` erweitern | S | ✅ Sprint 33 | AP2-02-01 |
| AP2-02-03 | Indoor-Beispielzone + Stationen im Seed-Skript (`scripts/seed-fleet-demo.sh`) | S | ✅ Sprint 33 | — |
| AP2-02-04 | Frontend Indoor-Kartendarstellung (SVG-Koordinatensystem, kein Leaflet) | M | ✅ Sprint 33 | AP2-02-02, AP2-02-03 |
| AP2-03 | Persistenzform für "gefahrene Route" entscheiden + implementieren | M | ✅ Sprint 32 | Verweist auf `FLEET-02` — identisch umgesetzt, siehe dort und [ADR-033](../docs/adr/033-vehicle-position-history.md) |
| AP2-04 | Prioritätenmanagement in der Task-UI über reine Zahlenanzeige hinaus ausbauen (Sortierung nach Priorität, visuelle Hervorhebung hoher Priorität) | M | ✅ Sprint 31 | `FleetTaskPanel.tsx`: `sortedTasks` (absteigend nach `priority`), Hervorhebung ab `priority >= 5` (fester Schwellenwert, Grill-Me 2026-07-18) |
| AP2-05 | Klären, ob Meilenstein 2 ein eigenständiges Abnahme-Artefakt für den Auftraggeber braucht | S | ✅ Sprint 32 | Nutzerentscheidung 2026-07-18: eigenständiges Dokument gewünscht (analog `AP1-04`). [docs/milestones/meilenstein-2-dashboard.md](../docs/milestones/meilenstein-2-dashboard.md) — wies zum Sprint-32-Stand auf `AP2-02` (Indoor-Kartenrendering) als einzigen noch offenen Punkt hin; `AP2-02` inzwischen Sprint 33 ✅, Dokument nachgezogen |

**Nicht dupliziert, sondern nur referenziert (bereits auf den jeweiligen Branches dokumentiert,
kommen beim Merge mit):** `TASKUI-01..05` (`taskui`-Branch-`backlog.md`: doppelte Demo-Stationen,
Übergangstabellen-Duplikation Backend/Frontend, fehlende volle Task-Audit-Historie, `null`-statt-`[]`
in mehreren `fleet-service`-List-Endpunkten, geteiltes Docker-Compose-Projekt über alle Worktrees
hinweg) sowie die im `audio`-Branch als "bewusst nicht in diesem Sprint" genannten optionalen
Sound-Erweiterungen (konfigurierbare Lautstärke/Tonhöhe, Snooze, Severity-spezifische Töne).

**Abhängigkeitspfad:**
```
AP2-01 (Merge, inkl. ADR-030-Konflikt, erledigt) → AP2-04
AP2-02, AP2-03, AP2-05 unabhängig von AP2-01
```

---

## EPIC: Hexagonale Architektur-Migration — Pilot fleet-service (ADR-031)

Strategie und vollständige Risikobewertung: [ADR-031](../docs/adr/031-hexagonal-architecture-migration.md).
Grill-Me 2026-07-16: Auslöser ist allgemeine Wartbarkeit/Onboarding (nicht ADR-027/ROS2), Scope nur
Go-Backend, Pilot bestätigt `fleet-service`, Start ursprünglich **nachgelagert nach AP2/AP3**. Umfang:
ausschließlich der fehlende `FleetStore`-Repository-Port (`internal/fleetservice/handler.go:32,37` →
Interface, analog zu `fleetgateway.FleetGateway`), keine Use-Case-Schicht-Extraktion in diesem
Schritt (siehe HEX-06).

**Start-Entscheidung (Sprint-33-Kickoff, 2026-07-18):** AP2 ist zu diesem Zeitpunkt zu 6 von 7
Anforderungsbereichen fertig (nur `AP2-02` noch offen, jetzt selbst Teil von Sprint 33), AP3 ist im
Repo weiterhin nicht als eigenes EPIC angelegt. Nutzerentscheidung: HEX-01..05 trotzdem jetzt
starten — die Tasks sind bereits einzeln klein geschnitten (S/M) und laut ADR-031 unabhängig von
AP3-Inhalten (reiner Repository-Port für `fleet-service`). HEX-01..05 daher nach
`tasks/current-sprint.md` (Sprint 33) verschoben, siehe dort für Ergebnisse. HEX-06 (optional)
bleibt hier als expliziter Entscheidungspunkt nach HEX-05.

**Arbeitsweise (bewusst kleinteilig geschnitten):** Jeder Task ist einzeln abschließbar und endet
mit einem Update dieses Eintrags (Status ✅ + Kurzergebnis) **bevor** die Session beendet/geclear't
wird — Definition-of-Done schließt die Doku-Aktualisierung ein, nicht nur den Code. Damit reicht
für eine neue Session pro Task ein Verweis auf die Task-ID hier, keine lange Übergabe nötig.

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| HEX-01 | `FleetStore`-Interface definieren (deckt alle Methoden von `*PostgresFleetStore` ab) + Compile-Time-Check `var _ FleetStore = (*PostgresFleetStore)(nil)`. Kein Verhaltens-, kein Signatur-Wechsel an `Handler` in diesem Schritt. | S | ✅ Sprint 33 | — |
| HEX-02 | `Handler.store` von `*PostgresFleetStore` auf `FleetStore`-Interface umstellen (`internal/fleetservice/handler.go:32,37`). Reine Typ-Änderung, HTTP-Verhalten unverändert — bestehende Postgres-gebundene Tests bleiben vorerst grün (noch kein Fake). | S | ✅ Sprint 33 | HEX-01 |
| HEX-03 | `FakeFleetStore` (In-Memory) implementieren, das `FleetStore` erfüllt — Test-Doppel für Handler-Tests ohne Postgres. | M | ✅ Sprint 33 | HEX-01 |
| HEX-04 | Bestehende Handler-Tests (ursprünglich auf ~20 der 24 geschätzt, tatsächlich 30 `DATABASE_URL`-gebundene Tests) auf `FakeFleetStore` migriert. | M | ✅ Sprint 33 | HEX-02, HEX-03 |
| HEX-05 | Verifikation: `go test ./internal/fleetservice/...` läuft grün **ohne** gesetzte `DATABASE_URL`. ADR-031-Status-Update (Pilot abgeschlossen) + Status-Update in diesem Eintrag + `DECISIONS.MD`. | S | ✅ Sprint 33 | HEX-04 |
| HEX-06 | *(Optional, eigener Entscheid nach HEX-05)* Use-Case-Schicht aus `Handler` extrahieren (Anwendungsfälle als eigene Funktionen zwischen HTTP-Layer und `FleetStore`/`AlertEngine`) — nur falls nach dem Pilot als lohnend bewertet, kein Bestandteil des Piloten selbst. | M | ✅ Sprint 45 | HEX-05 |

**Entscheidungspunkt nach HEX-05 (nicht Teil dieses Sprint-Plans):** ob und in welcher
Reihenfolge auth-service (JWT-Port) bzw. telemetry-service (kleinster Fall) folgen — siehe
Risikobewertung in ADR-031. `control-server` bleibt bis auf Weiteres ausdrücklich ausgeklammert
(eigenes ADR nötig, siehe ADR-031 "Offene Punkte").

**Sprint-34-Kickoff (2026-07-18):** beide offenen Entscheidungspunkte (HEX-06, Fortsetzung auf
auth-/telemetry-service) explizit dem Nutzer vorgelegt — Nutzerentscheidung: beide weiterhin
zurückstellen (kein neuer Anlass, passt zum Ziel kleinerer, token-budgetierter Sprints). Zusätzlich
inhaltlich relevant: `auth-service` ist ohnehin Ziel von `SEC-01` (JWT-Sicherheitslücke) in Sprint
34 — eine Hexagonal-Migration desselben Codes im selben Sprint hätte unnötig Overhead erzeugt.

### Fortsetzung, Schritt 2 — auth-service (JWT-Port), freigegeben Sprint-37-Kickoff

**Freigabe (2026-07-19):** Nutzer entscheidet sich explizit für Fortsetzung der
Hexagonal-Migration, damit ist der seit Sprint 34 zurückgestellte Entscheidungspunkt aufgelöst.
Scope folgt der Priorisierung in ADR-031 Schritt 2 (`auth-service`, Storage-Seite bereits über
`UserStore` gelöst — nur JWT-Port fehlt). `telemetry-service` (Schritt 3) bleibt bewusst ein
eigener, separat zu entscheidender Folgeschritt (ein Service pro Sprint, wie beim Piloten).
Use-Case-Extraktion (Login-/Handover-Policy als reine Funktionen) ist, analog zu `HEX-06` beim
Piloten, ein eigener Entscheidungspunkt **nach** Abschluss des Ports — nicht automatisch Teil
dieses Schritts.

Vorrecherche (2026-07-19, vor Sprint-Start durchgeführt): `internal/authservice/handler.go:305-332`
— `issueToken`/`parseToken` sind private `Handler`-Methoden, die direkt gegen
`golang-jwt/jwt/v5` arbeiten (`jwt.NewWithClaims`/`jwt.ParseWithClaims`), `Handler.secret []byte`
als Feld (Zeile 33). Anders als beim `FleetStore`-Piloten bringt der Port hier **keine
DB-Unabhängigkeit** — `handler_test.go` läuft bereits ohne externe Ressource, da JWT-Signierung
reine Berechnung ist (`UserStore` ist über `stubUserStore` bereits vollständig entkoppelt). Der
Nutzen ist Dependency Inversion: `Handler` importiert `golang-jwt/jwt/v5` danach nicht mehr direkt.
`cmd/auth-service/main.go:35` ruft `authservice.NewHandler(secret, userStore)` auf (`secret` als
`string`) — Konstruktorsignatur bleibt unverändert, wenn `NewHandler` intern einen
`JWTTokenIssuer` aus dem `secret`-String baut (identisches Muster zu HEX-02: `cmd/fleet-service/
main.go:59` musste bei der `FleetStore`-Umstellung ebenfalls nicht angepasst werden, da
`*PostgresFleetStore` das neue Interface strukturell bereits erfüllte).

**Nebenbefund, bewusst außerhalb dieses Schritts:** `golang-jwt/jwt/v5` mit identischem
Alg-Confusion-Guard (SEC-01) direkt in 6 Paketen importiert (`cmd/control-server/main.go`,
`cmd/vehicle-mock/main.go`, `internal/authservice/handler.go`, `internal/fleetservice/handler.go`,
`internal/vehicleconnection/handler.go`, `internal/controlserver/transport/websocket.go`) —
potenzielles Rule-3.1-Duplikat (gemeinsames `pkg/authtoken` denkbar), aber paketübergreifende
Extraktion ist nicht Teil des ADR-031-Scopes für diesen Schritt (nur `auth-service`s eigener
Handler-Code). Dokumentiert als möglicher späterer Folge-Task, nicht mit diesem Schritt vermischt.

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| HEXAUTH-01 | `TokenIssuer`-Port definieren (`IssueToken(subject string, role OperatorRole, ttl time.Duration) (string, error)`, `ParseToken(tokenStr string) (*Claims, error)`) in neuer Datei `internal/authservice/tokenissuer.go`; `JWTTokenIssuer`-Struct (wraps `golang-jwt/jwt/v5`, übernimmt `issueToken`/`parseToken`-Logik unverändert von `Handler`) + `NewJWTTokenIssuer(secret string)`. Compile-Time-Check `var _ TokenIssuer = (*JWTTokenIssuer)(nil)`. Kein `Handler`-Wechsel in diesem Schritt (mirrors HEX-01). | S | ✅ Sprint 37 | — |
| HEXAUTH-02 | `Handler.secret []byte` → `Handler.tokens TokenIssuer` (`internal/authservice/handler.go:33`). `NewHandler(secret string, userStore UserStore)` konstruiert `JWTTokenIssuer` intern — externe Signatur unverändert, `cmd/auth-service/main.go:35` braucht keine Anpassung (mirrors HEX-02/Fleet-Precedent). Alle Aufrufer von `h.issueToken`/`h.parseToken` auf `h.tokens.IssueToken`/`h.tokens.ParseToken` umgestellt, die beiden alten privaten Methoden entfernt. HTTP-Verhalten unverändert. | S | ✅ Sprint 37 | HEXAUTH-01 |
| HEXAUTH-03 | Verifikation: `go build ./...`, `go vet ./...`, `go test ./internal/authservice/...` grün (weiterhin ohne `DATABASE_URL`, wie zuvor — kein neuer Testgewinn hier, nur Architektur-Sauberkeit). ADR-031-Status-Update (Schritt 2 abgeschlossen) + `DECISIONS.MD` + dieser Backlog-Eintrag. | S | ✅ Sprint 37 | HEXAUTH-02 |
| HEXAUTH-04 | *(Optional, eigener Entscheid nach HEXAUTH-03)* Use-Case-Schicht aus `Handler` extrahieren (Login-/Handover-/Registrierungs-Policy als reine Funktionen zwischen HTTP-Layer und `TokenIssuer`/`UserStore`) — nur falls nach HEXAUTH-01..03 als lohnend bewertet, kein Bestandteil dieses Schritts selbst, analog `HEX-06`. | M | ✅ Sprint 45 | HEXAUTH-03 |

**Nach HEXAUTH-03:** nächster Entscheidungspunkt, ob `telemetry-service` (ADR-031 Schritt 3)
folgt — kein Automatismus, analog zum Entscheidungspunkt nach dem Piloten.

---

## EPIC: Go Coding Style Guide Rollout

Style Guide selbst: [docs/go-style-guide.md](../docs/go-style-guide.md) (Wortlaut vom Nutzer
vorgegeben, ergänzt um projektspezifische Anmerkungen). Referenziert aus CLAUDE.MD Abschnitt 12.

**Quantifizierte Bestandsaufnahme (2026-07-17, AST-basiert über alle 7 Go-Services + `pkg/`, 404
Funktionen/Methoden insgesamt):** 12 Funktionen >50 Zeilen (Rule 2.2, größtenteils `main()`-
Funktionen), 6 Funktionen/Methoden >4 Parameter (Rule 2.3), 3 Interfaces >3 Methoden plus 2 an der
Grenze (Rule 4.3), 0 generische utils/common/helpers-Pakete (Rule 4.1 — `pkg/db`, `pkg/logger`,
`pkg/ulid`, `pkg/audit` bestätigt einzweckig), 2 echte Dreifach-Duplikate (Rule 3.1: `envOr`-Helper
und DB-Open+WaitForReady-Block, je 3× identisch). Vertiefte Analyse aller Interfaces >1 Methode
ergab: **alle** sind aktuell producer-definiert (`UserStore`, `SessionRecorder`, `VehicleStore`,
`FleetGateway`, `AuditWriter`, `safety.Publisher`); `SessionRecorder` und `FleetGateway` werden
nirgends als Interface-Typ konsumiert (Dead Ports, deckt sich mit ADR-031); jeweils mind. eine
Bootstrap-/Lifecycle-Methode (`SeedAdmin`, `SeedDefault`, `Close`) ist nie über das Interface
aufgerufen. Die kleinen 1-Methoden-Interfaces (`Dispatcher`, `safetyPublisher`, `VehicleAdder`
u. a.) sind dagegen bereits vorbildlich konsumentenseitig geschnitten — Vorbild für Phase 2.

Grill-Me 2026-07-17, vier Fragen, Antworten unten eingearbeitet:
- **Umfang:** komplette Codebasis, alle 4 Regelblöcke (nicht nur ein Pilot-Service).
- **Interface-Regeln (Rule 4.2/4.3):** eigene, spätere Phase — koordiniert mit ADR-031
  (Hexagonal-Migration), da beide dieselben Interfaces anfassen würden.
- **Priorität:** Rules 1-3 + Duplikat-Extraktion laufen **jetzt parallel** zu den laufenden
  Dashboard-Strängen (keine Signaturänderungen nach außen, geringes Risiko). Rule 4 wartet auf
  ADR-031 (Pilot-Abschluss `HEX-05` + Post-AP2/AP3-Timing).
- **Durchsetzung:** `golangci-lint` wird eingerichtet, aber als **non-blocking Warn-Stufe** —
  löst das Henne-Ei-Problem (kein hartes Gate vor Abschluss der Angleichung nötig) und macht
  Fortschritt sprintübergreifend sichtbar, statt erst am Ende zu gaten.

**Sprint-Nummern:** Phase 1 ist zu groß für einen einzelnen Sprint (CLAUDE.MD Abschnitt 10: "aktive
Arbeit max. 3–10 Tasks") und wird daher in drei Sprints gesplittet (27/28/29 vorgeschlagen — **zur
Verifikation beim Merge**: `Sprint 26` ist bereits durch die parallelen Worktrees `driftaudit`/
`driftfix`/`drift-k1-k3-safety` belegt, analog zur ADR-030/031-Nummernkollision oben real möglich).

**Keine Signaturänderung nach außen, kein Verhaltenswechsel** in allen drei Sprints — jeder Task
endet mit vollem Testlauf des betroffenen Service + Diff-Review gegen genau diese Vorgabe
(CLAUDE.MD Abschnitt 15).

#### Sprint 27 — Fundament: `pkg/db`, `pkg/env`, non-blocking Linter-Gate — ✅ fertig

Umgesetzt und nach `tasks/current-sprint.md` verschoben (`GOSTYLE-01`, `GOSTYLE-02`, `GOSTYLE-15`,
Details/Ergebnisse dort). Branch `feature/fleet-service-foundation-gostyle`.

#### Sprint 28 — Risikoarme Services: Rule 2.2 + 2.3 (9 Tasks) — ✅ fertig

Umgesetzt und nach `tasks/current-sprint.md` verschoben (`GOSTYLE-03` bis `GOSTYLE-11`,
Details/Ergebnisse dort). Branch `feature/fleet-service-foundation-gostyle28`.

#### Sprint 29 — `control-server` (hohes Risiko) + Abschlussverifikation — ✅ fertig

Umgesetzt und nach `tasks/current-sprint.md` verschoben (`GOSTYLE-12`, `GOSTYLE-13`,
`GOSTYLE-14`, `GOSTYLE-16`, Details/Ergebnisse dort). Branch
`feature/fleet-service-foundation-gostyle29`. Damit ist Phase 1 des EPICs (Sprints 27/28/29)
vollständig abgeschlossen — Phase 2 (Interface-Segregation, Rule 4.2/4.3) folgt koordiniert mit
ADR-031/HEX-05.

**Folge-Task (aus GOSTYLE-13 gefunden, nicht Teil des Sprint-29-Scopes) — ✅ Sprint 36
(`TESTGAP-01`):** die drei WS-Integrationstests in `tests/integration/services_test.go`
(`TestIntegration_SessionLifecycle_StartAndEnd`, `_MediaFailed_TriggersDegrade_NeverSafeMode`,
`_EmergencyStop_TriggersSafeMode`) skippten im Docker-Test-Stack, weil ihre WS-Dial-URL nur
`?token=` statt `?token=&session_id=` mitgab — ein vorbestehender Test-Setup-Gap (nicht
WebRTC/SFU-bedingt wie die anderen 3 bekannten Skips), der `WSHandler.readLoop` komplett ohne
automatisierte Abdeckung ließ. Behoben: Reihenfolge umgekehrt (`/session/start` vor WS-Dial) plus
zusätzlich gefundener `vehicleRegistry.Connected`-Blocker durch `/vehicle/ws`-Registrierung der
Test-Vehicles gelöst. Details in `tasks/sprints/36-restposten-bereinigung-iii.md`.

**Bonus, außerhalb des Style-Guide-Scopes (nebenbei gefunden) — ✅ Sprint 36 (`GOSTYLE-FMT-01`):**
`gofmt -l` fand 10 unformatierte Dateien (9 nach der TECHDEBT-01-Löschung) — mit `gofmt -w .`
behoben, keine Logikänderung. Details in `tasks/sprints/36-restposten-bereinigung-iii.md`.

### Phase 2 — Interface-Segregation (Rule 4.2/4.3), nachgelagert nach ADR-031 — ✅ abgeschlossen (Sprint 35)

**Start erst nach `HEX-05`** (Hexagonal-Pilot fleet-service abgeschlossen) **und** nach
AP2/AP3-Meilensteinen, analog zum ADR-031-Timing. Koordiniert mit dem Hexagonal-Epic oben —
dieselben Interfaces, unterschiedlicher Fokus (dort: Repository-Port für `fleet-service`; hier:
Methodenzahl/Konsumenten-Zuschnitt projektweit). Alle 6 Tasks (GOSTYLE-IF-01..06) abgeschlossen,
keine weiteren Interface-Segregation-Folgetasks offen.

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| GOSTYLE-IF-01 | `recording.SessionRecorder` (6 Methoden, aktuell nirgends als Interface-Typ konsumiert): klären, ob Interface tatsächlich verwendet werden soll (`*MemoryRecorder` → `SessionRecorder` in `cmd/control-server/main.go:102`) oder aufgelöst wird, solange nur eine Implementierung existiert (Rule 1.2 — Abstraktion ohne Konsument ist unbegründet) | S | ✅ Sprint 34 | Interface komplett entfernt (`internal/recording/recorder.go`) — Details/Ergebnisse in `tasks/current-sprint.md` |
| GOSTYLE-IF-02 | `fleetgateway.FleetGateway` (3 Methoden, ungenutzt als Typ): gleiche Frage wie IF-01. `fleetservice.Dispatcher` (1 Methode) ist bereits der korrekte konsumentenseitige Schnitt und bleibt unverändert | S | ✅ Sprint 34 | **Nicht** wie IF-01 gelöscht (aktives ADR-027 schreibt die Abstraktion bewusst vor) — stattdessen tatsächlich nutzbar gemacht: `subscribeVehicleStatus`/`subscribeVehicleAlerts` in `cmd/fleet-service/main.go` nehmen jetzt `fleetgateway.FleetGateway` statt `*MQTTGateway` entgegen. Details in `tasks/current-sprint.md` |
| GOSTYLE-IF-03 | `pkg/audit.AuditWriter` (3 Methoden) in schlankes `WriteSync`-only Interface für die 5 Safety-/Command-Consumer aufspalten; `Close`/`QueryBySession` bleiben am konkreten Typ bzw. eigenem kleineren Interface für `main.go` | M | ✅ Sprint 35 | Neues `SafetyAuditWriter`-Interface (`WriteSync`-only) eingeführt, in allen 5 Consumern eingesetzt. `Close` aus `AuditWriter`/`SafetyAuditWriter` entfernt, dadurch `NoopWriter.Close` tot geworden und mit gelöscht. Details in `tasks/current-sprint.md` |
| GOSTYLE-IF-04 | `authservice.UserStore` (7 Methoden): `SeedAdmin` (reine Bootstrap-Methode, bereits am konkreten Typ genutzt) aus dem Interface entfernen; verbleibende 6 Methoden gegen tatsächlichen `Handler`-Bedarf prüfen | M | ✅ Sprint 35 | `SeedAdmin` aus `UserStore` entfernt, verbleibende 6 Methoden bestätigt in Gebrauch. Nebenbefund: `NoopUserStore` komplett ungenutzt (unabhängig von diesem Task, siehe neuer Eintrag unten). Details in `tasks/current-sprint.md` |
| GOSTYLE-IF-05 | `vehicleregistry.VehicleStore` (5 Methoden): `SeedDefault` (Bootstrap) aus Interface lösen, analog IF-04 | S | ✅ Sprint 34 | `SeedDefault` aus `VehicleStore` entfernt, dadurch `NoopVehicleStore.SeedDefault` tot geworden und mit gelöscht. Details in `tasks/current-sprint.md` |
| GOSTYLE-IF-06 | `controlserver/safety.Publisher` (2 Methoden) in `PublishEvent`-only Interface für `Engine`/Watchdogs aufteilen; `TriggerEmergencyStop` bleibt eigener Zugriffspfad, analog zum bereits vorbildlichen `vehicleconnection.safetyPublisher`-Muster | S | ✅ Sprint 34 | `TriggerEmergencyStop` aus `Publisher` entfernt (nur je über den konkreten `*HTTPPublisher` aufgerufen, nie interface-typisiert). Details in `tasks/current-sprint.md` |

**Nebenbefund, nicht Teil dieses Style-Guide-Scopes (Sicherheitsauffälligkeit):** beim
Duplikat-Scan (Rule 3.1) fiel auf, dass der JWT-Alg-Confusion-Check in mehreren JWT-Parse-Stellen
fehlt. Kein Style-Guide-Thema — als `SEC-01` aufgenommen, siehe EPIC "Security Findings" unten.

**Nebenbefund aus GOSTYLE-IF-04 (Sprint 35, kein Interface-Segregation-Thema):**
`internal/authservice/noop_userstore.go`s `NoopUserStore` ist komplett ungenutzt — keine
Referenz außerhalb der eigenen Datei, auch nicht in Tests (`handler_test.go` nutzt einen eigenen
`stubUserStore`). War bereits vor GOSTYLE-IF-04 tot (nicht erst durch das Entfernen von
`SeedAdmin` verursacht, anders als der `NoopVehicleStore.SeedDefault`-Fall in Sprint 34). Als
`TECHDEBT-01` aufgenommen, siehe EPIC "Tech Debt" unten.

---

## EPIC: Security Findings

| ID | Task | Typ | Status | Notizen |
|----|------|-----|--------|---------|
| SEC-01 | JWT-Alg-Confusion-Check fehlt an 3 von 5 `jwt.Parse*`-Stellen | S/M | ✅ Sprint 34 | Alle drei Stellen gefixt (`authservice.parseToken`, `transport.validateJWT`, `vehicleconnection.validateJWT`) — `t.Method.(*jwt.SigningMethodHMAC)`-Check ergänzt, analog zu den bereits korrekten Stellen. Regressionstests mit gefälschtem `alg:none`-Token an allen drei Stellen (2x gegen Flakiness geprüft, CLAUDE.MD Abschnitt 17). Details/Ergebnisse in `tasks/current-sprint.md`. |

### MQTT-Authentifizierung (Mosquitto Passwort-File), ✅ Sprint 38

**Freigabe (2026-07-19):** Nutzerentscheidung gegenüber der Alternative "Hexagonal-Migration
Schritt 3 (telemetry-service)" — Begründung CLAUDE.MD §0 Priorität 1 ("Sicherheit schlägt alles").
Schließt den seit Projektbeginn offenen Punkt "MQTT-Authentifizierung (Mosquitto Passwort-File)"
unten in "Offene Entscheidungen". Kein neues ADR nötig — ADR-003 legt Mosquitto bereits fest,
dieser Sprint aktiviert nur dessen eingebauten `password_file`-Mechanismus. TLS/MQTTS
(Transportverschlüsselung) ist bewusst **nicht** Teil dieses Sprints. Vollständige Vorrecherche
(Datei-/Zeilenreferenzen zu den 3 Go-MQTT-Verbindungsstellen, 3 Compose-Dateien, SSM/Deploy-
Präzedenzfall `TURN_USER`/`TURN_PASSWORD`) in `tasks/sprints/38-mqtt-authentifizierung.md`.

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| MQTTAUTH-01 | Mosquitto-Configs (Dev/Prod/Test) auf `allow_anonymous false` + `password_file` umstellen, gehashte Dev-/Test-Passwd-Dateien committen | S | ✅ Sprint 38 | — |
| MQTTAUTH-02 | `SetUsername`/`SetPassword` an den 3 Go-MQTT-Verbindungsstellen (`telemetryservice`, `fleetgateway`, `vehicle-mock`) + neue `MQTT_USERNAME`/`MQTT_PASSWORD`-Env-Vars | M | ✅ Sprint 38 | MQTTAUTH-01 |
| MQTTAUTH-03 | Neue Env-Vars an alle 4 MQTT-Consumer-Services in allen 3 Compose-Dateien durchreichen + `.env.example` | S | ✅ Sprint 38 | MQTTAUTH-02 |
| MQTTAUTH-04 | `scripts/setup-ssm.sh`/`scripts/deploy.sh`: SSM-Parameter + Passwd-Datei-Generierung zur Deploy-Zeit, analog `TURN_USER`/`TURN_PASSWORD` | M | ✅ Sprint 38 | MQTTAUTH-01 |
| MQTTAUTH-05 | Bestehende Unit-/Integrationstests mit direkten MQTT-Verbindungen (`fleetgateway/mqtt_test.go`, `fleet_simulation_test.go`, `fleet_service_test.go`) auf neue Credentials umstellen | S | ✅ Sprint 38 | MQTTAUTH-01..03 |
| MQTTAUTH-06 | Verifikation (Docker-Test-Stack + Dev-Stack) + Doku-Updates (`DECISIONS.MD`, `docs/architecture.md`) | S | ✅ Sprint 38 | MQTTAUTH-02..05 |

---

## EPIC: Testabdeckung sicherheitsrelevanter Services (ADR-031-Bestandsaufnahme) ✅ Sprint 39

**Freigabe (2026-07-19):** Nutzerentscheidung gegenüber drei Alternativen (Hexagonal-Migration
Schritt 3 telemetry-service, TLS/MQTTS-Härtung, Session-Recording-Storage-Entscheidung) —
Begründung CLAUDE.MD §0 Priorität 1 ("Sicherheit schlägt alles") + Abschnitt 17 (Teststandard).
Schließt den seit der ADR-031-Bestandsaufnahme (2026-07-16) offenen Punkt "Testabdeckung
`safety-service`/`webrtc-sfu`/`internal/recording` (0 Tests)" unten in "Offene Entscheidungen".
Reine Testabdeckung, keine Produktivcode-Verhaltensänderung, kein neues ADR nötig. Vollständige
Vorrecherche in `tasks/current-sprint.md` (Sprint 39).

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| TESTCOV-01 | `internal/recording/memory_recorder_test.go` — vollständige `MemoryRecorder`-Abdeckung (Start/EndSession, drei Record*-Methoden, `GetEntries`-Kopie-statt-Referenz, Session-Isolation) | S | ✅ Sprint 39 | — |
| TESTCOV-02 | `internal/safetyservice/bus_test.go` — `Bus`-Abdeckung (Publish/TriggerEmergencyStop/GetSafetyState/Subscribe mit mehreren Handlern/Reset), inkl. `-race`-Nebenläufigkeitstest | M | ✅ Sprint 39 | — |
| TESTCOV-03 | `cmd/safety-service/main_test.go` — HTTP-Handler-Tests (`newSafetyMux`, 4 Endpoints, `httptest`) | S/M | ✅ Sprint 39 | TESTCOV-02 |
| TESTCOV-04 | `internal/webrtcsfu/sfu_test.go` — `HandleSessionEvent`-Zustandsübergänge + `registerOperatorSubscription`-Dedup-Logik + `removePeer`, via `webrtc.NewPeerConnection` ohne echte Netzwerk-Negotiation | M | ✅ Sprint 39 | — |
| TESTCOV-05 | `cmd/webrtc-sfu/main_test.go` — HTTP-Handler-Tests (`newSFUMux`, 4 Endpoints, `httptest`; Erfolgsfall der Offer/Subscribe-Endpoints bewusst außerhalb des Scopes) | S | ✅ Sprint 39 | TESTCOV-04 |
| TESTCOV-06 | Verifikation (`go build`/`go vet`/`go test ./...` + `-race` für `safetyservice`/`webrtcsfu`) + Doku-Updates (`DECISIONS.MD`, `tasks/backlog.md`, `CONTEXT.MD`-Aktualitätsprüfung) | S | ✅ Sprint 39 | TESTCOV-01..05 |

**Nicht Teil dieses Sprints:** echte End-to-End-WebRTC-SDP-Negotiation (`CreateVehicleOffer`/
`SubscribeOperator`/`negotiateAnswer`, ADR-006: "zu flaky in CI"), `SFU.forwardTrack`
(RTP-Kopierschleife), Hexagonal-Migration der drei Services selbst (bleibt eigener,
separat zu entscheidender ADR-031-Folgeschritt).

---

## EPIC: TLS/MQTTS-Härtung für Mosquitto ✅ Sprint 40

**Freigabe (2026-07-19):** Nutzerentscheidung gegenüber zwei Alternativen (Hexagonal-Migration
Schritt 3 telemetry-service, Session-Recording-Storage-Entscheidung) — Begründung CLAUDE.MD §0
Priorität 1 ("Sicherheit schlägt alles"). Sprint-38-Nachfolge: Mosquitto-Authentifizierung war
bereits geschlossen, Transportverschlüsselung bewusst ausgeklammert (`DECISIONS.MD` Zeile
"TLS/MQTTS-Härtung für Mosquitto"). Kein neues ADR nötig — ADR-003 legt Mosquitto bereits fest,
dieser Sprint aktiviert nur dessen eingebauten TLS-Listener-Mechanismus. Trust-Modell bei der
Planung entschieden: echte CA-Zertifikatsprüfung (keine `InsecureSkipVerify`), kein mTLS (würde
Sprint 38s Username/Passwort-Auth duplizieren), harter Cutover auf Port 8883 ohne
Parallelbetrieb mit 1883. Vollständige Vorrecherche (Datei-/Zeilenreferenzen zu den 3
Go-MQTT-Verbindungsstellen, 3 Compose-Dateien, nginx-Zertifikats-Präzedenzfall in
`scripts/deploy.sh`) in `tasks/current-sprint.md` (Sprint 40).

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| MQTTS-01 | Zertifikatserzeugung: selbstsignierte CA + Mosquitto-Server-Zertifikat (Dev/Test committed, Prod via `scripts/deploy.sh` analog SSL-/Passwd-Muster) | S/M | ✅ Sprint 40 | — |
| MQTTS-02 | `mosquitto.conf`/`mosquitto-test.conf`/Prod-Konfiguration: `listener 1883` → `listener 8883` + `cafile`/`certfile`/`keyfile` | S | ✅ Sprint 40 | MQTTS-01 |
| MQTTS-03 | Go-Client-TLS an den 3 Verbindungsstellen (`telemetryservice`, `fleetgateway`, `vehicle-mock`): `tcp://`→`tls://`, `SetTLSConfig` mit `RootCAs`, neue `MQTT_CA_CERT`-Env-Var | M | ✅ Sprint 40 | MQTTS-01 |
| MQTTS-04 | Drei Compose-Dateien: Port 8883, CA/Cert/Key-Volume-Mounts, `MQTT_CA_CERT`-Env-Var an den 4 Consumer-Services | S/M | ✅ Sprint 40 | MQTTS-01..03 |
| MQTTS-05 | Testinfrastruktur: committetes Test-CA/Zertifikat-Paar, `mqtt_test.go`-Helper + 3 abhängige Testdateien auf TLS umstellen, Gegenprobe-Test „Verbindung ohne gültige CA abgelehnt" | S/M | ✅ Sprint 40 | MQTTS-01..04 |
| MQTTS-06 | Verifikation (`go build`/`go vet`/`go test ./...` + `make test-integration` gegen echten TLS-Broker) + Doku-Updates (`DECISIONS.MD`, `docs/architecture.md`, `tasks/backlog.md`) | S | ✅ Sprint 40 | MQTTS-01..05 |

**Nicht Teil dieses Sprints:** mTLS/Client-Zertifikate (siehe Trust-Modell-Begründung oben),
CA-Rotationsstrategie für Produktivbetrieb, SSM-Verteilung der CA (öffentliches Zertifikat, lokal
auf dem EC2-Host generiert, kein Cross-Host-Bedarf).

---

## EPIC: CI-Gates einführen (ADR-006-Bestandsaufnahme) ✅ Sprint 41

**Freigabe (2026-07-19):** Nutzer hat am 2026-07-19 eine Testing-Strategie-Bestandsaufnahme gegen
ADR-006 (Testing Strategy) + CLAUDE.MD Abschnitt 17 angefordert. Größter gefundener Bruch
zwischen Dokumentation und Realität: `.github/workflows/` enthält ausschließlich `lint.yml`
(non-blocking, `continue-on-error: true`) — die von ADR-006 beschriebene Pipeline mit 4
blockierenden Gates (Unit Go+Frontend, Safety Test Suite, Integration, Latency) + 1 non-blocking
WebRTC-E2E-Job existiert nicht. Alle Prüfungen laufen ausschließlich manuell über bestehende
`Makefile`-Targets (`test`/`test-safety`/`test-integration`/`test-latency`/`test-k6`) — der
Kommentar in `test-safety` ("CI safety gate — must stay 19/19 green") ist irreführend, es gibt
kein CI dafür. Nutzerentscheidung: dieser Befund wird priorisiert vor zwei kleineren Audit-Funden
(Concurrency-Test-Lücke in `internal/webrtcsfu/sfu_test.go`, Safety-Test-Suite testet aktuell den
falschen Typ — bleiben offene Folgepunkte unten). **Eingereiht nach Sprint 40 (TLS/MQTTS-Härtung)
— wird erst zu Sprint 41, sobald Sprint 40 abgeschlossen ist**, `tasks/current-sprint.md` bleibt
bis dahin unverändert Sprint 40.

*Nachtrag:* Sprints 41-44 wurden in parallelen Worktrees geplant/umgesetzt, bevor Sprint 40s Code
tatsächlich geschrieben wurde (Sprint 40 blieb als "Geplant, noch nicht begonnen" in
`tasks/current-sprint.md` stehen, während 41-44 unabhängig liefen) — die ursprüngliche
Reihenfolgen-Prämisse oben traf real nicht zu, alle fünf Sprints sind inzwischen ✅.

**Architektur-Entscheidung (bei der Planung getroffen):**
- **Latenz-Gate (`test-latency`/`test-k6`) bewusst non-blocking**, obwohl ADR-006 es als
  "BLOCKING" dokumentiert: GitHub-gehostete Runner haben stark schwankende CPU-Zuteilung
  (Shared-Tenancy) — eine harte `<100ms`-Assertion würde auf einem verrauschten Runner Merges
  blockieren, ohne dass sich der Code geändert hat (False Positives). Gleiches Muster wie ADR-006s
  eigene Begründung für die WebRTC-Non-Determinism-Policy (non-blocking + sichtbar statt hartes
  Gate). Ergebnis bleibt sichtbar (Benchmark-Output als Job-Log/Artifact), blockiert aber keinen
  Merge. Verschärfung auf "blocking" ist ein möglicher Folge-Task, sobald genug CI-Läufe
  Rausch-Baseline zeigen.
- **Bestehender Playwright-Spec (`tests/e2e/dashboard.spec.ts`) wird als non-blocking
  Informational-Job eingebunden**, aber nicht inhaltlich vertieft (bleibt oberflächlich, siehe
  Audit-Punkt D) — Ausbau der E2E-Tiefe ist ein eigener, größerer Folge-Task.
- **Branch-Protection-Aktivierung (Required Status Checks) nur vorbereitet, nicht scharf
  geschaltet** — das ist eine Repo-Einstellung, die alle zukünftigen PRs/Merges betrifft
  (geteiltes System), daher explizite Nutzerbestätigung vor Aktivierung nötig (siehe MB-Regeln zu
  risikoreichen/schwer umkehrbaren Aktionen).

**Vorrecherche (2026-07-19):**
- `Makefile:69-116`: `test` (`go test ./...` — läuft `tests/integration/...` **mit**, schlägt ohne
  laufenden Docker-Stack fehl, siehe Sprint-39-Verifikation), `test-safety` (`go test
  ./tests/unit/... -run Safety`, kein Docker nötig), `test-integration` (bringt
  `tests/docker-compose.test.yml`-Stack hoch, `go test ./tests/integration/...`, fährt Stack
  wieder runter), `test-latency` (Docker-Stack + `BenchmarkControlACKRoundtrip`,
  `b.Fatalf` bei p99>100ms), `test-k6` (Docker-Stack + k6 gegen `tests/performance/latency.js`,
  `thresholds: p(99)<100`).
- **`test` läuft für eine saubere CI-Unit-Gate nicht direkt verwendbar**, da es
  `tests/integration/...` ungefiltert mitnimmt — braucht einen neuen, zusätzlichen
  `test-unit`-Target (paketgefiltert, `go list ./... | grep -v /tests/integration`), ohne das
  bestehende `test`-Target zu ändern (Devs mit lokal laufendem Stack nutzen `test` weiterhin wie
  bisher).
- `.github/workflows/lint.yml` ist die einzige bestehende CI-Datei — dient als Stil-Vorlage
  (Trigger `on: push: branches: [main] / pull_request`, `actions/setup-go@v5`, Go 1.23).
- `tests/docker-compose.test.yml:1-4`: Kommentar bestätigt bewusst "Kein WebRTC/coturn — zu
  flaky in CI" — GitHub-Actions-Runner (`ubuntu-latest`) haben Docker Engine + `docker compose`
  v2 vorinstalliert, kein zusätzliches Setup nötig; Images werden aus Source gebaut
  (`build: context: ..`), erster CI-Lauf zeigt reale Build-Zeit (ggf. `timeout-minutes` setzen).
- `frontend/package.json`: `"test": "vitest run"` (Vitest, nicht Jest wie in ADR-006 benannt —
  funktional gleichwertig, keine Änderung nötig), `"test:e2e": "playwright test"`.
- **ADR-006-Update nötig**: Abschnitt "Update (2026-07-15)" wird um einen zweiten Update-Absatz
  ergänzt (CI-Automatisierung Sprint 41, Latenz-Gate-Abweichung non-blocking begründet).

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| CIGATE-01 | `Makefile`: neuer `test-unit`-Target (paketgefiltert ohne `tests/integration`), bestehende Targets unverändert | S | ✅ Sprint 41 | — |
| CIGATE-02 | `.github/workflows/test-go.yml`: 3 blockierende Jobs — `unit` (`make test-unit`), `safety` (`make test-safety`), `integration` (`make test-integration`, Docker-Stack) | M | ✅ Sprint 41 | CIGATE-01 |
| CIGATE-03 | `.github/workflows/test-frontend.yml`: Vitest-Unit-Tests blockierend (`npm ci && npm run test`) | S | ✅ Sprint 41 | — |
| CIGATE-04 | `.github/workflows/test-latency.yml`: Go-Benchmark + k6, bewusst non-blocking (`continue-on-error: true`, begründeter Kommentar analog `lint.yml`) | S/M | ✅ Sprint 41 | CIGATE-01 |
| CIGATE-05 | Bestehenden Playwright-Spec als non-blocking Informational-Job einbinden (kein Ausbau der Testtiefe) | S | ✅ Sprint 41 | — |
| CIGATE-06 | Branch-Protection: Required-Status-Checks vorbereiten/dokumentieren (welche 4 Jobs), Aktivierung selbst erst nach expliziter Nutzerbestätigung (Repo-Setting, betrifft alle PRs) | S | ✅ Vorbereitung Sprint 41, Aktivierung ✅ 2026-07-22 (CIPUB-04, Nachtrag nach Sprint 56) | CIGATE-02, CIGATE-03 |
| CIGATE-07 | Verifikation: mind. 2 aufeinanderfolgende grüne CI-Läufe (Flakiness-Ausschluss, CLAUDE.MD §17), Timeout-/Resourcen-Anpassung falls nötig, Doku-Updates (`DECISIONS.MD`, ADR-006-Update-Absatz, `tasks/backlog.md`) | S | ✅ (Sprint 41 lokal + Nachtrag 2026-07-20 echte Actions-Läufe) | CIGATE-01..06 |

**Hinweis zu CIGATE-07:** "2 aufeinanderfolgende grüne CI-Läufe" im Sinne von echten GitHub-Actions-
Runs konnte in Sprint 41 selbst nicht verifiziert werden, da kein Commit/Push erfolgte (Nutzervorgabe:
"nicht committen ohne ausdrückliche Aufforderung"). Ersatzweise wurden alle 3 blockierenden
`test-go.yml`-Jobs sowie der `test-frontend.yml`-Job **lokal je 2× mit frischem Cache
(`-count=1`)** gegen den jeweils echten Docker-Stack ausgeführt (siehe
`tasks/sprints/41-ci-gates-einfuehren.md`, Abschnitt Ergebnisse) — durchgehend grün, keine
Flakiness beobachtet. Alle 5 Workflow-Dateien wurden zusätzlich mit `actionlint` syntax-/
semantik-geprüft (0 Findings).

**Nachtrag (2026-07-20) — erste echte GitHub-Actions-Läufe:** Über einen dedizierten Test-PR
(`test/ci-pipeline-verification` → `main`) zwei echte, aufeinanderfolgende Läufe beobachtet
(Runs #3 und #4, `gh`-los über die öffentliche GitHub-REST-API ohne Auth abgefragt, da kein `gh`
und kein Token in dieser Umgebung verfügbar waren — für ein öffentliches Repo reicht das für
Run-/Job-Status, Logs/Artifacts brauchen dagegen Auth). Alle vier blockierenden Jobs (`unit`,
`safety`, `integration` aus `test-go.yml`, `vitest` aus `test-frontend.yml`) liefen in beiden
Runs auf echten `ubuntu-latest`-Runnern durch — echtes Grün, kein lokaler Ersatz mehr nötig.

Dabei echten, vorher unbekannten Bug gefunden und behoben: der allererste echte Lauf von
`test-e2e.yml` scheiterte mit `Cannot find module '@playwright/test'`, bevor überhaupt ein Test
lief — `tests/e2e/dashboard.spec.ts` lag außerhalb von `frontend/`s `node_modules`-Baum, Node löst
nackte Imports aber relativ zur eigenen Datei auf (Verzeichnis-Aufwärtssuche), nicht relativ zu
`playwright.config.ts`. Nie zuvor aufgefallen, weil die Sprint-41-Verifikation nur die laufende App
manuell prüfte (Chrome DevTools MCP), nie `npm run test:e2e` selbst ausführte. Fix: Spec nach
`frontend/tests/e2e/dashboard.spec.ts` verschoben, `playwright.config.ts`
(`testDir`/`reporter.outputFolder`) und der Artifact-Upload-Pfad in `test-e2e.yml` angepasst,
lokal mit frischem `npm ci` verifiziert (Modul löst auf, alle 5 Tests werden jetzt gefunden und
ausgeführt) und per zweitem echten Actions-Lauf bestätigt (Job läuft jetzt bis zum Ende, produziert
einen echten `playwright-report`-Artefakt). Der Job zeigt danach weiterhin `failure` auf Job-Ebene
(non-blocking, `continue-on-error: true` — der PR-Check selbst bleibt grün) — das deckt sich mit
der bereits in Sprint 41 dokumentierten, bewusst nicht behobenen Lücke (App zeigt seit ADR-024
zuerst ein Login-Formular, 4 von 5 Assertions erwarten Post-Login-Inhalt). Kein neuer Befund, nur
jetzt erstmals real bestätigt statt nur vermutet.

**Nachtrag (2026-07-21) — Login-Lücke geschlossen, alle 5 E2E-Tests laufen jetzt echt grün:**
`frontend/tests/e2e/dashboard.spec.ts` loggt sich jetzt vor den Dashboard-Assertions selbst ein
(Seed-Admin `admin`/`ADMIN_PASSWORD`, Default `admin_dev_secret` aus `docker-compose.yml`) und
bildet den vollen Weg nach: Login → `FleetOverview` (Sprint 22, Landing-View nach Login) →
Fahrzeug auswählen (`vehicle-mock`/`vehicle-mock-2` sind in der Compose-Datei immer vorhanden) →
Teleoperate. Dabei zwei weitere reale, vorher unbekannte Bugs gefunden (nur beim echten Lauf gegen
den laufenden Stack sichtbar, nicht durch statische Prüfung): (1) `FleetVehicleDetail.tsx`
verdrahtet sowohl den „Teleoperate"- als auch den „Beobachten"-Button auf denselben
`session.startSession`-Aufruf; sobald ein Test in einem Lauf die Session eines Fahrzeugs belegt,
zeigt `FleetOverview` für dieses Fahrzeug ab dem nächsten Test „Beobachten" statt „Teleoperate"
(ADR-028) — der Test akzeptiert jetzt bewusst beide Label. (2) `text=IDLE` matchte zwei
Elemente (Header-`SystemStateBadge` und `ConnectionPanel` im `main`-Bereich rendern denselben
State getrennt) — Playwright-Strict-Mode-Violation, behoben durch Eingrenzung auf den
Header-Badge. Lokal gegen den echten Docker-Stack verifiziert (5/5 grün, `npm run test:e2e`) und
per echtem GitHub-Actions-Lauf bestätigt. Damit läuft `test-e2e.yml` jetzt vollständig grün
(weiterhin `continue-on-error: true`, non-blocking per ADR-006 — das war eine bewusste
WebRTC-Nichtdeterminismus-Entscheidung, keine Kompensation für die jetzt geschlossene Login-Lücke).

**Nicht Teil dieses Sprints:** Latenz-Gate als hartes Blocking-Gate (siehe Architektur-
Entscheidung oben), Vertiefung der Playwright-E2E-Tests bzw. echte WebRTC-SDP/ICE-E2E-Automatisierung
(Audit-Punkt F — WebRTC bleibt bewusster Nicht-Scope, analog `WEBRTC-10`), Concurrency-Test-Lücke
in `internal/webrtcsfu/sfu_test.go` und weiteren Packages (eigener, kleinerer Folge-Task), Safety
Test Suite inhaltlich auf den echten `safetyservice.Bus` ausrichten (separater Folge-Task).

**Bei der Umsetzung gefunden, nicht Teil dieses Sprints (siehe `DECISIONS.MD`):**
`BenchmarkControlACKRoundtrip` skipt immer (fehlender `session_id`-Query-Parameter, ADR-025-Drift)
— weiterhin offen, unkritisch (non-blocking Gate). Die zweite Lücke (`dashboard.spec.ts` erwartete
Dashboard-Inhalt ohne Login) wurde am 2026-07-21 geschlossen, siehe Nachtrag oben.

---

## EPIC: Testing-Debt aus ADR-006-Bestandsaufnahme schließen ✅ Sprint 42

**Freigabe (2026-07-19):** Nutzer priorisiert bei der Testing-Strategie-Bestandsaufnahme
(siehe EPIC "CI-Gates einführen") CI-Gates zuerst (Sprint 41), lässt aber zwei kleinere Funde als
offene Folgepunkte in `DECISIONS.MD` stehen (Zeilen 82/83). Sprint 42 schließt genau diese zwei
Funde. Ausgeführt in separatem Worktree (`feature/fleet-service-foundation-testdebt`), parallel zu
Sprint 41 (separater Worktree/Strang, CI-Gates) — keine Code-Überschneidung, siehe
`tasks/sprints/42-testing-debt-adr-006.md` für das vollständige Ergebnis.

**Vorrecherche (2026-07-19):**
- **Fund 1 — Concurrency-Test-Lücke `internal/webrtcsfu/sfu_test.go`:** `SFU` (`internal/webrtcsfu/
  sfu.go:44-50`) hat ein `sync.RWMutex` (`s.mu`), das `peers`/`routing`/`state`-Maps schützt —
  echter geteilter Zustand, alle Zugriffe laufen bereits korrekt durch `s.mu.Lock()`/`RLock()`.
  `sfu_test.go` (Sprint 39, 10 Tests) prüft aber nur sequenzielles Verhalten, kein Test ruft
  `HandleSessionEvent`/`registerOperatorSubscription`/`removePeer` aus mehreren Goroutinen
  gleichzeitig auf — anders als `internal/safetyservice/bus_test.go` (selber Sprint 39), das mit
  `TestBus_ConcurrentPublishAndRead` genau so einen Test hat (WaitGroup + Timeout-Channel-Helfer
  `waitOrTimeout`). Fix: identisches Testmuster in `sfu_test.go` nachbauen (Helfer lokal dupliziert,
  analog zum akzeptierten Duplikat-Präzedenzfall aus Sprint 38 `MQTTAUTH-05`, drei identische
  MQTT-Test-Helfer in separaten Testdateien).
- **Fund 2 — `tests/unit/safety_test.go` testet nicht den echten `safetyservice.Bus`:** Die 20
  Tests der "Safety Test Suite" (Datei-Header nennt sie explizit "the safety gate in CI") bauen
  `statemachine.Machine` + `mocks.MockSafetyPublisher` zusammen (`newTestSetup`, Zeile 21-33) und
  prüfen nur, dass `Publisher.PublishEvent(...)` mit dem richtigen `SafetyEventType` aufgerufen
  wird (z.B. Zeile 89 `assert.Equal(t, safetyservice.EventDeadmanTimeout, pub.LastEventType())`).
  Der eigentliche Bus (`internal/safetyservice.Bus`, Produktivcode in `cmd/safety-service/main.go`)
  wird dabei nie erreicht — `internal/controlserver/safety.HTTPPublisher` (Produktiv-Implementierung
  von `Publisher`, `internal/controlserver/safety/http_publisher.go`) schickt Events per HTTP-POST
  an `/safety/event`, das erst serverseitig `bus.PublishSafetyEvent(event)` aufruft
  (`cmd/safety-service/main.go:38-48`, `newSafetyMux`). `newSafetyMux` liegt in `package main` und
  ist daher aus `tests/unit` (package `unit_test`) nicht importierbar — Fix baut keinen
  Produktivcode um, sondern verdrahtet in einer neuen Testdatei einen minimalen lokalen
  `httptest.Server`-Handler für `POST /safety/event` (identische zwei Zeilen wie in `newSafetyMux`,
  bewusst dupliziert statt Produktivcode zu exportieren — kein Scope für einen Hexagonal-Schritt
  hier) und lässt `HTTPPublisher` (mit `baseURL` = Test-Server-URL) echte Events an einen echten
  `safetyservice.NewBus()` schicken, verifiziert über `bus.GetSafetyState()`.
- **CLAUDE.MD-Leitplanken**: Abschnitt 17 (Teststandard) fordert für Typ M/L explizit
  "Nebenläufigkeit" als Fallgruppe (Fund 1) sowie Integrationstests gegen reale Abhängigkeiten statt
  In-Memory-Mocks wo sinnvoll (Fund 2 — `HTTPPublisher` gegen echten `Bus` statt nur gegen
  `MockSafetyPublisher`).

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| SFUCONC-01 | `internal/webrtcsfu/sfu_test.go`: neuer Concurrency-Test (mehrere Goroutinen rufen `HandleSessionEvent`/`registerOperatorSubscription`/`removePeer` gleichzeitig auf), lokaler `waitOrTimeout`-Helfer analog `bus_test.go`, `go test -race` grün. | S | ✅ Sprint 42 | — |
| SAFETYBUS-01 | Neue Testdatei `tests/unit/safety_bus_integration_test.go`: minimaler lokaler `httptest.Server`-Handler für `POST /safety/event` (dupliziert `newSafetyMux`s zwei Zeilen, kein Produktivcode-Umbau), `HTTPPublisher` gegen echten `safetyservice.NewBus()` verdrahtet, mind. 2 ADR-006-CRITICAL-Szenarien (Dead-man-Timeout, ACK-Timeout) verifiziert über `bus.GetSafetyState()`. | S/M | ✅ Sprint 42 | — |
| SAFETYBUS-02 | Datei-Header-Kommentar in `tests/unit/safety_test.go` präzisieren: bestehende 20 Tests decken Trigger-Logik (State-Machine → `Publisher`) ab, nicht den Bus selbst; Verweis auf die neuen Bus-Integrationstests aus `SAFETYBUS-01`. `docs/adr/006-testing-strategy.md` falls dort die Suite beschrieben wird, ebenfalls präzisieren. | S | ✅ Sprint 42 | SAFETYBUS-01 |
| TESTDEBT-VERIFY-01 | Verifikation: `go build ./...`, `go vet ./...`, `go test ./... -race` (mind. 2x gegen Flakiness, CLAUDE.MD §17), `DECISIONS.MD`-Zeilen 82/83 auf ✅, `tasks/backlog.md`-Status-Update. | S | ✅ Sprint 42 | SFUCONC-01, SAFETYBUS-01, SAFETYBUS-02 |

**Nicht Teil dieses Sprints:** Umbau von `cmd/safety-service/main.go`s `newSafetyMux` in ein
exportiertes/testbares Konstrukt (Hexagonal-artiger Schritt, eigener Entscheidungspunkt falls
später gewünscht), Abdeckung aller 5 ADR-006-CRITICAL-Szenarien gegen den echten Bus (nur die 2
wichtigsten Dead-man/ACK-Timeout, Rest bleibt Mock-basiert), Concurrency-Tests für weitere Packages
über `webrtcsfu` hinaus.

---

## EPIC: Hexagonale Architektur-Migration — Schritt 3, telemetry-service (ADR-031) ✅ Sprint 43 abgeschlossen

Strategie/Priorisierung: [ADR-031](../docs/adr/031-hexagonal-architecture-migration.md), Update
"Sprint-43-Kickoff". Fortsetzung nach Pilot (fleet-service, ✅ Sprint 33) und Schritt 2
(auth-service, ✅ Sprint 37) — Nutzer gibt Fortsetzung mit `telemetry-service` (Schritt 3 der
ADR-031-Priorisierung) am 2026-07-19 frei, nachdem dieser Entscheidungspunkt zuvor mehrfach
zurückgestellt wurde. Details/Ergebnisse: `tasks/sprints/43-hexagonal-migration-telemetry-service.md`.

**Vorrecherche (2026-07-19):**
- `internal/telemetryservice/client.go` (114 Zeilen, gesamter Service): `Client.client` (Zeile 29)
  ist vom Typ `mqtt.Client` — das ist bereits ein Interface, aber eines aus der Drittanbieter-
  Bibliothek `paho.mqtt.golang` (14 Methoden, u.a. `Publish`/`AddRoute`/`OptionsReader`, die
  telemetryservice nie nutzt). Kein projekteigener, schmaler Port — Verstoß gegen GOSTYLE Rule 2.2
  (Interface-Segregation) und ADR-031-Regel 3 (Driven Ports vom Consumer definiert, nicht vom
  Adapter/Anbieter). `Connect()` (Zeile 44-64) baut `mqtt.NewClientOptions()...` und ruft
  `c.client.Connect()`/`token.Wait()`/`token.Error()` direkt auf; `subscribe()` (Zeile 66-74)
  ebenso mit `mqtt.Token`.
- **`internal/telemetryservice` und `cmd/telemetry-service` haben aktuell 0 Tests** (`find
  internal/telemetryservice cmd/telemetry-service -name "*_test.go"` liefert nichts) — derselbe
  Bestandsaufnahme-Befund wie bei `safety-service`/`webrtc-sfu`/`internal/recording` vor Sprint 39.
  Grund: `Connect()`/`subscribe()` sind ohne echten MQTT-Broker nicht sinnvoll testbar, solange sie
  direkt gegen `mqtt.Client` programmieren. Ein projekteigener `MQTTConnection`-Port löst dieses
  Problem als direkten Nebeneffekt der Migration (kein separater Testabdeckungs-Sprint nötig).
- `handleMessage`/`GetLatest` (Zeile 76-107) sind bereits reine Domain-Logik (Proto-Parse,
  Map-Zugriff hinter `sync.RWMutex`) ohne I/O — laut ADR-031-Bestandsaufnahme "triviales reines
  Passthrough". Keine Use-Case-Extraktion nötig (anders als `HEX-06`/`HEXAUTH-04`, die für
  fleet-/auth-service optional zur Debatte stehen).
- `cmd/telemetry-service/main.go:29` (`telemetryservice.NewClient(broker, username, password)`) —
  externe Konstruktorsignatur bleibt unverändert, analog zum Präzedenzfall `HEX-02`/`HEXAUTH-02`
  (`*PostgresFleetStore` bzw. `JWTTokenIssuer` wurden beide intern konstruiert, ohne den Aufrufer
  anzupassen).
- `cmd/telemetry-service/main.go:38-79` (`newTelemetryMux`) — analoges Muster zu den in Sprint 39
  getesteten `newSafetyMux`/`newSFUMux`, aber bisher ungetestet.

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| HEXTELE-01 | `MQTTConnection`-Port definieren (neue Datei `internal/telemetryservice/mqttconnection.go`): schmales Interface (`Connect() error`, `Subscribe(topic string, qos byte, handler func(topic string, payload []byte)) error`, `Disconnect(quiesceMs uint)`, `IsConnected() bool`) statt direkter Kopplung an `paho.mqtt.golang`s `mqtt.Client`. `PahoConnection`-Adapter kapselt `mqtt.Token`/`mqtt.Message`-Handling intern. Compile-Time-Check `var _ MQTTConnection = (*PahoConnection)(nil)`. | S | ✅ Sprint 43 | — |
| HEXTELE-02 | `Client.client` (`internal/telemetryservice/client.go:29`) von `mqtt.Client` auf `MQTTConnection`-Port umgestellt. `Connect()`/`subscribe()`/`Disconnect()` rufen den Port statt paho direkt auf. `NewClient`-Signatur unverändert, Adapter wird intern konstruiert (`cmd/telemetry-service/main.go:29` unangetastet, analog `HEX-02`/`HEXAUTH-02`). | S | ✅ Sprint 43 | HEXTELE-01 |
| HEXTELE-03 | Erste Testabdeckung `internal/telemetryservice` + `cmd/telemetry-service` (aktuell 0 Tests): `FakeMQTTConnection`-Testdoppel; Tests für `handleMessage` (Proto-Parse, fehlender/leerer `vehicle_id`, Malformed Payload), `GetLatest`, `Connect`/`subscribe`-Wiring über den Fake (kein echter Broker nötig); `main_test.go` für `newTelemetryMux` via `httptest` (analog Sprint-39-Muster `main_test.go` für `safety-service`/`webrtc-sfu`). | M | ✅ Sprint 43 | HEXTELE-02 |
| HEXTELE-04 | Verifikation: `go build ./...`, `go vet ./...`, `go test ./internal/telemetryservice/... ./cmd/telemetry-service/... -race` (mind. 2x gegen Flakiness, CLAUDE.MD §17). ADR-031-Status-Update (Schritt 3 abgeschlossen, analog Sprint-37/HEXAUTH-03-Eintrag), `DECISIONS.MD`, `tasks/backlog.md`-Status-Update. | S | ✅ Sprint 43 | HEXTELE-03 |

**Nicht Teil dieses Sprints:** Use-Case-Extraktion (kein Bedarf, siehe Vorrecherche —
`handleMessage`/`GetLatest` sind bereits reine Funktionen), `control-server` (Schritt 4/5 der
ADR-031-Priorisierung, ausdrücklich zuletzt/eigenes ADR nötig), `safety-service`/`webrtc-sfu`/
`recording` (Hexagonal-Migration dort, unabhängig von der bereits vorhandenen Sprint-39-
Testabdeckung, ein separater Entscheidungspunkt).

---

## EPIC: control-server Hexagonal-Migration — Vorbereitung (ADR-035) ✅ Sprint 46

**Freigabe (2026-07-20):** Nutzerentscheidung gegenüber zwei Alternativen (Hexagonal-Migration
safety-service/webrtc-sfu/recording — verworfen, keine vergleichbare Infrastruktur-Abhängigkeit
dort; `newSafetyMux` exportieren — zu klein für einen eigenen Sprint). Schließt den seit ADR-031
offenen Punkt "control-server bräuchte eigenes ADR + Testabdeckung-Aufbau zuerst".

Vorrecherche korrigierte die ursprüngliche "geringste Testabdeckung"-Annahme: `go test
./tests/unit/... -coverpkg=./internal/controlserver/...` zeigt ~84% Coverage für die
`internal/controlserver`-Subpakete (State Machine, alle 5 Watchdogs, Session-/Handover-Manager —
bereits solide über `tests/unit/*.go` getestet). Die echte Lücke ist `cmd/control-server/main.go`
selbst (919 Zeilen, `package main`, 0% Coverage auf jeder Funktion) sowie `authcheck.Checker` und
`transport/websocket.go`. Details siehe `docs/adr/035-control-server-hexagonal-migration-prep.md`
und `tasks/sprints/46-control-server-hexagonal-migration-prep.md`.

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| CTRL-01 | Neues ADR `docs/adr/035-control-server-hexagonal-migration-prep.md` — korrigierte Risikoanalyse + Scope-Vorschlag/empfohlene Reihenfolge für eine spätere echte Migration. | S | ✅ Sprint 46 | — |
| CTRL-02 | `requireJWT`-Tabellentest (`cmd/control-server/main_test.go`, neu). | S | ✅ Sprint 46 | — |
| CTRL-03 | Test-Fixture `newTestControlServer(t)` + `handleHealth`-Test. | M | ✅ Sprint 46 | — |
| CTRL-04 | `handleSessionStart`/`advanceVehicleToActiveOperator`-Tests. | M | ✅ Sprint 46 | CTRL-03 |
| CTRL-05 | `handleSessionEnd`-Tests. | M | ✅ Sprint 46 | CTRL-03 |
| CTRL-06 | `handleEmergencyStop`-Tests — inkl. Fund: `IDLE→SAFE_MODE` kein gültiger State-Transition, Emergency-Stop auf Fahrzeug ohne aktive Session wird lautlos ignoriert (dokumentiert, nicht behoben). | M | ✅ Sprint 46 | CTRL-03 |
| CTRL-07 | `internal/controlserver/authcheck/checker_test.go` — `DATABASE_URL`-gated gegen echte `users`-Tabelle. | S | ✅ Sprint 46 | — |
| CTRL-08 | Verifikation + Doku-Updates (ADR-031-Cross-Reference, `DECISIONS.MD`, dieser Eintrag). | S | ✅ Sprint 46 | CTRL-01..07 |

**Nicht Teil dieses Sprints:** `internal/controlserver/transport/websocket.go`, die restlichen
~20 HTTP-Handler in `main.go`, die DB-touchende Bootstrap-Logik (`loadConfig`/`newAuditWriter`/
`newVehicleStore`/`newControlServer`/`main()`), der Fix für den `IDLE→SAFE_MODE`-Fund, die
eigentliche Hexagonal-Migration (Handler-Struct-Extraktion) selbst — alle eigene, noch offene
Folge-Entscheidungspunkte (siehe ADR-035 "Nächste Schritte").

---

## EPIC: Backup-Strategie Audit Store (ADR-018/023 Folge) ✅ Sprint 44

**Freigabe (2026-07-19):** Nutzer wählt dieses Thema für Sprint 44 gegenüber zwei Alternativen
(Migration zu AWS ECR, Session-Recording-Storage-Entscheidung). Schließt den seit ADR-019
offenen Punkt "Audit Store Backup-Strategie". **Eingereiht nach Sprint 43** — wird erst zu
Sprint 44, sobald Sprint 43 abgeschlossen ist, `tasks/current-sprint.md` bleibt bis dahin
unverändert.

**Wichtiger Vorrecherche-Befund — Backlog-Text war veraltet:** `tasks/backlog.md` und
`docs/adr/019-deployment-strategy.md` beschrieben den offenen Punkt bisher als "SQLite Volume auf
S3" (Stand ADR-018, Sprint 7). Tatsächlich hat ADR-023 (PostgreSQL-Migration) SQLite bereits
vollständig ersetzt: `audit_events` liegt seither in PostgreSQL (`postgres-data`-Docker-Volume,
`infrastructure/compose/docker-compose.prod.yml:31-46`), das `audit-data`-Volume aus ADR-018
existiert im Compose-Setup nicht mehr. `cmd/control-server/main.go:81-89` (`newAuditWriter`) nutzt
`audit.NewPostgresAuditWriter(db)`, kein SQLite-Pfad mehr im Code. ADR-023 selbst nennt
"Standardisierte Backup-Workflows (`pg_dump`)" bereits als erwarteten Vorteil der Migration — der
Backup-Task war seither nur nie eingeplant. Beide Doku-Stellen oben in diesem Sprint bereits auf
"Postgres-Volume" korrigiert.

**Architektur-Entscheidung (bei der Planung getroffen):**
- **`pg_dump` gegen den laufenden `postgres`-Container statt Datei-Kopie des Docker-Volumes** —
  ein Volume-Snapshot während laufendem Betrieb kann inkonsistent sein (kein atomarer Zustand),
  `pg_dump` liefert einen konsistenten logischen Dump zur Laufzeit, ohne den Service zu stoppen.
  Kein `pg_basebackup`/WAL-Archivierung (Point-in-Time-Recovery) — für dieses Betriebsmodell
  (Single-Instance-Testbetrieb, kein HA-Anspruch) ist ein tägliches logisches Backup ausreichend,
  analog zur bereits akzeptierten "kein Cross-Host-Bedarf"-Argumentation aus Sprint 40.
- **S3-Bucket-Name per SSM statt hartkodiert** — der bestehende `AppBucket` (CDK,
  `infrastructure/AWS/cdk_server-stack.ts:75-79`, bereits `grantReadWrite` für die Instance-Role,
  siehe Kommentar Zeile 138 "für zukünftige Audit-Log-Backups, ADR-018") bekommt einen neuen
  `ssm.StringParameter` unter `/avoc/prod/backup-bucket-name` direkt im selben CDK-Stack (kein
  manueller Zusatzschritt nach `cdk deploy` nötig) — konsistent mit dem bestehenden
  SSM-getriebenen Secret-Verteilungsmuster in `scripts/deploy.sh`.
- **S3-Lifecycle-Regel statt manueller Löschung** — Backups unter Prefix `backups/postgres/`
  verfallen nach 30 Tagen automatisch (`lifecycleRules` im CDK-Stack), damit der ohnehin schon
  `versioned: true`/`autoDeleteObjects: true` konfigurierte Bucket nicht unbegrenzt wächst.
- **Tägliches Cron-Backup auf dem EC2-Host statt Container-internem Scheduler** — einfachste
  Lösung ohne neuen Docker-Compose-Service, analog zur bestehenden "generiere/registriere einmalig
  bei Deploy"-Philosophie in `scripts/deploy.sh` (SSL-Zertifikat, Mosquitto-Passwd).

**Vorrecherche (2026-07-19):**
- `infrastructure/compose/docker-compose.prod.yml:31-46`: `postgres`-Service, `POSTGRES_USER=avoc`,
  `POSTGRES_DB=avoc`, `POSTGRES_PASSWORD=${DB_PASSWORD}` (aus `$APP_DIR/.env`, nicht SSM — siehe
  `scripts/deploy.sh` Kommentar Zeile 61-62), Healthcheck `pg_isready -U avoc -d avoc`.
- `infrastructure/AWS/cdk_server-stack.ts:75-79`: `AppBucket` (`s3.Bucket`, `versioned: true`,
  `removalPolicy: DESTROY`, `autoDeleteObjects: true`), Zeile 139 `bucket.grantReadWrite(instance.role)`
  bereits vorhanden. Zeile 203-205: `CfnOutput BucketName` existiert bereits, aber landet aktuell
  nur in der CDK-Konsolenausgabe, nicht in SSM — Backup-Skript bräuchte sonst den Bucket-Namen
  hartkodiert oder manuell in `.env` gepflegt.
- `scripts/deploy.sh:38-51` (`get`/`get_secure`-Helfer für SSM-Parameter) — Muster für den neuen
  `/avoc/prod/backup-bucket-name`-Parameter wiederverwendbar.
- `scripts/deploy.sh:94-115` (SSL-Zertifikat-Generierung, "einmalig generieren, wiederverwenden")
  als Strukturvorbild für ein neues Skript `scripts/backup-audit-store.sh` (hier aber täglich
  ausgeführt statt einmalig).
- CDK verwendet `aws-cdk-lib/aws-s3` bereits (Zeile 5); `aws-cdk-lib/aws-ssm` (für
  `ssm.StringParameter`) ist als Teil von `aws-cdk-lib` bereits verfügbar, kein neues
  `package.json`-Dependency nötig.
- `docs/adr/023-postgresql-migration.md:71` nennt "Standardisierte Backup-Workflows (`pg_dump`)"
  bereits explizit als erwarteten Vorteil — dieser Sprint löst genau dieses Versprechen ein.

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| AUDITBACKUP-01 | CDK-Stack (`infrastructure/AWS/cdk_server-stack.ts`): neuer `ssm.StringParameter` (`/avoc/prod/backup-bucket-name` = `bucket.bucketName`) + S3-Lifecycle-Regel (Prefix `backups/postgres/`, Expiration 30 Tage) auf `AppBucket`. | S | ✅ Sprint 44 | — |
| AUDITBACKUP-02 | Neues Skript `scripts/backup-audit-store.sh`: liest Bucket-Namen aus SSM (`get`-Helfer analog `deploy.sh`), `docker compose exec postgres pg_dump -U avoc avoc \| gzip`, Upload via `aws s3 cp` nach `s3://$BUCKET/backups/postgres/$(date +%F)-avoc.sql.gz`. | S | ✅ Sprint 44 | AUDITBACKUP-01 |
| AUDITBACKUP-03 | Cron-Registrierung: `scripts/deploy.sh` ergänzt einen idempotenten Crontab-Eintrag (täglich, z.B. 03:00 UTC) für `backup-audit-store.sh` unter `ec2-admin` — Prüfung auf Doppel-Registrierung analog zum "generiere einmalig"-Muster (`crontab -l \| grep -q ... \|\| ...`). | S | ✅ Sprint 44 | AUDITBACKUP-02 |
| AUDITBACKUP-04 | Verifikation: lokaler Trockenlauf von `backup-audit-store.sh` gegen den Dev-`postgres`-Container (Dump + lokale Datei, kein echter S3-Upload nötig für den Test). Doku: `docs/adr/019-deployment-strategy.md` Zeile "Audit Store Backup-Strategie" auf ✅, `DECISIONS.MD`, `tasks/backlog.md`-Status-Update. | S | ✅ Sprint 44 | AUDITBACKUP-01..03 |

**Nicht Teil dieses Sprints:** Point-in-Time-Recovery (`pg_basebackup`/WAL-Archivierung — kein
HA-Anspruch für Single-Instance-Testbetrieb), automatisierter Restore-Test/-Runbook (eigener
Folge-Task, sobald ein erstes echtes Backup vorliegt), Verschlüsselung des Dumps vor Upload
(Bucket-seitige S3-Default-Encryption gilt bereits, kein zusätzlicher Client-seitiger Schritt in
diesem Scope).

---

## EPIC: Lokale Ansible-VM als Hetzner-Nachbildung (AWS-Ersatz für die Testumgebung)

**Status:** ✅ EPIC vollständig abgeschlossen. Teil A (Autorierung, LOCALVM-01..07) Sprint 48 —
siehe [tasks/sprints/48-lokale-ansible-vm-teil-a.md](sprints/48-lokale-ansible-vm-teil-a.md). Teil B
(Verifikation gegen die echte VM, LOCALVM-08a/08b/08c/09) Sprint 49 — siehe
[tasks/sprints/49-lokale-ansible-vm-teil-b.md](sprints/49-lokale-ansible-vm-teil-b.md). Kompletter
Ansible-Lauf (`site.yml`+`deploy.yml`) real gegen eine lokale VM verifiziert, alle 15
Stack-Container laufen stabil, Frontend/Control-Server-API HTTP-erreichbar; 5 reale, vorher
unbekannte Bugs gefunden und behoben (siehe Sprint-49-Ergebnisse); `hetzner-setup.md` auf den
Ansible-Workflow umgestellt.

**Freigabe (2026-07-20):** Nutzer möchte AWS nicht länger als lokale Test-/Referenzumgebung
nutzen, sondern stattdessen eine **lokale VM per Ansible** provisionieren, die den **zukünftigen
Hetzner-Server** nachbildet (`docs/deployment/hetzner-setup.md`) — das wird die neue
Testumgebung für Deployment-Änderungen, bevor echtes Geld für einen echten Hetzner-Server
ausgegeben wird. Nutzer hat die Umsetzung per Sprints + spawnbaren Agenten angefordert (kein
manuelles Schritt-für-Schritt mehr).

**Wichtiger Vorrecherche-Befund — der "zukünftige Hetzner-Server" existiert bisher nur als
Dokumentation, nicht als Code:** `docs/deployment/hetzner-setup.md` (659 Zeilen) beschreibt den
kompletten Hetzner-Weg, aber `scripts/deploy-hetzner.sh`, `scripts/secrets-setup-hetzner.sh` und
`infrastructure/compose/docker-compose.hetzner.yml` existieren **nicht** als committete Dateien —
sie sind bisher nur Copy-Paste-Codeblöcke innerhalb des Markdown-Dokuments (`grep -r
"deploy-hetzner\|hetzner.yml" scripts/ infrastructure/` liefert 0 Treffer außerhalb der
Doku). Dieses EPIC materialisiert diese Dateien erstmals als echten, versionierten Code — und
verifiziert sie zum ersten Mal überhaupt gegen eine echte laufende Instanz (die lokale VM),
bevor sie je gegen einen echten, kostenpflichtigen Hetzner-Server laufen.

**Architektur-Entscheidung (bei der Planung getroffen):**
- **libvirt/KVM + `virt-install` + cloud-init statt Vagrant/VirtualBox** — auf diesem
  Linux-Entwicklungsrechner ist `libvirtd` bereits aktiv, `virsh`/`virt-install`/
  `qemu-system-x86_64` sind installiert, der Nutzer ist bereits Mitglied der Gruppen
  `libvirt`/`kvm` (`lsmod | grep kvm` zeigt `kvm_amd` geladen, `/dev/kvm` vorhanden). Kein
  zusätzliches Hypervisor-Tooling nötig. Zusätzlicher Vorteil gegenüber Vagrant: **cloud-init
  (NoCloud-Datenquelle) ist exakt der Mechanismus, den Hetzner Cloud selbst für neu angelegte
  Server verwendet** (SSH-Key-Injection, Hostname, User-Setup) — die lokale VM startet damit
  authentisch wie ein echter frisch angelegter Hetzner-Server, nicht wie eine Vagrant-Box mit
  eigenen Abweichungen.
- **Ansible statt weiterer Bash-Copy-Paste-Blöcke** — `hetzner-setup.md` Schritt 3 (Bootstrap:
  Docker, `avoc`-User, Verzeichnisse) ist aktuell ein einziger, nicht idempotenter Bash-Block,
  der manuell in eine SSH-Session eingefügt wird. Als Ansible-Rolle wird er wiederholbar,
  versioniert, diff-fähig — und exakt derselbe Playbook-Lauf funktioniert später 1:1 gegen den
  echten Hetzner-Server (nur das Inventory ändert sich, IP statt VM).
- **`docker-compose.hetzner.yml`/`deploy-hetzner.sh`/`secrets-setup-hetzner.sh` werden als echte
  Dateien aus der Doku extrahiert, nicht neu erfunden** — die Doku hat die Logik bereits
  vollständig durchdacht (coturn-Unterschied zu EC2, Loki/Promtail-Pfade, `.env`-Handling); dieses
  EPIC übernimmt sie 1:1 in echten Code und lässt Ansible sie ausrollen/ausführen, statt sie
  manuell per `scp`/SSH zu kopieren.
- **Lokale Test-Dummy-Secrets statt interaktivem `secrets-setup-hetzner.sh`-Prompt** — für die
  lokale VM (kein echtes Internet-exponiertes System, keine echten Nutzerdaten) generiert Ansible
  die `.env` aus fest hinterlegten, eindeutig als Test gekennzeichneten Werten (analog zum
  bestehenden Muster committeter Test-Credentials, z. B. `tests/mosquitto-passwd-test`) —
  spart die interaktive Eingabe bei jedem VM-Neuaufbau. Das echte, interaktive
  `secrets-setup-hetzner.sh` bleibt unverändert für den späteren echten Hetzner-Server nutzbar.
- **Kein automatischer Docker-Hub-Roundtrip für die lokale Verifikation** — `hetzner-setup.md`
  Schritt 5 baut Images und pusht sie nach Docker Hub, damit der Server sie pullen kann. Für die
  rein lokale VM auf demselben Host ist das ein unnötiger Netzwerk-Umweg; stattdessen werden
  Images lokal gebaut und direkt in die VM übertragen (`docker save`/`virsh`-Dateitransfer bzw.
  gemeinsames Netzwerk + `docker load`), analog zum bereits etablierten EC2-Muster
  ("Übergabe-Abweichung von ADR-019", `docs/deployment/UEBERGABE-ABWEICHUNGEN.md`). Der reguläre
  Docker-Hub-Weg bleibt für den echten Hetzner-Server unverändert.

**Vorrecherche (2026-07-20):**
- `docs/deployment/hetzner-setup.md` — vollständige Referenz für alle 9 Schritte (Server anlegen,
  Firewall, Bootstrap, Secrets, Images, Config-Kopie, Deploy-Script, Compose-Unterschiede zu EC2,
  erster Deploy). Ziel-OS: Ubuntu 24.04 LTS, empfohlene Größe CPX21 (3 vCPU/4 GB) — für die lokale
  VM als Näherung übernommen, ggf. an verfügbare Host-Ressourcen angepasst.
- `docs/deployment/ec2-bootstrap.md` — Vergleichsmuster für die bereits bestehende, dokumentierte
  Verzeichnisstruktur/`scp`-Liste (AWS-Pfad); dieses EPIC folgt für Hetzner demselben Aufbau, nur
  Ansible statt `scp`+manuellem SSH.
- `infrastructure/compose/docker-compose.prod.yml` — Ausgangsbasis für
  `docker-compose.hetzner.yml` (laut Doku: kopieren + 5 gezielte Änderungen, siehe
  hetzner-setup.md Schritt 8 Tabelle: coturn `--relay-ip` entfällt, `--external-ip` ohne
  NAT-Mapping, Loki/Promtail/Grafana-Volume-Pfade auf `/home/avoc/...` statt `../...`).
  Fleet-Service ist (Stand jetzt, wie in EC2-Doku vermerkt) nicht in `docker-compose.prod.yml`
  enthalten — dieselbe Lücke gilt für die Hetzner-Variante, kein neuer Befund.
- Lokale Virtualisierungsumgebung bereits verifiziert vorhanden: `libvirtd` aktiv,
  `/dev/kvm` vorhanden, `virt-install`/`virsh`/`qemu-system-x86_64` installiert, Nutzer in Gruppen
  `libvirt`+`kvm`. Kein `ansible`/`ansible-playbook` installiert — wird Teil von Sprint A.
- `scripts/deploy.sh` (EC2) und die darin etablierten Muster ("get"-SSM-Helfer, "einmalig
  generieren/registrieren, dann wiederverwenden" für SSL-Zertifikat) sind für Hetzner nicht 1:1
  übertragbar (kein SSM auf Hetzner) — `hetzner-setup.md` Schritt 4/7 hat dafür bereits eigene,
  SSM-freie Skript-Entwürfe (`.env`-Datei statt SSM, siehe Architektur-Entscheidung oben).

**Umsetzung als zwei Sprints** (Größe/Token-Budget, analog bestehender Sprint-Segmentierungs-Praxis
in diesem Projekt):
- **Sprint A — Autorierung** (kein Live-VM-Zugriff nötig, gut parallelisierbar): Ansible-Grundgerüst,
  alle Rollen/Playbooks, VM-Erzeugungsskript, Materialisierung der drei bisher nur dokumentierten
  Hetzner-Dateien.
- **Sprint B — Verifikation** (braucht die echte lokale VM, sequenziell): VM tatsächlich anlegen,
  kompletten Ansible-Lauf + Stack-Deploy gegen sie fahren, Smoke-Test, Doku aktualisieren
  (`hetzner-setup.md` verweist danach auf den Ansible-Workflow statt auf Copy-Paste-Blöcke).

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| LOCALVM-01 | Ansible-Grundgerüst: neues Verzeichnis `ansible/` (`ansible.cfg`, `inventory/`, `requirements.yml`, `roles/`-Skeleton), `ansible`/`ansible-playbook` lokal installieren/dokumentieren (z. B. `pipx`/venv, kein System-weites `pip install` als root) | S | ✅ Sprint 48 | — |
| LOCALVM-02 | Neues Skript `scripts/local-vm-create.sh`: `virt-install` + cloud-init (NoCloud-ISO: SSH-Key, Hostname, User) für eine Ubuntu-24.04-Cloud-Image-VM, Netzwerk/IP-Ermittlung für das Ansible-Inventory | M | ✅ Sprint 48 | LOCALVM-01 |
| LOCALVM-03 | Ansible-Rolle `bootstrap`: Docker-Installation, `avoc`-User, Verzeichnisstruktur — Ablösung von `hetzner-setup.md` Schritt 3 (Bash-Block) durch idempotente Tasks | M | ✅ Sprint 48 | LOCALVM-01 |
| LOCALVM-04 | Ansible-Rolle `firewall`: `ufw`-Regeln analog der Port-Tabelle aus `hetzner-setup.md` Schritt 2 | S | ✅ Sprint 48 | LOCALVM-01 |
| LOCALVM-05 | Ansible-Rolle `secrets`: `.env` aus fest hinterlegten Test-Dummy-Werten templaten (siehe Architektur-Entscheidung), SSL-Selfsigned-Zertifikat wie in `secrets-setup-hetzner.sh` beschrieben | S/M | ✅ Sprint 48 | LOCALVM-01 |
| LOCALVM-06 | Materialisierung als echte Dateien: `infrastructure/compose/docker-compose.hetzner.yml` (aus `docker-compose.prod.yml` + den 5 dokumentierten Änderungen), `scripts/deploy-hetzner.sh`, `scripts/secrets-setup-hetzner.sh` (1:1 aus `hetzner-setup.md`-Codeblöcken übernommen) | M | ✅ Sprint 48 | — |
| LOCALVM-07 | Ansible-Playbook `deploy.yml`: Config-Dateien (Mosquitto/MediaMTX/Loki/Promtail/Grafana) auf die VM bringen, lokal gebaute Images übertragen (kein Docker-Hub-Roundtrip, siehe Architektur-Entscheidung), `deploy-hetzner.sh` ausführen | M | ✅ Sprint 48 | LOCALVM-03..06 |
| LOCALVM-08a | VM-Erzeugung: `scripts/local-vm-create.sh` real ausführen, Ansible-Inventory-Befüllung verifizieren | M | ✅ Sprint 49 | LOCALVM-01..07 |
| LOCALVM-08b | Ansible-Lauf: `site.yml`+`deploy.yml` real gegen die VM ausführen, Fehler iterativ beheben | M | ✅ Sprint 49 | LOCALVM-08a |
| LOCALVM-08c | Smoke-Test gegen den laufenden Stack (`docker compose ps`, Frontend :3000, Control-Server-API :8080) | S | ✅ Sprint 49 | LOCALVM-08b |
| LOCALVM-09 | Doku: `docs/deployment/hetzner-setup.md` auf den Ansible-Workflow umstellen (Schritt 1/3/4/6/7 durch Verweis auf `local-vm-create.sh`+Ansible-Rollen ersetzt, Rest bleibt für den echten Server gültig), Mosquitto-Port/`UEBERGABE-ABWEICHUNGEN.md`-Lücken aus Sprint 48 mitkorrigiert, `DECISIONS.MD`, `tasks/backlog.md`-Status-Update | S | ✅ Sprint 49 | LOCALVM-08c |

**Nicht Teil dieses Vorhabens:** ein echter, kostenpflichtiger Hetzner-Cloud-Server (bleibt
Doku/manueller Schritt, keine automatisierte Bestellung — reale Cloud-Kosten sind keine
Agenten-Entscheidung); vollständige Stilllegung des bestehenden AWS-CDK/EC2-Pfads (Sprint 44 hat
ihn gerade erst erweitert — Koexistenz, kein Rückbau); CI-Integration der lokalen VM (die
Sprint-41-Pipeline bleibt GitHub-hosted-Runner-basiert, ein lokales libvirt-Ziel ist ohne
Self-Hosted-Runner nicht CI-fähig — eigener, größerer Folge-Task falls gewünscht); Let's-Encrypt/
Domain-Anbindung (ergibt für eine rein lokale VM ohne öffentliche IP keinen Sinn, bleibt für den
echten Server in `hetzner-setup.md` dokumentiert).

---

## EPIC: CI-Image-Publishing nach GHCR (Sprint 56, ✅ vollständig — CIPUB-04 als Nachtrag 2026-07-22)

**Auftrag (2026-07-22):** Nutzer möchte die `avoc-*`-Images zusätzlich in GitHub Actions bauen
lassen — bisher werden sie ausschließlich lokal gebaut (Voraussetzung für `ansible/deploy.yml`,
s. EPIC "Lokale Ansible-VM..." oben). Baut auf CIHARD-03 (Sprint 55, `docker-build.yml`) auf: die
Build-Verifikation für alle 8 Targets existiert bereits, nur noch ohne Push
(`push: false`, Zeile 42/58/74 in `.github/workflows/docker-build.yml`).

**Nutzerentscheidungen (Rückfrage 2026-07-22):**
- **Registry: GHCR** (`ghcr.io`), nicht Docker Hub — nutzt den eingebauten `GITHUB_TOKEN`, kein
  zusätzliches Secret nötig.
- **Bestehenden Job erweitern**, kein neuer separater Workflow — `docker-build.yml` bekommt einen
  Push-Schritt statt einer zweiten, dupliziert-parallelen Job-Definition für dieselben 8 Targets.

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| CIPUB-01 | `.github/workflows/docker-build.yml` erweitern: `docker/login-action` gegen `ghcr.io` (`GITHUB_TOKEN`), `permissions: packages: write` auf Job-Ebene, `push: true` nur bei `push`-Event auf `main` (bei `pull_request` weiterhin `push: false` — keine Images von unverifizierten PR-Branches veröffentlichen). Tag-Schema festlegen (z. B. `ghcr.io/<owner>/avoc-<service>:latest` + `:<git-sha>`). | M | ✅ Sprint 56 | CIHARD-03 (Sprint 55) |
| CIPUB-02 | Kurzer Hinweis-Kommentar in `ansible/deploy.yml`-Kopf und/oder `docs/deployment/UEBERGABE-ABWEICHUNGEN.md`, dass GHCR-Images jetzt existieren, **ohne** den bestehenden lokalen Build+`docker save`/`load`-Weg in diesem Sprint umzustellen (reine Doku-Ergänzung, kein Funktionsumbau). | S | ✅ Sprint 56 | CIPUB-01 |
| CIPUB-03 | Verifikation: `actionlint` gegen die geänderte Workflow-Datei, nach echtem Push auf `main` prüfen, dass die Packages tatsächlich unter GHCR erscheinen (Sichtbarkeit public/private je nach Repo-Einstellung explizit dokumentieren), Doku-/Backlog-Status-Update. | S | ✅ Sprint 56 (lokaler Teil: `actionlint` 0 Findings; Post-Merge-GHCR-Check bleibt offener Schritt, s. `tasks/sprints/56-ci-image-publishing.md`) | CIPUB-01 |
| CIPUB-04 | **CIGATE-06 abschließen** (seit Sprint 41 vorbereitet, nie aktiviert): Branch-Protection/Required-Status-Checks für die 4 blockierenden Jobs real im GitHub-Repo-Setting einschalten — nur nach nochmaliger expliziter Nutzerbestätigung unmittelbar vor der Aktivierung (betrifft alle künftigen PRs). Auf Nutzerwunsch (2026-07-22) in diesen Sprint mitaufgenommen, thematisch unabhängig von CIPUB-01..03. | S | ✅ 2026-07-22 (Nachtrag nach Sprint-56-Merge) | CIGATE-06 (Vorbereitung Sprint 41) |

**Nicht Teil dieses Sprints:** Umstellung von `ansible/deploy.yml` auf GHCR-Pull statt lokalem
Build+Transfer (separate, größere Architektur-Entscheidung — betrifft die "Kein automatischer
Docker-Hub-Roundtrip"-Entscheidung im Ansible-EPIC oben, nicht rückstandslos ohne erneute
Abwägung zu ändern); Docker-Hub als Alternative (Nutzerentscheidung: GHCR); Vergabe von
Image-Retention-/Cleanup-Policies in GHCR.

**Nachtrag CIPUB-04 (2026-07-22, nach dem Sprint-56-Merge):** Ursprünglich blockiert — weder der
github-MCP-Server (Default-Toolsets ohne `actions`, kein Branch-Protection-Tool in keinem
Toolset) noch das vorhandene Fine-Grained-Token (ohne `Administration`) reichten aus. Behoben:
`gh`-CLI installiert (`sudo apt-get install -y gh`, Ubuntu-`universe`-Repo), Token um
`Administration: Read and write` erweitert (Nutzeraktion), `gh auth login --with-token`. Erster
PUT-Versuch schlug mit `403 Resource not accessible` fehl (Token hatte zunächst nur
`Administration: Read`); nach Korrektur durch den Nutzer zweiter Versuch mit `422` (GitHub-API
verlangt getypte Booleans statt `-f`-Strings sowie explizite `null`-Werte für
`required_pull_request_reviews`/`restrictions`) — behoben durch rohen JSON-Body via
`gh api --input -`. Nach nochmaliger expliziter Nutzerbestätigung unmittelbar vor der Aktivierung
scharf geschaltet: `main` verlangt jetzt die 4 Required Status Checks (`Unit Tests`,
`Safety Test Suite`, `Integration Tests (Docker)`, `Vitest Unit Tests`), `strict=true`,
`enforce_admins=true` (gilt auch für Admin-Accounts, kein Bypass mehr). Verifiziert per
`gh api repos/FloriStein/AVOC/branches/main/protection`. `.mcp.json` zusätzlich um
`GITHUB_TOOLSETS` (inkl. `actions`) ergänzt, damit künftige Sessions Workflow-Runs/Logs direkt
über den github-MCP-Server einsehen können.

---

## EPIC: Tech Debt

| ID | Task | Typ | Status | Notizen |
|----|------|-----|--------|---------|
| TECHDEBT-01 | `internal/authservice/noop_userstore.go`s `NoopUserStore` löschen — komplett ungenutzt (kein Aufrufer außerhalb der eigenen Datei, auch nicht in Tests) | S | ✅ Sprint 36 | Nebenbefund aus GOSTYLE-IF-04 (Sprint 35). War bereits vor diesem Sprint tot, nicht durch die Interface-Verschlankung verursacht — daher nicht im Rahmen von GOSTYLE-IF-04 mit-entfernt (Scope-Grenze), eigener kleiner Folge-Task. Datei komplett gelöscht. Details in `tasks/sprints/36-restposten-bereinigung-iii.md` |

---

## EPIC: Drift-Audit-Fixes (Sprint 26, `docs/drift-audit-2026-07.md`)

Vollständige Befunde/Belege: [`docs/drift-audit-2026-07.md`](../docs/drift-audit-2026-07.md).
Kritische Befunde (`DRIFT-K*`) dürfen laut `CLAUDE.MD` §0 ("Sicherheit schlägt alles") **nicht**
direkt gefixt werden — je Themenblock erst eine Grill-Me-Session, ggf. neues ADR. Mittel/Niedrig
Typ S sind Fast-Track (reine Doku-Korrektur). Mittel Typ M/L durchlaufen die volle Phasenfolge
(§1), einzeln, nicht vermischt.

| ID | Task | Typ | Schweregrad | Status | Notizen |
|----|------|-----|-------------|--------|---------|
| DRIFT-K1 | ADR-009 „Auth Invalidation" (JWT-Widerruf→SAFE_MODE) ohne Produktivpfad — Grill-Me vor Fix | L | Kritisch | ✅ behoben (2026-07-17) | Grill-Me: implementieren statt Doku korrigieren. Neuer `AuthWatchdog` (`internal/controlserver/safety/auth_watchdog.go`), liest `users.is_active` direkt (`internal/controlserver/authcheck`), 5s×2-Poll wie `SafetyBusWatchdog`, feuert über `TransitionOperator(OpNoOperator)`. ADR-009 Update-Block |
| DRIFT-K2 | ADR-009 „No Active Operator" ohne Produktivpfad — Grill-Me vor Fix | L | Kritisch | ✅ behoben (2026-07-17) | Grill-Me: implementieren statt Doku korrigieren (Redundanz allein wäre für zukünftige Pfade wie DRIFT-K1 nicht selbsttragend). WS-Disconnect-Handler ruft jetzt `TransitionOperator(OpNoOperator)`; Nebenbefund dabei behoben — `TransitionOperator`s SAFE_MODE-Zweig umging bislang den `validSystemTransitions`-Guard (`transitionSystemLocked`-Refactor). ADR-009 Update-Block |
| DRIFT-K3 | DEGRADED-Tier (Media/Video/Telemetrie) komplett unverdrahtet — Grill-Me vor Fix | L | Kritisch | ✅ teilweise behoben (2026-07-17) | Grill-Me-Prämisse korrigiert: MEDIA_FAILED war bereits produktiv verdrahtet (`useWebRTC.ts`→`/media/event`), Audit hatte nur `internal/webrtcsfu`/`internal/mediamtx` geprüft. Neu: MEDIA_DEGRADED-Schwellwerte (`getStats()`, initiale unvalidierte Werte) + DEGRADED→CONNECTED-Recovery-Fix (vorher fehlte der Rückweg komplett). Partial Telemetry Loss bewusst zurückgestellt — siehe `DRIFT-K3-TELEMETRY` unten. ADR-009 Update-Block |
| DRIFT-K3-TELEMETRY | Partial Telemetry Loss (MQTT) → DEGRADED ohne Produktivpfad | M/L | Kritisch | ✅ behoben (Sprint 50, 2026-07-21) | Neu aus DRIFT-K3-Umsetzung (2026-07-17): `telemetry-service` ist eigener Prozess ohne bestehende Kopplung zu `control-server`. Zwei Grill-Me-Sessions (Typ L, sicherheitsnah): Sprint 47 (2026-07-20) baute das State-Machine-Fundament (`DegradedReason`-Set + `enterDegraded`/`exitDegraded`), Sprint 50 (2026-07-21) den eigentlichen `TelemetryWatchdog` (neues Paket `internal/controlserver/telemetrycheck`, 2s×2-Poll gegen `telemetry-service`, neue `TELEMETRY_SERVICE_URL`-Env-Var, `TransitionTelemetry(healthy bool)`). Dabei zusätzlich ein latenter Multi-Cause-Bug in `TransitionMedia`s Eintritts-Guard gefunden und behoben (griff nicht mehr, sobald SYSTEM bereits wegen eines anderen Grundes DEGRADED war). Details `tasks/sprints/50-telemetry-watchdog.md` |
| DRIFT-K4 | „CI Build-Fail" bei >100ms existiert nicht als Pipeline — Grill-Me vor Fix | L | Kritisch | 🔲 Grill-Me ausstehend | Kein CI-System im Repo überhaupt |
| DRIFT-K5 | WS-ACK-Roundtrip-Benchmark läuft im Standard-Testlauf nicht mit — Grill-Me vor Fix | M/L | Kritisch | 🔲 Grill-Me ausstehend | `b.Skip(...)` liefert zusätzlich stillen False-Positive |
| DRIFT-K6 | `TestLatencyBudget_DocumentedRequirement` ist tautologisch — Grill-Me vor Fix | S/M | Kritisch | 🔲 Grill-Me ausstehend | Vermutlich zusammen mit K4/K5 zu klären |
| DRIFT-M01 | `vision.md` §11 veraltete Frage zurückverlinken auf beantworteten `requirements.md`-Stand | S | Mittel | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | — |
| DRIFT-M02 | `ADR-018` datierten Update-Block ergänzen (Verweis auf ADR-023-Postgres-Migration) | S | Mittel | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | Nicht überschreiben (§6) |
| DRIFT-M03 | `ADR-011` Verweis auf `ADR-026` (Per-Vehicle-Isolation) ergänzen | S | Mittel | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | Auch in `requirements.md` (AUDIT-05) |
| DRIFT-M04 | `ADR-022` Verweis auf `ADR-023` (Postgres-Migration) ergänzen | S | Mittel | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | — |
| DRIFT-M05 | `ADR-021` um Hinweis auf `vehicle-mock`s Fleet-Simulator-Doppelrolle ergänzen | S | Mittel | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | — |
| DRIFT-M06 | `ADR-025`/`ADR-026` Cross-Referenz ergänzen (Koexistenz, kein Rename) | S | Mittel | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | — |
| DRIFT-M07 | `ADR-026` Text „GET /state entfällt" korrigieren (Compat-Shim bleibt bestehen) | S | Mittel | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | Bereits korrekt in architecture.md/DECISIONS.MD |
| DRIFT-M08 | `DECISIONS.MD` Status-Spalte ADR-014 auf „Superseded by ADR-020" korrigieren | S | Mittel | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | — |
| DRIFT-M09 | `ADR-012b`-Text zu `.gitignore`-Ausschluss korrigieren (Code ist getrackt) | S | Mittel | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | — |
| DRIFT-M10 | `docs/code-patterns.md` §1–3 auf ADR-026-Registry-Pattern aktualisieren | S | Mittel | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | Singleton-Beispiel ist überholt |
| DRIFT-M11 | `docs/code-patterns.md` §3 um Audit-Eintrag/OBSERVER-Block-Zweige ergänzen | S | Mittel | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | — |
| DRIFT-M12 | Kommentar in `useDeadmanSwitch.ts:34` von 2s auf 10s korrigieren | S | Mittel | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | Code-Verhalten selbst korrekt |
| DRIFT-M13 | `docs/architecture.md` Frontend-Komponententabelle um `FleetTaskPanel`/Audio-Module ergänzen | S | Mittel | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | — |
| DRIFT-M14 | `docs/architecture.md` Container-Tabelle um `postgres`-Zeile ergänzen | S | Mittel | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | — |
| DRIFT-M15 | `CONTEXT.MD` Failure-Classification: `ACKTimeoutWatcher`/`VehicleACKWatchdog` getrennt ausweisen | S | Mittel | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | Code deckt beide bereits korrekt ab |
| DRIFT-M16 | `docs/requirements.md` State-Machine-Requirements um Per-Vehicle-Hinweis (ADR-026) ergänzen | S | Mittel | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | Deckt sich mit M03 |
| DRIFT-M17 | `README.md`-Projektstruktur-Baum um `fleet-service`/`fleetgateway` ergänzen | S | Mittel | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | Service-Tabelle im selben Dokument bereits korrekt |
| DRIFT-M18 | OBSERVATION-Trigger „Auth-Service-down blockiert neue Sessions" — implementieren oder Doku korrigieren | M | Mittel | 🔲 Backlog | Entscheidung nötig, volle Phasenfolge |
| DRIFT-M19 | k6-Latenztest auf echten WS-ACK-Roundtrip umstellen (misst aktuell `GET /state`) | M | Mittel | 🔲 Backlog | — |
| DRIFT-M20 | Video-Latenzziel (100–300ms) — automatisierten Test ergänzen oder Doku als unverifiziert kennzeichnen | S/M | Mittel | 🔲 Backlog | ADR-014: kein Safety-Hartziel |
| DRIFT-M21 | `Doku.md` (EC2-Prod-Deployment) um `fleet-service` ergänzen (Compose + Build/Save-Listen) | M/L | Mittel | 🔲 Backlog | Aktuell kein lauffähiger Fleet-Stack auf EC2 dokumentiert |
| DRIFT-M22 | `-race` strukturell in Makefile-Standardziel(en) verankern statt nur ad-hoc | M | Mittel | 🔲 Backlog | Kein CI vorhanden — ggf. mit DRIFT-K4/K5 zusammen klären |
| DRIFT-M23 | Backlog-Task für dokumentierte `CreateAlert`-Fehlerpfad-Testlücke (FLEET-07-Nachtrag) nachtragen | S | Mittel | ✅ hiermit erledigt | Diese Zeile selbst ist der Backlog-Eintrag |
| DRIFT-M24 | `OperatorRole` als Typ statt Rohstring durchgängig verwenden (`session/manager.go` u. a.) | M | Mittel | 🔲 Backlog | Sicherheitsnah (ADR-025) — volle §17-Testabdeckung bei Umsetzung |
| DRIFT-M25 | Task-Status in `internal/fleetservice/store.go` als benannten Typ statt Rohstrings führen | M | Mittel | 🔲 Backlog | Vorbild: `statemachine/state.go` |
| DRIFT-N01 | `docs/adr/README.md` Kopfzeile „29 ADRs" auf 32 korrigieren | S | Niedrig | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | — |
| DRIFT-N02 | `ADR-002` Pseudocode-Methodennamen an echten Code (`Subscribe`, 3-Parameter-Signatur) angleichen | S | Niedrig | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | — |
| DRIFT-N03 | `ADR-009` Dateipfade (`detector.go` statt `deadman.go`) und Testanzahl (20 statt 18) korrigieren | S | Niedrig | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | — |
| DRIFT-N04 | `ADR-017` um `AuditWriter.QueryBySession`-Methode ergänzen | S | Niedrig | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | — |
| DRIFT-N05 | `useControls.ts` Magic-Number-Fallbacks (`?? 1/2/3`) als benannte Konstanten führen | S | Niedrig | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | Nur Edge-Case-Pfad |
| DRIFT-N06 | `README.md` Testanzahl „41 Tests" auf tatsächliche Zahl (252) korrigieren | S | Niedrig | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | — |
| DRIFT-N07 | `docs/requirements.md` Kopfzeilen-Datumsstempel nachziehen | S | Niedrig | ✅ behoben (Sprint 26, docs/drift-audit-2026-07.md) | — |
| DRIFT-AP3 | Neues `## EPIC: AP3` (Admin-Konsole) in diesem Backlog anlegen — Meilenstein ist 0 % begonnen, aber nicht als solcher sichtbar | S | Mittel (strukturell) | 🔲 Backlog | AUDIT-01-Empfehlung, kein Einzel-Drift-Fix |

---

## EPIC: Testabdeckungs-Gesamtaudit 2026-07-21 (alle Systeme, alle Testarten)

**Auslöser (2026-07-21):** Nutzerwunsch, die gesamte Anwendung (7 Go-Services, Frontend,
CI-Infrastruktur) auf sinnvoll ergänzbare Unit-/Integration-/Smoke-/E2E-Tests zu prüfen — zunächst
nur Ist-Analyse + Plan, keine Umsetzung. Durchgeführt über 3 parallele Recherche-Agenten
(Go-Backend-Coverage inkl. `-coverpkg`-Korrektur, Frontend-Coverage inkl. Vitest, CI-Wiring-Audit
aller 5 Workflows). Reine Bestandsaufnahme, deckt sich bewusst **nicht** mit bereits bekannten,
noch offenen Punkten — die werden hier nur referenziert, nicht dupliziert: `DRIFT-K3-TELEMETRY`
(läuft bereits als Sprint 47), `DRIFT-K4/K5/K6` (CI-/Latenz-Testing-Debt, Grill-Me weiterhin
ausstehend, Nutzer hat sie bei der Sprint-47-Freigabe bewusst zurückgestellt), `DRIFT-M19/M22`,
sowie der seit Sprint 41 bekannte `BenchmarkControlACKRoundtrip`-Skip-Bug (fehlender `session_id`-
Parameter, ADR-025-Drift).

**Zwei operative Funde, KEINE Test-Tasks, vor Umsetzung zu klären (siehe Fragen unten):**
1. **Branch-Divergenz:** `feature/fleet-service-foundation` (Basis für ~15 aktive Worktrees) hängt
   `origin/main` um 5 Commits hinterher, darunter der reale E2E-Login-Fix (`e57586c`,
   `frontend/tests/e2e/dashboard.spec.ts` mit `login()`-Helper). Hier im Worktree existiert
   stattdessen noch eine veraltete Alt-Datei `tests/e2e/dashboard.spec.ts` (Root-Level, ohne Login,
   referenziert von `.github/workflows/test-e2e.yml` mit dem alten "Known gap"-Kommentarblock).
   Die Frontend-E2E-Tasks unten (`E2ETEST-*`) setzen auf dem **origin/main-Stand**
   (`frontend/tests/e2e/`) auf, nicht auf der lokalen Alt-Datei.
2. **Unmerged Sprint-49-Branch:** `feature/fleet-service-foundation-localvm` (Commit `fd3c589`,
   Ansible-Lokal-VM-Verifikation) ist nicht Teil dieses Branches — kein Testabdeckungs-Bezug hier,
   nur der Vollständigkeit halber vermerkt, da der CI-Agent danach gesucht und "nicht gefunden"
   zurückgemeldet hatte.

**Priorisierung (Risiko vor Aufwand, CLAUDE.MD §0 "Sicherheit schlägt alles"):** Safety-kritische
Backend-Lücken (Teil 1) vor Integrationslücken zwischen Services (Teil 2) vor Frontend-Session-/
Safety-Hooks (Teil 3) vor E2E-Flow-Ausbau (Teil 4) vor CI-Härtung (Teil 5). Jeder Teil ein eigener,
in sich abgeschlossener Sprint (S/M-Tasks, ~200k-Token-Budget), Reihenfolge ist Empfehlung, kein
Zwang — Nutzer entscheidet beim jeweiligen Sprint-Kickoff wie gewohnt.

**Sprint-Zuweisung (2026-07-21):** Teil 1 als **Sprint 51** eingeplant (Nutzerentscheidung, höchste
Priorität laut CLAUDE.MD §0). Nächste freie Nummer nach Sprint 49 (Sprint 48/49 bereits durch die
parallel geplante lokale Ansible-VM belegt) — Sprint 47 selbst referenziert intern noch "Sprint 48"
als Telemetry-Watchdog-Folge-Sprint, das ist ein bekannter, hier bewusst nicht angefasster
Nummern-Konflikt (Sprint 47 läuft bereits, siehe `tasks/current-sprint.md`). Sprint 51 wird aktiv,
sobald `tasks/current-sprint.md` nach Sprint 47 wieder frei ist. Teil 2 als **Sprint 52**
umgesetzt (2026-07-21). Teil 3 als **Sprint 53** umgesetzt (2026-07-21). Teil 4 als **Sprint 54**
umgesetzt (2026-07-21), siehe `tasks/sprints/54-e2e-ausbau.md`. Teil 5 als **Sprint 55** umgesetzt
(2026-07-21), siehe `tasks/sprints/55-ci-haertung.md`. **EPIC vollständig abgeschlossen** — alle 5
Teile (Sprint 51-55) umgesetzt.

### Teil 1 — Safety-kritische Backend-Testlücken (Sprint 51, ✅ abgeschlossen)

**Vorrecherche (Go-Agent, 2026-07-21):** `go test ./... -cover` unterschätzt `control-server`s
Safety-Layer strukturell (Tests liegen in `tests/unit`, externes Package) — korrekt gemessen via
`-coverpkg` liegt `internal/controlserver/{statemachine,session,safety,command,vehiclecontext}`
bereits bei 84,1 %. Echte Nullstellen: `internal/mediamtx` (WHIP/WHEP-Auth-Client für den
SAFE_MODE-Video-Kick, 87 Zeilen, **0 Tests überhaupt**) und `internal/vehicleregistry` (Vehicle-
CRUD/Persistenz, 137 Zeilen, **0 Tests überhaupt**). Zusätzlich mehrere gezielte Funktionslücken
mit Sicherheitsbezug: `session/sfu_publisher.go` (`PublishSessionEvent`, `NewHTTPSFUPublisher`)
0 %, `safety/http_publisher.go:TriggerEmergencyStop` 0 % — der HTTP-Fehlerpfad zu safety-service
wird geloggt und verschluckt, dieses Silent-Failure-Verhalten ist nie verifiziert;
`command/engine.go`s Audit-Write-Fehlerpfad innerhalb der E-Stop-Behandlung
(`svcLog.Error("audit write failed — proceeding to SAFE_MODE")`) ebenfalls ungetestet;
`session/handover.go:issueHandoverToken` nur 20 % (auth-service-HTTP-Fehlerpfade offen).

| ID | Task | Typ | Abhängigkeiten |
|----|------|-----|-----------------|
| GOTEST-01 | `internal/mediamtx/client_test.go` — WHIP/WHEP-Auth-Client vollständig (Erfolg, HTTP-Fehler, Timeout, malformed Response). | S/M | — |
| GOTEST-02 | `internal/vehicleregistry/registry_test.go` — Vehicle-CRUD (`SQLiteVehicleStore`/`NoopVehicleStore`, `ErrNotFound`-Sentinel, Doppel-Registrierung, Löschen bei aktiver Session). | M | — |
| GOTEST-03 | `internal/controlserver/safety/http_publisher_test.go` — `TriggerEmergencyStop` gegen simulierten HTTP-Fehler (Netzwerkfehler, non-2xx) — verifiziert, dass der Swallow-Pfad wirklich geloggt und der Aufrufer nicht blockiert wird. | S | — |
| GOTEST-04 | `internal/controlserver/command/engine_test.go` — Audit-Write-Fehlerpfad innerhalb `handleEmergencyStop`/`forwardMovementCommand` (Audit-Writer liefert Fehler, E-Stop muss trotzdem nach SAFE_MODE laufen). | S/M | — |
| GOTEST-05 | `internal/controlserver/session/sfu_publisher_test.go` + `manager_test.go` — `PublishSessionEvent`/`NewHTTPSFUPublisher` (inkl. Fehlerpfad) und `ListSessions`. | S/M | — |
| GOTEST-06 | `internal/controlserver/session/handover_test.go` — `issueHandoverToken`-Fehlerpfade (auth-service antwortet mit non-2xx/Timeout). | S | — |
| GOTEST-07 | Verifikation: `go build ./...`, `go vet ./...`, `go test ./... -race` (2× gegen Flakiness), Coverage-Diff dokumentieren, `DECISIONS.MD`/`tasks/backlog.md`-Update. | S | GOTEST-01..06 |

**Nicht Teil dieses Teils:** `internal/webrtcsfu`s `forwardTrack`/echte SDP-Negotiation (bewusster
Nicht-Scope seit Sprint 39, ADR-006 "zu flaky in CI"), Load-/Stress-Test der Watchdogs unter
gleichzeitiger Multi-Vehicle-Last (siehe Teil 5).

**Geschätzter Umfang:** 7 Tasks (GOTEST-01..07), überwiegend S/M — innerhalb des
~200k-Token-Sprintbudgets.

**Ergebnis (2026-07-21):** Alle 7 Tasks umgesetzt, reine Testabdeckung ohne Produktivcode-
Verhaltensänderung. Coverage-Diff (nur betroffene Funktionen, `-coverprofile` je Paket): `internal/mediamtx`
0→97,0 %, `internal/vehicleregistry` 0→97,4 %, `safety/http_publisher.go:TriggerEmergencyStop`+`PublishEvent`
0→100 %, `command/engine.go:handleEmergencyStop`+`forwardMovementCommand` 0→100 %,
`session/sfu_publisher.go` 0→100 %, `session/manager.go:ListSessions` 0→100 %,
`session/handover.go:issueHandoverToken` 20→100 %. `go build`/`go vet`/`gofmt -l` sauber,
`make test-unit -race` 2× hintereinander grün (ein einmaliger, unabhängiger Flake in
`internal/fleetgateway` beobachtet — Paket in diesem Sprint nicht angefasst, in 3 von 4 Läufen grün,
vermutlich Port-/Ressourcenkonflikt unter Parallellast, nicht behoben da außerhalb des Scopes).
Details `tasks/sprints/51-testabdeckung-safety-backend.md`.

### Teil 2 — Fehlende Integrationstests zwischen Services (Sprint 52, ✅ abgeschlossen)

**Vorrecherche:** `tests/docker-compose.test.yml` baut aktuell `control-server`, `auth-service`,
`safety-service`, `fleet-service`, `mosquitto`, `vehicle-mock`, `postgres` — **`telemetry-service`
und `webrtc-sfu` fehlen komplett**, obwohl beide reguläre der 7 Kern-Services sind.
Konkret fehlt: control-server↔webrtc-sfu (SAFE_MODE-Media-Kick via mediamtx, Session-Event-Push an
die SFU — genau der Pfad, den `session/sfu_publisher.go`/`internal/mediamtx` bedienen, siehe Teil
1) und jegliche Ende-zu-Ende-Verifikation der telemetry-service-MQTT-Ingestion gegen echte
Downstream-Konsumenten.

**Korrektur bei Sprint-52-Umsetzung:** `telemetry-service` war entgegen der Vorrecherche bereits
seit Sprint 50 (TelemetryWatchdog) Teil des Teststacks — nur der direkte
`GET /telemetry/latest`-Zugriff über die Prozessgrenze fehlte tatsächlich noch (INTTEST-02 darauf
zugeschnitten, siehe `tasks/sprints/52-integrationstests-services.md`).

| ID | Task | Typ | Abhängigkeiten |
|----|------|-----|-----------------|
| INTTEST-01 | `webrtc-sfu` + `internal/mediamtx`-Ziel in `tests/docker-compose.test.yml` ergänzen; neuer Integrationstest control-server→webrtc-sfu (SAFE_MODE löst echten Media-Kick/Session-Event aus, verifiziert über SFU-HTTP-Status). | M/L | ✅ Sprint 52 |
| INTTEST-02 | `telemetry-service` in `tests/docker-compose.test.yml` ergänzen; Integrationstest MQTT-Publish (vehicle-mock) → telemetry-service-Ingestion → `GET /telemetry/latest`-Abfrage über Prozessgrenze. | M | ✅ Sprint 52 |
| INTTEST-03 | Verifikation: `make test-integration` gegen erweiterten Stack, Laufzeit-Impact prüfen (CI-Timeout `test-go.yml` ggf. anpassen), Doku-Updates. | S | ✅ Sprint 52 |

**Nicht Teil dieses Teils:** echte WebRTC-SDP/ICE-Negotiation im Integrationstest (bleibt Mock-/
Status-Ebene, ADR-006-Policy).

**Ergebnis (2026-07-21):** `webrtc-sfu` + echtes `bluenviron/mediamtx:latest` neu im Teststack;
neuer `GET /session/{id}/state`-Status-Endpoint auf `webrtc-sfu`. Zwei neue Integrationstests
(`webrtc_sfu_test.go`, `telemetry_service_test.go`). Nebenbefund: `make test-integration` lief
ohne `-count=1`, wodurch wiederholte Läufe ohne Codeänderung gecachte statt echter Testergebnisse
lieferten — behoben. Details `tasks/sprints/52-integrationstests-services.md`.

### Teil 3 — Frontend: Session-/Safety-kritische Hooks (Sprint 53, ✅ abgeschlossen)

**Vorrecherche (Frontend-Agent, 2026-07-21):** Gesamt-Coverage 52,2 % Stmts, aber die
sicherheitsrelevantesten Hooks liegen weit darunter: `useSession.ts` (Login/Logout/`startSession`
— genau der ADR-028-Teleoperate/Beobachten-Mechanismus) nur **2,6 %**, bislang ausschließlich
indirekt über gemockte Props in `FleetVehicleDetail.test.tsx`/`FleetOverview.test.tsx` berührt,
nie als eigene State-Machine getestet. `SafetyPanel.test.tsx` prüft beim Emergency-Stop-Button nur
Sichtbarkeit/Disabled-State — der vorhandene Mock wird nie auf tatsächlichen Aufruf assertiert,
kein Test simuliert den Klick. `useControls.ts` 36 % (Keyboard-/E-Stop-Wiring bei Zeile 66-191
größtenteils ungetestet). `ws-client.ts` 10,5 % / `fleet-ws-client.ts` 2,2 % (Connect/Reconnect/
Close-Logik ohne Testdatei). `useWebRTC.ts` 29 % (Connection-State-Übergänge Zeile 206-265
ungetestet).

| ID | Task | Typ | Abhängigkeiten |
|----|------|-----|-----------------|
| FETEST-01 | `frontend/src/hooks/useSession.test.ts` — `login`/`logout`/`startSession`/`endSession` als eigene State-Machine (nicht nur über gemockte Props), inkl. ADR-028-Fall (Vehicle bereits belegt → "Beobachten"). | M | ✅ Sprint 53 |
| FETEST-02 | `SafetyPanel.test.tsx` erweitern — echter Klick auf Emergency-Stop, Assertion dass `emergencyStop()` aufgerufen wurde, Disabled-State danach. | S | ✅ Sprint 53 |
| FETEST-03 | `useControls.test.ts` — Keyboard-Wiring inkl. Emergency-Stop-Taste (Zeile 66-191). | S/M | ✅ Sprint 53 |
| FETEST-04 | `frontend/src/lib/ws-client.test.ts` + `fleet-ws-client.test.ts` — Connect/Reconnect/Close, insbesondere die von Sprint-14 bekannte Race-Condition-Fixstelle (`onclose = null` vor `close()`) als Regressionsschutz. | M | ✅ Sprint 53 |
| FETEST-05 | `useWebRTC.test.ts` erweitern — Connection-State-Übergänge (Zeile 206-265). | S/M | ✅ Sprint 53 |
| FETEST-06 | Verifikation: `npx vitest run --coverage`, Coverage-Diff dokumentieren, `tasks/backlog.md`-Update. | S | ✅ Sprint 53 |

**Nicht Teil dieses Teils:** `useWHIPSender.ts`/`useTelemetry.ts`/`useVehicleAck.ts` (niedrige
Coverage, aber nicht sicherheitskritisch — reiner Datenfluss, kein Steuerpfad), `api-client.ts`-
Fehlerpfade (Teil 5), ErrorBoundary (existiert aktuell nicht im Codebase — eigener
Architektur-Entscheidungspunkt, kein Test-Task).

**Ergebnis (2026-07-21):** Alle 6 Tasks umgesetzt, 60 neue Tests in 4 neuen + 2 erweiterten
Testdateien (88 Tests gesamt in den betroffenen Dateien). Coverage-Sprünge: `useSession.ts`
2,6→89,56 %, `useControls.ts` 36→89,58 %, `ws-client.ts` 10,5→100 %, `fleet-ws-client.ts`
2,2→95,65 %, `useWebRTC.ts` 29→82,63 %, Frontend gesamt 52,2→73,85 % Stmts. `SafetyPanel.test.tsx`
testet jetzt einen echten Klick mit Assertion auf `emergencyStop()` statt nur Sichtbarkeit/
Disabled-State. Beide WS-Clients haben einen dedizierten Regressionstest für die Sprint-14-Fixstelle
(`onclose = null` **vor** `close()`). `npm run lint`/`npx tsc -b` sauber, `test:coverage` 2×
hintereinander identisch grün (370/370 Tests). Bewusst nicht vertieft: `useSession.ts`s
Backoff-Reconnect-Zusammenspiel mit dem echten `WSClient` (hier gemockt), Gamepad-Neutral-Reset in
`useControls.ts`, die Bitrate-basierte MEDIA_DEGRADED-Erkennung in `useWebRTC.ts` (bräuchte
`performance.now()`-Mocking, nicht Teil der bestehenden Testinfrastruktur). Details
`tasks/sprints/53-frontend-hooks.md`.

### Teil 4 — E2E-Flow-Ausbau (Sprint 54, ✅ abgeschlossen)

**Vorrecherche:** Aktueller Stand (`origin/main`, `frontend/tests/e2e/dashboard.spec.ts`) deckt nach
dem Login-Fix nur die 5 ursprünglichen Baseline-Assertions ab (Header, IDLE, SafetyPanel/
ConnectionPanel sichtbar, E-Stop-Button im DOM) — alle hinter echtem Login erreicht, aber
inhaltlich unverändert oberflächlich (bewusst so seit Sprint 41, CIGATE-05).

**Vorrecherche-Gegenprüfung (2026-07-21, bei Sprint-54-Planungsübernahme):** Die "Branch-Divergenz"
aus dem EPIC-Kopf ist gegenstandslos — analog Sprint 51-53 wird direkt gegen `main` gearbeitet,
`frontend/tests/e2e/dashboard.spec.ts` (inkl. vollständigem `login()`-Helper) existiert bereits,
die alte Root-Level-Datei `tests/e2e/` enthält keine Spec mehr. Details/Task-Verfeinerung
(konkrete Locators, Abmelden-Button-Pfad) in `tasks/current-sprint.md`.

| ID | Task | Typ | Abhängigkeiten |
|----|------|-----|-----------------|
| E2ETEST-01 | Login-Fehlerfall (falsches Passwort → Fehlermeldung sichtbar, kein Übergang zu FleetOverview). | S | ✅ Sprint 54 |
| E2ETEST-02 | Session-Konflikt real gegen Backend: zweiter Browser-Context/Operator sieht "Beobachten" statt "Teleoperate" (ADR-028), nicht nur UI-Mock. | M | ✅ Sprint 54 |
| E2ETEST-03 | Emergency-Stop echter Klick-Flow (State-Transition sichtbar, Button danach disabled). | S | ✅ Sprint 54 |
| E2ETEST-04 | Logout-Flow (zurück zu LoginPanel, Session serverseitig beendet). | S | ✅ Sprint 54 |
| E2ETEST-05 | Verifikation: `npm run test:e2e` 2× lokal gegen echten Docker-Stack (non-blocking Job, aber Flakiness-Ausschluss laut CLAUDE.MD §17 trotzdem sinnvoll), Doku-Update. | S | ✅ Sprint 54 |

**Nicht Teil dieses Teils:** WebRTC-Verbindungsabbruch-UI, DEGRADED/SAFE_MODE-UI-Übergang über echte
Verbindungsunterbrechung (technisch aufwändig in Playwright ohne echtes Video, eigener
Entscheidungspunkt falls gewünscht), Permission-Denied/OBSERVER-Rolle (bei Sprint-54-Umsetzung
korrigiert: OBSERVER-Rollen-Gating existiert im Frontend real und wäre E2E-testbar, siehe
`tasks/sprints/54-e2e-ausbau.md` — bewusst zurückgestellt, da ein zweiter Seed-User nötig wäre,
Kandidat für einen künftigen Sprint statt "gibt es vermutlich nicht" wie ursprünglich vermutet).

### Teil 5 — CI-Härtung (Sprint 55, ✅ abgeschlossen)

**Vorrecherche (CI-Agent, 2026-07-21):** Keiner der 5 Workflows scannt auf Sicherheitslücken
(kein `gosec`/`npm audit`/Trivy/CodeQL). Kein Contract-Test für `proto/` (5 `.proto`-Dateien, keine
`buf breaking`-Prüfung). Kein Container-Build-Verifikationsjob in CI (`docker buildx build` nur
manuell). `BenchmarkControlACKRoundtrip` (`tests/performance`) skipt seit Sprint 41 bekannt immer
(fehlender `session_id`-Parameter) — bislang nur dokumentiert, nicht behoben.

**Vorrecherche-Gegenprüfung (2026-07-21, bei Sprint-55-Planungsübernahme):** Der Skip-Bug in
CIHARD-01 ist tiefer als vermerkt — `tests/performance/latency_test.go`s Benchmark ruft
`/session/start` mit `vehicle_id: "bench-vehicle"` auf, aber `tests/docker-compose.test.yml`s
einziger vehicle-mock läuft mit `VEHICLE_ID=vehicle-int-mock`. `handleSessionStart` prüft
`vehicleRegistry.Connected(req.VehicleID)` vor dem eigentlichen `StartSession`-Aufruf und liefert
für das nie verbundene `bench-vehicle` immer 409 — der fehlende `session_id`-Parameter ist also
nur die zweite Bug-Hälfte. Details/Task-Verfeinerung in `tasks/current-sprint.md`. 8 konkrete
Build-Targets für CIHARD-03 identifiziert: 6 Go-Services (`SERVICE_NAME`-Matrix über
`go-service.Dockerfile`) + `frontend.Dockerfile` + `vehicle-mock.Dockerfile`.

| ID | Task | Typ | Abhängigkeiten |
|----|------|-----|-----------------|
| CIHARD-01 | `BenchmarkControlACKRoundtrip`-Skip-Bug beheben (`vehicle_id` korrigieren + `session_id`-Query-Parameter ergänzen) — schließt DRIFT-K5/K6 endgültig ab. | S | ✅ Sprint 55 |
| CIHARD-02 | Neuer non-blocking CI-Job: `gosec` (Go) + `npm audit` (Frontend), analog `lint.yml`-Muster (`continue-on-error`, informational). | S/M | ✅ Sprint 55 |
| CIHARD-03 | Container-Build-Verifikation als eigener CI-Job (`docker buildx build` je Service, kein Push) — verhindert "baut lokal, baut nicht in CI/Prod"-Drift. | S/M | ✅ Sprint 55 |
| CIHARD-04 | *(Zur Diskussion, kein fester Task)* `buf breaking` für `proto/` — nur sinnvoll falls mehrere Konsumenten unabhängig deployt werden; bei aktuell monolithischem Deploy (`docker-compose`) ggf. verzichtbar. Entscheidungspunkt vor Umsetzung. | S/M | 🔲 weiterhin offener Diskussionspunkt |
| CIHARD-05 | Verifikation: `make test-latency` läuft grün ohne Skip, neue Workflows so weit wie möglich lokal gegengeprüft, Doku-Update. | S | ✅ Sprint 55 |

**Nicht Teil dieses Teils:** CIHARD-04 bleibt Diskussionspunkt, kein umzusetzender Task;
Branch-Protection-Aktivierung (bereits als eigener, bewusst zurückgestellter Punkt seit Sprint 41
bekannt, CIGATE-06 — betrifft alle PRs, eigene Nutzerbestätigung nötig, unabhängig von diesem
Testabdeckungs-Audit).

**Ergebnis (2026-07-21):** Alle 4 Tasks (CIHARD-01, -02, -03, -05) umgesetzt. CIHARD-01 hatte bei
der Umsetzung eine dritte, erst durch tatsächliches Ausführen des Benchmarks entdeckte Bug-Ursache
(Session-Leak durch fehlendes `/session/end` zwischen den Kalibrierungsläufen des Go-Bench-
Harness — führte zu `OBSERVER` statt `ACTIVE_OPERATOR` beim zweiten Lauf und unbegrenztem Hang in
`conn.ReadMessage()`). `make test-latency` läuft jetzt 2× hintereinander grün ohne Skip (~200k
Iterationen je Lauf, p99 0-1ms gegen 100ms-Budget). Neue Workflows `security-scan.yml` (gosec +
npm audit) und `docker-build.yml` (8 Build-Targets) lokal gegengeprüft (`actionlint` sauber,
Kernbefehle jedes Jobs manuell reproduziert). CIHARD-04 gegengeprüft und bewusst weiterhin
zurückgestellt (Deployment unverändert monolithisches `docker-compose`). Details
`tasks/sprints/55-ci-haertung.md`. **Damit ist das EPIC "Testabdeckungs-Gesamtaudit 2026-07-21"
vollständig abgeschlossen (Teil 1-5, Sprint 51-55).**

---

## Offene Entscheidungen (blockieren zukünftige Tasks)

| Entscheidung | Blockiert | Referenz |
|---|---|---|
| Session Recording Storage (DB / Files / Object Storage) | offen | ADR-005 Folge — MemoryRecorder als Platzhalter |
| DDS-Produktivimplementierung | Nicht in diesem Scope | ADR-002 Folge |
| ~~Backup-Strategie Audit Store (Postgres `postgres-data`-Volume → S3)~~ | ✅ Sprint 44 | ADR-018/023 Folge — `pg_dump`+S3 täglich per Cron, siehe EPIC oben (`AUDITBACKUP-01..04`) |
| Migration zu AWS ECR | offen | ADR-019 Folge — für Produktivbetrieb |
| ~~MQTT-Authentifizierung (Mosquitto Passwort-File)~~ | ✅ Sprint 38 | Port 1883 lief seit Sprint 9 ohne Auth — siehe EPIC "Security Findings" oben (`MQTTAUTH-01..06`). TLS/MQTTS bewusst weiterhin offen (siehe dort) |
| Multi-Vehicle / vehicleId-Routing in MediaMTX | ✅ ADR-022 | VehicleSelector + SQLite-Registry; `~^vehicle-.*`-Regex aktiv |
| E2E Smoke Test mit aktiver WHIP-Quelle | offen | WEBRTC-09 Rest — Browser WiFi + 5G ICE-Pair verifizieren |
| ~~OBS-01 Vehicle Heartbeat~~ | ✅ Sprint 31 | AckBadge zeigt "Zuletzt gesehen vor Xs" aus `useTelemetry.ts`s `ageSinceUpdateMs`, unabhängig vom ACK-Kommandofluss |
| Multi-Vehicle Safety-Isolation (globale `sm`/Watchdog-Singletons) | ✅ ADR-026 | `VehicleContextRegistry` — Implementierung Sprint 17 |

---

## Phasen-Übersicht

```
Phase 6 — Testing & Quality Gates ✅ (abgeschlossen 2026-06-04)
  TEST-03 ✅  TEST-04 ✅  TEST-05 ✅  DC-04 ✅

Phase 7 — Logging & Audit Trail ✅ (abgeschlossen 2026-06-04)
  LOG-01..11 ✅ — Safety Regression 19/19 ✅

Phase 8 — EC2 Deployment via Docker Hub ✅ (abgeschlossen 2026-06-05)
  DEPLOY-01..07 ✅

Phase 9 — Video Stream: Larix WHIP → MediaMTX → Browser ✅ (abgeschlossen 2026-06-05)
  STREAM-01..09 ✅

Phase 10 — Browser WebRTC ICE Migration ✅ (deployed 2026-06-10)
  WEBRTC-01..09 ✅ (E2E Smoke Test offen)

Phase 11 — Vehicle Connectivity & Feedback ✅ (abgeschlossen 2026-06-11)
  VEH-01..12 ✅ — Go Build + 26/26 Unit Tests + 41/41 Frontend Tests grün

Phase 12 — Vehicle Registry ✅ (abgeschlossen 2026-06-12)
  VEH-REG-01..08 ✅ — ADR-022; SQLite vehicles-Tabelle; VehicleSelector; VEHICLE_ID-Hardcoding entfernt

Phase 13 — Dev-Stack Stabilisierung & Log-Korrelation ✅ (abgeschlossen 2026-06-13)
  DEV-01..03 ✅ — nginx.dev.conf HTTP-only; vehicle-mock im Build; session_id in TelemetryEvent

Phase 14 — Security & Observability ✅ (Bonus OBS-01 abgeschlossen Sprint 31)
  AUTH-01 ✅  ROB-01 ✅  UI-01 ✅  OBS-01 ✅
```
