import { test, expect } from '@playwright/test'
import { login } from './helpers'

// E2ETEST-03: echter Klick-Flow für den Emergency-Stop-Button (SafetyPanel.tsx). Zielt bewusst
// auf vehicle-002 (docker-compose.yml) statt auf das erste Fahrzeug in FleetOverview — die
// FLEET_VEHICLES-Einträge (lastenzug-01/lastenrad-01) verbinden sich nie per WebSocket mit dem
// control-server und bleiben daher dauerhaft IDLE (isConnected nie true, Button bliebe disabled).
// vehicle-002 läuft als echter vehicle-mock-Container und erreicht CONNECTED wie ein reales
// Fahrzeug. POST /emergency-stop prüft laut cmd/control-server/main.go:handleEmergencyStop keine
// Operator-Rolle — der Test funktioniert unabhängig davon, ob diese Session ACTIVE_OPERATOR oder
// (z.B. durch 03-session-conflict.spec.ts) bereits OBSERVER ist.
test.describe('Emergency Stop echter Klick-Flow (E2ETEST-03)', () => {
  test('Klick löst sichtbare SAFE_MODE-Transition aus, Button danach disabled', async ({ page }) => {
    await login(page, /vehicle-002/)

    const estopButton = page.getByRole('button', { name: /Emergency Stop/ })
    await expect(estopButton).toBeEnabled({ timeout: 20_000 })

    await estopButton.click()

    await expect(page.locator('header').getByText('SAFE_MODE')).toBeVisible({ timeout: 10_000 })
    await expect(estopButton).toBeDisabled()
  })
})
