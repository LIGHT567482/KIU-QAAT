import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './specs',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [['html'], ['list']],

  use: {
    // HTTPS, not HTTP. Every one of these is served through Caddy, which terminates
    // TLS — so the old http:// defaults connected to a port that speaks TLS and got
    // nowhere. Each spec then failed on a missing selector, which reads like a UI
    // regression rather than "the page never loaded".
    baseURL:       process.env.BASE_URL       ?? 'https://localhost:8443',
    dashboardURL:  process.env.DASHBOARD_URL  ?? 'https://localhost:3001',
    studentURL:    process.env.STUDENT_URL    ?? 'https://localhost:3003',
    // Caddy uses a self-signed certificate locally (infra/certs, CN=qaat-local).
    // Without this the browser refuses every navigation before a test can start.
    ignoreHTTPSErrors: true,
    trace:         'on-first-retry',
    screenshot:    'only-on-failure',
  } as Parameters<typeof defineConfig>[0]['use'] & Record<string, string>,

  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'mobile-android', use: { ...devices['Pixel 5'] } },
  ],
})
