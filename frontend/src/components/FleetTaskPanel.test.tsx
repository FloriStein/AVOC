import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { FleetTaskPanel } from './FleetTaskPanel'
import * as apiClient from '@/lib/api-client'
import type { FleetVehicle, Task, Station } from '@/lib/api-client'

vi.mock('@/lib/api-client', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api-client')>('@/lib/api-client')
  return { ...actual, listFleetStations: vi.fn() }
})

const V1: FleetVehicle = { id: 'v1', display_name: 'Lastenzug 01' }
const STATION_A: Station = { id: 'station-a', zone_id: 'zone-1', name: 'Station A', created_at: 't1' }
const STATION_B: Station = { id: 'station-b', zone_id: 'zone-1', name: 'Station B', created_at: 't1' }

const PENDING: Task = {
  id: 't1', vehicle_id: 'v1', from_station_id: 'station-a', to_station_id: 'station-b',
  status: 'pending', priority: 1, created_at: '2026-01-01T10:00:00Z',
}
const IN_PROGRESS: Task = { ...PENDING, id: 't2', status: 'in_progress' }
const COMPLETED: Task = { ...PENDING, id: 't3', status: 'completed', status_changed_by: 'op1' }
const CANCELLED: Task = { ...PENDING, id: 't4', status: 'cancelled', status_changed_by: 'op1' }

describe('FleetTaskPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(apiClient.listFleetStations).mockResolvedValue([STATION_A, STATION_B])
  })

  it('zeigt "Keine Tasks" bei leerer Liste', () => {
    render(<FleetTaskPanel tasks={[]} vehicles={[V1]} token="tok" onCreateTask={vi.fn()} onUpdateStatus={vi.fn()} />)
    expect(screen.getByText(/keine tasks/i)).toBeInTheDocument()
  })

  it('zeigt Fahrzeug-/Stationsnamen und Status je Task, sobald Stationen geladen sind', async () => {
    render(<FleetTaskPanel tasks={[PENDING]} vehicles={[V1]} token="tok" onCreateTask={vi.fn()} onUpdateStatus={vi.fn()} />)

    await waitFor(() => expect(screen.getByText(/Lastenzug 01: Station A → Station B/)).toBeInTheDocument())
    expect(screen.getByText('Ausstehend')).toBeInTheDocument()
  })

  it('lädt Stationen für die Dropdowns per listFleetStations', async () => {
    render(<FleetTaskPanel tasks={[]} vehicles={[V1]} token="tok" onCreateTask={vi.fn()} onUpdateStatus={vi.fn()} />)
    await waitFor(() => expect(apiClient.listFleetStations).toHaveBeenCalledWith('tok'))
    expect(await screen.findAllByText('Station A')).not.toHaveLength(0)
  })

  it('Formular ohne Pflichtfelder: zeigt Fehlermeldung, ruft onCreateTask nicht auf', async () => {
    const onCreateTask = vi.fn()
    render(<FleetTaskPanel tasks={[]} vehicles={[V1]} token="tok" onCreateTask={onCreateTask} onUpdateStatus={vi.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: /task anlegen/i }))

    expect(await screen.findByText(/erforderlich/i)).toBeInTheDocument()
    expect(onCreateTask).not.toHaveBeenCalled()
  })

  it('gültiges Formular ruft onCreateTask mit den gewählten Werten auf', async () => {
    const onCreateTask = vi.fn().mockResolvedValue(undefined)
    render(<FleetTaskPanel tasks={[]} vehicles={[V1]} token="tok" onCreateTask={onCreateTask} onUpdateStatus={vi.fn()} />)
    await waitFor(() => expect(apiClient.listFleetStations).toHaveBeenCalled())

    const selects = screen.getAllByRole('combobox')
    fireEvent.change(selects[0], { target: { value: 'v1' } })
    fireEvent.change(selects[1], { target: { value: 'station-a' } })
    fireEvent.change(selects[2], { target: { value: 'station-b' } })
    fireEvent.change(screen.getByRole('spinbutton'), { target: { value: '5' } })

    fireEvent.click(screen.getByRole('button', { name: /task anlegen/i }))

    await waitFor(() =>
      expect(onCreateTask).toHaveBeenCalledWith({
        vehicle_id: 'v1', from_station_id: 'station-a', to_station_id: 'station-b', priority: 5,
      }),
    )
  })

  it('Formular-Fehler bei onCreateTask-Rejection zeigt eine Fehlermeldung', async () => {
    const onCreateTask = vi.fn().mockRejectedValue(new Error('failed'))
    render(<FleetTaskPanel tasks={[]} vehicles={[V1]} token="tok" onCreateTask={onCreateTask} onUpdateStatus={vi.fn()} />)
    await waitFor(() => expect(apiClient.listFleetStations).toHaveBeenCalled())

    const selects = screen.getAllByRole('combobox')
    fireEvent.change(selects[0], { target: { value: 'v1' } })
    fireEvent.change(selects[1], { target: { value: 'station-a' } })
    fireEvent.change(selects[2], { target: { value: 'station-b' } })

    fireEvent.click(screen.getByRole('button', { name: /task anlegen/i }))

    expect(await screen.findByText(/konnte nicht angelegt werden/i)).toBeInTheDocument()
  })

  it('pending zeigt Starten/Stornieren, in_progress zeigt Abschließen/Stornieren', () => {
    render(<FleetTaskPanel tasks={[PENDING, IN_PROGRESS]} vehicles={[V1]} token="tok" onCreateTask={vi.fn()} onUpdateStatus={vi.fn()} />)

    expect(screen.getAllByRole('button', { name: /starten/i })).toHaveLength(1)
    expect(screen.getAllByRole('button', { name: /abschließen/i })).toHaveLength(1)
    expect(screen.getAllByRole('button', { name: /stornieren/i })).toHaveLength(2)
  })

  it('completed/cancelled (terminal) zeigen keine Status-Buttons', () => {
    render(<FleetTaskPanel tasks={[COMPLETED, CANCELLED]} vehicles={[V1]} token="tok" onCreateTask={vi.fn()} onUpdateStatus={vi.fn()} />)

    expect(screen.queryByRole('button', { name: /starten/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /abschließen/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /stornieren/i })).not.toBeInTheDocument()
  })

  it('Klick auf Statuswechsel-Button ruft onUpdateStatus mit id und Zielstatus auf', async () => {
    const onUpdateStatus = vi.fn().mockResolvedValue(undefined)
    render(<FleetTaskPanel tasks={[PENDING]} vehicles={[V1]} token="tok" onCreateTask={vi.fn()} onUpdateStatus={onUpdateStatus} />)

    fireEvent.click(screen.getByRole('button', { name: /starten/i }))

    await waitFor(() => expect(onUpdateStatus).toHaveBeenCalledWith('t1', 'in_progress'))
  })

  it('409/Fehler bei onUpdateStatus zeigt eine Inline-Fehlermeldung in der betroffenen Zeile', async () => {
    const onUpdateStatus = vi.fn().mockRejectedValue(new Error('updateFleetTaskStatus failed: 409'))
    render(<FleetTaskPanel tasks={[PENDING]} vehicles={[V1]} token="tok" onCreateTask={vi.fn()} onUpdateStatus={onUpdateStatus} />)

    fireEvent.click(screen.getByRole('button', { name: /starten/i }))

    expect(await screen.findByText(/statuswechsel fehlgeschlagen/i)).toBeInTheDocument()
  })

  it('Nebenläufigkeit: Statuswechsel eines Tasks deaktiviert nicht die Buttons eines anderen Tasks', () => {
    const onUpdateStatus = vi.fn(() => new Promise<void>(() => {}))
    render(
      <FleetTaskPanel
        tasks={[PENDING, { ...PENDING, id: 't5' }]}
        vehicles={[V1]}
        token="tok"
        onCreateTask={vi.fn()}
        onUpdateStatus={onUpdateStatus}
      />,
    )

    const startButtons = screen.getAllByRole('button', { name: /starten/i })
    fireEvent.click(startButtons[0])

    const remainingStartButtons = screen.getAllByRole('button', { name: /starten/i })
    expect(remainingStartButtons.some((b) => !b.hasAttribute('disabled'))).toBe(true)
  })
})
