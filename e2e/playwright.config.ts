import { defineConfig } from '@playwright/test'
import path from 'node:path'

export default defineConfig({
  testDir: './cases',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  reporter: 'dot',
  // No reports or browser state are retained here. The case explicitly writes
  // only safe observations and screenshots to the run's evidence directory.
  outputDir: path.join(process.env.YAAM_E2E_PRIVATE_HOME ?? '/tmp/yaam-e2e-unused', 'playwright'),
  use: {
    browserName: 'chromium',
    headless: true,
    viewport: { width: 1280, height: 900 },
    actionTimeout: 10_000,
    navigationTimeout: 10_000,
    trace: 'off',
    video: 'off',
    screenshot: 'off',
  },
})
