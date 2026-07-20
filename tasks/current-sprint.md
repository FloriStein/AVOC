> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Sprint 48 — Lokale Ansible-VM als Hetzner-Nachbildung, Teil A: Autorierung

Vollständiger Kontext, Architektur-Entscheidungen und Vorrecherche:
[tasks/backlog.md](backlog.md) EPIC "Lokale Ansible-VM als Hetzner-Nachbildung (AWS-Ersatz für die
Testumgebung)". Dieser Sprint deckt den ersten von zwei Teilen ab (**Autorierung**, kein
Zugriff auf eine echte laufende VM nötig) — Teil B (**Verifikation** gegen die echte VM) folgt als
eigener Sprint, sobald Teil A abgeschlossen ist.

**Freigabe (2026-07-20):** Nutzer möchte AWS als lokale Test-/Referenzumgebung durch eine lokale,
per Ansible provisionierte VM ersetzen, die den zukünftigen Hetzner-Server nachbildet. Umsetzung
über Sprints mit spawnbaren Agenten angefordert.

**Kurzfassung Architektur-Entscheidungen** (Details siehe EPIC in `tasks/backlog.md`):
- libvirt/KVM + `virt-install` + cloud-init (bereits vorhanden auf diesem Rechner), kein
  Vagrant/VirtualBox.
- Ansible-Rollen statt weiterer Bash-Copy-Paste-Blöcke aus `docs/deployment/hetzner-setup.md`.
- `docker-compose.hetzner.yml`/`deploy-hetzner.sh`/`secrets-setup-hetzner.sh` werden 1:1 aus der
  Doku als echte, versionierte Dateien materialisiert.
- Lokale Test-Dummy-Secrets statt interaktivem Prompt (nur für die lokale VM).
- Kein Docker-Hub-Roundtrip für die lokale Verifikation — Images lokal bauen und direkt in die VM
  übertragen.

Datum: 2026-07-20 | **Status: Geplant, zur Ausführung an Agenten übergeben**
Branch/Worktree: `feature/fleet-service-foundation-localvm` (neuer, dedizierter Worktree, Basis
`feature/fleet-service-foundation` @ Sprint 46, um das aktuell laufende Sprint 47 auf dem
Hauptstrang nicht zu stören).

## Tasks

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| LOCALVM-01 | Ansible-Grundgerüst: neues Verzeichnis `ansible/` (`ansible.cfg`, `inventory/`, `requirements.yml`, `roles/`-Skeleton), `ansible`/`ansible-playbook` lokal installieren/dokumentieren (z. B. `pipx`/venv, kein System-weites `pip install` als root) | S | 🔲 | — |
| LOCALVM-02 | Neues Skript `scripts/local-vm-create.sh`: `virt-install` + cloud-init (NoCloud-ISO: SSH-Key, Hostname, User) für eine Ubuntu-24.04-Cloud-Image-VM, Netzwerk/IP-Ermittlung für das Ansible-Inventory | M | 🔲 | LOCALVM-01 |
| LOCALVM-03 | Ansible-Rolle `bootstrap`: Docker-Installation, `avoc`-User, Verzeichnisstruktur — Ablösung von `hetzner-setup.md` Schritt 3 (Bash-Block) durch idempotente Tasks | M | 🔲 | LOCALVM-01 |
| LOCALVM-04 | Ansible-Rolle `firewall`: `ufw`-Regeln analog der Port-Tabelle aus `hetzner-setup.md` Schritt 2 | S | 🔲 | LOCALVM-01 |
| LOCALVM-05 | Ansible-Rolle `secrets`: `.env` aus fest hinterlegten Test-Dummy-Werten templaten, SSL-Selfsigned-Zertifikat wie in `secrets-setup-hetzner.sh` beschrieben | S/M | 🔲 | LOCALVM-01 |
| LOCALVM-06 | Materialisierung als echte Dateien: `infrastructure/compose/docker-compose.hetzner.yml` (aus `docker-compose.prod.yml` + den 5 dokumentierten Änderungen), `scripts/deploy-hetzner.sh`, `scripts/secrets-setup-hetzner.sh` (1:1 aus `hetzner-setup.md`-Codeblöcken übernommen) | M | 🔲 | — |
| LOCALVM-07 | Ansible-Playbook `deploy.yml`: Config-Dateien (Mosquitto/MediaMTX/Loki/Promtail/Grafana) auf die VM bringen, lokal gebaute Images übertragen (kein Docker-Hub-Roundtrip), `deploy-hetzner.sh` ausführen | M | 🔲 | LOCALVM-03..06 |

**Nicht Teil dieses Sprints:** echte VM-Erzeugung/End-to-End-Verifikation (Sprint B, LOCALVM-08),
Doku-Umstellung von `hetzner-setup.md` (Sprint B, LOCALVM-09), echter Hetzner-Server, Stilllegung
des AWS-Pfads, CI-Integration, Let's-Encrypt — siehe EPIC "Nicht Teil dieses Vorhabens" in
`tasks/backlog.md`.

**Hinweis für die Ausführung:** LOCALVM-01..07 sind reine Autorierungsarbeit (Ansible-Rollen,
Skripte, Compose-Datei) ohne Abhängigkeit von einer laufenden VM — `ansible-playbook --syntax-check`
und `--check` (Dry-Run) sind die verfügbaren Verifikationsmittel in diesem Sprint, kein echter
Playbook-Lauf gegen eine Zielmaschine. `bash -n` für alle neuen Shell-Skripte. Am Ende: Ergebnisse
in dieser Datei unter "## Ergebnisse" dokumentieren, nach `tasks/sprints/48-lokale-ansible-vm-teil-a.md`
archivieren, `tasks/done.md` + `tasks/backlog.md`-Status (`🔲 Sprint A` → `✅ Sprint 48`) aktualisieren,
committen (dedizierter Worktree/Branch, kein Push).
