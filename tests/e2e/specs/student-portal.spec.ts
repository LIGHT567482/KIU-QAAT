import { test, expect } from '@playwright/test'

const STUDENT_URL = process.env.STUDENT_URL ?? 'https://localhost:3003'

/**
 * The student portal is PASSWORDLESS. A student types their registration number
 * and sees their own attendance and eligibility — there is no account, no email
 * and no password.
 *
 * These specs previously drove an "Institution ID" + "Student email" + "Password"
 * form. The institution ID went in c5ed850 (single-institution build) and the
 * email/password login went with the QR subsystem in migration 063, so every one
 * of them had been failing on a selector that no longer exists rather than on
 * anything about the portal.
 */
test.describe('Student Portal', () => {

  test('Registration-number lookup is the only way in', async ({ page }) => {
    await page.goto(STUDENT_URL)
    await expect(page.locator('[placeholder="Enter your registration number"]')).toBeVisible()
    // The retired login fields must not come back: a password box here would mean
    // students had been given credentials they were never issued.
    await expect(page.locator('[placeholder="Institution ID"]')).toHaveCount(0)
    await expect(page.locator('[type="password"]')).toHaveCount(0)
  })

  test('Submit is disabled until a registration number is typed', async ({ page }) => {
    await page.goto(STUDENT_URL)
    const submit = page.locator('button[type="submit"]')
    await expect(submit).toBeDisabled()
    await page.fill('[placeholder="Enter your registration number"]', 'KIU/2024/0001')
    await expect(submit).toBeEnabled()
  })

  test('An unknown registration number is refused, not guessed at', async ({ page }) => {
    await page.goto(STUDENT_URL)
    await page.fill('[placeholder="Enter your registration number"]', 'NOT-A-REAL-REGNO')
    await page.click('button[type="submit"]')
    // Whatever the wording, the student must be told — never shown a blank page
    // that reads as "you have no attendance".
    // The gateway answers 404 "no student with that registration number at this
    // institution", and the portal surfaces that message verbatim. Matching on
    // "no student" as well as the generic wordings keeps this honest about what
    // the user is actually shown.
    await expect(
      page.locator('text=/no student|not found|no record|invalid|failed|could not/i').first(),
    ).toBeVisible({ timeout: 15000 })
  })

})
