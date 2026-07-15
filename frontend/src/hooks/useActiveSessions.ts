import { useEffect, useState } from 'react'
import { listSessions, type ActiveSession } from '@/lib/api-client'

// Polls GET /api/sessions — extracted out of App.tsx's AppContent (was previously inlined there)
// so FleetOverview can also consume it: determining "does vehicle X have an active operator"
// (ADR-028's 2026-07-15 proactive-takeover update) needs the same data.
export function useActiveSessions(token: string | null) {
  const [activeSessions, setActiveSessions] = useState<ActiveSession[]>([])
  const [hasPolled, setHasPolled] = useState(false)

  useEffect(() => {
    if (!token) return
    let active = true

    const poll = async () => {
      try {
        const data = await listSessions(token)
        if (active) setActiveSessions(data ?? [])
      } catch {
        // keep stale list on error
      } finally {
        if (active) setHasPolled(true)
      }
    }

    poll()
    const tid = setInterval(poll, 3000)
    return () => { active = false; clearInterval(tid) }
  }, [token])

  return { activeSessions, hasPolled }
}
