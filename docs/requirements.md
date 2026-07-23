# Requirements — Teleoperation Control System

Stand: 2026-07-16 (Drift-Audit-Fixes Sprint 26 — State Machine Requirements um Per-Vehicle-Hinweis ADR-026 ergänzt)

---

## Fleet Gateway Requirements (IBATOUR — vorläufig, ADR-027)

> **Status:** Vorläufig. Das Projekt entwickelt sich von einem Direct-Teleoperation-Proof-of-Concept
> hin zu einer Leitstellensoftware für das Förderprojekt IBATOUR (Bundesministerium für Verkehr,
> Auftraggeber-seitig: Professur Logistik). Die hier beschriebenen Anforderungen basieren auf der
> Leistungsbeschreibung der Ausschreibung und einer Architektur-Vorlage des Auftraggebers — die
> konkrete Schnittstelle zum externen Fahrzeug-Backend ist zum Stand 2026-07-10 noch nicht bestätigt.

- Fahrzeugseitige Automatisierung, Hardware-Schnittstellen (ROS/ROS2) und das fahrzeugseitige
  Backend werden extern durch die Professur Logistik bereitgestellt — **nicht** Teil dieses Projekts
- Dieses Projekt liefert die übergeordnete Leitstellen-Infrastruktur mit Schwerpunkt auf der
  clientseitigen Visualisierungsschicht (Web-Dashboard, Admin-Konsole)
- Kommunikation mit dem externen Fahrzeug-Backend läuft über ein abstraktes `FleetGateway`-Interface
  (ADR-027, konsistent mit dem Interface-over-Implementation-Prinzip aus `CONTEXT.MD` Prinzip 6) —
  nicht direkt fest verdrahtet gegen eine angenommene Schnittstelle
- **Angenommene Ziel-Designprinzipien** (unbestätigt, aus Architektur-Vorlage des AG):
  - Event-Driven Communication via ROS 2 DDS (Pub/Sub), ROSbridge + WebSocket als Fallback für ROS1
  - Multi-Protocol: DDS lokal, MQTT/Zenoh für WAN, WebSocket für Web-Clients
  - Time-Series-optimierte Speicherung für hochfrequente Flotten-Telemetrie
  - Security-First: Ende-zu-Ende-Verschlüsselung, Authentifizierung, Zero-Trust-Netzwerkarchitektur
- Bis zur Bestätigung der konkreten Schnittstelle (Workshop mit der Professur Logistik, Teil von AP1)
  läuft die Entwicklung von Web-Dashboard/Admin-Konsole gegen eine Mock-Implementierung des
  `FleetGateway` (analog zum bestehenden `vehicle-mock`-Muster, ADR-021)
- Weicht die tatsächliche Schnittstelle signifikant von der Annahme ab: neues ADR, `ADR-027` nicht
  überschreiben (siehe `CONTEXT.MD` Offene Fragen)

### Fleet-Domänenkonzepte (neu gegenüber bisherigem Direct-Teleop-Modell)

Aus der Leistungsbeschreibung (AP2/AP3) ergeben sich Anforderungen, die im bisherigen
Single-Vehicle-Direct-Teleop-Modell nicht existieren:

- **Flottenübersicht:** Echtzeit-Fahrzeugstatus mehrerer Fahrzeuge gleichzeitig, Batterielevel mit Alert-System
- **Kartenansicht:** Werkshallen-/Betriebsgelände-Visualisierung, Echtzeit-Positionsverfolgung, Routenübersicht
- **Task Management:** Aufgaben erstellen/zuweisen, Prozess-Status-Tracking, Prioritätenmanagement, Task-History
- **Alert System:** Echtzeit-Benachrichtigungen mit Prioritätsklassen, Audio-Alerts, Acknowledgement-Feature
- **Admin-Konsole:** zentrales Nutzer-/Rollenmanagement, Fahrzeugregistrierung/-konfiguration, Zonenzuweisung, Maintenance-Tracking, System-Health-Monitoring (Service-Status, API-Gateway-Health, Logs)

**Verhältnis zum bestehenden Direct-Teleop-System (geklärt, Grill-Me 2026-07-13):** Das
bestehende System (Control Server, Safety Event Bus, Deadman-Switch, WebRTC-Video, 4-Layer State
Machine) bleibt unverändert bestehen und wird aus dem Dashboard heraus verlinkt — ein "Teleoperate"-
Button am Fahrzeug navigiert zur vollständigen bestehenden Teleop-Oberfläche des jeweiligen
Fahrzeugs. Keine neue, parallele oder vereinfachte Steuerungsoberfläche im Dashboard selbst.

**Sicherheitskonzept fürs Betriebsgelände (geklärt):** Die Fahrzeuge verantworten ihre eigene
Sicherheit selbst (fahrzeugseitige Automatisierung/Safety, außerhalb dieses Projekts). Die
Leitstelle ist Monitoring/Dispatch-Ebene, nicht Safety-Enforcement-Instanz für autonome Fahrzeuge —
die bestehende Safety-Architektur (Deadman-Switch, SAFE_MODE etc.) gilt weiterhin ausschließlich
für den Fall, dass ein Operator ein Fahrzeug über die bestehende Teleop-Oberfläche aktiv steuert.

**Grundparadigma (geklärt, Grill-Me 2026-07-13/14, `ADR-028`):** Die Flotte fährt **autonom als
Normalfall** auf dem Betriebsgelände. Teleoperation ist die **Ausnahme**, ausgelöst durch vom
Fahrzeug selbst erkannte Probleme, die einen Eingriff erfordern (nicht kontinuierliche
Fernsteuerung). Details siehe "Notfall-Trigger-Modell" unten.

---

## Web Dashboard Requirements (Priority 1, AP2 — geklärt, Grill-Me 2026-07-13)

> Referenzmodell: zwei vom Auftraggeber bereitgestellte Screenshots (2026-07-11/12), gelten
> vorläufig als Referenzdesign. Die darin gezeigten "Scene 1–5"-Tabs sind reine Demo-Choreografie,
> keine reale App-Navigation — die tatsächliche Navigationsstruktur ist von uns zu entwerfen.
> Die dort gezeigten Fahrzeugtypen ("Tugger Train", "Forklift AGV") sind generisches
> Beispielmaterial des Auftraggebers, **nicht** die echten Zieltypen (siehe Vehicle-Datenmodell unten).

### Kartendarstellung (`ADR-029`)

- **Indoor:** SVG-Karte. Für den Start soll eine Beispiel-SVG-Karte (Werkshalle/Betriebsgelände) selbst erstellt werden — kein vorhandenes Material vom AG
- **Outdoor:** **ebenfalls SVG-Karte**, aber geo-referenziert — die SVG wird als Overlay über echte GPS-Koordinaten gelegt (Leaflet `imageOverlay`/`svgOverlay` mit Bounds), keine externen Kartenkacheln/SaaS-Abhängigkeit nötig
- **Routen-Darstellung:** sowohl geplante Route als auch gefahrene Historie, mit unterschiedlichen visuellen Markierungen (nicht identisch dargestellt) — Historie-Persistenz seit Sprint 32 umgesetzt (`vehicle_position_history`-Tabelle, `ADR-033`)
- **Zonen:** verwaltbares Konzept, nicht hart codiert — deckt sich mit AP3 "Räumliche Zonenzuweisung" (Admin-Konsole verwaltet Zonen)
- **Stationen:** auf der Karte markierte Punkte, Grundlage für das Task-Modell (siehe unten)

### Task-Modell (Erstversion)

- Ein Task = Bewegung eines Fahrzeugs zwischen auf der Karte markierten **Stationen**
- Kein komplexeres Job-Modell (Be-/Entladen, Prozessschritte) in der Erstversion — das ist eine spätere Erweiterung, kein aktuelles Requirement
- **Status-Lifecycle (geklärt, Grill-Me 2026-07-16, `ADR-030`):** ein Task startet immer bei
  `pending` (Erzeugung); solange die reale Fahrzeug-Anbindung (`ADR-027`, AP1-Workshop) noch
  aussteht, liefert nichts automatisch eine Statusrückmeldung — ein Operator kann den Status daher
  manuell über die Dashboard-UI setzen (`in_progress`/`completed`/`cancelled`, Zustandsmaschine in
  `ADR-030`). "Task-Historie" bedeutet für die Erstversion die vollständige Task-Liste über alle
  Status hinweg (inkl. wer die letzte Statusänderung vorgenommen hat), kein separates Audit-Log
  über mehrere Übergänge hinweg

### Notfall-Trigger-Modell (geklärt, Grill-Me 2026-07-13/14, `ADR-028`)

- Alle Fahrzeuge halten eine **dauerhafte WebSocket-Verbindung** zum Control Server, unabhängig
  von einer aktiven Teleop-Session (bereits heute so implementiert — `/vehicle/ws` ist von
  `session/start` entkoppelt, keine Codeänderung nötig)
- Immer mindestens ein Operator im Dienst, während die Flotte autonom fährt (Aufsichtspflicht)
- Fahrzeuge erkennen Probleme, die einen Eingriff erfordern, **selbstständig** und melden sie über
  das Alert-System (Weg: künftige FleetGateway-Schnittstelle, `ADR-027`)
- Operator sieht den Alert im Notification-System und wählt das betroffene Fahrzeug über das
  bestehende Dropdown-Muster (`VehicleSelector`) aus — kein automatisches Queue/Claim-System
- Jeder Operator kontrolliert jeweils genau ein Fahrzeug gleichzeitig; mehrere Operatoren an
  unterschiedlichen Arbeitsplätzen sind möglich (ergänzt `ADR-025`)
- Nach Problemlösung: **`endSession()` reicht vorerst** zur Rückgabe an die Autonomie. Ein
  expliziter Handshake mit dem Fahrzeug ist bewusst zurückgestellt (`tasks/backlog.md`)
- `NO_OPERATOR → SAFE_MODE` (`ADR-009/011`) gilt **nur innerhalb einer bereits aktiven Session**,
  nicht als Dauerzustand für autonom fahrende Fahrzeuge ohne Session (`ADR-028`)

### Alert-System

Zwei Quellen, beide laufen in dieselbe Notification-UI:

- **Leitstellen-generiert:** Schwellenwert-/Regellogik in `fleet-service` (z. B. Batterie-Warnung bei niedrigem Ladezustand) — berechnet aus empfangener Telemetrie
- **Fahrzeug-initiiert:** das Fahrzeug erkennt selbst ein Problem, das einen Operator-Eingriff erfordert, und sendet das explizit als Alert (Notfall-Trigger, siehe oben) — nicht aus Telemetrie-Schwellenwerten abgeleitet, sondern ein eigenständiges Ereignis vom Fahrzeug

### Vehicle-Datenmodell, Service-Grenze & Multi-Vehicle-Simulation (`ADR-029`)

- Neuer Service `fleet-service`; bestehende `vehicles`-Tabelle (`control-server`/`vehicleregistry`)
  bleibt einzige Identitäts-Quelle, erweitert um `vehicle_type` (**Lastenrad**, **Lastenzug** —
  nicht "Forklift AGV"/"Tugger Train" aus dem Referenzdesign, das war Beispielmaterial des AG).
  `fleet-service` besitzt neue, per Fremdschlüssel verknüpfte Tabellen: `vehicle_status`, `zones`,
  `stations`, `tasks`, `alerts` — Details siehe `ADR-029`
- Frontend führt `control-server` (Online-Status) und `fleet-service` (Typ/Batterie/Position/
  Task/Alerts) clientseitig über `vehicle_id` zusammen — keine Backend-zu-Backend-Kopplung
- `vehicle-mock` wird zu einer Multi-Vehicle-Simulation erweitert (mehrere simulierte Fahrzeuge
  gleichzeitig, Position/Batterie/Status), um Backend-Services und Dashboard gegen Bewegtdaten zu
  testen (Priority-1-Notiz: "enables testing of backend services", "multi-vehicle simulation demonstration")
- Das Datenformat dieser Simulation dient vorerst als konkrete Ausprägung des `FleetGateway`-Interfaces (`ADR-027`) — bis zur Bestätigung der echten ROS2-Schnittstelle

### Multi-Operator / Multi-Workstation

- Mehrere Leitstellen-Arbeitsplätze sollen gleichzeitig aktiv sein können (nicht nur ein Operator-Client) — Anforderung zusätzlich zum bereits bestehenden Multi-Operator-Modell pro Fahrzeug (`ADR-025`)
- Jeder Operator kontrolliert jeweils genau ein Fahrzeug gleichzeitig (`ADR-028`)

### Projektstatus

- Ausschreibung ist beauftragt (nicht mehr Angebotsstadium) — Stand 2026-07-13

---

## Vehicle Connection Requirements

- Secure session establishment
- JWT-based authentication für WebSocket-Verbindung (Operator + Vehicle, separater Auth Service — ADR-004)
- Operator-Rollen: Active Operator, Observer, Standby (ADR-011 OPERATOR STATE)
- Connection quality monitoring mit Latenzanzeige (<100ms Ziel für Control Loop)
- Auto-disconnect bei schlechter Verbindungsqualität
- Auto-reconnect mit exponential backoff nach CRITICAL Failure
- 30s Heartbeat (ping/pong)
- Exklusiver Active Operator pro Session (max. 1 gleichzeitig)

---

## Video Streaming Requirements

- Real-time Kamera-Feeds über WebRTC SFU (Pion/Go — ADR-014)
- NAT Traversal via ICE/STUN/TURN (coturn) — Vehicle ↔ Internet ↔ OCC Szenario
- **Kamera-Modell:**
  - 1 Primary Stream (immer aktiv, an alle Operatoren geforwardet)
  - 1–2 Secondary Streams (on-demand, Operator subscribed bei Bedarf)
- Adaptive Bitrate via WebRTC RTCP Feedback (built-in)
- Multi-Operator Video: alle Operatoren (Active + Observer) empfangen denselben Primary Stream
- Server-seitiges Video Recording (primär, Audit-fähig)
- Client-seitiges Recording (optional, via Browser MediaRecorder API)
- Video-Ausfall → DEGRADED State (kein Auto-Stop, kein SAFE MODE — ADR-009/011)
- QoS-Latenzziel Video: 100–300ms (kein Safety-Hartziel — ADR-014) — **unverifiziert/aspirational**
  (DRIFT-M20, Sprint 61): kein automatisierter Latenz-Test misst diesen Wert, nur die
  Farbcodierungs-/Schwellwertlogik der UI-Anzeige ist getestet (`useWebRTC.test.ts`). Bewusste
  Entscheidung, keinen `getStats()`-basierten Test nachzurüsten — siehe Begründung unter
  Performance Requirements unten.
- WebRTC Signaling via bestehenden WebSocket-Kanal (außerhalb Protobuf-Schema — ADR-008/014)

---

## Manual Control Requirements

- Virtual Joystick Interface
- Keyboard Support
- Gamepad Support
- Speed Control Slider
- Emergency Stop Button (CRITICAL — triggert SAFE MODE, Channel Close)
- Dead-man Switch (Mandatory Active Hold — Loslassen = CRITICAL Trigger)
- Control nur im SYSTEM STATE = CONNECTED oder DEGRADED erlaubt
- Control im SAFE MODE vollständig blockiert (CONTROL_BLOCKED)

---

## Safety Requirements

- Operator Acknowledgment vor Steuerungsaufnahme (AUTHENTICATED → CONNECTED)
- Operator Acknowledgment nach Recovery aus SAFE MODE (RECOVERING → CONNECTED)
- Dead-man Switch (mandatory active hold — Timeout = CRITICAL)
- Emergency Stop (sofortige Systemdeaktivierung — bypasses alle Ebenen)
- Auto-Stop bei CRITICAL Failure (Channel Close — ADR-010)
- Command ACK Timeout = CRITICAL Trigger → SAFE MODE (ADR-009)
- No Active Operator = CRITICAL Trigger → SAFE MODE
- Session Recording für Audit (abstraktes Interface — ADR-005)
- SAFE MODE: Fahrzeug steht, keine Commands, Channel geschlossen, UI read-only
- Kein automatisches Resume nach CRITICAL Failure — immer Operator-Ack
- Recovery Checkpoint bei SAFE_MODE-Eintritt (Vehicle-ID, Operator-ID, letzter System/Control State, Safety Reason Code, Session-ID — ADR-015)
- Recovery = Neu-Aktivierung eines validierten Zustands, kein automatisches Wiederherstellen

---

## State Machine Requirements (ADR-011)

Das System implementiert ein 4-Layer State Machine Modell. Seit `ADR-026` läuft SYSTEM/CONTROL/MEDIA
STATE pro Fahrzeug isoliert (`VehicleContextRegistry`) statt einmalig pro Prozess — siehe
„Multi-Vehicle Requirements" unten für die Per-Vehicle-Anforderungen im Detail:

### SYSTEM STATE (Safety Truth — Master)
Zustände: `IDLE → CONNECTING → AUTHENTICATED → CONNECTED ⇄ DEGRADED → SAFE_MODE → RECOVERING`

### CONTROL STATE (Command Flow)
Zustände: `CONTROL_INIT → CONTROL_ACTIVE → CONTROL_BLOCKED → CONTROL_LOST → CONTROL_RECOVERING`
Regel: SAFE_MODE ⇒ CONTROL_BLOCKED. VIDEO_LOSS ⇒ kein CONTROL STATE Change.

### MEDIA STATE (WebRTC/Video Health)
Zustände: `MEDIA_INIT → MEDIA_NEGOTIATING → MEDIA_CONNECTED → MEDIA_DEGRADED → MEDIA_FAILED`
Regel: MEDIA_FAILED → SYSTEM DEGRADED. Niemals SAFE_MODE.

### OPERATOR STATE (Human Governance)
Zustände: `NO_OPERATOR → OPERATOR_ASSIGNED → ACTIVE_OPERATOR ⇄ HANDOVER_PENDING → RECOVERING_OPERATOR`
Regel: NO_OPERATOR → SYSTEM SAFE_MODE. Max. 1 ACTIVE_OPERATOR pro Session.

---

## Session Requirements (ADR-015)

- **Primäre Session:** Control Session = 1 Vehicle + 1 Active Operator + 1 Control Server Instanz
- **Session-Hierarchie:**
  1. Vehicle Runtime Context (Transport: WebRTC/MQTT/WS)
  2. Control Session (Safety Layer — einzige Safety-relevante Einheit)
  3. Operator Session (Identity Layer — JWT/Login)
- Max. 1 aktive Control Session pro Vehicle
- Operator-Wechsel via Handover möglich — Control Session bleibt bestehen
- Vehicle-Reconnect via RECOVERING möglich — Control Session bleibt logisch bestehen
- Control Session endet bei: CRITICAL Failure ohne Recovery, explizitem Session-Ende, Timeout
- **Global Session Authority (GSA):** Control Server ist einziger Session-Erzeuger, -Verwalter und -Zerstörer
- Session State in-memory (ephemeral), Recovery Checkpoint bei SAFE_MODE
- SFU empfängt Session Context via Event-Push vom Control Server (SESSION_CREATED, OPERATOR_HANDOVER, SESSION_SAFE_MODE, SESSION_ENDED)

---

## Multi-Vehicle Requirements (ADR-026)

- Zwei Operatoren müssen zwei unterschiedliche Fahrzeuge gleichzeitig und sicherheitstechnisch unabhängig steuern können
- SAFE_MODE, Dead-man-Timeout und Command-ACK-Timeout eines Fahrzeugs dürfen kein anderes Fahrzeug beeinflussen (Per-Vehicle State Machine + Watchdogs)
- Ausfall geteilter Infrastruktur (Safety Event Bus) ist ein fleet-weites Ereignis — betrifft korrekt alle aktiven Fahrzeuge gleichzeitig, nicht nur eines
- Emergency Stop ohne explizite `vehicle_id` wirkt fleet-weit (Sicherheitsnetz); mit `vehicle_id` nur auf das angegebene Fahrzeug
- Live-State-Abfrage muss pro Fahrzeug möglich sein (`GET /vehicles/{id}/state`)
- Neue Fahrzeuge werden bei ihrem ersten Verbindungsaufbau automatisch registriert — kein manuelles Vorab-Anlegen nötig

---

## Operator Handover Requirements (ADR-011/014/015)

- Multi-Operator-Betrieb: ein Active Operator + beliebig viele Observer/Standby
- Steuerungsübergabe (Handover) zwischen Operatoren unterstützt
- HANDOVER_PENDING State: Übergabe muss von beiden Seiten bestätigt werden
- Während HANDOVER_PENDING: aktueller Operator behält Steuerung
- Observer sehen denselben Video Primary Stream wie Active Operator
- Auth Service verwaltet Operator-Rollen und Handover-Token (ADR-004)

---

## Communication Requirements

- **WebSocket (WSS):** Real-time Control, Protobuf-Messages, Synchron ACK-based (ADR-010/012b)
- **MQTT (Mosquitto):** Telemetry & Status Updates, asynchron (ADR-003/012b)
- **Safety Event Bus (Go, In-Memory):** Safety-critical Events, asynchron, DDS-Interface-kompatibel (ADR-002/012b)
- **WebRTC SFU (Pion/Go):** Video Streaming, NAT Traversal via coturn (ADR-014)
- **Message Format:** Protocol Buffers (Protobuf) für Control/Telemetry/Safety — ADR-008
- **WebRTC Signaling:** SDP/ICE außerhalb Protobuf-Schema (Media Layer — ADR-008/014)
- **Protobuf Versioning:** Field-based, keine Breaking Changes, CI Schema-Gate (ADR-012)
- **Code Generation:** Build-time via protoc, gitignored, protoc-gen-es (TS) + protoc-gen-go (ADR-012b)
- **Session Correlation ID:** ULID, generiert vom Control Server bei `CONNECTING → CONNECTED` (ADR-016)
- **CorrelationHeader** in allen `.proto`-Schemas: `session_id`, `event_id`, `vehicle_id`, `operator_id`, `timestamp`
- **Identifier-Hierarchie:** `Vehicle-ID → Session-ID (ULID) → Event-ID (ULID)` über alle Kanäle
- **JWT = Identity only** — kein Session Context im JWT (ADR-016)
- **Session-ID überlebt SAFE_MODE** als Root-Anchor; Recovery = neue Execution Branch unter gleicher Session-ID

---

## Performance Requirements

- **Control Loop Latenz:** < 100ms (ACK-Roundtrip Client → Control Server, CI Build-Fail bei Verletzung — ADR-010/006)
- **Video Latenz:** QoS-Ziel 100–300ms (kein Safety-Hartziel — ADR-014). **Unverifiziert/aspirational**
  (DRIFT-M20, Sprint 61) — bewusst kein `getStats()`-basierter automatisierter Test: ein Mock des
  `RTCPeerConnection.getStats()`-Rückgabewerts würde nur beweisen, dass unser Code den Mock-Wert
  korrekt liest, nicht dass echte Netzwerklatenz im Zielkorridor liegt; ein echter Test gegen einen
  echten MediaMTX-Container + Browser-Automatisierung wäre der einzige aussagekräftige Ansatz,
  steht aber in keinem Verhältnis zum Nutzen für ein nicht-sicherheitsrelevantes, nicht CI-Build-Fail-
  auslösendes QoS-Ziel (anders als die 100ms-Control-Loop-Latenz oben, die bereits real per
  k6/Go-Benchmark in CI erzwungen wird). Die Farbcodierung/Schwellwertlogik der UI-Anzeige bleibt
  weiterhin unit-getestet (`useWebRTC.test.ts`).
- **Safety Events:** near-instant (Safety Event Bus Priority Channel)
- High reliability connection recovery (Exponential Backoff)
- Graceful degradation bei schlechter Netzwerkqualität (DEGRADED State)
- Rate Limiting und Backpressure im Control Input System (ADR-010)

---

## Frontend Requirements (ADR-013)

- Web Application (Browser-based)
- **Sprache:** TypeScript
- **Framework:** React 18+
- **Build Tool:** Vite
- **Styling:** Tailwind CSS
- **Component Library:** Shadcn/ui
- **Protobuf:** protoc-gen-es (TypeScript-Klassen, build-time generiert)
- Real-time Communication mit Backend via WebSocket (Protobuf, sync ACK)
- WebRTC RTCPeerConnection für Video-Empfang (browser-nativ)
- Modular UI:
  - **Video Panel:** Primary Stream (always on) + Secondary Streams (on-demand), MEDIA STATE Anzeige
  - **Control Panel:** Joystick, Keyboard, Gamepad, Speed Slider
  - **Safety Panel:** Emergency Stop, Dead-man Switch, SAFE MODE Indikator, Operator Ack Flow
  - **Connection Status Panel:** SYSTEM STATE, Latenzanzeige, Operator-Rolle (Active/Observer)
  - **Operator Panel:** Handover-Anfrage, Observer-Liste
- UI blockiert Steuerung bei CONTROL_BLOCKED (SAFE MODE Overlay)
- DEGRADED State sichtbar im UI (Video-Warnung), Control bleibt möglich
- Reconnect-Logik nach Channel Close (ADR-010)

---

## Deployment Requirements (ADR-001/CONTEXT)

- System vollständig containerisiert via Docker
- Docker Compose orchestriert alle Services
- Lokale Entwicklung vollständig via `docker-compose up`
- **Services:**
  - `frontend` — React App (Vite Build, nginx serving)
  - `postgres` — PostgreSQL, gemeinsame DB `avoc` (ADR-023)
  - `control-server` — Go (WebSocket, 4-Layer State Machine, Safety Decision, Session Manager/GSA, ULID Generation)
  - `auth-service` — Go (JWT, Operator-Rollen, Handover-Token)
  - `safety-service` — Go (Safety Event Bus, In-Memory)
  - `telemetry-service` — Go (MQTT Bridge / Mosquitto Client)
  - `webrtc-sfu` — Go/Pion (passiver Session-Event-Subscriber, ADR-020)
  - `mediamtx` — WHIP/WHEP Router (Video-Ingestion/-Distribution, ADR-020)
  - `fleet-service` — Go (Fleet-Domäne: Zonen/Stationen/Tasks/Alerts/Live-Status, ADR-027/028/029/031); Dev-Stack, aktuell nicht in `docker-compose.prod.yml` (siehe `docs/deployment/ec2-bootstrap.md`)
  - `vehicle-mock` — Go (Fahrzeug-/Fleet-Simulator, ADR-021/027)
  - `stun-turn` — coturn (NAT Traversal für Vehicle ↔ Internet ↔ OCC)
  - `mosquitto` — Eclipse Mosquitto (MQTT Broker)
  - `loki` — Grafana Loki (Log-Aggregation — Phase 7)
  - `promtail` — Grafana Promtail (Log-Collector)
  - `grafana` — Grafana (Log-Visualisierung, Session-Dashboards — Phase 7)
- Kein Kubernetes in dieser Phase
- Full local reproducibility via `docker-compose up`
- CI benötigt Docker-Environment (Integration Tests, WebRTC Browser-Flags)

---

## Logging Requirements (ADR-017/018)

### Strukturiertes Logging

- Alle Services (Backend + Frontend) loggen strukturiert als JSON
- Jeder Log-Eintrag enthält: `time`, `level`, `service`, `session_id`, `event_id`, `vehicle_id`, `operator_id`, `event_type`, `msg`
- Kein Freitext-Logging — typisierter `event_type` aus definiertem Katalog
- Frontend sendet Logs via `POST /api/log` an Control Server (niemals direkt an Loki)

### Log-Klassen

| Klasse | Verlust erlaubt | Anforderung |
|--------|-----------------|-------------|
| Technical Log | Ja | async slog → stdout → Loki |
| Audit Log | Nein | async slog → stdout → Loki |
| Safety Event | Niemals | **synchron** via `AuditWriter.WriteSync()` → PostgreSQL + Loki |

### Safety-Log-Garantie

- Safety Events (`EMERGENCY_STOP`, `DEADMAN_TIMEOUT`, `SAFE_MODE_ENTERED`, `SAFE_MODE_EXITED`, `COMMAND_ACK_TIMEOUT`, `SAFETY_BUS_FAILURE`, `OPERATOR_HANDOVER_COMPLETED`, `SESSION_STARTED`, `SESSION_ENDED`) müssen garantiert persistiert werden
- Persistenz erfolgt **vor** dem Abschluss der SAFE_MODE-Transition (fsync)
- Loki-Ausfall darf Safety-Event-Persistenz nicht beeinflussen
- Audit Store: PostgreSQL, gemeinsame DB `avoc` (ADR-018/023)

### Session-Rekonstruktion

- Eine vollständige Teleoperation-Session muss über Frontend-, Backend-, Safety-, Telemetry- und Video-Ereignisse hinweg rekonstruierbar sein
- Korrelation über `session_id` (ULID, ADR-016) als primären Anker
- Grafana-Dashboard: LogQL-Abfragen wie `{session_id="01J..."}` über alle Services
- PostgreSQL-Query: `SELECT * FROM audit_events WHERE session_id=$1 ORDER BY timestamp`

### Latenz-Anforderung

- Technical-Log-Overhead: `<1ms` (async slog, stdout-Puffer)
- Safety-Event-Overhead: `~1–5ms` (fsync) — akzeptabel, da Safety Events selten
- Control-Loop-Budget `<100ms` (ADR-010) darf durch Logging nicht verletzt werden
