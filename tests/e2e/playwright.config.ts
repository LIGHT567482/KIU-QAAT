import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './specs',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [['html'], ['list']],

  use: {
    baseURL:       process.env.BASE_URL       ?? 'http://localhost:8443',
    dashboardURL:  process.env.DASHBOARD_URL  ?? 'http://localhost:3001',
    pwaURL:        process.env.PWA_URL        ?? 'http://localhost:3000',
    studentURL:    process.env.STUDENT_URL    ?? 'http://localhost:3003',
    trace:         'on-first-retry',
    screenshot:    'only-on-failure',
  } as Parameters<typeof defineConfig>[0]['use'] & Record<string, string>,

  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'mobile-android', use: { ...devices['Pixel 5'] } },
  ],
})
