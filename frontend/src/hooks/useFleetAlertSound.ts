import { useEffect, useRef, useState } from 'react'
import type { FleetAlert } from '@/lib/api-client'
import { isAudibleSeverity, shouldPlayNow, playAlertTone, type ToneAudioContext } from '@/lib/fleet-alert-sound'

const MUTE_STORAGE_KEY = 'fleet-alert-sound-muted'
const MIN_INTERVAL_MS = 2000

export interface FleetAlertSoundState {
  muted: boolean
  toggleMuted: () => void
}

function readStoredMuted(): boolean {
  try {
    return window.localStorage.getItem(MUTE_STORAGE_KEY) === '1'
  } catch {
    // Privater Modus/deaktiviertes localStorage — Default "nicht stumm", kein Crash.
    return false
  }
}

function createToneAudioContext(): ToneAudioContext | null {
  try {
    const Ctor =
      window.AudioContext ??
      (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
    if (!Ctor) return null
    return new Ctor()
  } catch {
    // AudioContext in dieser Umgebung nicht verfügbar/blockiert — kein Ton statt Crash.
    return null
  }
}

// Spielt bei jedem tatsächlich *neuen* Alert (nicht beim initialen REST-Snapshot) einen kurzen Ton
// für warning/critical-Severity (Grill-Me 2026-07-16). "Neu" heißt: eine Alert-id, die dieser Hook
// noch nie gesehen hat. Die allererste Beobachtung nach Ende des Ladevorgangs (`loading` wird
// `false`) wird nur als Baseline gemerkt, ohne Ton — sonst würde jeder bereits bestehende,
// unquittierte Alert beim Öffnen des Dashboards ein Sound-Feuerwerk auslösen. Absichtlich an
// `loading` statt an "erster Hook-Aufruf" gekoppelt: `alerts` ist beim Mount zunächst `[]` (State-
// Default in useFleetOverview.ts), erst nach dem REST-Fetch die echte Liste — ohne diese
// Unterscheidung würde der Übergang von `[]` auf die echte Liste selbst wie "lauter neue Alerts"
// aussehen.
//
// Danach löst jede zusätzliche id einen Ton aus — bewusst nicht nur exakt beim `alert_created`-WS-
// Event, sondern für jede id, die neu in der (von useFleetOverview gelieferten) `alerts`-Liste
// auftaucht. Das deckt zusätzlich den Resync-nach-Reconnect-Fall ab (FLEET-06s Hub.Broadcast hat
// kein Backlog) — ein Alert, der während eines kurzen WS-Aussetzers entstanden ist und erst durch
// den Resync sichtbar wird, ist für den Operator genauso neu und relevant.
export function useFleetAlertSound(alerts: FleetAlert[], loading: boolean): FleetAlertSoundState {
  const [muted, setMuted] = useState(readStoredMuted)

  const seenIdsRef = useRef<Set<string> | null>(null)
  const baselineEstablishedRef = useRef(false)
  const audioCtxRef = useRef<ToneAudioContext | null | undefined>(undefined)
  const lastPlayedAtRef = useRef<number | null>(null)

  useEffect(() => {
    if (loading) return

    if (!baselineEstablishedRef.current) {
      seenIdsRef.current = new Set(alerts.map((a) => a.id))
      baselineEstablishedRef.current = true
      return
    }

    const seen = seenIdsRef.current!
    const newAlerts = alerts.filter((a) => !seen.has(a.id))
    for (const a of newAlerts) seen.add(a.id)

    if (newAlerts.length === 0) return
    if (muted) return
    if (!newAlerts.some((a) => isAudibleSeverity(a.severity))) return

    const now = Date.now()
    if (!shouldPlayNow(lastPlayedAtRef.current, now, MIN_INTERVAL_MS)) return
    lastPlayedAtRef.current = now

    if (audioCtxRef.current === undefined) {
      audioCtxRef.current = createToneAudioContext()
    }
    const ctx = audioCtxRef.current
    if (!ctx) return

    try {
      // Browser-Autoplay-Policy: resume() liefert ein Promise, das abgelehnt werden kann, wenn
      // (noch) keine Nutzerinteraktion vorlag — abgefangen statt unbehandelte Rejection.
      ctx.resume?.()?.catch(() => {})
      playAlertTone(ctx)
    } catch {
      // Wiedergabe aus irgendeinem Grund blockiert/fehlgeschlagen — rein akustisches Feature, darf
      // das Dashboard nicht crashen.
    }
  }, [alerts, loading, muted])

  const toggleMuted = () => {
    setMuted((prev) => {
      const next = !prev
      try {
        window.localStorage.setItem(MUTE_STORAGE_KEY, next ? '1' : '0')
      } catch {
        // Persistenz optional — Präferenz gilt dann nur für die laufende Session.
      }
      return next
    })
  }

  return { muted, toggleMuted }
}
