import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  // Spec files must live inside frontend/'s own node_modules tree — Node resolves bare imports
  // (e.g. `from '@playwright/test'`) by walking up from the file's own path, not from this
  // config's location, so a spec outside frontend/ can never see frontend/node_modules.
  testDir: './tests/e2e',
  timeout: 30_000,
  retries: 1,
  // Seit Sprint 54 (E2E-Flow-Ausbau) beanspruchen mehrere Spec-Dateien echte, geteilte
  // Backend-Session-/Vehicle-State: vehicle-002 aus docker-compose.yml wird von
  // 03-session-conflict.spec.ts und 04-emergency-stop.spec.ts nacheinander verwendet, und der
  // Operator "admin" ist über alle Dateien hinweg dieselbe control-server-Operator-ID (der
  // ACTIVE_OPERATOR-Guard hinter POST /logout ist global pro Operator-ID, nicht pro
  // Browser-Session) — 02-logout.spec.ts läuft daher bewusst VOR den vehicle-002-Tests, solange
  // "admin" noch keine offene ACTIVE_OPERATOR-Session hält. Parallele Worker würden hier zu Race
  // Conditions zwischen Dateien führen. workers:1 erzwingt eine deterministische, serielle
  // Ausführung in alphabetischer Dateireihenfolge (CLAUDE.MD §17).
  workers: 1,
  use: {
    baseURL: 'http://localhost:3000',
    // WebRTC flags (ADR-006 — non-blocking E2E)
    launchOptions: {
      args: [
        '--allow-insecure-localhost',
        '--use-fake-ui-for-media-stream',
        '--use-fake-device-for-media-stream',
      ],
    },
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  // Non-blocking: E2E failures don't break CI (ADR-006)
  reporter: [['html', { outputFolder: './tests/e2e/reports' }]],
})
