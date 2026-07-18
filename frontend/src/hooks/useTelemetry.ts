import { useEffect, useState } from 'react'

export interface TelemetryData {
  speedKmh: number
  batteryPct: number
  status: string
  // Actuation feedback (ADR-021) — populated when vehicle-mock or real vehicle is connected
  steerCommanded: number
  throttleCommanded: number
  brakeCommanded: number
  steerActual: number
  throttleActual: number
  /** vehicle-side publish timestamp (ms since epoch) of this telemetry sample, 0 if unknown */
  timestampMs: number
  /** OBS-01: ms since this telemetry sample was published — a heartbeat independent of command
   *  ACKs (which only update while the operator is actively issuing commands), null if unknown */
  ageSinceUpdateMs: number | null
}

const POLL_MS = 1000

export function useTelemetry(vehicleId: string | null): TelemetryData | null {
  const [data, setData] = useState<TelemetryData | null>(null)

  useEffect(() => {
    if (!vehicleId) { setData(null); return }

    let active = true
    const poll = async () => {
      try {
        const res = await fetch(`/telemetry/latest/${vehicleId}`)
        if (!res.ok) return
        const json = await res.json() as {
          speed_kmh: number; battery_pct: number; status: string
          steer_commanded?: number; throttle_commanded?: number; brake_commanded?: number
          steer_actual?: number; throttle_actual?: number; timestamp?: number
        }
        const timestampMs = json.timestamp ?? 0
        if (active) setData({
          speedKmh: json.speed_kmh ?? 0,
          batteryPct: json.battery_pct ?? 0,
          status: json.status ?? '',
          steerCommanded: json.steer_commanded ?? 0,
          throttleCommanded: json.throttle_commanded ?? 0,
          brakeCommanded: json.brake_commanded ?? 0,
          steerActual: json.steer_actual ?? 0,
          throttleActual: json.throttle_actual ?? 0,
          timestampMs,
          ageSinceUpdateMs: timestampMs > 0 ? Date.now() - timestampMs : null,
        })
      } catch {
        // MQTT data not yet available — keep last known
      }
    }

    poll()
    const id = setInterval(poll, POLL_MS)
    return () => { active = false; clearInterval(id) }
  }, [vehicleId])

  return data
}
