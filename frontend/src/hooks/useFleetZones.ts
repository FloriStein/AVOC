import { useEffect, useState } from 'react'
import { listFleetZones, listFleetStations, type Zone, type Station } from '@/lib/api-client'

export interface FleetZonesState {
  zones: Zone[]
  stations: Station[]
  loading: boolean
  error: string | null
}

// One-shot REST fetch, deliberately not WS-driven like useFleetOverview.ts — FLEET-06's broadcast
// hub only emits vehicle_status/alert_created/alert_acknowledged/task_created, there is no
// zone/station change event. Zones/stations are static admin config for this sprint (no AP3
// zone-editing UI exists yet either); a page reload is the accepted way to pick up backend
// changes, not an oversight.
export function useFleetZones(token: string | null): FleetZonesState {
  const [zones, setZones] = useState<Zone[]>([])
  const [stations, setStations] = useState<Station[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!token) return
    let active = true

    const fetchZones = async () => {
      try {
        const [z, s] = await Promise.all([listFleetZones(token), listFleetStations(token)])
        if (!active) return
        setZones(z)
        setStations(s)
        setError(null)
      } catch {
        if (!active) return
        setError('Zonendaten konnten nicht geladen werden')
      } finally {
        if (active) setLoading(false)
      }
    }

    fetchZones()

    return () => {
      active = false
    }
  }, [token])

  return { zones, stations, loading, error }
}
