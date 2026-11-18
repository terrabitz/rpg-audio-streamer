import { defineConfig, devices } from '@playwright/test'
import { recording, recordingViewport } from './e2e/fixtures'

process.env.PLAYWRIGHT_NO_COPY_PROMPT = '1'

// Recording mode slows interactions so per-flow GIFs are watchable.
const recordingOptions = recording
  ? {
      video: { mode: 'on' as const, size: recordingViewport },
      viewport: recordingViewport,
    }
  : {}
const failureArtifacts = { trace: 'retain-on-failure' as const, screenshot: 'only-on-failure' as const }

export default defineConfig({
  testDir: './e2e',
  testMatch: '**/*.spec.ts',
  fullyParallel: true,
  globalSetup: './e2e/global-setup.ts',
  forbidOnly: !!process.env.CI,
  retries: 0,
  workers: recording ? 1 : 2,
  timeout: recording ? 300_000 : 60_000,
  expect: { timeout: recording ? 30_000 : 10_000 },
  reporter: [['list']],
  use: {
    trace: 'off',
    screenshot: 'off',
    // Keep real media clocks independent of the host's audio device.
    launchOptions: { args: ['--disable-audio-output'], ...(recording ? { slowMo: 600 } : {}) },
  },
  projects: [
    {
      name: 'chromium',
      grepInvert: recording ? /@discord|@timing|@infrastructure|@youtube/ : /@discord|@timing|@youtube/,
      use: { ...devices['Desktop Chrome'], ...failureArtifacts, ...recordingOptions },
    },
    {
      name: 'chromium-timing', grep: /@timing/, workers: 1,
      use: { ...devices['Desktop Chrome'], ...failureArtifacts, ...recordingOptions },
    },
    ...(recording ? [] : [
      { name: 'chromium-discord', grep: /@discord/, workers: 1, use: { ...devices['Desktop Chrome'] } },
      {
        name: 'chromium-youtube', grep: /@youtube/, workers: 1, timeout: 17 * 60_000,
        use: { ...devices['Desktop Chrome'], ...failureArtifacts },
      },
    ]),
  ],
})
