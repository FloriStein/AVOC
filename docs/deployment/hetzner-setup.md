# Hetzner Server Deployment Guide — AVOC

Dieses Dokument beschreibt den vollständigen Deployment-Prozess auf einem **Hetzner Cloud Server**
als Alternative zur AWS EC2-Infrastruktur (EC2-Guide: [ec2-bootstrap.md](ec2-bootstrap.md)).

Wesentliche Unterschiede zu AWS EC2:

| Aspekt | AWS EC2 | Hetzner Cloud |
|--------|---------|--------------|
| Secrets | AWS SSM Parameter Store | `.env`-Datei auf dem Server (chmod 600) |
| IP-Architektur | EIP via NAT (Instanz sieht nur private IP) | Public-IP direkt auf `eth0` |
| coturn-Konfiguration | `--relay-ip=PRIVATE --external-ip=PUBLIC/PRIVATE` | `--external-ip=PUBLIC` (kein NAT-Mapping nötig) |
| Server-Zugang | SSH oder AWS SSM Session Manager | SSH mit Key-Auth |
| Firewall | CDK Security Group | Hetzner Cloud Firewall oder `ufw` |
| Private IP auslesen | IMDSv2 Metadata Service | Nicht nötig — Public IP ist direkt zugreifbar |

---

## Übersicht

```
Dev-Rechner                           Hetzner Server
──────────────────────                ─────────────────────────
1. Server anlegen (Cloud Console)  ──▶ Ubuntu 24.04 LTS
2. hetzner-bootstrap.sh ausführen  ──▶ Docker + User + Firewall
3. secrets-setup.sh ausführen      ──▶ /home/avoc/app/.env (chmod 600)
4. make push                       ──▶ Docker Hub (private Repos)
5. Configs kopieren + deployen     ──▶ Stack läuft
```

> **Ansible-Workflow (seit Sprint 48/49) — empfohlener Weg, ersetzt Schritt 1/3/4/6/7:**
> Statt der manuellen Bash-Blöcke/`scp`-Befehle unten gibt es inzwischen `ansible/` (Rollen
> `bootstrap`/`firewall`/`secrets` + `site.yml`/`deploy.yml`). Ursprünglich für eine **lokale
> Verifikations-VM** gebaut (Hetzner-Nachbildung via `scripts/local-vm-create.sh` + libvirt/KVM,
> siehe `tasks/backlog.md` EPIC "Lokale Ansible-VM als Hetzner-Nachbildung"), funktionieren die
> Rollen unverändert auch gegen einen echten frischen Hetzner-Server — `roles/bootstrap` ist für
> beide Fälle idempotent gebaut (`ansible_user=avoc` bei der lokalen VM vs. `ansible_user=root`
> beim Erstbootstrap eines echten Servers, siehe Kommentare in `ansible/inventory/hosts.ini` und
> `roles/bootstrap/tasks/main.yml`). Für einen echten Server: eigene Inventory-Datei anlegen
> (analog `ansible/inventory/hosts.ini`, aber mit der Server-IP + `ansible_user=root` für den
> Erstlauf) und `ansible-playbook site.yml && ansible-playbook deploy.yml` ausführen. Schritt 2
> (Firewall-Portliste), 5 (Images bauen/pushen), 8 (Compose-Unterschiede zu EC2) und 9 (erster
> Deploy, jetzt per Ansible) bleiben unverändert bzw. mit Ansible-Verweis gültig — siehe dort.
> Real gegen eine VM verifiziert in Sprint 49 (`tasks/sprints/49-lokale-ansible-vm-teil-b.md`),
> inklusive drei dabei gefundener und behobener echter Bugs (SSH-Upgrade-Race, apt-Architektur-
> Mismatch, ufw-Kommentar-Quoting) und zwei Deploy-seitiger (Mosquitto-Dateiberechtigungen,
> MQTT-Test-Passwort) — Details in den Kommentaren der jeweiligen `ansible/`-Dateien.

---

## Schritt 1 — Server anlegen (Hetzner Cloud Console)

> **Lokale Verifikations-VM statt echtem Server:** `bash scripts/local-vm-create.sh` erzeugt via
> virt-install/cloud-init eine lokale Ubuntu-24.04-VM als Näherung dieses Schritts (gleiche
> RAM/vCPU-Empfehlung wie unten, `avoc`-User bereits per cloud-init statt manuell angelegt,
> `ansible/inventory/hosts.ini` wird automatisch mit der VM-IP befüllt). Kein Hetzner-Account
> nötig. Danach direkt mit dem Ansible-Workflow (Hinweis oben) fortfahren — dieser Schritt hier
> (Cloud-Console-Bedienung) ist dann nicht nötig.

Unter [console.hetzner.cloud](https://console.hetzner.cloud):

1. **Projekt** öffnen → **Server** → **Server hinzufügen**
2. Empfohlene Konfiguration:

| Parameter | Empfehlung |
|-----------|-----------|
| Standort | Nürnberg (nbg1) oder Falkenstein (fsn1) |
| Image | Ubuntu 24.04 LTS |
| Typ | **CPX21** (3 vCPU, 4 GB RAM) oder größer |
| SSH-Key | Vorhandenen Key auswählen / hochladen |
| Name | `avoc-server` |

> **RAM-Hinweis:** Der Stack hat 13+ Container. CPX11 (2 GB) ist zu knapp;
> CPX21 (4 GB) empfohlen. CX31 (8 GB) für Produktivbetrieb.

3. Nach dem Erstellen: **Public-IPv4** notieren (z. B. `5.161.x.x`).
   Diese IP ist statisch (solange der Server nicht gelöscht wird).

**Optional — Hetzner Floating IP:**
Entspricht der AWS Elastic IP — bleibt bei Server-Rebuild erhalten.
Anlegen: Netzwerk → Floating IPs → Floating IP hinzufügen → dem Server zuweisen.

---

## Schritt 2 — Hetzner Cloud Firewall konfigurieren

In der Cloud Console: **Firewalls** → **Firewall erstellen** → Regeln:

### Eingehend (Inbound)

| Port | Protokoll | Zweck |
|------|-----------|-------|
| 22 | TCP | SSH |
| 80 | TCP | HTTP (optional, für Let's Encrypt Challenge) |
| 443 | TCP | HTTPS |
| 3000 | TCP | Operator UI (Frontend) |
| 8080 | TCP | Control Server API |
| 8081 | TCP | Auth Service |
| 8082 | TCP | Safety Service |
| 8083 | TCP | Telemetry Service |
| 8084 | TCP | WebRTC SFU |
| 8889 | TCP | MediaMTX WHIP/WHEP Signaling |
| 9997 | TCP | MediaMTX Management API (optional, intern) |
| 8883 | TCP | MQTT Broker (TLS — seit MQTTS-01/MQTTAUTH-04, siehe Hinweis unten) |
| 3001 | TCP | Grafana |
| 3100 | TCP | Loki (optional, intern) |
| 3478 | TCP + UDP | STUN/TURN (coturn) |
| 5349 | TCP + UDP | TURN-TLS (coturn) |
| 8189 | UDP | MediaMTX ICE-Mux (WebRTC Media) |
| 49152–65535 | UDP | TURN Relay-Ports (coturn) |

Kein Eintrag für Port 8085 (Fleet Service) — `fleet-service` ist zwar Teil des Dev-Stacks, aber
(Stand jetzt) nicht in `docker-compose.prod.yml` eingetragen und läuft daher auf keinem per
`deploy.sh`/dieser Anleitung aufgesetzten Produktiv-Server, siehe `docs/deployment/ec2-bootstrap.md`
Abschnitt "Services & zugehöriger Quellcode".

Firewall dem Server zuweisen: **Server** → `avoc-server` → **Firewalls** → Zuweisen.

> **MQTT-Port-Korrektur (Sprint 49, LOCALVM-09):** Diese Tabelle nannte hier bis Sprint 48
> Port 1883 (Klartext-MQTT) — veraltet seit MQTTS-01/MQTTAUTH-04, Mosquitto läuft im
> tatsächlichen Compose-Stand (`docker-compose.prod.yml`/`docker-compose.hetzner.yml`) TLS-only
> auf Port 8883, Port 1883 ist im Compose-Setup nicht mal mehr nach außen exponiert. Korrigiert
> auf den echten Stand. Der Ansible-Workflow (`ansible/roles/firewall/defaults/main.yml`) setzt
> ohnehin automatisch beide Regeln (1883 als Doku-Altlast-Kommentar + der tatsächlich benötigte
> 8883) — bei manueller Firewall-Konfiguration reicht 8883.
>
> **Wichtig — UDP 8189:** Dieser Port ist entscheidend für WebRTC-Mediadaten.
> Ohne ihn schlägt ICE fehl (WHIP-Signaling gelingt, kein Video).
>
> **Wichtig — UDP 49152–65535:** Für TURN-Relay-Kandidaten (Fallback bei CGNAT/5G).

---

## Schritt 3 — Server-Bootstrap (einmalig)

**Per Ansible (empfohlen, seit Sprint 48/49):** `ansible/roles/bootstrap` deckt exakt das ab, was
früher hier als manueller Bash-Block dokumentiert war — System-Update, Docker-Installation,
`avoc`-User anlegen, `authorized_keys` übernehmen, App-Verzeichnisstruktur anlegen — idempotent
und wiederholbar. Für einen echten, frischen Hetzner-Server: eigene Inventory-Datei mit
`ansible_user=root` (root hat den Hetzner-Cloud-SSH-Key bereits, `roles/bootstrap` übernimmt
`authorized_keys` für den neu angelegten `avoc`-User automatisch von dort), dann:

```bash
SERVER_IP=5.161.x.x    # Public-IPv4 aus Schritt 1
cd ansible
ansible-playbook -i <eigene-inventory-mit-root-user>.ini site.yml
```

`roles/bootstrap` enthält (Stand Sprint 49, real gegen eine VM verifiziert) außerdem eine Absicherung
gegen einen bekannten Ubuntu-24.04-Fallstrick: ein direktes `apt upgrade` bricht ab, sobald ein
`openssh-server`-Update enthalten ist (der Dienst wird durch sein eigenes Postinst-Skript
neugestartet, während genau diese SSH-Verbindung noch den Ansible-Task ausführt) — der Task läuft
daher async, entkoppelt vom SSH-Kanal, mit anschließendem Verbindungs-Reset + Wartephase (siehe
Kommentar in `roles/bootstrap/tasks/main.yml`).

**Manuell (Fallback ohne Ansible):** Der ursprüngliche Bash-Block ist weiterhin in der
Sprint-Historie dokumentiert (`tasks/sprints/` vor Sprint 48) und lässt sich unverändert per SSH
ausführen, falls `ansible-playbook` auf dem Dev-Rechner nicht verfügbar ist — inhaltlich deckungsgleich
mit `roles/bootstrap/tasks/main.yml`, dort als kommentierte, idempotente Ansible-Tasks gepflegt
(die maßgebliche, aktuell gehaltene Fassung).

Prüfen:
```bash
ssh avoc@${SERVER_IP} "docker compose version && id avoc"   # → Compose v2.x.x, groups: docker
```

---

## Schritt 4 — Secrets einrichten (einmalig, vom Dev-Rechner)

Da Hetzner keinen SSM Parameter Store hat, werden Secrets in einer `.env`-Datei
auf dem Server gespeichert. Die Datei gehört `avoc:avoc`, ist nicht lesbar für
andere Nutzer (chmod 600) und liegt außerhalb des Source-Codes.

### Script: `scripts/secrets-setup-hetzner.sh`

Dieses Script liest Secrets interaktiv ein (JWT_SECRET, DB_PASSWORD, ADMIN_PASSWORD,
WHIP_STREAM_KEY, TURN_REALM/USER/PASSWORD, GRAFANA_ADMIN_USER/PASSWORD, DOCKER_USERNAME/PASSWORD —
je mit Mindestlängen-Prüfung), schreibt sie direkt als `/home/avoc/app/.env` (chmod 600) auf den
Server und generiert das Self-Signed-SSL-Zertifikat. Für den vollständigen, aktuell gehaltenen
Skriptinhalt siehe `scripts/secrets-setup-hetzner.sh` direkt (hier nicht dupliziert, um
Doku-Drift wie bei den in Sprint 49 gefundenen Lücken zu vermeiden).

```bash
SERVER_IP=5.161.x.x bash scripts/secrets-setup-hetzner.sh
```

> **Lokale Verifikations-VM:** `ansible/roles/secrets` ersetzt dieses interaktive Script für die
> lokale VM durch fest hinterlegte Test-Dummy-Werte (kein echtes, internet-exponiertes System,
> siehe Kopfkommentar der Rolle) — läuft automatisch als Teil von `ansible-playbook site.yml`,
> keine manuelle Eingabe nötig. **Wichtig, falls die generierten Mosquitto-Zugangsdaten geändert
> werden:** `avoc_secrets_mqtt_password` in `roles/secrets/defaults/main.yml` muss zum
> Klartext-Passwort passen, mit dem `infrastructure/mosquitto/passwd` gehasht wurde (Default:
> `avoc`/`changeme`, siehe `tasks/sprints/38-mqtt-authentifizierung.md`) — `ansible/deploy.yml`
> kopiert diese Datei unverändert, anders als `secrets-setup-hetzner.sh`, das für den echten
> Server keine Mosquitto-Datei anfasst (das übernimmt weiterhin `scripts/deploy.sh`-artige
> Logik bzw. muss für den Hetzner-Produktivpfad noch ergänzt werden, s. `MQTTAUTH-04`-Notizen).
> Ein Mismatch äußert sich als "not Authorized" in den MQTT-Client-Logs (telemetry-service/
> fleet-service/vehicle-mock) — real in Sprint 49 gefunden und auf den korrekten Wert korrigiert.

---

## Schritt 5 — Images bauen und nach Docker Hub pushen (Dev-Rechner)

Identisch zu EC2:

```bash
# Docker Hub Username setzen (einmalig)
echo "dein-dockerhub-username" > .docker-username

# Alle Images für linux/amd64 bauen und pushen
make push

# Oder mit explizitem Versionstag:
VERSION=1.0.0 make push
```

---

## Schritt 6 — Deployment-Dateien auf Hetzner kopieren

**Per Ansible (empfohlen):** `ansible/deploy.yml` übernimmt den kompletten Config-Transfer dieses
Schritts als eigene Tasks (`docker-compose.hetzner.yml`, `deploy-hetzner.sh`, Mosquitto/MediaMTX/
Loki/Promtail/Grafana-Configs) — inklusive der Mosquitto-TLS-Dateien (`passwd`, `ca.pem`,
`cert.pem`, `key.pem`), die in der ursprünglichen `scp`-Liste unten seit MQTTS-01/MQTTAUTH-04
fehlten (Doku-Lücke aus Sprint 48, hier in Sprint 49 korrigiert):

```bash
cd ansible
ansible-playbook -i <inventory>.ini deploy.yml
```

**Manuell (Fallback ohne Ansible)** — vollständige, korrigierte Liste inkl. der TLS-Dateien:

```bash
SERVER_IP=5.161.x.x

# deploy-hetzner.sh (liegt in scripts/)
scp scripts/deploy-hetzner.sh                             avoc@${SERVER_IP}:~/app/

# docker-compose.hetzner.yml
scp infrastructure/compose/docker-compose.hetzner.yml     avoc@${SERVER_IP}:~/app/

# Mosquitto: Config + TLS-Assets (passwd/ca.pem/cert.pem/key.pem — seit MQTTS-01/MQTTAUTH-04
# nötig, TLS-only auf Port 8883, s. Hinweis in Schritt 2)
scp infrastructure/mosquitto/mosquitto.conf                avoc@${SERVER_IP}:~/app/mosquitto/
scp infrastructure/mosquitto/passwd                        avoc@${SERVER_IP}:~/app/mosquitto/
scp infrastructure/mosquitto/certs/ca.pem                  avoc@${SERVER_IP}:~/app/mosquitto/
scp infrastructure/mosquitto/certs/cert.pem                avoc@${SERVER_IP}:~/app/mosquitto/
scp infrastructure/mosquitto/certs/key.pem                 avoc@${SERVER_IP}:~/app/mosquitto/
scp infrastructure/mediamtx/mediamtx.yml                   avoc@${SERVER_IP}:~/app/mediamtx/

# Loki + Promtail (bind-mount Pfade)
scp infrastructure/loki/loki.yml                          avoc@${SERVER_IP}:~/loki/loki.yml
scp infrastructure/promtail/promtail.yml                  avoc@${SERVER_IP}:~/promtail/promtail.yml

# Grafana Provisioning
scp infrastructure/grafana/provisioning/datasources/loki.yml \
                                                          avoc@${SERVER_IP}:~/grafana/provisioning/datasources/
scp infrastructure/grafana/provisioning/dashboards/dashboards.yml \
    infrastructure/grafana/provisioning/dashboards/avoc.json \
                                                          avoc@${SERVER_IP}:~/grafana/provisioning/dashboards/
```

> **Achtung, real in Sprint 49 gefunden:** `passwd`/`key.pem` NICHT mit `chmod 600` auf dem
> Server ablegen, obwohl das für Secrets naheliegend wirkt — der `eclipse-mosquitto:2`-Container
> konnte die Datei sonst nicht öffnen ("Unable to open pwfile", Container-Crash-Loop). `chmod 644`
> verwenden (für die hier verwendeten Test-/Dev-Credentials ausreichend, s. Kommentar in
> `ansible/deploy.yml`).

Erwartete Verzeichnisstruktur auf dem Server:
```
/home/avoc/
├── app/
│   ├── docker-compose.hetzner.yml
│   ├── deploy-hetzner.sh
│   ├── .env                          ← chmod 600, secrets
│   ├── ssl/
│   │   ├── cert.pem
│   │   └── key.pem
│   ├── mosquitto/
│   │   ├── mosquitto.conf
│   │   ├── passwd                    ← chmod 644, s. Hinweis oben
│   │   ├── ca.pem
│   │   ├── cert.pem
│   │   └── key.pem                   ← chmod 644, s. Hinweis oben
│   └── mediamtx/
│       └── mediamtx.yml
├── loki/
│   └── loki.yml
├── promtail/
│   └── promtail.yml
└── grafana/
    └── provisioning/
        ├── datasources/loki.yml
        └── dashboards/
            ├── dashboards.yml
            └── avoc.json
```

---

## Schritt 7 — `deploy-hetzner.sh` (Ablauf)

Das Deploy-Script für Hetzner entfällt der AWS-spezifische Code (SSM, IMDS). Secrets kommen aus
der `.env`-Datei, die in Schritt 4 angelegt wurde. Für den vollständigen, aktuell gehaltenen
Skriptinhalt siehe `scripts/deploy-hetzner.sh` direkt (hier nicht dupliziert — der frühere
vollständige Abdruck in dieser Doku war bereits veraltet, s. Sprint-49-Hinweis unten).

Ablauf in Kurzform: Pflichtdateien prüfen → Secrets aus `.env` laden → Self-Signed-SSL-Zertifikat
generieren (falls noch nicht vorhanden) → Docker-Images bereitstellen → Stack starten
(`docker compose up -d`) → Status ausgeben.

**Per Ansible (empfohlen):** `ansible-playbook deploy.yml` ruft `deploy-hetzner.sh` automatisch
mit `SKIP_REGISTRY_PULL=true` auf (Images wurden zuvor bereits per `docker save`/`docker load`
auf den Server übertragen, s. Schritt 6 — kein Docker-Hub-Login/-Pull nötig, s.
`docs/deployment/UEBERGABE-ABWEICHUNGEN.md` Abweichung 2).

**Manuell (regulärer Docker-Hub-Weg für den echten Server, Standardfall laut diesem Dokument):**

```bash
ssh avoc@${SERVER_IP}
cd ~/app
VERSION=latest bash deploy-hetzner.sh
```

`SKIP_REGISTRY_PULL=true VERSION=latest bash deploy-hetzner.sh` ist die manuelle Variante des
Ansible-Aufrufs oben (kein Docker-Hub-Schritt, prüft stattdessen nur, ob die benötigten Images
bereits lokal auf dem Server vorhanden sind) — nur sinnvoll, wenn die Images vorher bereits anders
auf den Server gelangt sind (z. B. per manuellem `docker save`/`docker load`).

---

## Schritt 8 — `docker-compose.hetzner.yml` (Unterschiede zu prod.yml)

Die Hetzner-Variante unterscheidet sich vom EC2-Stand in zwei Punkten:

**1. coturn — kein NAT-Hairpin-Mapping nötig:**

Auf AWS EC2 kennt die Instanz ihre Elastic IP nicht auf dem Netzwerk-Interface
(AWS NAT-Übersetzung). Daher war das Mapping `--external-ip=PUBLIC/PRIVATE` nötig.
Auf Hetzner liegt die Public-IP direkt auf `eth0` — kein Mapping erforderlich:

```yaml
# EC2 (alt):
stun-turn:
  command: >
    --relay-ip=${TURN_PRIVATE_IP}
    --external-ip=${TURN_EXTERNAL_IP}/${TURN_PRIVATE_IP}
    ...

# Hetzner (neu):
stun-turn:
  command: >
    --external-ip=${TURN_EXTERNAL_IP}
    ...
```

`TURN_PRIVATE_IP` entfällt vollständig (keine IMDS-Abfrage nötig).

**2. Volumes für Loki/Promtail — anderer Basisverzeichnis-Pfad:**

Auf EC2 lagen die Konfigurationen unter `~/` (also `/home/ec2-admin/`).
Auf Hetzner liegt der Nutzer unter `/home/avoc/`:

```yaml
# EC2 (alt):
loki:
  volumes:
    - ../loki/loki.yml:/etc/loki/config.yaml:ro

# Hetzner (neu — absoluter Pfad):
loki:
  volumes:
    - /home/avoc/loki/loki.yml:/etc/loki/config.yaml:ro
```

Vollständige `docker-compose.hetzner.yml` erstellen:

```bash
# Vom Dev-Rechner aus:
cp infrastructure/compose/docker-compose.prod.yml infrastructure/compose/docker-compose.hetzner.yml
```

Dann in `docker-compose.hetzner.yml` folgende Änderungen vornehmen:

| Stelle | Ersetzen | Durch |
|--------|----------|-------|
| coturn `command` | `--relay-ip=${TURN_PRIVATE_IP}` → entfernen | (leer) |
| coturn `command` | `--external-ip=${TURN_EXTERNAL_IP}/${TURN_PRIVATE_IP}` | `--external-ip=${TURN_EXTERNAL_IP}` |
| loki `volumes` | `- ../loki/loki.yml:...` | `- /home/avoc/loki/loki.yml:...` |
| promtail `volumes` | `- ../promtail/promtail.yml:...` | `- /home/avoc/promtail/promtail.yml:...` |
| grafana `volumes` | `- ../grafana/provisioning:...` | `- /home/avoc/grafana/provisioning:...` |

---

## Schritt 9 — Erster Deploy auf dem Server

**Per Ansible (empfohlen):** Schritt 3+4+6+7 zusammen als zwei Befehle vom Dev-Rechner aus (siehe
Hinweis am Dokumentanfang):

```bash
cd ansible
ansible-playbook -i <inventory>.ini site.yml     # Bootstrap + Firewall + Secrets
ansible-playbook -i <inventory>.ini deploy.yml   # Config-Transfer + Images + Stack-Start
```

**Manuell:**
```bash
ssh avoc@${SERVER_IP}
cd ~/app
VERSION=latest bash deploy-hetzner.sh
```

Stack-Status prüfen:
```bash
docker compose -f ~/app/docker-compose.hetzner.yml ps
```

Alle Container sollten `running` sein (`postgres` zusätzlich `healthy`) — real gegen die lokale
Verifikations-VM geprüft in Sprint 49 (LOCALVM-08c, alle 15 Container liefen stabil, keine
Restart-Loops).

---

## Erreichbare Services nach Deploy

| Service | URL |
|---|---|
| Operator UI (Frontend, HTTP) | `http://<SERVER_IP>:3000` |
| Operator UI (Frontend, HTTPS) | `https://<SERVER_IP>` (Self-Signed) |
| Control Server API | `http://<SERVER_IP>:8080` (Health-Check: `/health`) |
| MQTT Broker (Vehicle, TLS) | `<SERVER_IP>:8883` (seit MQTTS-01/MQTTAUTH-04 TLS-only, s. Hinweis Schritt 2) |
| Grafana | `http://<SERVER_IP>:3001` |
| STUN/TURN | `<SERVER_IP>:3478` |

> **HTTPS-Hinweis:** `getUserMedia` (Kamerazugriff im Browser) erfordert HTTPS oder localhost.
> Beim ersten Aufruf von `https://<SERVER_IP>` zeigt der Browser eine Zertifikatswarnung
> (Self-Signed). Ausnahme hinzufügen → danach volle Funktionalität.

---

## Rollback

```bash
ssh avoc@${SERVER_IP}
cd ~/app
VERSION=<alter-tag> bash deploy-hetzner.sh
```

---

## Updates einspielen

```bash
# Dev-Rechner: neue Images bauen und pushen
VERSION=1.1.0 make push

# Server: neue Version deployen
ssh avoc@${SERVER_IP} "VERSION=1.1.0 bash ~/app/deploy-hetzner.sh"
```

---

## Troubleshooting

**Container startet nicht:**
```bash
docker compose -f ~/app/docker-compose.hetzner.yml logs <service-name>
```

**coturn TURN relay funktioniert nicht:**
- Hetzner Firewall: Port 3478 TCP+UDP und UDP 49152–65535 offen?
- `TURN_EXTERNAL_IP` in `.env` korrekt (Public-IP des Servers)?
- Auf Hetzner ist kein `relay-ip` nötig — coturn bindet automatisch an `0.0.0.0`
  und advertised die `external-ip` korrekt

**WebRTC kein Video (ICE schlägt fehl):**
- UDP 8189 in Hetzner Firewall offen?
- `webrtcAdditionalHosts` in `mediamtx.yml` auf Server-IP gesetzt?
  ```yaml
  webrtcAdditionalHosts:
    - <SERVER_IP>
  ```
- MediaMTX-Logs prüfen:
  ```bash
  docker compose -f ~/app/docker-compose.hetzner.yml logs mediamtx
  ```

**getUserMedia schlägt fehl (HTTP):**
- Browser erlaubt Kamerazugriff nur auf HTTPS oder localhost
- HTTPS via Self-Signed: `https://<SERVER_IP>` aufrufen, Zertifikatsausnahme bestätigen
- Oder: Domain + Let's Encrypt (siehe unten)

**Speicher knapp:**
```bash
free -h
docker stats --no-stream
```
Bei dauerhaft >80% RAM: Upgrade auf CPX31 (8 GB) im Hetzner Cloud Console
(Server-Typ ändern — Downtime ca. 2–3 Min.).

**SSL-Zertifikat erneuern (Self-Signed):**
```bash
rm ~/app/ssl/cert.pem ~/app/ssl/key.pem
bash ~/app/deploy-hetzner.sh  # generiert automatisch neues Zertifikat
```

---

## Optional — Let's Encrypt (Domain statt Self-Signed)

Wenn eine Domain auf die Server-IP zeigt, kann `certbot` ein echtes TLS-Zertifikat ausstellen.
Dann entfällt die Browser-Zertifikatswarnung und `getUserMedia` funktioniert ohne Ausnahme.

```bash
# Certbot installieren
apt-get install -y certbot

# Zertifikat ausstellen (Port 80 muss kurz frei sein)
docker compose -f ~/app/docker-compose.hetzner.yml stop frontend
certbot certonly --standalone -d avoc.example.com
docker compose -f ~/app/docker-compose.hetzner.yml start frontend

# Zertifikate in ssl/ kopieren (nginx erwartet diese Pfade)
cp /etc/letsencrypt/live/avoc.example.com/fullchain.pem ~/app/ssl/cert.pem
cp /etc/letsencrypt/live/avoc.example.com/privkey.pem   ~/app/ssl/key.pem

# Auto-Renewal einrichten
echo "0 3 * * * root certbot renew --quiet && \
  cp /etc/letsencrypt/live/avoc.example.com/fullchain.pem /home/avoc/app/ssl/cert.pem && \
  cp /etc/letsencrypt/live/avoc.example.com/privkey.pem /home/avoc/app/ssl/key.pem && \
  docker compose -f /home/avoc/app/docker-compose.hetzner.yml restart frontend" \
  > /etc/cron.d/avoc-certbot
```

In `docker-compose.hetzner.yml` den nginx-Port 80 → 443 und die `server_name` in `nginx.conf`
auf die Domain umstellen.

---

## Fahrzeug-Kamera (WHIP-Publish)

Identisch zum EC2-Setup — nur die IP ändert sich:

- WHIP-URL: `http://<SERVER_IP>:8889/vehicle-001/whip`
- Authorization: `Bearer <WHIP_STREAM_KEY>` (aus `.env` auf dem Server)
