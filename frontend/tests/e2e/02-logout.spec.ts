import { test, expect } from '@playwright/test'
import { authenticate } from './helpers'

// E2ETEST-04: Logout über FleetOverview.tsx's eigenen "Abmelden"-Button im Header (vor
// Fahrzeugauswahl/Session-Claim) — bewusst NICHT der Cockpit-"Abmelden"-Button in App.tsx, der
// disabled ist solange eine ACTIVE_OPERATOR-Session aktiv ist (Backend-409-Guard, siehe
// handleLogout in cmd/control-server/main.go).
test.describe('Logout-Flow über FleetOverview (E2ETEST-04)', () => {
  test('Abmelden vor Fahrzeugauswahl beendet Session serverseitig, zurück zu LoginPanel', async ({ page }) => {
    await authenticate(page)
    await expect(page.getByRole('heading', { name: 'Fahrzeuge' })).toBeVisible({ timeout: 15_000 })

    const logoutButton = page.getByRole('button', { name: 'Abmelden' })
    await expect(logoutButton).toBeEnabled()
    await logoutButton.click()

    // POST /logout liefert nur dann 204, wenn HasActiveOperatorSession(operatorID) false ist —
    // sonst 409 "active_session", und useSession.ts's disconnect() bricht dann früh ab, OHNE
    // token/state zu löschen (bleibt auf FleetOverview). Der sichtbare Übergang zurück zum
    // LoginPanel beweist also den erfolgreichen Server-Roundtrip, nicht nur einen Client-Reset.
    await expect(page.getByRole('button', { name: 'Anmelden' })).toBeVisible({ timeout: 10_000 })
  })
})
