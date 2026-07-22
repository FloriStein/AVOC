> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

## Kein aktiver Sprint

Sprint 57 (GHCR-Pull-Deployment, 2026-07-22) ist vollständig abgeschlossen, siehe
[tasks/sprints/57-ghcr-pull-deployment.md](sprints/57-ghcr-pull-deployment.md): CI pusht bei
`v*`-Tags Versions-Tags nach GHCR (GHCRPULL-02), `docker-compose.hetzner.yml` referenziert die
`avoc-*`-Images jetzt über `${REGISTRY}` (GHCRPULL-03), `ansible/deploy.yml` pullt per Default von
GHCR statt lokal zu bauen+`docker save`/`load` zu übertragen — der alte Weg bleibt als Fallback
über `avoc_skip_registry_pull=true` erhalten (GHCRPULL-04). Beide Wege real gegen `avoc-local-vm`
verifiziert (GHCRPULL-05), inkl. Health-Checks (HTTP 200 auf Frontend + Control-Server).

**Möglicher nächster Sprint:** Für einen neuen Sprint siehe die übrigen offenen Punkte in
`tasks/backlog.md` — u. a. `hetzner-setup.md`-Volltextumstellung (Docker-Hub-Sprache auf
Registry-agnostisch, bewusst nicht Teil von Sprint 57, s. dortiges "Nachtrag") sowie der reguläre
Post-Merge-Verifikationsschritt aus CIPUB-03 (GHCR-Packages-Sichtbarkeit). Nutzer entscheidet beim
nächsten Sprint-Kickoff wie gewohnt.
