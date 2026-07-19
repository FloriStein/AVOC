# Teleoperation System Architecture

Stand: 2026-06-16 (aktualisiert nach ADR-001 bis ADR-026, Sprint 17 deployed); Fleet-System-Abschnitt
nachgezogen 2026-07-16 (Sprint 21–23, ADR-027/028/029) — bis dahin unbeschrieben, obwohl seit
Sprint 21 im Betrieb. Fleet-System-Abschnitt erneut nachgezogen 2026-07-19 (Sprint 30–35): Task-
Status-Lifecycle + -Historie (ADR-030/032), Vehicle-Position-Historie (ADR-033), Indoor-
Fahrzeugposition (ADR-034), Hexagonal-Pilot `fleet-service` (ADR-031, siehe "FleetGateway-
Abstraktion" unten).

---

## Overview

Das Teleoperation System besteht aus **zwei orthogonalen Hubs** und **vier Kommunikationskanälen**:

### Hub-Hierarchie (ADR-007)

```
CONTROL HUB (Rang 1 — Safety Truth)       VIDEO HUB (Rang 2 — Awareness only)
Control Server (Go)                        MediaMTX (WHIP/WHEP Router — ADR-020)
  · State Machine (4 Layer)                 · WHIP Ingestion (Fahrzeug-Kamera, 5G)
  · Safety Decision Engine                  · WHEP Distribution (Operator Browser)
  · Session Manager (GSA)                   · Auth-Hook → Control Server (einzige Auth-Instanz)
  · Failure Detection                        · SAFE_MODE-Kick via Management API (:9997)
  · Operator Handover
  · MediaMTX Auth-Hook (POST /internal/media/auth)
  · MediaMTX SAFE_MODE-Kick (KickVehicle)

WebRTC SFU (Pion/Go) — passiv: Session-Event-Subscriber, kein Media-Routing (ADR-020)
```

**Invariante:** `CONTROL HUB > VIDEO HUB`. Video Hub darf System State nie beeinflussen außer via DEGRADED-Annotation.

**Warum getrennte Transportkanäle statt einer gemeinsamen WebRTC-PeerConnection:** Control (WS/Protobuf)
und Video (WHIP/WHEP) laufen bewusst über vollständig getrennte Transporte, nicht über eine
PeerConnection mit DataChannel (Control) + Media-Track (Video). Standard-WebRTC bündelt bei
diesem verbreiteten Muster alles außer Audio/Video in einem DataChannel und priorisiert bei
Bandbreitenengpässen Audio > Video > Daten — für Teleoperation falsch herum, da ein verlorener
Steuerbefehl kritisch, ein eingefrorenes Videobild aber überlebbar ist. Branchenkritiker an
Standard-WebRTC für Robotik (z. B. Adamo, RoboticsTomorrow) empfehlen deshalb explizit eine
"Control-first"-Architektur mit eigenem, hochpriorisiertem Kanal für Steuerbefehle — das deckt
sich mit dem hier gewählten Design (Recherche 2026-07-10, siehe `CONTEXT.MD` Architekturprinzip 14).

### Vier Kommunikationskanäle

1. Control Channel (WebSocket, Protobuf)
2. Video Channel (WHIP/WHEP via MediaMTX + coturn für ICE)
3. Telemetry Channel (MQTT / Mosquitto)
4. Safety Channel (Safety Event Bus, Go In-Memory)

---

## Communication Architecture

### WebSocket Layer (Control Channel)

- WSS secured connection (ADR-004)
- JWT authentication im Handshake (Operator + Vehicle)
- Protobuf-Messages (ADR-008), CorrelationHeader in jeder Message (ADR-016)
- Event-driven Stream, synchroner ACK-based Loop (ADR-010/012b)
- Heartbeat alle 30 Sekunden
- Auto-reconnect mit exponential backoff
- Channel Close bei CRITICAL Safety Event (ADR-010)

### MQTT Layer (Telemetry Channel)

- Vehicle Telemetry + Status Updates
- Eclipse Mosquitto Broker (ADR-003)
- Protobuf-Messages (ADR-008), CorrelationHeader trägt Session-ID (ADR-016)
- Asynchron, fire-and-forget (ADR-012b)

### Safety Event Bus Layer (Safety Channel — ADR-002)

- Go-Service, In-Memory Message Queue
- DDS-Interface-kompatibel (späterer Austausch per ADR)
- Protobuf-Messages (ADR-008), CorrelationHeader trägt Session-ID (ADR-016)
- Asynchron, fire-and-forget (ADR-012b)
- Safety Events: `PublishSafetyEvent`, `EmergencyStop`, `GetSafetyState`

### WHIP/WHEP Layer (Video Channel — ADR-020)

- **MediaMTX** als WHIP/WHEP Router (ersetzt Pion SFU als Media-Layer)
- **WHIP** (WebRTC-HTTP Ingestion Protocol): Fahrzeug-Kamera (Onboard-Client, 5G) → MediaMTX Port 8889
- **WHEP** (WebRTC-HTTP Egress Protocol): Browser → nginx `/whep/` Proxy → MediaMTX Port 8889
- **ICE/NAT Traversal**: STUN + TURN via coturn (TURN_USER/TURN_PASSWORD aus SSM)
- **Auth**: Einziger Mechanismus — `externalAuthenticationURL` → `POST /internal/media/auth` (Control Server)
  - Publish (WHIP): Bearer Token == WHIP_STREAM_KEY → 200 / 401
  - Read (WHEP): JWT + aktive Operator-Session → 200 / 401
- **SAFE_MODE**: Control Server ruft `GET /v3/webrtcsessions/list` → `POST kick/{id}` auf MediaMTX Management API (:9997)
- **vehicleId-Routing**: Pfad-Regex `~^vehicle-.*`; vehicleId wird dynamisch aus der Vehicle Registry bezogen (VehicleSelector, ADR-022) — `vehicle-001` ist auto-geseedet
- **ICE non-trickle**: Browser wartet vollständige ICE-Gathering (`icegatheringstatechange → complete`) vor WHEP-Request

**WebRTC SFU (Pion/Go) — passiver Session-Event-Subscriber (ADR-020):**
- Empfängt SESSION_* Events vom Control Server (ADR-015) — wie bisher
- Kein ausgehender Call zu MediaMTX oder anderen Services
- Kein Media-Routing mehr (von ADR-014 übernommen durch ADR-020)

---

## Session Architecture (ADR-015/016)

### Session-Hierarchie

```
1. Vehicle Runtime Context   (Transport: WebRTC/MQTT/WS/5G)
2. Control Session           (Safety Layer — primäre Einheit)
   └── 1 Vehicle + 1 Active Operator + 1 Control Server
3. Operator Session          (Identity Layer — JWT/Login)
```

### Global Session Authority (GSA)

Der **Control Server** ist einziger Session-Erzeuger, -Verwalter und -Zerstörer:
- Session-ID generieren (ULID, Zeitpunkt: `CONNECTING → CONNECTED`)
- Session State als Single Source of Truth führen
- Recovery Checkpoint bei SAFE_MODE speichern
- Session Events asynchron an SFU pushen

### SFU Session Events (passiv empfangen, ADR-020)

```
SESSION_CREATED       → SFU protokolliert (kein Routing-Effekt)
OPERATOR_ASSIGNED     → SFU protokolliert
OPERATOR_HANDOVER     → SFU protokolliert
SESSION_DEGRADED      → SFU protokolliert
SESSION_SAFE_MODE     → Control Server kickt MediaMTX-Sessions direkt (kein SFU-Umweg)
SESSION_ENDED         → SFU protokolliert
```

MediaMTX übernimmt alle Media-Routing-Verantwortlichkeiten. Der SFU hat keine ausgehenden Calls.

### Session Correlation ID (ADR-016)

```
Format:    ULID (zeitlich sortierbar, URL-safe, distributed-safe)
Erzeuger:  Control Server (Session Manager)
Zeitpunkt: CONNECTING → CONNECTED

Hierarchie:
  Vehicle-ID
    └── Session-ID (ULID — Root Anchor, überlebt SAFE_MODE)
          └── Event-ID (ULID — pro Message/Command/Frame)

CorrelationHeader (in allen .proto Schemas):
  session_id, event_id, vehicle_id, operator_id, timestamp

JWT = Identity (Wer bist du?) ≠ Session-ID (In welchem Kontext?)
```

---

## State Machine Architecture (ADR-011, pro Fahrzeug seit ADR-026)

4-Layer Model — kein monolithischer State, 4 orthogonale Maschinen. **Seit ADR-026 existiert eine Instanz PRO FAHRZEUG** (`vehiclecontext.Registry`, lazy erzeugt, dauerhaft gehalten), nicht mehr ein globaler Prozess-Singleton. `DeadmanWatchdog`, `ACKTimeoutWatcher` und `VehicleACKWatchdog` sind ebenfalls Teil des `VehicleContext` und damit pro Fahrzeug isoliert. Nur `SafetyBusWatchdog` bleibt ein globaler Singleton (eine geteilte Safety-Service-Instanz, ADR-002) — bei Ausfall transitioniert er alle aktiven Fahrzeuge einzeln nach SAFE_MODE.

```
OPERATOR STATE (Human Governance)
  NO_OPERATOR → ASSIGNED → ACTIVE ⇄ HANDOVER_PENDING
  NO_OPERATOR → SYSTEM SAFE_MODE

SYSTEM STATE (Safety Truth — Master)
  IDLE → CONNECTING → AUTHENTICATED → CONNECTED ⇄ DEGRADED
  CONNECTED/DEGRADED → SAFE_MODE → RECOVERING → AUTHENTICATED

CONTROL STATE (Command Flow, abhängig von SYSTEM STATE)
  CONTROL_INIT → CONTROL_ACTIVE → CONTROL_BLOCKED → CONTROL_LOST → CONTROL_RECOVERING
  SAFE_MODE ⇒ CONTROL_BLOCKED

MEDIA STATE (WebRTC, unabhängig — beeinflusst nur DEGRADED)
  MEDIA_INIT → MEDIA_NEGOTIATING → MEDIA_CONNECTED → MEDIA_DEGRADED → MEDIA_FAILED
  MEDIA_FAILED → SYSTEM DEGRADED (niemals SAFE_MODE)
```

---

## Safety Architecture (ADR-009/010/011)

- Dead-man Switch überschreibt alle Inputs → CRITICAL → SAFE_MODE
- Emergency Stop bypasses alle Ebenen → CRITICAL → SAFE_MODE
- Command ACK Timeout → CRITICAL → SAFE_MODE
- Auto-Stop bei Disconnect → Channel Close (ADR-010)
- Session Recording für Auditierbarkeit (abstraktes Interface — ADR-005)

**Formale Invarianten:**
```
INVARIANT 1: Media Layer SHALL NOT influence SAFE_MODE transitions
             except via DEGRADED annotation evaluated by the Control Hub.

INVARIANT 2: SAFE_MODE transitions are exclusively triggered by
             Control, Safety Bus, or Operator-level failures.

INVARIANT 3: Control Hub is Single Source of Truth for Session State.
             Conflicting states resolve in favor of Control Hub.
```

---

## REST API — Authentifizierung (Sprint 14)

Alle schreibenden REST-Endpoints sind durch `requireJWT`-Middleware geschützt (Bearer-Token-Prüfung via `github.com/golang-jwt/jwt/v5`):

**Geschützt (13 Endpoints):** `POST /session/start`, `POST /session/end`, `POST /logout`, `POST /handover/request`, `POST /handover/confirm`, `POST /handover/cancel`, `POST /media/event`, `POST /emergency-stop`, `GET /audit/events`, `GET /recording/`, `POST /vehicles`, `DELETE /vehicles/{id}`, `GET /sessions`

**Bewusst offen:** `GET /vehicles/{id}/state`, `GET /health`, `GET /vehicles`, `GET /ice-config`, `GET /vehicle/ack/latest/{id}`, `POST /log` — Polling ohne Session-Kontext, nicht-sensitive Lesezugriffe, oder Fire-and-forget Logger (muss vor Login feuern können).

**ADR-026:** `GET /vehicles/{id}/state` liefert den 4-Layer-Snapshot eines einzelnen Fahrzeugs — das Frontend nutzt seit Sprint 17 (MV-07) ausschließlich diesen Endpoint für Live-Polling. Page-Reload-Recovery und der Unreachable-Banner nutzen `GET /sessions` (liefert alle aktiven Sessions, kein Fahrzeugbezug nötig). `GET /state` (global, ohne Fahrzeugbezug) bleibt als Compat-Shim erhalten — kein Frontend-Konsument mehr, aber noch von `tests/performance/latency.js` und `tests/integration/services_test.go` genutzt (geplante Entfernung: Backlog MV-12).

---

## Fleet System (ADR-027/028/029, Sprint 21–23)

Zweiter, orthogonaler Service-Cluster neben dem Direct-Teleop-System oben — entstanden aus dem
IBATOUR-Kurswechsel (Fördervertrag-Pivot Direct-Teleop-PoC → Leitstellensoftware). Bewusst als
**eigener Go-Service** (`fleet-service`), nicht als Erweiterung von `control-server`: koexistiert
auf derselben `vehicles`-Tabelle (`vehicle_type`-Spalte, ADR-029), aber eigener DB-Connection-Pool,
eigener Prozess, kein `depends_on` in beide Richtungen (Startreihenfolge ist absichtlich egal —
beide Services legen ihre jeweils benötigten Tabellen bei Bedarf selbst per
`CREATE TABLE IF NOT EXISTS` an).

### Datenmodell (ADR-029)

Neue, per Fremdschlüssel an `vehicles.id` gehängte Tabellen: `vehicle_status`, `zones`,
`stations`, `tasks`, `alerts`. Zonen/Stationen tragen `svg_geometry` (rohes SVG-Markup) und bei
Outdoor-Zonen zusätzlich `geo_bounds` (JSON-String mit `sw`/`ne`-Ecken, geo-referenziert für
Leaflet `svgOverlay`/Marker-Bounds) — Format in ADR-029 verbindlich definiert (Sprint 23, MAP-02).
Fahrzeuge haben für Outdoor `position_lat/lon`, für Indoor `position_zone_id` **und** `position_x/y`
(`vehicle_status.position_x/y`, ergänzt in ADR-034/Sprint 33, analog zu `stations.position_x/y`).
**Befüllung bleibt offen:** kein Schreiber setzt aktuell `position_zone_id`/`position_x/y` — der
Simulator (`fleet_simulator.go`) liefert bisher nur Outdoor-GPS; Backend/API/Frontend-Rendering
(`FleetIndoorMap.tsx`) sind bereits fertig und unabhängig davon verifiziert (manuell gesetzte
Testdaten), siehe `DECISIONS.MD`.

### FleetGateway-Abstraktion (ADR-027)

`internal/fleetgateway`: Interface (`SubscribeVehicleStatus`/`SubscribeVehicleAlerts`/
`DispatchTask`) zwischen `fleet-service` und der tatsächlichen Fahrzeug-Anbindung — Event-Typen
bewusst von den `fleetservice`-Persistenztypen getrennt, damit das Interface stabil bleibt, falls
sich das DB-Schema ändert. Zwei Implementierungen:

- `MockGateway` (`mock.go`) — In-Process-Simulation, für Unit-/Integrationstests.
- `MQTTGateway` (`mqtt.go`) — Produktiv-Pfad: abonniert `fleet/{id}/status`/`fleet/{id}/alert` auf
  dem bestehenden Mosquitto-Broker (Telemetry Channel, siehe oben), parst JSON (nicht Protobuf —
  bewusste Abweichung vom Direct-Teleop-Schema, da Fleet-Fahrzeuge nur MQTT sprechen, nie eine
  WS-Verbindung zum Control Server aufbauen). `EnsureVehicleExists` (`INSERT ... ON CONFLICT DO
  NOTHING`) registriert neue Fleet-Fahrzeuge in `vehicles` selbst, analog zu `control-server`s
  Auto-Register-Verhalten bei WS-Connect.

`vehicle-mock` (`cmd/vehicle-mock/fleet_simulator.go`) publiziert als externer Prozess auf diese
MQTT-Topics — Bewegung zwischen Demo-Stationen, Batterie sinkt/lädt, seltene fahrzeug-initiierte
Alerts, zwei Fahrzeugtypen (`lastenzug`/`lastenrad`) mit unterschiedlichem Verbrauchsprofil.

**Hexagonaler Pilot (ADR-031, abgeschlossen Sprint 33):** `fleet-service`s Persistenzzugriff läuft
zusätzlich über einen `FleetStore`-Repository-Port (19 Methoden, `internal/fleetservice/store.go`)
mit `FakeFleetStore` für Tests — Strangler-Fig-Migration, bewusst nur dieser eine Service als Pilot
(`control-server` explizit ausgeklammert, höchstes Risiko/geringste Testabdeckung). Fortsetzung auf
weitere Services ist ein expliziter Entscheidungspunkt, kein Automatismus (siehe `DECISIONS.MD`).

### fleet-service REST-API + WS-Broadcast

`cmd/fleet-service/main.go`, Handler in `internal/fleetservice/handler.go`. `RequireAuth` prüft
nur JWT-Signatur/Gültigkeit, keine Rollenprüfung (Fleet-Daten sind operator-facing, nicht
rollenspezifisch wie Admin-Aktionen).

| Methode | Pfad | Zweck |
|---|---|---|
| GET | `/health` | Health-Check, ungeschützt |
| GET | `/fleet/vehicles` | Fahrzeugliste inkl. `vehicle_type` |
| GET/POST | `/fleet/zones` | Zonen-CRUD (Read/Create) |
| GET/POST | `/fleet/stations` | Stations-CRUD (Read/Create) |
| GET/POST | `/fleet/tasks` | Task-CRUD; `CreateTask` dispatcht fire-and-forget an die Gateway (Ack-Semantik der echten Fahrzeug-Anbindung noch offen, ADR-027) |
| PATCH | `/fleet/tasks/{id}/status` | Manueller Status-Übergang (`pending→in_progress→completed`/`cancelled`), race-safe atomares UPDATE (ADR-030, Sprint 30) |
| GET | `/fleet/tasks/{id}/history` | Vollständige Task-Status-Historie, chronologisch aufsteigend (ADR-032, Sprint 31) |
| GET | `/fleet/vehicles/{id}/history` | Gefahrene Route der letzten 30 Tage (`vehicle_position_history`, ADR-033, Sprint 32) |
| GET | `/fleet/alerts` | Alert-Liste |
| POST | `/fleet/alerts/{id}/acknowledge` | Alert bestätigen |
| GET | `/fleet/ws` | WS-Upgrade — Live-Broadcast (siehe unten) |

**Live-Updates ohne Polling (ADR-028, FLEET-06):** `fleet-service` betreibt einen eigenen
Broadcast-Hub (`internal/fleetservice/broadcast.go`) — jede über die MQTT-Gateway eingehende
`vehicle_status`/`alert`-Änderung wird an alle verbundenen `/fleet/ws`-Clients gepusht
(Multi-Workstation-fähig, kein Backlog/Replay). Frontend resynct deshalb bei jedem Reconnect nach
dem ersten vollständig per REST-Snapshot (`useFleetOverview.ts`). Alert-Schwellenwertlogik
(Batterie-Warnung u. ä.) läuft server-seitig in `internal/fleetservice/alertengine.go`, getrennt
von fahrzeug-initiierten Alerts, die bereits fertig über die Gateway hereinkommen (FLEET-07).

**Bekannte, bewusst nicht in Sprint 23 gefixte Lücke:** `GET /fleet/zones`/`/fleet/stations`
liefern bei leerer Tabelle JSON `null` statt `[]` (Go `nil`-Slice-Marshalling) — Frontend
normalisiert defensiv (`?? []`), Backend-Fix offen (`DECISIONS.MD`).

### Frontend — Fleet Overview Dashboard (Sprint 22–23)

Zweite Landing-View neben dem Direct-Teleop-Cockpit, `App.tsx` routet dorthin, sobald ein Token
aber keine `sessionId` gesetzt ist (kein Router nötig für zwei Post-Login-Views).

| Komponente/Hook | Zweck | Sprint |
|---|---|---|
| `FleetOverview.tsx` | Container — führt `fleet-service`-Daten mit `control-server`s `activeSessions` clientseitig zusammen (ADR-029: "Frontend führt Services clientseitig zusammen") | 22 |
| `FleetVehicleList.tsx` / `FleetVehicleDetail.tsx` | Fahrzeugliste + Detail-Panel; ADR-028-Gating (Teleoperate-Button nur ohne aktiven Operator) | 22 |
| `FleetAlertsPanel.tsx` | Alert-Liste, Severity-farbig, Acknowledge pro Zeile | 22 |
| `FleetMap.tsx` | Outdoor-Zonen-Karte — `L.svgOverlay` (Zonen-`svg_geometry`, imperativ via `useMap()`, kein react-leaflet-`<SVGOverlay>` da rohes Markup statt JSX), `L.divIcon`-Marker für Stationen (Quadrate) und Fahrzeuge (Kreise, Klick synct mit Detail-Panel). Kein `<TileLayer>` — ADR-029 schließt externe Kartenkacheln/SaaS aus; zeigt zusätzlich gefahrene Route (`useVehiclePositionHistory.ts`) | 23/32 |
| `FleetIndoorMap.tsx` | Indoor-Karte — reines SVG-Koordinatensystem (`position_x/y`, kein Leaflet/`geo_bounds`), Werkshallen-SVG als Rendering-Grundlage | 33 |
| `FleetTaskPanel.tsx` | Task-Liste inkl. manuellem Status-Übergang und Historie-Anzeige (ADR-030/032) | 30/31 |
| `useFleetOverview.ts` | REST-Snapshot + WS-Deltas (`fleet-ws-client.ts`), Resync bei jedem Reconnect nach dem ersten | 22 |
| `useFleetZones.ts` | Einmaliger REST-Fetch für Zonen/Stationen — bewusst kein WS (Broadcast-Hub kennt keine Zonen-/Stations-Events) | 23 |
| `useVehiclePositionHistory.ts` | REST-Fetch für gefahrene Route (`GET /fleet/vehicles/{id}/history`, ADR-033) | 32 |
| `useActiveSessions.ts` | Extrahiert aus `App.tsx`, pollt `GET /api/sessions` | 22 |

**Scope-Grenze Sprint 23 (Grill-Me 2026-07-16):** ursprünglich auf Outdoor-Zonen beschränkt, da
`position_x/y` fehlte. Indoor-Rendering seither nachgeliefert (Sprint 33, ADR-034):
`FleetIndoorMap.tsx` rendert die Werkshallen-SVG mit Fahrzeugen/Stationen als reines
SVG-Koordinatensystem (kein Leaflet/`geo_bounds`, anders als die Outdoor-`FleetMap.tsx`) —
Befüllung von `position_zone_id`/`position_x/y` durch den Simulator bleibt Folge-Task (siehe oben).
Zusätzlich seit Sprint 31–32: `FleetTaskPanel.tsx` (Task-Status-Übergänge inkl. Historie,
ADR-030/032) und `useVehiclePositionHistory.ts` (gefahrene Route, ADR-033) auf `FleetMap.tsx`.

---

## Control Server — Interne Modulstruktur

Ein Service, 5 logische Module:

```
1. Transport Layer      → WebSocket, JWT Verify, Heartbeat, Channel Close
2. Command Engine       → Input Validation, Rate Limiting, Backpressure, Routing
3. State Machine Engine → SYSTEM / CONTROL / MEDIA / OPERATOR STATE
4. Safety Decision Module → CRITICAL/DEGRADED Klassifizierung, Invarianten-Enforcement
5. Session Manager (GSA) → Session-ID (ULID), Operator-Rollen, Handover, SFU Event Push
```

---

## Latency Targets

| Kanal | Ziel | Typ |
|-------|------|-----|
| Control Loop | < 100ms (ACK-Roundtrip) | Hard — CI Build-Fail |
| Video | 100–300ms QoS-Ziel | Soft — kein Safety-Hartziel |
| Safety Events | near-instant | Priority Channel |

---

## Frontend System (ADR-013)

- Framework: React 18+ + TypeScript
- Build Tool: Vite
- Styling: Tailwind CSS
- Component Library: Shadcn/ui
- Protobuf Code-Gen: protoc-gen-es (TypeScript-Klassen, build-time, gitignored)
- Rendering: SPA (Single Page Application)
- Communication:
  - WebSocket (Control, Protobuf, Sync ACK-based — ADR-012b)
  - WebRTC RTCPeerConnection (Video, browser-nativ)
  - MQTT (optional Telemetry Display)

### UI Module

| Komponente | Implementierung | Sprint |
|------------|----------------|--------|
| **Video Panel** | `VideoPanel.tsx` + `useWebRTC.ts` — RTCPeerConnection, WHEP-Protokoll via `/whep/{vehicleId}/whep`, ICE non-trickle Gathering, MEDIA STATE Badge, DEGRADED-Overlay, `onVideoLatency`-Callback (Sprint 14) | Sprint 5/9/14 ✅ |
| **Control Panel** | `ControlPanel.tsx` + `useControls.ts` — Keyboard WASD/Pfeiltasten, Virtual Joystick SVG, Gamepad API, Speed Slider, 20 Hz Protobuf Command Loop | Sprint 5 ✅ |
| **Safety Panel** | `SafetyPanel.tsx` + `useDeadmanSwitch.ts` — Emergency Stop, Dead-man Switch (Spacebar/Button), SAFE MODE Indikator; `token`-Prop (Sprint 14) | Sprint 3/14 ✅ |
| **Connection Status Panel** | `ConnectionPanel.tsx` — SYSTEM STATE, **Dual-Channel-Latenz: Control (WS-ACK-RTT) + Video (WebRTC ICE-RTT)**, Session-ID (ULID), Operator-Rolle, Speed/Battery (Telemetrie), VehicleSelector (Sprint 12) | Sprint 3/5/12/14 ✅ |
| **SAFE MODE Overlay** | `SafeModeOverlay.tsx` — Fullscreen-Block, Operator-Ack-Button für Recovery | Sprint 3 ✅ |
| **Backend-Unreachable-Banner** | `App.tsx` + `useSystemState.ts` — Rotes Banner + ControlPanel-Sperre nach 3 fehlgeschlagenen State-Polls (1,5s); vehicle-scoped seit Sprint 17 (`useSystemState(vehicleId, token)` — pollt `GET /vehicles/{id}/state` mit Fahrzeug, sonst `GET /sessions` nur als Reachability-Probe) | Sprint 14/17 ✅ |
| **Vehicle Selector** | `VehicleSelector.tsx` + `useVehicles.ts` — Dropdown mit Online-Indikator, Session-Start-Button | Sprint 12 ✅ |
| **Input Indicator Panel** | `InputIndicatorPanel.tsx` + `useVehicleAck.ts` — Lenkrad-SVG, ActuationBars, AckBadge | Sprint 11 ✅ |
| **Operator Panel** | Handover-Anfrage, Observer-Liste | nicht implementiert (kein eigener Sprint geplant) |
| **Login Panel** | `LoginPanel.tsx` — Operator-Login gegen `auth-service` | ADR-004/024 ✅ |
| **User Management Panel** | `UserManagementPanel.tsx` — Nutzerverwaltung hinter `RequireAdmin`, `activeOperatorIds[]`-Anzeige | ADR-024 ✅ |

---

## Projekt-Verzeichnisstruktur

```
AutonomousVehicleOperationalControlCenter/
├── proto/                        # .proto Source — Single Source of Truth (ADR-008)
│   ├── common.proto              # CorrelationHeader (shared, ADR-016)
│   ├── control.proto
│   ├── safety.proto
│   ├── telemetry.proto
│   └── session.proto
├── gen/                          # Generated — gitignored, nie committen
│   ├── go/                       # protoc-gen-go output
│   └── ts/                       # protoc-gen-es output
├── cmd/                          # Go Service Entry Points
│   ├── control-server/
│   ├── auth-service/
│   ├── safety-service/
│   ├── telemetry-service/
│   ├── webrtc-sfu/
│   ├── fleet-service/            # Fleet-REST-API + WS-Broadcast (Sprint 21, ADR-027/028/029)
│   └── vehicle-mock/             # Simulierte Fleet-Fahrzeuge — fleet_simulator.go, publiziert via MQTT
├── internal/                     # Go Service-interne Pakete
│   ├── controlserver/
│   │   ├── command/              # Command Engine — Protobuf Parsing, Rate Limiting (BE-04)
│   │   ├── transport/            # WebSocket Layer
│   │   ├── statemachine/         # 4-Layer State Machine (ADR-011)
│   │   ├── safety/               # Safety Decision Module (ADR-009)
│   │   ├── session/              # Session Manager / GSA (ADR-015/016)
│   │   └── vehiclecontext/       # vehiclecontext.Registry — pro Fahrzeug SM + Watchdogs (ADR-026)
│   ├── mediamtx/                 # MediaMTX Management API Client — KickVehicle (ADR-020)
│   ├── recording/                # Session Recording Interface + MemoryRecorder (BE-07)
│   ├── telemetryservice/         # MQTT Telemetry Client — Paho (BE-05)
│   ├── vehicleconnection/        # Vehicle WebSocket Handler (BE-06)
│   ├── webrtcsfu/                # WebRTC SFU Pion/Go — passiver Session-Event-Subscriber (ADR-020)
│   ├── fleetservice/             # Fleet REST-Handler, Store, Broadcast-Hub, Alert-Engine (Sprint 21, ADR-029)
│   └── fleetgateway/             # FleetGateway-Interface + Mock-/MQTT-Implementierung (ADR-027)
├── pkg/                          # Shared Go-Pakete
│   ├── ulid/                     # ULID-Wrapper (ADR-016)
│   ├── logger/                   # Strukturierter slog-Wrapper — JSON, Event-Type-Katalog (ADR-017)
│   ├── audit/                    # SafetyAuditWriter/AuditWriter Interfaces + PostgresAuditWriter (ADR-018/023)
│   ├── db/                       # DB-Open+WaitForReady-Helper, gebündelt aus 3 main.go-Duplikaten (Sprint 27)
│   └── env/                      # env.Require/OptionalOr-Helper, analog (Sprint 27)
├── frontend/                     # React App (ADR-013)
│   └── src/
│       ├── components/           # VideoPanel, ControlPanel, SafetyPanel, ConnectionPanel, SafeModeOverlay, LoginPanel, UserManagementPanel, FleetOverview/FleetMap/FleetIndoorMap/FleetVehicleList/FleetVehicleDetail/FleetAlertsPanel/FleetTaskPanel
│       ├── hooks/                # useControls, useWebRTC, useTelemetry, useSession, useSystemState, useDeadmanSwitch, useFleetOverview, useFleetZones, useActiveSessions, useVehiclePositionHistory
│       └── lib/                  # ws-client (Protobuf ACK), api-client, fleet-ws-client, fleet-map
├── infrastructure/               # Docker & Compose
│   ├── docker/                   # Dockerfiles je Service, nginx.conf
│   ├── compose/                  # docker-compose.yml + docker-compose.prod.yml
│   ├── mediamtx/                 # mediamtx.yml — WHIP/WHEP Router Config (ADR-020)
│   ├── coturn/                   # STUN/TURN Config
│   ├── mosquitto/                # MQTT Broker Config
│   ├── grafana/ loki/ promtail/  # Log-Aggregation & -Visualisierung (ADR-017)
│   └── AWS/                      # CDK Stack (EC2, Security Groups, ADR-019)
├── tests/                        # Test-Suites
│   ├── performance/              # k6 Load Test + latency.js (ADR-006)
│   ├── unit/
│   ├── integration/
│   └── e2e/
├── go.mod                        # module avoc
├── Makefile                      # proto-gen, build, up, test, lint
└── .gitignore                    # gen/ gitignored
```

**Code-Gen:** `make proto-gen` → erzeugt `gen/go/` und `gen/ts/` aus `proto/*.proto`.

---

## Proto Schema Struktur (ADR-008/012/016)

```
proto/
├── common.proto      → CorrelationHeader (shared across all domains — ADR-016)
├── control.proto     → Control Commands + ControlAck
├── telemetry.proto   → Telemetry Events
├── safety.proto      → Safety Events (alle CRITICAL Trigger als SafetyEventType)
└── session.proto     → Session Events (SFU Push) + RecordingEntry
```

WebRTC Signaling (SDP/ICE) ist **bewusst außerhalb** des Protobuf-Schemas.

---

## Container Architecture (Docker Compose)

Alle Komponenten laufen containerisiert. Keine Kubernetes-Abhängigkeit.

### Services

| Service | Technologie | Zweck |
|---------|-------------|-------|
| `frontend` | React/Vite, nginx | SPA serving; nginx routet `/whep/` → MediaMTX |
| `postgres` | PostgreSQL | Gemeinsame DB `avoc`; separate Connection-Pools pro Service (ADR-023) |
| `control-server` | Go | WebSocket, State Machine, GSA, MediaMTX Auth-Hook + SAFE_MODE-Kick |
| `auth-service` | Go | JWT Ausstellung, Operator-Rollen, Handover-Token |
| `safety-service` | Go | Safety Event Bus (In-Memory, DDS-ready) |
| `telemetry-service` | Go | MQTT Bridge / Mosquitto Client |
| `fleet-service` | Go | Fleet-Domäne (Zonen/Stationen/Tasks/Alerts/Live-Status), eigener Postgres-Pool auf `avoc`, REST + `/fleet/ws`-Broadcast, konsumiert `FleetGateway` (MQTT) — Sprint 21, ADR-027/028/029 |
| `vehicle-mock` / `vehicle-mock-2` | Go | Simulierte Fleet-Fahrzeuge (`lastenzug-01`/`lastenrad-01`), publizieren Status/Alerts über MQTT (`ADR-027`); daneben Direct-Teleop-Einzelfahrzeug (`ADR-021`) |
| `mosquitto` | Eclipse Mosquitto | MQTT Broker (Direct-Teleop-Telemetrie + Fleet-Gateway) |
| `webrtc-sfu` | Go / Pion | Passiver Session-Event-Subscriber (ADR-020); kein Media-Routing |
| `mediamtx` | bluenviron/mediamtx | WHIP/WHEP Router; Management API :9997 (ADR-020) |
| `stun-turn` | coturn | STUN/TURN für NAT Traversal; ICE-Credentials für MediaMTX + Browser |
| `loki` | Grafana Loki | Log-Aggregation (Phase 7) |
| `promtail` | Grafana Promtail | Log-Collector (Docker-Label-Discovery → Loki) |
| `grafana` | Grafana | Log-Visualisierung, Session-Dashboards (Phase 7) |

### Design Principles

- Jeder Service in isoliertem Container
- Keine geteilten Runtime-Dependencies
- Kommunikation nur via definierte Protokolle
- Volle lokale Reproduzierbarkeit via `docker-compose up`
- CI benötigt Docker + STUN/TURN (WebRTC E2E Tests non-blocking — ADR-006)

---

## Logging Architecture (ADR-017/018)

### Drei Log-Klassen

| Klasse | Verlust | Pfad |
|--------|---------|------|
| Technical Log | Erlaubt | async → slog → stdout → Docker → Loki |
| Audit Log | Nicht erlaubt | async → slog → stdout → Docker → Loki |
| **Safety Event** | **Niemals** | sync → `AuditWriter.WriteSync()` → PostgreSQL (ADR-023) + async → Loki |

### Hybrid Sync/Async-Pipeline

```
                    ┌───────────────────┐
                    │  Control Server   │
                    └─────────┬─────────┘
                              │
              ┌───────────────┴───────────────┐
              │                               │
              ▼                               ▼

      Technical Logger                 Safety Logger
    (async — slog → stdout)        (sync — AuditWriter.WriteSync)

              │                               │
              ▼                               ▼

      Docker → Promtail          PostgreSQL (pkg/audit)
              │                               │
              ▼                               ▼
           Loki                         Audit Store
              │                   (garantierte Persistenz)
              ▼
           Grafana
```

**Invariante:** SAFE_MODE-Transition erst nach erfolgreichem `AuditWriter.WriteSync()` + fsync.

### Frontend Log-Ingestion

```
Browser → POST /api/log → Control Server → slog (service="frontend") → Loki
```

Keine direkte Loki-Verbindung aus dem Browser — Authentifizierung und Session-Kontext bleiben zentral.

### Pflichtfelder (alle Log-Einträge)

```json
{
  "time": "2026-06-03T19:00:00Z", "level": "INFO", "service": "control-server",
  "session_id": "01JTXY...", "event_id": "01JTXY...",
  "vehicle_id": "vehicle-001", "operator_id": "operator-1",
  "event_type": "SAFE_MODE_ENTERED", "msg": "Dead-man timeout"
}
```
