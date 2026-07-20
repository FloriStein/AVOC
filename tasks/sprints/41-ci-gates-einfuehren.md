> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Sprint 41 — CI-Gates einführen (ADR-006-Bestandsaufnahme)

**Freigabe (2026-07-19):** Nutzer hat am 2026-07-19 eine Testing-Strategie-Bestandsaufnahme gegen
ADR-006 (Testing Strategy) + CLAUDE.MD Abschnitt 17 angefordert. Größter gefundener Bruch
zwischen Dokumentation und Realität: `.github/workflows/` enthält ausschließlich `lint.yml`
(non-blocking, `continue-on-error: true`) — die von ADR-006 beschriebene Pipeline mit 4
blockierenden Gates (Unit Go+Frontend, Safety Test Suite, Integration, Latency) + 1 non-blocking
WebRTC-E2E-Job existiert nicht. Alle Prüfungen laufen ausschließlich manuell über bestehende
`Makefile`-Targets (`test`/`test-safety`/`test-integration`/`test-latency`/`test-k6`) — der
Kommentar in `test-safety` ("CI safety gate — must stay 19/19 green") ist irreführend, es gibt
kein CI dafür. Nutzerentscheidung: dieser Befund wird priorisiert vor zwei kleineren Audit-Funden
(Concurrency-Test-Lücke in `internal/webrtcsfu/sfu_test.go`, Safety-Test-Suite testet aktuell den
falschen Typ — bleiben offene Folgepunkte, siehe Sprint 42).

**Architektur-Entscheidung (bei der Planung getroffen, nicht mehr offen):**
- **Latenz-Gate (`test-latency`/`test-k6`) bewusst non-blocking**, obwohl ADR-006 es als
  "BLOCKING" dokumentiert: GitHub-gehostete Runner haben stark schwankende CPU-Zuteilung
  (Shared-Tenancy) — eine harte `<100ms`-Assertion würde auf einem verrauschten Runner Merges
  blockieren, ohne dass sich der Code geändert hat (False Positives). Gleiches Muster wie ADR-006s
  eigene Begründung für die WebRTC-Non-Determinism-Policy (non-blocking + sichtbar statt hartes
  Gate). Ergebnis bleibt sichtbar (Benchmark-Output als Job-Log/Artifact), blockiert aber keinen
  Merge. Verschärfung auf "blocking" ist ein möglicher Folge-Task, sobald genug CI-Läufe
  Rausch-Baseline zeigen.
- **Bestehender Playwright-Spec (`tests/e2e/dashboard.spec.ts`) wird als non-blocking
  Informational-Job eingebunden**, aber nicht inhaltlich vertieft — Ausbau der E2E-Tiefe ist ein
  eigener, größerer Folge-Task.
- **Branch-Protection-Aktivierung (Required Status Checks) nur vorbereitet, nicht scharf
  geschaltet** — das ist eine Repo-Einstellung, die alle zukünftigen PRs/Merges betrifft
  (geteiltes System), daher explizite Nutzerbestätigung vor Aktivierung nötig (siehe MB-Regeln zu
  risikoreichen/schwer umkehrbaren Aktionen).

**Vorrecherche (2026-07-19):**
- `Makefile:69-116`: `test` (`go test ./...` — läuft `tests/integration/...` **mit**, schlägt ohne
  laufenden Docker-Stack fehl), `test-safety` (`go test ./tests/unit/... -run Safety`, kein Docker
  nötig), `test-integration` (bringt `tests/docker-compose.test.yml`-Stack hoch, `go test
  ./tests/integration/...`, fährt Stack wieder runter), `test-latency` (Docker-Stack +
  `BenchmarkControlACKRoundtrip`, `b.Fatalf` bei p99>100ms), `test-k6` (Docker-Stack + k6 gegen
  `tests/performance/latency.js`, `thresholds: p(99)<100`).
- **`test` läuft für eine saubere CI-Unit-Gate nicht direkt verwendbar**, da es
  `tests/integration/...` ungefiltert mitnimmt — braucht einen neuen, zusätzlichen
  `test-unit`-Target (paketgefiltert, `go list ./... | grep -v /tests/integration`), ohne das
  bestehende `test`-Target zu ändern (Devs mit lokal laufendem Stack nutzen `test` weiterhin wie
  bisher).
- `.github/workflows/lint.yml` ist die einzige bestehende CI-Datei — dient als Stil-Vorlage
  (Trigger `on: push: branches: [main] / pull_request`, `actions/setup-go@v5`, Go 1.23).
- `tests/docker-compose.test.yml:1-4`: Kommentar bestätigt bewusst "Kein WebRTC/coturn — zu
  flaky in CI" — GitHub-Actions-Runner (`ubuntu-latest`) haben Docker Engine + `docker compose`
  v2 vorinstalliert, kein zusätzliches Setup nötig; Images werden aus Source gebaut
  (`build: context: ..`), erster CI-Lauf zeigt reale Build-Zeit (ggf. `timeout-minutes` setzen).
- `frontend/package.json`: `"test": "vitest run"` (Vitest, nicht Jest wie in ADR-006 benannt —
  funktional gleichwertig, keine Änderung nötig), `"test:e2e": "playwright test"`.
- `frontend/playwright.config.ts`: `testDir: '../tests/e2e'` → Spec liegt unter `tests/e2e/
  dashboard.spec.ts` (Repo-Root, nicht `frontend/tests/e2e/`), `baseURL: 'http://localhost:3000'`
  — Voraussetzung laut Spec-Kommentar: "Docker-Stack auf localhost:3000 läuft", also der volle
  Dev-Stack (`infrastructure/compose/docker-compose.yml`, Frontend-Port `3000:80`), nicht der
  schlanke Test-Stack (`tests/docker-compose.test.yml`, kein Frontend-Service).
  `infrastructure/compose/docker-compose.yml` erwartet eine `.env`-Datei (`make up --env-file
  .env`) — `.env.example` im Repo-Root committed, im CI-Job vor `docker compose up` nach `.env`
  kopieren.
- **ADR-006-Update nötig**: Abschnitt "Update (2026-07-15)" wird um einen zweiten Update-Absatz
  ergänzt (CI-Automatisierung Sprint 41, Latenz-Gate-Abweichung non-blocking begründet).

## Tasks

| ID | Task | Typ | Status |
|----|------|-----|--------|
| CIGATE-01 | `Makefile`: neuer `test-unit`-Target (paketgefiltert ohne `tests/integration`), bestehende Targets unverändert | S | 🔲 |
| CIGATE-02 | `.github/workflows/test-go.yml`: 3 blockierende Jobs — `unit` (`make test-unit`), `safety` (`make test-safety`), `integration` (`make test-integration`, Docker-Stack) | M | 🔲 |
| CIGATE-03 | `.github/workflows/test-frontend.yml`: Vitest-Unit-Tests blockierend (`npm ci && npm run test`) | S | 🔲 |
| CIGATE-04 | `.github/workflows/test-latency.yml`: Go-Benchmark + k6, bewusst non-blocking (`continue-on-error: true`, begründeter Kommentar analog `lint.yml`) | S/M | 🔲 |
| CIGATE-05 | Bestehenden Playwright-Spec als non-blocking Informational-Job einbinden (kein Ausbau der Testtiefe) | S | 🔲 |
| CIGATE-06 | Branch-Protection: Required-Status-Checks vorbereiten/dokumentieren (welche 4 Jobs), Aktivierung selbst erst nach expliziter Nutzerbestätigung (Repo-Setting, betrifft alle PRs) | S | 🔲 |
| CIGATE-07 | Verifikation: mind. 2 aufeinanderfolgende grüne CI-Läufe (Flakiness-Ausschluss, CLAUDE.MD §17), Timeout-/Resourcen-Anpassung falls nötig, Doku-Updates (`DECISIONS.MD`, ADR-006-Update-Absatz, `tasks/backlog.md`) | S | 🔲 |

**Nicht Teil dieses Sprints:** Latenz-Gate als hartes Blocking-Gate (siehe Architektur-
Entscheidung oben), Vertiefung der Playwright-E2E-Tests bzw. echte WebRTC-SDP/ICE-E2E-Automatisierung
(WebRTC bleibt bewusster Nicht-Scope, analog `WEBRTC-10`), Concurrency-Test-Lücke in
`internal/webrtcsfu/sfu_test.go` und weiteren Packages (eigener, kleinerer Folge-Task, Sprint 42),
Safety Test Suite inhaltlich auf den echten `safetyservice.Bus` ausrichten (separater Folge-Task,
Sprint 42).

Datum: 2026-07-19 | **Status: Abgeschlossen (lokal verifiziert, echte CI-Läufe stehen nach Push aus)**
Vorgänger: Sprint 40 (TLS/MQTTS-Härtung — läuft in paralleler Worktree, keine Code-Überschneidung
mit diesem Sprint).
Branch/Worktree: `feature/fleet-service-foundation-cigates` (Basis: `feature/fleet-service-foundation-sprint35`),
paralleler Worktree zu Sprint 42 (Testing-Debt, andere Dateien: `internal/webrtcsfu`, `tests/unit`)
— keine Code-Überschneidung, Doku-Dateien (`current-sprint.md`/`backlog.md`/`DECISIONS.MD`/
`done.md`) werden bewusst unabhängig in beiden Branches aktualisiert, manuelles Zusammenführen
erfolgt später.

## Ergebnisse

**CIGATE-01** — `Makefile`: `test-unit` ergänzt (`go test $(go list ./... | grep -v /tests/integration)`).
Bestehende Targets unverändert. Lokal 2× frisch (`-count=1`) grün, 16 Pakete, keine Failures.

**CIGATE-02** — `.github/workflows/test-go.yml`: 3 blockierende Jobs (`unit`, `safety`,
`integration`), `actions/setup-go@v5` Go 1.23 analog `lint.yml`. `unit` läuft zusätzlich
`go vet ./...` vor `make test-unit`. Timeouts: `unit`/`safety` 10 min, `integration` 20 min
(lokaler Kaltstart-Build ~55s, großzügiger Puffer für CI-Cold-Cache).

**CIGATE-03** — `.github/workflows/test-frontend.yml`: Vitest blockierend. Dabei entdeckt:
`npm run test` schlägt in einem frischen Checkout hart fehl (`Cannot find @/gen/control_pb.js`),
da `frontend/src/gen/` gitignored und nur build-time generiert wird (`make proto-gen-ts` oder der
Docker-Image-Build) — ADR-006/CI hatte diese Voraussetzung nirgends dokumentiert. Job generiert
daher vorher via `make proto-gen-ts` (Docker, kein lokaler `protoc` nötig). Zweiter Fund dabei:
dieser Docker-Lauf schreibt `frontend/node_modules` als root in den Bind-Mount zurück (identisches,
in `README.md` bereits für lokale Entwicklung dokumentiertes Problem, "Cannot find
@rollup/rollup-linux-x64-gnu") — vor dem host-seitigen `npm ci` wird `node_modules` daher über
einen zweiten `docker run` (gleiches Muster wie `README.md`s Troubleshooting-Fix) entfernt,
`package-lock.json` bleibt unangetastet (CI braucht ihn für `npm ci`). End-to-End lokal verifiziert
(2× frisch: 29 Testdateien / 288 Tests grün).

**CIGATE-04** — `.github/workflows/test-latency.yml`: 2 Jobs (`go-benchmark`, `k6`), beide
`continue-on-error: true` mit begründetem Kommentar. Dabei entdeckt und behoben: `make test-k6`
war vor diesem Sprint immer rot (`docker run` ohne `-i` — Skript-Inhalt kam nie im Container an,
`k6` meldete "no exported functions in script"). Fix: `-i` ergänzt (`Makefile`). Danach lokal 1×
sauberer Lauf (5820 Requests, 0% Fehlerrate, p95 http_req_duration ~1.1ms). Zweiter Fund, NICHT
behoben (Testverhalten, nicht CI-Wiring — siehe `DECISIONS.MD`): `BenchmarkControlACKRoundtrip`
skipt deterministisch (2× verifiziert), weil er `/ws` ohne den seit ADR-025 pflichtigen
`session_id`-Query-Parameter dialt — Server antwortet 400 vor jedem Verbindungsversuch. Non-blocking
Gate fängt das ab, aber der Benchmark liefert aktuell nie echte Latenzzahlen.

**CIGATE-05** — `.github/workflows/test-e2e.yml`: Playwright-Job, `continue-on-error: true`,
bringt den vollen Dev-Stack (`infrastructure/compose/docker-compose.yml`, 16 Services) hoch,
wartet auf `localhost:3000`, lädt Report als Artifact hoch. Dabei entdeckt (manuell per Chrome
DevTools MCP verifiziert): die App zeigt zuerst ein Login-Formular (Operator-ID/Passwort) —
nur die "AVOC"-Header-Assertion der bestehenden Spec trifft noch zu, die 4 Assertions für
Post-Login-Inhalt (IDLE-State, Safety-/Connection-Panel, Emergency-Stop-Button) schlagen
erwartbar fehl. Unkritisch (non-blocking), Spec-Ausbau bewusst außerhalb des Scopes (siehe
"Nicht Teil dieses Sprints").

**CIGATE-06** — `README.md` neuer Abschnitt "CI-Pipeline & Branch-Protection": Tabelle aller 5
Workflow-Dateien mit Blocking-Status, Liste der 4 als "Required" vorzusehenden Checks, sowie ein
Referenz-`gh api`-Befehl (nicht ausgeführt). Branch-Protection selbst nicht aktiviert.

**CIGATE-07** — Verifikation:
- `go build ./...` / `go vet ./...`: grün.
- `make test-unit`, `make test-safety`, `make test-integration`: je 2× frisch (`-count=1` bzw.
  neuer Docker-Stack) ausgeführt — durchgehend grün, keine Flakiness (Details siehe CIGATE-01..03
  oben; `test-integration` 2× je 31/31 Tests PASS).
- `frontend npm run test`: 2× frisch, 288/288 Tests grün.
- Alle 5 `.github/workflows/*.yml` mit `actionlint` (frisch installiert, `go install
  github.com/rhysd/actionlint/...@latest`) geprüft: 0 Findings.
- **Echte GitHub-Actions-Läufe:** nicht durchgeführt — kein Commit/Push in diesem Sprint
  (Nutzervorgabe). Die "2 grünen CI-Läufe" aus CLAUDE.MD §17 sind daher durch lokale
  Äquivalenz-Läufe ersetzt, siehe Hinweis in `tasks/backlog.md`. Erste echte Ausführung steht nach
  Push durch den Nutzer aus.
- Doku aktualisiert: `DECISIONS.MD` (Zeile CI-Gates auf ✅, 2 neue offene Folgepunkte-Zeilen),
  `docs/adr/006-testing-strategy.md` (neuer Update-Absatz 2026-07-20), `tasks/backlog.md`
  (EPIC + alle CIGATE-Zeilen), `README.md` (CI-Pipeline-Abschnitt).

**Bewusst unverändert gelassen (siehe "Nicht Teil dieses Sprints"):** Latenz-Gate als Blocking-Gate,
Playwright-Spec-Vertiefung/Login-Flow-Fix, `BenchmarkControlACKRoundtrip`-`session_id`-Fix,
Concurrency-Test-Lücke `webrtcsfu`, Safety-Test-Suite-Typkorrektur (beide Sprint 42).
