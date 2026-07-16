#!/usr/bin/env bash
# Sprint 23 (MAP-01): seeds a demo outdoor zone + stations via fleet-service's existing REST
# CRUD (POST /fleet/zones, POST /fleet/stations) so the map visualization has something real to
# render. No DB migration, no new Go code — this only exercises endpoints that already exist
# (FLEET-05). Idempotent: skips seeding if the zone already exists.
#
# Coordinates deliberately encompass cmd/vehicle-mock/fleet_simulator.go's hardcoded demo path
# (52.130100,11.640100) <-> (52.130500,11.641200) — the simulated lastenzug-01/lastenrad-01
# vehicles physically move within these bounds, so seeding anything narrower would render live
# vehicle markers outside the zone/station layout.
#
# Usage: BASE_URL=http://localhost:3000 ADMIN_PASSWORD=... ./scripts/seed-fleet-demo.sh

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:3000}"
ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin_dev_secret}"

ZONE_ID="zone-betriebshof-nord"
ZONE_NAME="Betriebshof Nord"

echo "[seed-fleet-demo] Logging in as ${ADMIN_USERNAME} against ${BASE_URL} ..."
LOGIN_RESPONSE=$(curl -sf -X POST "${BASE_URL}/auth/operator/login" \
  -H 'Content-Type: application/json' \
  -d "$(python3 -c 'import json,sys; print(json.dumps({"username": sys.argv[1], "password": sys.argv[2]}))' "${ADMIN_USERNAME}" "${ADMIN_PASSWORD}")")

TOKEN=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])' <<<"${LOGIN_RESPONSE}")

if [[ -z "${TOKEN}" ]]; then
  echo "[seed-fleet-demo] ERROR: no token in login response: ${LOGIN_RESPONSE}" >&2
  exit 1
fi

echo "[seed-fleet-demo] Checking for existing zone '${ZONE_ID}' ..."
EXISTING_ZONES=$(curl -sf "${BASE_URL}/fleet/zones" -H "Authorization: Bearer ${TOKEN}")
ALREADY_SEEDED=$(python3 -c '
import json, sys
# GET /fleet/zones returns JSON null (not []) when the zones table is empty — Go leaves the
# backing slice nil for zero rows, which json.Marshal renders as null.
zones = json.load(sys.stdin) or []
print("yes" if any(z.get("id") == sys.argv[1] for z in zones) else "no")
' "${ZONE_ID}" <<<"${EXISTING_ZONES}")

if [[ "${ALREADY_SEEDED}" == "yes" ]]; then
  echo "[seed-fleet-demo] Zone '${ZONE_ID}' already exists — nothing to do (idempotent)."
  exit 0
fi

# Simple, dark-theme-matching facility layout: two hall outlines + a loading zone. Rendered as a
# geo-referenced overlay stretched across geo_bounds — the SVG's internal viewBox is not
# geographically precise per-pixel, it just fills the outer zone bounding box.
SVG_GEOMETRY=$(cat <<'SVG'
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 300">
  <rect x="4" y="4" width="392" height="292" fill="none" stroke="#4b5563" stroke-width="2" stroke-dasharray="6 4"/>
  <rect x="40" y="40" width="160" height="100" fill="#374151" stroke="#9ca3af" stroke-width="2"/>
  <text x="120" y="95" font-size="14" fill="#d1d5db" text-anchor="middle">Halle 1</text>
  <rect x="230" y="150" width="130" height="90" fill="#374151" stroke="#9ca3af" stroke-width="2"/>
  <text x="295" y="200" font-size="14" fill="#d1d5db" text-anchor="middle">Halle 2</text>
  <rect x="40" y="180" width="150" height="60" fill="#1f2937" stroke="#6b7280" stroke-width="2" stroke-dasharray="4 3"/>
  <text x="115" y="215" font-size="12" fill="#9ca3af" text-anchor="middle">Ladezone</text>
</svg>
SVG
)

# geo_bounds must be sent as a JSON-encoded STRING, not a nested object — Zone.GeoBounds is Go
# *string (internal/fleetservice/store.go), so json.Decode into the Zone struct requires a string
# value on the wire. See docs/adr/029-fleet-vehicle-data-model.md's dated format update (MAP-02).
GEO_BOUNDS_JSON='{"sw":{"lat":52.1295,"lon":11.6390},"ne":{"lat":52.1312,"lon":11.6425}}'

ZONE_PAYLOAD=$(python3 -c '
import json, sys
zone_id, name, svg, geo_bounds = sys.argv[1:5]
print(json.dumps({
    "id": zone_id,
    "name": name,
    "environment": "outdoor",
    "svg_geometry": svg,
    "geo_bounds": geo_bounds,
}))
' "${ZONE_ID}" "${ZONE_NAME}" "${SVG_GEOMETRY}" "${GEO_BOUNDS_JSON}")

echo "[seed-fleet-demo] Creating zone '${ZONE_NAME}' (${ZONE_ID}) ..."
curl -sf -X POST "${BASE_URL}/fleet/zones" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${TOKEN}" \
  -d "${ZONE_PAYLOAD}" >/dev/null

# Two stations exactly match vehicle-mock's hardcoded demo-station-a/b coordinates (so seeded
# stations and the simulated vehicles' endpoints visually coincide on the map), three more are
# plausible facility points inside the same bounds but off the simulated path — the current
# simulator never visits them (documented limitation, not a bug).
create_station() {
  local id="$1" name="$2" lat="$3" lon="$4"
  local payload
  payload=$(python3 -c '
import json, sys
station_id, zone_id, name, lat, lon = sys.argv[1:6]
print(json.dumps({
    "id": station_id,
    "zone_id": zone_id,
    "name": name,
    "position_lat": float(lat),
    "position_lon": float(lon),
}))
' "${id}" "${ZONE_ID}" "${name}" "${lat}" "${lon}")
  echo "[seed-fleet-demo] Creating station '${name}' (${id}) ..."
  curl -sf -X POST "${BASE_URL}/fleet/stations" \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer ${TOKEN}" \
    -d "${payload}" >/dev/null
}

create_station "station-ladezone-a" "Ladezone A"          52.130100 11.640100
create_station "station-ladezone-b" "Ladezone B"          52.130500 11.641200
create_station "station-wartung"    "Wartungsbereich"     52.129800 11.639700
create_station "station-verwaltung" "Verwaltungsgebäude"  52.131000 11.642000
create_station "station-tor"        "Einfahrtstor"        52.129600 11.641500

echo "[seed-fleet-demo] Done. Verifying geo_bounds round-trips as parseable JSON ..."
python3 -c '
import json, sys
zones = json.load(sys.stdin)
zone = next(z for z in zones if z["id"] == sys.argv[1])
parsed = json.loads(zone["geo_bounds"])
print(f"  geo_bounds OK: {parsed}")
' "${ZONE_ID}" < <(curl -sf "${BASE_URL}/fleet/zones" -H "Authorization: Bearer ${TOKEN}")

echo "[seed-fleet-demo] Seed complete."
