> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Sprint 60 — Drift-Audit-Restposten Teil 1 (Grill-Me-Entscheidungen)

Vollständiger Kontext: [tasks/backlog.md](backlog.md) EPIC "Drift-Audit-Fixes (Sprint 26,
`docs/drift-audit-2026-07.md`)". Erster von zwei Sprints, die die seit Sprint 26 offenen
`DRIFT-K*`/`DRIFT-M18`-Punkte abschließen (Sprint-59-Nachbesprechung, 2026-07-22 —
Nutzerentscheidung: aufteilen statt alles in einen Sprint zu packen, Begründung Token-Budget).

**Auftrag (2026-07-22):** Die drei kritischsten offenen Drift-Findings (`DRIFT-K4`, `DRIFT-K6`,
`DRIFT-M18`) sind laut `CLAUDE.MD` §0 nicht direkt fixbar — jedes braucht erst eine eigene
Grill-Me-Session (Entscheidung, ggf. ADR-Bezug), bevor überhaupt Code angefasst wird. Dieser Sprint
liefert die drei Entscheidungen plus die Umsetzung, sofern sie klein genug ausfällt (S/M) —
größere Folge-Implementierungen wandern nach Sprint 61. Zusätzlich ein trivialer Backlog-Doku-Fix
(`DRIFT-K5`), der beim Vorrecherchieren dieses Sprints auffiel.

**Vorrecherche (2026-07-22, gegen echten Code verifiziert statt nur den Backlog-Stand zu
übernehmen):**
- **DRIFT-K4** — `.github/workflows/test-latency.yml` existiert bereits (seit Sprint 41) und führt
  den ACK-Roundtrip-Benchmark bei jedem Push/PR aus, ist aber laut eigenem Kopfkommentar bewusst
  `continue-on-error: true` — Abweichung von ADR-006s "BLOCKING", Begründung: Shared-Runner-
  CPU-Rauschen auf GitHub-hosted Runnern würde harte <100ms-Assertions an Runner-Rauschen statt
  echten Regressionen scheitern lassen. Der Kommentar selbst nennt "Tightening to blocking is a
  possible follow-up once enough CI runs establish a noise baseline" — seit Sprint 41 (2026-07-20)
  sind inzwischen >30 Sprints/PRs durch diese Pipeline gelaufen. Das Finding "existiert nicht als
  Pipeline" ist damit **überholt** (sie existiert), die eigentliche offene Frage ist: reicht die
  informational Variante dauerhaft, oder wird jetzt auf blocking umgestellt?
- **DRIFT-K5** — `tests/backlog.md` listet dies noch als "🔲 Grill-Me ausstehend", aber Sprint 55
  (`CIHARD-01`) hat den `BenchmarkControlACKRoundtrip`-Skip-Bug real behoben — `make test-latency`
  läuft seitdem nachweislich 2× hintereinander grün ohne Skip (siehe
  `tasks/sprints/55-ci-haertung.md`). Reiner Backlog-Pflegefehler, kein offener Code-Task.
- **DRIFT-K6** — `tests/performance/latency_test.go`s `TestLatencyBudget_DocumentedRequirement`
  wurde gegengelesen: prüft ausschließlich `if latencyBudget != 100*time.Millisecond`, also dass
  eine Konstante sich selbst nicht geändert hat — tautologisch, wie im Finding beschrieben.
  Sprint 55s Backlog-Zeile zu `CIHARD-01` behauptet fälschlich "schließt DRIFT-K5/K6 endgültig
  ab" — das stimmt nur für K5, K6 ist nach wie vor unbehoben (Backlog-Fehler, nicht nur veraltet).
- **DRIFT-M18** — `CONTEXT.MD` (Zeilen 36/102) dokumentiert OBSERVATION als eigene Stufe ("Auth
  Service down (bestehende Session) → neue Sessions blockiert, JWT-Validierung lokal weiter"),
  aber `grep -rn "OBSERVATION\|StateObservation"` über `internal/`/`cmd/` findet **keine**
  Code-Referenz außer einem Kommentar in `internal/controlserver/authcheck/checker.go:8`, der
  selbst nur auf `CONTEXT.MD` verweist. Bestätigt: reine Dokumentations-Behauptung ohne
  Produktivpfad — Grill-Me muss entscheiden, ob implementiert oder die Doku korrigiert wird.

## Tasks

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| DRIFT60-01 | Backlog-Doku-Fix: `DRIFT-K5` in `tasks/backlog.md` von "🔲 Grill-Me ausstehend" auf "✅ Sprint 55" korrigieren (Status war real bereits erledigt, nur nie nachgezogen); `CIHARD-01`-Zeile korrigieren (behauptet fälschlich, auch K6 zu schließen). | S | ✅ | — |
| DRIFT60-02 | Grill-Me `DRIFT-K4`: Entscheidung, ob `test-latency.yml`s `go-benchmark`-Job dauerhaft non-blocking bleibt (ADR-006-Abweichung formal in `DECISIONS.MD`/ADR-006 als dauerhaft festschreiben) oder jetzt — nach >30 Sprints ohne dokumentierten Noise-Vorfall — auf `continue-on-error: false` umgestellt wird. Bei Entscheidung "blocking": Branch-Protection-Erweiterung analog SEC-CI-05 als eigener Folgeschritt (Nutzerbestätigung nötig), nicht automatisch Teil dieses Tasks. | M | ✅ | — |
| DRIFT60-03 | Grill-Me `DRIFT-K6`: Entscheidung + Umsetzung — `TestLatencyBudget_DocumentedRequirement` entweder durch einen echten Test ersetzen (z. B. Assertion direkt im Benchmark statt separatem tautologischen Test) oder ersatzlos streichen, falls der Benchmark selbst (`b.Fatalf` bei p99>100ms) die Anforderung bereits ausreichend verifiziert. | S/M | ✅ | DRIFT60-02 (gleiche Datei, im selben Aufwasch sinnvoll) |
| DRIFT60-04 | Grill-Me `DRIFT-M18`: Entscheidung — OBSERVATION-Trigger "Auth-Service-down blockiert neue Sessions" produktiv implementieren (neuer Watchdog analog `AuthWatchdog`/`TelemetryWatchdog`, sicherheitsnah — volle §17-Testabdeckung) oder `CONTEXT.MD` auf den tatsächlichen Stand korrigieren (kein Produktivpfad). Nur die Entscheidung + ggf. reine Doku-Korrektur sind Teil dieses Sprints — eine volle Watchdog-Implementierung wäre eigener Typ-L-Folge-Task für Sprint 61 oder später. | M | ✅ | — |
| DRIFT60-05 | Verifikation: `go build ./...`, `go vet ./...`, `gofmt -l` sauber; `make test-latency` weiterhin grün; Doku-Updates (`DECISIONS.MD`, `tasks/backlog.md` Drift-Audit-Fixes-Tabelle, ggf. ADR-006/`CONTEXT.MD`). | S | ✅ | DRIFT60-01..04 |

**Nicht Teil dieses Sprints (→ Sprint 61, s. `tasks/backlog.md`):** `DRIFT-M19` (k6-Latenztest auf
echten WS-ACK-Roundtrip), `DRIFT-M20` (Video-Latenzziel-Test/Doku), `DRIFT-M21` (`Doku.md`
EC2-Deployment `fleet-service`-Ergänzung), `DRIFT-M22` (`-race` strukturell in Makefile
verankern), `DRIFT-M24` (`OperatorRole` als Typ), `DRIFT-M25` (Task-Status als Typ in
`fleetservice/store.go`) — bewusst nach den Sprint-60-Entscheidungen verschoben, da `M22`
inhaltlich von `DRIFT60-02`/`-03` abhängt (dieselbe Latenz-/Benchmark-Baustelle) und eine volle
`DRIFT-M18`-Watchdog-Implementierung (falls so entschieden) den Sprint sonst sprengen würde.
Ebenfalls nicht Teil: `DRIFT-AP3` (neues EPIC „AP3" anlegen, strukturell, kein Drift-Fix),
`CIHARD-04` (`buf breaking`, weiterhin offener Diskussionspunkt ohne neuen Anlass).

## Ergebnis

**DRIFT60-01:** `tasks/backlog.md` korrigiert — `DRIFT-K5` von "🔄 Sprint 60 geplant" auf "✅
Sprint 55" (war real bereits durch `CIHARD-01` behoben, nur nie nachgezogen); `CIHARD-01`-Zeile
korrigiert, behauptet jetzt nur noch, K5 (nicht auch K6) zu schließen.

**DRIFT60-02 — Grill-Me `DRIFT-K4`, Entscheidung: blocking.** Vor der Entscheidung echte
CI-Historie via `github`-MCP geprüft (`mcp__github__actions_list`/`get_job_logs`, nicht nur den
Backlog-Stand übernommen): 47 aufgezeichnete Läufe von `test-latency.yml` seit Sprint 41, **alle**
grün, gemessenes p99 in jeder Stichprobe (aktuellster + mehrere historische Läufe, u. a. vom
20./21./22.07.) durchgängig **0-1ms** bei einem 100ms-Budget — ~100-facher Puffer. Der
Rausch-Vorfall, den die Non-blocking-Entscheidung von Sprint 41 befürchtete, ist nie eingetreten.
`go-benchmark`-Job in `.github/workflows/test-latency.yml` auf blocking umgestellt
(`continue-on-error` entfernt), `k6`-Job bleibt unabhängig davon non-blocking. ADR-006
Update-Block, `DECISIONS.MD`, Backlog-Zeile aktualisiert. Branch-Protection
(Required-Status-Check) bewusst **nicht** aktiviert — eigener Folgeschritt, Nutzerbestätigung
nötig.

**DRIFT60-03 — Grill-Me `DRIFT-K6`, Entscheidung: ersatzlos streichen.**
`TestLatencyBudget_DocumentedRequirement` prüfte nur `latencyBudget != 100*time.Millisecond` —
eine Konstante gegen sich selbst, keine echte Testabdeckung. Der Benchmark selbst
(`BenchmarkControlACKRoundtrip`s `b.Fatalf` bei p99>100ms, ADR-010) verifiziert die eigentliche
Anforderung bereits vollständig. Test in `tests/performance/latency_test.go` entfernt, keine
Ersatzassertion nötig. Verifiziert: `make test-latency` läuft weiterhin grün (s. u.).

**DRIFT60-04 — Grill-Me `DRIFT-M18`, Entscheidung: Doku korrigieren, nicht implementieren.**
Vor der Entscheidung per Explore-Subagent gegen echten Code verifiziert: kein `SystemState`-Wert,
keine Transition, kein Handler-Zweig für OBSERVATION irgendwo in `internal/`/`cmd/`; `/session/
start` validiert JWTs rein lokal (HMAC-Secret) und ruft auth-service nie über HTTP auf —
„neue Sessions blockiert, wenn auth-service down" hatte nie einen Produktivpfad. Der real
relevante Fall (Operator-Account wird während laufender Session ungültig) ist bereits durch
`AuthWatchdog`/CRITICAL abgedeckt — dessen DB-Direktzugriff-Design wurde 2026-07-16 explizit
gewählt, *um* eine Kopplung an OBSERVATION zu vermeiden (`authcheck/checker.go`-Kommentar). Eine
echte OBSERVATION-Implementierung wäre eine neue sicherheitsrelevante Abhängigkeit ohne bisherigen
Betriebsanlass. `CONTEXT.MD` (Glossar + Failure-Classification-Tabelle) korrigiert, ADR-009
Update-Block, Backlog-Zeile aktualisiert.

**Gemini-mcp Sub-Agent (zweite Meinung, DRIFT60-02/-04):** Für beide Architektur-Trade-offs wurde
versucht, den neu verbundenen `gemini-mcp`-Server (`gemini_subagent_start`/`gemini_brainstorm`)
als zweite Meinung einzuholen. Ergebnis: **nicht verfügbar** — erster Versuch scheiterte an
Free-Tier-Rate-Limit (429), der Retry sowie ein direkter synchroner `gemini_brainstorm`-Aufruf
scheiterten beide an einem serverseitigen Fallback-Bug (404 `models/gemini-1.5-flash is not
found`, unabhängig vom angeforderten `model_override`). Beide Entscheidungen beruhen stattdessen
auf eigenständig verifizierter Evidenz (CI-Lauf-Historie via `github`-MCP für K4, Code-Grep +
Explore-Subagent für M18) — kein Blocker, aber der `gemini-mcp`-Server selbst braucht
Aufmerksamkeit (Modell-Fallback-Kette zeigt auf ein nicht mehr existierendes Modell).

**DRIFT60-05 — Verifikation:** `go build ./...` sauber, `go vet ./...` sauber, `gofmt -l` ohne
Findings. `make test-latency`: `PASS`, `BenchmarkControlACKRoundtrip-16  189322  63052 ns/op
p50=0s p95=0s p99=0s (budget=100ms)`, `ok avoc/tests/performance 14.655s` — kein Skip, keine
Regression durch das Streichen des tautologischen Tests. Doku-Updates: `DECISIONS.MD` (ADR-006/
ADR-009-Zeilen), `tasks/backlog.md` (Drift-Audit-Fixes-Tabelle, 4 Zeilen), ADR-006/ADR-009
Update-Blöcke, `CONTEXT.MD`.

**Bewusst nicht getestet/umgesetzt:**
- Branch-Protection-Aktivierung für den jetzt blockierenden `go-benchmark`-Job — eigener
  Folgeschritt, Nutzerbestätigung nötig (s. DRIFT60-02).
- Volle `DRIFT-M18`-OBSERVATION-Implementierung wurde bewusst nicht gebaut (Grill-Me-Entscheidung
  gegen Implementierung, s. o.) — kein Folge-Task, da die Doku-Korrektur als abschließend gilt.
- CI-Verifikation der `test-latency.yml`-Änderung lief nicht auf einem echten GitHub-Actions-Run
  (kein Push in diesem Sprint) — echtes Verhalten erst nach Push/PR sichtbar.
