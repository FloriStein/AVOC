import { useEffect, useRef, useState } from 'react'
import { getVehicleState, listSessions, type SystemStateResponse } from '@/lib/api-client'

const INITIAL: SystemStateResponse = {
  system: 'IDLE',
  control: 'CONTROL_INIT',
  media: 'MEDIA_INIT',
  operator: 'NO_OPERATOR',
}

const POLL_MS = 500
// 3 consecutive failures × 500ms = 1.5s before showing the unreachable banner.
const UNREACHABLE_THRESHOLD = 3

// Polls GET /api/vehicles/{id}/state every 500ms once a vehicle is selected (ADR-026).
// Without a vehicleId (no session yet), polls GET /api/sessions purely as a reachability
// probe — its result is discarded, state stays at the IDLE/NO_OPERATOR default.
// unreachable becomes true after UNREACHABLE_THRESHOLD consecutive poll failures.
export function useSystemState(vehicleId: string | null, token: string) {
  const [state, setState] = useState<SystemStateResponse>(INITIAL)
  const [unreachable, setUnreachable] = useState(false)
  const failCount = useRef(0)

  useEffect(() => {
    let active = true
    failCount.current = 0 // fresh start per vehicle — failures on the old vehicle must not bleed into the new one

    const poll = async () => {
      try {
        if (vehicleId) {
          const s = await getVehicleState(vehicleId)
          if (active) setState(s)
        } else {
          await listSessions(token)
          if (active) setState(INITIAL)
        }
        if (active) {
          failCount.current = 0
          setUnreachable(false)
        }
      } catch {
        failCount.current++
        if (active && failCount.current >= UNREACHABLE_THRESHOLD) setUnreachable(true)
      }
    }

    poll()
    const id = setInterval(poll, POLL_MS)
    return () => {
      active = false
      clearInterval(id)
    }
  }, [vehicleId, token])

  return { ...state, unreachable }
}
