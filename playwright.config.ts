import { defineConfig, devices } from '@playwright/test';

const db = '.e2e/shelf.db';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : [['list']],
  timeout: 30_000,
  expect: { timeout: 8_000 },
  use: {
    baseURL: 'http://127.0.0.1:6290',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    reducedMotion: 'reduce',
    ...devices['Desktop Chrome'],
  },
  webServer: [
    {
      command: 'go run ./cmd/fakebgg -addr :6291',
      url: 'http://127.0.0.1:6291/',
      reuseExistingServer: false,
      timeout: 90_000,
    },
    {
      command: [
        'mkdir -p .e2e',
        `rm -f ${db}`,
        `DATABASE_PATH=${db} go run ./cmd/seed`,
        `DATABASE_PATH=${db} DEV_FAKE_AUTH=true BGG_BASE_URL=http://127.0.0.1:6291/xmlapi2 PORT=6290 BASE_URL=http://127.0.0.1:6290 go run ./cmd/server`,
      ].join(' && '),
      url: 'http://127.0.0.1:6290/healthz',
      reuseExistingServer: false,
      timeout: 120_000,
    },
  ],
});
