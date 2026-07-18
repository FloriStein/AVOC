import { useMemo } from 'react'
import type { Zone, Station, FleetVehicle } from '@/lib/api-client'
import { parseSvgGeometry } from '@/lib/fleet-map'
import { indoorZones, stationsInZone, vehiclesInZone, autonomyFillColor } from '@/lib/fleet-indoor-map'

interface Props {
  zones: Zone[]
  stations: Station[]
  vehicles: FleetVehicle[]
  selectedVehicleId: string | null
  onSelectVehicle: (vehicleId: string) => void
  className?: string
}

// Sprint 33 (ADR-034) — Indoor-Kartenrendering. Unlike FleetMap.tsx (Leaflet, geo-referenced),
// indoor zones have no geo_bounds: a zone's own svg_geometry viewBox IS the coordinate system, so
// this renders plain stacked <svg> elements (background geometry + a marker overlay) instead of a
// map library. Renders nothing if there are no indoor zones yet (most existing deployments before
// this sprint's seed script ran) — same "don't show an empty state for a feature nobody has
// configured" instinct as FleetMap's own empty-zones guard, just via early return instead of a
// placeholder message since this section is entirely optional additional content.
export function FleetIndoorMap({ zones, stations, vehicles, selectedVehicleId, onSelectVehicle, className }: Props) {
  const zonesToRender = useMemo(() => indoorZones(zones), [zones])

  if (zonesToRender.length === 0) return null

  return (
    <section className={`bg-gray-800 rounded-lg border border-gray-700 p-3 flex flex-col gap-3 ${className ?? ''}`}>
      <h2 className="text-xs font-semibold text-gray-400 uppercase tracking-wide">Indoor-Zonen</h2>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        {zonesToRender.map((zone) => (
          <IndoorZoneCard
            key={zone.id}
            zone={zone}
            stations={stationsInZone(stations, zone.id)}
            vehicles={vehiclesInZone(vehicles, zone.id)}
            selectedVehicleId={selectedVehicleId}
            onSelectVehicle={onSelectVehicle}
          />
        ))}
      </div>
    </section>
  )
}

function IndoorZoneCard({
  zone,
  stations,
  vehicles,
  selectedVehicleId,
  onSelectVehicle,
}: {
  zone: Zone
  stations: Station[]
  vehicles: FleetVehicle[]
  selectedVehicleId: string | null
  onSelectVehicle: (vehicleId: string) => void
}) {
  const svgEl = useMemo(() => parseSvgGeometry(zone.svg_geometry), [zone.svg_geometry])
  const viewBox = svgEl?.getAttribute('viewBox') ?? '0 0 400 300'
  // innerHTML, not outerHTML: the background layer's own <svg viewBox> wrapper below supplies the
  // coordinate system — only the geometry inside zone.svg_geometry's <svg> is needed here.
  const backgroundMarkup = svgEl?.innerHTML ?? ''

  return (
    <div className="bg-gray-900 rounded border border-gray-700 overflow-hidden">
      <p className="text-xs text-gray-400 px-2 py-1 border-b border-gray-700">{zone.name}</p>
      {!svgEl ? (
        <p className="text-xs text-gray-500 text-center py-8">Keine Kartengeometrie hinterlegt</p>
      ) : (
        <div className="relative w-full aspect-[4/3] bg-gray-950">
          <svg
            viewBox={viewBox}
            className="absolute inset-0 w-full h-full"
            // Background geometry arrives as an opaque markup string from the backend, not
            // author-time JSX — same rationale as FleetMap.tsx's ZoneSvgOverlay for using
            // L.svgOverlay instead of JSX-composed shapes there.
            dangerouslySetInnerHTML={{ __html: backgroundMarkup }}
          />
          <svg viewBox={viewBox} className="absolute inset-0 w-full h-full">
            {stations.map((s) => (
              <circle key={s.id} cx={s.position_x} cy={s.position_y} r={5} className="fill-gray-300 stroke-gray-600 stroke-1">
                <title>{s.name}</title>
              </circle>
            ))}
            {vehicles.map((v) => (
              <circle
                key={v.id}
                cx={v.position_x}
                cy={v.position_y}
                r={7}
                className={`cursor-pointer ${autonomyFillColor(v.autonomy_mode)} ${
                  v.id === selectedVehicleId ? 'stroke-white stroke-2' : 'stroke-none'
                }`}
                onClick={() => onSelectVehicle(v.id)}
              >
                <title>{v.display_name}</title>
              </circle>
            ))}
          </svg>
        </div>
      )}
    </div>
  )
}
