## Sprint 46 — control-server: Hexagonal-Migration Vorbereitung (neues ADR + Testaufbau)

**Freigabe (2026-07-20):** Nutzerentscheidung gegenüber zwei Alternativen (Hexagonal-Migration
safety-service/webrtc-sfu/recording — verworfen, siehe Befund unten; `newSafetyMux` exportieren —
verworfen als zu klein). `control-server` ist laut ADR-031/DECISIONS.MD seit dem Piloten explizit
ausgeklammert ("höchstes Risiko, geringste Testabdeckung") — dieser Sprint baut die Testabdeckung
auf und schreibt das dafür nötige ADR. **Kein Produktivcode-Refactor in diesem Sprint selbst.**

**Vorab-Befund zu den Alternativen (2026-07-20):** `safety-service`/`webrtc-sfu`/`internal/recording`
haben keine mit fleet-service/auth-service/telemetry-service vergleichbare Infrastruktur-Abhängigkeit,
die sich hinter einem Port kapseln ließe (`safetyservice.Bus` ist rein in-memory; `recording`s
`SessionRecorder`-Interface wurde in Sprint 34 bewusst entfernt, solange kein zweiter Adapter
existiert; `webrtcsfu`s `pion/webrtc`-Kopplung abzukapseln wäre ein grundlegend größerer, riskanterer
Schnitt als der MQTT-Wrap bei telemetry-service). Einzig realer Punkt dort (`newSafetyMux` aus
`package main` lösen) ist zu klein für einen eigenen Sprint.

**Scope-Vorrecherche (2026-07-20):** Coverage-Analyse mit `go test ./tests/unit/...
-coverpkg=./internal/controlserver/...,./cmd/control-server/...` (per-Package-Coverage ohne
`-coverpkg` täuscht 0% vor, weil die umfangreiche bestehende Testsuite in `tests/unit/*.go` —
`watchdog_test.go`, `safety_test.go`, `multioperator_test.go`, `vehiclecontext_test.go`, 2711
Zeilen — die `internal/controlserver`-Subpakete von außen testet):

- `internal/controlserver/{statemachine,safety,session,command,vehiclecontext}`: **84.1% Coverage**
  über `tests/unit` — State Machine, alle 5 Watchdogs (Deadman/ACKTimeout/VehicleACK/AuthWatchdog/
  SafetyBusWatchdog), Session-Manager, Handover-Manager sind bereits solide getestet.
- **`cmd/control-server/main.go` (919 Zeilen, `package main`): 0% Coverage auf jeder einzelnen
  Funktion** — Bootstrap (`loadConfig`/`newAuditWriter`/`newVehicleStore`/`newControlServer`/
  `buildHandlers`/`newMux`/`main`) UND alle ~25 HTTP-Handler. Das ist die eigentliche, echte Lücke.
- `internal/controlserver/authcheck/checker.go` (46 Zeilen, DB-Query gegen `users`-Tabelle): 0%.
- `internal/controlserver/transport/websocket.go`: sehr niedrig (~5%, nur die kleine eigene
  `websocket_test.go`) — bleibt bewusst außerhalb dieses Sprints (eigene, große Sicherheitsfläche).

**Scope dieses Sprints:** ADR + Testaufbau für den sicherheitskritischsten Teil von
`cmd/control-server/main.go` — den Auth-Gate (`requireJWT`) und die Session-Lifecycle-Handler, die
Watchdogs scharf schalten/deaktivieren und SAFE_MODE auslösen können (`handleSessionStart`/
`advanceVehicleToActiveOperator`, `handleSessionEnd`, `handleEmergencyStop`). Test-Fixture baut
einen echten `controlServer` aus den bereits gut getesteten Bausteinen (`session.NewManager`,
`vehiclecontext.NewRegistry`, `command.NewEngine` usw., alle ohne echte Postgres-Verbindung
konstruierbar) — **kein** Mock/Fake-Refactor an Produktivcode nötig, da `HTTPPublisher`/
`HTTPSFUPublisher` bereits gegen eine Test-URL zeigbar sind (analog `safety_bus_integration_test.go`).

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| CTRL-01 | Neues ADR `docs/adr/035-control-server-hexagonal-migration-prep.md` — Risikoanalyse (korrigiert die "0% überall"-Annahme: `internal/controlserver`-Subpakete sind über `tests/unit` bereits 84% getestet, die echte Lücke ist `cmd/control-server/main.go` selbst), Scope-Vorschlag + empfohlene Reihenfolge für eine spätere echte Migration (Handler-Struct-Extraktion analog fleet-service/auth-service, aber erst nach Testaufbau — Präzedenzfall Sprint 39). ADR-031/DECISIONS.MD referenzieren das neue ADR. | S | ✅ Sprint 46 | — |
| CTRL-02 | `requireJWT` (`cmd/control-server/main.go:893`) — eigenständige Funktion ohne `controlServer`-Abhängigkeit, isolierter Tabellen-Test in neuem `cmd/control-server/main_test.go`: fehlender Header, malformed Token, falsches Secret, gültiger Token inkl. Rollen-Weiterreichung in den Request-Context. | S | ✅ Sprint 46 | — |
| CTRL-03 | Test-Fixture `newTestControlServer(t)` in `cmd/control-server/main_test.go` — baut einen echten `*controlServer` aus handgebauten Abhängigkeiten (`session.NewManager(&mocks.MockSFUPublisher{})`, `vehiclecontext.NewRegistry(...)`, `command.NewEngine(...)`, `csafety.NewHTTPPublisher(testServerURL)` gegen einen lokalen `httptest.Server`, `recording.NewMemoryRecorder()`, `vehicleconnection.NewRegistry()`/`NewAckStore()`, `mediamtx.NewClient("")`), ruft `buildHandlers()` + `newMux()`. Erster Beweis: `handleHealth`-Test. | M | ✅ Sprint 46 | — |
| CTRL-04 | `handleSessionStart`/`advanceVehicleToActiveOperator` Tests: Fahrzeug nicht verbunden (409), OBSERVER darf keine neue Session claimen (403), gültiger ACTIVE_OPERATOR-Pfad schaltet Deadman/VehicleACKWatchdog scharf und fährt die State Machine bis CONNECTED hoch, gültiger OBSERVER-Join-Pfad lässt die State Machine unangetastet. | M | ✅ Sprint 46 | CTRL-03 |
| CTRL-05 | `handleSessionEnd` Tests: Beendigung per `session_id` (ACTIVE_OPERATOR resettet auf IDLE, OBSERVER lässt Fahrzeug-State unangetastet), unbekannte `session_id` (No-Op, 204), Legacy-Pfad ohne `session_id` (fleet-weite Beendigung, mehrere Fahrzeuge gleichzeitig zurückgesetzt). | M | ✅ Sprint 46 | CTRL-03 |
| CTRL-06 | `handleEmergencyStop` Tests: fahrzeugspezifisch vs. fleet-weit (kein `vehicle_id` im Body), SAFE_MODE-Transition verifiziert, `TriggerEmergencyStop` beim lokalen Test-Server angekommen (analog `safety_bus_integration_test.go`-Muster). **Echter Fund dabei:** ein Fahrzeug ohne aktive Session steht in SYSTEM=IDLE, und `IDLE→SAFE_MODE` ist laut `validSystemTransitions` kein gültiger Übergang — die Transition wird lautlos abgelehnt (nur Warn-Log), `TriggerEmergencyStop` feuert trotzdem und der Endpoint antwortet weiterhin 202. Vorbestehendes Produktivverhalten, kein Fix in diesem Sprint (siehe Scope), aber als Kandidat im neuen ADR dokumentiert. | M | ✅ Sprint 46 | CTRL-03 |
| CTRL-07 | `internal/controlserver/authcheck/checker_test.go` — `DATABASE_URL`-gated Test gegen die echte `users`-Tabelle (aktiver/inaktiver/gelöschter Username), analog dem Postgres-Testmuster aus `internal/fleetservice/store_test.go` vor dem Hexagonal-Pilot. Verifiziert gegen einen ephemeren lokalen `postgres:16-alpine`-Container (3x grün, `-race -count=2`), danach entfernt. | S | ✅ Sprint 46 | — |
| CTRL-08 | Verifikation: `go build ./...`, `go vet ./...`, `go test ./cmd/control-server/... ./internal/controlserver/authcheck/... -race -count=2` (Flakiness-Check) — alles grün, zusätzlich `go test ./... -race` gesamt geprüft (nur die erwartbaren `tests/integration`-Fehlschläge, kein Docker-Teststack). ADR-031-Cross-Reference auf ADR-035 + `DECISIONS.MD` + `tasks/backlog.md` (neuer EPIC-Eintrag "control-server Testaufbau"). | S | ✅ Sprint 46 | CTRL-01..07 |

**Nicht Teil dieses Sprints:** `internal/controlserver/transport/websocket.go` (eigene, große
Sicherheitsfläche — eigener Folge-Sprint), die restlichen ~20 HTTP-Handler in `main.go`
(Handover/Media/Log/Audit/Recording/MediaAuth/Vehicles-CRUD/Sessions/ICE — Folge-Sprints nach
Priorität), die DB-touchende Bootstrap-Logik selbst (`loadConfig`/`newAuditWriter`/
`newVehicleStore`/`newControlServer`/`main()` — bräuchte eigene Postgres-Test-Infrastruktur,
anderer Scope), die eigentliche Hexagonal-Migration (Handler-Struct-Extraktion) — kommt laut ADR-035
erst nach diesem Testaufbau als eigener, separat zu entscheidender Schritt.

**Geschätzter Umfang:** 8 Tasks, überwiegend Typ S/M — innerhalb des ~200k-Token-Sprintbudgets.

**Ergebnis:** Neues ADR `docs/adr/035-control-server-hexagonal-migration-prep.md` (36 ADRs
gesamt). Neue Testdatei `cmd/control-server/main_test.go` (20 neue Tests: 6 `requireJWT`, 1
`handleHealth`, 4 `handleSessionStart`, 4 `handleSessionEnd`, 3 `handleEmergencyStop`, plus die
`newTestControlServer`-Fixture), neue `internal/controlserver/authcheck/checker_test.go` (3
`DATABASE_URL`-gated Tests). Kein Produktivcode geändert. Echter Fund dokumentiert (siehe CTRL-06):
`IDLE→SAFE_MODE` ist kein gültiger State-Transition, Emergency-Stop auf ein Fahrzeug ohne aktive
Session wird lautlos ignoriert — als offener Entscheidungspunkt im ADR festgehalten, nicht
behoben. Verifiziert: `go build`/`go vet ./...` sauber, `go test ./cmd/control-server/...
./internal/controlserver/authcheck/... -race -count=2` zweimal grün; `authcheck`-Tests zusätzlich
gegen einen ephemeren `postgres:16-alpine`-Container real verifiziert (danach entfernt); `go test
./... -race` gesamt geprüft, nur die erwartbaren `tests/integration`-Fehlschläge (kein laufender
Docker-Teststack). Doku aktualisiert: ADR-031 (Cross-Reference-Update-Block), `DECISIONS.MD`
(neue ADR-035-Zeile + korrigierte control-server-Zeile), `docs/adr/README.md` (36 ADRs),
`tasks/backlog.md` (neuer EPIC "control-server Hexagonal-Migration — Vorbereitung").

---

Vorgänger: Sprint 45 ✅ (Hexagonale Architektur: Use-Case-Extraktion fleet-service/auth-service
(HEX-06/HEXAUTH-04), siehe `tasks/sprints/45-hexagonal-usecase-extraktion.md`)
