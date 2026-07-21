import { test, expect } from '@playwright/test'
import { login } from './helpers'

// E2E-Baseline: Dashboard lädt und zeigt initialen Zustand (ADR-006 non-blocking).
// Voraussetzung: Docker-Stack auf localhost:3000 läuft.
//
// Login-Flow (2026-07-21, Nachtrag): Seit ADR-024 zeigt die App zuerst ein LoginPanel statt
// direkt das Dashboard. Nach Login landet man außerdem laut App.tsx zuerst auf FleetOverview
// (Sprint 22), nicht im Cockpit — erst nach Fahrzeugauswahl + Teleoperate/Beobachten wird
// AppContent (Safety/Connection-Panel, Emergency-Stop) gerendert. `login()` (./helpers.ts, seit
// Sprint 54 geteilt mit den weiteren E2E-Specs) bildet den kompletten Weg nach: Anmelden
// (Seed-Admin aus SeedAdmin/ADR-024) → erstes Fahrzeug in FleetOverview auswählen → Teleoperate/
// Beobachten. Jeder Test hier claimt eine echte Server-Session auf dem Fahrzeug und gibt sie nie
// wieder frei (kein logout/endSession) — ab dem zweiten Test in diesem Lauf zeigt
// FleetVehicleDetail.tsx für das bereits belegte Fahrzeug "Beobachten" statt "Teleoperate"
// (ADR-028), daher akzeptiert login() bewusst beide Label.

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
