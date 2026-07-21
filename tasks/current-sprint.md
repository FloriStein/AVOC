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
| CIHARD-01 | `BenchmarkControlACKRoundtrip`-Skip-Bug beheben: `vehicle_id` auf `vehicle-int-mock` korrigieren, `session_id` aus der `/session/start`-Antwort parsen und an die WS-URL anhängen (`tests/performance/latency_test.go`). | S | 🔲 Backlog | — |
| CIHARD-02 | Neuer non-blocking CI-Job (`gosec` für Go + `npm audit` für Frontend), `continue-on-error: true` analog `lint.yml`-Muster. | S/M | 🔲 Backlog | — |
| CIHARD-03 | Neuer non-blocking CI-Job: `docker buildx build` je der 8 Build-Targets (6 Go-Services über `SERVICE_NAME`-Matrix + `frontend.Dockerfile` + `vehicle-mock.Dockerfile`), kein Push — verhindert "baut lokal, baut nicht in CI"-Drift. | S/M | 🔲 Backlog | — |
| CIHARD-05 | Verifikation: `make test-latency` läuft grün ohne Skip, neue Workflows lokal so weit wie möglich gegengeprüft (Job-Syntax, Docker-Build lokal reproduziert), Doku-Update (`tasks/backlog.md`, `DECISIONS.MD`). | S | CIHARD-01..03 |

**Nicht Teil dieses Sprints:** CIHARD-04 (`buf breaking`) bleibt Diskussionspunkt, kein
umzusetzender Task (siehe Vorrecherche-Gegenprüfung oben); Branch-Protection-Aktivierung
(CIGATE-06, bewusst zurückgestellt seit Sprint 41, betrifft alle PRs, eigene Nutzerbestätigung
nötig, unabhängig von diesem EPIC).

**Geschätzter Umfang:** 4 Tasks (CIHARD-01, -02, -03, -05), überwiegend S/M — reine
CI-Workflow-/Testfix-Arbeit ohne Docker-Stack-Start/-Stopp-Overhead wie in Sprint 54, daher
voraussichtlich unter dem ~200k-Token-Sprintbudget.

---

Vorgänger: Sprint 54 ✅ (Testabdeckungs-Gesamtaudit 2026-07-21, Teil 4 — E2E-Flow-Ausbau), siehe
[tasks/sprints/54-e2e-ausbau.md](sprints/54-e2e-ausbau.md).
