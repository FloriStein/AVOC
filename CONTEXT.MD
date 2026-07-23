# CONTEXT.MD — Domänenwissen & Architekturprinzipien

---

## Glossar

| Begriff | Definition |
|---------|-----------|
| **Operator** | Menschlicher Nutzer, der ein Fahrzeug über das System steuert |
| **Vehicle** | Ferngesteuertes Fahrzeug, das Steuerbefehle empfängt |
| **Control Session** | Primäre operative Einheit: 1 Vehicle + 1 Active Operator + 1 Control Server. Einzige Safety-relevante Einheit (ADR-015) |
| **Control Input** | Steuerbefehl (Joystick, Keyboard, Gamepad) vom Operator |
| **Dead-man Switch** | Muss aktiv gehalten werden — Loslassen löst Auto-Stop aus |
| **Emergency Stop** | Sofortige Systemdeaktivierung, bypasses alle anderen Ebenen |
| **Auto-Stop** | Automatischer Stopp bei CRITICAL Failure (Channel Close) |
| **SAFE MODE** | Systemzustand: Fahrzeug steht, keine Commands, Channel geschlossen, UI read-only, Operator-Ack für Recovery |
| **Operator Ack** | Explizite Bestätigung des Operators — vor Aktivierung UND nach Recovery aus SAFE MODE |
| **Recovery Checkpoint** | Zustandssnapshot bei SAFE_MODE-Eintritt (Vehicle-ID, Operator-ID, letzter State, Safety Reason, Session-ID) |
| **Safety Event Bus** | Go-Service, In-Memory; ersetzt DDS in dieser Phase; DDS-Interface-kompatibel (ADR-002) |
| **Global Session Authority (GSA)** | Control Server als einziger Session-Erzeuger, -Verwalter und -Zerstörer (ADR-015) |
| **Control Hub** | Control Server als Rang-1-Hub für Safety Truth und Command Routing (ADR-007) |
| **Video Hub** | MediaMTX WHIP/WHEP Router als Rang-2-Hub für Live-Video Ingestion und Distribution (ADR-007/014/020) |
| **WebRTC SFU** | Selective Forwarding Unit (Pion/Go) — bleibt als Session-Event-Consumer aktiv; Media-Routing durch MediaMTX (ADR-020) |
| **vehicle-mock** | Docker-Go-Service der ein echtes Fahrzeug simuliert: JWT self-gen, WS-Connect zu `/vehicle/ws`, Protobuf-Decode, VehicleCommandAck senden, MQTT-Telemetrie mit Aktuator-Simulation (ADR-021) |
| **VehicleForwarder** | Go-Interface im Command Engine — `ForwardCommand(vehicleID string, data []byte) error`; implementiert durch vehicleconnection.Registry (ADR-021) |
| **VehicleCommandAck** | Protobuf-Nachricht vom Fahrzeug: bestätigt Empfang eines ControlCommand per WebSocket; enthält command_event_id-Echo und received_at_ms (ADR-021) |
| **ActuationFeedback** | Aktuator-Ist-Werte im TelemetryEvent: steer_actual, throttle_actual (mit simuliertem 15%-Lag via Lerp); zeigt was das Fahrzeug tatsächlich ausführt vs. was befohlen wurde (ADR-021) |
| **InputIndicatorPanel** | React-Komponente im Footer: zeigt Fahrzeug-Ist-Zustand (Lenkrad-SVG rotiert mit steerActual, ActuationBars Soll/Ist, AckBadge mit Verbindungsstatus) |
| **VehicleContext** | Bündelt pro Fahrzeug eine eigene `statemachine.Machine` + `DeadmanWatchdog` + `ACKTimeoutWatcher` + `VehicleACKWatchdog`; lazy erzeugt, dauerhaft gehalten (ADR-026, `internal/controlserver/vehiclecontext/registry.go`) |
| **vehiclecontext.Registry** | `map[vehicleID]*VehicleContext` + Mutex im Control Server; ersetzt die bisherigen Prozess-Singletons für State Machine und Watchdogs; verdrahtet in `command.Engine`, `transport.WSHandler`, `vehicleconnection.Handler` (ADR-026, deployed Sprint 17) |
| **Protobuf** | Protocol Buffers — Application Bus für Control/Safety/Telemetry (ADR-008) |
| **CorrelationHeader** | Protobuf-Nachrichtenfeld in allen Schemas: session_id, event_id, vehicle_id, operator_id, timestamp (ADR-016) |
| **ULID** | Universally Unique Lexicographically Sortable Identifier — Format der Session-IDs (zeitlich sortierbar) |
| **CRITICAL Failure** | Fehler der sofortigen Channel Close + SAFE MODE auslöst |
| **DEGRADED State** | Teilausfall (Video/Telemetrie) — Control möglich, Awareness reduziert |
| **OBSERVATION** | *(Sprint 60, DRIFT-M18: nie implementiert)* Ursprünglich geplante Klasse für „Auth Service down" — nie produktiv umgesetzt, s. Failure Classification unten |

---

## Domänenmodell

### Entities

- Vehicle
- Control Session (primäre Einheit — ADR-015)
- Operator (Active / Observer / Standby)
- Control Input
- Video Stream (Primary / Secondary)
- Safety State (inkl. SAFE MODE + Recovery Checkpoint)
- Connection State

### Session-Hierarchie (ADR-015)

```
1. Vehicle Runtime Context   (Transport: WebRTC/MQTT/WS/5G)
2. Control Session           (Safety Layer — primäre Einheit)
   └── 1 Vehicle + 1 Active Operator + 1 Control Server
3. Operator Session          (Identity Layer — JWT/Login)
```

### 4-Layer State Machine (ADR-011, pro Fahrzeug isoliert seit ADR-026)

**Seit ADR-026 existiert die 4-Layer State Machine einmal PRO FAHRZEUG** (`VehicleContextRegistry`), nicht mehr global für den Prozess. Ebenso `DeadmanWatchdog` und `VehicleACKWatchdog`. Nur der `SafetyBusWatchdog` bleibt global (eine geteilte Safety-Service-Instanz), fächert bei Ausfall aber auf alle aktiven Fahrzeuge auf.

**SYSTEM STATE (Safety Truth — Master):**
```
IDLE → CONNECTING → AUTHENTICATED → CONNECTED ⇄ DEGRADED
CONNECTED/DEGRADED → SAFE_MODE (CRITICAL) → RECOVERING → AUTHENTICATED
```
CRITICAL Trigger: WS Disconnect, Safety Bus Failure, Dead-man Timeout, Command ACK Timeout, Auth Invalidation, No Active Operator

**CONTROL STATE** (abhängig von SYSTEM STATE):
`CONTROL_INIT → CONTROL_ACTIVE → CONTROL_BLOCKED (bei SAFE_MODE) → CONTROL_LOST → CONTROL_RECOVERING`

**MEDIA STATE** (WebRTC, unabhängig — beeinflusst nur DEGRADED):
`MEDIA_INIT → MEDIA_NEGOTIATING → MEDIA_CONNECTED ⇄ MEDIA_DEGRADED → MEDIA_FAILED`
MEDIA_FAILED → SYSTEM DEGRADED. **Niemals SAFE_MODE.**

**OPERATOR STATE** (Human Governance):
`NO_OPERATOR → OPERATOR_ASSIGNED → ACTIVE_OPERATOR ⇄ HANDOVER_PENDING → RECOVERING_OPERATOR`
Regel: Max. 1 ACTIVE_OPERATOR. NO_OPERATOR → SYSTEM SAFE_MODE.

---

## Safety Concepts

- **Dead-man Switch:** muss aktiv gehalten werden — Timeout = CRITICAL
- **Emergency Stop:** sofortige Systemdeaktivierung, bypasses alle Ebenen → CRITICAL
- **Auto-Stop:** Channel Close bei CRITICAL Failure (ADR-010)
- **Command ACK Timeout:** CRITICAL Trigger → SAFE_MODE
- **Operator Ack:** vor Aktivierung UND nach Recovery — niemals automatisches Resume
- **SAFE MODE:** Channel geschlossen, CONTROL_BLOCKED, UI read-only, Recovery Checkpoint gespeichert

---

## Failure Classification (ADR-009)

| Klasse | Trigger | Reaktion |
|--------|---------|----------|
| **CRITICAL** | WS Disconnect, Safety Bus Failure, Dead-man Timeout, **Command ACK Timeout** (`ACKTimeoutWatcher`, 100ms — Server-ACK an Operator bleibt aus), **Vehicle ACK Timeout** (`VehicleACKWatchdog`, 1s — Fahrzeug bestätigt weitergeleiteten Befehl nicht), Auth Invalidation (`AuthWatchdog`, 2026-07-16), No Active Operator (`TransitionOperator(OpNoOperator)`, produktiv verdrahtet seit 2026-07-16) | Channel Close → SAFE_MODE |
| **DEGRADED** | Video Lost (MEDIA_FAILED), Video Degraded (MEDIA_DEGRADED, Schwellwert-basiert seit 2026-07-16), Partial Telemetry Loss (noch kein Produktivpfad — siehe unten) | DEGRADED State, Warnung im UI, Control möglich |
| **OBSERVATION** *(korrigiert, Sprint 60)* | War als eigene Klasse für „Auth Service down" geplant, aber nie implementiert (kein `SystemState`-Wert, keine Transition, kein Handler) — Grill-Me-Entscheidung `DRIFT-M18`: kein Folge-Task, Doku korrigiert statt nachgebaut. Der real relevante Fall (Operator-Account wird während einer laufenden Session ungültig) läuft bereits über `AuthWatchdog` als **CRITICAL** (s. o.), der direkt gegen die geteilte Postgres-DB pollt statt gegen auth-service selbst. `/session/start` validiert JWTs rein lokal (HMAC-Secret) und ruft auth-service nie auf — „neue Sessions blockiert, wenn auth-service down ist" hat und hatte nie einen Produktivpfad. Details: ADR-009 Update (2026-07-23). |

**„Command ACK Timeout" ist kein einzelner Mechanismus**, sondern zwei eigenständige Watchdogs mit
unterschiedlichem Timeout/Trigger (beide Teil des `VehicleContext`, ADR-026):
- `ACKTimeoutWatcher` (100ms) — überwacht den **Server-internen** ACK-Roundtrip: Command empfangen
  → `ControlAck` an den Operator zurückgesendet. Bildet das <100ms-Control-Loop-Budget ab.
- `VehicleACKWatchdog` (1s) — überwacht, ob das **Fahrzeug** einen weitergeleiteten Bewegungsbefehl
  (`ForwardCommand`) mit einem `VehicleCommandAck` bestätigt (ADR-021).

**Auth Invalidation / No Active Operator (2026-07-16, DRIFT-K1/K2, ADR-009 Update):** Bis Sprint 26
existierten beide Trigger nur in `tests/unit/safety_test.go`, ohne echten Produktivpfad — Details
und Herleitung: `docs/adr/009-failure-model.md` Update-Block. Beide feuern jetzt über denselben
kanonischen Trigger `TransitionOperator(OpNoOperator)`: einmal vom WS-Disconnect-Handler (SYSTEM
war da bereits SAFE_MODE, korrigiert nur den OPERATOR-Layer), einmal vom neuen `AuthWatchdog`
(5s-Poll × 2 Fails, prüft `users.is_active` direkt gegen die geteilte Postgres-DB — kein
Account = kein aktiver Operator mehr, gleiche Semantik).

**Video Degraded (2026-07-16, DRIFT-K3):** `MEDIA_FAILED` war bereits produktiv verdrahtet
(`useWebRTC.ts` → `POST /media/event`), der Audit-Befund war hier ungenau. Neu: `MEDIA_DEGRADED`
über Paketverlust-/Bitrate-Schwellwerte aus `RTCPeerConnection.getStats()` (initiale, nicht
feldvalidierte Werte), sowie ein bislang fehlender Rückweg DEGRADED → CONNECTED bei Media-Recovery
(vorher blieb SYSTEM STATE nach jedem Videoverlust dauerhaft bei DEGRADED). **Partial Telemetry
Loss bleibt unverdrahtet** — `telemetry-service` ist ein eigener Prozess ohne Kopplung zu
control-server, bewusst zurückgestellt (`DRIFT-K3-TELEMETRY`, `tasks/backlog.md`).

**Formale Invarianten:**
```
INVARIANT 1: Media Layer SHALL NOT influence SAFE_MODE transitions
             except via DEGRADED annotation evaluated by the Control Hub.
INVARIANT 2: SAFE_MODE is exclusively triggered by Control, Safety Bus,
             or Operator-level failures. Never by Media events.
INVARIANT 3: Control Hub is Single Source of Truth for Session State.
```

---

## Communication Layers (entschieden)

| Kanal | Protokoll | Zweck | ADR |
|-------|-----------|-------|-----|
| Control | WebSocket (WSS + JWT), sync ACK | Bidirektionale Steuerung, <100ms | ADR-004/010/012b |
| Telemetry | MQTT (Mosquitto), async | Fahrzeugstatus pub/sub | ADR-003/012b |
| Safety | Safety Event Bus (Go, In-Memory), async | Safety-Events; DDS-Interface-kompatibel | ADR-002/012b |
| Video | MediaMTX WHIP/WHEP + coturn | Live-Video (Fahrzeug/Browser→WHIP, Browser→WHEP), HTTPS-Pflicht für getUserMedia, NAT Traversal | ADR-014/020 |
| Message Format | Protobuf (Application Bus) | Control/Safety/Telemetry + CorrelationHeader | ADR-008/016 |
| WebRTC Signaling | SDP/ICE (WebRTC Standard) | Media Layer — bewusst außerhalb Protobuf | ADR-014 |
| Session Correlation | ULID (Vehicle-ID→Session-ID→Event-ID) | Systemweite Korrelation aller Kanäle | ADR-016 |

> DDS wird in dieser Phase NICHT implementiert. Der Safety Event Bus ist als DDS-Drop-in-Replacement designed.

---

## Systemtopologie (ADR-007)

**Zwei orthogonale Hubs — CONTROL HUB > VIDEO HUB (strikte Hierarchie):**

```
CONTROL HUB (Rang 1 — Safety Truth)
┌─────────────────────────────────┐
│        Control Server           │
│  · State Machine (4-Layer)      │
│  · Safety Decision Engine       │
│  · Session Manager (GSA)        │
│  · SFU Event Publisher          │
└───────┬──────────┬──────────────┘
        │          │
 Auth Service   Safety Bus   MQTT Telemetry
                              (Mosquitto)

VIDEO HUB (Rang 2 — Awareness only)
┌─────────────────────────────────┐
│   MediaMTX (WHIP/WHEP)          │
│  · WHIP-Ingestion (Fahrzeug/5G) │
│  · WHEP-Egress (Browser)        │
│  · Auth-Hook → Control Server   │
│  · SAFE_MODE-Kick per API       │
│                                 │
│   WebRTC SFU (Pion/Go)          │
│  · Session Event Consumer       │
└───────────────┬─────────────────┘
                │ NAT Traversal
           coturn (STUN/TURN)
```

---

## Session Correlation ID (ADR-016)

```
Format:    ULID (zeitlich sortierbar, URL-safe, distributed-safe)
Erzeuger:  Control Server (GSA), Zeitpunkt: CONNECTING → CONNECTED

Hierarchie:
  Vehicle-ID
    └── Session-ID (ULID — Root Anchor, überlebt SAFE_MODE)
          └── Event-ID (ULID — pro Message/Command/Frame)

JWT = Identity (Wer bist du?) ≠ Session-ID (In welchem Kontext?)
```

---

## Backend & Tech Decisions (entschieden)

| Komponente | Technologie | ADR |
|------------|-------------|-----|
| Backend-Services | Go | ADR-001 |
| Test-Framework (Go) | testing + testify | ADR-006 |
| Frontend | React 18+ + TypeScript + Vite + Tailwind CSS + Shadcn/ui | ADR-013 |
| Message Format | Protobuf (Application Bus) | ADR-008 |
| Protobuf Code-Gen | protoc-gen-es (TS) + protoc-gen-go, build-time | ADR-012b/013 |
| MQTT Broker | Eclipse Mosquitto | ADR-003 |
| Video | MediaMTX WHIP/WHEP + coturn | ADR-014/020 |
| Authentifizierung | Separater Auth Service (JWT, Operator + Vehicle) | ADR-004 |
| Vehicle Connectivity | `/vehicle/ws` WebSocket, Protobuf end-to-end, vehicleconnection.Registry | ADR-021 |
| Vehicle Feedback | VehicleCommandAck (WS) + TelemetryEvent Actuation Fields (MQTT) | ADR-021 |
| Session-ID | ULID, generiert vom Control Server (GSA) | ADR-016 |
| Session Recording | Abstraktes Interface (Storage noch offen) | ADR-005 |
| Deployment | Docker Compose (kein Kubernetes) | CONTEXT |

---

## Control Loop Definition (ADR-010)

- **Modell:** Event-driven Stream — Operator-Inputs als kontinuierlicher WebSocket-Event-Stream
- **Latenz-Messpunkt:** Client → Server → ACK zurück (Roundtrip, <100ms = CI Build-Fail)
- **Safety Override:** Channel Close — WebSocket bei CRITICAL Event geschlossen
- **Konsequenz:** Frontend muss Reconnect-Logik nach Channel Close implementieren

---

## Protobuf Versioning & Code Generation (ADR-012/012b)

- **Versioning:** Field-based — keine Field IDs ändern, keine Felder entfernen, neue Features als optionale Felder
- **CI:** Schema-Kompatibilitätsprüfung als Pflicht-Gate
- **Code-Gen:** Build-time via `protoc`, gitignored — `.proto` ist Single Source of Truth
- **Frontend:** protoc-gen-es (TypeScript-Klassen, ADR-013)

---

## Sync/Async-Modell (ADR-012b)

| Kanal | Modell |
|-------|--------|
| Control Server → Safety Event Bus | Async (fire-and-forget) |
| Control Server → Auth Service | Async + lokale JWT-Validierung |
| Frontend → Control Server | Synchron ACK-based (non-blocking UI) |
| Control Server → MQTT Telemetry | Async (one-way) |
| Control Server → SFU | Async Event-Push (SESSION_* Events) |
| Control Server → Vehicle (vehicleconnection.Registry) | Sync Fire-and-Forget WebSocket (ADR-021) |
| Vehicle → Control Server (VehicleCommandAck) | Async WebSocket, gespeichert in AckStore (ADR-021) |
| Frontend → GET /vehicle/ack/latest/ | Polling 500ms (useVehicleAck) |

---

## Assumptions

- Netzwerk ist nicht stabil — Vehicle ↔ Internet ↔ OCC (uncontrolled routing)
- Latenz schwankt stark, kann jederzeit >100ms steigen
- NAT, CGNAT, Firewalls können UDP blockieren — WebRTC mit STUN/TURN löst das
- Kein automatisches Resume nach CRITICAL Failure — Operator-Ack immer erforderlich
- DDS wird in dieser Phase nicht eingesetzt

---

## Risks

| Risiko | Mitigierung |
|--------|-------------|
| Netzwerk-Latenz >100ms | CI-Latenz-Tests, <100ms Build-Fail (ADR-006) |
| NAT/CGNAT blockiert Video | WebRTC + coturn (STUN/TURN) — ADR-014 |
| Safety Event Bus kein echtes DDS | DDS-kompatibles Interface, Austausch per ADR-002 |
| Control Server als Single Point of Failure | Deployment-Strategie als zukünftiges ADR |
| Stale Command Execution nach Reconnect | SAFE MODE + Operator-Ack + Channel Close (ADR-009/010) |
| Inkonsistente Message-Strukturen | Protobuf + CI Schema-Gate (ADR-008/012) |
| Control Hub / Video Hub Zustandsdrift | GSA als Single Source of Truth, SFU Event-Push (ADR-015) |
| WebRTC CI-Flakiness | Non-blocking E2E Tests, Retry 3×, threshold-based (ADR-006) |
| Session Korrelationsverlust bei SAFE_MODE | Session-ID überlebt SAFE_MODE als Root-Anchor (ADR-016) |

---

## Offene Fragen

| Frage | Blockiert | Referenz |
|-------|-----------|----------|
| Prioritätsmodell technisch (Channels vs. Header-Flag) | offen | ADR-008 Folge — BE-04 nutzt Publisher-Pattern; explizite Channel-Trennung noch offen |
| Session Recording Storage (DB / Files / Object Storage) | offen | ADR-005 Folge — MemoryRecorder seit Sprint 4 als Platzhalter aktiv; Storage-ADR ausstehend |
| DDS-Produktivimplementierung | Nicht in diesem Scope | ADR-002 Folge |
| Backup-Strategie Audit Store | offen | ADR-019 möglich — Audit Store läuft seit ADR-023 in der gemeinsamen PostgreSQL-DB `avoc` (nicht mehr SQLite); Backup-Strategie für `avoc` insgesamt weiterhin offen (LOG-10 implementiert) |
| ~~vehicle-mock in Makefile GO_SERVICES~~ | gelöst | `make build-prod`/`push` bauen/pushen `vehicle-mock` inzwischen explizit als eigenes Image (separat von `GO_SERVICES`, siehe `Makefile`) |
| session_id in MQTT-TelemetryEvent | offen | ADR-021 Folge — Log-Korrelations-Gap: vehicle-mock hat keinen Operator-Session-Kontext |
| Vehicle-Dropdown Live-State-Badge | offen | ADR-026 Folge (MV-09) — bewusst ausgeklammert (Grill-Me 2026-06-14) |
| ~~Multi-Vehicle Handover~~ | ✅ bereits erledigt | ADR-026 Folge (MV-11) — bei Sprint-30-Bearbeitung als bereits behoben vorgefunden (Commit `f68346a`, 2026-07-15): `HandoverManager` löst pro Fahrzeug über `vehiclecontext.Registry` auf; Regressionstest `TestSafety_Handover_TwoVehicles_IndependentHandovers` |
| Produktiv-Video-Empfang (WHEP) auf AWS nach WEBRTC-10-Fix mit echtem Chromium verifizieren | offen | ADR-014/020 Folge — Sprint 32 hat den `actpass → active`-SDP-Zwang entfernt und lokal gegen `mediamtx:latest` (Pion-WHIP-Client, kein echter Browser) verifiziert (`docs/webrtc.md`), Produktivverifikation mit echtem Chromium auf AWS steht noch aus |
| Konkrete ROS2/DDS-Fahrzeugschnittstelle (IBATOUR) | offen | ADR-027 Folge — Workshop mit Professur Logistik (Teil von AP1) noch ausstehend; `FleetGateway`-Interface läuft bis dahin gegen Mock/Simulation. Bezug zu ADR-002 (Safety Bus bereits DDS-interface-kompatibel designed) prüfen, sobald die reale Schnittstelle bekannt ist |
| Handshake-basierte Autonomie-Rückgabe (statt einfachem Session-Ende) | offen | ADR-028 Folge — bewusst zurückgestellt, `endSession()` reicht für den Anfang, siehe `tasks/backlog.md` |
| ~~Persistenzform für "gefahrene Route" (Historie)~~ | ✅ Sprint 32 | `ADR-033` — einfache `vehicle_position_history`-Tabelle (kein Zeitreihen-DB-Dienst), gedrosselter Schreibpfad, 30-Tage-Retention |
| Task-Status-Übergangstabelle dupliziert (Go Backend + TypeScript Frontend) | offen | ADR-030 Folge — Frontend spiegelt die Zustandsmaschine nur für UI-Button-Sichtbarkeit, Backend bleibt autoritativ |
| Video-Latenzziel (100–300ms) unverifiziert | bewusst offen gelassen | ADR-014 Folge (DRIFT-M20, Sprint 61) — kein Safety-Hartziel (anders als die 100ms-Control-Loop-ACK-Latenz, die real per k6/Go-Benchmark in CI erzwungen wird), daher Doku als "unverifiziert/aspirational" statt eines `getStats()`-basierten Tests: ein Mock von `RTCPeerConnection.getStats()` würde nur die eigene Mock-Lesefunktion beweisen, nicht echte Netzwerklatenz; ein echter E2E-Test bräuchte einen echten MediaMTX-Container + Browser-Automatisierung — unverhältnismäßiger Aufwand für ein nicht-blockierendes QoS-Ziel. Farbcodierungs-/Schwellwertlogik der UI bleibt unit-getestet (`useWebRTC.test.ts`) |
| `session.Manager.PushSFUEvent` liest `*Session`-Pointer nach `RUnlock()` | offen | Gefunden bei Gemini-Zweitmeinung (DRIFT-M24, Sprint 61) — `PushSFUEvent` (manager.go) hält die Session als Pointer unter `RLock`, liest `s.ID`/`s.OperatorID` aber erst nach `RUnlock()`; ein zeitgleicher `UpdateOperator`-Call auf dieselbe Session (gleicher Pointer) kann `s.OperatorID` währenddessen überschreiben — potenzielle Data Race. Vorbestehend, unabhängig vom `OperatorRole`-Typ-Refactor (gleiches Muster existierte bereits mit rohem `string`), von `make test-race` (Sprint 61) nicht getriggert, da aktuelle Tests kein gleichzeitiges `UpdateOperator`+`PushSFUEvent` auf derselben Session ausüben. Bewusst nicht in Sprint 61 mitgefixt (Scope: reiner Typ-Refactor, keine Verhaltensänderung) — Fix: Kopie statt Pointer zurückgeben oder Lesevorgang in die Lock-Sektion verschieben |
| `tests/integration` nicht `-count=2`-fest (hartkodierte Fixture-IDs) | offen | Gefunden bei DRIFT-M22/DRIFT61-07 (Sprint 61) — beim ersten echten `-race -count=2`-Lauf über `./...` (zuvor gab es laut Sprint-Historie nur Ad-hoc-Einzelläufe, nie strukturell verankert, s. `test-race`-Makefile-Target). Mehrere Tests in `tests/integration` (z. B. `TestIntegration_FleetService_ZoneStationTaskCRUD`, Zone-ID `"itg-rest-zone"`) verwenden feste literale IDs statt pro Testlauf eindeutiger IDs — ein zweiter Durchlauf im selben Prozess gegen dieselbe DB kollidiert mit einem Unique-Constraint (HTTP 500 statt 201). Reproduziert identisch auf unverändertem `main` vor Sprint 61 — kein Sprint-61-Regression, sondern eine vorbestehende Lücke, die erst jetzt sichtbar wird, weil `-count=2` zum ersten Mal überhaupt strukturell gelaufen ist. `make test-integration`/`make test-race` laufen deshalb bewusst mit `-count=1` gegen diese Suite (wie schon vor Sprint 61); ein `-count=2`-fester Fix bräuchte pro Test eindeutige IDs (z. B. ULID-Suffix) — eigener Folge-Task, nicht in Sprint 61 (Scope wäre eine breite Änderung an vielen Testdateien, nicht Teil von DRIFT-M22s ursprünglichem Zuschnitt) |
| ~~Vollständige Task-Status-Audit-Historie~~ | ✅ Sprint 31 | `ADR-032` — additive `task_status_history`-Tabelle, `GET /fleet/tasks/{id}/history` |
| Prinzip 6 ("Interface over Implementation") — `safety-service`s Bus-Interface (ADR-002) existiert weiterhin nicht als Go-Code | offen | ADR-031 Bestandsaufnahme (2026-07-16). Die übrigen zwei Teilbefunde sind inzwischen behoben (Go-Coding-Style-Guide-Rollout Phase 2, Rule 4.2/4.3, Sprint 34/35, `tasks/backlog.md` EPIC "Go Coding Style Guide Rollout"): `SessionRecorder` (ADR-005) war tote Abstraktion und wurde entfernt (GOSTYLE-IF-01); `fleetgateway` (ADR-027) wird jetzt tatsächlich interface-typisiert konsumiert (GOSTYLE-IF-02); `AuditWriter`/`UserStore`/`VehicleStore`/`safety.Publisher` wurden ebenfalls konsumentenseitig verschlankt (GOSTYLE-IF-03/04/05/06) |
| Partial Telemetry Loss (DEGRADED-Trigger) ohne Produktivpfad | offen | ADR-009 Update (2026-07-16, DRIFT-K3-TELEMETRY) — bewusst zurückgestellt: `telemetry-service` ist ein eigener Prozess ohne bestehende Kopplung zu `control-server`, bräuchte einen neuen Cross-Service-Watchdog (eigene HTTP-Abhängigkeit, eigene Failure-Handling-Entscheidung) statt nur bestehende Endpunkte wiederzuverwenden — siehe `tasks/backlog.md` |

---

## Architekturprinzipien

1. **Safety First** — SAFE MODE ist nicht überbrückbar; Media darf Safety nie triggern
2. **Explicit Failure Model** — jeder Fehlerfall klassifiziert und deterministisch behandelt (ADR-009)
3. **Schema-First** — Protobuf vor Implementierung, kein implizites Nachrichtenformat (ADR-008)
4. **Control-plane-driven** — Control Server = Brain; SFU = Reflex-System; Vehicle = Execution Layer
5. **Hub-Hierarchie** — CONTROL HUB > VIDEO HUB; Video nie in Safety-Entscheidungen (ADR-007)
6. **Interface over Implementation** — Safety Bus, Session Recording, SFU gegen Interfaces (ADR-002/005/015)
7. **No Auto-Resume** — nach CRITICAL Failure immer Operator-Ack (ADR-009/010)
8. **Global Session Authority** — Control Server ist einziger Session-Erzeuger und Single Source of Truth (ADR-015)
9. **Correlation-First** — alle Kanäle tragen Session-ID (ULID); vollständige Rekonstruierbarkeit (ADR-016)
10. **Hybrid Logging** — Technical Logs async (slog → Loki); Safety Events sync (AuditWriter.WriteSync + fsync vor SAFE_MODE); drei Log-Klassen (ADR-017/018)
11. **Safety-Log-Garantie** — Safety Events dürfen niemals verloren gehen; separater PostgreSQL Audit Store (`avoc`-DB) orthogonal zu Loki (ADR-018, Storage seit ADR-023 PostgreSQL statt SQLite)
12. **Vehicle-Feedback-First** — InputIndicatorPanel zeigt was beim Fahrzeug ankommt, nicht was der Operator eingibt; Ist-Werte > Soll-Werte für Safety-Awareness (ADR-021)
13. **Per-Vehicle Safety Isolation** — State Machine und Deadman-/ACK-Watchdogs existieren pro Fahrzeug, nicht global; ein Fahrzeug in SAFE_MODE darf kein anderes Fahrzeug blockieren (ADR-026)
14. **Control-first Kanaltrennung** — Control (WS/Protobuf, ADR-004/010) und Video (WHIP/WHEP, ADR-014/020) laufen über vollständig getrennte Transportkanäle statt einer gemeinsamen WebRTC-PeerConnection mit DataChannel+Media-Track. Verhindert, dass Steuerbefehle hinter Video-Traffic anstehen — Standard-WebRTC priorisiert bei Bandbreitenengpässen Audio > Video > Daten, für Teleoperation genau falsch herum (ein verlorener Steuerbefehl ist kritisch, ein eingefrorenes Videobild ist überlebbar → DEGRADED statt SAFE_MODE, Prinzip 1). Deckt sich mit dokumentierter Branchenpraxis ("Control-first architecture", Recherche 2026-07-10 — RoboticsTomorrow, Adamo, AWS Teleoperation-Referenzarchitektur)
