#!/bin/bash
# secrets-setup-hetzner.sh — Richtet .env auf Hetzner Server ein.
# 1:1 materialisiert aus docs/deployment/hetzner-setup.md Schritt 4 (LOCALVM-06, Sprint 48).
#
# Für die lokale Test-VM (Sprint 48 EPIC) unverändert NICHT genutzt — dort übernimmt
# ansible/roles/secrets dieselbe Aufgabe aus fest hinterlegten Test-Dummy-Werten (kein
# interaktiver Prompt bei jedem VM-Neuaufbau). Dieses Skript bleibt für den späteren echten
# Hetzner-Server nutzbar.
#
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
