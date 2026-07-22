> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Sprint 58 — Continuous Deployment für die lokale VM

Vollständiger Kontext: [tasks/backlog.md](backlog.md) EPIC "Continuous Deployment für die lokale
VM (Sprint 58)". Baut auf Sprint 57 (GHCR-Pull-Deployment,
[tasks/sprints/57-ghcr-pull-deployment.md](sprints/57-ghcr-pull-deployment.md)) auf: die VM pullt
zwar schon von GHCR, aber nur wenn `ansible-playbook deploy.yml` manuell angestoßen wird.

**Auftrag (2026-07-22):** Nutzer möchte, dass neu in GHCR gebaute Images automatisch auf
`avoc-local-vm` landen — `push nach main → CI baut+pusht Images → automatischer Deploy`, ohne
manuellen Schritt.

**Kritische Randbedingung:** Das AVOC-Repo ist **public**. GitHub-gehostete Runner erreichen
`avoc-local-vm` nicht (kein öffentliches IP, bewusst keine Portweiterleitung) — ein Deploy-Schritt
muss auf einem **Self-hosted Runner** auf dem Dev-Rechner laufen. Self-hosted Runner + public Repo
ist ein bekanntes Sicherheitsrisiko (beliebige PR-Autoren könnten sonst Code auf dem Runner
ausführen). Zwingende Mitigation: Deploy-Job läuft **ausschließlich** auf `push` nach `main`
(niemals `pull_request`/`pull_request_target`), Repo-Setting "Require approval for all outside
collaborators" muss aktiv sein, Deploy-Job bleibt getrennt von den weiterhin GitHub-gehosteten
Build-/Test-Jobs.

**Nutzerentscheidungen (Rückfrage 2026-07-22):**
- **CD-Ziel:** Nur die lokale Test-VM. Kein echter Hetzner-Server in diesem Sprint (existiert
  noch nicht, eigenes größeres Vorhaben).
- **Mechanismus:** Self-hosted GitHub Actions Runner (native Actions-Integration) statt
  losgelöstem lokalem Poll-/Cron-Skript.
- **Trigger:** Automatisches Deployment bei jedem Push nach `main` (echtes CD, kein manuelles
  Freigabe-Gate über GitHub Environments).

Datum: 2026-07-22 | Status: 🔲 geplant, noch nicht umgesetzt.

## Tasks

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| CD-01 | **Nutzeraktion (mit Anleitung):** Self-hosted Runner auf dem Dev-Rechner registrieren und als systemd-Service einrichten. Vorher prüfen: Repo-Setting "Require approval for all outside collaborators" aktiv. Runner-Scope: nur dieses Repo. | M | 🔲 | — |
| CD-02 | Neuer Workflow-Job `deploy-local-vm` (`.github/workflows/deploy-local-vm.yml`): `runs-on: self-hosted`, Trigger **nur** `push: branches: [main]`, `needs:`/`workflow_run` nach den Build-Jobs, `concurrency`-Gruppe gegen überlappende Deploys. | M | 🔲 | CD-01 |
| CD-03 | GHCR-Login im Deploy-Job über `secrets.GITHUB_TOKEN` + `github.actor` statt persönlichem PAT (ephemer pro Run) — als `GHCR_USERNAME`/`GHCR_TOKEN`-Env-Var für `ansible-playbook deploy.yml`, `ansible/roles/secrets/defaults/main.yml` bleibt unverändert. | S | 🔲 | CD-02 |
| CD-04 | Post-Deploy Health-Check-Step (Frontend HTTPS + Control-Server `/health`) — Job schlägt sichtbar fehl, wenn Stack nach Deploy nicht gesund ist. Bewusst kein automatischer Rollback. | S | 🔲 | CD-02 |
| CD-05 | Verifikation real: harmlosen Commit nach `main` mergen, beobachten dass der Runner-Job automatisch anspringt, deployt, Health-Check grün wird — ohne manuellen `ansible-playbook`-Aufruf. | S | 🔲 | CD-03, CD-04 |
| CD-06 | Doku-Update: README.md CI/CD-Abschnitt, `UEBERGABE-ABWEICHUNGEN.md`, `DECISIONS.MD`, Backlog-Status — insbesondere den Self-hosted-Runner-Sicherheitshinweis dauerhaft dokumentieren. | S | 🔲 | CD-05 |

**Nicht Teil dieses Sprints:** Automatischer Rollback bei fehlgeschlagenem Health-Check; echter
Hetzner-Server/Produktivumgebung; manuelles Freigabe-Gate (Nutzerentscheidung: kein Gate);
Multi-Environment-Pipeline (nur eine VM vorhanden).

## Ergebnis

_Noch offen — Sprint ist geplant, aber noch nicht umgesetzt._
