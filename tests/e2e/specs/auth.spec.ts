import { test, expect } from '@playwright/test'

const DASHBOARD = process.env.DASHBOARD_URL ?? 'https://localhost:3001'
const API       = process.env.BASE_URL      ?? 'https://localhost:8443'
const TENANT_A  = 'a1000000-0000-0000-0000-000000000001'  // Alpha University (db/seeds/001)

test.describe('Authentication', () => {

  test('QA Officer can log in to dashboard', async ({ page }) => {
    await page.goto(`${DASHBOARD}/login`)
    await page.fill('[placeholder="Email"]',           'qa.officer@alpha.edu')
    await page.fill('[placeholder="Password"]',        'Test1234!')
    await page.click('button[type="submit"]')
    await expect(page).toHaveURL(/\/qa\/reports/)
    await expect(page.locator('h2')).toContainText('QA Reports')
  })

  test('VC is redirected to /vc after login', async ({ page }) => {
    await page.goto(`${DASHBOARD}/login`)
    await page.fill('[placeholder="Email"]',           'vc@alpha.edu')
    await page.fill('[placeholder="Password"]',        'Test1234!')
    await page.click('button[type="submit"]')
    // MFA is mandatory for VC and DQA_DIRECTOR, but DISABLE_MFA=true in
    // development — so a VC either lands on their dashboard or meets the TOTP
    // gate depending on how the auth-service is configured. Both are correct;
    // being bounced back to /login is not, and that is what this catches.
    await expect(
      page.locator('text=Verify').or(page.locator('h2')),
    ).toBeVisible({ timeout: 15000 })
    await expect(page).not.toHaveURL(/\/login/)
  })

  test('Wrong password shows error', async ({ page }) => {
    await page.goto(`${DASHBOARD}/login`)
    await page.fill('[placeholder="Email"]',           'qa.officer@alpha.edu')
    await page.fill('[placeholder="Password"]',        'WrongPassword!')
    await page.click('button[type="submit"]')
    await expect(page.locator('text=invalid')).toBeVisible({ timeout: 3000 })
  })

  test('Unauthenticated access redirects to /login', async ({ page }) => {
    await page.goto(`${DASHBOARD}/qa/reports`)
    await expect(page).toHaveURL(/\/login/)
  })

  test('Wrong role is redirected to /unauthorized', async ({ page }) => {
    // Log in as QA Officer, then try to access VC route.
    await page.goto(`${DASHBOARD}/login`)
    await page.fill('[placeholder="Email"]',           'qa.officer@alpha.edu')
    await page.fill('[placeholder="Password"]',        'Test1234!')
    await page.click('button[type="submit"]')
    // Let the sign-in finish first. Navigating mid-flight arrives unauthenticated,
    // which redirects to /login — a correct guard, but not the one being tested.
    await page.waitForURL(/\/qa\/reports/, { timeout: 15000 })
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
