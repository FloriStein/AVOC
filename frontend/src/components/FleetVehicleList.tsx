import type { FleetVehicle, ActiveSession } from '@/lib/api-client'
import { AUTONOMY_DOT } from '@/lib/fleet-map'

interface Props {
  vehicles: FleetVehicle[]
  activeSessions: ActiveSession[]
  selectedVehicleId: string | null
  onSelect: (vehicleId: string) => void
}

function activeOperatorFor(vehicleId: string, activeSessions: ActiveSession[]): string | null {
  const match = activeSessions.find((s) => s.vehicle_id === vehicleId && s.role === 'ACTIVE_OPERATOR')
  return match?.operator_id ?? null
}

export function FleetVehicleList({ vehicles, activeSessions, selectedVehicleId, onSelect }: Props) {
  return (
    <section className="bg-gray-800 rounded-lg border border-gray-700 p-4 flex flex-col gap-2 min-h-0">
      <h2 className="text-sm font-semibold text-gray-400 uppercase tracking-wide">Fahrzeuge</h2>

      {vehicles.length === 0 ? (
        <p className="text-xs text-gray-500 text-center py-4">Keine Fahrzeuge</p>
      ) : (
        <div className="flex flex-col gap-1 overflow-y-auto">
          {vehicles.map((v) => {
            const operatorId = activeOperatorFor(v.id, activeSessions)
            const dotColor = v.autonomy_mode ? AUTONOMY_DOT[v.autonomy_mode] ?? 'bg-gray-500' : null
            return (
              <button
                key={v.id}
                onClick={() => onSelect(v.id)}
                className={`flex items-center justify-between w-full rounded px-2 py-1.5 text-xs text-left transition-colors ${
                  selectedVehicleId === v.id ? 'bg-indigo-800' : 'bg-gray-700 hover:bg-gray-600'
                }`}
              >
                <span className="flex items-center gap-2">
                  {dotColor ? (
                    <span className={`w-2 h-2 rounded-full ${dotColor}`} title={v.autonomy_mode} />
                  ) : (
                    <span
                      className="w-2 h-2 rounded-full border border-dashed border-gray-500"
                      title="Keine Daten"
                    />
                  )}
                  <span className="font-mono text-gray-100">{v.display_name}</span>
                  {v.vehicle_type && <span className="text-gray-500">({v.vehicle_type})</span>}
                </span>
                <span className="flex items-center gap-2">
                  {v.battery_pct !== undefined && (
                    <span className={`font-mono ${v.battery_pct < 20 ? 'text-red-400' : 'text-gray-300'}`}>
                      {v.battery_pct.toFixed(0)}%
                    </span>
                  )}
                  {operatorId && (
                    <span className="text-green-300 bg-green-900 px-1.5 py-0.5 rounded text-xs">
                      Aktiv: {operatorId}
                    </span>
                  )}
                </span>
              </button>
            )
          })}
        </div>
      )}
    </section>
  )
}
