import { defineConfig, devices } from '@playwright/test';

// Screenshots depend on fonts and rendering, so baselines are only valid from the pinned Docker image.
if (!process.env.PLAYWRIGHT_IN_DOCKER) {
  throw new Error('Run e2e through Docker: `make e2e` or `make e2e-update` (set PLAYWRIGHT_IN_DOCKER=1 to override).');
}

export default defineConfig({
  testDir: './tests',
  snapshotPathTemplate: '{testDir}/__snapshots__/{testFilePath}/{projectName}/{arg}{ext}',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: [['list'], ['html', { open: 'never' }]],
  expect: {
    toHaveScreenshot: { animations: 'disabled', caret: 'hide', maxDiffPixelRatio: 0.01 },
  },
  use: {
    baseURL: process.env.BASE_URL ?? 'http://localhost:8080',
    trace: 'retain-on-failure',
  },
  projects: [
    { name: 'desktop', use: { ...devices['Desktop Chrome'] } },
    { name: 'mobile', use: { ...devices['Pixel 7'] } },
  ],
});
