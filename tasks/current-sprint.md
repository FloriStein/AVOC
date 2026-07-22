> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

## Kein aktiver Sprint

Sprint 56 (CI-Image-Publishing nach GHCR, 2026-07-22) ist abgeschlossen — CIPUB-01..03, siehe
[tasks/sprints/56-ci-image-publishing.md](sprints/56-ci-image-publishing.md). CIPUB-04
(Branch-Protection-Aktivierung) bleibt offen, blockiert durch fehlende
`Administration: Read and write`-Berechtigung des verfügbaren GitHub-Tokens.

**Möglicher nächster Sprint:** Für einen neuen Sprint siehe die übrigen offenen Punkte in
`tasks/backlog.md` — u. a. CIPUB-04 (Branch-Protection-Aktivierung, sobald ein Token mit
Administration-Berechtigung verfügbar ist) sowie der reguläre Post-Merge-Verifikationsschritt aus
CIPUB-03 (GHCR-Packages nach dem ersten echten Push auf `main` prüfen). Nutzer entscheidet beim
nächsten Sprint-Kickoff wie gewohnt.
