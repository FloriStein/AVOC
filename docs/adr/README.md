# Architecture Decision Records (ADRs)

Alle Architekturentscheidungen des AVOC-Systems. Jede Entscheidung ist unveränderlich — neue Erkenntnisse führen zu einem neuen ADR, niemals zur stillen Überschreibung eines bestehenden.

Vollständige Live-Übersicht: [DECISIONS.MD](../../DECISIONS.MD)

---

## ADR-Index (29 ADRs)

| ADR | Titel | Kernentscheidung |
|-----|-------|-----------------|
| [ADR-001](001-backend-language.md) | Backend-Sprache | Go |
| [ADR-002](002-safety-channel.md) | Safety Channel | Safety Event Bus (Go, In-Memory, DDS-ready) |
| [ADR-003](003-mqtt-broker.md) | MQTT Broker | Eclipse Mosquitto |
| [ADR-004](004-authentication.md) | Authentifizierung | Separater Auth Service; Operator-Rollen; JWT = Identity only |
| [ADR-005](005-session-recording.md) | Session Recording | Abstraktes Interface; ULID als Session Root Key |
| [ADR-006](006-testing-strategy.md) | Testing Strategy | testing+testify / Docker / Safety Suite / Jest+Playwright / CI Latenz; WebRTC E2E non-blocking |
| [ADR-007](007-system-runtime-topology.md) | System Topologie | Hub-and-Spoke; CONTROL HUB > VIDEO HUB; GSA = Control Server |
| [ADR-008](008-message-protocol.md) | Message Protocol | Protobuf (Application Bus); common.proto mit CorrelationHeader; WebRTC außerhalb |
| [ADR-009](009-failure-model.md) | Failure Model | CRITICAL/DEGRADED/OBSERVATION; 9 CRITICAL-Trigger vollständig implementiert (Sprint 16: VehicleACKWatchdog + SafetyBusWatchdog) |
| [ADR-010](010-control-loop.md) | Control Loop | Event-driven Stream; ACK-Roundtrip <100ms; Channel Close als Safety Override |
| [ADR-011](011-system-state-machine.md) | State Machine | 4-Layer: SYSTEM / CONTROL / MEDIA / OPERATOR |
| [ADR-012](012-message-flow-runtime.md) | Message Flow | Field-based Protobuf Versioning; CI Schema-Gate |
| [ADR-012b](012b-message-flow-runtime-sync-codegen.md) | Sync/Async & Code-Gen | Safety/MQTT async; Auth async+lokal; Frontend sync ACK; Build-time protoc |
| [ADR-013](013-frontend-tech-stack.md) | Frontend Stack | React 18 + TypeScript + Vite + Tailwind + Shadcn/ui + protoc-gen-es |
| [ADR-014](014-video-streaming.md) | Video Streaming | WebRTC SFU (Pion/Go) + coturn; ursprüngliche Media-Routing-Entscheidung; abgelöst durch ADR-020 |
| [ADR-015](015-session-coordinator.md) | Session Coordinator | Control Session als primäre Einheit; GSA; Ephemeral + Checkpoint; SFU Event-Push |
| [ADR-016](016-session-correlation-id.md) | Correlation ID | ULID; Vehicle-ID → Session-ID → Event-ID; JWT = Identity only |
| [ADR-017](017-logging-strategy.md) | Logging Strategy | Hybrid: Technical async (slog → Loki); Safety sync (AuditWriter.WriteSync); 3 Log-Klassen; Interface-first |
| [ADR-018](018-audit-trail-strategy.md) | Audit Trail Strategy | SQLite WAL als AuditWriter; fsync vor SAFE_MODE; garantierte Safety-Event-Persistenz; kein extra Service |
| [ADR-019](019-deployment-strategy.md) | Deployment-Strategie | Docker Hub + AWS EC2 + SSM Parameter Store; kein Quellcode auf Instanz; CDK Infrastructure Stack |
| [ADR-020](020-mediamtx-whip-whep.md) | MediaMTX WHIP/WHEP | MediaMTX als WHIP/WHEP Router; Control Server als einzige Auth- und SAFE_MODE-Instanz; Pion SFU passiv |
| [ADR-021](021-vehicle-connectivity-feedback.md) | Vehicle Connectivity & Feedback | JWT sub=vehicleID; Protobuf end-to-end; WebSocket ACK (Transport) + MQTT Telemetrie (Aktuator-Ist-Werte); vehicle-mock Docker-Service |
| [ADR-022](022-vehicle-registry.md) | Vehicle Registry | SQLite `vehicles`-Tabelle; VehicleSelector UI; manuelles Pre-configure; SeedDefault vehicle-001; ADR-015-Invariante erhalten |
| [ADR-023](023-postgresql-migration.md) | PostgreSQL-Migration | PostgreSQL ersetzt SQLite vollständig; gemeinsame DB `avoc`; separate Connection-Pools pro Service |
| [ADR-024](024-user-management.md) | User Management | bcrypt-Passwörter; ADMIN-Rolle; Auto-Seed Admin; `/auth/users` CRUD hinter RequireAdmin |
| [ADR-025](025-multi-operator.md) | Multi-Operator | ACTIVE_OPERATOR/OBSERVER pro Vehicle; `vehicleController`-Map; WS connect nach `session/start` |
| [ADR-026](026-multi-vehicle-state-isolation.md) | Multi-Vehicle State Isolation | Pro-Fahrzeug State Machine + Watchdogs (`vehiclecontext.Registry`); SafetyBusWatchdog fächert fleet-weit auf; `GET /vehicles/{id}/state` ergänzt — implementiert + deployed (Sprint 17, 2026-06-16) |
| [ADR-027](027-fleet-gateway-interface.md) | Fleet Gateway Interface | Abstraktes Interface gegen noch unbestätigte externe ROS2/DDS-Fahrzeugschnittstelle (IBATOUR); Mock jetzt, Adapter nach AP1-Workshop mit Professur Logistik — konkrete Schnittstellenparameter vorläufig |
| [ADR-028](028-autonomy-first-operator-model.md) | Autonomy-First Operator Model | Flotte fährt autonom als Normalfall; Teleop nur als Notfall-Ausnahme, ausgelöst durch fahrzeugseitig erkannte Probleme via Alert; `NO_OPERATOR→SAFE_MODE`-Regel (ADR-009/011) gilt nur innerhalb aktiver Session, nicht als Dauerzustand für autonome Fahrzeuge; Vehicle-WS bereits heute session-unabhängig (keine Codeänderung) |
| [ADR-029](029-fleet-vehicle-data-model.md) | Fleet Vehicle Data Model | Neuer Service `fleet-service`; bestehende `vehicles`-Tabelle bleibt Identitäts-Quelle (erweitert um `vehicle_type`), neue Tabellen (vehicle_status/zones/stations/tasks/alerts) per FK verknüpft; Frontend führt beide Services clientseitig zusammen; Indoor+Outdoor beide SVG-basiert, Outdoor geo-referenziert (Leaflet Overlay) |
| [ADR-030](030-task-status-lifecycle.md) | Task-Status-Lifecycle & manueller Status-Übergangs-Endpoint | Zustandsmaschine `pending→in_progress→completed`/`cancelled` (beide terminal, sonst 409); `PATCH /fleet/tasks/{id}/status` per atomarem herkunftsbeschränktem UPDATE (race-safe statt read-then-write); `status_changed_by`-Spalte statt volle Audit-Tabelle (bewusste Scope-Entscheidung); Fix des zuvor unbemerkten `vehicle_status.current_task_id`-Staleness-Bugs bei Terminal-Status; Demo-Seed für `zones`/`stations` mit `-taskui`-Namensraum als akzeptiertes Merge-Risiko zur parallelen Sprint-23-Session |

---

## Offene Folge-Entscheidungen

| Thema | Blockiert | Referenz |
|-------|-----------|----------|
| Prioritätsmodell technisch (Channels vs. Header-Flag) | offen | ADR-008 Folge |
| Session Recording Storage (DB/Files/Object Storage) | offen | ADR-005 Folge — MemoryRecorder aktiv seit Sprint 4 |
| DDS-Produktivimplementierung | Nicht in diesem Scope | ADR-002 Folge |
| Backup-Strategie Audit Store (SQLite → S3) | offen | ADR-018 Folge — S3-Bucket im CDK vorhanden |
| HTTPS / TLS-Terminierung auf EC2 | ✅ Sprint 10 | nginx SSL + Self-Signed Cert via deploy.sh implementiert |
| Multi-Vehicle vehicleId-Routing in MediaMTX | ✅ ADR-022 | VehicleSelector + SQLite-Registry; `~^vehicle-.*`-Regex in MediaMTX aktiv |
| JWT-Schutz REST-Endpoints | ✅ Sprint 14 | `requireJWT` Middleware, 9 Endpoints geschützt |
| E2E Smoke Test 5G TURN-Relay | offen | WEBRTC-09 Rest |
| OBS-01 Vehicle Heartbeat | offen | Sprint-14-Bonus — AckBadge "zuletzt gesehen" Timestamp |
| Multi-Vehicle Safety-Isolation (globale `sm`/Watchdog-Singletons) | ✅ ADR-026 | `vehiclecontext.Registry` pro Fahrzeug — implementiert + deployed Sprint 17 |
| E-Stop ohne vehicle_id (fleet-weiter Notaus) | ✅ ADR-026 | Implementiert + getestet (Sprint 17) |
| Vehicle-Dropdown Live-State-Badge pro Fahrzeug | offen | ADR-026 Folge (MV-09) — bewusst ausgeklammert (Grill-Me 2026-06-14) |
| GC für `VehicleContext`-Instanzen bei großer Flotte | offen | ADR-026 Folge (MV-10) — aktuell dauerhaft behalten (kleine Flotte) |
| Multi-Vehicle Handover | offen | ADR-026 Folge (MV-11) — `HandoverManager` nutzt noch eine globale State Machine |
| `GET /state` final entfernen | offen | ADR-026 Folge (MV-12) — nur noch `latency.js`/`services_test.go` hängen daran |
| Task-Status-Übergangstabelle dupliziert (Go Backend + TypeScript Frontend) | offen | ADR-030 Folge — Frontend spiegelt die Zustandsmaschine nur für UI-Button-Sichtbarkeit, Backend bleibt autoritativ |
| Vollständige Task-Status-Audit-Historie (mehrere Übergänge) | offen | ADR-030 Folge — bewusst auf `status_changed_by` (nur letzter Übergang) begrenzt statt eigener Audit-Tabelle |

---

## Template

Neue ADRs verwenden [000-template.md](000-template.md) als Basis.
