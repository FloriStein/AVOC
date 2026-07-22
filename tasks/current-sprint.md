> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Sprint 57 — GHCR-Pull-Deployment

Vollständiger Kontext: [tasks/backlog.md](backlog.md) EPIC "GHCR-Pull-Deployment (Sprint 57)".
Baut auf Sprint 56 (CI-Image-Publishing nach GHCR,
[tasks/sprints/56-ci-image-publishing.md](sprints/56-ci-image-publishing.md)) auf: die
`avoc-*`-Images existieren bereits in GHCR, das VM-Deployment (`ansible/deploy.yml`, lokaler
Build+`docker save`/`load`-Weg) nutzt sie bisher nicht — das war in Sprint 56 bewusst
ausgeklammert ("Nicht Teil dieses Sprints").

**Auftrag (2026-07-22):** Nutzer möchte den VM-Deploy-Workflow auf `commit → push → merge main
→ GitHub baut+pusht Images → VM pullt von GHCR` umstellen, statt die Images für jedes Deployment
nochmal lokal zu bauen.

**Nutzerentscheidungen (Rückfrage 2026-07-22):**
- **Fallback bleibt erhalten:** Lokaler Build+save/load-Weg wird nicht entfernt, bleibt über den
  bestehenden `SKIP_REGISTRY_PULL`-Mechanismus als Opt-out erreichbar. Nur der Default dreht sich
  um (GHCR-Pull wird Normalfall für die lokale Test-VM).
- **Versions-Tags werden mit eingeführt:** CI pusht bei echten Git-Tag-Pushes (z. B. `v1.2.3`)
  zusätzlich einen Versions-Tag nach GHCR, nicht nur `:latest`/`:<sha>`.

Datum: 2026-07-22 | Status: 🔲 geplant, noch nicht umgesetzt.

## Tasks

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| GHCRPULL-01 | **Nutzeraktion:** Least-Privilege GHCR-PAT anlegen — Classic PAT nur mit `read:packages`-Scope (Fine-grained-PATs unterstützen GHCR-Login/Packages-API nachweislich nicht zuverlässig). NICHT den `Administration`-Token aus CIPUB-04 wiederverwenden. | S | 🔲 | — |
| GHCRPULL-02 | `.github/workflows/docker-build.yml`: Tag-Push-Trigger (`on.push.tags: ['v*']`), zusätzlicher Versions-Tag `ghcr.io/<owner>/avoc-<service>:<git-tag>` bei echtem Tag-Push. `pull_request` bleibt `push: false`. | M | 🔲 | CIPUB-01 (Sprint 56) |
| GHCRPULL-03 | `infrastructure/compose/docker-compose.hetzner.yml`: die 8 `avoc-*`-Image-Referenzen auf `ghcr.io/<owner-lowercase>/avoc-<service>:${VERSION}` umstellen (Drittanbieter-Images unverändert). | S | 🔲 | GHCRPULL-02 |
| GHCRPULL-04 | `ansible/roles/secrets` (echten PAT einbinden, NICHT im Repo committen) + `ansible/deploy.yml`/`scripts/deploy-hetzner.sh`: `SKIP_REGISTRY_PULL`-Default umdrehen, `REGISTRY`/`DOCKER_USERNAME`/`DOCKER_PASSWORD` real befüllen. Fallback bleibt über Flag erreichbar. | M | 🔲 | GHCRPULL-01, GHCRPULL-03 |
| GHCRPULL-05 | Verifikation real gegen `avoc-local-vm` (libvirt/KVM, analog Sprint 49): Internet-Konnektivität VM→`ghcr.io`, GHCR-Pull-Weg UND Fallback-Weg je einmal real durchspielen, Doku-Updates. | M | 🔲 | GHCRPULL-04 |

**Nicht Teil dieses Sprints:** Umbenennung `DOCKER_USERNAME`/`DOCKER_PASSWORD` auf generische
Registry-Namen (kosmetisch, eigener Task falls gewünscht); echter Hetzner-Produktivserver
existiert noch nicht (Verifikation nur gegen lokale Test-VM); automatisches Rollback bei
fehlgeschlagenem Pull; Image-Retention-/Cleanup-Policies in GHCR.

## Ergebnis

_Noch offen — Sprint ist geplant, aber noch nicht umgesetzt._
