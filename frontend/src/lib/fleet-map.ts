// Pure helpers for the outdoor Fleet Map (Sprint 23, MAP-04) — kept free of React/Leaflet so
// parsing/validation logic is unit-testable in isolation, mirroring fleet-merge.ts's precedent.

import type { FleetVehicle } from '@/lib/api-client'

export interface GeoBounds {
  sw: { lat: number; lon: number }
  ne: { lat: number; lon: number }
}

// Autonomy-mode → marker/dot color, shared by FleetVehicleList.tsx (list dots) and FleetMap.tsx
// (vehicle markers) — single source of truth instead of duplicated color literals.
export const AUTONOMY_DOT: Record<string, string> = {
  autonomous: 'bg-green-500',
  teleoperated: 'bg-blue-500',
  manual: 'bg-yellow-500',
}

export const UNKNOWN_AUTONOMY_DOT = 'bg-gray-500'

// zones.geo_bounds is a raw JSON string on the wire (Zone.GeoBounds is Go *string — see
// docs/adr/029-fleet-vehicle-data-model.md's 2026-07-16 update). Never throws: missing, malformed
// JSON, wrong shape, or non-finite coordinates all resolve to null so a single bad zone can't take
// down the whole map.
export function parseGeoBounds(raw: string | undefined): GeoBounds | null {
  if (!raw) return null
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return null
  }
  if (typeof parsed !== 'object' || parsed === null) return null
  const { sw, ne } = parsed as Record<string, unknown>
  if (!isLatLon(sw) || !isLatLon(ne)) return null
  return { sw, ne }
}

function isLatLon(value: unknown): value is { lat: number; lon: number } {
  if (typeof value !== 'object' || value === null) return false
  const { lat, lon } = value as Record<string, unknown>
  return typeof lat === 'number' && Number.isFinite(lat) && typeof lon === 'number' && Number.isFinite(lon)
}

// zones.svg_geometry is raw <svg>...</svg> markup. Uses DOMParser (available under jsdom too, so
// this stays unit-testable without mounting Leaflet) — returns null for an empty string or
// malformed XML instead of throwing, mirroring parseGeoBounds's error convention.
export function parseSvgGeometry(raw: string): SVGSVGElement | null {
  if (!raw.trim()) return null
  const doc = new DOMParser().parseFromString(raw, 'image/svg+xml')
  if (doc.querySelector('parsererror')) return null
  const svg = doc.documentElement
  if (svg.tagName.toLowerCase() !== 'svg') return null
  return svg as unknown as SVGSVGElement
}

export function autonomyMarkerColor(mode: string | undefined): string {
  if (!mode) return UNKNOWN_AUTONOMY_DOT
  return AUTONOMY_DOT[mode] ?? UNKNOWN_AUTONOMY_DOT
}

// Filters out vehicles with no known position — covers the vehicle-001 case (never reported
// status, both position_lat/lon undefined) so FleetMap.tsx never has to special-case it itself.
// Multiple vehicles at identical coordinates are all kept (visual overlap is an accepted,
// undeduplicated limitation for this sprint).
export function vehiclesWithPosition(vehicles: FleetVehicle[]): FleetVehicle[] {
  return vehicles.filter((v) => v.position_lat !== undefined && v.position_lon !== undefined)
}
