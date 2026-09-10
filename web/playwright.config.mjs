import {defineConfig} from '@playwright/test'
import os from 'node:os'
import path from 'node:path'

export default defineConfig({
  testDir: './e2e',
  globalSetup: './e2e/setup.mjs',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 45000,
  expect: {timeout: 8000},
  reporter: 'list',
  outputDir: process.env.MSS_E2E_OUTPUT || path.join(os.tmpdir(), 'mihomo-smart-selector-e2e-results'),
  use: {
    browserName: 'chromium', viewport: {width: 1440, height: 1000},
    locale: 'zh-CN', timezoneId: 'Asia/Shanghai',
    screenshot: 'only-on-failure', trace: 'retain-on-failure',
  },
})
