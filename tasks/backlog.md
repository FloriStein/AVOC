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
| OBS-01 | Vehicle "zuletzt gesehen" Heartbeat-Timestamp in AckBadge (Bonus) | S | 🔲 offen | — |

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
| WEBRTC-10 | SDP `a=setup:active`-Workaround (`useWebRTC.ts`, `useWHIPSender.ts`) inkompatibel mit aktuellem Chromium (`setRemoteDescription` wirft „Offerer must use actpass") | M/L | 🔲 Backlog | Entdeckt Sprint 19 (LOCAL-04) beim ersten Mal, dass WHIP tatsächlich bis zum SDP-Austausch kam. Ursprünglich dokumentierter Fix für Pion-v1.19.0-DTLS-Bug (`docs/webrtc.md`). Vor jeder Änderung prüfen: (1) hat `mediamtx:latest` den Pion-Bug noch (Version pinnen/prüfen), (2) betrifft `useWebRTC.ts` auch den Produktiv-Video-Empfang auf AWS — kein reines Lokal-Problem. Ggf. Grill-Me + eigenes ADR nötig, da Video-Hub-Architektur (ADR-014/020) berührt |
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
| FLEET-01 | Handshake-basierte Autonomie-Rückgabe (statt einfachem `endSession()`) | M | 🔲 Backlog | `ADR-028` — bewusst zurückgestellt; Risiko: Fahrzeug könnte Kontrolle zurückerhalten, bevor es sicher verarbeitet ist. `endSession()` reicht für den Anfang |
| FLEET-02 | Persistenzform für gefahrene Route (Historie) — Zeitreihen-DB vs. einfache Tabelle | M | 🔲 Backlog | `ADR-029` — noch nicht entschieden, betrifft Karten-Route-Darstellung (Historie-Linie) |
| FLEET-03 | Bestehende `POST /vehicles`/`DELETE /vehicles/{id}` in `control-server` vs. neue `fleet-service`-Admin-API — Ablösung oder Koexistenz | S | 🔲 Backlog | `ADR-029` — kein Bruch im aktuellen Schritt, aber langfristig vermutlich durch reichhaltigere Fleet-Admin-API abgelöst |
| FLEET-04 | `vehicle-mock`s Fleet-Simulation nutzt hartcodierte Demo-Stationskoordinaten (`cmd/vehicle-mock/fleet_simulator.go`) statt echter Zonen/Stationen | S | 🔲 Backlog | Sprint 21 (FLEET-04) — vehicle-mock hat keinen DB-Zugriff; sobald `fleet-service`s REST-API (FLEET-05) Zonen/Stationen liefert, könnte die Simulation echte Daten abfragen statt sie zu erfinden |
| TASKUI-01 | Doppelte Demo-Stationsanlage bereinigen (`demo-zone-taskui`/`demo-station-*-taskui` aus Sprint 24 vs. `zone-betriebshof-nord`/echte Stationen aus der parallelen Sprint-23-Session) | S | ✅ Sprint 30 | `demoStationSeed` (INSERT) durch `taskuiDemoSeedCleanup` (DELETE, FK-sicher via `NOT EXISTS`-Guard) ersetzt |
| TASKUI-02 | Task-Status-Übergangstabelle dupliziert (Go `internal/fleetservice/store.go` `taskTransitionSources` vs. TypeScript `frontend/src/components/FleetTaskPanel.tsx` `NEXT_TRANSITIONS`) | S | ✅ Sprint 30 | Server-berechnetes `Task.AllowedTransitions`-Feld ersetzt die hartcodierte Frontend-Kopie; `FleetTaskPanel.tsx` rendert Buttons jetzt aus `task.allowed_transitions` |
| TASKUI-03 | Vollständige Task-Status-Audit-Historie (mehrere Übergänge, nicht nur der letzte) | M | 🔲 Backlog | `ADR-030` — bewusst auf `tasks.status_changed_by` (nur letzter Übergang) begrenzt statt eigener `task_status_history`-Tabelle (Scope-Entscheidung, Grill-Me 2026-07-16). Bei Bedarf: neue Tabelle + Migration, kein Bruch des bestehenden ADRs. Sprint-30-Triage 2026-07-18: bewusst zurückgestellt (Sprint-Zuschnitt) |
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
| AP1-04 | Klären, ob Meilenstein 1 ein eigenständiges Architektur-Liefer-Dokument für den Auftraggeber erfordert; falls ja, bestehende Docs (`docs/vision.md`, `docs/requirements.md`, `CONTEXT.MD`, `docs/architecture.md`, `ADR-027/028/029`) zu einem auftraggebertauglichen Dokument konsolidieren | S/M | 🔲 Zu klären | Kein Beleg im Repo für die genaue Abnahme-Form — vor Bearbeitung mit Auftraggeber/Nutzer klären, keine Annahme treffen |

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
| Kartenansicht — Indoor | 🔲 offen, kein Task mit ID bisher | Fahrzeuge haben kein `position_x/y` (nur `position_zone_id`) — Backend-Erweiterung nötig, siehe CONTEXT.MD/DECISIONS.MD "Offene Fragen" |
| Routenübersicht (gefahrene Historie) | 🔲 offen | `FLEET-02` (oben) — Persistenzform (Zeitreihen-DB vs. Tabelle) unentschieden |
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
| AP2-02 | Indoor-Kartenrendering — Backend-Erweiterung für Fahrzeug-Punktposition innerhalb einer Indoor-Zone | L | 🔲 Backlog | Bisher nur als "Offene Frage" in CONTEXT.MD/DECISIONS.MD geführt, hier erstmals als konkreter Task erfasst. Braucht eigene Datenmodell-Entscheidung (`vehicle_status`-Erweiterung) — vermutlich eigenes ADR, da Datenmodell-Änderung (ADR-029 nicht überschreiben) |
| AP2-03 | Persistenzform für "gefahrene Route" entscheiden + implementieren | M | 🔲 Backlog | Verweist auf bestehenden `FLEET-02` — hier nur referenziert, nicht dupliziert |
| AP2-04 | Prioritätenmanagement in der Task-UI über reine Zahlenanzeige hinaus ausbauen (Sortierung nach Priorität, visuelle Hervorhebung hoher Priorität) | M | 🔲 Backlog | Sprint 24 (`FleetTaskPanel.tsx`) hat `priority` als reines Eingabe-/Anzeigefeld ohne Sortierung/Hervorhebung umgesetzt — "Prioritätenmanagement" aus der Leistungsbeschreibung impliziert mehr als das; jetzt bearbeitbar (AP2-01 erledigt) |
| AP2-05 | Klären, ob Meilenstein 2 ein eigenständiges Abnahme-Artefakt für den Auftraggeber braucht | S | 🔲 Zu klären | Analog `AP1-04` — kein Beleg im Repo für die genaue Abnahme-Form |

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
Go-Backend, Pilot bestätigt `fleet-service`, Start **nachgelagert nach AP2/AP3** — kein fixes Datum,
daher hier in `backlog.md` statt in `current-sprint.md` (analog zum früheren Vorgehen bei "EPIC:
Fleet Dashboard Planung"). Umfang: ausschließlich der fehlende `FleetStore`-Repository-Port
(`internal/fleetservice/handler.go:32,37` → Interface, analog zu `fleetgateway.FleetGateway`),
keine Use-Case-Schicht-Extraktion in diesem Schritt (siehe HEX-06).

**Arbeitsweise (bewusst kleinteilig geschnitten):** Jeder Task ist einzeln abschließbar und endet
mit einem Update dieses Eintrags (Status ✅ + Kurzergebnis) **bevor** die Session beendet/geclear't
wird — Definition-of-Done schließt die Doku-Aktualisierung ein, nicht nur den Code. Damit reicht
für eine neue Session pro Task ein Verweis auf die Task-ID hier, keine lange Übergabe nötig.

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| HEX-01 | `FleetStore`-Interface definieren (deckt alle Methoden von `*PostgresFleetStore` ab) + Compile-Time-Check `var _ FleetStore = (*PostgresFleetStore)(nil)`. Kein Verhaltens-, kein Signatur-Wechsel an `Handler` in diesem Schritt. | S | 🔲 Backlog | — |
| HEX-02 | `Handler.store` von `*PostgresFleetStore` auf `FleetStore`-Interface umstellen (`internal/fleetservice/handler.go:32,37`). Reine Typ-Änderung, HTTP-Verhalten unverändert — bestehende Postgres-gebundene Tests bleiben vorerst grün (noch kein Fake). | S | 🔲 Backlog | HEX-01 |
| HEX-03 | `FakeFleetStore` (In-Memory) implementieren, das `FleetStore` erfüllt — Test-Doppel für Handler-Tests ohne Postgres. | M | 🔲 Backlog | HEX-01 |
| HEX-04 | Bestehende Handler-Tests (~20 der 24, aktuell `DATABASE_URL`-gebunden) schrittweise auf `FakeFleetStore` migrieren. | M | 🔲 Backlog | HEX-02, HEX-03 |
| HEX-05 | Verifikation: `go test ./internal/fleetservice/...` läuft grün **ohne** gesetzte `DATABASE_URL`. ADR-031-Status-Update (Pilot abgeschlossen) + Status-Update in diesem Eintrag + `DECISIONS.MD`. | S | 🔲 Backlog | HEX-04 |
| HEX-06 | *(Optional, eigener Entscheid nach HEX-05)* Use-Case-Schicht aus `Handler` extrahieren (Anwendungsfälle als eigene Funktionen zwischen HTTP-Layer und `FleetStore`/`AlertEngine`) — nur falls nach dem Pilot als lohnend bewertet, kein Bestandteil des Piloten selbst. | M | 🔲 Backlog (optional) | HEX-05 |

**Entscheidungspunkt nach HEX-05 (nicht Teil dieses Sprint-Plans):** ob und in welcher
Reihenfolge auth-service (JWT-Port) bzw. telemetry-service (kleinster Fall) folgen — siehe
Risikobewertung in ADR-031. `control-server` bleibt bis auf Weiteres ausdrücklich ausgeklammert
(eigenes ADR nötig, siehe ADR-031 "Offene Punkte").

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

**Neuer Folge-Task (aus GOSTYLE-13 gefunden, nicht Teil des Sprint-29-Scopes):** die drei
WS-Integrationstests in `tests/integration/services_test.go`
(`TestIntegration_SessionLifecycle_StartAndEnd`, `_MediaFailed_TriggersDegrade_NeverSafeMode`,
`_EmergencyStop_TriggersSafeMode`) skippen im Docker-Test-Stack, weil ihre WS-Dial-URL nur
`?token=` statt `?token=&session_id=` mitgibt — ein vorbestehender Test-Setup-Gap (nicht
WebRTC/SFU-bedingt wie die anderen 3 bekannten Skips), der `WSHandler.readLoop` komplett ohne
automatisierte Abdeckung lässt. Typ S, unabhängig von GOSTYLE-* erledigbar.

**Bonus, außerhalb des Style-Guide-Scopes (nebenbei gefunden, unabhängig einreihbar):**
`gofmt -l` findet 13 unformatierte Dateien — trivialer `gofmt -w .`-Task (Typ S, keine
Logikänderung), unabhängig von GOSTYLE-* erledigbar.

### Phase 2 — Interface-Segregation (Rule 4.2/4.3), nachgelagert nach ADR-031

**Start erst nach `HEX-05`** (Hexagonal-Pilot fleet-service abgeschlossen) **und** nach
AP2/AP3-Meilensteinen, analog zum ADR-031-Timing. Koordiniert mit dem Hexagonal-Epic oben —
dieselben Interfaces, unterschiedlicher Fokus (dort: Repository-Port für `fleet-service`; hier:
Methodenzahl/Konsumenten-Zuschnitt projektweit).

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| GOSTYLE-IF-01 | `recording.SessionRecorder` (6 Methoden, aktuell nirgends als Interface-Typ konsumiert): klären, ob Interface tatsächlich verwendet werden soll (`*MemoryRecorder` → `SessionRecorder` in `cmd/control-server/main.go:102`) oder aufgelöst wird, solange nur eine Implementierung existiert (Rule 1.2 — Abstraktion ohne Konsument ist unbegründet) | S | 🔲 Backlog | HEX-05 |
| GOSTYLE-IF-02 | `fleetgateway.FleetGateway` (3 Methoden, ungenutzt als Typ): gleiche Frage wie IF-01. `fleetservice.Dispatcher` (1 Methode) ist bereits der korrekte konsumentenseitige Schnitt und bleibt unverändert | S | 🔲 Backlog | HEX-05 |
| GOSTYLE-IF-03 | `pkg/audit.AuditWriter` (3 Methoden) in schlankes `WriteSync`-only Interface für die 5 Safety-/Command-Consumer aufspalten; `Close`/`QueryBySession` bleiben am konkreten Typ bzw. eigenem kleineren Interface für `main.go` | M | 🔲 Backlog | HEX-05 |
| GOSTYLE-IF-04 | `authservice.UserStore` (7 Methoden): `SeedAdmin` (reine Bootstrap-Methode, bereits am konkreten Typ genutzt) aus dem Interface entfernen; verbleibende 6 Methoden gegen tatsächlichen `Handler`-Bedarf prüfen | M | 🔲 Backlog | HEX-05 |
| GOSTYLE-IF-05 | `vehicleregistry.VehicleStore` (5 Methoden): `SeedDefault` (Bootstrap) aus Interface lösen, analog IF-04 | S | 🔲 Backlog | HEX-05 |
| GOSTYLE-IF-06 | `controlserver/safety.Publisher` (2 Methoden) in `PublishEvent`-only Interface für `Engine`/Watchdogs aufteilen; `TriggerEmergencyStop` bleibt eigener Zugriffspfad, analog zum bereits vorbildlichen `vehicleconnection.safetyPublisher`-Muster | S | 🔲 Backlog | HEX-05 |

**Nebenbefund, nicht Teil dieses Style-Guide-Scopes (Sicherheitsauffälligkeit):** beim
Duplikat-Scan (Rule 3.1) fiel auf, dass der JWT-Alg-Confusion-Check in 2 von 4 JWT-Parse-Stellen
(`authservice`, `fleetservice`, `vehicleconnection`, `control-server`) fehlt. Kein Style-Guide-
Thema — separat über `/security-review` oder einen eigenen Sicherheits-Task nachverfolgen.

---

## Offene Entscheidungen (blockieren zukünftige Tasks)

| Entscheidung | Blockiert | Referenz |
|---|---|---|
| Session Recording Storage (DB / Files / Object Storage) | offen | ADR-005 Folge — MemoryRecorder als Platzhalter |
| DDS-Produktivimplementierung | Nicht in diesem Scope | ADR-002 Folge |
| Backup-Strategie Audit Store (SQLite Volume → S3) | offen | ADR-018 Folge — S3-Bucket im CDK vorhanden |
| Migration zu AWS ECR | offen | ADR-019 Folge — für Produktivbetrieb |
| MQTT-Authentifizierung (Mosquitto Passwort-File) | offen | Port 1883 aktuell ohne Auth offen |
| Multi-Vehicle / vehicleId-Routing in MediaMTX | ✅ ADR-022 | VehicleSelector + SQLite-Registry; `~^vehicle-.*`-Regex aktiv |
| E2E Smoke Test mit aktiver WHIP-Quelle | offen | WEBRTC-09 Rest — Browser WiFi + 5G ICE-Pair verifizieren |
| OBS-01 Vehicle Heartbeat | offen | Sprint-14-Bonus — AckBadge "zuletzt gesehen" Timestamp |
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

Phase 14 — Security & Observability 🔄 (Sprint 14, in Bearbeitung)
  AUTH-01 ✅  ROB-01 ✅  UI-01 ✅  OBS-01 🔲 (Bonus offen)
```
