# Sprint 30 — Restposten-Bereinigung (MV-Folge-Tasks, TASKUI-Nacharbeiten, Doku)

Ziel: Neun bislang unzugeordnete Backlog-Tasks aus drei unterschiedlichen EPICs abarbeiten — kein
gemeinsames fachliches Thema, daher als reiner Cleanup-Sprint zusammengefasst. Aus "Multi-Vehicle
State Isolation" (Sprint 17): `MV-09` (Live-State-Badge im Vehicle-Dropdown), `MV-11` (Handover-
State-Machine pro Fahrzeug isolieren), `MV-12` (`GET /state` entfernen). Aus "Fleet Dashboard
Planung" (TASKUI-Nachträge, Sprint 24): `TASKUI-01` (doppelte Demo-Stationsanlage), `TASKUI-02`
(Task-Status-Übergangstabelle Go/TS dedupliziert), `TASKUI-04` (Nil-Slice-→-JSON-`null`-Fix),
`TASKUI-05` (`COMPOSE_PROJECT_NAME` pro Worktree). Aus "Lokaler Dev-Stack Verifikation"
(Sprint 19): `DOC-01` (`frontend/README.md`), `DOC-02` (versionierte `control-server`-Binary aus
Tracking entfernen).

**Sprint-Zuschnitt-Rückfrage 2026-07-18** (CLAUDE.MD Abschnitt 10, max. 10 Tasks): 11 Kandidaten
im Backlog gefunden. `MV-10` (GC für `VehicleContext`-Instanzen) zurückgestellt — Backlog-Text
sagt selbst "aktuell nicht relevant (kleine Flotte)". `TASKUI-03` (vollständige Task-Status-
Audit-Historie, neue Tabelle+Migration) auf Nutzerentscheidung ebenfalls vorab zurückgestellt —
inhaltlich am wenigsten mit dem Rest verwandt, komfortabler Abstand zum 10er-Limit. Damit 9 Tasks
in diesem Sprint.

**Keine Signaturänderung nach außen ohne Zweck, kein ungewollter Verhaltenswechsel** (CLAUDE.MD
Abschnitt 15) — Ausnahme sind `MV-09`/`MV-11`, wo die Verhaltensänderung explizit der Taskzweck
ist (dort gegen die im Task beschriebene Zielsemantik getestet, nicht auf Unverändertheit). Bei
den Go-Tasks (`MV-09`, `MV-11`, `MV-12`, `TASKUI-02`, `TASKUI-04`) gilt zusätzlich
`docs/go-style-guide.md` (Funktionslänge, Parameterzahl) für neuen/geänderten Code.

Datum: 2026-07-18 | **Status: Alle Tasks ✅**
Vorgänger: Sprint 29 ✅ (dieser Branch zweigt von `feature/fleet-service-foundation-gostyle29` ab,
NICHT von `feature/fleet-service-foundation` direkt, da `MV-09`/`MV-11`/`MV-12` `cmd/control-
server/main.go` anfassen, das in Sprint 27/28/29 komplett auf das `controlServer`-Struct+Methoden-
Muster umgebaut wurde)
Branch: `feature/fleet-service-foundation-cleanup30`

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| TASKUI-01 | Doppelte Demo-Stationsanlage bereinigen | S | ✅ |
| DOC-01 | `frontend/README.md` aktualisieren | S | ✅ |
| DOC-02 | Versionierte `control-server`-Binary aus Tracking entfernen | S | ✅ |
| TASKUI-04 | Nil-Slice-→-JSON-`null`-Fix in `store.go` List*-Methoden | S | ✅ |
| TASKUI-02 | Task-Status-Übergangstabelle Go/TS dedupliziert | S | ✅ |
| MV-12 | `GET /state` entfernen, Konsumenten migrieren | S | ✅ |
| TASKUI-05 | `COMPOSE_PROJECT_NAME` pro Worktree | M | ✅ |
| MV-09 | Vehicle-Dropdown Live-State-Badge | M | ✅ |
| MV-11 | Multi-Vehicle Handover — State Machine pro Fahrzeug isolieren | M | ✅ (bereits erledigt vorgefunden, siehe Ergebnisse) |

## Ergebnisse

**TASKUI-01 — Doppelte Demo-Stationsanlage bereinigt ✅**
`demoStationSeed` (INSERT-basiert, `-taskui`-Platzhalterdaten) in `internal/fleetservice/store.go`
durch `taskuiDemoSeedCleanup` (DELETE-basiert) ersetzt: die echte Seed-Quelle
(`scripts/seed-fleet-demo.sh`, MAP-01, Zone `zone-betriebshof-nord`) macht den `-taskui`-
Platzhalter aus Sprint 24 überflüssig. Läuft wie die bestehenden ALTER-Migrationen bei jedem
Start; `NOT EXISTS`-Guards verhindern sowohl unnötige Wiederholung als auch einen FK-Fehler,
falls ein echter Task doch auf eine `-taskui`-Station verweisen sollte (dann No-op statt Crash).
`go build`/`go test ./internal/fleetservice/...` grün.

**DOC-01 — `frontend/README.md` aktualisiert ✅**
Proxy-Tabelle um `/vehicle/ws`, `/fleet/`, `/whip/`, `/whep/` ergänzt; `useWebRTC`-Beschreibung
korrigiert (WHEP-Signaling statt altem `/sfu/subscribe/`-Pfad); Komponenten-/Hooks-/Lib-Liste
komplett auf den aktuellen Stand gebracht (Fleet-Dashboard-Komponenten, `LoginPanel`,
`UserManagementPanel`, `StreamSenderPanel`, `useWHIPSender`, `useVehicleAck` u. a. ergänzt);
neuer Abschnitt "Nutzerverwaltung (Sprint 15)" und "Fleet Dashboard (Sprint 22–25)" im
Funktionsumfang.

**DOC-02 — Binary aus Tracking entfernt ✅**
`control-server`-Binary (~12 MB) im Repo-Root per `git rm --cached` aus dem Tracking entfernt
(lokale Datei bleibt erhalten), `/control-server` zu `.gitignore` hinzugefügt. Nur aktuelles
Tracking bereinigt — **keine** History-Rewrite (per Vorgabe nicht eigenmächtig ausgeführt).

**TASKUI-04 — Nil-Slice-Fix in fünf weiteren List*-Methoden ✅**
Gleiches Muster wie das bereits gefixte `ListTasks` (`var x []T` → `x := []T{}`) auf `ListZones`,
`ListStations`, `ListVehicleStatus`, `ListVehiclesWithStatus`, `ListAlerts` angewendet
(`internal/fleetservice/store.go`). `go test ./internal/fleetservice/...`: 36/36 Tests grün.

**TASKUI-02 — Task-Status-Übergangstabelle dedupliziert ✅**
Statt eines Codegen-Schritts (im Backlog nur als "ggf." vorgeschlagen) wird die Übergangsmatrix
jetzt serverseitig abgeleitet und über die Wire geschickt: `Task.AllowedTransitions
[]string` (neues Feld, `json:"allowed_transitions"`) wird in `CreateTask`/`ListTasks`/
`UpdateTaskStatus` über die neue Funktion `allowedTaskTransitions(status)` befüllt — die Inverse
von `taskTransitionSources`, mit fixer `taskTransitionOrder` für deterministische Button-
Reihenfolge. `FleetTaskPanel.tsx`s hartcodierte `NEXT_TRANSITIONS`-Map entfällt; die Buttons
rendern jetzt aus `task.allowed_transitions`, nur noch eine reine Label-Zuordnung
(`TRANSITION_LABEL`, unabhängig vom Ausgangsstatus) bleibt clientseitig. Damit gibt es nur noch
eine Quelle für die Zustandsmaschine selbst (Backend); die Frontend-Seite kann nicht mehr
divergieren. Bestehende Test-Fixtures (`FleetTaskPanel.test.tsx`, `fleet-merge.test.ts`,
`useFleetOverview.test.ts`) um `allowed_transitions` ergänzt. Go: 36/36 Tests grün. Frontend:
`tsc -b --noEmit` sauber, 255/255 Vitest-Tests grün.

**MV-12 — `GET /state` entfernt, Konsumenten migriert ✅**
Route `GET /state` sowie `handleState` aus `cmd/control-server/main.go` entfernt.
`tests/performance/latency.js` auf `GET /sessions` migriert (keine Fahrzeug-Bindung in `setup()`
— identischer No-Vehicle-Reachability-Probe-Pfad wie `useSystemState.ts` im Frontend).
`tests/integration/services_test.go` (5 Aufrufstellen) auf `GET /vehicles/{id}/state` migriert,
je mit dem im selben Testfall verwendeten `vehicle_id` (bzw. einer dedizierten ID für den
Initial-State-Test ohne Session). Verifiziert gegen den echten Docker-Test-Stack
(`docker compose -f tests/docker-compose.test.yml`): 26/29 PASS + 3 vorbestehende WS-Skips
(session_id-Lücke, siehe Sprint 29 GOSTYLE-13-Notiz — nicht Teil dieses Tasks), `curl .../state`
bestätigt 404. `sessionMgr.GetCurrentSession()` ist durch die Entfernung von `handleState` jetzt
ohne Produktions-Aufrufer (nur noch eigene Unit-Tests) — bewusst nicht entfernt, da das eine
andere Baustelle ist als "GET /state migrieren" (siehe "Bewusst nicht in diesem Sprint" unten).

**TASKUI-05 — `COMPOSE_PROJECT_NAME` pro Worktree ✅**
`infrastructure/compose/docker-compose.yml`s `name: avoc` bleibt unverändert — `COMPOSE_PROJECT_NAME`
hat laut `docker compose config`-Test empirisch Vorrang vor dem Datei-`name:`. Fix stattdessen im
`Makefile`: neue Variable `COMPOSE_PROJECT_NAME ?= $(shell basename $(CURDIR) | tr -d '\n' | tr
'A-Z' 'a-z' | tr -c 'a-z0-9_-' '-')`, `export`iert für alle `docker compose`-Aufrufe in `up`/`down`.
Verifiziert über `docker compose config` mit zwei verschiedenen worktree-abgeleiteten Projektnamen
(`controlcenter-aws-cleanup30`, `controlcenter-aws-taskui`): vollständig getrennte Netzwerk-
(`<project>_avoc-net`) und Container-Namensräume (`<project>-control-server-1` etc.) bestätigt —
kein voller Parallel-Stack-Build nötig, da das der Mechanismus ist, den Compose selbst zur
Container-Wiederverwendungs-Entscheidung nutzt. Ein initialer Bug (`basename`s eigener
Trailing-Newline wurde von `tr -c ... '-'` in einen literalen Bindestrich umgewandelt, bevor
Makes `$(shell ...)`-Trailing-Newline-Stripping greifen konnte) wurde beim Verifizieren gefunden
und mit `tr -d '\n'` vor der Zeichen-Sanitisierung behoben.

**MV-09 — Vehicle-Dropdown Live-State-Badge ✅**
Backend: `handleVehiclesList` (`cmd/control-server/main.go`) liefert jetzt `system_state` pro
Fahrzeug (`s.vehicleContexts.Get(v.ID).SM.Get()` — derselbe Mechanismus wie
`handleVehicleState`/`GET /vehicles/{id}/state`, hier auf die ganze Liste angewendet). Frontend:
`VehicleInfo.system_state` (neues Pflichtfeld) und `VehicleSelector.tsx` zeigt bei `SAFE_MODE`
🔴 + " — SAFE_MODE" statt 🟢 (einziger badge-würdiger Zustand, analog zur bestehenden
SAFE_MODE-Sonderbehandlung in `ConnectionPanel`/`SafetyPanel`/`SafeModeOverlay`). Neuer Test
`VehicleSelector.test.tsx` (3 Fälle: normal, SAFE_MODE, gemischte Liste — badged nur das
betroffene Fahrzeug). Verifiziert gegen den echten Docker-Test-Stack: `GET /vehicles` liefert das
neue Feld korrekt (Default `IDLE` für neu registriertes Fahrzeug); der SAFE_MODE-Übergang selbst
läuft über dieselbe, bereits durch `TestIntegration_MultiVehicle_ScopedEmergencyStop_...`
abgedeckte `SM.Get()`-Quelle — keine zusätzliche Live-WS-Verifikation nötig, da kein neuer
State-Übergangspfad entstanden ist, nur eine zusätzliche Lesestelle. Go: `go build`/`go vet ./...`
sauber. Frontend: `tsc -b --noEmit` sauber, 255/255 Vitest-Tests grün (3 neue).

**MV-11 — bereits erledigt vorgefunden, keine Neuimplementierung ✅**
Vor der Umsetzung geprüft (`grep -rn "handoverSM"` → keine Treffer, `git log --follow
internal/controlserver/session/handover.go`): der im Backlog beschriebene Bug (`HandoverManager`
mit einer einzigen globalen State Machine) wurde bereits am 2026-07-15 in Commit `f68346a`
("Mehrere Fahrzeuge gleichzeitig durch verschiedene Operatoren steuerbar") behoben —
`HandoverManager.pending` ist seitdem eine `map[string]*pendingHandover` pro Fahrzeug, aufgelöst
über dieselbe `vehiclecontext.Registry` wie State Machine/Watchdogs (ADR-026). Ein dedizierter
Regressionstest existiert bereits: `tests/unit/safety_test.go`s
`TestSafety_Handover_TwoVehicles_IndependentHandovers` (Kommentar im Test: "is the MV-11
regression test") prüft exakt die im Backlog beschriebene Garantie — ein Handover auf Fahrzeug 1
blockiert/leakt nicht in Fahrzeug 2. Alle 4 `TestSafety_Handover_*`-Tests erneut laufen lassen:
4/4 PASS. Statt redundant erneut zu implementieren (Risiko: bestehenden, bereits korrekten und
getesteten Code unnötig anzufassen, CLAUDE.MD Abschnitt 15), nur `tasks/backlog.md` auf den
tatsächlichen Stand nachgezogen (Status 🔲 → ✅, Referenz auf Commit + Test ergänzt) — die
Backlog-Zeile war schlicht nicht nachgepflegt worden, ein weiterer Fall der bereits mehrfach
aufgetretenen Doku-Drift zwischen parallelen Sessions (analog ADR-030/031, Sprint-26-Kollision).

**Verifikation (gesamt) ✅**
`go build ./...`, `go vet ./...` sauber (kein `go build` im Repo-Root ohne Zielpfad, siehe
Sprint-28-Binary-Vorfall). `go test $(go list ./... | grep -v /tests/integration)`: alle Pakete
grün. `go test ./tests/integration/...` gegen den echten Docker-Test-Stack
(`tests/docker-compose.test.yml`): 26/29 PASS + 3 vorbestehende WS-Skips (unverändert seit
Sprint 29, session_id-Testlücke). Frontend: `npx tsc -b --noEmit` sauber, `npm test -- --run`:
255/255 Tests grün (26 Dateien). `gofmt`/`golangci-lint` nicht erneut über den gesamten Bestand
laufen lassen — außerhalb des Scopes dieses reinen Cleanup-Sprints, war bereits Gegenstand von
Sprint 27–29.

**Bewusst nicht in diesem Sprint:**
- `session.Manager.GetCurrentSession()` ist seit der `GET /state`-Entfernung (MV-12) ohne
  Produktions-Aufrufer (nur noch eigene Unit-Tests in `tests/unit/multioperator_test.go`) —
  bleibt als exportierte, direkt getestete `Manager`-Methode bestehen; ihre Entfernung wäre ein
  eigener, nicht angefragter Scope (würde auch mehrere bestehende Tests anfassen) und ist als
  möglicher Folge-Task vorzumerken, nicht Teil von MV-12.
- `MV-10` (GC für `VehicleContext`-Instanzen) und `TASKUI-03` (volle Task-Status-Audit-Historie)
  wie in der Sprint-Zuschnitt-Rückfrage entschieden zurückgestellt, bleiben in `tasks/backlog.md`.
- Kein neuer Codegen-Mechanismus für TASKUI-02 (Backlog nannte das nur als "ggf."-Option) — die
  serverseitig-berechnete `allowed_transitions`-Lösung erreicht dieselbe Single-Source-of-Truth-
  Garantie ohne zusätzliche Build-Infrastruktur.
