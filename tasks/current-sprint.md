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
| DRIFT60-01 | Backlog-Doku-Fix: `DRIFT-K5` in `tasks/backlog.md` von "🔲 Grill-Me ausstehend" auf "✅ Sprint 55" korrigieren (Status war real bereits erledigt, nur nie nachgezogen); `CIHARD-01`-Zeile korrigieren (behauptet fälschlich, auch K6 zu schließen). | S | 🔲 | — |
| DRIFT60-02 | Grill-Me `DRIFT-K4`: Entscheidung, ob `test-latency.yml`s `go-benchmark`-Job dauerhaft non-blocking bleibt (ADR-006-Abweichung formal in `DECISIONS.MD`/ADR-006 als dauerhaft festschreiben) oder jetzt — nach >30 Sprints ohne dokumentierten Noise-Vorfall — auf `continue-on-error: false` umgestellt wird. Bei Entscheidung "blocking": Branch-Protection-Erweiterung analog SEC-CI-05 als eigener Folgeschritt (Nutzerbestätigung nötig), nicht automatisch Teil dieses Tasks. | M | 🔲 | — |
| DRIFT60-03 | Grill-Me `DRIFT-K6`: Entscheidung + Umsetzung — `TestLatencyBudget_DocumentedRequirement` entweder durch einen echten Test ersetzen (z. B. Assertion direkt im Benchmark statt separatem tautologischen Test) oder ersatzlos streichen, falls der Benchmark selbst (`b.Fatalf` bei p99>100ms) die Anforderung bereits ausreichend verifiziert. | S/M | 🔲 | DRIFT60-02 (gleiche Datei, im selben Aufwasch sinnvoll) |
| DRIFT60-04 | Grill-Me `DRIFT-M18`: Entscheidung — OBSERVATION-Trigger "Auth-Service-down blockiert neue Sessions" produktiv implementieren (neuer Watchdog analog `AuthWatchdog`/`TelemetryWatchdog`, sicherheitsnah — volle §17-Testabdeckung) oder `CONTEXT.MD` auf den tatsächlichen Stand korrigieren (kein Produktivpfad). Nur die Entscheidung + ggf. reine Doku-Korrektur sind Teil dieses Sprints — eine volle Watchdog-Implementierung wäre eigener Typ-L-Folge-Task für Sprint 61 oder später. | M | 🔲 | — |
| DRIFT60-05 | Verifikation: `go build ./...`, `go vet ./...`, `gofmt -l` sauber; `make test-latency` weiterhin grün; Doku-Updates (`DECISIONS.MD`, `tasks/backlog.md` Drift-Audit-Fixes-Tabelle, ggf. ADR-006/`CONTEXT.MD`). | S | 🔲 | DRIFT60-01..04 |

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

_Noch offen — Sprint ist geplant, aber noch nicht umgesetzt._
