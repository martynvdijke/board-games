import { defineConfig, devices } from '@playwright/test';

const db = '.e2e/shelf.db';
const port = process.env.E2E_PORT ?? '6270';
const fakeBGGPort = process.env.E2E_FAKE_BGG_PORT ?? '6291';
const baseURL = `http://127.0.0.1:${port}`;

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : [['list']],
  timeout: 30_000,
  expect: { timeout: 8_000 },
  use: {
    baseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    reducedMotion: 'reduce',
    ...devices['Desktop Chrome'],
  },
  webServer: [
    {
      command: `go run ./cmd/fakebgg -addr :${fakeBGGPort}`,
      url: `http://127.0.0.1:${fakeBGGPort}/`,
      reuseExistingServer: false,
      timeout: 90_000,
    },
    {
      command: [
        'mkdir -p .e2e',
        `rm -f ${db}`,
        `DATABASE_PATH=${db} go run ./cmd/seed`,
        `DATABASE_PATH=${db} DEV_FAKE_AUTH=true BGG_BASE_URL=http://127.0.0.1:${fakeBGGPort}/xmlapi2 PORT=${port} BASE_URL=${baseURL} go run ./cmd/server`,
      ].join(' && '),
      url: `${baseURL}/healthz`,
      reuseExistingServer: false,
      timeout: 120_000,
    },
  ],
});
