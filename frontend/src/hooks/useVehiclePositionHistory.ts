import { useEffect, useState } from 'react'
import { listVehiclePositionHistory, type VehiclePositionHistoryPoint } from '@/lib/api-client'

export interface VehiclePositionHistoryState {
  points: VehiclePositionHistoryPoint[]
  loading: boolean
  error: string | null
}

// One-shot REST fetch per vehicle selection, not live-polled — mirrors useFleetZones.ts's
// rationale (ADR-033: the historie-linie is a map overlay, not a live-updating value; re-selecting
// the vehicle is the accepted way to pick up newly recorded points). Re-fetches whenever
// vehicleId changes; returns empty/idle state for null (no vehicle selected).
export function useVehiclePositionHistory(token: string | null, vehicleId: string | null): VehiclePositionHistoryState {
  const [points, setPoints] = useState<VehiclePositionHistoryPoint[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!token || !vehicleId) {
      setPoints([])
      setError(null)
      setLoading(false)
      return
    }
    let active = true
    setLoading(true)

    const fetchHistory = async () => {
      try {
        const p = await listVehiclePositionHistory(token, vehicleId)
        if (!active) return
        setPoints(p)
        setError(null)
      } catch {
        if (!active) return
        setError('Routenverlauf konnte nicht geladen werden')
      } finally {
        if (active) setLoading(false)
      }
    }

    fetchHistory()

    return () => {
      active = false
    }
  }, [token, vehicleId])

  return { points, loading, error }
}
