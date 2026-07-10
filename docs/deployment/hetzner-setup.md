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

---

## Schritt 1 — Server anlegen (Hetzner Cloud Console)

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
| 1883 | TCP | MQTT Broker |
| 3001 | TCP | Grafana |
| 3100 | TCP | Loki (optional, intern) |
| 3478 | TCP + UDP | STUN/TURN (coturn) |
| 5349 | TCP + UDP | TURN-TLS (coturn) |
| 8189 | UDP | MediaMTX ICE-Mux (WebRTC Media) |
| 49152–65535 | UDP | TURN Relay-Ports (coturn) |

Firewall dem Server zuweisen: **Server** → `avoc-server` → **Firewalls** → Zuweisen.

> **Wichtig — UDP 8189:** Dieser Port ist entscheidend für WebRTC-Mediadaten.
> Ohne ihn schlägt ICE fehl (WHIP-Signaling gelingt, kein Video).
>
> **Wichtig — UDP 49152–65535:** Für TURN-Relay-Kandidaten (Fallback bei CGNAT/5G).

---

## Schritt 3 — Server-Bootstrap (einmalig)

```bash
SERVER_IP=5.161.x.x    # Public-IPv4 aus Schritt 1

ssh root@${SERVER_IP}
```

Im Server-Terminal das folgende Bootstrap-Script ausführen:

```bash
#!/bin/bash
set -euo pipefail

# ─── System aktualisieren ────────────────────────────────────────────────────
apt-get update && apt-get upgrade -y

# ─── Docker installieren ─────────────────────────────────────────────────────
apt-get install -y ca-certificates curl gnupg
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] \
  https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
  > /etc/apt/sources.list.d/docker.list
apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# ─── Deployment-User anlegen ─────────────────────────────────────────────────
useradd -m -s /bin/bash avoc
usermod -aG docker avoc

# ─── Authorized Keys für avoc-User kopieren ──────────────────────────────────
mkdir -p /home/avoc/.ssh
cp /root/.authorized_keys /home/avoc/.ssh/authorized_keys 2>/dev/null || \
  cp /root/.ssh/authorized_keys /home/avoc/.ssh/authorized_keys
chown -R avoc:avoc /home/avoc/.ssh
chmod 700 /home/avoc/.ssh
chmod 600 /home/avoc/.ssh/authorized_keys

# ─── App-Verzeichnis anlegen ─────────────────────────────────────────────────
mkdir -p /home/avoc/app/mosquitto
mkdir -p /home/avoc/app/mediamtx
mkdir -p /home/avoc/loki
mkdir -p /home/avoc/promtail
mkdir -p /home/avoc/grafana/provisioning/datasources
mkdir -p /home/avoc/grafana/provisioning/dashboards
chown -R avoc:avoc /home/avoc/

# ─── Docker autostart ────────────────────────────────────────────────────────
systemctl enable docker
systemctl start docker

echo "Bootstrap abgeschlossen. Login als 'avoc' per SSH möglich."
```

Prüfen:
```bash
docker compose version   # → Docker Compose version v2.x.x
id avoc                  # → groups: docker
```

---

## Schritt 4 — Secrets einrichten (einmalig, vom Dev-Rechner)

Da Hetzner keinen SSM Parameter Store hat, werden Secrets in einer `.env`-Datei
auf dem Server gespeichert. Die Datei gehört `avoc:avoc`, ist nicht lesbar für
andere Nutzer (chmod 600) und liegt außerhalb des Source-Codes.

### Script: `scripts/secrets-setup-hetzner.sh`

Dieses Script liest Secrets interaktiv ein und schreibt sie direkt auf den Server:

```bash
#!/bin/bash
# secrets-setup-hetzner.sh — Richtet .env auf Hetzner Server ein.
# Verwendung: SERVER_IP=5.161.x.x bash scripts/secrets-setup-hetzner.sh
set -euo pipefail

SERVER_IP=${SERVER_IP:?Bitte SERVER_IP setzen}

echo "=== AVOC Hetzner Secrets Setup ==="
echo "Server: ${SERVER_IP}"
echo ""
echo "Bitte Werte eingeben:"
echo ""

read -rp   "SERVER_IP (Public-IPv4 des Hetzner Servers): " server_ip
server_ip=${server_ip:-$SERVER_IP}

read -rp   "JWT_SECRET (min. 32 Zeichen): " jwt_secret
[ ${#jwt_secret} -lt 32 ] && echo "ERROR: Zu kurz." && exit 1

read -rsp  "DB_PASSWORD (min. 16 Zeichen): " db_password; echo ""
[ ${#db_password} -lt 16 ] && echo "ERROR: Zu kurz." && exit 1

read -rsp  "ADMIN_PASSWORD (min. 12 Zeichen): " admin_password; echo ""
[ ${#admin_password} -lt 12 ] && echo "ERROR: Zu kurz." && exit 1

read -rsp  "WHIP_STREAM_KEY (min. 32 Zeichen): " whip_stream_key; echo ""
[ ${#whip_stream_key} -lt 32 ] && echo "ERROR: Zu kurz." && exit 1

read -rp   "TURN_REALM [avoc.example.com]: " turn_realm
turn_realm=${turn_realm:-avoc.example.com}

read -rp   "TURN_USER [avoc]: " turn_user
turn_user=${turn_user:-avoc}

read -rsp  "TURN_PASSWORD (min. 16 Zeichen): " turn_password; echo ""
[ ${#turn_password} -lt 16 ] && echo "ERROR: Zu kurz." && exit 1

read -rp   "GRAFANA_ADMIN_USER [admin]: " grafana_user
grafana_user=${grafana_user:-admin}

read -rsp  "GRAFANA_ADMIN_PASSWORD (min. 12 Zeichen): " grafana_password; echo ""
[ ${#grafana_password} -lt 12 ] && echo "ERROR: Zu kurz." && exit 1

read -rp   "DOCKER_USERNAME (Docker Hub Benutzername): " docker_username
[ -z "$docker_username" ] && echo "ERROR: Pflichtfeld." && exit 1

read -rsp  "DOCKER_PASSWORD (Docker Hub Access Token): " docker_password; echo ""
[ -z "$docker_password" ] && echo "ERROR: Pflichtfeld." && exit 1

# .env auf Server schreiben
ENV_CONTENT=$(cat <<EOF
# AVOC Hetzner Deployment Secrets
# Erstellt: $(date -u '+%Y-%m-%d %H:%M UTC')
# ACHTUNG: chmod 600 — nicht committen, nicht loggen

REGISTRY=docker.io/${docker_username}
VERSION=latest

JWT_SECRET=${jwt_secret}
DB_PASSWORD=${db_password}
ADMIN_PASSWORD=${admin_password}
WHIP_STREAM_KEY=${whip_stream_key}

TURN_EXTERNAL_IP=${server_ip}
TURN_REALM=${turn_realm}
TURN_USER=${turn_user}
TURN_PASSWORD=${turn_password}

GRAFANA_ADMIN_USER=${grafana_user}
GRAFANA_ADMIN_PASSWORD=${grafana_password}

DOCKER_USERNAME=${docker_username}
DOCKER_PASSWORD=${docker_password}
EOF
)

echo ""
echo "Schreibe /home/avoc/app/.env auf Server..."
ssh avoc@${SERVER_IP} "cat > /home/avoc/app/.env && chmod 600 /home/avoc/app/.env" <<< "$ENV_CONTENT"
echo "  /home/avoc/app/.env geschrieben (chmod 600)"

# SSL-Zertifikat generieren (für HTTPS/getUserMedia)
echo ""
echo "Generiere Self-Signed SSL-Zertifikat..."
ssh avoc@${SERVER_IP} "
  mkdir -p /home/avoc/app/ssl
  if [ ! -f /home/avoc/app/ssl/cert.pem ]; then
    openssl req -x509 -newkey rsa:2048 \
      -keyout /home/avoc/app/ssl/key.pem \
      -out    /home/avoc/app/ssl/cert.pem \
      -days 365 -nodes \
      -subj '/CN=${server_ip}' \
      -addext 'subjectAltName=IP:${server_ip}' 2>/dev/null
    echo '  SSL-Zertifikat erstellt.'
  else
    echo '  SSL-Zertifikat bereits vorhanden.'
  fi
"

echo ""
echo "=== Secrets Setup abgeschlossen ==="
```

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

```bash
SERVER_IP=5.161.x.x

# deploy-hetzner.sh (liegt in scripts/)
scp scripts/deploy-hetzner.sh                             avoc@${SERVER_IP}:~/app/

# docker-compose.hetzner.yml
scp infrastructure/compose/docker-compose.hetzner.yml     avoc@${SERVER_IP}:~/app/

# Konfigurationsdateien
scp infrastructure/mosquitto/mosquitto.conf               avoc@${SERVER_IP}:~/app/mosquitto/
scp infrastructure/mediamtx/mediamtx.yml                  avoc@${SERVER_IP}:~/app/mediamtx/

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
│   │   └── mosquitto.conf
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

## Schritt 7 — `deploy-hetzner.sh` (Script-Inhalt)

Das Deploy-Script für Hetzner entfällt der AWS-spezifische Code (SSM, IMDS).
Secrets kommen aus der `.env`-Datei, die in Schritt 4 angelegt wurde.

Inhalt von `scripts/deploy-hetzner.sh`:

```bash
#!/bin/bash
# deploy-hetzner.sh — AVOC Deployment auf Hetzner Server.
# Liest Secrets aus .env (kein AWS SSM).
# Kein IMDSv2 — Hetzner-Server kennen ihre Public-IP direkt.
#
# Voraussetzung auf Server:
#   - docker + docker compose plugin installiert
#   - /home/avoc/app/.env vorhanden (chmod 600), enthält alle Secrets
#   - docker-compose.hetzner.yml liegt in APP_DIR
#
# Verwendung:
#   VERSION=latest bash ~/app/deploy-hetzner.sh
#
set -euo pipefail

APP_DIR=${APP_DIR:-$(dirname "$(realpath "$0")")}
VERSION=${VERSION:-latest}

echo "=== AVOC Hetzner Deploy === Version: $VERSION"
echo ""

# ─── Pflichtdateien prüfen ───────────────────────────────────────────────────

REQUIRED_FILES=(
  "$APP_DIR/docker-compose.hetzner.yml"
  "$APP_DIR/.env"
  "$APP_DIR/mediamtx/mediamtx.yml"
  "$APP_DIR/mosquitto/mosquitto.conf"
)
for f in "${REQUIRED_FILES[@]}"; do
  [ ! -f "$f" ] && echo "ERROR: Pflichtdatei fehlt: $f" && exit 1
done

# ─── Secrets aus .env laden ──────────────────────────────────────────────────

echo "[1/4] Lade Secrets aus .env..."
set -a
# shellcheck source=/dev/null
source "$APP_DIR/.env"
set +a

export VERSION

echo "  REGISTRY            ${REGISTRY}"
echo "  TURN_EXTERNAL_IP    ${TURN_EXTERNAL_IP}"
echo "  TURN_REALM          ${TURN_REALM}"
echo "  VERSION             ${VERSION}"

# ─── SSL-Zertifikat ──────────────────────────────────────────────────────────

SSL_DIR="$APP_DIR/ssl"
mkdir -p "$SSL_DIR"
if [ ! -f "$SSL_DIR/cert.pem" ]; then
  echo "Generiere Self-Signed SSL-Zertifikat für ${TURN_EXTERNAL_IP}..."
  openssl req -x509 -newkey rsa:2048 \
    -keyout "$SSL_DIR/key.pem" \
    -out    "$SSL_DIR/cert.pem" \
    -days 365 -nodes \
    -subj "/CN=${TURN_EXTERNAL_IP}" \
    -addext "subjectAltName=IP:${TURN_EXTERNAL_IP}" 2>/dev/null
  echo "  Zertifikat erstellt."
else
  echo "  SSL-Zertifikat vorhanden."
fi

# ─── Docker Hub Login ─────────────────────────────────────────────────────────

echo ""
echo "[2/4] Docker Hub Login..."
echo "$DOCKER_PASSWORD" | docker login \
  --username "$DOCKER_USERNAME" \
  --password-stdin

# ─── Images pullen ────────────────────────────────────────────────────────────

echo ""
echo "[3/4] Pull Images..."
cd "$APP_DIR"
docker compose -f docker-compose.hetzner.yml pull

# ─── Stack starten ────────────────────────────────────────────────────────────

echo ""
echo "[4/4] Start Stack..."
docker compose -f docker-compose.hetzner.yml up -d

echo ""
echo "=== Deploy abgeschlossen — $(date) ==="
echo ""
echo "Status:"
docker compose -f docker-compose.hetzner.yml ps
```

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

```bash
ssh avoc@${SERVER_IP}
cd ~/app
VERSION=latest bash deploy-hetzner.sh
```

Stack-Status prüfen:
```bash
docker compose -f ~/app/docker-compose.hetzner.yml ps
```

Alle Container sollten `running` / `healthy` sein.

---

## Erreichbare Services nach Deploy

| Service | URL |
|---|---|
| Operator UI (Frontend, HTTP) | `http://<SERVER_IP>:3000` |
| Operator UI (Frontend, HTTPS) | `https://<SERVER_IP>` (Self-Signed) |
| Control Server API | `http://<SERVER_IP>:8080` |
| MQTT Broker (Vehicle) | `<SERVER_IP>:1883` |
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
