import { useEffect, useState } from 'react'
import type { FleetVehicle, Task, Station, CreateFleetTaskInput } from '@/lib/api-client'
import { listFleetStations } from '@/lib/api-client'

interface Props {
  tasks: Task[]
  vehicles: FleetVehicle[]
  token: string | null
  onCreateTask: (input: CreateFleetTaskInput) => Promise<void>
  onUpdateStatus: (id: string, status: string) => Promise<void>
}

const STATUS_LABEL: Record<Task['status'], string> = {
  pending: 'Ausstehend',
  in_progress: 'In Bearbeitung',
  completed: 'Abgeschlossen',
  cancelled: 'Storniert',
}

const STATUS_STYLE: Record<Task['status'], string> = {
  pending: 'text-gray-300 bg-gray-700',
  in_progress: 'text-blue-300 bg-blue-950/50',
  completed: 'text-green-300 bg-green-950/50',
  cancelled: 'text-gray-500 bg-gray-800',
}

// Client-side mirror of ADR-030's backend transition matrix (internal/fleetservice/store.go's
// taskTransitionSources) — the backend remains the authoritative validator (a stale/duplicated
// mirror here only affects which buttons render, not correctness; a rejected transition still
// surfaces as an inline error below). Documented as known duplication debt, see DECISIONS.MD.
const NEXT_TRANSITIONS: Record<Task['status'], { status: string; label: string }[]> = {
  pending: [
    { status: 'in_progress', label: 'Starten' },
    { status: 'cancelled', label: 'Stornieren' },
  ],
  in_progress: [
    { status: 'completed', label: 'Abschließen' },
    { status: 'cancelled', label: 'Stornieren' },
  ],
  completed: [],
  cancelled: [],
}

const emptyForm = { vehicle_id: '', from_station_id: '', to_station_id: '', priority: '0' }

// Sprint 24 (ADR-030) — Task-Management-UI: Liste aller Tasks (= Historie, alle Status),
// Anlage-Formular und Status-Übergangs-Buttons. Styled per FleetAlertsPanel.tsx's conventions
// (dark theme, per-row pending-state so one row's in-flight action doesn't disable another's).
export function FleetTaskPanel({ tasks, vehicles, token, onCreateTask, onUpdateStatus }: Props) {
  const [stations, setStations] = useState<Station[]>([])
  const [form, setForm] = useState(emptyForm)
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [pendingTaskId, setPendingTaskId] = useState<string | null>(null)
  const [statusErrors, setStatusErrors] = useState<Record<string, string>>({})

  // Stations are reference data (rarely change, no WS live-update channel of their own) — fetched
  // once here rather than folded into useFleetOverview's live vehicle/alert/task loop.
  useEffect(() => {
    if (!token) return
    let active = true
    listFleetStations(token)
      .then((s) => { if (active) setStations(s) })
      .catch(() => { if (active) setStations([]) })
    return () => { active = false }
  }, [token])

  const vehicleName = (id: string) => vehicles.find((v) => v.id === id)?.display_name ?? id
  const stationName = (id: string) => stations.find((s) => s.id === id)?.name ?? id

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!form.vehicle_id || !form.from_station_id || !form.to_station_id) {
      setFormError('Fahrzeug, Von-Station und Nach-Station sind erforderlich')
      return
    }
    setSubmitting(true)
    setFormError(null)
    try {
      await onCreateTask({
        vehicle_id: form.vehicle_id,
        from_station_id: form.from_station_id,
        to_station_id: form.to_station_id,
        priority: Number(form.priority) || 0,
      })
      setForm(emptyForm)
    } catch {
      setFormError('Task konnte nicht angelegt werden')
    } finally {
      setSubmitting(false)
    }
  }

  const handleStatusChange = async (id: string, status: string) => {
    setPendingTaskId(id)
    setStatusErrors((prev) => ({ ...prev, [id]: '' }))
    try {
      await onUpdateStatus(id, status)
    } catch {
      setStatusErrors((prev) => ({ ...prev, [id]: 'Statuswechsel fehlgeschlagen' }))
    } finally {
      setPendingTaskId(null)
    }
  }

  return (
    <section className="bg-gray-800 rounded-lg border border-gray-700 p-4 flex flex-col gap-3 min-h-0 col-span-full">
      <h2 className="text-sm font-semibold text-gray-400 uppercase tracking-wide">Tasks</h2>

      <form onSubmit={handleSubmit} className="flex flex-wrap items-end gap-2 text-xs">
        <label className="flex flex-col gap-1">
          <span className="text-gray-500">Fahrzeug</span>
          <select
            value={form.vehicle_id}
            onChange={(e) => setForm((f) => ({ ...f, vehicle_id: e.target.value }))}
            className="bg-gray-900 border border-gray-600 rounded px-2 py-1 text-gray-200"
          >
            <option value="">—</option>
            {vehicles.map((v) => (
              <option key={v.id} value={v.id}>{v.display_name}</option>
            ))}
          </select>
        </label>

        <label className="flex flex-col gap-1">
          <span className="text-gray-500">Von</span>
          <select
            value={form.from_station_id}
            onChange={(e) => setForm((f) => ({ ...f, from_station_id: e.target.value }))}
            className="bg-gray-900 border border-gray-600 rounded px-2 py-1 text-gray-200"
          >
            <option value="">—</option>
            {stations.map((s) => (
              <option key={s.id} value={s.id}>{s.name}</option>
            ))}
          </select>
        </label>

        <label className="flex flex-col gap-1">
          <span className="text-gray-500">Nach</span>
          <select
            value={form.to_station_id}
            onChange={(e) => setForm((f) => ({ ...f, to_station_id: e.target.value }))}
            className="bg-gray-900 border border-gray-600 rounded px-2 py-1 text-gray-200"
          >
            <option value="">—</option>
            {stations.map((s) => (
              <option key={s.id} value={s.id}>{s.name}</option>
            ))}
          </select>
        </label>

        <label className="flex flex-col gap-1">
          <span className="text-gray-500">Priorität</span>
          <input
            type="number"
            value={form.priority}
            onChange={(e) => setForm((f) => ({ ...f, priority: e.target.value }))}
            className="bg-gray-900 border border-gray-600 rounded px-2 py-1 text-gray-200 w-16"
          />
        </label>

        <button
          type="submit"
          disabled={submitting}
          className="px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 disabled:cursor-not-allowed rounded text-white font-semibold transition-colors"
        >
          {submitting ? '…' : 'Task anlegen'}
        </button>

        {formError && <span className="text-red-400 w-full">{formError}</span>}
      </form>

      <hr className="border-gray-700" />

      {tasks.length === 0 ? (
        <p className="text-xs text-gray-500 text-center py-4">Keine Tasks</p>
      ) : (
        <div className="flex flex-col gap-1.5 overflow-y-auto max-h-64">
          {tasks.map((t) => (
            <div
              key={t.id}
              className="flex flex-col gap-1 rounded border border-gray-700 bg-gray-900/50 px-2 py-1.5 text-xs"
            >
              <div className="flex items-center justify-between gap-2">
                <span className="font-mono text-gray-200">
                  {vehicleName(t.vehicle_id)}: {stationName(t.from_station_id)} → {stationName(t.to_station_id)}
                </span>
                <span className={`px-1.5 py-0.5 rounded ${STATUS_STYLE[t.status]}`}>
                  {STATUS_LABEL[t.status]}
                </span>
              </div>
              <div className="flex items-center justify-between text-gray-500">
                <span>Priorität {t.priority} · {new Date(t.created_at).toLocaleTimeString()}</span>
                {t.status_changed_by && <span>zuletzt: {t.status_changed_by}</span>}
              </div>
              <div className="flex items-center gap-2">
                {NEXT_TRANSITIONS[t.status].map(({ status, label }) => (
                  <button
                    key={status}
                    onClick={() => handleStatusChange(t.id, status)}
                    disabled={pendingTaskId === t.id}
                    className="px-2 py-0.5 bg-gray-700 hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed rounded text-gray-200 font-semibold transition-colors"
                  >
                    {pendingTaskId === t.id ? '…' : label}
                  </button>
                ))}
                {statusErrors[t.id] && <span className="text-red-400">{statusErrors[t.id]}</span>}
              </div>
            </div>
          ))}
        </div>
      )}
    </section>
  )
}
