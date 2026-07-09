# ADR-009: Failure Model & System Behavior under Faults

Status: Accepted (erweitert durch ADR-014; Lücken geschlossen Sprint 16)

## Kontext

Ein Teleoperation-System muss bei Netzwerk- und Service-Fehlern deterministisch reagieren. Mit der Einführung von WebRTC (ADR-014) und dem 4-Layer State Machine Modell (ADR-011) wird das Failure Model auf alle vier Kanäle präzisiert: Control, Safety, Video (Media) und Auth.

Sprint 16 hat zwei offene Lücken des ursprünglichen Failure Models implementiert: **VehicleACKWatchdog** (Fahrzeug antwortet nicht auf Steuerbefehle) und **SafetyBusWatchdog** (Safety Service nicht erreichbar). Beide waren als CRITICAL klassifiziert, aber noch nicht als Watchdog implementiert.

## Failure Classification (vollständig)

### CRITICAL — Sofortiger Auto-Stop → SYSTEM SAFE_MODE

| Fehlerfall | Trigger | Verhalten | Implementierung |
|---|---|---|---|
| Operator WebSocket Disconnect | WS_DISCONNECT | Channel Close → SAFE_MODE | `transport/websocket.go` readLoop defer |
| Fahrzeug WebSocket Disconnect | WS_DISCONNECT | Channel Close → SAFE_MODE | `vehicleconnection/handler.go` readLoop defer |
| Safety Event Bus Failure | Safety Bus unreachable | Auto-Stop → SAFE_MODE | **SafetyBusWatchdog** (Sprint 16) |
| Dead-man Switch Timeout | Kein Heartbeat vom Operator | Auto-Stop → SAFE_MODE | `safety/deadman.go` DeadmanWatchdog |
| Vehicle ACK Timeout | Fahrzeug bestätigt Befehl nicht | Auto-Stop → SAFE_MODE | **VehicleACKWatchdog** (Sprint 16) |
| Command ACK Timeout | Control ACK zum Operator überschritten | Auto-Stop → SAFE_MODE | `safety/detector.go` ACKTimeoutWatcher |
| Auth Invalidation | JWT revoked, laufende Session | Auto-Stop → SAFE_MODE | Auth Service (ADR-004) |
| No Active Operator | OPERATOR_STATE = NO_OPERATOR | Auto-Stop → SAFE_MODE | `session/manager.go` |
| Emergency Stop | Operator-Kommando | Sofort → SAFE_MODE | `command/engine.go` |

### DEGRADED — Warnung, Control bleibt möglich

| Fehlerfall | Trigger | Verhalten |
|---|---|---|
| Video Stream Lost | MEDIA_FAILED | SYSTEM → DEGRADED, Warnung im UI |
| Video Qualitätsverlust | MEDIA_DEGRADED | SYSTEM → DEGRADED, Warnung |
| Secondary Camera Failure | Partial MEDIA_FAILED | SYSTEM → DEGRADED |
| Partial Telemetry Loss | MQTT teilweise verloren | SYSTEM → DEGRADED |

### OBSERVATION — Kein Stop, laufende Session bleibt aktiv

| Fehlerfall | Verhalten |
|---|---|
| Auth Service nicht erreichbar (laufende Session) | Neue Sessions blockiert, JWT lokale Validierung weiterhin möglich |

---

## Watchdog-Implementierungen (Sprint 16)

### VehicleACKWatchdog (`internal/controlserver/safety/detector.go`)

Überwacht ob das Fahrzeug auf weitergeleitete Steuerbefehle antwortet.

**Lifecycle:**
- `Start(sessionID, vehicleID)` — beim `session/start`; setzt stopped=false
- `Stop()` — beim `session/end` (beide Code-Pfade); setzt stopped=true + cancelt pending Timer

**Trigger-Logik:**
- `CommandForwarded()` — startet/resettet sliding-window AfterFunc-Timer (Default: 1s)
- `ACKReceived()` — cancelt den pending Timer
- Wenn Timer feuert ohne ACK: prüft `stopped`-Flag → SAFE_MODE + `EventVehicleACKTimeout`

**Wichtige Invarianten:**
- `CommandForwarded()` wird nur bei Typ STEER/THROTTLE/BRAKE/SPEED nach erfolgreichem `ForwardCommand()` aufgerufen (kein Timer für DEADMAN_HOLD etc.)
- Nach `Stop()` startet `CommandForwarded()` keinen neuen Timer mehr (stopped-Flag-Check)
- Parallel-sichere Implementierung via `sync.Mutex`

**Timeout-Wert:** `DefaultVehicleACKTimeout = 1 * time.Second`

---

### SafetyBusWatchdog (`internal/controlserver/safety/bus_watchdog.go`)

Pollt periodisch `GET /health` des Safety-Service. Zwei aufeinanderfolgende Failures → SAFE_MODE.

**Lifecycle:**
- `Start(sessionID, vehicleID)` — beim `session/start`; cancelt vorherigen Context + startet neue Polling-Goroutine via `context.WithCancel`
- `Stop()` — beim `session/end`; ruft `cancel()` auf → Goroutine endet beim nächsten Tick

**Trigger-Logik:**
- Ticker-Loop mit `DefaultBusCheckInterval = 5 * time.Second`
- Failure-Counter: jedes non-200 oder Connection-Error inkrementiert, HTTP 200 resettet
- Bei `failures >= DefaultBusFailThreshold (= 2)` → SAFE_MODE + `EventSafetyBusDown`
- Recovery: Success resettet Counter → Bus-Ausfall unter 10s triggert kein SAFE_MODE

**Health-Check:** `GET <safetyURL>/health` — 200 = ok, alles andere = failure (inkl. Timeout, Connection Refused, 503)

**Wichtige Invarianten:**
- Goroutine läuft IMMER über Context, nie als unbegrenzter Background-Task
- `Start()` nach `Stop()` cancelt vorherigen Context bevor neuer gestartet wird (Leak-Prevention)
- `triggerSafeMode()` prüft erst `sys == StateSafeMode` — kein doppelter Transition-Versuch

---

## Valider SAFE_MODE Zustand

Ein valider SAFE_MODE ist erreicht, wenn:

- Fahrzeugbewegung sofort auf 0 gesetzt
- Control WebSocket geschlossen (Channel Close — ADR-010)
- CONTROL STATE = CONTROL_BLOCKED
- Keine Commands werden akzeptiert oder weitergeleitet
- SYSTEM STATE = SAFE_MODE
- MEDIA STATE läuft weiter (optional — Video bleibt für Monitoring aktiv)
- Operator UI = read-only

## Recovery Sequence

Nach einem CRITICAL Failure:

```
1. Auto-Stop → SAFE_MODE
2. Channel Close (WebSocket geschlossen)
3. Reconnect aller kritischen Services (automatisch, Exponential Backoff)
   - Control WebSocket
   - Safety Event Bus
4. Validierung des aktuellen Systemzustands
5. Operator erhält Resume-Aufforderung (HANDOVER_PENDING oder direktes Ack)
6. Operator bestätigt explizit (Operator ACK)
7. System → RECOVERING → AUTHENTICATED → CONNECTED
8. CONTROL STATE = CONTROL_ACTIVE
```

**Kein automatisches Resume nach Reconnect — immer Operator-Ack.**

---

## Formale System-Invarianten

```
INVARIANT 1:
  Media Layer SHALL NOT influence SAFE_MODE transitions
  except via DEGRADED annotation evaluated by the Control Hub.

INVARIANT 2:
  SAFE_MODE transitions are exclusively triggered by:
  - Control Channel failures
  - Safety Bus failures
  - Operator-level failures (Dead-man, ACK Timeout, No Operator)
  Never by: Video, Telemetry, or Media Layer events.

INVARIANT 3:
  Control Hub is the Single Source of Truth for Session State.
  Video Hub (SFU) derives session context from Control Hub.
  Conflicting states resolve in favor of Control Hub.
```

## Safety Rules

**Video darf niemals SAFE_MODE triggern.** MEDIA_FAILED → DEGRADED, nicht SAFE_MODE.

**Control ist der einzige sicherheitskritische Kanal.** Control + Safety Bus = System Safety.

---

## Konsequenzen

### Positiv:
- Vollständige Failure-Klassifizierung über alle 4 Kanäle
- Video-Failure explizit als nicht-kritisch definiert
- Safety Test Suite kann jeden Trigger isoliert testen
- Klare Recovery-Sequenz implementierbar
- Alle 9 CRITICAL-Trigger sind als Watchdog/Handler implementiert (Sprint 16)
- 18 Unit-Tests in `tests/unit/watchdog_test.go` decken alle Edge Cases ab

### Negativ:
- Command ACK Timeout als CRITICAL erfordert präzises Timeout-Management im Control Server
- SAFE_MODE lässt Media weiter laufen — erfordert bewusste Implementierungsentscheidung
- SafetyBusWatchdog: 10s Reaktionszeit (2 × 5s Intervall) ist ein bewusster Trade-off gegen False Positives bei kurzen Netzwerk-Flakiness
