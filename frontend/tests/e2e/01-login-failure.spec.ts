import { test, expect } from '@playwright/test'

// E2ETEST-01: falsches Passwort → LoginPanel.tsx zeigt "Ungültige Zugangsdaten" (catch-Block von
// handleSubmit) und bleibt auf dem LoginPanel, kein State-Übergang zu FleetOverview.
test.describe('Login-Fehlerfall (E2ETEST-01)', () => {
  test('Falsches Passwort zeigt Fehlermeldung, kein Übergang zu FleetOverview', async ({ page }) => {
    await page.goto('/')
    await page.getByPlaceholder('admin').fill('admin')
    await page.getByPlaceholder('••••••••').fill('definitely-wrong-password')
    await page.getByRole('button', { name: 'Anmelden' }).click()

    await expect(page.getByText('Ungültige Zugangsdaten')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByRole('heading', { name: 'Fahrzeuge' })).not.toBeVisible()
    await expect(page.getByRole('button', { name: 'Anmelden' })).toBeVisible()
  })
})
