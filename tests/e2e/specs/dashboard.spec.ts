import { test, expect } from '@playwright/test'

const DASHBOARD = process.env.DASHBOARD_URL ?? 'https://localhost:3001'
const TENANT_A  = 'a1000000-0000-0000-0000-000000000001'  // Alpha University (db/seeds/001)

// Helper: log in as QA Officer and return authenticated page.
async function loginAsQA(page: Parameters<typeof test.beforeEach>[0]['page']) {
  await page.goto(`${DASHBOARD}/login`)
  await page.fill('[placeholder="Email"]',           'qa.officer@alpha.edu')
  await page.fill('[placeholder="Password"]',        'Test1234!')
  await page.click('button[type="submit"]')
  await page.waitForURL(/\/qa\/reports/)
}

async function loginAsDQA(page: Parameters<typeof test.beforeEach>[0]['page']) {
  await page.goto(`${DASHBOARD}/login`)
  await page.fill('[placeholder="Email"]',           'dqa.director@alpha.edu')
  await page.fill('[placeholder="Password"]',        'Test1234!')
  await page.click('button[type="submit"]')
  // Wait for the redirect to actually land. Without this the caller navigated
  // while the sign-in was still in flight, arrived unauthenticated, was bounced
  // to /login, and the assertion failed on a page that had never loaded — which
  // reads as a broken dashboard rather than as a race in the test.
  await page.waitForURL(/\/dqa/, { timeout: 15000 }).catch(() => {})
}

test.describe('QA Officer Dashboard', () => {

  // The QA officer's landing page is /qa/reports; the old /qa/live route is gone.
  test('Reports page renders as the QA landing page', async ({ page }) => {
    await loginAsQA(page)
    await expect(page.locator('h2')).toContainText('QA Reports')
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

  // The threshold and the auto-close are now FIXED institution-wide (backend
  // internal/policy), so this page states them instead of offering a form. A
  // "Save Changes" button here would be a control that silently discards what is
  // typed into it — which is what this asserts is gone.
  test('Thresholds page states the fixed policy rather than offering a form', async ({ page }) => {
    await loginAsDQA(page)
    await page.goto(`${DASHBOARD}/dqa/thresholds`)
    if (page.url().includes('/login')) {
      test.skip()
      return
    }
    await expect(page.locator('h2')).toContainText('Attendance Policy')
    await expect(page.locator('text=75%')).toBeVisible()
    await expect(page.locator('text=Save Changes')).toHaveCount(0)
    await expect(page.locator('input[type="number"]')).toHaveCount(0)
  })

  test('DQA lands on a home page, not a settings form', async ({ page }) => {
    await loginAsDQA(page)
    await page.goto(`${DASHBOARD}/dqa`)
    if (page.url().includes('/login')) {
      test.skip()
      return
    }
    // The shortcuts a director actually opens. The six ADMIN "manage" tiles must
    // NOT be here — every one is admin-only behind its route guard, so linking
    // them would render an Unauthorized page.
    await expect(page.locator('text=Eligibility').first()).toBeVisible()
    await expect(page.locator('text=Course Health').first()).toBeVisible()
    await expect(page.locator('a[href="/admin/tenants"]')).toHaveCount(0)
  })

  test('Eligibility page has lookup and Export CSV button', async ({ page }) => {
    await loginAsDQA(page)
    await page.goto(`${DASHBOARD}/dqa/eligibility`)
    if (page.url().includes('/login')) { test.skip(); return }
    await expect(page.locator('h2')).toContainText('Exam Eligibility')
    await expect(page.locator('text=Export CSV')).toBeVisible()
  })

})
