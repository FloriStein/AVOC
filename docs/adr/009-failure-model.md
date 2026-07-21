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
| Dead-man Switch Timeout | Kein Heartbeat vom Operator | Auto-Stop → SAFE_MODE | `safety/detector.go` DeadmanWatchdog |
| Vehicle ACK Timeout | Fahrzeug bestätigt Befehl nicht | Auto-Stop → SAFE_MODE | **VehicleACKWatchdog** (Sprint 16) |
| Command ACK Timeout | Control ACK zum Operator überschritten | Auto-Stop → SAFE_MODE | `safety/detector.go` ACKTimeoutWatcher |
| Auth Invalidation | Operator-Account gelöscht/deaktiviert, laufende Session | Auto-Stop → SAFE_MODE | **AuthWatchdog** (2026-07-16, siehe Update unten) |
| No Active Operator | OPERATOR_STATE = NO_OPERATOR | Auto-Stop → SAFE_MODE | `statemachine/state.go` (`TransitionOperator(OpNoOperator)`), produktiv verdrahtet seit 2026-07-16 (siehe Update unten) |
| Emergency Stop | Operator-Kommando | Sofort → SAFE_MODE | `command/engine.go` |

### DEGRADED — Warnung, Control bleibt möglich

| Fehlerfall | Trigger | Verhalten | Implementierung |
|---|---|---|---|
| Video Stream Lost | MEDIA_FAILED (ICE failed/disconnected, WHEP-Fehler) | SYSTEM → DEGRADED, Warnung im UI | `useWebRTC.ts` → `POST /media/event` → `TransitionMedia`, produktiv verdrahtet seit Initial Project State |
| Video Qualitätsverlust | MEDIA_DEGRADED (Paketverlust >5% oder Bitrate <100kbps, 3 Samples) | SYSTEM → DEGRADED, Warnung, automatische Erholung zu CONNECTED | `useWebRTC.ts` `getStats()`-Polling, produktiv verdrahtet seit 2026-07-16 (siehe Update unten) |
| Secondary Camera Failure | Partial MEDIA_FAILED | SYSTEM → DEGRADED | Kein Produktivpfad — nur eine Kamera pro Fahrzeug im aktuellen Scope |
| Partial Telemetry Loss | MQTT teilweise verloren | SYSTEM → DEGRADED | Kein Produktivpfad — zurückgestellt, eigener Backlog-Task (siehe Update unten) |

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
- 20 Unit-Tests in `tests/unit/watchdog_test.go` decken alle Edge Cases ab

### Negativ:
- Command ACK Timeout als CRITICAL erfordert präzises Timeout-Management im Control Server
- SAFE_MODE lässt Media weiter laufen — erfordert bewusste Implementierungsentscheidung
- SafetyBusWatchdog: 10s Reaktionszeit (2 × 5s Intervall) ist ein bewusster Trade-off gegen False Positives bei kurzen Netzwerk-Flakiness

---

## Update (2026-07-17)

Sprint-26-Drift-Audit (`docs/drift-audit-2026-07.md`, AUDIT-02/AUDIT-05) fand drei der oben
tabellierten CRITICAL/DEGRADED-Trigger ohne echten Produktivpfad — nur in
`tests/unit/safety_test.go` synthetisch erzeugt (`EventAuthInvalid`, `TransitionOperator
(OpNoOperator)`, `TransitionMedia(...)`). Für jeden Befund vor der Umsetzung eine eigene
Grill-Me-Session (§1.1/§5 CLAUDE.MD, Typ L) — Nutzer entschied sich in allen drei Fällen für
Implementierung statt Doku-Korrektur. Diese Änderung überschreibt **nicht** die ursprüngliche
Tabelle oben (§6) — die „Implementierung"-Spalte für die drei betroffenen Zeilen wurde auf den
aktuellen Stand nachgezogen, alle übrigen Inhalte bleiben unverändert stehen.

### DRIFT-K1: Auth Invalidation

**Befund:** Kein Revocation-Mechanismus existierte. Konkret exploitierbar: `DELETE
/auth/users/{id}` (Admin-Endpunkt) konnte den Account eines aktiven Operators mitten in der
Session löschen, ohne jede Wirkung — `websocket.go validateJWT` prüft das JWT nur einmalig beim
Handshake, danach nie wieder. `POST /logout` blockiert zwar Self-Logout während einer aktiven
Session (ADR-025), das deckt aber nicht den Fall ab, dass ein Admin den Account löscht.

**Entscheidung (Grill-Me):** Implementieren, nicht nur Doku korrigieren. Umfang bewusst
minimal gehalten — nur Account-Existenz/`is_active`-Check, kein `token_version`-Mechanismus,
da aktuell einzig `DELETE /auth/users/{id}` als Revocation-Pfad existiert (kein
Soft-Deactivate-Endpunkt). Timing mirrort `SafetyBusWatchdog` (5s Poll × 2 Fails ≈ 10s) — gleiche
Risikoabwägung wie bei einer bereits akzeptierten Watchdog-Klasse, siehe „Negativ" oben.

**Implementierung:**
- Neues Paket `internal/controlserver/authcheck` — liest `users.is_active` direkt aus der
  geteilten `avoc`-Postgres-DB (gleiches Muster wie `internal/vehicleregistry`/`pkg/audit`), statt
  einer neuen HTTP-Abhängigkeit auf auth-service (würde die Watchdog-Verfügbarkeit an einen
  zweiten Netzwerk-Hop koppeln und mit der OBSERVATION-Klasse „Auth Service nicht erreichbar"
  kollidieren).
- Neuer `AuthWatchdog` (`internal/controlserver/safety/auth_watchdog.go`) — per-`VehicleContext`
  (nicht global wie `SafetyBusWatchdog`, da ein widerrufener Operator-Account nur dessen eigenes
  Fahrzeug betreffen darf, ADR-026). Lifecycle (`Start`/`Stop`) an Session-Start/-Ende und
  WS-Reconnect-Recovery gekoppelt (`vehiclecontext.Registry.WithUserChecker`, optional — nil-safe
  für bestehende Tests, die keinen Checker setzen).
- Feuert über `TransitionOperator(OpNoOperator)`, nicht über einen zweiten, parallelen
  `TransitionSystem`-Aufruf — ein widerrufener Account IST aus Sicht der State Machine „kein
  aktiver Operator mehr" (siehe DRIFT-K2). Publiziert `EventAuthInvalid` auf dem Safety Event Bus.

### DRIFT-K2: No Active Operator

**Befund:** `TransitionOperator(OpNoOperator)` wurde produktiv nirgends aufgerufen — der
Sicherheits-Effekt entstand nur zufällig über den WS-Disconnect-Handler, der SYSTEM STATE direkt
transitionierte, ohne den OPERATOR-Layer zu berühren. Der Layer blieb nach jedem Disconnect bei
`ACTIVE_OPERATOR` hängen.

**Entscheidung (Grill-Me):** Implementieren. Der WS-Disconnect-Pfad deckte den heutigen
Ist-Zustand zwar bereits redundant ab — aber ohne echten OPERATOR-Layer-Pfad hätte jeder
*zukünftige* Code-Pfad, der eine Session ohne WS-Close beendet (z. B. DRIFT-K1s neuer
AuthWatchdog), den OPERATOR-Layer erneut umgangen. `TransitionOperator(OpNoOperator)` ist jetzt
der eine kanonische „Operator ist weg"-Trigger, den beide Fälle teilen.

**Implementierung:**
- `internal/controlserver/transport/websocket.go` readLoop-defer ruft nach dem bestehenden
  `TransitionSystem(StateSafeMode)` zusätzlich `TransitionOperator(OpNoOperator)` auf — SYSTEM ist
  zu dem Zeitpunkt bereits SAFE_MODE, der Aufruf korrigiert nur noch das OPERATOR-Feld selbst.
  Publiziert neu `EventNoOperator` auf dem Safety Event Bus (`WSHandler.WithPublisher`, vorher
  fehlte der Publisher am WSHandler komplett).
- **Nebenbefund beim Implementieren:** `TransitionOperator`s interner SAFE_MODE-Zweig setzte
  `m.System` bisher direkt, ohne den `validSystemTransitions`-Guard, den `TransitionSystem`
  erzwingt (`statemachine/state.go`). Funktional bislang folgenlos, da die Guard-Bedingung
  (`System == CONNECTED/DEGRADED`) zufällig mit den einzigen gültigen Zieltransitionen
  übereinstimmt — aber ein Stilbruch, der riskant geworden wäre, sobald der Pfad produktiv
  aufgerufen wird. Behoben durch Extraktion von `transitionSystemLocked` (gemeinsam von
  `TransitionSystem` und `TransitionOperator` genutzt) — jede SAFE_MODE-Transition läuft jetzt
  über denselben validierten, geloggten Pfad.

### DRIFT-K3: DEGRADED-Tier (Media)

**Befund korrigiert gegenüber Audit-Annahme:** Der Audit behauptete, `TransitionMedia(...)` werde
produktiv nie aufgerufen (geprüft wurden nur `internal/webrtcsfu`/`internal/mediamtx`). Tatsächlich
verifiziert: `frontend/src/hooks/useWebRTC.ts` meldet echte ICE-Connection-State-Änderungen via
`reportMediaState()` → `POST /api/media/event` → `TransitionMedia()` — dieser Pfad existierte
bereits und wird vom echten `VideoPanel` genutzt. **MEDIA_FAILED war also bereits produktiv
verdrahtet.** Echte Lücken: `MEDIA_DEGRADED` (Qualitätsverlust) wurde vom Frontend nie emittiert,
und `TransitionMedia` hatte keinen Rückweg von DEGRADED zu CONNECTED (CONTEXT.MD dokumentiert
CONNECTED ⇄ DEGRADED als bidirektional — nur die Eintritts-Richtung war implementiert; einmal
verlorenes Video ließ SYSTEM STATE dauerhaft bei DEGRADED hängen, selbst nach Recovery). Video
Qualitätsverlust und Partial Telemetry Loss (MQTT) hatten tatsächlich keinen Pfad.

**Entscheidung (Grill-Me):** Video-Teil implementieren (Schwellwerte + Recovery), Telemetrie-Teil
bewusst zurückstellen — `telemetry-service` ist ein komplett separater Prozess ohne bestehende
Kopplung zu control-server; das Verdrahten hätte einen neuen Cross-Service-Watchdog erfordert
(neuer HTTP-Client, neue `TELEMETRY_SERVICE_URL`, eigene Failure-Handling-Entscheidung für den
Watchdog selbst) — andere Risikoklasse als die Video-Änderung, die nur bestehende Endpunkte
wiederverwendet. Eigener Backlog-Task `DRIFT-K3-TELEMETRY` (`tasks/backlog.md`).

**Implementierung:**
- `statemachine/state.go` `TransitionMedia`: `MediaConnected` während `StateDegraded` transitioniert
  jetzt zurück zu `StateConnected` (über `transitionSystemLocked`, siehe DRIFT-K2). Der
  MEDIA_FAILED/DEGRADED-Eintrittszweig nutzt jetzt ebenfalls `transitionSystemLocked` statt
  direktem Feld-Zugriff, aus demselben Konsistenzgrund wie bei `TransitionOperator`.
- `frontend/src/hooks/useWebRTC.ts`: `getStats()`-Polling (schon vorhanden für RTT) erweitert um
  Paketverlust-Ratio und Empfangs-Bitrate aus dem `inbound-rtp`-Report. Schwellwerte (>5%
  Paketverlust ODER <100kbps, 3 aufeinanderfolgende 1s-Samples) sind **initiale, nicht
  feldvalidierte Werte** (Grill-Me-Entscheidung: keine reale Flotte zum Kalibrieren verfügbar,
  Formel/Hysterese wichtiger als die exakten Zahlen — vor Produktivbetrieb mit echten
  Netzwerkbedingungen aus Pilot/Demo nachjustieren). Reine Entscheidungslogik als exportierte,
  pure Funktionen ausgelagert (`computeLossRatio`, `computeBitrateBps`, `isDegradedSample`,
  `nextStreak`, `nextMediaStateFromStreak`) — keine `RTCPeerConnection`-Mock-Infrastruktur im
  Repo vorhanden, direkter Unit-Test der Schwellwert-/Hysterese-Logik statt dessen.

### Teststandard (§17)

Neue/erweiterte Tests: `tests/unit/safety_test.go` (Guard-Grenzfälle `TransitionOperator`/
`TransitionMedia`, Recovery-Pfad), `tests/unit/watchdog_test.go` (8 neue `AuthWatchdog`-Tests:
Normalfall, Trigger, Fehlerpfad, Recovery-nach-1-Fehler, Stop/Restart, Nebenläufigkeit
`-race`), `frontend/src/hooks/useWebRTC.test.ts` (22 Tests für die reinen
Schwellwert-/Hysterese-Funktionen), `tests/integration/services_test.go` (3 neue Tests gegen den
echten Docker-Teststack: `TestIntegration_MediaDegraded_TriggersDegrade_ThenRecovers`,
`TestIntegration_WSDisconnect_OperatorLayerReflectsNoOperator`,
`TestIntegration_AuthWatchdog_DeletedAccount_TriggersSafeMode` — letzterer über eine echte
Postgres-`DELETE`-Operation, nicht gemockt). Vollständige Ergebnisliste inkl. bewusst nicht
abgedeckter Fälle: `tasks/current-sprint.md` Sprint-26-Nachtrag.

---

## Update (2026-07-20)

Sprint 47 (`tasks/sprints/47-multi-cause-degraded-fundament.md`) legt das State-Machine-Fundament
für die Telemetrie-Hälfte von DRIFT-K3 (Partial Telemetry Loss, siehe DRIFT-K3-Update oben und
`tasks/backlog.md` `DRIFT-K3-TELEMETRY`, dort seit 2026-07-17 zurückgestellt). Vor der Umsetzung
zwei Grill-Me-Sessions (§1.1/§5 CLAUDE.MD, Typ L — Kernsystem State Machine/Sicherheitsmodell):

### DRIFT-K3-TELEMETRY: Watchdog-Architektur

**Entscheidung:** Poll-basierter Watchdog in `control-server` (analog `SafetyBusWatchdog`/
`AuthWatchdog` oben), nicht ein aktiver Meldemechanismus vom `telemetry-service` aus.
`telemetry-service` bleibt vollständig unwissend über Sessions/Fahrzeuge — kein neuer
Kontrollfluss, keine neue Kopplung in die Gegenrichtung. Begründung: konsistent mit dem
bestehenden Watchdog-Muster dieses ADRs; ein meldender `telemetry-service` müsste Session-/
Vehicle-Kontext kennen, den er heute nicht hat, und würde eine neue Abhängigkeit in einen Service
einführen, der laut ADR-031 gerade erst hexagonal migriert wurde.

**Architekturskizze für den TelemetryWatchdog-Folge-Sprint** (Nummer noch offen — Sprint 48/49
wurden zwischenzeitlich durch die lokale Ansible-VM belegt, siehe `DECISIONS.MD`; damit dieser
Folge-Sprint nicht erneut recherchieren muss):
- Neues Paket `internal/controlserver/telemetrycheck`, `TelemetryWatchdog` analog
  `SafetyBusWatchdog`/`AuthWatchdog` (Lifecycle `Start(sessionID, vehicleID)`/`Stop()`).
  Per-`VehicleContext` wie `AuthWatchdog`, nicht global wie `SafetyBusWatchdog` — ein
  Telemetrieausfall betrifft nur die DEGRADED-Ursache des eigenen Fahrzeugs (ADR-026).
- Pollt `telemetry-service` per HTTP (neue `TELEMETRY_SERVICE_URL`-Env-Var). Genaue Schwellwerte
  (Poll-Interval/Fail-Threshold/"noch nie empfangen"-Handling) bewusst nicht hier vorentschieden —
  eigene Grill-Me-Session zu Beginn des Folge-Sprints, siehe Nicht-Scope unten.
- Feuert über die neuen `enterDegraded(DegradedReasonTelemetry)`/`exitDegraded(...)`-Helper (siehe
  Multi-Cause-Update unten) — nicht über einen zweiten, parallelen `transitionSystemLocked`-Aufruf,
  aus demselben Konsistenzgrund wie DRIFT-K1/K2 oben.

### DRIFT-K3-TELEMETRY: Multi-Cause-DEGRADED

**Befund:** `statemachine.Machine` trackte DEGRADED bislang als einzelnen Zustand ohne Ursache.
`TransitionMedia`s Recovery-Zweig (`MediaConnected` während `StateDegraded` → `StateConnected`,
siehe DRIFT-K3 oben) würde einen zweiten, unabhängigen DEGRADED-Trigger (Telemetrie) fälschlich
mit aufheben, sobald sich nur das Video erholt, obwohl Telemetrie noch gestört ist. Bislang rein
hypothetisch (nur eine DEGRADED-Quelle existierte produktiv), wird mit dem neuen
`TelemetryWatchdog` (eigener Folge-Sprint, Nummer noch offen) real.

**Entscheidung:** Multi-Cause jetzt beheben (Sprint 47), nicht zurückstellen bis der Folge-Sprint
den Watchdog verdrahtet — sonst müsste dieser dieselbe sicherheitskritische State-Machine-Änderung
zusammen mit der Watchdog-Einführung verifizieren, statt beides isoliert zu testen (analog zur
Praxis, DRIFT-Themenblöcke einzeln statt vermischt zu behandeln). Fix: Set aktiver
Degraded-Gründe (`degradedReasons map[DegradedReason]bool` auf `Machine`) statt Einzelzustand,
Rückkehr zu CONNECTED nur wenn das Set danach leer ist. Neue private Helper
`enterDegraded(reason)`/`exitDegraded(reason)` in `statemachine/state.go` (Caller muss `m.mu`
bereits halten, analog `transitionSystemLocked`); `TransitionMedia` auf die Helper umgestellt —
Verhalten für den bestehenden Single-Cause-Fall (nur Media) bit-identisch zu vorher, reine interne
Umleitung. SAFE_MODE-Eintritt (`transitionSystemLocked`s `StateSafeMode`-Zweig) leert zusätzlich
das Set: SAFE_MODE ist ein vollständiger Stopp unabhängig von der Ursache, Invariante 1/2 oben
bleiben unberührt — Media/Telemetrie lösen SAFE_MODE weiterhin nie aus und heben es nie auf. Nach
Recovery aus SAFE_MODE prüft jeder Watchdog/Media-Poll unabhängig neu, ob sein Grund noch
besteht, und tritt bei Bedarf erneut in DEGRADED ein — kein Datenverlust, nur kein Vorgriff auf
einen möglicherweise inzwischen behobenen Zustand.

**Nicht Teil von Sprint 47:** der `TelemetryWatchdog` selbst, `internal/controlserver/telemetrycheck`,
Verdrahtung in `vehiclecontext.Registry`/`cmd/control-server/main.go`, `TELEMETRY_SERVICE_URL`,
Schwellwert-Entscheidungen — alles Teil des TelemetryWatchdog-Folge-Sprints, siehe Architekturskizze
oben.

**Teststandard (§17):** neue `internal/controlserver/statemachine/state_test.go` (bisher kein
eigenes internes Testfile für dieses Paket) — Media-DEGRADED→CONNECTED unverändert
(Regressionsschutz), Multi-Cause-Szenario (zwei Gründe aktiv, Entfernen nur eines Grundes bleibt
DEGRADED, Entfernen des zweiten wechselt zu CONNECTED), SAFE_MODE leert das Set. Bestehende
`tests/unit`-Suite (`statemachine`/`safety`/`session`) bleibt unverändert grün — reines internes
Umrouten, keine Verhaltensänderung für den Single-Cause-Fall. Details:
`tasks/sprints/47-multi-cause-degraded-fundament.md`.

## Update (2026-07-21)

Sprint 50 (`tasks/sprints/50-telemetry-watchdog.md`) implementiert den in Sprint 47 skizzierten
`TelemetryWatchdog` produktiv. Grill-Me-Session (2026-07-21, CLAUDE.MD §1.1/§5) hat die vier zuvor
offen gelassenen Schwellwerte geklärt:

- **Poll-Interval: 2s.** Bewusst abweichend von `SafetyBusWatchdog`/`AuthWatchdog`s 5s-Standard —
  Telemetrie ist die Hauptquelle für das Situationsbewusstsein des Operators, schnellere Erkennung
  gerechtfertigt.
- **Fail-Threshold: 2** aufeinanderfolgende Fehlschläge — gleiches Muster wie die anderen
  Watchdogs, ergibt mit 2s-Intervall ein Budget von **4s** bis DEGRADED.
- **"Noch nie/nicht aktuell empfangen"-Handling:** Vorrecherche ergab, dass
  `GET /telemetry/latest/{vehicleID}` (`telemetry-service`, bestehender Endpoint aus BE-05) den
  letzten je empfangenen Wert **über Session-Grenzen hinweg** cached (`internal/telemetryservice/
  client.go`s `latest map[string]*TelemetryEvent`, keyed nur nach `vehicleID`, nicht nach Session).
  Ein HTTP 200 mit veraltetem `timestamp` von einer früheren Session könnte damit einen echten
  Ausfall in der aktuellen Session verdecken. Entscheidung: sowohl HTTP 404 (nie empfangen) als
  auch HTTP 200 mit `timestamp`-Alter größer als `maxAge` (= Intervall × Threshold = 4s) zählen
  gleichwertig als ein Fehlschlag Richtung Threshold — kein separater, unabhängiger
  Staleness-Schwellwert.
- **Timeout/HTTP-Fehlerverhalten:** 3s `http.Client`-Timeout, identisch zu `SafetyBusWatchdog`.
  Netzwerkfehler/Timeout/unerwarteter Status (≠404) zählen gleich wie ein Freshness-Fehlschlag —
  aus Sicht des Watchdogs sind 404, Stale-200 und Netzwerkfehler alle nur "keine frischen Daten in
  diesem Poll", keine Sonderbehandlung nötig.

**Architekturumsetzung:** `internal/controlserver/telemetrycheck` (neues Paket, wie skizziert) mit
`Checker.HasFreshTelemetry(ctx, vehicleID, maxAge) (bool, error)` (HTTP-Polling gegen
`telemetry-service`) und `TelemetryWatchdog` (Lifecycle `Start(sessionID, vehicleID)`/`Stop()`,
Fehlschlag-Zählung, feuert über die neue exportierte `statemachine.Machine.TransitionTelemetry
(healthy bool)` — analog `TransitionMedia`, nutzt die bestehenden privaten `enterDegraded`/
`exitDegraded`-Helper aus Sprint 47 unverändert). Anders als `AuthWatchdog` **stoppt der Loop nach
einem Trigger nicht** — ein Telemetrieausfall ist reversibel und beendet die Session nicht, im
Gegensatz zu einem revozierten Operator-Account. `TelemetryWatchdog` ist — anders als das optionale
`AuthWatchdog` — immer aktiv (kein Nil-Check nötig), da `TELEMETRY_SERVICE_URL` eine reguläre
konfigurierte URL ist, keine optionale DB-Abhängigkeit.

**SAFE_MODE-Verhalten:** der Watchdog pollt während SAFE_MODE unverändert weiter (Start/Stop ist an
die Session gebunden, nicht an SYSTEM STATE), aber `TransitionTelemetry`s Guards (analog
`TransitionMedia`: enter nur bei `StateConnected`, exit nur bei `StateDegraded`) machen seine
Aufrufe während SAFE_MODE automatisch zu No-Ops — Invariante 1 bleibt gewahrt, kein Sonderfall im
Watchdog-Code nötig. Details, Task-Aufschlüsselung und Teststandard:
`tasks/sprints/50-telemetry-watchdog.md`.
