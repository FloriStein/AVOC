import { useState } from 'react'
import type { SessionState } from '@/hooks/useSession'
import { useFleetOverview } from '@/hooks/useFleetOverview'
import { useActiveSessions } from '@/hooks/useActiveSessions'
import { useFleetZones } from '@/hooks/useFleetZones'
import { useFleetAlertSound } from '@/hooks/useFleetAlertSound'
import { parseTokenRole } from '@/lib/api-client'
import { FleetVehicleList } from '@/components/FleetVehicleList'
import { FleetVehicleDetail } from '@/components/FleetVehicleDetail'
import { FleetAlertsPanel } from '@/components/FleetAlertsPanel'
import { FleetMap } from '@/components/FleetMap'
import { FleetTaskPanel } from '@/components/FleetTaskPanel'

interface Props {
  session: SessionState
}

// Sprint 22 landing view — shown after login while no teleop session is active (App.tsx gates on
// !session.sessionId). Replaces the old "straight into the cockpit with an embedded
// VehicleSelector" flow. Combines fleet-service's vehicle/alert data with control-server's
// session list client-side (ADR-029: "Frontend führt Services clientseitig zusammen") to
// determine per-vehicle operator availability for the Teleoperate/Beobachten gating (ADR-028).
export function FleetOverview({ session }: Props) {
  const { vehicles, alerts, tasks, loading, error, acknowledgeAlert, createTask, updateTaskStatus } = useFleetOverview(
    session.token,
    session.operatorId,
  )
  const { activeSessions } = useActiveSessions(session.token)
  const { zones, stations } = useFleetZones(session.token)
  const { muted, toggleMuted } = useFleetAlertSound(alerts, loading)
  const [selectedVehicleId, setSelectedVehicleId] = useState<string | null>(null)

  const isObserverRole = parseTokenRole(session.token!) === 'OBSERVER'

  const selectedVehicle = vehicles.find((v) => v.id === selectedVehicleId) ?? null
  const activeOperatorSession = selectedVehicleId
    ? activeSessions.find((s) => s.vehicle_id === selectedVehicleId && s.role === 'ACTIVE_OPERATOR')
    : undefined
  const hasActiveOperator = !!activeOperatorSession

  return (
    <div className="min-h-screen bg-gray-900 text-white flex flex-col">
      <header className="bg-gray-800 border-b border-gray-700 px-6 py-3 flex items-center justify-between">
        <h1 className="text-lg font-bold tracking-wide">AVOC — Fleet Overview</h1>
        <button
          onClick={session.disconnect}
          className="px-3 py-1 rounded text-xs font-semibold bg-gray-700 hover:bg-gray-600 text-gray-300"
        >
          Abmelden
        </button>
      </header>

      {loading && (
        <div className="flex-1 flex items-center justify-center text-gray-500 text-sm">
          Lädt Flottendaten…
        </div>
      )}

      {!loading && error && (
        <div className="flex-1 flex items-center justify-center text-red-400 text-sm">{error}</div>
      )}

      {!loading && !error && (
        <main className="flex-1 flex flex-col gap-4 p-4 min-h-0">
          <FleetMap
            zones={zones}
            stations={stations}
            vehicles={vehicles}
            selectedVehicleId={selectedVehicleId}
            onSelectVehicle={setSelectedVehicleId}
            className="h-[45vh] min-h-80 shrink-0"
          />
          <div className="grid grid-cols-3 gap-4 flex-1 min-h-0">
            <FleetVehicleList
              vehicles={vehicles}
              activeSessions={activeSessions}
              selectedVehicleId={selectedVehicleId}
              onSelect={setSelectedVehicleId}
            />
            <FleetVehicleDetail
              vehicle={selectedVehicle}
              hasActiveOperator={hasActiveOperator}
              activeOperatorId={activeOperatorSession?.operator_id ?? null}
              isObserverRole={isObserverRole}
              onTeleoperate={session.startSession}
              onObserve={session.startSession}
            />
            <FleetAlertsPanel
              alerts={alerts}
              onAcknowledge={acknowledgeAlert}
              muted={muted}
              onToggleMuted={toggleMuted}
            />
          </div>
          <FleetTaskPanel
            tasks={tasks}
            vehicles={vehicles}
            token={session.token}
            onCreateTask={createTask}
            onUpdateStatus={updateTaskStatus}
          />
        </main>
      )}
    </div>
  )
}
