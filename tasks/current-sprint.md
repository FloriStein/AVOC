> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Sprint 59 — CI-Sicherheits-Findings beheben (gosec + npm audit)

Vollständiger Kontext: [tasks/backlog.md](backlog.md) EPIC "CI-Sicherheits-Findings beheben —
gosec + npm audit (Sprint 59)".

**Auftrag (2026-07-22):** `gosec` (Go) und `npm audit` (Frontend) laufen seit Sprint 55
(CIHARD-02, `security-scan.yml`) als bewusst nicht-blockierende, informational Checks mit —
seitdem laufen sie aber bei praktisch jedem PR rot mit, ohne dass die Findings je behoben wurden.
Nutzer möchte das jetzt beheben.

**Findings-Snapshot (PR #9, Job-Logs vom 2026-07-22 — Ausgangspunkt, keine Garantie dass bei
Sprint-Start identisch, da laufend neuer Code/neue Dependencies dazukommen):**

`gosec` — 44 Issues gesamt:
- 32× `G104` (CWE-703) "Errors unhandled", Severity LOW — fast ausschließlich
  `json.NewEncoder(w).Encode(...)` in HTTP-Handlern über `control-server`, `safety-service` u. a.
- 6× `G114` (CWE-676) "Use of net/http serve function that has no support for setting timeouts",
  Severity MEDIUM — nacktes `http.ListenAndServe(...)` ohne Timeouts in allen 6 Go-Services
  (`cmd/auth-service`, `cmd/safety-service`, `cmd/webrtc-sfu`, `cmd/control-server`,
  `cmd/telemetry-service`, `cmd/fleet-service/main.go`).
- 1× `G304` (CWE-22) "Potential file inclusion via variable", Severity MEDIUM — genaue Fundstelle
  noch nicht lokalisiert, braucht eigene Analyse.

`npm audit` (`frontend/`) — 4 Vulnerabilities (3 high, 1 low), alle transitiv, alle laut
Tool-Output mit "fix available via `npm audit fix`": `brace-expansion` (DoS, via
`@typescript-eslint/typescript-estree`), `esbuild` 0.27.3–0.28.0 (Dev-Server-only, Windows),
`js-yaml` 4.0.0–4.2.0 (quadratischer CPU-Verbrauch), `undici` 7.0.0–7.27.2 (mehrere CVEs, u. a.
TLS-Bypass, Header-Injection, WebSocket-DoS).

Datum: 2026-07-22 | Status: 🔲 geplant, noch nicht umgesetzt.

## Tasks

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| SEC-CI-01 | `G104`-Findings beheben: alle `json.NewEncoder(w).Encode(...)`-Aufrufe in HTTP-Handlern — Encode-Fehler statt Ignorieren strukturiert loggen (Response ist zu dem Zeitpunkt bereits committet, kann nicht mehr sinnvoll per HTTP-Statuscode reagiert werden). | M | 🔲 | — |
| SEC-CI-02 | `G114`-Findings beheben: alle 6 `http.ListenAndServe(...)`-Aufrufe (`cmd/*/main.go`) auf explizites `http.Server{Addr:, Handler:, ReadHeaderTimeout:, ReadTimeout:, WriteTimeout:}` + `.ListenAndServe()` umstellen. Timeout-Werte einheitlich wählen (Vorschlag: 10s ReadHeaderTimeout, an bestehende Latenzbudgets aus `tests/performance` anlehnen). | M | 🔲 | — |
| SEC-CI-03 | `G304`-Finding lokalisieren + beheben (Pfad-Validierung/Whitelisting) oder als begründetes False-Positive mit `#nosec G304 -- <Begründung>`-Kommentar markieren. | S | 🔲 | — |
| SEC-CI-04 | `npm audit fix` in `frontend/` ausführen, Ergebnis verifizieren (`npm run build`, `npm test`, Vitest-Suite grün) — falls einzelne Findings einen Major-Bump brauchen, den jeweils isoliert bewerten (Breaking-Change-Risiko vor `--force`). | S | 🔲 | — |
| SEC-CI-05 | Beide Checks (`gosec (Go)`, `npm audit (Frontend)` in `security-scan.yml`) von informational auf required (Merge-Gate, analog `CIGATE-06`/Branch-Protection) umstellen, sobald SEC-CI-01..04 0 Findings liefern — verhindert stillen Rückfall in denselben Zustand. | S | 🔲 | SEC-CI-01, SEC-CI-02, SEC-CI-03, SEC-CI-04 |
| SEC-CI-06 | Doku-Update: `DECISIONS.MD` (warum informational → required), Backlog-Status, ggf. `docs/go-style-guide.md`-Hinweis auf `http.Server{}`-Timeout-Pflicht für neue Services. | S | 🔲 | SEC-CI-05 |

**Nicht Teil dieses Sprints:** Zusätzliche SAST-Tools über `gosec`/`npm audit` hinaus (z. B.
CodeQL/Trivy — separat zu entscheidende Erweiterung, analog CIHARD-04); automatisierte
Dependabot-Konfiguration (eigener, kleinerer Folge-Task falls gewünscht).

## Ergebnis

_Noch offen — Sprint ist geplant, aber noch nicht umgesetzt._
