import { test, expect } from '@playwright/test'

const STUDENT_URL = process.env.STUDENT_URL ?? 'http://localhost:3003'
const TENANT_A    = 'a0000000-0000-0000-0000-000000000001'

test.describe('Student Portal', () => {

  test('Login page renders', async ({ page }) => {
    await page.goto(STUDENT_URL)
    await expect(page.locator('h2')).toContainText('QAAT Student Portal')
    await expect(page.locator('[placeholder="Institution ID"]')).toBeVisible()
    await expect(page.locator('[placeholder="Student email"]')).toBeVisible()
  })

  test('Wrong credentials shows error', async ({ page }) => {
    await page.goto(STUDENT_URL)
    await page.fill('[placeholder="Institution ID"]', TENANT_A)
    await page.fill('[placeholder="Student email"]',   'nobody@alpha.edu')
    await page.fill('[placeholder="Password"]',         'WrongPW!')
    await page.click('button[type="submit"]')
    await expect(page.locator('text=invalid').or(page.locator('text=failed'))).toBeVisible({ timeout: 5000 })
  })

  test('Empty form does not submit', async ({ page }) => {
    await page.goto(STUDENT_URL)
    await page.click('button[type="submit"]')
    // Should remain on login page.
    await expect(page.locator('h2')).toContainText('QAAT Student Portal')
  })

})
