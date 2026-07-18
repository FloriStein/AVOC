# AVOC Frontend

React 18 + TypeScript + Vite — Leitstellensoftware UI (Fleet-Dashboard + Teleoperation).

Teil des AVOC-Systems. Vollständige Projektdokumentation: [../README.md](../README.md)

---

## Starten

**Im Docker-Stack (empfohlen):**
```bash
# Aus dem Repo-Root:
make up
# → http://localhost:3000
```

**Lokal mit Hot-Reload:**
```bash
# 1. Proto-Dateien generieren (einmalig, oder nach proto/-Änderungen)
make proto-gen-ts   # aus Repo-Root — generiert frontend/src/gen/*.ts

# 2. Dependencies installieren
npm install

# 3. Dev-Server starten
npm run dev
# → http://localhost:5173
# Voraussetzung: docker-compose up läuft für die Backend-Services
```

Vite proxied automatisch:

| Pfad | Ziel |
|------|------|
| `/ws` | ws://localhost:8080 (Control Server WebSocket) |
| `/vehicle/ws` | ws://localhost:8080 (Control Server, Fahrzeug-seitiger WS) |
| `/api/` | http://localhost:8080 (Control Server REST) |
| `/auth/` | http://localhost:8081 (Auth Service) |
| `/telemetry/` | http://localhost:8083 (Telemetry Service) |
| `/sfu/` | http://localhost:8084 (WebRTC SFU, Legacy-Signaling-Pfad) |
| `/fleet/`, `/fleet/ws` | http://localhost:8085 / ws://localhost:8085 (Fleet Service REST + WS) |
| `/whip/`, `/whep/` | http://localhost:8889 (MediaMTX WHIP/WHEP) |

> **Wichtig:** `src/gen/*.ts` ist gitignored. Nach jedem `git clone` muss `make proto-gen-ts` einmal ausgeführt werden, bevor `npm run dev` funktioniert.

---

## Proto-Code generieren

TypeScript-Klassen werden build-time aus `../proto/*.proto` generiert:

```bash
# Via Docker (kein lokales protoc nötig):
make proto-gen-ts    # aus Repo-Root

# Oder lokal (protoc + protoc-gen-es muss installiert sein):
npm run proto-gen
```

Generierte Dateien landen in `src/gen/` (gitignored). Beide Schemas sind im Bundle:
- `control_pb.js` — `ControlCommandSchema`, `ControlAckSchema`, `CommandType`
- `common_pb.js` — `CorrelationHeaderSchema`

---

## Build

```bash
npm run build   # TypeScript-Kompilierung + Vite Build → dist/
npm run lint    # ESLint
npm test        # Vitest
```

---

## Struktur

```
src/
├── App.tsx                        # Drei Top-Level-Views, kein Router: kein Token → LoginPanel;
│                                   # Token, keine aktive Session → FleetOverview (Sprint-22-
│                                   # Landing-View); aktive Session → Teleoperations-UI
├── components/
│   ├── LoginPanel.tsx              # Login-Formular (Sprint 15 — Nutzerverwaltung)
│   ├── UserManagementPanel.tsx     # Admin-UI für Nutzerverwaltung (Sprint 15)
│   ├── FleetOverview.tsx           # Fleet-Dashboard-Landingpage (Sprint 22) — bündelt die
│   │                                # Fleet-Panels unten
│   ├── FleetMap.tsx                # Outdoor-/Indoor-Kartenansicht mit Zonen/Stationen (Sprint 23)
│   ├── FleetVehicleList.tsx        # Fahrzeugliste mit Live-Status (Sprint 22)
│   ├── FleetVehicleDetail.tsx      # Detailansicht einzelnes Fahrzeug (Sprint 22)
│   ├── FleetTaskPanel.tsx          # Task-Erstellung/-Zuweisung/-Statuswechsel (Sprint 24)
│   ├── FleetAlertsPanel.tsx        # Alert-Liste mit Priorität/Ack, Audio-Benachrichtigung
│   │                                # (Sprint 22/25)
│   ├── VehicleSelector.tsx         # Dropdown zur Fahrzeugauswahl für die Teleoperations-UI
│   ├── ConnectionPanel.tsx         # SYSTEM STATE, Latenz, Session-ID, Operator-Rolle, Speed/Battery
│   ├── ControlPanel.tsx            # Virtual Joystick SVG, Speed Slider, Steer/Throttle Bars
│   ├── InputIndicatorPanel.tsx     # Rohe Keyboard/Joystick/Gamepad-Eingaben zur Diagnose
│   ├── SafeModeOverlay.tsx         # Fullscreen-Block bei SAFE_MODE, Resume-Button
│   ├── SafetyPanel.tsx             # Emergency Stop + Dead-man Switch
│   ├── VideoPanel.tsx              # WebRTC Video Element (WHEP), MEDIA STATE Badge, Overlays, Retry
│   └── StreamSenderPanel.tsx       # WHIP-Sender-UI (Fahrzeug-seitiger Test-/Demo-Videoeinspeiser)
├── hooks/
│   ├── useSession.ts               # Login, WS-Connect, Session-Start, Reconnect (Exp. Backoff)
│   ├── useSystemState.ts           # Polling GET /api/vehicles/{id}/state (500ms, ADR-026);
│   │                                # ohne Fahrzeug nur GET /api/sessions als Reachability-Probe
│   ├── useVehicleAck.ts            # Polling Command-ACK-Status je Fahrzeug
│   ├── useVehicles.ts              # GET /api/vehicles — Fahrzeugliste für VehicleSelector
│   ├── useActiveSessions.ts        # Polling GET /api/sessions
│   ├── useTelemetry.ts             # Polling GET /telemetry/latest/{vehicleId} (1000ms)
│   ├── useControls.ts              # 20 Hz Keyboard/Joystick/Gamepad → Protobuf STEER/THROTTLE/BRAKE/SPEED
│   ├── useDeadmanSwitch.ts         # Spacebar/Mousedown → DEADMAN_HOLD Commands (1500ms Interval)
│   ├── useWebRTC.ts                # RTCPeerConnection, WHEP-Signaling (`/whep/{vehicleId}/whep`), MEDIA STATE
│   ├── useWHIPSender.ts            # RTCPeerConnection für StreamSenderPanel, WHIP-Signaling
│   ├── useMediaRecorder.ts         # Lokale Aufzeichnung des Videostreams (Diagnose/Demo)
│   ├── useFleetOverview.ts         # Polling Fleet-Vehicles/-Alerts/-Tasks für FleetOverview
│   ├── useFleetZones.ts            # GET /fleet/zones + /fleet/stations für FleetMap
│   └── useFleetAlertSound.ts       # Audio-Benachrichtigung bei neuen Alerts (Sprint 25), Mute-Toggle
├── lib/
│   ├── api-client.ts               # HTTP-Client (Control-/Auth-/Fleet-Service-Endpunkte)
│   ├── ws-client.ts                # WebSocket-Client, Protobuf ControlAck parsen, Latenz-Messung
│   ├── fleet-ws-client.ts          # WebSocket-Client für Fleet-Service Live-Updates
│   ├── fleet-ws-events.ts          # Event-Typen/-Parsing für den Fleet-WS-Kanal
│   ├── fleet-merge.ts              # Merge-Logik: WS-Deltas in den Fleet-Overview-State einspielen
│   ├── fleet-map.ts                # Koordinaten-/Geometrie-Hilfsfunktionen für FleetMap
│   ├── fleet-alert-sound.ts        # Ton-Generierung/-Throttling für useFleetAlertSound
│   └── logger.ts                   # Strukturiertes Client-Logging
└── gen/                            # Protobuf-generiert — gitignored
    ├── common_pb.js
    ├── control_pb.js
    └── ...
```

---

## Tech-Stack (ADR-013)

| Bereich | Technologie |
|---------|------------|
| Framework | React 18 |
| Sprache | TypeScript |
| Build Tool | Vite |
| Styling | Tailwind CSS v4 |
| Protobuf | @bufbuild/protobuf + @bufbuild/protoc-gen-es |
| ULID | ulidx |
| Tests | Vitest |

---

## Implementierter Funktionsumfang

### Nutzerverwaltung (Sprint 15)
- `LoginPanel` — Login gegen `POST /auth/operator/login`
- `UserManagementPanel` — Admin-UI zum Anlegen/Verwalten von Operator-Konten

### Fleet Dashboard (Sprint 22–25, `ADR-027/028/029/030`)
- **Landing-View nach Login:** `FleetOverview` zeigt alle Fahrzeuge mit Live-Status
  (Batterie, Modus, aktueller Task), statt direkt in eine Einzelfahrzeug-Teleop-Ansicht zu springen
- **Kartenansicht:** `FleetMap` — Outdoor (geo-referenziert) und Indoor (SVG) Zonen/Stationen,
  Live-Fahrzeugpositionen
- **Task Management:** `FleetTaskPanel` — Erstellen, Zuweisen, Statuswechsel (Zustandsmaschine
  serverseitig autoritativ, siehe `internal/fleetservice/store.go`)
- **Alert System:** `FleetAlertsPanel` — Echtzeit-Alerts mit Priorität, Acknowledge, optionaler
  Audio-Benachrichtigung (`useFleetAlertSound`, Sprint 25) mit Mute-Toggle
- **Multi-Vehicle State Isolation (`ADR-026`):** jedes Fahrzeug hat eigenen SYSTEM/CONTROL/MEDIA/
  OPERATOR-State; `VehicleSelector` wechselt zwischen Fahrzeugen innerhalb der Teleop-Ansicht

### Control Panel (Sprint 5 — FE-05)
- **Keyboard:** WASD + Pfeiltasten → STEER/THROTTLE Commands
- **Virtual Joystick:** SVG mit Pointer-Events; X-Achse = STEER, Y-Achse = THROTTLE
- **Gamepad API:** Left Stick → STEER/THROTTLE, Left Trigger (Button 6) → BRAKE
- **Speed Slider:** Multiplikator 10–100 % skaliert alle Achswerte
- **20 Hz Command Loop:** Protobuf `ControlCommand` via WebSocket, 50ms Interval
- **Priorität:** Gamepad > Joystick > Keyboard
- **CONTROL_BLOCKED:** ControlPanel deaktiviert im SAFE_MODE

### Video Panel (Sprint 9/10 — WHIP/WHEP-Migration)
- **RTCPeerConnection:** WHEP-Signaling via `POST /whep/{vehicleId}/whep` gegen MediaMTX
  (abgelöst: früherer SFU-Signaling-Pfad `/sfu/subscribe/`, siehe `docs/webrtc.md`)
- **MEDIA STATE Tracking:** INIT → NEGOTIATING → CONNECTED / FAILED → DEGRADED (Invariante 1)
- **Reporting:** `POST /api/media/event` bei jeder MEDIA STATE-Änderung → Control Server
- **Overlays:** MEDIA_NEGOTIATING (Spinner-Text), MEDIA_FAILED (Warnung + Retry), MEDIA_CONNECTED (Video)
- **Auto-Connect:** startet wenn Session aktiv; trennt bei Disconnect/SAFE_MODE
- **StreamSenderPanel/`useWHIPSender`:** WHIP-Einspeisung für Test-/Demo-Zwecke (Fahrzeug-seitige Kamera simulieren)

### Dashboard Integration (Sprint 5 — FE-07)
- **Telemetrie:** Speed (km/h), Battery (%), Status via MQTT; 1 Hz Polling
- **Operator-Rolle:** Header zeigt ACTIVE_OPERATOR / HANDOVER_PENDING etc.
- **Protobuf ControlAck:** `fromBinary()` parst `success` und `error_msg` aus Server-Response
- **Fallback-Handling:** Server sendet Protobuf binary (nie JSON) bei fehlender Session

### Verhalten bei Systemzuständen

| Zustand | Verhalten |
|---------|-----------|
| CONNECTED | Control Panel aktiv, Video verbindet |
| DEGRADED | Control möglich, Video-Warnung sichtbar |
| SAFE_MODE | ControlPanel deaktiviert, SafeModeOverlay, kein Video |
| RECOVERING | WS reconnect läuft, Operator-Ack erforderlich |
