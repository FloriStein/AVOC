#!/bin/bash
# deploy.sh — AVOC Deployment auf EC2.
# Holt Secrets aus AWS SSM Parameter Store, loggt sich in Docker Hub ein,
# pulled alle Images und startet den Stack.
#
# Voraussetzung auf EC2:
#   - docker + docker compose plugin installiert
#   - aws cli installiert
#   - IAM Instance Profile mit SSM-Leseberechtigung auf /avoc/*
#   - docker-compose.prod.yml liegt in APP_DIR
#   - mediamtx/mediamtx.yml liegt in APP_DIR (aws s3 cp ... ~/app/mediamtx/mediamtx.yml)
#   - mosquitto/mosquitto.conf liegt in APP_DIR (mosquitto/passwd wird von diesem Skript aus
#     dem SSM-Secret generiert, siehe MQTTAUTH-04 — nicht committen/manuell anlegen)
#
# Verwendung:
#   AWS_REGION=eu-central-1 VERSION=latest bash ~/app/deploy.sh
#
set -euo pipefail

REGION=${AWS_REGION:-eu-central-1}
APP_DIR=${APP_DIR:-$(dirname "$(realpath "$0")")}
VERSION=${VERSION:-latest}

echo "=== AVOC Deploy === Region: $REGION  Version: $VERSION"
echo ""

# ─── Voraussetzungen prüfen ───────────────────────────────────────────────────

REQUIRED_FILES=(
  "$APP_DIR/docker-compose.prod.yml"
  "$APP_DIR/mediamtx/mediamtx.yml"
  "$APP_DIR/mosquitto/mosquitto.conf"
)
for f in "${REQUIRED_FILES[@]}"; do
  if [ ! -f "$f" ]; then
    echo "ERROR: Pflichtdatei fehlt: $f" && exit 1
  fi
done

# ─── SSM Parameter lesen ──────────────────────────────────────────────────────

get() {
  aws ssm get-parameter \
    --region "$REGION" \
    --name "$1" \
    --query Parameter.Value \
    --output text
}

get_secure() {
  aws ssm get-parameter \
    --region "$REGION" \
    --name "$1" \
    --with-decryption \
    --query Parameter.Value \
    --output text
}

echo "[1/5] Lade Secrets..."

# DB_PASSWORD + ADMIN_PASSWORD kommen aus $APP_DIR/.env (docker-compose liest sie automatisch).
# Alle anderen Secrets kommen aus AWS SSM Parameter Store.
ENV_FILE="$APP_DIR/.env"
if [ ! -f "$ENV_FILE" ]; then
  echo "ERROR: $ENV_FILE fehlt — bitte DB_PASSWORD und ADMIN_PASSWORD dort eintragen." && exit 1
fi

export JWT_SECRET=$(get_secure          /avoc/prod/jwt-secret)
export WHIP_STREAM_KEY=$(get_secure     /avoc/prod/whip-stream-key)
export TURN_EXTERNAL_IP=$(get           /avoc/prod/turn-external-ip)
export TURN_REALM=$(get                 /avoc/prod/turn-realm)
export TURN_USER=$(get                  /avoc/prod/turn-user)
export TURN_PASSWORD=$(get_secure       /avoc/prod/turn-password)
export MQTT_USERNAME=$(get              /avoc/prod/mqtt-username)
export MQTT_PASSWORD=$(get_secure       /avoc/prod/mqtt-password)
export GRAFANA_ADMIN_USER=$(get         /avoc/prod/grafana-admin-user)
export GRAFANA_ADMIN_PASSWORD=$(get_secure /avoc/prod/grafana-admin-password)

# EC2 Private IP aus Instance Metadata Service (IMDS v2) — für coturn relay-ip + external-ip=PUBLIC/PRIVATE.
# IMDSv2 erfordert Token-Header (Standard auf Amazon Linux 2023).
_IMDS_TOKEN=$(curl -sf -X PUT "http://169.254.169.254/latest/api/token" \
  -H "X-aws-ec2-metadata-token-ttl-seconds: 21600")
export TURN_PRIVATE_IP=$(curl -sf -H "X-aws-ec2-metadata-token: ${_IMDS_TOKEN}" \
  http://169.254.169.254/latest/meta-data/local-ipv4)

export VERSION

echo "  JWT_SECRET          loaded (SSM)"
echo "  DB_PASSWORD         from .env"
echo "  ADMIN_PASSWORD      from .env"
echo "  WHIP_STREAM_KEY     loaded (SSM)"
echo "  TURN_EXTERNAL_IP    ${TURN_EXTERNAL_IP}"
echo "  TURN_PRIVATE_IP     ${TURN_PRIVATE_IP}"
echo "  TURN_REALM          ${TURN_REALM}"
echo "  MQTT_USERNAME       loaded (SSM)"
echo "  VERSION             ${VERSION}  (Images bereits lokal via 'docker load' vorhanden — kein Registry-Pull)"

# ─── Self-Signed SSL-Zertifikat (für getUserMedia auf HTTPS) ──────────────────
# Browser blockiert getUserMedia auf HTTP — HTTPS mit self-signed cert umgeht das.
# Einmalig generieren; bei erneutem Deploy wird es wiederverwendet.

SSL_DIR="$APP_DIR/ssl"
mkdir -p "$SSL_DIR"
if [ ! -f "$SSL_DIR/cert.pem" ]; then
  echo "Generiere Self-Signed SSL-Zertifikat für $TURN_EXTERNAL_IP..."
  openssl req -x509 -newkey rsa:2048 \
    -keyout "$SSL_DIR/key.pem" \
    -out    "$SSL_DIR/cert.pem" \
    -days 365 -nodes \
    -subj "/CN=${TURN_EXTERNAL_IP}" \
    -addext "subjectAltName=IP:${TURN_EXTERNAL_IP}" 2>/dev/null
  echo "  Zertifikat erstellt: $SSL_DIR/cert.pem"
else
  echo "  SSL-Zertifikat vorhanden: $SSL_DIR/cert.pem"
fi

# ─── Mosquitto Passwort-File (MQTTAUTH-04) ────────────────────────────────────
# Der Broker verweigert seit MQTTAUTH-01 anonyme Verbindungen (allow_anonymous false) und
# erwartet eine gehashte Passwort-Datei unter mosquitto/passwd. Wird bei jedem Deploy neu aus dem
# SSM-Secret generiert statt committed (anders als die Dev-/Test-Datei mit ihrem festen,
# unkritischen changeme-Passwort) — Prod-Credential darf nicht im Repo landen.

echo ""
echo "[2/5] Generiere Mosquitto-Passwort-Datei..."
docker run --rm -v "$APP_DIR/mosquitto:/out" --entrypoint mosquitto_passwd \
  eclipse-mosquitto:2 -b -c /out/passwd "$MQTT_USERNAME" "$MQTT_PASSWORD" >/dev/null
echo "  mosquitto/passwd erzeugt für Benutzer $MQTT_USERNAME"

# ─── Cron-Registrierung: tägliches Audit-Store-Backup (AUDITBACKUP-03) ────────
# backup-audit-store.sh liegt (analog deploy.sh selbst) in APP_DIR neben
# docker-compose.prod.yml. Idempotent: bei erneutem Deploy kein doppelter Cron-Eintrag.

echo ""
echo "[3/5] Prüfe Cron-Registrierung für Audit-Store-Backup..."
CRON_CMD="0 3 * * * /usr/bin/env bash $APP_DIR/backup-audit-store.sh >> $APP_DIR/backup-audit-store.log 2>&1"
(crontab -l 2>/dev/null | grep -qF "backup-audit-store.sh") || \
  { (crontab -l 2>/dev/null; echo "$CRON_CMD") | crontab -; }
echo "  Cron-Eintrag vorhanden: backup-audit-store.sh täglich 03:00 UTC"

# ─── Stack starten ────────────────────────────────────────────────────────────
# Kein Docker-Hub-Login, kein 'docker compose pull' — Images liegen bereits lokal
# (per 'make deploy-images' via docker save/scp/docker load übertragen).
# Übergabe-Abweichung von ADR-019, siehe docs/deployment/UEBERGABE-ABWEICHUNGEN.md

echo ""
echo "[4/5] Prüfe Images..."
cd "$APP_DIR"
docker compose -f docker-compose.prod.yml config --images | while read -r img; do
  docker image inspect "$img" >/dev/null 2>&1 || echo "  WARNUNG: Image fehlt lokal: $img (siehe 'make deploy-images')"
done

echo ""
echo "[5/5] Start Stack..."
docker compose -f docker-compose.prod.yml up -d

echo ""
echo "=== Deploy abgeschlossen — $(date) ==="
echo ""
echo "Status:"
docker compose -f docker-compose.prod.yml ps
