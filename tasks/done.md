# Done

Lifecycle: backlog → sprint → done

Kompakter Index aller abgeschlossenen Sprints. Volltext (Tasks, Testprotokolle, Datei-Listen,
Scope-Details) je Sprint in [tasks/sprints/](sprints/). Sprint 26 existiert auf diesem Branch
nicht — Nummernkollision mit einem nie gemergten Parallel-Strang (`feature/docs-drift-audit`,
siehe `tasks/backlog.md`).

---

## Sprint 39 — Testabdeckung `safety-service`/`webrtc-sfu`/`internal/recording` ✅
2026-07-19 → [tasks/sprints/39-testabdeckung-sicherheitsrelevanter-services.md](sprints/39-testabdeckung-sicherheitsrelevanter-services.md)
- Schließt seit der ADR-031-Bestandsaufnahme (2026-07-16) offene Lücke: drei Services mit 0 automatisierten Tests, darunter der Safety Event Bus (`safety-service`). Nutzerentscheidung 2026-07-19 gegenüber 3 Alternativen (Hexagonal-Migration Schritt 3, TLS/MQTTS-Härtung, Session-Recording-Storage), Begründung CLAUDE.MD §0 Priorität 1 ("Sicherheit schlägt alles").
- 36 neue Unit-Tests über 5 Testdateien: `internal/recording/memory_recorder_test.go`, `internal/safetyservice/bus_test.go`, `cmd/safety-service/main_test.go`, `internal/webrtcsfu/sfu_test.go`, `cmd/webrtc-sfu/main_test.go`. `-race`-sauber für `safetyservice`/`webrtcsfu`. Reine Testabdeckung, 0 Produktivcode-Änderungen.
- Bewusst nicht Teil des Scopes: E2E-WebRTC-SDP-Negotiation (ADR-006: "zu flaky in CI"), `SFU.forwardTrack`, Hexagonal-Migration der drei Services selbst.
- Verifiziert: `go build`/`go vet` sauber, `go test ./...` grün für alle betroffenen Pakete (vorbestehende `tests/integration/...`-Fehlschläge unverändert, da Docker-Test-Stack nicht gestartet).

## Sprint 38 — MQTT-Authentifizierung (Mosquitto Passwort-File) ✅
2026-07-19 → [tasks/sprints/38-mqtt-authentifizierung.md](sprints/38-mqtt-authentifizierung.md)
- Schließt seit Sprint 9 offenen Sicherheits-Gap: Mosquitto lief mit `allow_anonymous true` — jeder Prozess im `avoc-net` konnte ohne Credentials publizieren/subscriben. Nutzerentscheidung 2026-07-19, Priorität vor Hexagonal-Migration Schritt 3 (CLAUDE.MD §0 "Sicherheit schlägt alles").
- `password_file`-Mechanismus aktiviert (Dev/Test/Prod), alle 3 Go-MQTT-Clients (`telemetryservice`, `fleetgateway`, `vehicle-mock`) setzen `MQTT_USERNAME`/`MQTT_PASSWORD`, Prod-Credential nur über SSM SecureString + deploy-zeitige Passwd-Generierung auf dem EC2-Host (nie committed).
- Bewusst nicht Teil des Scopes: TLS/MQTTS (Transportverschlüsselung) — Klartext-Credentials über das interne Docker-Bridge-Netzwerk, analog `DATABASE_URL`.
- Verifiziert: `go build`/`go vet` sauber, `go test ./...` ohne Regression, `make test-integration` 30/30 grün gegen den echten authentifizierten Broker, manuelle Dev-Stack-Verifikation bestätigt Roundtrip UND Ablehnung anonymer Verbindungen ("Connection Refused: not authorised").

## Sprint 37 — Hexagonale Architektur-Migration, Schritt 2: auth-service (JWT-Port) ✅
2026-07-19 → [tasks/sprints/37-auth-service-hexagonal-jwt-port.md](sprints/37-auth-service-hexagonal-jwt-port.md)
- ADR-031 Schritt 2: `TokenIssuer`-Port + `JWTTokenIssuer`-Adapter (`internal/authservice/tokenissuer.go`) — `Handler.secret` → `Handler.tokens TokenIssuer`, `NewHandler`-Signatur unverändert.
- `handler.go` importiert `golang-jwt/jwt/v5` danach nicht mehr direkt (`Claims`-Typ mit nach `tokenissuer.go` verschoben) — reine Dependency Inversion, keine neue DB-Unabhängigkeit (anders als beim `fleet-service`-Piloten).
- Verifiziert: `go build`/`go vet` sauber, `go test ./internal/authservice/...` 22/22 grün, kein Docker-Stack nötig. HEXAUTH-04 (Use-Case-Extraktion) und `telemetry-service` (Schritt 3) bleiben eigene Folgeschritte.

## Sprint 36 — Restposten-Bereinigung III (Tech Debt, Formatierung, Prod-Lücke, Test-Gap) ✅
2026-07-19 → [tasks/sprints/36-restposten-bereinigung-iii.md](sprints/36-restposten-bereinigung-iii.md)
- TECHDEBT-01: `internal/authservice/noop_userstore.go` (komplett unreferenziert) gelöscht.
- GOSTYLE-FMT-01: `gofmt -w .` für die 9 verbleibenden unformatierten Dateien, reine Whitespace-Änderung.
- DEPLOY-08: `fleet-service` in `infrastructure/compose/docker-compose.prod.yml` ergänzt (Port 8085, Postgres+Mosquitto-Dependencies, Env analog `auth-service`/`telemetry-service`).
- TESTGAP-01: 3 skippende WS-Integrationstests gefixt — zusätzlich zur geplanten Reihenfolge-Umkehr musste die Test-Vehicle-Registrierung über `/vehicle/ws` ergänzt werden (`vehicleRegistry.Connected`-Check in `handleSessionStart`, bisher nicht dokumentiert). Gegen echten Docker-Test-Stack verifiziert: 27/27 grün, 0 Skips.

## Sprint 35 — GOSTYLE-IF-03/04 (Phase-2-Abschluss Interface-Segregation) ✅
2026-07-18 → [tasks/sprints/35-gostyle-if-phase2-abschluss.md](sprints/35-gostyle-if-phase2-abschluss.md)
- `pkg/audit.AuditWriter` in schlankes `SafetyAuditWriter` (`WriteSync`-only) für die 5 Safety-/Command-Consumer aufgespalten, `Close` aus beiden Interfaces entfernt (`NoopWriter.Close` dadurch tot geworden und mit gelöscht).
- `SeedAdmin` aus `authservice.UserStore` entfernt; Nebenbefund `NoopUserStore` komplett ungenutzt bestätigt und als `TECHDEBT-01` im Backlog erfasst.
- Damit ist Phase 2 (Interface-Segregation, Rule 4.2/4.3) des Go-Coding-Style-Guide-EPICs vollständig abgeschlossen.

## Sprint 34 — JWT-Alg-Confusion-Fix (SEC-01) + GOSTYLE-IF-01/02/05/06 ✅
2026-07-18 → [tasks/sprints/34-jwt-fix-gostyle-if.md](sprints/34-jwt-fix-gostyle-if.md)
- SEC-01: fehlender Signaturmethoden-Check an 3 von 5 `jwt.Parse*`-Stellen behoben (Alg-Confusion), Regressionstests mit gefälschtem `alg:none`-Token.
- GOSTYLE-IF-01/02/05/06: `SessionRecorder` als tote Abstraktion entfernt, `FleetGateway` tatsächlich interface-typisiert gemacht, `VehicleStore`/`safety.Publisher` um Bootstrap-Methoden verschlankt.

## Sprint 33 — Indoor-Kartenrendering (ADR-034) + Hexagonal-Pilot-Start (HEX-01..05) ✅
2026-07-18 → [tasks/sprints/33-indoor-kartenrendering-hexagonal-pilot.md](sprints/33-indoor-kartenrendering-hexagonal-pilot.md)
- ADR-034: `vehicle_status.position_x/y` ergänzt, `FleetIndoorMap.tsx` rendert SVG-Koordinatensystem (Befüllung durch Simulator bewusst ausgeklammert).
- HEX-01..05: Hexagonal-Pilot auf `fleet-service` — `FleetStore`-Repository-Port (19 Methoden) + `FakeFleetStore` für Tests.

## Sprint 32 — Klärungsrunde + Route-Historie (ADR-033), WebRTC-Fix, Meilenstein-Dokumente ✅
2026-07-18 → [tasks/sprints/32-route-historie-webrtc-fix.md](sprints/32-route-historie-webrtc-fix.md)
- ADR-033: `vehicle_position_history`-Tabelle, gedrosselter Schreibpfad, 30-Tage-Retention, `GET /fleet/vehicles/{id}/history`.
- WEBRTC-10: `actpass→active`-SDP-Workaround entfernt (inkompatibel mit aktuellem Chrome), Offer bleibt Standard-`actpass`.
- Meilenstein-1/2-Dokumente für den Auftraggeber (IBATOUR) konsolidiert.

## Sprint 31 — AP2-Vervollständigung + Fleet-/Observability-Nacharbeiten ✅
2026-07-18 → [tasks/sprints/31-ap2-vervollstaendigung.md](sprints/31-ap2-vervollstaendigung.md)
- AP2-04 Prioritätenmanagement in der Task-UI, TASKUI-03 Nacharbeiten.
- OBS-01 (Vehicle-Heartbeat, seit Sprint 14 offen) nachgeholt: AckBadge zeigt "zuletzt gesehen vor Xs".

## Sprint 30 — Restposten-Bereinigung (MV-Folge-Tasks, TASKUI-Nacharbeiten, Doku) ✅
2026-07-18 → [tasks/sprints/30-restposten-bereinigung.md](sprints/30-restposten-bereinigung.md)
- MV-09 (Live-State-Badge), MV-11 (Multi-Vehicle-Handover — bei Bearbeitung als bereits erledigt vorgefunden, Commit `f68346a`), MV-12 (`GET /state`-Folgeaufräumung) sowie TASKUI-Nachträge aus Sprint 24.

## Sprint 29 — `control-server` (hohes Risiko) + Abschlussverifikation ✅
2026-07-18 → [tasks/sprints/29-control-server-abschlussverifikation.md](sprints/29-control-server-abschlussverifikation.md)
- Dritter Go-Style-Guide-Rollout-Sprint: `main()` (683 Zeilen) im sicherheitskritischsten Service zerlegt (Rule 2.2), Abschlussverifikation über alle drei Rollout-Sprints (27–29).

## Sprint 28 — Risikoarme Services: Rule 2.2 + 2.3 ✅
2026-07-17 → [tasks/sprints/28-risikoarme-services-rule22-23.md](sprints/28-risikoarme-services-rule22-23.md)
- Zweiter Go-Style-Guide-Rollout-Sprint: `main()`-Funktionen >50 Zeilen und Funktionen >4 Parameter außerhalb von `control-server` zerlegt.

## Sprint 27 — Fundament: `pkg/db`, `pkg/env`, non-blocking Linter-Gate ✅
2026-07-17 → [tasks/sprints/27-fundament-pkg-db-env-linter.md](sprints/27-fundament-pkg-db-env-linter.md)
- Erster Go-Style-Guide-Rollout-Sprint: DB-Open+WaitForReady- und `envOr`-Dreifach-Duplikate in `pkg/db`/`pkg/env` gebündelt; `golangci-lint` als non-blocking Warn-Gate eingerichtet.

## Sprint 25 — Audio-Benachrichtigungen ✅
2026-07-16 → [tasks/sprints/25-audio-benachrichtigungen.md](sprints/25-audio-benachrichtigungen.md)
- Audio-Benachrichtigung bei neuen Fleet-Alerts (`useFleetAlertSound`, Mute-Toggle) — schließt die in Sprint 22 unvollständig umgesetzte Notification-Anforderung.

## Sprint 24 — Task-Management-UI ✅
2026-07-16 → [tasks/sprints/24-task-management-ui.md](sprints/24-task-management-ui.md)
- Task-Erstellung/-Zuweisung, Status-Tracking, Priorität, Task-Historie im Fleet-Overview-Dashboard (`FleetTaskPanel.tsx`), aufbauend auf `fleet-service`.

## Sprint 23 — Outdoor Karten-/Zonen-Visualisierung im Fleet Overview ✅
2026-07-16 → [tasks/sprints/23-outdoor-karten-zonen-visualisierung.md](sprints/23-outdoor-karten-zonen-visualisierung.md)
- SVG-Karte mit Live-Fahrzeugpositionen (`FleetMap.tsx`, Leaflet `svgOverlay`), Scope per Grill-Me auf Outdoor-Zonen begrenzt (Indoor folgte in Sprint 33).

## Sprint 22 — Fleet Overview Dashboard ✅
2026-07-15 → [tasks/sprints/22-fleet-overview-dashboard.md](sprints/22-fleet-overview-dashboard.md)
- Erste Slice des Web-Dashboards (AP2): Fahrzeugliste, Status-/Detail-Panel, Alerts gegen das `fleet-service`-Backend — bewusst ohne Karte/Task-UI (folgten in Sprint 23/24).

## Sprint 21 — Fleet Backend Foundation (fleet-service, Multi-Vehicle-Simulation) ✅
2026-07-14 (Branches gemergt 2026-07-15) → [tasks/sprints/21-fleet-backend-foundation.md](sprints/21-fleet-backend-foundation.md)
- `fleet-service` real aufgesetzt (ADR-027/028/029): Zonen/Stationen/Tasks/Alerts-Endpoints, `FleetGateway`, Multi-Vehicle-Simulator — Fundament für das gesamte AP2-Dashboard.

## Sprint 20 — Bugfix: Session-Neustart nach Session-Ende blockiert ✅
2026-07-10 → [tasks/sprints/20-session-neustart-bugfix.md](sprints/20-session-neustart-bugfix.md)
- Nach Session-Ende mit verbundenem Fahrzeug ließ sich keine neue Session starten (`VehicleSelector` erschien nicht wieder) — behoben.

## Sprint 19 — Lokaler Dev-Stack: Verifikation & Robustheit ✅
2026-07-10 (1 Folge-Task an Backlog übergeben) → [tasks/sprints/19-dev-stack-verifikation.md](sprints/19-dev-stack-verifikation.md)
- Reproduzierbarkeit auf frischem Checkout sichergestellt, SSL/HTTPS-Dev-Frage geklärt, dokumentierte Stolpersteine.

## Sprint 18 — Cleanup & ADR-Vorbereitung
2026-06-18 → [tasks/sprints/18-cleanup-adr-vorbereitung.md](sprints/18-cleanup-adr-vorbereitung.md)
- CI-Blocker (`go vet`-Fehler durch veraltete `handler_test.go`) behoben, Multi-Vehicle-Handover architektonisch vorbereitet (Grill-Me + ADR, kein Code).

## Sprint 17 — Multi-Vehicle State Isolation (ADR-026) ✅
2026-06-14, deployed 2026-06-16 (Commits `32de463`, `d20e9f2`) → [tasks/sprints/17-multi-vehicle-state-isolation.md](sprints/17-multi-vehicle-state-isolation.md)
- State Machine, DeadmanWatchdog, VehicleACKWatchdog von Prozess-Singletons auf `vehiclecontext.Registry` (pro Fahrzeug) umgestellt — zwei Operatoren können zwei Fahrzeuge unabhängig steuern; `SafetyBusWatchdog` bleibt global, fächert aber korrekt auf.

## Sprint 16 — Safety Hardening (ADR-009 Lücken geschlossen) ✅
2026-06-14, Commit `844a6ef` → [tasks/sprints/16-safety-hardening.md](sprints/16-safety-hardening.md)
- `VehicleACKWatchdog` + `SafetyBusWatchdog` (2 fehlende ADR-009-CRITICAL-Trigger) implementiert; 3 Session-Lifecycle-Bugfixes (WS-Disconnect-Race, CONNECTED→IDLE-Transition, Page-Reload-Recovery); 18 neue Unit-Tests.

## Sprint 15 — PostgreSQL-Migration + Nutzerverwaltung ✅
2026-06-14 → [tasks/sprints/15-postgresql-migration-nutzerverwaltung.md](sprints/15-postgresql-migration-nutzerverwaltung.md)
- ADR-023: SQLite vollständig durch PostgreSQL ersetzt. ADR-024: echte Authentifizierung mit bcrypt, ADMIN-Rolle, `LoginPanel`/`UserManagementPanel`.

## Sprint 14 — Security & Observability ✅ (OBS-01 nachgeholt Sprint 31)
2026-06-13 → [tasks/sprints/14-security-observability.md](sprints/14-security-observability.md)
- JWT-Pflicht auf REST-Endpoints (`requireJWT`), Dual-Channel-Latenzanzeige (Control/Video getrennt), Backend-nicht-erreichbar-Banner im Frontend.

## Sprint 13 — Dev-Stack Stabilisierung & Log-Korrelation ✅
2026-06-13 → [tasks/sprints/13-dev-stack-stabilisierung.md](sprints/13-dev-stack-stabilisierung.md)
- HTTP-only Dev-nginx (SSL-Fehler bei `make up` behoben), `vehicle-mock` in `build-prod`/`push` ergänzt, `session_id`-Propagation durch alle Kanäle (vollständige Log-Korrelation).

## Sprint 12 — Vehicle Registry (ADR-022) ✅
2026-06-12 → [tasks/sprints/12-vehicle-registry.md](sprints/12-vehicle-registry.md)
- `VehicleStore`-Interface + SQLite-Implementierung (später Postgres, ADR-023), `GET/POST/DELETE /vehicles`, `VehicleSelector.tsx` — Hardcoded-Vehicle-ID vollständig entfernt.

## Sprint 11 — Vehicle Connectivity & Feedback (ADR-021) ✅
2026-06-11 → [tasks/sprints/11-vehicle-connectivity-feedback.md](sprints/11-vehicle-connectivity-feedback.md)
- `VehicleCommandAck` (WS) + Actuation-Feedback-Felder (MQTT), `vehicleconnection.Registry`/`AckStore`, `vehicle-mock` Docker-Service, `InputIndicatorPanel.tsx`.

## Sprint 10 — Browser WebRTC ICE Migration ✅
2026-06-10 → [tasks/sprints/10-webrtc-ice-migration.md](sprints/10-webrtc-ice-migration.md)
- coturn als eigenständiger STUN/TURN-Service (`network_mode: host`), `GET /ice-config`-Endpoint, DTLS-Fix in `useWebRTC.ts`, Deploy auf EC2.

## Sprint 9 — WebRTC Videostream: Larix WHIP → MediaMTX → Browser ✅
2026-06-05 → [tasks/sprints/09-webrtc-videostream-larix-mediamtx.md](sprints/09-webrtc-videostream-larix-mediamtx.md)
- ADR-020: MediaMTX als WHIP/WHEP-Router löst das Custom-SFU-Signaling ab; Control Server als einzige Auth-Instanz (`/internal/media/auth`) + SAFE_MODE-Kick.

## Sprint 8 — EC2 Deployment via Docker Hub ✅
2026-06-04 → [tasks/sprints/08-ec2-deployment-docker-hub.md](sprints/08-ec2-deployment-docker-hub.md)
- ADR-019: Docker-Hub-private-Repos + EC2 Elastic IP + SSM Parameter Store. `make build-prod`/`push`, `docker-compose.prod.yml`, `deploy.sh`, EC2-Bootstrap-Guide.

## Sprint 7 — Logging & Audit Trail ✅
2026-06-04 → [tasks/sprints/07-logging-audit-trail.md](sprints/07-logging-audit-trail.md)
- `pkg/logger` (strukturiertes slog, ADR-017) über alle Services ausgerollt; `pkg/audit` (`AuditWriter`, ursprünglich SQLite-WAL — seit ADR-023 PostgreSQL); Loki + Grafana + Promtail.

## Sprint 6 — Testing & Quality Gates ✅
2026-06-04 → [tasks/sprints/06-testing-quality-gates.md](sprints/06-testing-quality-gates.md)
- Integration-Test-Stack (`docker-compose.test.yml`), Vitest + RTL + Playwright, Performance/Latency-Tests (Go Benchmark + k6), README-Contributor-Guide.

## Sprint 5 — Feature Completion Frontend ✅
2026-06-03 → [tasks/sprints/05-feature-completion-frontend.md](sprints/05-feature-completion-frontend.md)
- Control Panel (Keyboard/Joystick/Gamepad, 20 Hz Command Loop), Video Panel (WebRTC RTCPeerConnection), Dashboard-Integration mit Telemetrie.

## Sprint 4 — Core Backend Services ✅
2026-06-03 → [tasks/sprints/04-core-backend-services.md](sprints/04-core-backend-services.md)
- Command Engine (Rate Limiting, Protobuf-Routing), MQTT Telemetry Service, Session Recording (abstraktes Interface + `MemoryRecorder`), WebRTC SFU (Pion).

## Sprint 3 — Frontend Core ✅
2026-06-03 → [tasks/sprints/03-frontend-core.md](sprints/03-frontend-core.md)
- Protobuf-Frontend-Adapter + Build-Pipeline, WebSocket-Client mit State-Polling, SAFE-MODE-Overlay + Operator-Ack-Flow, Emergency-Stop + Dead-man-Switch.

## Sprint 2 — Safety & Failure Model ✅
2026-06-03 → [tasks/sprints/02-safety-failure-model.md](sprints/02-safety-failure-model.md)
- Vehicle Connection Service, Session Manager (GSA), Failure Detection (DeadmanWatchdog + ACKTimeoutWatcher), Operator Handover — erste vollständige Safety Test Suite (19/19).

## Sprint 1 — Foundation Layer ✅
2026-06-03 → [tasks/sprints/01-foundation-layer.md](sprints/01-foundation-layer.md)
- Proto-Schema-Repository, React-Projekt-Setup, Auth-Service (JWT), coturn, Safety Event Bus (In-Memory), Control-Server-WebSocket+JWT, Dockerfiles, Docker-Compose-Orchestrierung.
