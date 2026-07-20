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

Datum: 2026-07-20 | **Status: Geplant, zur Ausführung an Agenten übergeben (teilweise blockiert,
siehe oben)**
Branch/Worktree: `feature/fleet-service-foundation-localvm` (unverändert seit Sprint 48).

## Tasks

| ID | Task | Typ | Status | Abhängigkeiten |
|----|------|-----|--------|-----------------|
| LOCALVM-08a | VM-Erzeugung: `scripts/local-vm-create.sh` real ausführen (Ubuntu-24.04-Cloud-Image, cloud-init, SSH-Erreichbarkeit abwarten), Ansible-Inventory-Befüllung verifizieren. Kein Ansible nötig — nicht vom Sudo-Blocker betroffen. | M | 🔲 | — |
| LOCALVM-08b | Ansible-Lauf: `ansible/site.yml` (bootstrap+firewall+secrets) und `ansible/deploy.yml` (Config-Transfer, Image-Transfer ohne Docker-Hub, `deploy-hetzner.sh`) real gegen die VM ausführen, Fehler iterativ beheben. Braucht funktionierendes `ansible-playbook` — **blockiert bis Nutzer `sudo apt-get install -y ansible` ausgeführt hat.** | M | 🔲 | LOCALVM-08a, Ansible-Install |
| LOCALVM-08c | Smoke-Test gegen den laufenden Stack in der VM: `docker compose ps` (alle Container healthy/running), HTTP-Erreichbarkeit Frontend (Port 3000)/Control-Server-API (Port 8080) von der VM-IP aus, analog Sprint-41-CI-Verifikationsmuster. | S | 🔲 | LOCALVM-08b |
| LOCALVM-09 | Doku: `docs/deployment/hetzner-setup.md` auf den Ansible-Workflow umstellen (Schritt 1/3/4/6/7 durch Verweis auf `local-vm-create.sh` + Ansible-Rollen ersetzen, Rest bleibt für den echten Server gültig); dabei die zwei aus Sprint 48 bekannten Doku-Lücken mitkorrigieren (Mosquitto-Port/Copy-Liste auf TLS-8883-Stand, `UEBERGABE-ABWEICHUNGEN.md` entweder anlegen oder Referenzen entfernen); `DECISIONS.MD`, `tasks/backlog.md`-Status-Update (EPIC komplett ✅). | S/M | 🔲 | LOCALVM-08c |

**Nicht Teil dieses Sprints:** echter Hetzner-Server, Stilllegung des AWS-Pfads, CI-Integration,
Let's-Encrypt — siehe EPIC "Nicht Teil dieses Vorhabens" in `tasks/backlog.md`.

**Hinweis für die Ausführung:** Falls `ansible-playbook` zu Beginn der Ausführung noch nicht
verfügbar ist: LOCALVM-08a trotzdem vollständig durchführen (kein Blocker), Ergebnis
dokumentieren, dann mit klarer Statusmeldung pausieren statt zu raten/zu simulieren. Kein
`--syntax-check`-Ersatz für den echten Lauf verwenden und als "verifiziert" ausgeben — Teil A hat
das bereits getan, Teil B muss den echten Lauf zeigen. Nach Abschluss: Ergebnisse unter
"## Ergebnisse" dokumentieren, nach `tasks/sprints/49-lokale-ansible-vm-teil-b.md` archivieren,
`tasks/done.md` + `tasks/backlog.md`-Status aktualisieren, committen (kein Push).
