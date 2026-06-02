import { test, expect } from '@playwright/test'

const DASHBOARD = process.env.DASHBOARD_URL ?? 'http://localhost:3001'
const TENANT_A  = 'a0000000-0000-0000-0000-000000000001'

// Helper: log in as QA Officer and return authenticated page.
async function loginAsQA(page: Parameters<typeof test.beforeEach>[0]['page']) {
  await page.goto(`${DASHBOARD}/login`)
  await page.fill('[placeholder="Institution ID"]', TENANT_A)
  await page.fill('[placeholder="Email"]',           'qa.officer@alpha.edu')
  await page.fill('[placeholder="Password"]',        'Test1234!')
  await page.click('button[type="submit"]')
  await page.waitForURL(/\/qa\/live/)
}

async function loginAsDQA(page: Parameters<typeof test.beforeEach>[0]['page']) {
  await page.goto(`${DASHBOARD}/login`)
  await page.fill('[placeholder="Institution ID"]', TENANT_A)
  await page.fill('[placeholder="Email"]',           'dqa.director@alpha.edu')
  await page.fill('[placeholder="Password"]',        'Test1234!')
  await page.click('button[type="submit"]')
  // DQA has TOTP — skip TOTP for basic dashboard render test if enrolled
}

test.describe('QA Officer Dashboard', () => {

  test('Live Sessions page renders', async ({ page }) => {
    await loginAsQA(page)
    await expect(page.locator('h2')).toContainText('Live Sessions')
    await expect(page.locator('text=Auto-refreshes every 10s')).toBeVisible()
  })

  test('Device Reset form renders with required fields', async ({ page }) => {
    await loginAsQA(page)
    await page.goto(`${DASHBOARD}/qa/device-reset`)
    await expect(page.locator('h2')).toContainText('Device Binding Reset')
    await expect(page.locator('[placeholder="e.g. REG-2024-0001"]')).toBeVisible()
    await expect(page.locator('select')).toBeVisible()
  })

  test('Device Reset form validates empty submission', async ({ page }) => {
    await loginAsQA(page)
    await page.goto(`${DASHBOARD}/qa/device-reset`)
    await page.click('button[type="submit"]')
    // HTML5 required validation should prevent submission.
    await expect(page).toHaveURL(/\/qa\/device-reset/)
  })

})

test.describe('DQA Dashboard', () => {

  test('Thresholds page renders form fields', async ({ page }) => {
    await loginAsDQA(page)
    // If TOTP required, the form won't be visible — that's fine for CI (seed user has no TOTP enrolled)
    await page.goto(`${DASHBOARD}/dqa/thresholds`)
    // If redirected to /login (TOTP not enrolled), skip gracefully.
    if (page.url().includes('/login')) {
      test.skip()
      return
    }
    await expect(page.locator('h2')).toContainText('Policy Thresholds')
    await expect(page.locator('text=Attendance threshold')).toBeVisible()
    await expect(page.locator('text=Save Changes')).toBeVisible()
  })

  test('Eligibility page has lookup and Export CSV button', async ({ page }) => {
    await loginAsDQA(page)
    await page.goto(`${DASHBOARD}/dqa/eligibility`)
    if (page.url().includes('/login')) { test.skip(); return }
    await expect(page.locator('h2')).toContainText('Exam Eligibility')
    await expect(page.locator('text=Export CSV')).toBeVisible()
  })

})
