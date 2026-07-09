import { useEffect, useState } from 'react'
import { listUsers, createUser, deleteUser, updateUserRole, UserInfo } from '@/lib/api-client'

interface Props {
  token: string
  currentUsername: string
  // Multiple vehicles can each have their own ACTIVE_OPERATOR at the same time (ADR-026) —
  // a single activeOperatorId would only lock one of them, leaving the others editable.
  activeOperatorIds?: string[]
  onClose: () => void
}

const ROLES = ['OBSERVER', 'STANDBY', 'ACTIVE_OPERATOR', 'ADMIN']

export default function UserManagementPanel({ token, currentUsername, activeOperatorIds, onClose }: Props) {
  const [users, setUsers] = useState<UserInfo[]>([])
  const [error, setError] = useState<string | null>(null)
  const [newUsername, setNewUsername] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [newRole, setNewRole] = useState('OBSERVER')

  const load = async () => {
    try {
      const data = await listUsers(token)
      setUsers(data ?? [])
      setError(null)
    } catch (e) {
      setError(String(e))
    }
  }

  useEffect(() => { load() }, [])

  const handleCreate = async () => {
    if (!newUsername || !newPassword) return
    try {
      await createUser(token, newUsername, newPassword, newRole)
      setNewUsername('')
      setNewPassword('')
      setNewRole('OBSERVER')
      load()
    } catch (e) {
      setError(String(e))
    }
  }

  const handleDelete = async (id: number, username: string) => {
    if (!confirm(`Nutzer "${username}" wirklich löschen?`)) return
    try {
      await deleteUser(token, id)
      load()
    } catch (e) {
      setError(String(e))
    }
  }

  const handleRoleChange = async (id: number, role: string) => {
    try {
      await updateUserRole(token, id, role)
      load()
    } catch (e) {
      setError(String(e))
    }
  }

  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/60">
      <div className="w-full max-w-3xl rounded-2xl border border-gray-700 bg-gray-900 p-6 shadow-2xl">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-white">Nutzerverwaltung</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-white text-xl leading-none">✕</button>
        </div>

        {error && <p className="mb-3 text-sm text-red-400">{error}</p>}

        <div className="mb-4 overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-700 text-left text-xs text-gray-400">
                <th className="pb-2 pr-3">#</th>
                <th className="pb-2 pr-3">Benutzername</th>
                <th className="pb-2 pr-3">Rolle</th>
                <th className="pb-2 pr-3">Aktiv</th>
                <th className="pb-2">Aktion</th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => {
                const isSelf = u.username === currentUsername
                const isActiveSession = !!activeOperatorIds?.includes(u.username)
                const locked = isSelf || isActiveSession
                return (
                  <tr key={u.id} className="border-b border-gray-800">
                    <td className="py-2 pr-3 font-mono text-gray-500 text-xs">{u.id}</td>
                    <td className="py-2 pr-3 font-mono text-gray-300">
                      <span>{u.username}</span>
                      {isSelf && <span className="ml-1 text-xs text-indigo-400">(ich)</span>}
                    </td>
                    <td className="py-2 pr-3">
                      <select
                        value={u.role}
                        onChange={(e) => handleRoleChange(u.id, e.target.value)}
                        disabled={locked}
                        title={isSelf ? 'Eigene Rolle nicht änderbar' : isActiveSession ? 'Nutzer in aktiver Session' : undefined}
                        className="rounded border border-gray-600 bg-gray-800 px-2 py-1 text-xs text-white disabled:opacity-40 disabled:cursor-not-allowed"
                      >
                        {ROLES.map((r) => <option key={r} value={r}>{r}</option>)}
                      </select>
                    </td>
                    <td className="py-2 pr-3">
                      <span className={u.is_active ? 'text-green-400' : 'text-gray-500'}>
                        {u.is_active ? 'Ja' : 'Nein'}
                      </span>
                    </td>
                    <td className="py-2">
                      {!locked && (
                        <button
                          onClick={() => handleDelete(u.id, u.username)}
                          className="rounded px-2 py-1 text-xs text-red-400 hover:bg-red-900/30"
                        >
                          Löschen
                        </button>
                      )}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>

        <div className="border-t border-gray-700 pt-4">
          <p className="mb-2 text-xs font-medium text-gray-400">Neuen Nutzer anlegen</p>
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
            <input
              value={newUsername}
              onChange={(e) => setNewUsername(e.target.value)}
              placeholder="Benutzername"
              className="rounded border border-gray-600 bg-gray-800 px-2 py-1 text-sm text-white placeholder-gray-500"
            />
            <input
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              placeholder="Passwort"
              className="rounded border border-gray-600 bg-gray-800 px-2 py-1 text-sm text-white placeholder-gray-500"
            />
            <select
              value={newRole}
              onChange={(e) => setNewRole(e.target.value)}
              className="rounded border border-gray-600 bg-gray-800 px-2 py-1 text-sm text-white"
            >
              {ROLES.map((r) => <option key={r} value={r}>{r}</option>)}
            </select>
          </div>
          <button
            onClick={handleCreate}
            disabled={!newUsername || !newPassword}
            className="mt-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
          >
            Anlegen
          </button>
        </div>
      </div>
    </div>
  )
}
