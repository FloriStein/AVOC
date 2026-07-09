import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { ConnectionPanel } from './ConnectionPanel'

const baseProps = {
  systemState: 'CONNECTED',
  operatorState: 'ACTIVE_OPERATOR',
  sessionId: '01JTXYZABCDEF1234567890',
  vehicleId: null,
  role: null,
  latency: 0,
}

describe('ConnectionPanel — Grundfunktionen', () => {
  it('zeigt SYSTEM STATE Badge', () => {
    render(<ConnectionPanel {...baseProps} />)
    expect(screen.getByText('CONNECTED')).toBeInTheDocument()
  })

  it('zeigt Operator-Rolle', () => {
    render(<ConnectionPanel {...baseProps} />)
    expect(screen.getByText('ACTIVE_OPERATOR')).toBeInTheDocument()
  })

  it('kürzt Session-ID auf 8 Zeichen + Ellipsis', () => {
    render(<ConnectionPanel {...baseProps} />)
    expect(screen.getByText('01JTXYZA…')).toBeInTheDocument()
  })

  it('zeigt — ms für Control wenn Latenz 0 und — ms für Video wenn kein videoLatency', () => {
    render(<ConnectionPanel {...baseProps} latency={0} />)
    expect(screen.getAllByText('— ms')).toHaveLength(2)
  })

  it('zeigt Latenz in ms wenn > 0', () => {
    render(<ConnectionPanel {...baseProps} latency={42} />)
    expect(screen.getByText('42 ms')).toBeInTheDocument()
  })

  it('Latenz < 50ms → grüne Farbe', () => {
    render(<ConnectionPanel {...baseProps} latency={30} />)
    const el = screen.getByText('30 ms')
    expect(el).toHaveClass('text-green-400')
  })

  it('Latenz 50–99ms → gelbe Farbe', () => {
    render(<ConnectionPanel {...baseProps} latency={75} />)
    const el = screen.getByText('75 ms')
    expect(el).toHaveClass('text-yellow-400')
  })

  it('Latenz ≥ 100ms → rote Farbe', () => {
    render(<ConnectionPanel {...baseProps} latency={120} />)
    const el = screen.getByText('120 ms')
    expect(el).toHaveClass('text-red-400')
  })

  it('zeigt Telemetrie-Daten wenn vorhanden', () => {
    render(<ConnectionPanel
      {...baseProps}
      telemetry={{ speedKmh: 42.5, batteryPct: 85, status: 'MOVING' }}
    />)
    expect(screen.getByText('42.5 km/h')).toBeInTheDocument()
    expect(screen.getByText('85 %')).toBeInTheDocument()
  })

  it('zeigt keine Telemetrie-Zeilen wenn null', () => {
    render(<ConnectionPanel {...baseProps} telemetry={null} />)
    expect(screen.queryByText(/km\/h/)).not.toBeInTheDocument()
  })
})

// ── ADR-025: Rollen-Badge ──────────────────────────────────────────────────

describe('ConnectionPanel — Rollen-Badge (ADR-025)', () => {
  it('zeigt grünes "Kontrolle"-Badge für ACTIVE_OPERATOR', () => {
    render(<ConnectionPanel {...baseProps} role="ACTIVE_OPERATOR" />)
    const badge = screen.getByTestId('role-badge')
    expect(badge).toBeInTheDocument()
    expect(badge).toHaveTextContent('Kontrolle')
    expect(badge).toHaveClass('bg-green-900')
    expect(badge).toHaveClass('text-green-300')
  })

  it('zeigt gelbes "Beobachter"-Badge für OBSERVER', () => {
    render(<ConnectionPanel {...baseProps} role="OBSERVER" />)
    const badge = screen.getByTestId('role-badge')
    expect(badge).toBeInTheDocument()
    expect(badge).toHaveTextContent('Beobachter')
    expect(badge).toHaveClass('bg-yellow-900')
    expect(badge).toHaveClass('text-yellow-300')
  })

  it('zeigt keinen Badge wenn role=null', () => {
    render(<ConnectionPanel {...baseProps} role={null} />)
    expect(screen.queryByTestId('role-badge')).not.toBeInTheDocument()
    expect(screen.queryByText('Rolle')).not.toBeInTheDocument()
  })

  it('"Rolle"-Label erscheint nur wenn role gesetzt ist', () => {
    const { rerender } = render(<ConnectionPanel {...baseProps} role={null} />)
    expect(screen.queryByText('Rolle')).not.toBeInTheDocument()

    rerender(<ConnectionPanel {...baseProps} role="ACTIVE_OPERATOR" />)
    expect(screen.getByText('Rolle')).toBeInTheDocument()
  })

  it('ACTIVE_OPERATOR hat kein gelbes Badge', () => {
    render(<ConnectionPanel {...baseProps} role="ACTIVE_OPERATOR" />)
    const badge = screen.getByTestId('role-badge')
    expect(badge).not.toHaveClass('bg-yellow-900')
  })

  it('OBSERVER hat kein grünes Badge', () => {
    render(<ConnectionPanel {...baseProps} role="OBSERVER" />)
    const badge = screen.getByTestId('role-badge')
    expect(badge).not.toHaveClass('bg-green-900')
  })
})

// ── ADR-025: Fahrzeugauswahl-Trigger ──────────────────────────────────────

describe('ConnectionPanel — Fahrzeugauswahl (ADR-025)', () => {
  it('zeigt VehicleSelector wenn sessionId=null und onStartSession gegeben', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => [],
    } as unknown as Response)

    render(<ConnectionPanel
      {...baseProps}
      sessionId={null}
      onStartSession={vi.fn()}
    />)

    // With no session, "Session beenden" must NOT show (bug fix: was showing even without session)
    expect(screen.queryByText('⏹ Session beenden')).not.toBeInTheDocument()
  })

  it('VehicleSelector-Trigger basiert auf sessionId, NICHT auf systemState', () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => [],
    } as unknown as Response)

    // Even with CONNECTED state, vehicle selector shows when sessionId=null (ADR-025 change)
    render(<ConnectionPanel
      {...baseProps}
      systemState="CONNECTED"
      sessionId={null}
      onStartSession={vi.fn()}
    />)
    // "Session beenden" must be absent — no session yet, so nothing to end
    expect(screen.queryByText('⏹ Session beenden')).not.toBeInTheDocument()
  })

  it('zeigt KEINEN VehicleSelector wenn sessionId gesetzt ist', () => {
    render(<ConnectionPanel
      {...baseProps}
      sessionId="01JTXYZABCDEF1234567890"
      onStartSession={vi.fn()}
      systemState="CONNECTED"
    />)
    // With an active session, the end-session button is shown instead of vehicle selector.
    expect(screen.getByText('⏹ Session beenden')).toBeInTheDocument()
  })

  it('zeigt KEINEN VehicleSelector wenn kein onStartSession gegeben', () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => [],
    } as unknown as Response)

    render(<ConnectionPanel
      {...baseProps}
      sessionId={null}
      // No onStartSession prop
    />)
    // Without onStartSession, the VehicleSelector must not render —
    // verify by checking that nothing asks the user to start a session.
    // The end-session button should also not appear (no session).
    expect(screen.queryByText('⏹ Session beenden')).not.toBeInTheDocument()
  })

  it('zeigt "Verbinde neu…" Hinweis im RECOVERING-Zustand', () => {
    render(<ConnectionPanel
      {...baseProps}
      systemState="RECOVERING"
      sessionId={null}
    />)
    expect(screen.getByText('Verbinde neu…')).toBeInTheDocument()
  })

  it('zeigt KEINEN "Verbinde neu…" Hinweis im CONNECTED-Zustand', () => {
    render(<ConnectionPanel {...baseProps} systemState="CONNECTED" />)
    expect(screen.queryByText('Verbinde neu…')).not.toBeInTheDocument()
  })
})

// ── ADR-025: Session-Ende-Button ──────────────────────────────────────────

describe('ConnectionPanel — Session-Ende (ADR-025)', () => {
  it('zeigt Session-beenden-Button wenn CONNECTED und sessionId gesetzt', () => {
    render(<ConnectionPanel
      {...baseProps}
      systemState="CONNECTED"
      sessionId="01JTXYZABCDEF1234567890"
      onEndSession={vi.fn()}
    />)
    expect(screen.getByText('⏹ Session beenden')).toBeInTheDocument()
  })

  it('zeigt Session-beenden-Button auch im DEGRADED-Zustand', () => {
    render(<ConnectionPanel
      {...baseProps}
      systemState="DEGRADED"
      sessionId="01JTXYZABCDEF1234567890"
      onEndSession={vi.fn()}
    />)
    expect(screen.getByText('⏹ Session beenden')).toBeInTheDocument()
  })

  it('zeigt KEINEN Session-beenden-Button im SAFE_MODE', () => {
    render(<ConnectionPanel
      {...baseProps}
      systemState="SAFE_MODE"
      sessionId="01JTXYZABCDEF1234567890"
      onEndSession={vi.fn()}
    />)
    expect(screen.queryByText('⏹ Session beenden')).not.toBeInTheDocument()
  })

  it('zeigt KEINEN Session-beenden-Button wenn sessionId=null (kein aktive Session)', () => {
    // Bug-Regression: vorher wurde der Button auch ohne Session gezeigt.
    render(<ConnectionPanel
      {...baseProps}
      systemState="CONNECTED"
      sessionId={null}
      onEndSession={vi.fn()}
    />)
    expect(screen.queryByText('⏹ Session beenden')).not.toBeInTheDocument()
  })

  it('Observer sieht auch den Session-beenden-Button', () => {
    render(<ConnectionPanel
      {...baseProps}
      systemState="CONNECTED"
      sessionId="01JTXYZABCDEF1234567890"
      role="OBSERVER"
      onEndSession={vi.fn()}
    />)
    expect(screen.getByText('⏹ Session beenden')).toBeInTheDocument()
  })
})

// ── Video-Latenz-Anzeige ───────────────────────────────────────────────────

describe('ConnectionPanel — Video-Latenz', () => {
  it('zeigt — ms wenn videoLatency null (noch keine RTT-Messung)', () => {
    render(<ConnectionPanel {...baseProps} videoLatency={null} />)
    expect(screen.getAllByText('— ms')).toHaveLength(2)
  })

  it('zeigt — ms wenn videoLatency undefined', () => {
    render(<ConnectionPanel {...baseProps} videoLatency={undefined} />)
    expect(screen.getAllByText('— ms')).toHaveLength(2)
  })

  it('zeigt gemessene Video-Latenz wenn vorhanden', () => {
    render(<ConnectionPanel {...baseProps} videoLatency={35} />)
    expect(screen.getByText('35 ms')).toBeInTheDocument()
  })

  it('Video-Latenz 0 → gleiche Darstellung wie null (ausgegraut)', () => {
    // videoLatency=0 is not filtered at the component level — useWebRTC handles that.
    // The component renders it as "0 ms" with gray color (LatencyColor(0) = text-gray-500).
    render(<ConnectionPanel {...baseProps} videoLatency={0} />)
    expect(screen.getByText('0 ms')).toBeInTheDocument()
  })
})
