import { useCallback, useEffect, useRef, useState } from 'react'
import {
  listFleetVehicles,
  listFleetAlerts,
  acknowledgeFleetAlert,
  type FleetVehicle,
  type FleetAlert,
} from '@/lib/api-client'
import { FleetWSClient } from '@/lib/fleet-ws-client'
import { mergeVehicleStatus, upsertAlert, applyAlertAcknowledged } from '@/lib/fleet-merge'

export interface FleetOverviewState {
  vehicles: FleetVehicle[]
  alerts: FleetAlert[]
  loading: boolean
  error: string | null
  acknowledgeAlert: (id: string) => Promise<void>
}

// Drives the Fleet Overview dashboard: initial REST snapshot, then live deltas via the Fleet
// WebSocket (FLEET-06). Modeled on this codebase's polling-hook cleanup pattern (an `active`
// closure flag guarding setState-after-unmount) but WS-driven instead of setInterval.
export function useFleetOverview(token: string | null, operatorId: string | null): FleetOverviewState {
  const [vehicles, setVehicles] = useState<FleetVehicle[]>([])
  const [alerts, setAlerts] = useState<FleetAlert[]>([])
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
        const [v, a] = await Promise.all([listFleetVehicles(token), listFleetAlerts(token)])
        if (!active) return
        setVehicles(v)
        setAlerts(a)
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

  return { vehicles, alerts, loading, error, acknowledgeAlert }
}
