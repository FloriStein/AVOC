# Sprint 16 — Safety Hardening (ADR-009 Lücken geschlossen)

Ziel: Zwei fehlende ADR-009-CRITICAL-Trigger als Watchdog implementieren: VehicleACKWatchdog (Fahrzeug antwortet nicht auf Steuerbefehle) und SafetyBusWatchdog (Safety-Service nicht erreichbar). Außerdem drei Bugfixes im Session-Lifecycle: WS-Disconnect-Race, fehlende CONNECTED→IDLE-Transition, Page-Reload-Recovery.

Datum: 2026-06-14 | Abgeschlossen: 2026-06-14 | **Status: Alle Tasks ✅ · Edge-Case-Tests dokumentiert**
Vorgänger: Sprint 15 ✅

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| SAF-01 | VehicleACKWatchdog — 1s Timeout nach ForwardCommand ohne ACK → SAFE_MODE (ADR-009) | M | ✅ |
| SAF-02 | SafetyBusWatchdog — 2×5s polling /health → SAFE_MODE bei 2 Failures (ADR-009) | M | ✅ |
| BUG-01 | WS-Disconnect-Race: saubes session/end löst kein SAFE_MODE mehr aus | S | ✅ |
| BUG-02 | CONNECTED→IDLE fehlte in validSystemTransitions | S | ✅ |
| BUG-03 | Page-Reload-Recovery: GET /state gibt vehicle_id+role zurück; App restoriert session | S | ✅ |
| TEST-01 | 18 Unit-Tests für beide Watchdogs (alle Edge Cases) | M | ✅ |

---

## Scope-Details

### SAF-01 — VehicleACKWatchdog ✅
- `internal/controlserver/safety/detector.go`: `VehicleACKWatchdog` Struct
  - `Start(sessionID, vehicleID)` — setzt stopped=false, speichert Session-Kontext
  - `Stop()` — stopped=true + cancelt pending Timer (via `sync.Mutex` + `*time.Timer`)
  - `CommandForwarded()` — startet/resettet `AfterFunc(1s, fire)` wenn nicht stopped
  - `ACKReceived()` — cancelt pending Timer
  - `fire()` — prüft stopped-Flag, schreibt Audit, transitioniert → SAFE_MODE, publiziert `EventVehicleACKTimeout`
- `internal/controlserver/command/engine.go`: nach erfolgreichem `ForwardCommand()` → `vehicleACKWatchdog.CommandForwarded()`
- `internal/vehicleconnection/handler.go`: nach `ackStore.Store()` → `vehicleACKWatchdog.ACKReceived()`
- `cmd/control-server/main.go`: erstellt + verdrahtet Watchdog; Start bei `session/start`, Stop bei beiden `session/end`-Pfaden
- `DefaultVehicleACKTimeout = 1 * time.Second`

### SAF-02 — SafetyBusWatchdog ✅
- `internal/controlserver/safety/bus_watchdog.go`: eigenständige Datei
  - `Start()` — cancelt vorherigen Context, startet neue Goroutine via `context.WithCancel`
  - `Stop()` — ruft `cancel()` auf, Goroutine endet beim nächsten `<-ctx.Done()`
  - `run(ctx, ...)` — Ticker-Loop: Failure-Counter; bei `>= threshold` → `triggerSafeMode()`
  - `ping()` — `GET healthURL` mit 3s Timeout; nur HTTP 200 = ok
  - `triggerSafeMode()` — IdempotenzCheck (`sys == StateSafeMode`), Transition, Publish `EventSafetyBusDown`
- `DefaultBusCheckInterval = 5 * time.Second`, `DefaultBusFailThreshold = 2` (= 10s total)

### BUG-01 — WS-Disconnect-Race ✅
- `internal/controlserver/transport/websocket.go`: readLoop defer prüft jetzt `sessionStillActive`
- War: jeder WS-Disconnect → SAFE_MODE; Neu: nur wenn Session noch aktiv (nicht nach sauberem `session/end`)

### BUG-02 — CONNECTED→IDLE fehlte ✅
- `internal/controlserver/statemachine/state.go`: `StateConnected → {StateDegraded, StateSafeMode, StateIdle}`
- War: `session/end` konnte State nicht von CONNECTED auf IDLE setzen → Session-Lifecycle broken bei mehreren Zyklen

### BUG-03 — Page-Reload-Recovery ✅
- `cmd/control-server/main.go`: `GET /state` gibt jetzt `vehicle_id` und `role` zurück (aus aktiver Session)
- `frontend/src/hooks/useSession.ts`: `restoreFromServerState(sessionId, vehicleId, role)` setzt Refs + State
- `frontend/src/App.tsx`: useEffect erkennt "orphaned SAFE_MODE" (Server hat Session, Local-State verloren) → stellt wieder her

### TEST-01 — Edge-Case-Tests ✅
Siehe **Edge-Case-Dokumentation** unten.

---

## Edge-Case-Dokumentation

### VehicleACKWatchdog (`tests/unit/watchdog_test.go`) — 9 Tests

| Test | Erwartetes Verhalten |
|------|---------------------|
| `TestVehicleACK_TimeoutFires_SafeMode` | CommandForwarded ohne ACK → SAFE_MODE nach Timeout |
| `TestVehicleACK_ACKPrevents_SafeMode` | CommandForwarded + ACKReceived → kein SAFE_MODE |
| `TestVehicleACK_SlidingWindow` | Zweites CommandForwarded resettet Timer |
| `TestVehicleACK_StopCancelsTimer` | Stop() verhindert Transition auch wenn Timer läuft |
| `TestVehicleACK_NoOpAfterStop` | CommandForwarded nach Stop() startet keinen Timer |
| `TestVehicleACK_NoDuplicateTransition` | Bereits SAFE_MODE → kein zweiter Übergang |
| `TestVehicleACK_FreshSessionAfterStop` | Start() nach Stop() funktioniert sauber |
| `TestVehicleACK_ConcurrentRaceSafety` | 10 Goroutinen parallel → kein Data Race |
| `TestVehicleACK_SpuriousACKSafe` | ACKReceived ohne pending Timer → kein Crash |

### SafetyBusWatchdog (`tests/unit/watchdog_test.go`) — 9 Tests

| Test | Erwartetes Verhalten |
|------|---------------------|
| `TestSafetyBus_ThresholdTriggers_SafeMode` | 2 Failures → SAFE_MODE |
| `TestSafetyBus_PartialFailure_NoTrigger` | 1 Failure + Recovery → kein SAFE_MODE |
| `TestSafetyBus_CounterResets_OnSuccess` | Success resettet Failure-Counter |
| `TestSafetyBus_StopCancels` | Stop() beendet Polling-Goroutine |
| `TestSafetyBus_NoDuplicate_WhenAlreadySafeMode` | Bereits SAFE_MODE → kein zweiter Übergang |
| `TestSafetyBus_503_CountsAsFailure` | HTTP 503 zählt als Failure |
| `TestSafetyBus_ConnectionRefused_CountsAsFailure` | Connection Refused zählt als Failure |
| `TestSafetyBus_StartAfterStop_FreshStart` | Start() nach Stop() startet sauber neu |
| `TestSafetyBus_AlwaysHealthy_NeverTriggers` | 200-Responses → kein SAFE_MODE |

**Testergebnis:** `ok avoc/tests/unit 3.482s` — alle 18 Tests grün.

---

## Neue/geänderte Dateien

- `internal/controlserver/safety/detector.go` — `VehicleACKWatchdog` + `DefaultVehicleACKTimeout`; `EventVehicleACKTimeout`
- `internal/controlserver/safety/bus_watchdog.go` — NEU: `SafetyBusWatchdog` vollständig
- `internal/controlserver/command/engine.go` — `vehicleACKWatchdog`-Feld + `WithVehicleACKWatchdog()` + Aufruf nach ForwardCommand
- `internal/vehicleconnection/handler.go` — `vehicleACKWatchdog`-Feld + `WithVehicleACKWatchdog()` + Aufruf nach ACK-Store
- `internal/controlserver/statemachine/state.go` — `StateConnected`/`StateDegraded` → `StateIdle` als valide Transition
- `internal/controlserver/transport/websocket.go` — `sessionStillActive`-Check in readLoop defer
- `cmd/control-server/main.go` — beide Watchdogs erstellt + verdrahtet; `GET /state` gibt `vehicle_id`+`role`
- `internal/safetyservice/bus.go` — `EventVehicleACKTimeout` + `EventSafetyBusDown`
- `pkg/logger/event_types.go` — `EventVehicleACKTimeout` + `EventSafetyBusDown`
- `frontend/src/lib/api-client.ts` — `SystemStateResponse` um `vehicle_id?` + `role?` erweitert
- `frontend/src/hooks/useSession.ts` — `restoreFromServerState(sessionId, vehicleId, role)` hinzugefügt
- `frontend/src/App.tsx` — useEffect für orphaned-SAFE_MODE-Recovery nach Page-Reload
- `tests/unit/watchdog_test.go` — NEU: 9 VehicleACK + 9 SafetyBus Tests
- `docs/adr/009-failure-model.md` — Watchdog-Implementierungen dokumentiert; alle 9 CRITICAL-Trigger

## Commit

`844a6ef feat: VehicleACKWatchdog + SafetyBusWatchdog (ADR-009 Lücken geschlossen)`

---

**Redaktionshinweis (MD-Konsolidierung, 2026-07-19):** Diese Datei führt zwei zuvor parallel
geführte Versionen dieses Sprints zusammen (`tasks/current-sprint.md`: Scope-Details +
Edge-Case-Dokumentation; `tasks/done.md`: Datei-Liste + Commit-Hash) — beide Versionen waren
inhaltlich komplementär, nicht redundant.
