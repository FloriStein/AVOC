import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import UserManagementPanel from './UserManagementPanel'
import * as apiClient from '@/lib/api-client'

vi.mock('@/lib/api-client', () => ({
  listUsers: vi.fn(),
  createUser: vi.fn(),
  deleteUser: vi.fn(),
  updateUserRole: vi.fn(),
}))

const baseUsers: apiClient.UserInfo[] = [
  { id: 1, username: 'admin', role: 'ADMIN', is_active: true, created_at: '2026-01-01T00:00:00Z' },
  { id: 2, username: 'op1', role: 'OBSERVER', is_active: true, created_at: '2026-01-02T00:00:00Z' },
]

const defaultProps = { token: 'tok', currentUsername: 'admin', onClose: vi.fn() }

describe('UserManagementPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(apiClient.listUsers).mockResolvedValue(baseUsers)
    vi.mocked(apiClient.createUser).mockResolvedValue(undefined)
    vi.mocked(apiClient.deleteUser).mockResolvedValue(undefined)
    vi.mocked(apiClient.updateUserRole).mockResolvedValue(undefined)
  })

  it('zeigt alle Nutzer aus der API', async () => {
    render(<UserManagementPanel {...defaultProps} />)
    await waitFor(() => {
      expect(screen.getByText('admin')).toBeInTheDocument()
      expect(screen.getByText('op1')).toBeInTheDocument()
    })
  })

  it('kein Löschen-Button für eigenen Account', async () => {
    render(<UserManagementPanel {...defaultProps} />)
    await waitFor(() => screen.getByText('op1'))
    const rows = screen.getAllByRole('row')
    const adminRow = rows.find(r => within(r).queryByText('admin'))
    expect(adminRow).toBeDefined()
    expect(within(adminRow!).queryByRole('button', { name: /löschen/i })).toBeNull()
  })

  it('Löschen-Button vorhanden für anderen Account', async () => {
    render(<UserManagementPanel {...defaultProps} />)
    await waitFor(() => screen.getByText('op1'))
    const rows = screen.getAllByRole('row')
    const op1Row = rows.find(r => within(r).queryByText('op1'))
    expect(op1Row).toBeDefined()
    expect(within(op1Row!).getByRole('button', { name: /löschen/i })).toBeInTheDocument()
  })

  it('ruft deleteUser auf nach Bestätigung', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<UserManagementPanel {...defaultProps} />)
    await waitFor(() => screen.getByText('op1'))
    const rows = screen.getAllByRole('row')
    const op1Row = rows.find(r => within(r).queryByText('op1'))!
    await userEvent.click(within(op1Row).getByRole('button', { name: /löschen/i }))
    expect(apiClient.deleteUser).toHaveBeenCalledWith('tok', 2)
  })

  it('ruft deleteUser NICHT auf wenn Bestätigung abgelehnt', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    render(<UserManagementPanel {...defaultProps} />)
    await waitFor(() => screen.getByText('op1'))
    const rows = screen.getAllByRole('row')
    const op1Row = rows.find(r => within(r).queryByText('op1'))!
    await userEvent.click(within(op1Row).getByRole('button', { name: /löschen/i }))
    expect(apiClient.deleteUser).not.toHaveBeenCalled()
  })

  it('Anlegen-Button ist disabled wenn Felder leer', async () => {
    render(<UserManagementPanel {...defaultProps} />)
    await waitFor(() => screen.getByText('op1'))
    expect(screen.getByRole('button', { name: /anlegen/i })).toBeDisabled()
  })

  it('ruft createUser mit korrekten Daten auf', async () => {
    render(<UserManagementPanel {...defaultProps} />)
    await waitFor(() => screen.getByText('op1'))
    await userEvent.type(screen.getByPlaceholderText('Benutzername'), 'newop')
    await userEvent.type(screen.getByPlaceholderText('Passwort'), 'secret123')
    await userEvent.click(screen.getByRole('button', { name: /anlegen/i }))
    await waitFor(() =>
      expect(apiClient.createUser).toHaveBeenCalledWith('tok', 'newop', 'secret123', 'OBSERVER')
    )
  })

  it('zeigt API-Fehler als Fehlermeldung an', async () => {
    vi.mocked(apiClient.listUsers).mockRejectedValue(new Error('403 Forbidden'))
    render(<UserManagementPanel {...defaultProps} />)
    await waitFor(() =>
      expect(screen.getByText(/403 Forbidden/)).toBeInTheDocument()
    )
  })

  it('ruft updateUserRole auf wenn Rolle geändert wird', async () => {
    render(<UserManagementPanel {...defaultProps} />)
    await waitFor(() => screen.getByText('op1'))
    const rows = screen.getAllByRole('row')
    const op1Row = rows.find(r => within(r).queryByText('op1'))!
    const select = within(op1Row).getByRole('combobox')
    await userEvent.selectOptions(select, 'ACTIVE_OPERATOR')
    expect(apiClient.updateUserRole).toHaveBeenCalledWith('tok', 2, 'ACTIVE_OPERATOR')
  })

  it('kein Löschen/Rollenänderung für Nutzer in aktiver Session', async () => {
    render(<UserManagementPanel {...defaultProps} activeOperatorIds={['op1']} />)
    await waitFor(() => screen.getByText('op1'))
    const rows = screen.getAllByRole('row')
    const op1Row = rows.find(r => within(r).queryByText('op1'))!
    expect(within(op1Row).queryByRole('button', { name: /löschen/i })).toBeNull()
    const select = within(op1Row).getByRole('combobox')
    expect(select).toBeDisabled()
  })

  it('mehrere aktive Operatoren gleichzeitig gesperrt (Multi-Vehicle, ADR-026)', async () => {
    const threeUsers = [...baseUsers, { id: 3, username: 'op2', role: 'OBSERVER', is_active: true, created_at: '2026-01-03T00:00:00Z' }]
    vi.mocked(apiClient.listUsers).mockResolvedValue(threeUsers)
    render(<UserManagementPanel {...defaultProps} activeOperatorIds={['op1', 'op2']} />)
    await waitFor(() => screen.getByText('op2'))
    const rows = screen.getAllByRole('row')
    const op1Row = rows.find(r => within(r).queryByText('op1'))!
    const op2Row = rows.find(r => within(r).queryByText('op2'))!
    expect(within(op1Row).getByRole('combobox')).toBeDisabled()
    expect(within(op2Row).getByRole('combobox')).toBeDisabled()
  })

  it('schließt Panel bei Klick auf ✕', async () => {
    render(<UserManagementPanel {...defaultProps} />)
    await waitFor(() => screen.getByText('op1'))
    await userEvent.click(screen.getByText('✕'))
    expect(defaultProps.onClose).toHaveBeenCalled()
  })
})
