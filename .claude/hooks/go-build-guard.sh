#!/bin/bash
set -euo pipefail
COMMAND=$(jq -r '.tool_input.command // empty')

# Nur bare "go build" oder explizite Einzelziele ohne -o und ohne "./..." matchen
if echo "$COMMAND" | grep -qE '(^|[;&|]+)\s*go build(\s|$)' \
   && ! echo "$COMMAND" | grep -q -- '-o ' \
   && ! echo "$COMMAND" | grep -qE 'go build\s+\./\.\.\.'; then
  echo "go build ohne -o und ohne './...' kann bestehende Root-Binaries überschreiben (Sprint-28-Binary-Vorfall, siehe tasks/sprints/28-risikoarme-services-rule22-23.md:131-134). Zielpfad angeben (-o <pfad>) oder go build ./... / go vet ./... für den reinen Compile-Check nutzen." >&2
  exit 2
fi
exit 0
