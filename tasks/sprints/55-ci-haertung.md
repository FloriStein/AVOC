> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

## Sprint 55 — Testabdeckungs-Gesamtaudit 2026-07-21, Teil 5 (CI-Härtung)

**Kickoff (2026-07-21):** Letzter Teil des EPICs — Teil 1 (Sprint 51), Teil 2 (Sprint 52), Teil 3
(Sprint 53) und Teil 4 (Sprint 54) sind abgeschlossen. Reine Sprint-Planung in diesem Commit —
Umsetzung folgt in einem eigenen, dedizierten Worktree analog Sprint 51-54
(`git worktree add ../controlcenter-aws-sprint55 -b feature/sprint55-ci-haertung main`).

**Vorrecherche-Gegenprüfung (2026-07-21):** Bestätigt gegen den aktuellen Code-/Workflow-Stand:
- Keiner der 5 bestehenden Workflows (`lint.yml`, `test-go.yml`, `test-frontend.yml`,
  `test-e2e.yml`, `test-latency.yml`) enthält `gosec`/`npm audit`/Trivy/CodeQL — Sicherheitsscanning
  fehlt komplett, wie im Backlog vermerkt.
- Kein Container-Build-Verifikationsjob existiert. Alle 6 Go-Services (`control-server`,
  `auth-service`, `safety-service`, `telemetry-service`, `webrtc-sfu`, `fleet-service`) teilen sich
  `infrastructure/docker/go-service.Dockerfile` über den `SERVICE_NAME`-Build-Arg
  (`infrastructure/compose/docker-compose.yml`), dazu kommen `frontend.Dockerfile` und
  `vehicle-mock.Dockerfile` (von `vehicle-mock`/`vehicle-mock-2` geteilt) — 8 Build-Targets gesamt.
- **Korrektur zur Backlog-Beschreibung von CIHARD-01:** Der Skip-Bug in
  `BenchmarkControlACKRoundtrip` (`tests/performance/latency_test.go`) ist **nicht nur** der
  fehlende `session_id`-Query-Parameter, sondern zusätzlich eine falsche `vehicle_id`: der
  Benchmark ruft `/session/start` mit `vehicle_id: "bench-vehicle"` auf, aber
  `tests/docker-compose.test.yml`s einziger vehicle-mock-Container läuft mit
  `VEHICLE_ID=vehicle-int-mock`. `handleSessionStart` (`cmd/control-server/main.go`) prüft
  `vehicleRegistry.Connected(req.VehicleID)` (`internal/vehicleconnection/registry.go` — rein
  WS-Verbindungs-keyed) **vor** dem eigentlichen `StartSession`-Aufruf und liefert für ein nie
  verbundenes `bench-vehicle` immer 409 "vehicle not connected" — die Session wird nie erstellt,
  `session_id` bleibt so oder so leer. Der Fix braucht beides: `vehicle_id` auf
  `"vehicle-int-mock"` korrigieren **und** `session_id` aus der `/session/start`-JSON-Antwort
  (`{"session_id":...,"role":...,"vehicle_id":...}`) parsen und an die WS-URL anhängen (statt wie
  bisher Response und Fehler zu verwerfen).
- CIHARD-04 (`buf breaking` für `proto/`) bleibt laut Backlog ein reiner Diskussionspunkt, kein
  fester Task — bei aktuell monolithischem Docker-Compose-Deployment (keine unabhängig deployten
  proto-Konsumenten) vorerst nicht eingeplant, siehe "Nicht Teil dieses Sprints" unten.

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| CIHARD-01 | `BenchmarkControlACKRoundtrip`-Skip-Bug beheben: `vehicle_id` auf `vehicle-int-mock` korrigieren, `session_id` aus der `/session/start`-Antwort parsen und an die WS-URL anhängen (`tests/performance/latency_test.go`). | S | ✅ | — |
| CIHARD-02 | Neuer non-blocking CI-Job (`gosec` für Go + `npm audit` für Frontend), `continue-on-error: true` analog `lint.yml`-Muster. | S/M | ✅ | — |
| CIHARD-03 | Neuer non-blocking CI-Job: `docker buildx build` je der 8 Build-Targets (6 Go-Services über `SERVICE_NAME`-Matrix + `frontend.Dockerfile` + `vehicle-mock.Dockerfile`), kein Push — verhindert "baut lokal, baut nicht in CI"-Drift. | S/M | ✅ | — |
| CIHARD-05 | Verifikation: `make test-latency` läuft grün ohne Skip, neue Workflows lokal so weit wie möglich gegengeprüft (Job-Syntax, Docker-Build lokal reproduziert), Doku-Update (`tasks/backlog.md`, `DECISIONS.MD`). | S | ✅ | CIHARD-01..03 |

**Nicht Teil dieses Sprints:** CIHARD-04 (`buf breaking`) bleibt Diskussionspunkt, kein
umzusetzender Task (siehe Vorrecherche-Gegenprüfung oben); Branch-Protection-Aktivierung
(CIGATE-06, bewusst zurückgestellt seit Sprint 41, betrifft alle PRs, eigene Nutzerbestätigung
nötig, unabhängig von diesem EPIC).

**Geschätzter Umfang:** 4 Tasks (CIHARD-01, -02, -03, -05), überwiegend S/M — reine
CI-Workflow-/Testfix-Arbeit ohne Docker-Stack-Start/-Stopp-Overhead wie in Sprint 54, daher
voraussichtlich unter dem ~200k-Token-Sprintbudget.

---

## Ergebnis (2026-07-21)

**CIHARD-01 — dritte Bug-Ursache erst bei der Verifikation entdeckt:** Die im Kickoff
beschriebenen zwei Ursachen (falsche `vehicle_id`, fehlender `session_id`-Query-Parameter) waren
beide korrekt, reichten aber nicht: nach dem Fix hing der Benchmark beim zweiten
Kalibrierungslauf des Go-Bench-Harness (`go test -bench` ruft `BenchmarkControlACKRoundtrip` bei
`-benchtime=10s` mehrfach mit steigendem `b.N` auf, bis die Zielzeit erreicht ist — jeder Aufruf
führt Login/`session/start`/WS-Dial erneut komplett aus) auf unbestimmte Zeit in
`conn.ReadMessage()` (beobachtet: `SIGQUIT: quit ... ran too long (11m0s)` nach 660s). Ursache:
die erste Session wurde nie über `/session/end` beendet, `vehicleController["vehicle-int-mock"]`
blieb belegt — `Manager.StartSession` (`internal/controlserver/session/manager.go:53`) vergibt für
ein bereits gesperrtes Fahrzeug die Rolle `OBSERVER` statt `ACTIVE_OPERATOR`; OBSERVER-Commands
werden nie ge-ACKt, `ReadMessage()` blockiert also dauerhaft. Fix: neuer `endSession`-Helper
(`POST /session/end` mit `session_id`), per `defer` direkt nach `startSession` registriert —
läuft dadurch auch bei `b.Fatalf` (via `runtime.Goexit`) zuverlässig. Schließt DRIFT-K5/K6
endgültig ab.

**Verifikation CIHARD-01/05 (Flakiness-Ausschluss §17 — 2×, jeweils frischer Docker-Stack):**
- Lauf 1: `PASS`, `BenchmarkControlACKRoundtrip-16  198902  58764 ns/op  p50=0-1ms p95=0-1ms
  p99=0-1ms` — kein Skip, `ok avoc/tests/performance 14.360s`.
- Lauf 2: `PASS`, `202474  58791 ns/op`, identisches Bild — `ok avoc/tests/performance 14.566s`.
- p99 in beiden Läufen weit unter dem 100ms-Budget (ADR-010), Assertion grün ohne Skip-Pfad.
- `go build ./...` und `go vet ./...` sauber (keine Regression durch die Testdatei-Änderung).

**CIHARD-02 — `.github/workflows/security-scan.yml`:** Zwei Jobs, beide `continue-on-error: true`
(analog `lint.yml`): `gosec` (Go, `securego/gosec@master`) und `npm audit` (Frontend). Lokal
gegengeprüft: `go run github.com/securego/gosec/v2/cmd/gosec@latest ./...` läuft durch und findet
44 Findings (durchweg `G104` unhandled-error bei `json.NewEncoder(w).Encode(...)`-Aufrufen,
informational) — bestätigt, dass der Job-Mechanismus gegen diesen Code funktioniert. `npm audit`
(im `frontend/`-Verzeichnis) findet 4 Schwachstellen (1 low, 3 high: `brace-expansion`, `esbuild`,
`js-yaml`, `undici`, alle in transitiven Dependencies) und beendet sich mit Exit-Code 1 — bestätigt
die Notwendigkeit von `continue-on-error: true`. Findings selbst wurden bewusst nicht behoben
(Scope dieses Tasks ist der CI-Job, nicht Dependency-Remediation — eigener Folge-Task falls
gewünscht, siehe "Bewusst nicht getestet/umgesetzt" unten).

**CIHARD-03 — `.github/workflows/docker-build.yml`:** Drei Jobs, alle `continue-on-error: true`,
kein `push`: `go-services` (Matrix über die 6 Go-Services, `go-service.Dockerfile` +
`SERVICE_NAME`-Build-Arg), `frontend` (`frontend.Dockerfile`), `vehicle-mock`
(`vehicle-mock.Dockerfile`, einmal gebaut, wie von `vehicle-mock`/`vehicle-mock-2` geteilt) — 8
Build-Targets gesamt wie vorrecherchiert. Lokal gegengeprüft: `docker buildx build` für
`control-server` (repräsentativ für die `go-service.Dockerfile`-Matrix), `frontend.Dockerfile` und
`vehicle-mock.Dockerfile` liefen alle drei erfolgreich durch (Kontext `.` = Repo-Root, wie in
`infrastructure/compose/docker-compose.yml` konfiguriert). Die übrigen 5 Go-Service-Targets wurden
zusätzlich indirekt über `docker compose -f tests/docker-compose.test.yml up --build` (Teil von
`make test-latency`, s.o.) gebaut. `actionlint` (lokal installiert) meldet für beide neuen
Workflow-Dateien keine Findings.

**CIHARD-04 — Gegengeprüft, bewusst weiterhin zurückgestellt:** Deployment ist unverändert
monolithisches `docker-compose` (kein unabhängig deploybarer proto-Konsument). Einzige Änderung
seit der Kickoff-Vorrecherche: `proto/` enthält jetzt 6 statt 5 `.proto`-Dateien — ändert nichts an
der Schlussfolgerung. Bleibt reiner Diskussionspunkt, kein Task.

**Bewusst nicht getestet/umgesetzt:**
- Die neuen Workflows liefen nicht auf einem echten GitHub-Actions-Runner (kein `act` installiert,
  kein Push in diesem Sprint) — Verifikation beschränkt sich auf `actionlint`-Syntaxprüfung plus
  manuelles Nachvollziehen der Kernbefehle jedes Jobs lokal (s.o.). Echte CI-Ausführung erst nach
  Push/PR sichtbar.
- `gosec`- und `npm audit`-Findings wurden nicht trianiert oder behoben (44 bzw. 4 Findings) — der
  Task war die Job-Einrichtung, nicht Remediation; beide Jobs sind bewusst informational
  (`continue-on-error: true`).
- CIHARD-04 (`buf breaking`) bewusst nicht umgesetzt (Diskussionspunkt, kein Task, s.o.).
- Branch-Protection-Aktivierung (CIGATE-06) weiterhin bewusst zurückgestellt, unabhängig von
  diesem EPIC.

**EPIC-Abschluss:** Mit Sprint 55 sind alle 5 Teile des EPICs "Testabdeckungs-Gesamtaudit
2026-07-21" abgeschlossen (Teil 1 Sprint 51, Teil 2 Sprint 52, Teil 3 Sprint 53, Teil 4 Sprint 54,
Teil 5 Sprint 55).

---

Vorgänger: Sprint 54 ✅ (Testabdeckungs-Gesamtaudit 2026-07-21, Teil 4 — E2E-Flow-Ausbau), siehe
[tasks/sprints/54-e2e-ausbau.md](sprints/54-e2e-ausbau.md).
