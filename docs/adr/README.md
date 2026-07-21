# Architecture Decision Records (ADRs)

Alle Architekturentscheidungen des AVOC-Systems. Jede Entscheidung ist unveränderlich — neue Erkenntnisse führen zu einem neuen ADR, niemals zur stillen Überschreibung eines bestehenden.

Kernentscheidungen, Status und offene Folge-Entscheidungen je ADR: **[DECISIONS.MD](../../DECISIONS.MD)** — die Live-Übersicht. Diese Datei hier ist nur die reine Dateiliste zum schnellen Blättern.

---

## ADR-Index (36 ADRs)

| ADR | Titel |
|-----|-------|
| [ADR-001](001-backend-language.md) | Backend-Sprache |
| [ADR-002](002-safety-channel.md) | Safety Channel |
| [ADR-003](003-mqtt-broker.md) | MQTT Broker |
| [ADR-004](004-authentication.md) | Authentifizierung |
| [ADR-005](005-session-recording.md) | Session Recording |
| [ADR-006](006-testing-strategy.md) | Testing Strategy |
| [ADR-007](007-system-runtime-topology.md) | System Topologie |
| [ADR-008](008-message-protocol.md) | Message Protocol |
| [ADR-009](009-failure-model.md) | Failure Model |
| [ADR-010](010-control-loop.md) | Control Loop |
| [ADR-011](011-system-state-machine.md) | State Machine |
| [ADR-012](012-message-flow-runtime.md) | Message Flow |
| [ADR-012b](012b-message-flow-runtime-sync-codegen.md) | Sync/Async & Code-Gen |
| [ADR-013](013-frontend-tech-stack.md) | Frontend Stack |
| [ADR-014](014-video-streaming.md) | Video Streaming (abgelöst durch ADR-020) |
| [ADR-015](015-session-coordinator.md) | Session Coordinator |
| [ADR-016](016-session-correlation-id.md) | Correlation ID |
| [ADR-017](017-logging-strategy.md) | Logging Strategy |
| [ADR-018](018-audit-trail-strategy.md) | Audit Trail Strategy |
| [ADR-019](019-deployment-strategy.md) | Deployment-Strategie |
| [ADR-020](020-mediamtx-whip-whep.md) | MediaMTX WHIP/WHEP |
| [ADR-021](021-vehicle-connectivity-feedback.md) | Vehicle Connectivity & Feedback |
| [ADR-022](022-vehicle-registry.md) | Vehicle Registry |
| [ADR-023](023-postgresql-migration.md) | PostgreSQL-Migration |
| [ADR-024](024-user-management.md) | User Management |
| [ADR-025](025-multi-operator.md) | Multi-Operator |
| [ADR-026](026-multi-vehicle-state-isolation.md) | Multi-Vehicle State Isolation |
| [ADR-027](027-fleet-gateway-interface.md) | Fleet Gateway Interface |
| [ADR-028](028-autonomy-first-operator-model.md) | Autonomy-First Operator Model |
| [ADR-029](029-fleet-vehicle-data-model.md) | Fleet Vehicle Data Model |
| [ADR-030](030-task-status-lifecycle.md) | Task-Status-Lifecycle & manueller Status-Übergangs-Endpoint |
| [ADR-031](031-hexagonal-architecture-migration.md) | Hexagonale Architektur-Migration |
| [ADR-032](032-task-status-history.md) | Task-Status-Audit-Historie |
| [ADR-033](033-vehicle-position-history.md) | Vehicle Position History (gefahrene Route) |
| [ADR-034](034-indoor-vehicle-position.md) | Indoor-Fahrzeugposition |
| [ADR-035](035-control-server-hexagonal-migration-prep.md) | control-server Hexagonal-Migration — Vorbereitung (Testaufbau vor Refactor) |

---

## Template

Neue ADRs verwenden [000-template.md](000-template.md) als Basis. Nach dem Anlegen: Eintrag hier
UND in [DECISIONS.MD](../../DECISIONS.MD) ergänzen.
