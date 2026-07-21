import { Page } from '@playwright/test'

const ADMIN_PASSWORD = process.env.ADMIN_PASSWORD ?? 'admin_dev_secret'

// Login only — leaves the operator on FleetOverview (Sprint 22 landing view, ADR-024), before
// any vehicle/session claim. Shared by every spec that needs a fresh authenticated session.
export async function authenticate(page: Page) {
  await page.goto('/')
  await page.getByPlaceholder('admin').fill('admin')
  await page.getByPlaceholder('••••••••').fill(ADMIN_PASSWORD)
  await page.getByRole('button', { name: 'Anmelden' }).click()
}

export function fleetSection(page: Page) {
  return page.locator('section', { has: page.getByRole('heading', { name: 'Fahrzeuge' }) })
}

// `name` omitted selects whatever the fleet list shows first (non-deterministic across runs —
// fine for the original baseline spec, which doesn't care which vehicle it gets).
export function fleetVehicleButton(page: Page, name?: RegExp) {
  return name ? fleetSection(page).getByRole('button', { name }) : fleetSection(page).getByRole('button').first()
}

// Full login through to the cockpit (AppContent) — picks a vehicle in FleetOverview (first by
// default, or matching `vehicleName` if given) and claims/joins its session via
// Teleoperate/Beobachten. Both buttons call session.startSession identically (ADR-028) — once a
// vehicle is claimed by any operator, later logins see "Beobachten" instead of "Teleoperate" for
// it, so this helper accepts either label; callers asserting a specific role/label should check
// it themselves before/after calling this.
export async function login(page: Page, vehicleName?: RegExp) {
  await authenticate(page)
  await fleetVehicleButton(page, vehicleName).click({ timeout: 15_000 })
  await page.getByRole('button', { name: /^(Teleoperate|Beobachten)$/ }).click()
}
