import type { FleetVehicle } from '@/lib/api-client'

interface Props {
  vehicle: FleetVehicle | null
  hasActiveOperator: boolean
  activeOperatorId: string | null
  isObserverRole: boolean
  onTeleoperate: (vehicleId: string) => void
  onObserve: (vehicleId: string) => void
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between text-sm items-center">
      <span className="text-gray-400">{label}</span>
      <span className="font-mono text-gray-200 text-sm">{value}</span>
    </div>
  )
}

// ADR-028 (2026-07-15 update): a vehicle already under another operator's active control must
// NOT show an available Teleoperate button — proactive takeover is only for vehicles with no
// active operator. Observing an active session remains possible via the existing session-join
// flow (session.startSession, same as ConnectionPanel's onJoinSession today).
export function FleetVehicleDetail({
  vehicle,
  hasActiveOperator,
  activeOperatorId,
  isObserverRole,
  onTeleoperate,
  onObserve,
}: Props) {
  if (!vehicle) {
    return (
      <section className="bg-gray-800 rounded-lg border border-gray-700 p-4 flex items-center justify-center">
        <p className="text-xs text-gray-500">Fahrzeug auswählen</p>
      </section>
    )
  }

  return (
    <section className="bg-gray-800 rounded-lg border border-gray-700 p-4 flex flex-col gap-2">
      <h2 className="text-sm font-semibold text-gray-400 uppercase tracking-wide">
        {vehicle.display_name}
      </h2>

      <DetailRow label="Typ" value={vehicle.vehicle_type ?? '—'} />
      <DetailRow
        label="Batterie"
        value={vehicle.battery_pct !== undefined ? `${vehicle.battery_pct.toFixed(0)}%` : '—'}
      />
      <DetailRow
        label="Geschwindigkeit"
        value={vehicle.speed !== undefined ? `${vehicle.speed.toFixed(1)} km/h` : '—'}
      />
      <DetailRow label="Autonomie-Modus" value={vehicle.autonomy_mode ?? '—'} />
      <DetailRow label="Zone" value={vehicle.position_zone_id ?? '—'} />

      <hr className="border-gray-700 my-1" />

      {hasActiveOperator ? (
        <div className="flex flex-col gap-2">
          <span className="text-xs text-yellow-300 bg-yellow-900/50 px-2 py-1 rounded text-center">
            Aktiver Operator: {activeOperatorId}
          </span>
          <button
            onClick={() => onObserve(vehicle.id)}
            className="w-full py-1.5 px-3 bg-gray-700 hover:bg-gray-600 border border-gray-600 rounded text-xs text-gray-200 font-semibold transition-colors"
          >
            Beobachten
          </button>
        </div>
      ) : (
        !isObserverRole && (
          <button
            onClick={() => onTeleoperate(vehicle.id)}
            className="w-full py-1.5 px-3 bg-indigo-600 hover:bg-indigo-500 rounded text-xs text-white font-semibold transition-colors"
          >
            Teleoperate
          </button>
        )
      )}
    </section>
  )
}
