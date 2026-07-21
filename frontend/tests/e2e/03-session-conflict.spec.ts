import { test, expect } from '@playwright/test'
import { authenticate, fleetVehicleButton } from './helpers'

// E2ETEST-02: Session-Konflikt real gegen Backend (ADR-028). Zwei unabhängige
// Playwright-BrowserContexts (= zwei getrennte localStorage/Token, wie zwei echte Operatoren)
// claimen dasselbe Fahrzeug (vehicle-002 aus docker-compose.yml — direkt WS-verbunden, im
// Unterschied zu den FLEET_VEHICLES-Einträgen, die nie CONNECTED werden, siehe emergency-stop
// Spec). Operator A bekommt den Vehicle-Lock (ACTIVE_OPERATOR), Operator B sieht denselben
// Eintrag danach als "Beobachten" statt "Teleoperate" — geprüft gegen die echte
// active-sessions-Antwort des control-servers, nicht nur einen UI-Mock.
test.describe('Session-Konflikt real gegen Backend (E2ETEST-02, ADR-028)', () => {
  test('Zweiter Operator sieht "Beobachten" statt "Teleoperate" auf demselben Fahrzeug', async ({ browser }) => {
    const contextA = await browser.newContext()
    const contextB = await browser.newContext()

    try {
      const pageA = await contextA.newPage()
      await authenticate(pageA)
      await fleetVehicleButton(pageA, /vehicle-002/).click({ timeout: 15_000 })
      await pageA.getByRole('button', { name: 'Teleoperate' }).click()

      const pageB = await contextB.newPage()
      await authenticate(pageB)
      await fleetVehicleButton(pageB, /vehicle-002/).click({ timeout: 15_000 })
      await expect(pageB.getByRole('button', { name: 'Beobachten' })).toBeVisible({ timeout: 10_000 })
      await expect(pageB.getByRole('button', { name: 'Teleoperate' })).toHaveCount(0)

      // Sauberes Teardown statt einfach den Context zu schließen: ein hart getrennter
      // ACTIVE_OPERATOR-WS lässt das Fahrzeug laut ADR-025 als Sicherheitsnetz in SAFE_MODE
      // zurück — das würde 04-emergency-stop.spec.ts (nutzt dasselbe vehicle-002) einen bereits
      // blockierten Emergency-Stop-Button bescheren. "Session beenden" beendet die Session
      // serverseitig sauber und setzt das Fahrzeug zurück auf IDLE.
      await pageA.getByRole('button', { name: /Session beenden/ }).click()
    } finally {
      await contextA.close()
      await contextB.close()
    }
  })
})
