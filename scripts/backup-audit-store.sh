#!/bin/bash
# backup-audit-store.sh — Tägliches pg_dump-Backup des Audit Store (PostgreSQL) nach S3.
#
# ADR-018/023-Folge: der Audit Store lief bis Sprint 7 auf SQLite, seit ADR-023 auf PostgreSQL
# (postgres-data-Volume). pg_dump gegen den laufenden Container statt Volume-Kopie, da ein
# Snapshot während laufendem Betrieb inkonsistent sein kann — pg_dump liefert einen konsistenten
# logischen Dump ohne Service-Stopp. Kein pg_basebackup/WAL-Archivierung (Point-in-Time-Recovery) —
# für dieses Betriebsmodell (Single-Instance-Testbetrieb, kein HA-Anspruch) reicht ein tägliches
# logisches Backup.
#
# Voraussetzung auf EC2 (analog deploy.sh):
#   - docker + docker compose plugin installiert
#   - aws cli installiert
#   - IAM Instance Profile mit SSM-Leseberechtigung auf /avoc/* und S3-Schreibrechten auf AppBucket
#   - docker-compose.prod.yml liegt in APP_DIR, Stack läuft (postgres-Service healthy)
#   - Registrierung als täglicher Cronjob erfolgt automatisch durch deploy.sh
#
# Verwendung:
#   AWS_REGION=eu-central-1 bash ~/app/backup-audit-store.sh
#
set -euo pipefail

REGION=${AWS_REGION:-eu-central-1}
APP_DIR=${APP_DIR:-$(dirname "$(realpath "$0")")}
COMPOSE_FILE=${COMPOSE_FILE:-docker-compose.prod.yml}

echo "=== AVOC Audit-Store-Backup === Region: $REGION"

get() {
  aws ssm get-parameter \
    --region "$REGION" \
    --name "$1" \
    --query Parameter.Value \
    --output text
}

BUCKET=$(get /avoc/prod/backup-bucket-name)

TIMESTAMP=$(date +%F)
DUMP_FILE="/tmp/${TIMESTAMP}-avoc.sql.gz"
S3_KEY="backups/postgres/${TIMESTAMP}-avoc.sql.gz"

echo "Erzeuge Dump..."
cd "$APP_DIR"
docker compose -f "$COMPOSE_FILE" exec -T postgres pg_dump -U avoc avoc | gzip > "$DUMP_FILE"
echo "  Dump erzeugt: $DUMP_FILE ($(du -h "$DUMP_FILE" | cut -f1))"

echo "Lade nach s3://$BUCKET/$S3_KEY hoch..."
aws s3 cp --region "$REGION" "$DUMP_FILE" "s3://$BUCKET/$S3_KEY"

rm -f "$DUMP_FILE"

echo "=== Backup abgeschlossen: s3://$BUCKET/$S3_KEY — $(date) ==="
