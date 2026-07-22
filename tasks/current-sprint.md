> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

## Kein aktiver Sprint

Sprint 56 (CI-Image-Publishing nach GHCR, 2026-07-22) ist vollständig abgeschlossen, siehe
[tasks/sprints/56-ci-image-publishing.md](sprints/56-ci-image-publishing.md) (CIPUB-01..03) und
den Nachtrag zu CIPUB-04 in `tasks/backlog.md` (EPIC "CI-Image-Publishing nach GHCR", Abschnitt
"Nachtrag CIPUB-04", 2026-07-22): Branch-Protection/Required-Status-Checks für `main` sind jetzt
real aktiv (4 Jobs, `enforce_admins=true`) — CIGATE-06 damit ebenfalls abgeschlossen.

**Möglicher nächster Sprint:** Für einen neuen Sprint siehe die übrigen offenen Punkte in
`tasks/backlog.md` — u. a. der reguläre Post-Merge-Verifikationsschritt aus CIPUB-03
(GHCR-Packages-Sichtbarkeit nach dem ersten echten Push auf `main` prüfen). Nutzer entscheidet
beim nächsten Sprint-Kickoff wie gewohnt.
