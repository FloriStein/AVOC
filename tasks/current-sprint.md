> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Sprint 56 — CI-Image-Publishing nach GHCR

Vollständiger Kontext: [tasks/backlog.md](backlog.md) EPIC "CI-Image-Publishing nach GHCR
(Sprint 56)". Baut auf CIHARD-03 (Sprint 55, [tasks/sprints/55-ci-haertung.md](sprints/55-ci-haertung.md))
auf — der bestehende `docker-build.yml`-Job baut bereits alle 8 Targets (6 Go-Services +
Frontend + vehicle-mock), aktuell nur zur Build-Verifikation (`push: false`).

**Auftrag (2026-07-22):** Nutzer möchte die `avoc-*`-Images zusätzlich in GitHub Actions bauen
lassen, statt sie ausschließlich lokal zu bauen (bisherige Voraussetzung für
`ansible/deploy.yml`, siehe EPIC "Lokale Ansible-VM als Hetzner-Nachbildung").

**Nutzerentscheidungen (Rückfrage 2026-07-22):**
- Registry: **GHCR** (`ghcr.io`), nicht Docker Hub.
- **Bestehenden Job in `.github/workflows/docker-build.yml` erweitern**, kein neuer separater
  Workflow.

Datum: 2026-07-22 | Status: 🔲 geplant, noch nicht umgesetzt.

## Tasks

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| CIPUB-01 | `.github/workflows/docker-build.yml` erweitern: `docker/login-action` gegen `ghcr.io` (`GITHUB_TOKEN`), `permissions: packages: write` auf Job-Ebene, `push: true` nur bei `push`-Event auf `main` (bei `pull_request` weiterhin `push: false`). Tag-Schema festlegen (z. B. `ghcr.io/<owner>/avoc-<service>:latest` + `:<git-sha>`). | M | 🔲 | CIHARD-03 (Sprint 55) |
| CIPUB-02 | Kurzer Hinweis-Kommentar in `ansible/deploy.yml`-Kopf und/oder `docs/deployment/UEBERGABE-ABWEICHUNGEN.md`, dass GHCR-Images jetzt existieren, ohne den bestehenden lokalen Build+`docker save`/`load`-Weg in diesem Sprint umzustellen. | S | 🔲 | CIPUB-01 |
| CIPUB-03 | Verifikation: `actionlint` gegen die geänderte Workflow-Datei, nach echtem Push auf `main` prüfen, dass die Packages tatsächlich unter GHCR erscheinen (Sichtbarkeit public/private dokumentieren), Doku-/Backlog-Status-Update. | S | 🔲 | CIPUB-01 |
| CIPUB-04 | **CIGATE-06 abschließen** (seit Sprint 41 vorbereitet, nie aktiviert): Branch-Protection/Required-Status-Checks für die 4 blockierenden Jobs real im GitHub-Repo-Setting einschalten — nur nach nochmaliger expliziter Nutzerbestätigung unmittelbar vor der Aktivierung. Auf Nutzerwunsch (2026-07-22) mitaufgenommen, thematisch unabhängig von CIPUB-01..03. | S | 🔲 | CIGATE-06 (Vorbereitung Sprint 41) |

**Nicht Teil dieses Sprints:** Umstellung von `ansible/deploy.yml` auf GHCR-Pull statt lokalem
Build+Transfer; Docker-Hub als Alternative; Image-Retention-/Cleanup-Policies in GHCR.

## Ergebnis

_Noch offen — Sprint ist geplant, aber noch nicht umgesetzt._
