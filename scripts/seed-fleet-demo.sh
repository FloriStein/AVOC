#!/usr/bin/env bash
# Sprint 23 (MAP-01): seeds a demo outdoor zone + stations via fleet-service's existing REST
# CRUD (POST /fleet/zones, POST /fleet/stations) so the map visualization has something real to
# render. No DB migration, no new Go code — this only exercises endpoints that already exist
# (FLEET-05). Sprint 33 (ADR-034) added a second, indoor example zone. Both zones are seeded
# independently and idempotently (skipping one because it already exists must not skip the
# other — each has its own existence check, not a single early-exit for the whole script).
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

zone_exists() {
  local zone_id="$1"
  python3 -c '
import json, sys
# GET /fleet/zones returns JSON null (not []) when the zones table is empty — Go leaves the
# backing slice nil for zero rows, which json.Marshal renders as null.
zones = json.load(sys.stdin) or []
print("yes" if any(z.get("id") == sys.argv[1] for z in zones) else "no")
' "${zone_id}" < <(curl -sf "${BASE_URL}/fleet/zones" -H "Authorization: Bearer ${TOKEN}")
}

seed_outdoor_zone() {
  local zone_id="zone-betriebshof-nord"
  local zone_name="Betriebshof Nord"

  echo "[seed-fleet-demo] Checking for existing zone '${zone_id}' ..."
  if [[ "$(zone_exists "${zone_id}")" == "yes" ]]; then
    echo "[seed-fleet-demo] Zone '${zone_id}' already exists — skipping (idempotent)."
    return 0
  fi

  # Simple, dark-theme-matching facility layout: two hall outlines + a loading zone. Rendered as
  # a geo-referenced overlay stretched across geo_bounds — the SVG's internal viewBox is not
  # geographically precise per-pixel, it just fills the outer zone bounding box.
  local svg_geometry
  svg_geometry=$(cat <<'SVG'
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
  # *string (internal/fleetservice/store.go), so json.Decode into the Zone struct requires a
  # string value on the wire. See docs/adr/029-fleet-vehicle-data-model.md's dated format update
  # (MAP-02).
  local geo_bounds_json='{"sw":{"lat":52.1295,"lon":11.6390},"ne":{"lat":52.1312,"lon":11.6425}}'

  local zone_payload
  zone_payload=$(python3 -c '
import json, sys
zone_id, name, svg, geo_bounds = sys.argv[1:5]
print(json.dumps({
    "id": zone_id,
    "name": name,
    "environment": "outdoor",
    "svg_geometry": svg,
    "geo_bounds": geo_bounds,
}))
' "${zone_id}" "${zone_name}" "${svg_geometry}" "${geo_bounds_json}")

  echo "[seed-fleet-demo] Creating zone '${zone_name}' (${zone_id}) ..."
  curl -sf -X POST "${BASE_URL}/fleet/zones" \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer ${TOKEN}" \
    -d "${zone_payload}" >/dev/null

  # Two stations exactly match vehicle-mock's hardcoded demo-station-a/b coordinates (so seeded
  # stations and the simulated vehicles' endpoints visually coincide on the map), three more are
  # plausible facility points inside the same bounds but off the simulated path — the current
  # simulator never visits them (documented limitation, not a bug).
  create_outdoor_station() {
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
' "${id}" "${zone_id}" "${name}" "${lat}" "${lon}")
    echo "[seed-fleet-demo] Creating station '${name}' (${id}) ..."
    curl -sf -X POST "${BASE_URL}/fleet/stations" \
      -H 'Content-Type: application/json' \
      -H "Authorization: Bearer ${TOKEN}" \
      -d "${payload}" >/dev/null
  }

  create_outdoor_station "station-ladezone-a" "Ladezone A"          52.130100 11.640100
  create_outdoor_station "station-ladezone-b" "Ladezone B"          52.130500 11.641200
  create_outdoor_station "station-wartung"    "Wartungsbereich"     52.129800 11.639700
  create_outdoor_station "station-verwaltung" "Verwaltungsgebäude"  52.131000 11.642000
  create_outdoor_station "station-tor"        "Einfahrtstor"        52.129600 11.641500

  echo "[seed-fleet-demo] Verifying geo_bounds round-trips as parseable JSON ..."
  python3 -c '
import json, sys
zones = json.load(sys.stdin)
zone = next(z for z in zones if z["id"] == sys.argv[1])
parsed = json.loads(zone["geo_bounds"])
print(f"  geo_bounds OK: {parsed}")
' "${zone_id}" < <(curl -sf "${BASE_URL}/fleet/zones" -H "Authorization: Bearer ${TOKEN}")
}

# seed_indoor_zone (ADR-034, Sprint 33): not geo-referenced (no geo_bounds, environment=indoor) —
# svg_geometry's viewBox IS the coordinate system stations/vehicles are positioned in via
# position_x/y, rendered as a plain SVG floor plan rather than a Leaflet overlay
# (FleetIndoorMap.tsx, no lat/lon involved at all). Serves as example data for AP2-02's
# Indoor-Kartenrendering; no vehicle currently reports a position_zone_id/position_x/y inside this
# zone (ADR-034 Scope — deliberately deferred to a follow-up task), so the frontend renders the
# floor plan + stations here but no vehicle marker without seeding a vehicle_status row separately.
seed_indoor_zone() {
  local zone_id="zone-lager-indoor"
  local zone_name="Lager (Indoor)"

  echo "[seed-fleet-demo] Checking for existing zone '${zone_id}' ..."
  if [[ "$(zone_exists "${zone_id}")" == "yes" ]]; then
    echo "[seed-fleet-demo] Zone '${zone_id}' already exists — skipping (idempotent)."
    return 0
  fi

  local svg_geometry
  svg_geometry=$(cat <<'SVG'
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 300">
  <rect x="4" y="4" width="392" height="292" fill="none" stroke="#4b5563" stroke-width="2"/>
  <rect x="20" y="20" width="150" height="260" fill="#1f2937" stroke="#6b7280" stroke-width="2"/>
  <text x="95" y="155" font-size="14" fill="#9ca3af" text-anchor="middle">Regalzone</text>
  <rect x="190" y="20" width="190" height="120" fill="#374151" stroke="#9ca3af" stroke-width="2"/>
  <text x="285" y="85" font-size="14" fill="#d1d5db" text-anchor="middle">Kommissionierung</text>
  <rect x="190" y="160" width="190" height="120" fill="#374151" stroke="#9ca3af" stroke-width="2"/>
  <text x="285" y="225" font-size="14" fill="#d1d5db" text-anchor="middle">Verladung</text>
</svg>
SVG
)

  local zone_payload
  zone_payload=$(python3 -c '
import json, sys
zone_id, name, svg = sys.argv[1:4]
print(json.dumps({
    "id": zone_id,
    "name": name,
    "environment": "indoor",
    "svg_geometry": svg,
}))
' "${zone_id}" "${zone_name}" "${svg_geometry}")

  echo "[seed-fleet-demo] Creating zone '${zone_name}' (${zone_id}) ..."
  curl -sf -X POST "${BASE_URL}/fleet/zones" \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer ${TOKEN}" \
    -d "${zone_payload}" >/dev/null

  create_indoor_station() {
    local id="$1" name="$2" x="$3" y="$4"
    local payload
    payload=$(python3 -c '
import json, sys
station_id, zone_id, name, x, y = sys.argv[1:6]
print(json.dumps({
    "id": station_id,
    "zone_id": zone_id,
    "name": name,
    "position_x": float(x),
    "position_y": float(y),
}))
' "${id}" "${zone_id}" "${name}" "${x}" "${y}")
    echo "[seed-fleet-demo] Creating station '${name}' (${id}) ..."
    curl -sf -X POST "${BASE_URL}/fleet/stations" \
      -H 'Content-Type: application/json' \
      -H "Authorization: Bearer ${TOKEN}" \
      -d "${payload}" >/dev/null
  }

  create_indoor_station "station-lager-regal-a"    "Regal A"          95  90
  create_indoor_station "station-lager-kommission" "Kommissionierung" 285 85
  create_indoor_station "station-lager-verladung"  "Verladung"        285 225
}

echo "[seed-fleet-demo] Logging in as ${ADMIN_USERNAME} against ${BASE_URL} ..."
LOGIN_RESPONSE=$(curl -sf -X POST "${BASE_URL}/auth/operator/login" \
  -H 'Content-Type: application/json' \
  -d "$(python3 -c 'import json,sys; print(json.dumps({"username": sys.argv[1], "password": sys.argv[2]}))' "${ADMIN_USERNAME}" "${ADMIN_PASSWORD}")")

TOKEN=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])' <<<"${LOGIN_RESPONSE}")

if [[ -z "${TOKEN}" ]]; then
  echo "[seed-fleet-demo] ERROR: no token in login response: ${LOGIN_RESPONSE}" >&2
  exit 1
fi

seed_outdoor_zone
seed_indoor_zone

echo "[seed-fleet-demo] Seed complete."
