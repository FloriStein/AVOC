# Sprint 27 — Fundament: `pkg/db`, `pkg/env`, non-blocking Linter-Gate

Ziel: Erster von drei Sprints (27/28/29) zum Rollout des Go Coding Style Guide
(`docs/go-style-guide.md`, EPIC "Go Coding Style Guide Rollout" in `tasks/backlog.md`). Sprint 27
ist bewusst das Fundament: die beiden echten Dreifach-Duplikate aus der Bestandsaufnahme (DB-
Open+WaitForReady-Block, `envOr`-Helper + manuelle Required-Env-Var-Checks in `auth-`/`fleet-`/
`control-server`-`main.go`) werden in `pkg/db`/`pkg/env` gebündelt, bevor in Sprint 28/29 einzelne
`main()`-Funktionen zerlegt werden — unblockt beide Folge-Sprints und macht Fortschritt sofort
sichtbar (`golangci-lint` als Metrik ab Tag 1, auch wenn der Großteil des Codes noch nicht
konform ist).

Grill-Me (2026-07-17, EPIC-weit, nicht sprintspezifisch — siehe `tasks/backlog.md`): Umfang
komplette Codebasis/alle 4 Regelblöcke, Interface-Regeln (4.2/4.3) auf spätere, mit ADR-031
koordinierte Phase verschoben, Rules 1–3 + Duplikat-Extraktion laufen jetzt parallel zu den
laufenden Dashboard-Strängen (keine Signaturänderungen nach außen), `golangci-lint` bewusst
non-blocking (Henne-Ei-Problem: Großteil des Codes noch nicht angeglichen).

**Keine Signaturänderung nach außen, kein Verhaltenswechsel** (CLAUDE.MD Abschnitt 15) — jeder
Task endet mit vollem Testlauf der drei betroffenen Services + Diff-Review gegen genau diese
Vorgabe. Explizit nicht Teil dieses Sprints: `main()`-Zerlegungen (Sprint 28/29), Interface-
Änderungen (Phase 2, wartet auf ADR-031/HEX-05).

Datum: 2026-07-17 | **Status: Alle Tasks ✅**
Vorgänger: Sprint 25 ✅ (dieser Branch zweigt vom bereits gemergten `feature/fleet-service-
foundation`-Stand ab, unabhängig von parallelen Sessions in anderen Worktrees)
Branch: `feature/fleet-service-foundation-gostyle`

---

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| GOSTYLE-01 | `pkg/db`: `OpenAndWait`-Helper ergänzen (bündelt Open+WaitForReady+Degraded-Warn-Block, aktuell 3× identisch in `auth-`/`fleet-`/`control-server`-`main.go`) + alle 3 Call-Sites umstellen | M | ✅ |
| GOSTYLE-02 | Neues `pkg/env`: `Require`/`OptionalOr`-Helper (ersetzt 3× identisches `envOr` + 3× manuellen Required-Var-Check in `auth-`/`fleet-`/`control-server`-`main.go`) | M | ✅ |
| GOSTYLE-15 | `golangci-lint` einrichten (`funlen` max-func-lines=50, `revive` argument-limit=4) als **non-blocking Warn-Stufe** im CI (kein Merge-Gate) | M | ✅ |

## Ergebnisse

**GOSTYLE-01 — `pkg/db.OpenAndWait` ✅**
Neue Funktion `OpenAndWait(databaseURL string, log *logger.Logger, degradedModeMsg string) *sql.DB`
in `pkg/db/postgres.go`, bündelt `Open` + `WaitForReady(db, DefaultConnectRetries,
DefaultConnectRetryDelay)` + die Degraded-Mode-Warnung. `degradedModeMsg` bleibt Parameter statt
fest codiert, weil sich der Warntext zwischen den drei Services unterscheidet
(`control-server`: "...— starting in degraded mode", `auth-`/`fleet-service`: "...— proceeding
anyway") — CLAUDE.MD Abschnitt 15 verlangt identische Log-Ausgaben, nicht nur identisches
Verhalten. Der `Open`-Fehlerfall bleibt `log.Fatal("failed to open database", ...)`, war an allen
drei Call-Sites bereits wortgleich. Alle drei `main.go` umgestellt auf `db :=
pkgdb.OpenAndWait(databaseURL, log, "...")`; die service-übergreifende Erklärung zum Docker-
`restart: unless-stopped`-Problem (Sprint 18 Bugfix) ist jetzt zentral als GoDoc auf
`OpenAndWait` dokumentiert statt dreifach (mit leicht unterschiedlichem Wortlaut) im Kommentar an
jeder Call-Site — reine Lesbarkeits-Konsolidierung, keine inhaltliche Änderung.

**GOSTYLE-02 — `pkg/env` ✅**
Neues Paket mit `OptionalOr(key, fallback string) string` (identisch zum bisherigen `envOr`) und
`Require(key string, log *logger.Logger) string` (loggt `"<KEY> environment variable is
required"` und beendet den Prozess über `log.Fatal`, falls leer/unset — alle bisherigen manuellen
Checks folgten exakt diesem Nachrichtenformat, daher reiner 1:1-Ersatz). `auth-service` hatte
`envOr` bisher gar nicht als Funktion (nur ein einzelner Inline-Check für `AUTH_PORT`) — auch
dieser Fall wird jetzt konsistent über `env.OptionalOr` gelöst. Lokale `envOr()`-Definitionen in
`fleet-service` und `control-server` entfernt (jetzt unbenutzt); `vehicle-mock`s eigenes `envOr`
bewusst unangetastet gelassen (nicht Teil des Aufgabenumfangs dieser Sitzung, siehe Sprint 28
GOSTYLE-08).

**GOSTYLE-15 — `golangci-lint` non-blocking ✅**
`.golangci.yml` (nur `funlen` max 50 Zeilen + `revive` `argument-limit` max 4, alle anderen Linter
deaktiviert) plus `.github/workflows/lint.yml` (`continue-on-error: true` auf Job-Ebene **und**
`--issues-exit-code=0` als Tool-Flag — doppelt abgesichert non-blocking, damit der Check auch bei
Findings grün statt "fehlgeschlagen, aber ignoriert" anzeigt). Kein bereits existierender
`.github/workflows/`-Ordner im `-cicd`-Worktree zum Zeitpunkt dieser Sitzung gefunden (geprüft vor
Beginn) — daher eigenständige Datei angelegt statt in eine bestehende Pipeline eingehängt, wie in
der Aufgabenstellung als Fallback vorgesehen. Konfiguration lokal gegen den echten Code
verifiziert (`golangci-lint v1.64.8` installiert, `golangci-lint run --issues-exit-code=0`
ausgeführt): Ergebnis deckt sich mit der quantifizierten Bestandsaufnahme aus `tasks/backlog.md`
(6 Funktionen mit >4 Parametern in Produktionscode — `bus_watchdog.go`, `memory_recorder.go` ×3,
`vehicleconnection/handler.go`, `vehicle-mock/main.go` — plus mehrere `main()`-Funktionen und
weitere Methoden >50 Zeilen, alles bereits als Sprint-28/29-Arbeit im Backlog vorgemerkt).
YAML-Syntax mit `actionlint` geprüft (keine Fehler).

**Verifikation (alle drei Tasks zusammen) ✅**
`go build ./...`, `go vet ./...` sauber. `gofmt -l .` findet weiterhin genau die bereits vor
diesem Sprint bekannten 13 unformatierten Dateien (siehe `tasks/backlog.md`, Bonus-Task
außerhalb des Style-Guide-Scopes) — keine davon durch diese Änderungen neu hinzugekommen (geprüft
per `gofmt -d` gegen jede der drei editierten Dateien: alle Abweichungen liegen an unberührten,
bereits vorher unformatierten Zeilen). `go test ./...`: alle Unit-Tests grün, neue Tests für
`pkg/db.OpenAndWait` (implizit über die bestehenden `WaitForReady`-Tests) und `pkg/env` (4 neue
Tests, `OptionalOr` beide Zweige + leerer String, `Require`-Erfolgsfall — der `Fatal`/`os.Exit`-Pfad
ist wie bei `pkg/logger` bereits zuvor nicht in-process testbar). Zusätzlich vollständiger
`make test-integration`-Lauf gegen den echten Docker-Test-Stack (`tests/docker-compose.test.yml`)
für genau die drei geänderten Services: alle Tests grün (3 vorbestehende WebSocket-Skips, unrelated
— minimaler Test-Stack hat kein echtes WebRTC/SFU), inklusive Health-Checks, JWT/DB-Required-Var-
Checks und Operator-Login — bestätigt, dass `env.Require`/`pkgdb.OpenAndWait` in echten
Container-Startups identisch funktionieren wie die vorherigen Inline-Blöcke.

**Bewusst nicht in diesem Sprint:** keine `main()`-Zerlegungen (Sprint 28/29), keine Interface-
Änderungen (Phase 2, wartet auf ADR-031/HEX-05), kein `gofmt -w .` (separater Bonus-Task,
unabhängig vom Style-Guide-Scope), `vehicle-mock`s `envOr`/Parameter-Duplikate unangetastet
(Sprint 28 GOSTYLE-08).
