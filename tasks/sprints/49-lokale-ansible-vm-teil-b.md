> **Lebenszyklus (seit 2026-07-19, MD-Doku-Konsolidierung):** Diese Datei enthält ausschließlich
> den aktuell aktiven Sprint. Bei Sprint-Abschluss wandert der Inhalt unverändert nach
> `tasks/sprints/NN-slug.md`, diese Datei wird für den nächsten Sprint geleert, und `tasks/done.md`
> bekommt einen neuen kompakten Index-Eintrag mit Link auf die neue Datei. Vollständige
> Sprint-Historie: [tasks/done.md](done.md) (Index) → [tasks/sprints/](sprints/) (Volltext je Sprint).

---

# Sprint 49 — Lokale Ansible-VM als Hetzner-Nachbildung, Teil B: Verifikation

Vollständiger Kontext: [tasks/backlog.md](backlog.md) EPIC "Lokale Ansible-VM als
Hetzner-Nachbildung (AWS-Ersatz für die Testumgebung)". Vorgänger:
[tasks/sprints/48-lokale-ansible-vm-teil-a.md](sprints/48-lokale-ansible-vm-teil-a.md) (Teil A —
Autorierung, ✅ abgeschlossen: `ansible/`-Rollen, `scripts/local-vm-create.sh`,
`docker-compose.hetzner.yml`/`deploy-hetzner.sh`/`secrets-setup-hetzner.sh`, alles nur
syntax-/statisch geprüft, noch nie gegen eine echte Maschine gelaufen).

**Bekannter Blocker zu Sprint-Start:** `ansible`/`ansible-playbook` ist auf diesem Rechner nicht
systemweit installiert; `sudo apt-get install -y ansible` scheitert in nicht-interaktiven
Agenten-Sessions (kein TTY für die Passwortabfrage) — weder Hintergrund-Agent noch Orchestrator
haben interaktiven `sudo`-Zugriff. `virsh`/`virt-install` funktionieren dagegen bereits ohne
`sudo` (Gruppenmitgliedschaft `libvirt`/`kvm` ausreichend, verifiziert). Der Nutzer muss daher
einmalig selbst `sudo apt-get install -y ansible` in einem echten Terminal ausführen, bevor
LOCALVM-08 vollständig abgeschlossen werden kann. VM-Erzeugung (kein Ansible nötig) kann parallel
dazu schon laufen.

**Erkannte, aus Teil A übernommene Doku-Lücken** (siehe Sprint 48 "Erkannte, nicht behobene
Lücken"): `hetzner-setup.md` Schritt 2/6 sind seit MQTTS-01/MQTTAUTH-04 veraltet (Mosquitto
TLS-only 8883 statt 1883, fehlende Zertifikats-Kopie); `docs/deployment/UEBERGABE-ABWEICHUNGEN.md`
wird an zwei Stellen referenziert, existiert aber nicht als Datei. Beide werden in LOCALVM-09
mitkorrigiert.

Datum: 2026-07-20 | **Status: ✅ Abgeschlossen — LOCALVM-08b/08c/09 vollständig durchgeführt, siehe
"## Ergebnisse" unten.**
Branch/Worktree: `feature/fleet-service-foundation-localvm` (unverändert seit Sprint 48).

## Tasks

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| LOCALVM-08a | VM-Erzeugung: `scripts/local-vm-create.sh` real ausführen (Ubuntu-24.04-Cloud-Image, cloud-init, SSH-Erreichbarkeit abwarten), Ansible-Inventory-Befüllung verifizieren. Kein Ansible nötig — nicht vom Sudo-Blocker betroffen. | M | ✅ | — |
| LOCALVM-08b | Ansible-Lauf: `ansible/site.yml` (bootstrap+firewall+secrets) und `ansible/deploy.yml` (Config-Transfer, Image-Transfer ohne Docker-Hub, `deploy-hetzner.sh`) real gegen die VM ausführen, Fehler iterativ beheben. Braucht funktionierendes `ansible-playbook` — **blockiert bis Nutzer `sudo apt-get install -y ansible` ausgeführt hat.** | M | ✅ | LOCALVM-08a, Ansible-Install |
| LOCALVM-08c | Smoke-Test gegen den laufenden Stack in der VM: `docker compose ps` (alle Container healthy/running), HTTP-Erreichbarkeit Frontend (Port 3000)/Control-Server-API (Port 8080) von der VM-IP aus, analog Sprint-41-CI-Verifikationsmuster. | S | ✅ | LOCALVM-08b |
| LOCALVM-09 | Doku: `docs/deployment/hetzner-setup.md` auf den Ansible-Workflow umstellen (Schritt 1/3/4/6/7 durch Verweis auf `local-vm-create.sh` + Ansible-Rollen ersetzen, Rest bleibt für den echten Server gültig); dabei die zwei aus Sprint 48 bekannten Doku-Lücken mitkorrigieren (Mosquitto-Port/Copy-Liste auf TLS-8883-Stand, `UEBERGABE-ABWEICHUNGEN.md` entweder anlegen oder Referenzen entfernen); `DECISIONS.MD`, `tasks/backlog.md`-Status-Update (EPIC komplett ✅). | S/M | ✅ | LOCALVM-08c |

## Zwischenstand (2026-07-20, nach LOCALVM-08a)

**VM steht, wartet auf Ansible-Installation durch den Nutzer.**

LOCALVM-08a wurde real ausgeführt und vollständig verifiziert (nicht nur syntaktisch geprüft):
`scripts/local-vm-create.sh` erzeugt die VM `avoc-local-vm` erfolgreich per `virt-install --import`
+ cloud-init NoCloud-ISO. VM läuft, `cloud-init status` meldet `running`/abgeschlossen, SSH als
`avoc`-User mit dem in `ansible/inventory/hosts.ini` hinterlegten Key funktioniert, passwortloser
`sudo` funktioniert (`sudo -n whoami` → `root`), Hostname korrekt (`avoc-local-vm`),
`ansible/inventory/hosts.ini` wurde vom Skript automatisch mit der ermittelten VM-IP befüllt
(reproduzierbar über mehrere Neuerzeugungen hinweg getestet, aktuell `192.168.122.13`, ändert sich
bei jedem `local-vm-create.sh`-Lauf per DHCP).

Das Skript lief **nie zuvor** (Sprint 48 war reine Autorierung) — beim ersten echten Lauf traten
wie erwartet zwei reale, in Sprint 48 nicht vorhersehbare Fehler auf, beide in
`scripts/local-vm-create.sh` behoben (Details/Begründung als Kommentare direkt im Skript):

1. **Verzeichnis-Traversal-Rechte für `libvirt-qemu`:** `virt-install` nutzt per Default
   `qemu:///system` — der QEMU-Prozess läuft als Systemnutzer `libvirt-qemu`, nicht als der
   aufrufende Nutzer. `$HOME/.local/share` (Ubuntu-Default-Rechte, kein `o+x`) war für
   `libvirt-qemu` nicht traversierbar → `virt-install` scheiterte mit "Cannot access storage
   file ... Keine Berechtigung". Fix: Skript setzt jetzt selbst automatisch nur das
   Traversal-Bit (`o+x`, bewusst kein `o+r`, kein Verzeichnislisting) auf den Pfadsegmenten
   zwischen `$HOME` und `WORK_DIR` — idempotent, kein manueller Host-Eingriff mehr nötig,
   funktioniert reproduzierbar auch aus dem "kaputten" Ausgangszustand heraus getestet.
2. **AppArmor blockiert Backing-File-Zugriff:** Die ursprüngliche Disk-Erzeugung nutzte ein
   qcow2-Backing-File-Overlay (`qemu-img create -b`). Libvirts Pro-Domain-AppArmor-Profil
   (`virt-aa-helper`) nimmt aber nur die in der Domain-XML direkt referenzierten Disk-Pfade auf,
   nicht die qcow2-Backing-Chain — das Basis-Cloud-Image blieb für den konfinierten QEMU-Prozess
   unzugänglich ("Permission denied" trotz korrekter Unix-Rechte). Eine Korrektur der
   System-AppArmor-Policy bräuchte Root (nicht verfügbar). Fix: `qemu-img convert` statt
   Backing-File-Overlay — eigenständige Kopie des Basis-Images statt dünnem Overlay (mehr
   Plattenplatz, aber einzige praktikable Lösung ohne Root-Zugriff auf die AppArmor-Config).
3. **`set -e`/`pipefail`-Bug in der IP-Poll-Schleife** (unabhängig von den beiden obigen Funden):
   `virsh domifaddr --source agent` schlägt praktisch immer fehl (kein `qemu-guest-agent` in
   einem frischen Cloud-Image), und die ursprüngliche Schleife nutzte `cmd1 && cmd2`/
   `cmd1 || cmd2` als alleinstehende Anweisungen — unter `set -euo pipefail` beendete das die
   gesamte Domäne bereits beim ersten Durchlauf **still, ohne jede Fehlermeldung** (genau das
   real beobachtete Symptom: Skript brach nach "Warte auf Boot..." ab, ohne WARNUNG oder
   Erfolgsmeldung). Fix: `if`-Blöcke statt bare `&&`/`||`, `|| true` an den Pipelines.

Alle drei Fixes wurden gegen einen vollständig frischen Durchlauf verifiziert (VM destroy/undefine,
Artefakte gelöscht, Host-Berechtigungen bewusst wieder auf den kaputten Ausgangszustand gesetzt,
Skript erneut von Null ausgeführt) — reproduzierbar grün, kein einmaliger Zufallstreffer.

**Blocker, unverändert seit Sprint-Start:** `command -v ansible-playbook` liefert weiterhin nichts
— das Paket ist in dieser Agenten-Session nicht systemweit installiert (kein interaktives `sudo`
möglich). Damit bleiben LOCALVM-08b (echter `site.yml`/`deploy.yml`-Lauf), LOCALVM-08c
(Smoke-Test) und LOCALVM-09 (Doku-Umstellung) laut Sprint-Auftrag bewusst **unbearbeitet** —
kein `--syntax-check`-Ersatz, keine Simulation, kein Raten.

**Für den Nutzer offen:** einmalig `sudo apt-get install -y ansible` in einem echten Terminal
ausführen. Danach kann ein neuer Agenten-Lauf direkt mit LOCALVM-08b fortsetzen — die VM steht
bereits, `ansible/inventory/hosts.ini` ist bereits korrekt befüllt (ggf. IP per erneutem
`bash scripts/local-vm-create.sh`-Lauf frisch ziehen, falls die VM zwischenzeitlich neu gestartet
wurde und die DHCP-Lease sich geändert hat — `virsh domifaddr avoc-local-vm` zum Prüfen).

**Nicht Teil dieses Sprints:** echter Hetzner-Server, Stilllegung des AWS-Pfads, CI-Integration,
Let's-Encrypt — siehe EPIC "Nicht Teil dieses Vorhabens" in `tasks/backlog.md`.

**Hinweis für die Ausführung:** Falls `ansible-playbook` zu Beginn der Ausführung noch nicht
verfügbar ist: LOCALVM-08a trotzdem vollständig durchführen (kein Blocker), Ergebnis
dokumentieren, dann mit klarer Statusmeldung pausieren statt zu raten/zu simulieren. Kein
`--syntax-check`-Ersatz für den echten Lauf verwenden und als "verifiziert" ausgeben — Teil A hat
das bereits getan, Teil B muss den echten Lauf zeigen. Nach Abschluss: Ergebnisse unter
"## Ergebnisse" dokumentieren, nach `tasks/sprints/49-lokale-ansible-vm-teil-b.md` archivieren,
`tasks/done.md` + `tasks/backlog.md`-Status aktualisieren, committen (kein Push).

## Ergebnisse

**Blocker aufgehoben, Sprint vollständig durchgeführt.** `ansible --version`/
`command -v ansible-playbook` bestätigten die vom Nutzer durchgeführte Installation
(`ansible-core 2.20.1`, `community.general` 12.1.0 bereits vorhanden). LOCALVM-08a gegengecheckt:
VM lief unverändert unter derselben IP (`192.168.122.13`, `virsh domifaddr` bestätigt), SSH
funktionierte; `ansible/inventory/hosts.ini` enthielt allerdings noch den unveränderten
`<VM_IP_PLACEHOLDER>` (die Sed-Ersetzung aus dem vorigen Agentenlauf war nicht committet/
persistiert) — von Hand auf die verifizierte IP gesetzt, `ansible -m ping` bestätigte
Erreichbarkeit.

### LOCALVM-08b — `site.yml`/`deploy.yml` real ausgeführt, 5 echte Bugs gefunden und behoben

Wie im Auftrag erwartet (Sprint 48 war reine Autorierung, nie ein echter Lauf) traten beim ersten
echten Durchlauf mehrere reale, vorher unbekannte Fehler auf. Alle behoben, jeweils mit Kommentar
an der Fundstelle dokumentiert:

1. **`apt upgrade` bricht SSH mitten im Lauf ab, sobald `openssh-server` im Upgrade-Satz ist**
   (`roles/bootstrap/tasks/main.yml`): dpkg-Fehler "Could not execute systemctl" /
   "installed openssh-server package post-installation script subprocess returned error exit
   status 1" — Ursache: das Postinst von `openssh-server` startet den SSH-Dienst neu, während
   genau diese SSH-Verbindung noch den laufenden Ansible-`apt`-Task ausführt (Ubuntu 24.04 nutzt
   `ssh.socket`/`ssh@.service`; der durch die fehlgeschlagene Konfiguration kaputte Dienst nahm
   TCP-Verbindungen zwar weiter an, brach sie aber sofort ohne SSH-Banner ab — reproduzierbar per
   `ssh -vvv`/`nc`/rohem `/dev/tcp` verifiziert). Die VM war nur noch per `virsh reboot` wieder
   erreichbar. Fix: Task läuft jetzt `async`+`poll: 0` (vom SSH-Kanal entkoppelt, überlebt einen
   Verbindungsabbruch), separates Polling mit `ignore_unreachable`, danach `meta: reset_connection`
   + `wait_for_connection`, abschließend `dpkg --configure -a` als Nachzieh-Schritt. Gegen einen
   vollständigen Neustart des Playbooks verifiziert (reproduzierbar grün, kein Ausreißer).
2. **Docker-Apt-Repo nutzte die falsche Architekturbezeichnung** (`roles/bootstrap/tasks/main.yml`):
   `ansible_architecture` liefert auf x86_64 den Wert `x86_64` (uname-Konvention), APTs `arch=`
   erwartet aber `amd64` (dpkg-Konvention) — stiller Fehlschlag, `apt-get update` lief fehlerfrei,
   aber `apt-cache policy docker-ce` zeigte `Candidate: (none)`, der Folge-Task scheiterte mit
   "No package matching 'docker-ce' is available". Fix: eigener Task ermittelt die Architektur
   jetzt per `dpkg --print-architecture`.
3. **`ufw`-Kommentar mit Apostroph bricht das Modul** (`roles/firewall/defaults/main.yml`): Port-80-
   Kommentar "Let's Encrypt..." enthielt ein Apostroph, `community.general.ufw` scheiterte beim
   internen Shell-Quoting ("No closing quotation"). Fix: Apostroph aus dem Text entfernt.
4. **Mosquitto-Container startete nicht — Dateiberechtigungen** (`ansible/deploy.yml`): `passwd`/
   `key.pem` wurden mit `mode: 0600`, Owner `avoc` übertragen; der `eclipse-mosquitto:2`-
   Container-Prozess konnte die Datei nicht öffnen ("Unable to open pwfile", Crash-Loop) —
   Bind-Mount macht die Host-UID maßgeblich, der Container-Prozess läuft nicht unter exakt dieser
   UID. Fix: `mode: 0644` (für reine Test-Dummy-Werte auf einer rein lokalen VM ausreichend,
   empirisch verifiziert).
5. **MQTT-Testpasswort passte nicht zum committeten Passwd-Hash** (`roles/secrets/defaults/main.yml`):
   `avoc_secrets_mqtt_password` war `"local-test-mqtt-password"`, `infrastructure/mosquitto/passwd`
   ist aber laut `tasks/sprints/38-mqtt-authentifizierung.md` mit `avoc`/`changeme` gehasht (Datei
   wird von `deploy.yml` unverändert kopiert, nicht neu generiert) — alle drei MQTT-Clients
   (telemetry-service/fleet-service/vehicle-mock) scheiterten mit "not Authorized". Fix: Default auf
   `changeme` korrigiert.

**Zusätzlicher, kein Ansible-Bug, aber real gefundener Betriebsfund:** Die bereits lokal
gebauten `avoc-telemetry-service`/`avoc-fleet-service`/`avoc-vehicle-mock`-Images (2026-07-19)
waren älter als der TLS-Client-Support aus Sprint 40 (MQTTS-03, Commit `fd0930c`,
2026-07-20 07:27) — die alten Binaries verbanden sich per Klartext-TCP statt TLS zum
TLS-only-Broker (Mosquitto-Log: "OpenSSL Error ... wrong version number"). Kein Ansible-/
Deploy-Code-Fehler, sondern eine reale Voraussetzung, die der Kopfkommentar von `ansible/deploy.yml`
bereits benennt ("Images müssen lokal aktuell gebaut sein") — hier zum ersten Mal real erlebt.
Fix: `avoc-telemetry-service`/`avoc-fleet-service`/`avoc-vehicle-mock` frisch aus aktuellem HEAD
gebaut (`docker buildx build ... --load`), Binary-Inhalt per `strings` gegen `avoc/pkg/mqtttls`
verifiziert, danach erneut deployed.

Nach allen fünf Fixes: `ansible-playbook site.yml` und `ansible-playbook deploy.yml` liefen beide
mit `failed=0` durch (`site.yml`: `ok=28 changed=8`, `deploy.yml`: `ok=19 changed=6`), wiederholt
gegen einen vollständig neu erzeugten Stack (`docker compose down` + Re-Deploy) verifiziert.

### LOCALVM-08c — Smoke-Test: alle 15 Container laufen stabil, HTTP-Endpunkte erreichbar

- `docker compose -f docker-compose.hetzner.yml ps`: alle 15 Container `Up` (`postgres` zusätzlich
  `(healthy)`), keine Restart-Loops, über mehrere Prüfungen im Abstand von Sekunden bis Minuten
  stabil geblieben.
- `curl http://192.168.122.13:3000/` → `HTTP 200`, echtes HTML (Vite/React-Frontend-Shell).
- `curl http://192.168.122.13:8080/health` → `HTTP 200`,
  `{"service":"control-server","status":"ok"}`.
- MQTT-Clientlogs (telemetry-service/fleet-service/vehicle-mock) zeigen `MQTT connected` ohne
  Fehler nach den obigen Fixes.

### LOCALVM-09 — Doku umgestellt

- `docs/deployment/hetzner-setup.md`: neuer Hinweisblock am Anfang zum Ansible-Workflow (ersetzt
  Schritt 1/3/4/6/7 als empfohlenen Weg, Rest bleibt für den echten Server unverändert gültig);
  Schritt 1 um lokalen-VM-Hinweis ergänzt; Schritt 2 MQTT-Portzeile auf TLS-8883 korrigiert (samt
  Erklärungsbox zur früheren 1883-Falschangabe); Schritt 3 (Bootstrap-Bash-Block) und Schritt 4
  (Secrets-Script-Volltext) durch Kurzbeschreibung + Ansible-/Skript-Verweis ersetzt (kein
  dupliziertes, driftgefährdetes Skript-Volltext-Abbild mehr in der Doku); Schritt 6 (Config-Kopie)
  um die zuvor fehlenden Mosquitto-TLS-Dateien (`passwd`/`ca.pem`/`cert.pem`/`key.pem`) ergänzt
  plus Berechtigungs-Hinweis (`chmod 644`, s. Bugfix 4 oben); Schritt 7 (`deploy-hetzner.sh`-
  Volltext) durch Kurzbeschreibung + `SKIP_REGISTRY_PULL`-Hinweis ersetzt; Schritt 9 + "Erreichbare
  Services"-Tabelle auf Ansible-Aufruf bzw. TLS-Port 8883 aktualisiert.
- `docs/deployment/UEBERGABE-ABWEICHUNGEN.md` neu angelegt (war an zwei Stellen referenziert —
  `docker-compose.prod.yml`, `scripts/deploy.sh` — existierte aber nie): dokumentiert die
  Docker-Hub-Abweichung von ADR-019 (lokaler `docker save`/`load`-Weg statt Registry) inkl.
  Revert-Pfad, sowie den analogen Sprint-48/49-Befund für die lokale VM (Save/Load-Weg setzt
  aktuell gebaute Images voraus).
- `DECISIONS.MD`: neue Zeile in "Offene Folge-Entscheidungen" — EPIC als Sprint-48+49 abgeschlossen
  markiert, inkl. Kurzfassung aller 5 in Sprint 49 gefundenen Bugs.
- `tasks/backlog.md`: EPIC-Status auf "✅ vollständig abgeschlossen" gesetzt, LOCALVM-08 in
  08a/08b/08c aufgesplittet (passend zum Sprint-49-Auftrag) und alle vier Zeilen (08a/08b/08c/09)
  auf ✅ Sprint 49 gesetzt.

### Nicht Teil dieses Sprints (wie im Auftrag)

Echter Hetzner-Server, Stilllegung des AWS-Pfads, CI-Integration, Let's-Encrypt — unverändert
offen, siehe EPIC "Nicht Teil dieses Vorhabens" in `tasks/backlog.md`.
