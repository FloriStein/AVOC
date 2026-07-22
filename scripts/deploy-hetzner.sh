#!/bin/bash
# deploy-hetzner.sh — AVOC Deployment auf Hetzner Server.
# 1:1 materialisiert aus docs/deployment/hetzner-setup.md Schritt 7 (LOCALVM-06, Sprint 48),
# mit EINER dokumentierten, optionalen Ergänzung (s. "SKIP_REGISTRY_PULL" unten) — alles andere
# unverändert.
#
# Liest Secrets aus .env (kein AWS SSM).
# Kein IMDSv2 — Hetzner-Server kennen ihre Public-IP direkt.
#
# Voraussetzung auf Server:
#   - docker + docker compose plugin installiert
#   - /home/avoc/app/.env vorhanden (chmod 600), enthält alle Secrets
#   - docker-compose.hetzner.yml liegt in APP_DIR
#
# Verwendung (Standardfall, seit Sprint 57 GHCRPULL-04 — GHCR-Pull statt Docker Hub):
#   VERSION=latest bash ~/app/deploy-hetzner.sh
#   Login-Registry wird aus REGISTRY (.env, z. B. "ghcr.io/floristein") abgeleitet — der Teil
#   vor dem ersten "/" ist der Host, den `docker login` erwartet. DOCKER_USERNAME/
#   DOCKER_PASSWORD (Namen unverändert aus der Docker-Hub-Ära, s. "Nicht Teil dieses Sprints"
#   in tasks/backlog.md EPIC "GHCR-Pull-Deployment") enthalten den GHCR-Nutzernamen + PAT.
#
# SKIP_REGISTRY_PULL=true (Fallback, s. ansible/deploy.yml avoc_skip_registry_pull):
#   Überspringt Registry-Login + 'docker compose pull' und prüft stattdessen nur, ob die
#   benötigten Images bereits lokal vorhanden sind (identisches Muster wie scripts/deploy.sh
#   auf EC2, "Übergabe-Abweichung von ADR-019" — s. docker-compose.prod.yml-Kommentar).
#   Default seit Sprint 57: false (GHCR-Pull ist der Normalfall, auch für die lokale Test-VM —
#   s. tasks/backlog.md EPIC "GHCR-Pull-Deployment").
#
set -euo pipefail

APP_DIR=${APP_DIR:-$(dirname "$(realpath "$0")")}
VERSION=${VERSION:-latest}
SKIP_REGISTRY_PULL=${SKIP_REGISTRY_PULL:-false}

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

cd "$APP_DIR"

if [ "$SKIP_REGISTRY_PULL" = "true" ]; then
  # Lokale Test-VM (kein Docker-Hub-Roundtrip) — Images wurden bereits per
  # ansible/deploy.yml (docker save/load) auf diese Maschine übertragen.
  echo ""
  echo "[2/4] SKIP_REGISTRY_PULL=true — kein Docker Hub Login/Pull, prüfe lokale Images..."
  docker compose -f docker-compose.hetzner.yml config --images | while read -r img; do
    docker image inspect "$img" >/dev/null 2>&1 || echo "  WARNUNG: Image fehlt lokal: $img"
  done

  echo ""
  echo "[3/4] (übersprungen — kein Pull bei SKIP_REGISTRY_PULL=true)"
else
  # ─── Registry-Login ──────────────────────────────────────────────────────────
  # Host = Teil von REGISTRY vor dem ersten "/" (z. B. "ghcr.io/floristein" -> "ghcr.io").
  REGISTRY_HOST="${REGISTRY%%/*}"
  echo ""
  echo "[2/4] Registry-Login (${REGISTRY_HOST})..."
  echo "$DOCKER_PASSWORD" | docker login "$REGISTRY_HOST" \
    --username "$DOCKER_USERNAME" \
    --password-stdin

  # ─── Images pullen ──────────────────────────────────────────────────────────
  echo ""
  echo "[3/4] Pull Images..."
  docker compose -f docker-compose.hetzner.yml pull
fi

# ─── Stack starten ────────────────────────────────────────────────────────────

echo ""
echo "[4/4] Start Stack..."
docker compose -f docker-compose.hetzner.yml up -d

echo ""
echo "=== Deploy abgeschlossen — $(date) ==="
echo ""
echo "Status:"
docker compose -f docker-compose.hetzner.yml ps
