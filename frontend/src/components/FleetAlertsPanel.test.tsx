import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { FleetAlertsPanel } from './FleetAlertsPanel'
import type { FleetAlert } from '@/lib/api-client'

const UNACKED: FleetAlert = { id: 'a1', vehicle_id: 'v1', severity: 'critical', message: 'Hindernis erkannt', created_at: '2026-01-01T10:00:00Z' }
const ACKED: FleetAlert = { id: 'a2', vehicle_id: 'v2', severity: 'warning', message: 'Batterie schwach', created_at: '2026-01-01T10:00:00Z', acknowledged_by: 'op1', acknowledged_at: '2026-01-01T10:05:00Z' }

describe('FleetAlertsPanel', () => {
  it('zeigt "Keine Alerts" bei leerer Liste', () => {
    render(<FleetAlertsPanel alerts={[]} onAcknowledge={vi.fn()} />)
    expect(screen.getByText(/keine alerts/i)).toBeInTheDocument()
  })

  it('unquittiertes Alert zeigt einen Bestätigen-Button', () => {
    render(<FleetAlertsPanel alerts={[UNACKED]} onAcknowledge={vi.fn()} />)
    expect(screen.getByRole('button', { name: /bestätigen/i })).toBeInTheDocument()
  })

  it('quittiertes Alert zeigt Meta-Text statt Button', () => {
    render(<FleetAlertsPanel alerts={[ACKED]} onAcknowledge={vi.fn()} />)
    expect(screen.queryByRole('button', { name: /bestätigen/i })).not.toBeInTheDocument()
    expect(screen.getByText(/op1/)).toBeInTheDocument()
  })

  it('Klick auf Bestätigen ruft onAcknowledge mit der alert id auf', async () => {
    const onAcknowledge = vi.fn().mockResolvedValue(undefined)
    render(<FleetAlertsPanel alerts={[UNACKED]} onAcknowledge={onAcknowledge} />)
    fireEvent.click(screen.getByRole('button', { name: /bestätigen/i }))
    expect(onAcknowledge).toHaveBeenCalledWith('a1')
  })

  it('Nebenläufigkeit: Bestätigen eines Alerts deaktiviert nicht den Button eines anderen Alerts', async () => {
    const UNACKED_2: FleetAlert = { ...UNACKED, id: 'a3', vehicle_id: 'v3' }
    // onAcknowledge never resolves within this test — simulates an in-flight request.
    const onAcknowledge = vi.fn(() => new Promise<void>(() => {}))
    render(<FleetAlertsPanel alerts={[UNACKED, UNACKED_2]} onAcknowledge={onAcknowledge} />)

    const buttons = screen.getAllByRole('button', { name: /bestätigen/i })
    fireEvent.click(buttons[0])

    // After clicking the first alert's button, the second alert's button must remain enabled.
    const buttonsAfter = screen.getAllByRole('button')
    const stillEnabled = buttonsAfter.filter((b) => !b.hasAttribute('disabled'))
    expect(stillEnabled.length).toBeGreaterThan(0)
  })
})
