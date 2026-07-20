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
| LOCALVM-01 | Ansible-Grundgerüst: neues Verzeichnis `ansible/` (`ansible.cfg`, `inventory/`, `requirements.yml`, `roles/`-Skeleton), `ansible`/`ansible-playbook` lokal installieren/dokumentieren (z. B. `pipx`/venv, kein System-weites `pip install` als root) | S | ✅ | — |
| LOCALVM-02 | Neues Skript `scripts/local-vm-create.sh`: `virt-install` + cloud-init (NoCloud-ISO: SSH-Key, Hostname, User) für eine Ubuntu-24.04-Cloud-Image-VM, Netzwerk/IP-Ermittlung für das Ansible-Inventory | M | ✅ | LOCALVM-01 |
| LOCALVM-03 | Ansible-Rolle `bootstrap`: Docker-Installation, `avoc`-User, Verzeichnisstruktur — Ablösung von `hetzner-setup.md` Schritt 3 (Bash-Block) durch idempotente Tasks | M | ✅ | LOCALVM-01 |
| LOCALVM-04 | Ansible-Rolle `firewall`: `ufw`-Regeln analog der Port-Tabelle aus `hetzner-setup.md` Schritt 2 | S | ✅ | LOCALVM-01 |
| LOCALVM-05 | Ansible-Rolle `secrets`: `.env` aus fest hinterlegten Test-Dummy-Werten templaten, SSL-Selfsigned-Zertifikat wie in `secrets-setup-hetzner.sh` beschrieben | S/M | ✅ | LOCALVM-01 |
| LOCALVM-06 | Materialisierung als echte Dateien: `infrastructure/compose/docker-compose.hetzner.yml` (aus `docker-compose.prod.yml` + den 5 dokumentierten Änderungen), `scripts/deploy-hetzner.sh`, `scripts/secrets-setup-hetzner.sh` (1:1 aus `hetzner-setup.md`-Codeblöcken übernommen) | M | ✅ | — |
| LOCALVM-07 | Ansible-Playbook `deploy.yml`: Config-Dateien (Mosquitto/MediaMTX/Loki/Promtail/Grafana) auf die VM bringen, lokal gebaute Images übertragen (kein Docker-Hub-Roundtrip), `deploy-hetzner.sh` ausführen | M | ✅ | LOCALVM-03..06 |

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

---

## Ergebnisse

### Was wurde gebaut

**`ansible/`** (LOCALVM-01) — neues Grundgerüst: `ansible.cfg` (Inventory-Pfad, `host_key_checking
= False` explizit nur für die wechselnde lokale VM begründet, `become`-Defaults), `requirements.yml`
(`community.general` für das `ufw`-Modul), `inventory/hosts.ini` (Gruppe `[local_vm]`, Platzhalter
`<VM_IP_PLACEHOLDER>`, wird von `local-vm-create.sh` befüllt), `group_vars/local_vm.yml`
(`avoc_user`/`app_dir`/`app_base_dir` rollenübergreifend), drei Rollen-Skeletons.

**`scripts/local-vm-create.sh`** (LOCALVM-02) — `virt-install --import` gegen ein
Ubuntu-24.04-Cloud-Image (Backing-File-Overlay via `qemu-img`), cloud-init NoCloud-Seed-ISO
(`user-data`/`meta-data`, SSH-Key aus `~/.ssh/id_ed25519.pub`, Hostname, `avoc`-User mit
passwortlosem `sudo` — Docker selbst wird bewusst NICHT per cloud-init installiert, sondern
einheitlich über `roles/bootstrap`, kein zweiter Installationspfad). Ermittelt die VM-IP per
`virsh domifaddr` (Polling bis 120s) und trägt sie automatisch in `ansible/inventory/hosts.ini`
ein (`sed`, mit `.bak`-Backup). ISO-Tool-Auswahl mit Fallback (`xorriso` → `genisoimage` →
`cloud-localds`) — auf diesem Rechner ist nur `xorriso` vorhanden, getestet.

**`ansible/roles/bootstrap`** (LOCALVM-03) — idempotente Ablösung von `hetzner-setup.md` Schritt 3:
Python3-Bootstrap per `raw` (Ubuntu-Cloud-Images liefern es nicht immer vorinstalliert;
`gather_facts: false` im Playbook + expliziter `setup`-Task danach), System-Update, Docker-APT-Repo
+ -Pakete, `avoc`-User (idempotent für beide Fälle: frischer echter Hetzner-Server, wo Ansible
initial als `root` verbindet und `avoc` erst anlegt — vs. lokale VM, wo cloud-init `avoc` bereits
angelegt hat; `authorized_keys`-Übernahme von `/root/.ssh/` ist entsprechend `ignore_errors: true`
für den lokalen-VM-Fall), App-Verzeichnisstruktur.

**`ansible/roles/firewall`** (LOCALVM-04) — `ufw`-Regeln 1:1 aus der Port-Tabelle
`hetzner-setup.md` Schritt 2 (Default-Deny eingehend, Port-Liste, TURN-Relay-Range
49152–65535/UDP). Port 8085 (Fleet Service) bewusst NICHT geöffnet, wie in der Doku vermerkt
(kein neuer Befund). Zusätzlich Port 8883/TCP ergänzt (s. "Erkannte, nicht behobene Lücken"
unten) — Port 1883 bleibt trotzdem wie dokumentiert in der Liste (1:1-Auftrag).

**`ansible/roles/secrets`** (LOCALVM-05) — `.env` aus fest hinterlegten Test-Dummy-Werten
(`templates/env.j2`, klar als Test-Dummy gekennzeichnet, Mindestlängen an
`secrets-setup-hetzner.sh` angelehnt), Self-Signed-SSL-Zertifikat idempotent (nur erzeugt, wenn
noch nicht vorhanden — analog `deploy.sh`/`deploy-hetzner.sh`-Muster), `TURN_EXTERNAL_IP` aus der
Inventory-IP der VM abgeleitet.

**`infrastructure/compose/docker-compose.hetzner.yml`**, **`scripts/deploy-hetzner.sh`**,
**`scripts/secrets-setup-hetzner.sh`** (LOCALVM-06) — erstmals als echte, committete Dateien
materialisiert (vorher nur Codeblöcke in `hetzner-setup.md`). Compose-Datei: `docker-compose.prod.yml`
kopiert + alle 5 dokumentierten Änderungen (coturn `--relay-ip` entfernt, `--external-ip` ohne
NAT-Mapping, Loki-/Promtail-/Grafana-Volumes auf `/home/avoc/...`) — per `docker compose config`
gegen eine Dummy-`.env` verifiziert (siehe unten). `secrets-setup-hetzner.sh` ist reines 1:1 (keine
Änderung). `deploy-hetzner.sh` ist bis auf einen einzigen, klar kommentierten Zusatz 1:1: eine
optionale `SKIP_REGISTRY_PULL=true`-Verzweigung (Default `false` = unverändertes Doku-Verhalten),
die Docker-Hub-Login/-Pull überspringt und stattdessen nur lokale Image-Präsenz prüft — notwendig,
weil die Architektur-Entscheidung "kein Docker-Hub-Roundtrip" (EPIC) sonst mit dem 1:1-Skript
kollidiert hätte (das Doku-Skript loggt sich unbedingt bei Docker Hub ein). Dieselbe Technik
verwendet bereits `scripts/deploy.sh` auf EC2 ("Übergabe-Abweichung von ADR-019").

**`ansible/site.yml`** (Provisionierung: bootstrap → firewall → secrets) und **`ansible/deploy.yml`**
(LOCALVM-07) — Config-Dateien (Mosquitto inkl. TLS-Zertifikate/Passwd, MediaMTX, Loki, Promtail,
Grafana-Provisioning) auf die VM kopieren, benötigte `avoc-*`-Images per
`docker compose config --images` ermitteln, lokale Präsenz prüfen (Warnung bei fehlendem Image),
per `docker save`/`copy`/`docker load` ohne Docker-Hub-Roundtrip auf die VM übertragen, danach
`deploy-hetzner.sh` mit `SKIP_REGISTRY_PULL=true` ausführen. Bewusste Trennung Build/Deploy: das
Playbook baut keine Images selbst (das lokale Vorhandensein der Images ist eine dokumentierte
Voraussetzung, analog `push: build-prod` im Makefile).

### Architektur-Entscheidungen dieses Sprints (über die EPIC-Vorgaben hinaus, dokumentiert statt blockiert)

- **`SKIP_REGISTRY_PULL`-Zusatz in `deploy-hetzner.sh`** (s. oben) — kleinste mögliche, klar
  kommentierte Abweichung vom 1:1-Auftrag, um die bereits freigegebene
  "kein-Docker-Hub-Roundtrip"-Architektur-Entscheidung tatsächlich funktionsfähig zu machen; ohne
  diesen Zusatz hätte `ansible/deploy.yml` das materialisierte Skript für die lokale VM gar nicht
  sinnvoll aufrufen können.
- **`ansible/deploy.yml` kopiert zusätzlich Mosquitto-TLS-Zertifikate + `passwd`**, obwohl
  `hetzner-setup.md` Schritt 6 dafür nur `mosquitto.conf` auflistet — die Doku ist an dieser
  Stelle älter als MQTTS-01/MQTTAUTH-04 (TLS-Pflicht seit Sprint 40) und daher bereits vor diesem
  Sprint lückenhaft für einen tatsächlich lauffähigen Hetzner-Deploy. Committete
  Test-TLS-Assets aus `infrastructure/mosquitto/` werden 1:1 mitkopiert (kein Neu-Generieren, keine
  neuen Secrets).
- **`ansible/roles/firewall` öffnet zusätzlich Port 8883/TCP**, obwohl die Doku-Tabelle nur 1883
  nennt — echter Compose-Stand nutzt TLS-only auf 8883 (s. o.); 1883 bleibt trotzdem in der Liste
  (1:1-Auftrag für die Tabelle selbst, nicht stillschweigend entfernt).

### Erkannte, nicht behobene Lücken (bewusst außerhalb des Sprint-Scopes, für Sprint B/LOCALVM-09 vorgemerkt)

- `hetzner-setup.md` Schritt 2 (MQTT-Port 1883) und Schritt 6 (Mosquitto-Copy-Liste) sind seit
  MQTTS-01/MQTTAUTH-04 veraltet (echter Stand: TLS-only 8883, plus `passwd`+3 Zertifikate nötig).
- `hetzner-setup.md` Schritt 5 (Docker-Hub-Push mit `${REGISTRY}/avoc-x`-Tag) und die
  Compose-Datei (`avoc-x:${VERSION}` ohne Registry-Präfix) sind bereits ohne Zutun dieses Sprints
  inkonsistent (unterschiedliche Image-Tag-Namen) — betrifft nur den echten
  Docker-Hub-Pfad, nicht den in diesem EPIC gebauten lokalen-VM-Pfad (der umgeht den Docker-Hub
  komplett).
- Der in mehreren Dateien referenzierte `docs/deployment/UEBERGABE-ABWEICHUNGEN.md` existiert
  nicht als Datei im Repo (nur als Kommentar-Verweis in `docker-compose.prod.yml`/`scripts/deploy.sh`)
  — vorbestehender Befund, nicht Teil dieses Sprints.
- Diese drei Punkte sind reine Doku-Drift-Befunde (kein Sicherheitsrisiko, keine Auswirkung auf
  die in diesem Sprint gebauten Dateien) und wurden nicht behoben, um den Sprint-Scope (reine
  Autorierung der LOCALVM-Dateien) nicht zu sprengen — Kandidat für LOCALVM-09 (Sprint B) bzw.
  einen eigenen kleinen Doku-Fast-Track-Task.

### Was verifiziert wurde

- **`ansible`/`ansible-playbook`-Installation (LOCALVM-01):** `sudo apt-get install -y ansible`
  konnte in dieser Agenten-Session NICHT ausgeführt werden — `sudo` verlangt interaktive
  Passwortabfrage, die in diesem Hintergrund-Job nicht möglich ist (kein TTY). Als Nachweis, dass
  das vorgesehene Paket (`ansible-core` 2.20.1 + `ansible` 13.1.0, beide im Ubuntu-Repo
  vorhanden) tatsächlich funktioniert, wurden `ansible-core`, `python3-resolvelib`,
  `python3-paramiko`, `python3-dnspython` per `apt-get download` (kein Root nötig) geladen und
  per `dpkg-deb -x` in ein lokales, session-privates Verzeichnis entpackt (kein
  System-weiter Eingriff, nichts committet). Damit erfolgreich verifiziert:
  `ansible --version` / `ansible-playbook --version` (beide `core 2.20.1`, Python 3.14.4).
  **Für den Nutzer offen:** `sudo apt-get install -y ansible` einmalig interaktiv ausführen, um
  `ansible`/`ansible-playbook` dauerhaft systemweit verfügbar zu machen (Paket ist als Kandidat
  bestätigt, nur die Root-Rechte fehlten in dieser Session).
- **`ansible-playbook site.yml --syntax-check`** und **`ansible-playbook deploy.yml --syntax-check`**:
  beide grün (gegen die oben beschriebene lokale Ansible-Installation, `community.general` per
  `ansible-galaxy collection install -r requirements.yml` in ein lokales Verzeichnis geladen, da
  `--syntax-check` Modulnamen auflöst).
- **`--list-tasks`** für beide Playbooks zusätzlich geprüft (korrekte Rollen-/Task-Reihenfolge:
  bootstrap → firewall → secrets in `site.yml`; Configs → Images → Deploy-Script in `deploy.yml`).
- Alle YAML-Dateien unter `ansible/` zusätzlich per `python3 -c "yaml.safe_load(...)"` einzeln
  geprüft (10 Dateien, alle fehlerfrei).
- **`bash -n`** für alle drei neuen/geänderten Shell-Skripte (`local-vm-create.sh`,
  `deploy-hetzner.sh`, `secrets-setup-hetzner.sh`) — alle drei fehlerfrei.
- **`docker compose -f infrastructure/compose/docker-compose.hetzner.yml config`** gegen eine
  Dummy-`.env` (alle Pflichtvariablen gesetzt, keine echten Werte) — Exit 0, vollständig
  aufgelöste YAML geprüft: kein `--relay-ip` mehr vorhanden, `--external-ip=<dummy-ip>` ohne
  `/PRIVATE`-Mapping, alle drei Loki-/Promtail-/Grafana-Volumes zeigen auf `/home/avoc/...`. Auch
  `--images` separat geprüft (alle 14 erwarteten Images, inkl. `avoc-fleet-service`, korrekt
  gelistet).
- **`ansible-lint`**: nicht verfügbar (`apt-cache policy` liefert keinen Kandidaten unter diesem
  Namen bzw. Installation wie bei `ansible` selbst durch fehlendes `sudo` blockiert) — laut
  Auftrag optional/nicht blockierend, daher kein Blocker.
- **`shellcheck`**: nicht installiert, aus denselben `sudo`-Gründen nicht nachinstallierbar in
  dieser Session — nicht explizit vom Sprint-Auftrag gefordert (nur `bash -n`), daher nicht
  blockierend.

### Was bewusst NICHT getestet wurde (und warum)

- **Kein echter `ansible-playbook`-Lauf gegen eine Zielmaschine** (weder `site.yml` noch
  `deploy.yml`) — es existiert keine laufende VM in diesem Sprint (Teil A ist explizit reine
  Autorierung, s. Sprint-Kopf). Das schließt auch `--check`/Dry-Run mit ein: ein echter
  `--check`-Lauf bräuchte ebenfalls einen erreichbaren SSH-Host, den es noch nicht gibt —
  `--syntax-check` + `--list-tasks` sind die in diesem Sprint tatsächlich verfügbaren
  Prüfmittel (wie im Sprint-Kopf "Hinweis für die Ausführung" vorgesehen).
  Playbook-interne Logikfehler, die erst zur Laufzeit sichtbar würden (z. B. falsche
  Variablen-Referenzen zwischen Rollen, `become_user`-Rechteprobleme, tatsächliches
  ufw-/apt-Verhalten auf Ubuntu 24.04), sind entsprechend nicht ausgeschlossen — das ist explizit
  Gegenstand von Sprint B (LOCALVM-08).
- **`scripts/local-vm-create.sh` wurde nicht ausgeführt** (Auftrag: explizit nicht Teil dieses
  Sprints) — nur `bash -n` und Durchsicht. `virt-install`/cloud-init-Verhalten, IP-Ermittlung per
  `virsh domifaddr` und die `sed`-Inventory-Befüllung sind dadurch ungetestet gegen eine echte VM.
- **Kein echter Image-Build/-Transfer** (`ansible/deploy.yml`s `docker save`/`docker load`-Pfad) —
  setzt bereits gebaute `avoc-*`-Images voraus, die in dieser Session nicht gebaut wurden (kein
  Scope dieses Sprints, separate Build-Vorbedingung).
- **Kein `ansible-galaxy install` als dauerhafter, committeter Schritt** — die
  `community.general`-Collection wurde nur für die Syntax-Check-Verifikation in ein
  session-lokales, nicht committetes Verzeichnis geladen; im echten Einsatz lädt der Nutzer sie
  regulär per `ansible-galaxy collection install -r requirements.yml` (Standard-Workflow, in
  `ansible.cfg`/`requirements.yml`-Kommentaren dokumentiert).
