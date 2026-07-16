import { useState } from 'react'
import type { FleetAlert } from '@/lib/api-client'

interface Props {
  alerts: FleetAlert[]
  onAcknowledge: (id: string) => Promise<void>
  muted: boolean
  onToggleMuted: () => void
}

const SEVERITY_STYLE: Record<string, string> = {
  critical: 'border-red-700 bg-red-950/50 text-red-300',
  warning: 'border-yellow-700 bg-yellow-950/50 text-yellow-300',
  info: 'border-gray-600 bg-gray-900/50 text-gray-300',
}

export function FleetAlertsPanel({ alerts, onAcknowledge, muted, onToggleMuted }: Props) {
  // Per-row (not global) so acknowledging one alert doesn't disable a different alert's button.
  const [acknowledgingId, setAcknowledgingId] = useState<string | null>(null)

  const handleAcknowledge = async (id: string) => {
    setAcknowledgingId(id)
    try {
      await onAcknowledge(id)
    } finally {
      setAcknowledgingId(null)
    }
  }

  return (
    <section className="bg-gray-800 rounded-lg border border-gray-700 p-4 flex flex-col gap-2 min-h-0">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-gray-400 uppercase tracking-wide">Alerts</h2>
        <button
          onClick={onToggleMuted}
          aria-pressed={muted}
          title={
            muted
              ? 'Ton für neue Alerts ist stummgeschaltet — klicken zum Aktivieren'
              : 'Ton für neue Alerts ist aktiv — klicken zum Stummschalten'
          }
          className="px-2 py-0.5 rounded border border-gray-600 text-gray-300 hover:bg-gray-700 text-xs font-semibold transition-colors"
        >
          {muted ? 'Stumm' : 'Ton an'}
        </button>
      </div>

      {alerts.length === 0 ? (
        <p className="text-xs text-gray-500 text-center py-4">Keine Alerts</p>
      ) : (
        <div className="flex flex-col gap-1.5 overflow-y-auto">
          {alerts.map((a) => (
            <div
              key={a.id}
              className={`flex flex-col gap-1 rounded border px-2 py-1.5 text-xs ${
                SEVERITY_STYLE[a.severity] ?? SEVERITY_STYLE.info
              }`}
            >
              <div className="flex items-center justify-between">
                <span className="font-mono font-semibold">{a.vehicle_id}</span>
                <span className="text-gray-500">{new Date(a.created_at).toLocaleTimeString()}</span>
              </div>
              <p>{a.message}</p>
              {a.acknowledged_at ? (
                <span className="text-gray-500">
                  ✓ {a.acknowledged_by} · {new Date(a.acknowledged_at).toLocaleTimeString()}
                </span>
              ) : (
                <button
                  onClick={() => handleAcknowledge(a.id)}
                  disabled={acknowledgingId === a.id}
                  className="self-start px-2 py-0.5 bg-gray-700 hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed rounded text-xs text-gray-200 font-semibold transition-colors"
                >
                  {acknowledgingId === a.id ? '…' : 'Bestätigen'}
                </button>
              )}
            </div>
          ))}
        </div>
      )}
    </section>
  )
}
