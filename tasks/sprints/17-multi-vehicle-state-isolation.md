# Sprint 17 — Multi-Vehicle State Isolation (ADR-026)

Ziel: Drei Safety-kritische Prozess-Singletons (State Machine, DeadmanWatchdog, VehicleACKWatchdog) werden pro Fahrzeug isoliert, damit zwei Operatoren zwei unterschiedliche Fahrzeuge wirklich unabhängig steuern können. SafetyBusWatchdog bleibt global, fächert bei Ausfall aber korrekt auf alle aktiven Fahrzeuge auf. `GET /state` wird durch `GET /vehicles/{id}/state` ersetzt.

Datum: 2026-06-14 | Abgeschlossen: 2026-06-16 | **Status: Backend + Frontend fertig (TDD) ✅ · Deployed 2026-06-16 (Commits `32de463`, `d20e9f2`) ✅**
Vorgänger: Sprint 16 ✅
Voraussetzung: [ADR-026](../../docs/adr/026-multi-vehicle-state-isolation.md) (Grill-Me-Session 2026-06-14 abgeschlossen)

Arbeitsweise: test-driven auf Wunsch des Nutzers — pro Task zuerst Szenarien/Edge-Cases durchdacht und Tests geschrieben (Red), danach implementiert (Green). Beim Implementieren stellte sich heraus, dass MV-03/04/05 nicht unabhängig voneinander gehen (State Machine ist eine gemeinsam genutzte Safety-Ressource — Teilmigration hätte einen Split-Brain zwischen altem globalem `sm` und neuer Registry erzeugt), daher in einem kohärenten Durchgang umgesetzt.

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| MV-01 | `vehiclecontext.Registry` — `VehicleContext` (SM + Deadman + ACKTimeoutWatcher + VehicleACKWatchdog), lazy `Get(vehicleID)`, Mutex-safe | M | ✅ |
| MV-02 | `SafetyBusWatchdog` umbauen — global, iteriert `sessionMgr.ActiveVehicleIDs()` bei Ausfall, SAFE_MODE pro betroffenem Fahrzeug | M | ✅ |
| MV-03 | `main.go` — `session/start`, `session/end`, `media/event`, `emergency-stop` auf `vehicleContexts.Get(...)` umgestellt; E-Stop ohne `vehicle_id` = fleet-weit (via `ActiveVehicleIDs()`), mit `vehicle_id` = gezielt | M | ✅ |
| MV-04 | WS-Handler + Command Engine — `sm`/`deadman`/`ackWatcher`-Felder durch Registry-Lookup über `sess.VehicleID` ersetzt | M | ✅ |
| MV-05 | `vehicleconnection.Handler` — Deadman/ACK-Watchdog-Start über `VehicleContext` (Lookup über `claims.Subject`) statt globaler Felder | M | ✅ |
| MV-06 | `GET /vehicles/{id}/state` Endpoint ergänzt. `GET /state` **bewusst nicht entfernt** — bleibt als Compat-Shim (k6 latency.js, ältere Clients), bis MV-07 alle Frontend-Konsumenten migriert hat | S | ✅ (Teil 2 verschoben) |
| MV-07 | Frontend `useSystemState(vehicleId, token)` — Live-Polling vs. Page-Reload-Recovery (`GET /sessions`) getrennt; Unreachable-Banner nutzt `GET /sessions` ohne Fahrzeug | M | ✅ |
| MV-08 | Edge-Case-Tests: 2 Fahrzeuge parallel unabhängiges SAFE_MODE, SafetyBusWatchdog fleet-wide, E-Stop mit/ohne `vehicle_id` | M | ✅ |

**Test-Ergebnis Backend:** 13 neue Unit-Tests (`vehiclecontext_test.go`) + 11 neu geschriebene SafetyBusWatchdog-Tests (`watchdog_test.go`, alte Single-Session-API durch Fleet-Wide-API ersetzt) + 4 neue Integrationstests (`multivehicle_test.go`, gegen echten Docker-Teststack) — alle grün, `-race`-sauber. Gesamte bestehende Unit-Suite (112 Tests) bleibt grün.

**Test-Ergebnis Frontend (MV-07):** 12 neue Tests für `useSystemState` (Vehicle-Polling, Reachability-Probe ohne Fahrzeug, Fehlerzähler-Reset bei Fahrzeugwechsel, Unmount-Cleanup) + 1 neuer Test für `UserManagementPanel` (mehrere aktive Operatoren gleichzeitig gesperrt). Gesamte Frontend-Suite (103 Tests, 10 Dateien) grün, `tsc --noEmit` sauber, Produktionsbuild erfolgreich.

**MV-07 Detailänderungen:**
- `api-client.ts`: `getState()` entfernt (kein Konsument mehr im Frontend), neu `getVehicleState(vehicleId)` → `GET /api/vehicles/{id}/state`; `SystemStateResponse` auf die 4 Layer reduziert (kein `session_id`/`vehicle_id`/`role`/`operator_id` mehr — das kam ohnehin nur aus dem alten globalen `GET /state`)
- `useSystemState(vehicleId, token)`: mit `vehicleId` → pollt das Fahrzeug; ohne → pollt `GET /sessions` rein als Reachability-Probe (Ergebnis verworfen); Fehlerzähler wird bei Fahrzeugwechsel zurückgesetzt
- `App.tsx`: Page-Reload-Recovery nutzt jetzt den ohnehin laufenden `activeSessions`-Poll (eigene Session per `operator_id` finden), entkoppelt von `isSafeMode` (vorher zirkulär: brauchte den State, um den State zu bekommen)
- `UserManagementPanel`: `activeOperatorId?: string` → `activeOperatorIds?: string[]` — mehrere Fahrzeuge können gleichzeitig einen ACTIVE_OPERATOR haben, ein einzelner Wert hätte nur einen davon gesperrt

**Nebenbei gefunden + behoben (unabhängig von ADR-026, eigener Commit `3dc3c9e`):** `tests/integration/services_test.go` + `tests/performance/latency_test.go` + `latency.js` nutzten noch das alte Login-Feld `id` statt `username` und fehlende Auth-Header — Regression aus der Nutzerverwaltungs-Aufgabe, nie mit `go vet`/`go test` geprüft.

**Bewusst nicht angefasst (Scope-Grenze, im Backlog verfolgt):**
- `HandoverManager` behält eine eigene, dedizierte einzelne State Machine (`handoverSM`) — er transitioniert nur die OPERATOR-Schicht, nie SAFE_MODE, daher kein Split-Brain-Risiko. Multi-Vehicle-Handover ist nicht Teil von ADR-026 (Folge-Task `MV-11` — mittlerweile erledigt, siehe `tasks/sprints/30-restposten-bereinigung.md`).
- `internal/authservice/handler_test.go` ist unabhängig von ADR-026 bereits kaputt (alte string-ID/`DisplayName`-API) — als `AUTH-TEST-01` im Backlog, nicht in diesem Sprint angefasst.
- `GET /state` vollständig entfernen — als `MV-12` im Backlog, hängt nur noch an `latency.js`/`services_test.go`.

**Abhängigkeitspfad:** MV-01 → MV-02, MV-03, MV-04, MV-05 (mussten kohärent zusammen erfolgen) → MV-06 (teilweise, `GET /state` bewusst erhalten) → MV-07 ✅ → MV-08 ✅

---

## Neue/geänderte Dateien

- `internal/controlserver/vehiclecontext/registry.go` — NEU: `VehicleContext` + `Registry`
- `internal/controlserver/safety/bus_watchdog.go` — `SafetyBusWatchdog` komplett umgebaut (fleet-wide)
- `internal/controlserver/session/manager.go` — `ActiveVehicleIDs()`, `GetSessionByVehicle()`
- `internal/controlserver/command/engine.go` — `vehicleContexts *vehiclecontext.Registry` statt `sm`/`deadman`
- `internal/controlserver/transport/websocket.go` — `vehicleContexts` statt `sm`/`deadman`/`ackWatcher`
- `internal/vehicleconnection/handler.go` — `vehicleContexts` statt `sm`/`vehicleACKWatchdog`
- `cmd/control-server/main.go` — komplette Verdrahtung; `GET /vehicles/{id}/state`; `emergency-stop` Scoping; `GET /state` Compat-Shim
- `frontend/src/hooks/useSystemState.ts` + `.test.ts` — NEU: vehicle-scoped Polling
- `frontend/src/lib/api-client.ts` — `getVehicleState()` statt `getState()`
- `frontend/src/App.tsx` — Page-Reload-Recovery über `activeSessions`; `activeOperatorIds`
- `frontend/src/components/UserManagementPanel.tsx` + `.test.tsx` — `activeOperatorIds[]`
- `docs/adr/026-multi-vehicle-state-isolation.md` — NEU

---

**Redaktionshinweis (MD-Konsolidierung, 2026-07-19):** Diese Datei führt zwei zuvor parallel
geführte Versionen dieses Sprints zusammen (`tasks/current-sprint.md`: Narrativ + MV-07-
Detailänderungen + Abhängigkeitspfad; `tasks/done.md`: Datei-Liste) — beide Versionen waren
inhaltlich komplementär, nicht redundant.
