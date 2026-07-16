# DECISIONS.MD — Live-Übersicht aller Architekturentscheidungen

## Aktive ADRs

| ADR | Titel | Status | Entscheidung |
|-----|-------|--------|--------------|
| [ADR-001](docs/adr/001-backend-language.md) | Backend-Sprache & Framework | Accepted | Go |
| [ADR-002](docs/adr/002-safety-channel.md) | Safety Channel | Accepted | Safety Event Bus (kein DDS), DDS-Interface-kompatibel |
| [ADR-003](docs/adr/003-mqtt-broker.md) | MQTT Broker | Accepted | Eclipse Mosquitto |
| [ADR-004](docs/adr/004-authentication.md) | Authentifizierung | Accepted | Separater Auth Service (Operator + Vehicle, OAuth-erweiterbar) |
| [ADR-005](docs/adr/005-session-recording.md) | Session Recording | Accepted | Abstraktes Recording Interface (Persistenz als Folge-ADR offen) |
| [ADR-006](docs/adr/006-testing-strategy.md) | Testing Strategy | Accepted | testing+testify / Docker / Safety Test Suite / Jest+Playwright / CI Latenz / WebRTC E2E non-blocking. Update 2026-07-15: verpflichtende Fallgruppen-Checkliste pro Task, siehe CLAUDE.MD Abschnitt 17 |
| [ADR-007](docs/adr/007-system-runtime-topology.md) | System Runtime Topology | Accepted | Hub-and-Spoke (Control Hub ≠ Data Hub); CONTROL HUB > VIDEO HUB; SFU = orthogonaler zweiter Hub |
| [ADR-008](docs/adr/008-message-protocol.md) | Message Protocol Contract Layer | Accepted | Protobuf (Application Bus); WebRTC Signaling bewusst außerhalb |
| [ADR-009](docs/adr/009-failure-model.md) | Failure Model & System Behavior under Faults | Accepted | CRITICAL/DEGRADED/OBSERVATION; Video=DEGRADED; Command ACK Timeout=CRITICAL; 3 formale Invarianten. Update 2026-07-17 (DRIFT-K1/K2/K3): Auth Invalidation (neuer `AuthWatchdog`) und No Active Operator (`TransitionOperator(OpNoOperator)`) jetzt produktiv verdrahtet, teilen denselben kanonischen Trigger; MEDIA_DEGRADED-Schwellwerte + DEGRADED→CONNECTED-Recovery ergänzt; Partial Telemetry Loss bewusst zurückgestellt |
| [ADR-010](docs/adr/010-control-loop.md) | Control Loop Definition & Safety Override | Accepted | Event-driven Stream / ACK-Roundtrip / Channel Close |
| [ADR-011](docs/adr/011-system-state-machine.md) | System State Machine | Accepted | 4-Layer: SYSTEM / CONTROL / MEDIA / OPERATOR STATE |
| [ADR-012](docs/adr/012-message-flow-runtime.md) | Message Flow Runtime Model | Accepted | Field-based Protobuf Versioning, CI Schema-Gate |
| [ADR-012b](docs/adr/012b-message-flow-runtime-sync-codegen.md) | Sync/Async-Modell & Code Generation | Accepted | Safety/MQTT async; Auth async+lokale JWT; Frontend sync ACK; Build-time protoc Code-Gen |
| [ADR-013](docs/adr/013-frontend-tech-stack.md) | Frontend Technology Stack | Accepted | React 18+ + TypeScript + Vite + Tailwind CSS + Shadcn/ui + protoc-gen-es |
| [ADR-014](docs/adr/014-video-streaming.md) | Video Streaming Technologie | Superseded by ADR-020 | WebRTC SFU (Pion/Go) + coturn; 1 Primary + 1-2 Secondary; server-seitiges Recording; Handover |
| [ADR-015](docs/adr/015-session-coordinator.md) | Session Coordinator | Accepted | Control Session als primäre Einheit; GSA = Control Server; Ephemeral + Checkpoint; SFU Event-Push |
| [ADR-016](docs/adr/016-session-correlation-id.md) | Session Correlation ID | Accepted | ULID; Control Server generiert bei CONNECTING→CONNECTED; Vehicle-ID→Session-ID→Event-ID; JWT = Identity only |
| [ADR-017](docs/adr/017-logging-strategy.md) | Logging Strategy | Accepted | Hybrid: Technical Logs async (slog → Loki); Safety Events sync (AuditWriter.WriteSync); 3 Log-Klassen; Frontend via POST /log; Interface-first (ADR-018) |
| [ADR-018](docs/adr/018-audit-trail-strategy.md) | Audit Trail Strategy | Accepted | SQLite WAL (embedded, kein extra Service) als AuditWriter-Implementierung; garantierte Persistenz Safety Events; fsync vor SAFE_MODE |
| [ADR-019](docs/adr/019-deployment-strategy.md) | Deployment-Strategie | Accepted | Docker Hub private Repos + EC2 Elastic IP + AWS SSM Parameter Store; linux/amd64; kein Quellcode auf EC2; deploy.sh als Deployment-Mechanismus |
| [ADR-020](docs/adr/020-mediamtx-whip-whep.md) | Video Ingestion & Distribution | Accepted | MediaMTX als WHIP/WHEP Router; Fahrzeug → WHIP; Browser → WHEP; Control Server = einzige Auth-Instanz; Control Server kickt MediaMTX bei SAFE_MODE. Sprint 10 Nachtrag: `webrtcICEServers2` entfernt — Browser übernimmt ICE-Gathering via `/api/ice-config`. Update 2026-07-10: Larix Broadcaster nicht mehr Referenz-Client, siehe ADR-020 |
| [ADR-021](docs/adr/021-vehicle-connectivity-feedback.md) | Vehicle Connectivity & Feedback | Accepted | JWT `sub=vehicleID` (Auth Service); Protobuf end-to-end; VehicleForwarder-Interface; Zwei-Stufen-Feedback: WS ACK (Transport) + MQTT Telemetrie (Aktuator-Ist-Werte); vehicle-mock Docker-Service |
| [ADR-022](docs/adr/022-vehicle-registry.md) | Vehicle Registry | Accepted | SQLite/Postgres `vehicles`-Tabelle; VehicleSelector UI; SeedDefault vehicle-001 |
| [ADR-023](docs/adr/023-postgresql-migration.md) | PostgreSQL-Migration | Accepted | PostgreSQL ersetzt SQLite vollständig; gemeinsame DB `avoc`; separate Connection-Pools |
| [ADR-024](docs/adr/024-user-management.md) | User Management | Accepted | bcrypt-Passwörter; ADMIN-Rolle; Auto-Seed Admin; `/auth/users` CRUD hinter RequireAdmin |
| [ADR-025](docs/adr/025-multi-operator.md) | Multi-Operator | Accepted | ACTIVE_OPERATOR/OBSERVER pro Vehicle; `vehicleController`-Map; WS connect nach `session/start` |
| [ADR-026](docs/adr/026-multi-vehicle-state-isolation.md) | Multi-Vehicle State Isolation | Accepted, implementiert + deployed (Sprint 17, 2026-06-16) | `vehiclecontext.Registry` — pro Fahrzeug eigene State Machine + Deadman-/ACKTimeout-/VehicleACK-Watchdog; SafetyBusWatchdog fächert fleet-weit auf; `GET /vehicles/{id}/state` ergänzt (`GET /state` als Compat-Shim erhalten) |
| [ADR-027](docs/adr/027-fleet-gateway-interface.md) | Fleet Gateway Interface | Accepted (Strategie), Schnittstellenparameter vorläufig | Abstraktes Interface gegen das noch unbestätigte externe ROS2/DDS-Fahrzeugbackend (IBATOUR); Mock jetzt, Adapter nach AP1-Workshop mit Professur Logistik |
| [ADR-028](docs/adr/028-autonomy-first-operator-model.md) | Autonomy-First Operator Model | Accepted | Flotte fährt autonom als Normalfall, Teleop nur Notfall-Ausnahme (fahrzeuginitiiert via Alert); `NO_OPERATOR→SAFE_MODE` (ADR-009/011) gilt nur innerhalb aktiver Session; Vehicle-WS bereits session-unabhängig. Update 2026-07-15: zusätzlich proaktive Übernahme durch Operator möglich, wenn Fahrzeug keinen aktiven Operator hat (Fleet Overview Dashboard) |
| [ADR-029](docs/adr/029-fleet-vehicle-data-model.md) | Fleet Vehicle Data Model | Accepted | Neuer `fleet-service`; bestehende `vehicles`-Tabelle bleibt Identitäts-Quelle (+`vehicle_type`), neue FK-Tabellen (vehicle_status/zones/stations/tasks/alerts); Frontend führt Services clientseitig zusammen; Indoor+Outdoor beide SVG, Outdoor geo-referenziert. Update 2026-07-16 (Sprint 23): `svg_geometry`/`geo_bounds`-Wire-Format verbindlich definiert (Leaflet `svgOverlay`, `geo_bounds` als JSON-String mit `sw`/`ne`-Ecken) |
| [ADR-030](docs/adr/030-task-status-lifecycle.md) | Task-Status-Lifecycle & manueller Status-Übergangs-Endpoint | Accepted | Zustandsmaschine `pending→in_progress→completed`/`cancelled` (beide terminal); `PATCH /fleet/tasks/{id}/status`, atomares herkunftsbeschränktes UPDATE (race-safe), `ErrTaskNotFound`/`ErrInvalidTransition` → 404/409; `status_changed_by`-Spalte statt Audit-Tabelle (Scope-Entscheidung); Fix: `vehicle_status.current_task_id` wird bei Terminal-Status genullt; Demo-Seed `-taskui`-Namensraum (akzeptiertes Merge-Risiko mit paralleler Sprint-23-Session) |
| [ADR-031](docs/adr/031-hexagonal-architecture-migration.md) | Hexagonale Architektur-Migration | Accepted (Strategie), nur Pilot beauftragt | Strangler-Fig statt Big-Bang; Pilot ausschließlich `fleet-service` (`FleetStore`-Repository-Port); Scope nur Go-Backend; Start nachgelagert nach AP2/AP3; `control-server` explizit ausgeklammert (höchstes Risiko, geringste Testabdeckung); Fortsetzung auf weitere Services ist expliziter Entscheidungspunkt nach Pilot, kein Automatismus |

## Offene Folge-Entscheidungen

| Thema | Blockiert | Referenz |
|-------|-----------|----------|
| Prioritätsmodell technisch (getrennte Channels vs. Header-Flag) | offen | ADR-008 Folge — BE-04 nutzt Publisher-Pattern; explizite Channel-Trennung noch offen |
| Session Recording Storage (DB/Files/Object Storage) | offen | ADR-005 Folge — MemoryRecorder seit Sprint 4 als Platzhalter aktiv; Storage-ADR ausstehend |
| DDS-Produktivimplementierung | Nicht in diesem Scope | ADR-002 Folge — Safety Event Bus als DDS-Drop-in implementiert |
| Backup-Strategie Audit Store (SQLite Volume → S3) | nach Sprint 9 | ADR-018 Folge — S3-Bucket im CDK vorhanden |
| Migration zu AWS ECR | nach Sprint 9 | ADR-019 Folge — für Produktivbetrieb sinnvoll (IAM statt Docker Hub Credentials) |
| HTTPS / TLS-Terminierung auf EC2 | nach Sprint 9 | ADR-019 Folge — für Testphase HTTP akzeptabel |
| MQTT-Authentifizierung (Mosquitto Passwort-File) | nach Sprint 9 | Port 1883 aktuell ohne Auth offen — Vehicle-IP-Einschränkung in SG empfohlen |
| Multi-Vehicle / vehicleId-Routing in MediaMTX | ✅ ADR-022 | VehicleSelector + Registry; `~^vehicle-.*`-Regex aktiv |
| WHEP Auth für mehrere Operatoren (Session-basiert) | offen | ADR-020 Folge |
| E2E Smoke Test (Browser WiFi + 5G TURN-Relay) | offen | WEBRTC-09 Rest — WHIP-Quelle nötig |
| vehicle-mock in Makefile GO_SERVICES | offen | ADR-021 Folge — `make build-prod` / `make push` bauen vehicle-mock nicht |
| session_id in MQTT-TelemetryEvent | offen | ADR-021 Folge — vehicle-mock setzt kein session_id; Log-Korrelations-Gap |
| Dev-Stack SSL-Fix | offen | Sprint-10-Regression — nginx.conf erwartet SSL-Cert; `make up` startet Frontend nicht |
| Multi-Vehicle Safety-Isolation (globale `sm`/Watchdog-Singletons) | ✅ ADR-026 | `vehiclecontext.Registry` pro Fahrzeug — implementiert + deployed Sprint 17 (2026-06-16) |
| E-Stop ohne vehicle_id (fleet-weiter Notaus) | ✅ ADR-026 | Implementiert: ohne `vehicle_id` → alle `ActiveVehicleIDs()`; mit `vehicle_id` → gezielt. Getestet (Integration + Unit) |
| Vehicle-Dropdown Live-State-Badge pro Fahrzeug | offen | ADR-026 Folge (MV-09) — bewusst ausgeklammert (Grill-Me 2026-06-14) |
| GC für `VehicleContext`-Instanzen bei großer Flotte | offen | ADR-026 Folge (MV-10) — aktuell dauerhaft behalten (kleine Flotte) |
| Multi-Vehicle Handover | offen | ADR-026 Folge (MV-11) — `HandoverManager` nutzt noch eine einzelne globale State Machine für die OPERATOR-Schicht |
| `GET /state` final entfernen | offen | ADR-026 Folge (MV-12) — Frontend migriert seit MV-07, nur noch `latency.js`/`services_test.go` hängen daran |
| `internal/authservice/handler_test.go` veraltet | offen | AUTH-TEST-01 — unabhängig von ADR-026, kompiliert nicht (`go vet`), entdeckt während Sprint-17-Verifikation |
| Konkrete ROS2/DDS-Schnittstelle zum externen Fahrzeug-Backend | offen | ADR-027 Folge — Workshop mit Professur Logistik (Teil von AP1, IBATOUR) noch ausstehend; `FleetGateway` läuft bis dahin gegen Mock |
| Handshake-basierte Autonomie-Rückgabe (statt einfachem Session-Ende) | offen | ADR-028 Folge — bewusst zurückgestellt, `endSession()` reicht für den Anfang |
| Persistenzform für "gefahrene Route" (Historie) | offen | ADR-029 Folge — Zeitreihen-DB vs. einfache Tabelle noch nicht entschieden |
| Indoor-Fahrzeugposition auf der Karte (`position_x/y` fehlt für Fahrzeuge) | offen | ADR-029 Folge — `VehicleStatus`/`FleetVehicle` haben nur `position_lat/lon`+`position_zone_id`, kein `position_x/y` wie `Station`; blockiert Indoor-Kartenrendering, Sprint 23 (MAP) bewusst auf Outdoor beschränkt (Grill-Me 2026-07-16); jetzt als `AP2-02` in `tasks/backlog.md` erfasst |
| `GET /fleet/zones`/`/fleet/stations` liefern `null` statt `[]` bei leerer Tabelle | offen | Sprint 23 (MAP-01) gefunden — Frontend normalisiert defensiv (`?? []`), Backend-Fix (`ListZones`/`ListStations`/potenziell `ListVehiclesWithStatus`/`ListAlerts`) nicht in diesem Sprint; identisches Muster für `ListTasks` im `taskui`-Branch (Sprint 24) bereits gefixt, siehe dessen `TASKUI-04` |
| Task-Status-Übergangstabelle dupliziert (Go Backend + TypeScript Frontend) | offen | ADR-030 Folge — Frontend spiegelt die Zustandsmaschine nur für UI-Button-Sichtbarkeit, Backend bleibt autoritativ; bei künftiger Änderung der Zustandsmaschine müssen beide Stellen angepasst werden |
| Vollständige Task-Status-Audit-Historie (mehrere Übergänge, nicht nur der letzte) | offen | ADR-030 Folge — bewusst auf `status_changed_by` (nur letzter Übergang) statt eigener `task_status_history`-Tabelle begrenzt (Scope-Entscheidung, Grill-Me 2026-07-16) |
| ~~ADR-Nummernkollision ADR-030~~ | gelöst | Der `-hexarch`-Branch hatte unabhängig vom `taskui`-Branch am 2026-07-16 ebenfalls ein `ADR-030` angelegt. Gelöst beim Merge (2026-07-16): `taskui`s `030-task-status-lifecycle.md` behält die Nummer, das Hexagonal-Migrations-ADR wurde auf **ADR-031** verschoben (`031-hexagonal-architecture-migration.md`) — siehe `AP2-01` in `tasks/backlog.md` |
| Fortsetzung Hexagonal-Migration auf auth-/telemetry-service nach Pilot | offen | ADR-031 Folge — expliziter Entscheidungspunkt nach HEX-05, kein Automatismus |
| Hexagonal-Migration `control-server` | Nicht in diesem Scope | ADR-031 Folge — bräuchte eigenes ADR + Testabdeckung-Aufbau zuerst; höchstes Risiko im Projekt |
| Testabdeckung `safety-service`/`webrtc-sfu`/`internal/recording` (0 Tests) | offen | ADR-031 Bestandsaufnahme — Voraussetzung, bevor dort überhaupt eine Hexagonal-Migration sinnvoll wäre |
| Partial Telemetry Loss → DEGRADED (kein Produktivpfad) | offen | ADR-009 Update (2026-07-17, DRIFT-K3-TELEMETRY) — bewusst zurückgestellt, bräuchte neuen Cross-Service-Watchdog gegen `telemetry-service` (bisher keine Kopplung zu `control-server`); Backlog-Task angelegt |
