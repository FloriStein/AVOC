// Connection Status Panel — live SYSTEM STATE, latency, session-ID, operator role, telemetry (ADR-016).

import { useState } from 'react'
import { VehicleSelector } from '@/components/VehicleSelector'
import type { ActiveSession } from '@/lib/api-client'

interface TelemetryData {
  speedKmh: number
  batteryPct: number
  status: string
}

interface Props {
  systemState: string
  operatorState: string
  sessionId: string | null
  vehicleId: string | null
  role: string | null            // 'ACTIVE_OPERATOR' | 'OBSERVER' | null
  latency: number
  videoLatency?: number | null
  telemetry?: TelemetryData | null
  activeSessions?: ActiveSession[]
  onStartSession?: (vehicleId: string) => void
  onJoinSession?: (vehicleId: string) => void
  onEndSession?: () => Promise<void>
}

const STATE_COLORS: Record<string, string> = {
  IDLE:          'bg-gray-500',
  CONNECTING:    'bg-blue-500',
  AUTHENTICATED: 'bg-blue-400',
  CONNECTED:     'bg-green-500',
  DEGRADED:      'bg-yellow-500',
  SAFE_MODE:     'bg-red-600',
  RECOVERING:    'bg-orange-500',
}

function StateBadge({ state }: { state: string }) {
  const color = STATE_COLORS[state] ?? 'bg-gray-600'
  return (
    <span className={`px-2 py-0.5 rounded text-white text-xs font-mono ${color}`}>
      {state}
    </span>
  )
}

function LatencyColor(ms: number): string {
  if (ms === 0) return 'text-gray-500'
  if (ms < 50) return 'text-green-400'
  if (ms < 100) return 'text-yellow-400'
  return 'text-red-400'
}

export function ConnectionPanel({ systemState, operatorState, sessionId, vehicleId, role, latency, videoLatency, telemetry, activeSessions, onStartSession, onJoinSession, onEndSession }: Props) {
  const shortId = sessionId ? sessionId.slice(0, 8) + '…' : '—'
  const [ending, setEnding] = useState(false)

  const isActive = systemState === 'CONNECTED' || systemState === 'DEGRADED'

  const handleEndSession = async () => {
    if (!onEndSession) return
    setEnding(true)
    try {
      await onEndSession()
    } finally {
      setEnding(false)
    }
  }

  // Active ACTIVE_OPERATOR sessions another user can join as observer
  const joinableSessions = (activeSessions ?? []).filter(s => s.role === 'ACTIVE_OPERATOR')

  return (
    <section className="bg-gray-800 rounded-lg border border-gray-700 p-4 flex flex-col gap-2">
      <h2 className="text-sm font-semibold text-gray-400 uppercase tracking-wide">Connection</h2>

      <div className="flex justify-between text-sm items-center">
        <span className="text-gray-400">State</span>
        <StateBadge state={systemState} />
      </div>

      <div className="flex justify-between text-sm items-center">
        <span className="text-gray-400">Control</span>
        <span className={`font-mono text-sm ${LatencyColor(latency)}`}>
          {latency > 0 ? `${latency} ms` : '— ms'}
        </span>
      </div>

      <div className="flex justify-between text-sm items-center">
        <span className="text-gray-400">Video</span>
        <span className={`font-mono text-sm ${videoLatency !== null && videoLatency !== undefined ? LatencyColor(videoLatency) : 'text-gray-500'}`}>
          {videoLatency !== null && videoLatency !== undefined ? `${videoLatency} ms` : '— ms'}
        </span>
      </div>

      <div className="flex justify-between text-sm items-center">
        <span className="text-gray-400">Operator</span>
        <span className="text-gray-300 text-xs font-mono">{operatorState}</span>
      </div>

      {role && (
        <div className="flex justify-between text-sm items-center">
          <span className="text-gray-400">Rolle</span>
          <span
            data-testid="role-badge"
            className={`text-xs font-mono font-semibold px-1.5 py-0.5 rounded ${
              role === 'ACTIVE_OPERATOR'
                ? 'bg-green-900 text-green-300'
                : 'bg-yellow-900 text-yellow-300'
            }`}
          >
            {role === 'ACTIVE_OPERATOR' ? 'Kontrolle' : 'Beobachter'}
          </span>
        </div>
      )}

      <div className="flex justify-between text-sm items-center">
        <span className="text-gray-400">Session</span>
        <span className="font-mono text-gray-500 text-xs" title={sessionId ?? ''}>
          {shortId}
        </span>
      </div>

      {/* Active session — shown when CONNECTED or DEGRADED and a session exists (ADR-025) */}
      {isActive && sessionId && (
        <div className="mt-1 flex flex-col gap-2 border-t border-gray-700 pt-2">
          <div className="flex justify-between text-sm items-center">
            <span className="text-gray-400">Fahrzeug</span>
            <span className="font-mono text-green-400 text-xs">{vehicleId ?? '—'}</span>
          </div>
          <button
            onClick={handleEndSession}
            disabled={ending}
            className="w-full py-1.5 px-3 bg-gray-700 hover:bg-red-900 border border-gray-600 hover:border-red-700
                       disabled:opacity-50 disabled:cursor-not-allowed rounded text-xs text-gray-300
                       hover:text-red-300 font-semibold transition-colors"
          >
            {ending ? 'Beende…' : '⏹ Session beenden'}
          </button>
        </div>
      )}

      {/* Vehicle selector — shown when logged in but no session active yet */}
      {!sessionId && onStartSession && (
        <VehicleSelector
          onStartSession={onStartSession}
          defaultVehicleId={vehicleId}
        />
      )}

      {/* Observer join — shown when no session and there are controllable vehicles */}
      {!sessionId && onJoinSession && (
        <div className="mt-1 border-t border-gray-700 pt-2 flex flex-col gap-1">
          <p className="text-xs text-gray-400 font-medium">Aktive Sitzungen</p>
          {joinableSessions.length === 0 ? (
            <p className="text-xs text-gray-500 text-center py-1">Keine aktive Sitzung</p>
          ) : (
            joinableSessions.map(s => (
              <button
                key={s.session_id}
                onClick={() => onJoinSession(s.vehicle_id)}
                className="flex items-center justify-between w-full rounded px-2 py-1.5 bg-gray-700 hover:bg-indigo-800 text-xs transition-colors"
              >
                <span className="font-mono text-gray-200">{s.vehicle_id}</span>
                <span className="text-gray-400">{s.operator_id} · {s.created_at}</span>
                <span className="text-indigo-300 font-semibold">Ansehen →</span>
              </button>
            ))
          )}
        </div>
      )}

      {/* Global session overview — shown when in an active session (informational) */}
      {sessionId && activeSessions && activeSessions.length > 0 && (
        <div className="mt-1 border-t border-gray-700 pt-2 flex flex-col gap-1">
          <p className="text-xs text-gray-400 font-medium">Alle Sitzungen</p>
          {activeSessions.map(s => (
            <div key={s.session_id} className="flex items-center justify-between text-xs">
              <span className="font-mono text-gray-300">{s.vehicle_id}</span>
              <span className="text-gray-500">{s.operator_id}</span>
              <span className={`font-mono px-1 rounded ${s.role === 'ACTIVE_OPERATOR' ? 'text-green-400' : 'text-yellow-400'}`}>
                {s.role === 'ACTIVE_OPERATOR' ? 'aktiv' : 'obs'}
              </span>
            </div>
          ))}
        </div>
      )}

      {/* Reconnecting hint */}
      {systemState === 'RECOVERING' && (
        <p className="text-xs text-orange-400 text-center mt-1">Verbinde neu…</p>
      )}

      {/* Telemetry — shown when MQTT data is available */}
      {telemetry && (
        <>
          <hr className="border-gray-700" />
          <div className="flex justify-between text-sm items-center">
            <span className="text-gray-400">Speed</span>
            <span className="font-mono text-blue-300 text-sm">{telemetry.speedKmh.toFixed(1)} km/h</span>
          </div>
          <div className="flex justify-between text-sm items-center">
            <span className="text-gray-400">Battery</span>
            <span className={`font-mono text-sm ${telemetry.batteryPct < 20 ? 'text-red-400' : 'text-green-400'}`}>
              {telemetry.batteryPct.toFixed(0)} %
            </span>
          </div>
          {telemetry.status && (
            <div className="flex justify-between text-sm items-center">
              <span className="text-gray-400">Status</span>
              <span className="text-gray-300 text-xs font-mono">{telemetry.status}</span>
            </div>
          )}
        </>
      )}
    </section>
  )
}
