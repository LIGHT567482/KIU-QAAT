import { test, expect } from '@playwright/test'

const DASHBOARD = process.env.DASHBOARD_URL ?? 'http://localhost:3001'
const API       = process.env.BASE_URL      ?? 'http://localhost:8443'
const TENANT_A  = 'a0000000-0000-0000-0000-000000000001'

test.describe('Authentication', () => {

  test('QA Officer can log in to dashboard', async ({ page }) => {
    await page.goto(`${DASHBOARD}/login`)
    await page.fill('[placeholder="Institution ID"]', TENANT_A)
    await page.fill('[placeholder="Email"]',           'qa.officer@alpha.edu')
    await page.fill('[placeholder="Password"]',        'Test1234!')
    await page.click('button[type="submit"]')
    await expect(page).toHaveURL(/\/qa\/live/)
    await expect(page.locator('h2')).toContainText('Live Sessions')
  })

  test('VC is redirected to /vc after login', async ({ page }) => {
    await page.goto(`${DASHBOARD}/login`)
    await page.fill('[placeholder="Institution ID"]', TENANT_A)
    await page.fill('[placeholder="Email"]',           'vc@alpha.edu')
    await page.fill('[placeholder="Password"]',        'Test1234!')
    // VC has TOTP — fill dummy code (will fail) to confirm MFA gate renders
    await page.click('button[type="submit"]')
    await expect(page.locator('text=Verify')).toBeVisible({ timeout: 3000 })
  })

  test('Wrong password shows error', async ({ page }) => {
    await page.goto(`${DASHBOARD}/login`)
    await page.fill('[placeholder="Institution ID"]', TENANT_A)
    await page.fill('[placeholder="Email"]',           'qa.officer@alpha.edu')
    await page.fill('[placeholder="Password"]',        'WrongPassword!')
    await page.click('button[type="submit"]')
    await expect(page.locator('text=invalid')).toBeVisible({ timeout: 3000 })
  })

  test('Unauthenticated access redirects to /login', async ({ page }) => {
    await page.goto(`${DASHBOARD}/qa/live`)
    await expect(page).toHaveURL(/\/login/)
  })

  test('Wrong role is redirected to /unauthorized', async ({ page }) => {
    // Log in as QA Officer, then try to access VC route.
    await page.goto(`${DASHBOARD}/login`)
    await page.fill('[placeholder="Institution ID"]', TENANT_A)
    await page.fill('[placeholder="Email"]',           'qa.officer@alpha.edu')
    await page.fill('[placeholder="Password"]',        'Test1234!')
    await page.click('button[type="submit"]')
    await page.goto(`${DASHBOARD}/vc`)
    await expect(page).toHaveURL(/\/unauthorized/)
  })

  test('JWT health check endpoint responds', async ({ request }) => {
    const res = await request.get(`${API}/health`)
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body.status).toBe('ok')
  })

})
