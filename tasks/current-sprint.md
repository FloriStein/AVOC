> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Sprint 35 — GOSTYLE-IF-03/04 (Phase-2-Abschluss Interface-Segregation)

Ziel: Sprint 34 hat 4 der 6 GOSTYLE-IF-Tasks abgeschlossen (IF-01/02/05/06) und bewusst die beiden
größeren M-Tasks auf diesen Sprint verschoben (Token-Budget-Grill-Me 2026-07-18). Mit IF-03/04
ist Phase 2 (Interface-Segregation, Rule 4.2/4.3) des Go-Coding-Style-Guide-EPICs vollständig
abgeschlossen — keine weiteren Interface-Segregation-Folgetasks im Backlog offen.

**Explizites Nutzer-Budget:** wie Sprint 34 ca. 200.000 Token — bei nur 2 Tasks unkritisch, aber
beide sind Typ M (mehr Konsumenten, volle Teststandard-Pflicht nach CLAUDE.MD Abschnitt 17), daher
kein dritter Task ergänzt.

Vorrecherche (2026-07-18, vor Sprint-Start durchgeführt, damit die Tasks ohne erneute
Grill-Me-Session direkt umsetzbar sind):

- **GOSTYLE-IF-03** (`pkg/audit.AuditWriter`, 3 Methoden): `WriteSync` wird ausschließlich von 5
  Safety-/Command-Consumern gebraucht (`command.Engine`, `transport.WSHandler`,
  `vehiclecontext.Registry`, `safety.DeadmanWatchdog`, `safety.ACKTimeoutWatcher`,
  `safety.VehicleACKWatchdog` — alle über `WithAuditWriter(aw audit.AuditWriter)`). `QueryBySession`
  wird tatsächlich interface-typisiert gebraucht, aber von einem anderen Konsumenten:
  `controlServer.auditWriter` (`cmd/control-server/main.go:615`, `GET /audit/events`-Handler) —
  kein toter Methodenrest wie bei IF-01/02. `Close()` dagegen wird nur einmalig auf dem konkreten
  `*PostgresAuditWriter` in `newAuditWriter`s Closure aufgerufen (`cmd/control-server/main.go:88`),
  nie über das Interface — exakt das Bootstrap-Muster aus IF-05. Erwarteter Zuschnitt: schlankes
  `WriteSync`-only Interface für die 5 Consumer, `AuditWriter` (breiter, `WriteSync`+`QueryBySession`)
  bleibt für `newAuditWriter`s Rückgabetyp/`controlServer.auditWriter`, `Close` raus aus jedem
  Interface (bleibt konkrete Methode auf `PostgresAuditWriter`).
- **GOSTYLE-IF-04** (`authservice.UserStore`, 7 Methoden): `SeedAdmin` wird nur einmalig auf dem
  konkreten `*PostgresUserStore` in `cmd/auth-service/main.go:30` aufgerufen, bevor der Store als
  `UserStore`-Interface an `NewHandler` übergeben wird — nie über das Interface. Die verbleibenden 6
  Methoden (`Create`, `Authenticate`, `FindByID`, `List`, `Delete`, `UpdateRole`) sind alle über
  `h.userStore` (Interface-Feld in `authservice.Handler`) tatsächlich in Gebrauch — kein weiterer
  Schnitt nötig, nur `SeedAdmin` raus, analog IF-05.
  **Nebenbefund für die Umsetzung:** `internal/authservice/noop_userstore.go`s `NoopUserStore`
  scheint komplett ungenutzt (keine Referenz außerhalb der eigenen Datei gefunden) — beim
  Bearbeiten von IF-04 gegenprüfen, ob das stimmt; falls ja, als eigenen kleinen Fund im
  Sprint-Ergebnis vermerken (nicht unaufgefordert zusätzlich aufräumen, nur dokumentieren/im
  Anschluss dem Nutzer vorlegen — passt zum Sprint-34-Präzedenzfall `NoopVehicleStore.SeedDefault`).

**Keine Signaturänderung nach außen ohne Zweck** (CLAUDE.MD Abschnitt 15) — beide Tasks entfernen
lediglich Bootstrap-Methoden aus den jeweiligen Interfaces, kein API-/HTTP-Verhaltenswechsel. Kein
neues ADR nötig (reine Interface-Verschlankung, keine Datenstruktur-/Architekturänderung).
`docs/go-style-guide.md` gilt weiterhin für beide Tasks.

Datum: 2026-07-18 | **Status: Geplant, noch nicht begonnen**
Vorgänger: Sprint 34 ✅ (committed, Commit `6f50ba3`)
Branch/Worktree: noch nicht festgelegt — kann in einem neuen Worktree von `main`/diesem Branch aus
starten (Sprint 34 ist bereits committed, im Gegensatz zum Sprint-33/34-Übergang gibt es hier keine
uncommitted Abhängigkeit, die einen bestimmten Worktree erzwingt).

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| GOSTYLE-IF-03 | `pkg/audit.AuditWriter`: `WriteSync`-only Interface für die 5 Safety-/Command-Consumer, `Close` aus jedem Interface entfernen | M | ✅ |
| GOSTYLE-IF-04 | `authservice.UserStore`: `SeedAdmin` aus Interface lösen, verbleibende 6 Methoden bestätigt in Gebrauch | M | ✅ |

## Ergebnisse

- **GOSTYLE-IF-03**: Konsumenten per grep bestätigt (deckt sich mit Vorrecherche, keine
  Code-Änderung seither): `WriteSync` wird ausschließlich von `command.Engine`,
  `transport.WSHandler`, `vehiclecontext.Registry` und den 3 Watchdog-Structs in
  `internal/controlserver/safety/detector.go` (`DeadmanWatchdog`, `ACKTimeoutWatcher`,
  `VehicleACKWatchdog`) gebraucht, alle über `WithAuditWriter(...)`. Neues schlankes Interface
  `SafetyAuditWriter` (nur `WriteSync`) in `pkg/audit/writer.go` eingeführt und als Feld-/
  Parametertyp in allen 5 Consumern (`detector.go`, `engine.go`, `registry.go`, `websocket.go`)
  eingesetzt statt des bisherigen `audit.AuditWriter`. `AuditWriter` bleibt bestehen (jetzt via
  Embedding `SafetyAuditWriter` + `QueryBySession`) für `newAuditWriter`s Rückgabetyp und
  `controlServer.auditWriter` (`cmd/control-server/main.go:615`, `GET /audit/events`-Handler).
  `Close` aus beiden Interfaces entfernt; `PostgresAuditWriter.Close` bleibt konkrete Methode
  (weiterhin einmalig in der `newAuditWriter`-Bootstrap-Closure aufgerufen,
  `cmd/control-server/main.go:88`). `NoopWriter.Close` dadurch tot geworden und mit gelöscht
  (`pkg/audit/noop_writer.go`) — analog zum `NoopVehicleStore.SeedDefault`-Präzedenzfall aus
  Sprint 34. Neue Regressionstests in `pkg/audit/writer_test.go`: 4 Compile-Time-Checks
  (`var _ SafetyAuditWriter/AuditWriter = (*PostgresAuditWriter/NoopWriter)(nil)`) plus 2
  Verhaltenstests für `NoopWriter` (Zero-Value-Event, leere Session-ID) — `pkg/audit` hatte zuvor
  keine Tests.
- **GOSTYLE-IF-04**: Konsumenten per grep bestätigt: `SeedAdmin` wird nur einmalig auf dem
  konkreten `*PostgresUserStore` in `cmd/auth-service/main.go:30` aufgerufen, bevor der Store als
  `UserStore` an `NewHandler` übergeben wird (Zeile 35) — nie über das Interface. Die
  verbleibenden 6 Methoden sind alle über `h.userStore` in Gebrauch. `SeedAdmin` aus
  `UserStore`-Interface (`internal/authservice/userstore.go`) entfernt, bleibt konkrete Methode
  auf `PostgresUserStore`. Neuer Compile-Time-Check `internal/authservice/userstore_test.go`
  (`var _ UserStore = (*PostgresUserStore)(nil)`); bestehender `handler_test.go` deckt die 6
  verbleibenden Methoden bereits über `stubUserStore` ab (22 Tests, alle weiterhin grün).
  **Nebenbefund bestätigt:** `internal/authservice/noop_userstore.go`s `NoopUserStore` ist
  tatsächlich komplett ungenutzt — keine Referenz außerhalb der eigenen Datei, auch nicht in
  Tests. Anders als `NoopVehicleStore.SeedDefault` in Sprint 34 wurde `NoopUserStore` nicht erst
  durch diesen Task tot: alle 8 Methoden (inkl. der 6 weiterhin im Interface verbleibenden) waren
  bereits vorher unreferenziert. Scope-Grenze dieses Sprints (nur `SeedAdmin` war beauftragt) —
  bewusst **nicht** mit-gelöscht, stattdessen hier dokumentiert und als neuer Backlog-Eintrag
  vorgeschlagen (toter Code, eigener kleiner S-Task, unabhängig von GOSTYLE-IF-04).

**Verifikation gesamt**: `go build ./...`, `go vet ./...` sauber. `go test ./...` — alle Unit-/
Package-Tests grün (`pkg/audit`, `internal/authservice`, `internal/controlserver/transport`,
`internal/fleetgateway`, `internal/fleetservice`, `internal/vehicleconnection`, `pkg/db`,
`pkg/env`, `tests/unit`, `tests/performance`, `cmd/vehicle-mock`); `internal/controlserver/
{command,safety,vehiclecontext,session,statemachine}` haben weiterhin keine Testdateien (durch
diesen Sprint nicht verändert). `tests/integration` schlägt mit 29 `connection refused`-Fehlern
fehl, da kein docker-compose-Stack läuft — laut Sprint-Vorgabe für diesen rein
Backend-Interface-Task ohne Schema-/API-Verhaltensänderung erwartet und kein Blocker. Die drei
neu/verändert getesteten Pakete (`internal/authservice`, `internal/controlserver/transport`,
`pkg/audit`) 2x hintereinander mit `-count=1` gegen frischen Zustand laufen lassen (Flakiness-
Check gemäß CLAUDE.MD Abschnitt 17) — beide Durchläufe grün. Kein Frontend-/Browser-Check, da
keine der beiden Änderungen client-sichtbares Verhalten oder API-Response-Shapes ändert (reine
Interface-Verschlankung, gedeckt durch die "reine Backend-Interface-Arbeit"-Sprintvorgabe).
