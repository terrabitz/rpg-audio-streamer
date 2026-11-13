import { defineConfig, devices } from '@playwright/test'
import { recording, recordingViewport } from './e2e/fixtures'

const apiPort = process.env.E2E_API_PORT ?? '18080'
const uiPort = process.env.E2E_UI_PORT ?? '15173'

// Recording mode slows interactions so per-flow GIFs are watchable.
const recordingOptions = recording
  ? {
      video: { mode: 'on' as const, size: recordingViewport },
      viewport: recordingViewport,
      launchOptions: { slowMo: 600 },
    }
  : {}

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: 0,
  workers: 1,
  timeout: recording ? 300_000 : 30_000,
  expect: { timeout: recording ? 30_000 : 10_000 },
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: `http://127.0.0.1:${uiPort}`,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'], ...recordingOptions } }],
  webServer: [
    {
      command: 'node e2e/server.mjs',
      url: `http://127.0.0.1:${apiPort}/api/v1/auth/status`,
      reuseExistingServer: false,
      timeout: 120_000,
      gracefulShutdown: { signal: 'SIGTERM', timeout: 5_000 },
    },
    {
      command: `npm run build && npm run preview -- --host 127.0.0.1 --port ${uiPort} --strictPort`,
      url: `http://127.0.0.1:${uiPort}`,
      env: { API_PROXY_TARGET: `http://127.0.0.1:${apiPort}` },
      reuseExistingServer: false,
      timeout: 120_000,
    },
  ],
})
