import { test, expect, Page } from '@playwright/test'

// E2E-Baseline: Dashboard lädt und zeigt initialen Zustand (ADR-006 non-blocking).
// Voraussetzung: Docker-Stack auf localhost:3000 läuft.
//
// Login-Flow (2026-07-21, Nachtrag): Seit ADR-024 zeigt die App zuerst ein LoginPanel statt
// direkt das Dashboard. Nach Login landet man außerdem laut App.tsx zuerst auf FleetOverview
// (Sprint 22), nicht im Cockpit — erst nach Fahrzeugauswahl + Teleoperate/Beobachten wird
// AppContent (Safety/Connection-Panel, Emergency-Stop) gerendert. `login()` bildet den kompletten
// Weg nach: Anmelden (Seed-Admin aus SeedAdmin/ADR-024) → erstes Fahrzeug in FleetOverview
// auswählen (vehicle-mock/vehicle-mock-2 aus docker-compose.yml sind immer vorhanden) →
// Teleoperate/Beobachten. Jeder Test claimt eine echte Server-Session auf dem Fahrzeug und gibt
// sie nie wieder frei (kein logout/endSession) — ab dem zweiten Test in diesem Lauf zeigt
// FleetVehicleDetail.tsx für das bereits belegte Fahrzeug "Beobachten" statt "Teleoperate"
// (ADR-028). Beide Buttons rufen aber identisch session.startSession auf
// (FleetOverview.tsx: onTeleoperate={session.startSession}, onObserve={session.startSession}),
// daher hier bewusst beide Label akzeptieren statt nur "Teleoperate".
async function login(page: Page) {
  await page.goto('/')
  await page.getByPlaceholder('admin').fill('admin')
  await page.getByPlaceholder('••••••••').fill(process.env.ADMIN_PASSWORD ?? 'admin_dev_secret')
  await page.getByRole('button', { name: 'Anmelden' }).click()

  const fleetSection = page.locator('section', { has: page.getByRole('heading', { name: 'Fahrzeuge' }) })
  await fleetSection.getByRole('button').first().click({ timeout: 15_000 })
  await page.getByRole('button', { name: /^(Teleoperate|Beobachten)$/ }).click()
}

test.describe('AVOC Dashboard', () => {
  test('Dashboard lädt und zeigt AVOC-Header', async ({ page }) => {
    await login(page)
    await expect(page.locator('h1')).toContainText('AVOC')
  })

  test('Initialer SYSTEM STATE ist IDLE', async ({ page }) => {
    await login(page)
    // Zwei State-Badges zeigen denselben Wert (Header-SystemStateBadge + ConnectionPanel im
    // main-Content) — 'text=IDLE' matcht real zweimal (Strict-Mode-Violation), daher auf den
    // Header beschränkt statt .first() (bewusster Anker, nicht Zufallstreffer).
    await expect(page.locator('header').getByText('IDLE')).toBeVisible({ timeout: 5_000 })
  })

  test('Safety Panel ist sichtbar', async ({ page }) => {
    await login(page)
    await expect(page.locator('text=Safety')).toBeVisible()
  })

  test('Connection Panel ist sichtbar', async ({ page }) => {
    await login(page)
    await expect(page.locator('text=Connection')).toBeVisible()
  })

  test('Emergency Stop Button ist im DOM', async ({ page }) => {
    await login(page)
    await expect(page.locator('button:has-text("Emergency Stop")')).toBeVisible()
  })
})
