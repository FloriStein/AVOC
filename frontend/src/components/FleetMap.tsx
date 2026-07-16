import { useEffect, useMemo } from 'react'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import { MapContainer, Marker, useMap } from 'react-leaflet'
import type { Zone, Station, FleetVehicle } from '@/lib/api-client'
import { parseGeoBounds, parseSvgGeometry, autonomyMarkerColor, vehiclesWithPosition } from '@/lib/fleet-map'

interface Props {
  zones: Zone[]
  stations: Station[]
  vehicles: FleetVehicle[]
  selectedVehicleId: string | null
  onSelectVehicle: (vehicleId: string) => void
  className?: string
}

// Sprint 23 (MAP-05) — outdoor-only zone/station/vehicle map. No <TileLayer>: ADR-029 explicitly
// rules out external map tiles/SaaS, the zone's own svg_geometry is the only visual background.
export function FleetMap({ zones, stations, vehicles, selectedVehicleId, onSelectVehicle, className }: Props) {
  const zonesWithBounds = useMemo(
    () =>
      zones
        .map((zone) => ({ zone, bounds: parseGeoBounds(zone.geo_bounds) }))
        .filter((z): z is { zone: Zone; bounds: NonNullable<ReturnType<typeof parseGeoBounds>> } => z.bounds !== null),
    [zones],
  )

  const overallBounds = useMemo(() => {
    if (zonesWithBounds.length === 0) return null
    const corners: L.LatLngExpression[] = zonesWithBounds.flatMap(({ bounds }) => [
      [bounds.sw.lat, bounds.sw.lon],
      [bounds.ne.lat, bounds.ne.lon],
    ])
    return L.latLngBounds(corners)
  }, [zonesWithBounds])

  if (!overallBounds) {
    return (
      <section className={`bg-gray-800 rounded-lg border border-gray-700 p-4 flex items-center justify-center ${className ?? ''}`}>
        <p className="text-xs text-gray-500 text-center py-4">Keine Zonen konfiguriert</p>
      </section>
    )
  }

  return (
    <section className={`bg-gray-800 rounded-lg border border-gray-700 overflow-hidden ${className ?? ''}`}>
      <MapContainer
        bounds={overallBounds}
        zoomControl={false}
        attributionControl={false}
        className="w-full h-full bg-gray-900"
      >
        {zonesWithBounds.map(({ zone, bounds }) => (
          <ZoneSvgOverlay key={zone.id} zone={zone} bounds={bounds} />
        ))}

        {stations
          .filter((s) => s.position_lat !== undefined && s.position_lon !== undefined)
          .map((s) => (
            <Marker
              key={s.id}
              position={[s.position_lat!, s.position_lon!]}
              icon={stationIcon(s.name)}
            />
          ))}

        {vehiclesWithPosition(vehicles).map((v) => (
          <Marker
            key={v.id}
            position={[v.position_lat!, v.position_lon!]}
            icon={vehicleIcon(v, v.id === selectedVehicleId)}
            eventHandlers={{ click: () => onSelectVehicle(v.id) }}
          />
        ))}
      </MapContainer>
    </section>
  )
}

// Renders a zone's raw svg_geometry as a geo-referenced Leaflet overlay via the core L.svgOverlay
// API (available since Leaflet 1.0, no plugin needed) instead of react-leaflet's JSX-composed
// <SVGOverlay>, since the geometry arrives as an opaque markup string from the backend, not
// author-time JSX. Imperative useMap()+useEffect with an explicit cleanup — required under
// StrictMode, which double-invokes effects in dev and would otherwise add the layer twice.
function ZoneSvgOverlay({ zone, bounds }: { zone: Zone; bounds: NonNullable<ReturnType<typeof parseGeoBounds>> }) {
  const map = useMap()

  useEffect(() => {
    const svgEl = parseSvgGeometry(zone.svg_geometry)
    if (!svgEl) {
      console.warn(`FleetMap: zone "${zone.id}" has empty or malformed svg_geometry, skipping overlay`)
      return
    }
    const layer = L.svgOverlay(svgEl, [
      [bounds.sw.lat, bounds.sw.lon],
      [bounds.ne.lat, bounds.ne.lon],
    ]).addTo(map)
    return () => {
      map.removeLayer(layer)
    }
  }, [map, zone.id, zone.svg_geometry, bounds])

  return null
}

function stationIcon(name: string): L.DivIcon {
  return L.divIcon({
    className: '',
    html: `<span class="block w-2.5 h-2.5 bg-gray-300 border border-gray-500" title="${escapeHtml(name)}"></span>`,
    iconSize: [10, 10],
    iconAnchor: [5, 5],
  })
}

function vehicleIcon(vehicle: FleetVehicle, selected: boolean): L.DivIcon {
  const color = autonomyMarkerColor(vehicle.autonomy_mode)
  const ring = selected ? 'ring-2 ring-white' : ''
  return L.divIcon({
    className: '',
    html: `<span class="block w-3.5 h-3.5 rounded-full ${color} ${ring}" title="${escapeHtml(vehicle.display_name)}"></span>`,
    iconSize: [14, 14],
    iconAnchor: [7, 7],
  })
}

function escapeHtml(value: string): string {
  return value.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}
