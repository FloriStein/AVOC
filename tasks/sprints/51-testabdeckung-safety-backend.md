> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Sprint 51 — Testabdeckungs-Gesamtaudit 2026-07-21, Teil 1 (Safety-kritische Backend-Testlücken)

**Freigabe (2026-07-21):** Bereits vollständig geplant im EPIC "Testabdeckungs-Gesamtaudit
2026-07-21" (`tasks/backlog.md`, Abschnitt "Teil 1"). Reine Testabdeckung für bestehenden,
unveränderten Produktivcode — keine Verhaltensänderung, kein Grill-Me nötig (Typ S/M, keine
sicherheitskritische Architekturentscheidung, siehe CLAUDE.MD §1.1/§17). Nächster Sprint nach
Sprint 50 (`tasks/sprints/50-telemetry-watchdog.md`, DRIFT-K3-TELEMETRY vollständig abgeschlossen).

## Vorrecherche (Go-Agent, 2026-07-21, bereits vor Sprint-Kickoff durchgeführt)

`go test ./... -cover` unterschätzt `control-server`s Safety-Layer strukturell (Tests liegen in
`tests/unit`, externes Package) — korrekt gemessen via `-coverpkg` liegt
`internal/controlserver/{statemachine,session,safety,command,vehiclecontext}` bereits bei 84,1 %.
Echte Nullstellen und gezielte Funktionslücken mit Sicherheitsbezug:

- `internal/mediamtx` (WHIP/WHEP-Auth-Client für den SAFE_MODE-Video-Kick, 87 Zeilen,
  **0 Tests überhaupt**).
- `internal/vehicleregistry` (Vehicle-CRUD/Persistenz, 137 Zeilen, **0 Tests überhaupt**).
- `session/sfu_publisher.go` (`PublishSessionEvent`, `NewHTTPSFUPublisher`) 0 %.
- `safety/http_publisher.go:TriggerEmergencyStop` 0 % — der HTTP-Fehlerpfad zu safety-service wird
  geloggt und verschluckt, dieses Silent-Failure-Verhalten ist nie verifiziert.
- `command/engine.go`s Audit-Write-Fehlerpfad innerhalb der E-Stop-Behandlung
  (`svcLog.Error("audit write failed — proceeding to SAFE_MODE")`) ebenfalls ungetestet.
- `session/handover.go:issueHandoverToken` nur 20 % (auth-service-HTTP-Fehlerpfade offen).

## Tasks

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| GOTEST-01 | `internal/mediamtx/client_test.go` — WHIP/WHEP-Auth-Client vollständig (Erfolg, HTTP-Fehler, Timeout, malformed Response). Aktuell 0 Tests, 87 Zeilen. | S/M | ✅ | — |
| GOTEST-02 | `internal/vehicleregistry/registry_test.go` — Vehicle-CRUD (`SQLiteVehicleStore`/`NoopVehicleStore`, `ErrNotFound`-Sentinel, Doppel-Registrierung, Löschen bei aktiver Session). Aktuell 0 Tests, 137 Zeilen. | M | ✅ | — |
| GOTEST-03 | `internal/controlserver/safety/http_publisher_test.go` — `TriggerEmergencyStop` gegen simulierten HTTP-Fehler (Netzwerkfehler, non-2xx) — verifiziert, dass der Swallow-Pfad wirklich geloggt und der Aufrufer nicht blockiert wird. | S | ✅ | — |
| GOTEST-04 | `internal/controlserver/command/engine_test.go` — Audit-Write-Fehlerpfad innerhalb `handleEmergencyStop`/`forwardMovementCommand` (Audit-Writer liefert Fehler, E-Stop muss trotzdem nach SAFE_MODE laufen). | S/M | ✅ | — |
| GOTEST-05 | `internal/controlserver/session/sfu_publisher_test.go` + `manager_test.go` — `PublishSessionEvent`/`NewHTTPSFUPublisher` (inkl. Fehlerpfad) und `ListSessions`. | S/M | ✅ | — |
| GOTEST-06 | `internal/controlserver/session/handover_test.go` — `issueHandoverToken`-Fehlerpfade (auth-service antwortet mit non-2xx/Timeout). | S | ✅ | — |
| GOTEST-07 | Verifikation: `go build ./...`, `go vet ./...`, `go test ./... -race` (2× gegen Flakiness), Coverage-Diff dokumentieren, `DECISIONS.MD`/`tasks/backlog.md`-Update. | S | ✅ | GOTEST-01..06 |

**Nicht Teil dieses Sprints:** `internal/webrtcsfu`s `forwardTrack`/echte SDP-Negotiation (bewusster
Nicht-Scope seit Sprint 39, ADR-006 "zu flaky in CI"), Load-/Stress-Test der Watchdogs unter
gleichzeitiger Multi-Vehicle-Last (das ist Teil 5 des EPICs, eigener späterer Sprint-Kandidat).

**Geschätzter Umfang:** 7 Tasks, überwiegend S/M — innerhalb des ~200k-Token-Sprintbudgets.

## Ergebnis (2026-07-21, CLAUDE.MD §17 — Ergebnis dokumentieren, nicht behaupten)

**Coverage-Diff (nur die betroffenen Dateien/Funktionen, `-coverprofile` je Paket gemessen):**

| Datei/Funktion | Vorher | Nachher |
|---|---|---|
| `internal/mediamtx/client.go` (gesamtes Paket) | 0 % | 97,0 % |
| `internal/vehicleregistry` (gesamtes Paket) | 0 % | 97,4 % |
| `safety/http_publisher.go:TriggerEmergencyStop`/`PublishEvent`/`post` | 0 % | 100 % |
| `command/engine.go:handleEmergencyStop`/`forwardMovementCommand` | 0 % | 100 % |
| `session/sfu_publisher.go` (`PublishSessionEvent`/`NewHTTPSFUPublisher`) | 0 % | 100 % |
| `session/manager.go:ListSessions` | 0 % | 100 % |
| `session/handover.go:issueHandoverToken` | 20 % | 100 % |

**Getestete Fallgruppen je Task** (Pflicht-Checkliste CLAUDE.MD §17): Grenzwerte (leere Listen,
leere Strings, keine Sessions/Vehicles), fehlerhafte Eingaben (malformed JSON), Fehlerpfade
(Netzwerkfehler, Timeout, non-2xx-Status via `httptest.Server`, DB-Fehler via `go-sqlmock` für
`PostgresVehicleStore`), Wiederholung/Idempotenz (`KickVehicle` zweimal, `handleEmergencyStop`
zweimal), Nebenläufigkeit (`-race`, u. a. `TestKickVehicle_ConcurrentCalls`,
`TestListSessions_ConcurrentWithStartSession`), Zugriffsgrenzen (OBSERVER darf keine
Movement-Commands senden, GOTEST-04). Neue Test-Dependency: `github.com/DATA-DOG/go-sqlmock`
(nur `go.mod`/`go.sum`, kein Produktivcode-Import) — ermöglicht Fehlerpfad-Tests gegen
`*sql.DB` ohne echten Postgres.

**Bewusst nicht abgedeckt:** `IsVehicleLocked`-Löschguard vor `vehicleregistry.Delete` (lebt als
HTTP-Handler-Logik in `cmd/control-server/main.go`, nicht im `vehicleregistry`-Paket selbst — außerhalb
des Datei-Scopes dieses Sprints). `session/manager.go`s übrige 0%-Funktionen
(`GetCurrentSession`/`HasActiveOperatorSession`/`IsVehicleLocked`/`EndSession`/`SaveCheckpoint`/
`LoadCheckpoint`/`GetSessionByVehicle`/`ActiveVehicleIDs`) — nicht Teil des GOTEST-05-Zuschnitts
(nur `ListSessions` explizit beauftragt), teils bereits indirekt über `tests/unit` abgedeckt.
`HandoverManager.CancelHandover`/`IsPending` — nicht Teil von GOTEST-06 (nur `issueHandoverToken`
beauftragt).

**Verifikation:** `go build ./...` sauber, `go vet ./...` sauber, `gofmt -l .` sauber. `go test
$(go list ./... | grep -v /tests/integration) -race -count=1` zweimal hintereinander grün
(alle Pakete inkl. der 5 neu getesteten). `tests/integration` erwartungsgemäß rot ohne laufenden
Docker-Teststack (`make test-integration` nötig, siehe `Makefile` — unverändertes Bestandsverhalten,
nicht Teil dieses Sprints). Einmaliger, unabhängiger Flake in `internal/fleetgateway` beobachtet
(1 von 4 Vollläufen) — Paket in diesem Sprint nicht angefasst, isoliert reproduzierbar immer grün;
vermutlich Port-/Ressourcenkonflikt unter paralleler Testlast, nicht behoben (außerhalb des Scopes).

---

Vorgänger: Sprint 50 ✅ (TelemetryWatchdog, DRIFT-K3-TELEMETRY Teil 2 — vollständig abgeschlossen),
siehe `tasks/sprints/50-telemetry-watchdog.md`.
