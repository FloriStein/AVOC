import { useCallback, useEffect, useRef, useState } from 'react'
import {
  listFleetVehicles,
  listFleetAlerts,
  listFleetTasks,
  acknowledgeFleetAlert,
  createFleetTask,
  updateFleetTaskStatus,
  type FleetVehicle,
  type FleetAlert,
  type Task,
  type CreateFleetTaskInput,
} from '@/lib/api-client'
import { FleetWSClient } from '@/lib/fleet-ws-client'
import {
  mergeVehicleStatus,
  upsertAlert,
  applyAlertAcknowledged,
  upsertTask,
  applyTaskStatusChanged,
} from '@/lib/fleet-merge'

export interface FleetOverviewState {
  vehicles: FleetVehicle[]
  alerts: FleetAlert[]
  tasks: Task[]
  loading: boolean
  error: string | null
  acknowledgeAlert: (id: string) => Promise<void>
  createTask: (input: CreateFleetTaskInput) => Promise<void>
  updateTaskStatus: (id: string, status: string) => Promise<void>
}

// Drives the Fleet Overview dashboard: initial REST snapshot, then live deltas via the Fleet
// WebSocket (FLEET-06). Modeled on this codebase's polling-hook cleanup pattern (an `active`
// closure flag guarding setState-after-unmount) but WS-driven instead of setInterval.
export function useFleetOverview(token: string | null, operatorId: string | null): FleetOverviewState {
  const [vehicles, setVehicles] = useState<FleetVehicle[]>([])
  const [alerts, setAlerts] = useState<FleetAlert[]>([])
  const [tasks, setTasks] = useState<Task[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const wsClientRef = useRef<FleetWSClient | null>(null)
  const hasConnectedOnceRef = useRef(false)

  useEffect(() => {
    if (!token) return
    let active = true
    hasConnectedOnceRef.current = false

    // Unlike useVehicles.ts's silent-stale-on-error convention, this hook surfaces an explicit
    // error — Fleet Overview is the dashboard's primary content, not a secondary sidebar.
    const fetchSnapshot = async () => {
      try {
        const [v, a, t] = await Promise.all([listFleetVehicles(token), listFleetAlerts(token), listFleetTasks(token)])
        if (!active) return
        setVehicles(v)
        setAlerts(a)
        setTasks(t)
        setError(null)
      } catch {
        if (!active) return
        setError('Flottendaten konnten nicht geladen werden')
      } finally {
        if (active) setLoading(false)
      }
    }

    fetchSnapshot().then(() => {
      if (!active) return

      const client = new FleetWSClient()
      wsClientRef.current = client

      client.onOpen = () => {
        // Hub.Broadcast (backend) drops events for a disconnected/slow client — no backlog, no
        // replay. Without a resync on every reconnect after the first, a brief WS blip would
        // leave the dashboard silently stale until a manual page reload.
        if (hasConnectedOnceRef.current) {
          fetchSnapshot()
        }
        hasConnectedOnceRef.current = true
      }

      client.onEvent = (event) => {
        switch (event.type) {
          case 'vehicle_status':
            setVehicles((prev) => mergeVehicleStatus(prev, event.data))
            break
          case 'alert_created':
            setAlerts((prev) => upsertAlert(prev, event.data))
            break
          case 'alert_acknowledged':
            setAlerts((prev) => applyAlertAcknowledged(prev, event.data))
            break
          case 'task_created':
            // Also applied directly in createTask() below from the REST response, before this
            // broadcast can arrive — upsertTask is idempotent by id, so this re-application (for
            // task_created events triggered by *other* Dashboard clients) is harmless here.
            setTasks((prev) => upsertTask(prev, event.data))
            break
          case 'task_status_changed':
            setTasks((prev) => applyTaskStatusChanged(prev, event.data))
            break
          case 'unknown':
            break
        }
      }

      client.connect(token)
    })

    return () => {
      active = false
      wsClientRef.current?.disconnect()
      wsClientRef.current = null
    }
  }, [token])

  // Optimistically applies the acknowledgement locally (client-synthesized timestamp) so the UI
  // updates immediately rather than waiting on the round-trip alert_acknowledged WS event —
  // idempotent, since that event later overwrites with the server's exact timestamp via the same
  // merge function. Re-throws on failure (unlike fire-and-forget calls elsewhere in this
  // codebase) so the calling component can show inline feedback for this deliberate user action.
  const acknowledgeAlert = useCallback(
    async (id: string) => {
      if (!token || !operatorId) return
      await acknowledgeFleetAlert(token, id, operatorId)
      setAlerts((prev) =>
        applyAlertAcknowledged(prev, {
          id,
          acknowledged_by: operatorId,
          acknowledged_at: new Date().toISOString(),
        }),
      )
    },
    [token, operatorId],
  )

  // Applies the REST response directly (already the real, authoritative row — no need to
  // synthesize one) rather than waiting on the task_created WS broadcast. Re-throws on failure so
  // FleetTaskPanel can show inline form feedback (ADR-030).
  const createTask = useCallback(
    async (input: CreateFleetTaskInput) => {
      if (!token) return
      const created = await createFleetTask(token, input)
      setTasks((prev) => upsertTask(prev, created))
    },
    [token],
  )

  // Re-throws on failure (in particular the ADR-030 409 for an invalid transition) so
  // FleetTaskPanel can show inline feedback per task row, matching acknowledgeAlert's convention.
  const updateTaskStatus = useCallback(
    async (id: string, status: string) => {
      if (!token || !operatorId) return
      const updated = await updateFleetTaskStatus(token, id, status, operatorId)
      setTasks((prev) => upsertTask(prev, updated))
    },
    [token, operatorId],
  )

  return { vehicles, alerts, tasks, loading, error, acknowledgeAlert, createTask, updateTaskStatus }
}
