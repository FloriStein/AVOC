import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  // Spec files must live inside frontend/'s own node_modules tree — Node resolves bare imports
  // (e.g. `from '@playwright/test'`) by walking up from the file's own path, not from this
  // config's location, so a spec outside frontend/ can never see frontend/node_modules.
  testDir: './tests/e2e',
  timeout: 30_000,
  retries: 1,
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
