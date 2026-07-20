// Pure helpers for the Indoor Fleet Map (Sprint 33, ADR-034) — kept free of React/SVG so
// filtering logic is unit-testable in isolation, mirroring fleet-map.ts's precedent. Indoor zones
// are not geo-referenced (no geo_bounds): a zone's own svg_geometry viewBox IS the coordinate
// system stations/vehicles are positioned in via position_x/y — see ADR-034.

import type { Zone, Station, FleetVehicle } from '@/lib/api-client'

// Autonomy-mode → SVG fill color. A separate map from fleet-map.ts's AUTONOMY_DOT (which holds
// Tailwind `bg-*` classes for HTML <span> markers) — SVG shape fill is a different CSS property
// than `background-color`, so `bg-*` utilities have no effect on <circle>/<path> fill.
const AUTONOMY_FILL: Record<string, string> = {
  autonomous: 'fill-green-500',
  teleoperated: 'fill-blue-500',
  manual: 'fill-yellow-500',
}

export const UNKNOWN_AUTONOMY_FILL = 'fill-gray-500'

export function autonomyFillColor(mode: string | undefined): string {
  if (!mode) return UNKNOWN_AUTONOMY_FILL
  return AUTONOMY_FILL[mode] ?? UNKNOWN_AUTONOMY_FILL
}

export function indoorZones(zones: Zone[]): Zone[] {
  return zones.filter((z) => z.environment === 'indoor')
}

export function stationsInZone(stations: Station[], zoneId: string): Station[] {
  return stations.filter((s) => s.zone_id === zoneId && s.position_x !== undefined && s.position_y !== undefined)
}

// Only a vehicle whose position_zone_id matches AND has both position_x/y set is renderable —
// position_zone_id alone (already possible today, though nothing currently writes it either, see
// ADR-034 Scope) is not enough without a concrete point to place the marker at.
export function vehiclesInZone(vehicles: FleetVehicle[], zoneId: string): FleetVehicle[] {
  return vehicles.filter(
    (v) => v.position_zone_id === zoneId && v.position_x !== undefined && v.position_y !== undefined,
  )
}
