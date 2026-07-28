# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: student-portal.spec.ts >> Student Portal >> Wrong credentials shows error
- Location: specs/student-portal.spec.ts:15:7

# Error details

```
Test timeout of 30000ms exceeded.
```

```
Error: page.fill: Test timeout of 30000ms exceeded.
Call log:
  - waiting for locator('[placeholder="Institution ID"]')

```

# Page snapshot

```yaml
- generic [active] [ref=e1]: Client sent an HTTP request to an HTTPS server.
```

# Test source

```ts
  1  | import { test, expect } from '@playwright/test'
  2  | 
  3  | const STUDENT_URL = process.env.STUDENT_URL ?? 'http://localhost:3003'
  4  | const TENANT_A    = 'a0000000-0000-0000-0000-000000000001'
  5  | 
  6  | test.describe('Student Portal', () => {
  7  | 
  8  |   test('Login page renders', async ({ page }) => {
  9  |     await page.goto(STUDENT_URL)
  10 |     await expect(page.locator('h2')).toContainText('QAAT Student Portal')
  11 |     await expect(page.locator('[placeholder="Institution ID"]')).toBeVisible()
  12 |     await expect(page.locator('[placeholder="Student email"]')).toBeVisible()
  13 |   })
  14 | 
  15 |   test('Wrong credentials shows error', async ({ page }) => {
  16 |     await page.goto(STUDENT_URL)
> 17 |     await page.fill('[placeholder="Institution ID"]', TENANT_A)
     |                ^ Error: page.fill: Test timeout of 30000ms exceeded.
  18 |     await page.fill('[placeholder="Student email"]',   'nobody@alpha.edu')
  19 |     await page.fill('[placeholder="Password"]',         'WrongPW!')
  20 |     await page.click('button[type="submit"]')
  21 |     await expect(page.locator('text=invalid').or(page.locator('text=failed'))).toBeVisible({ timeout: 5000 })
  22 |   })
  23 | 
  24 |   test('Empty form does not submit', async ({ page }) => {
  25 |     await page.goto(STUDENT_URL)
  26 |     await page.click('button[type="submit"]')
  27 |     // Should remain on login page.
  28 |     await expect(page.locator('h2')).toContainText('QAAT Student Portal')
  29 |   })
  30 | 
  31 | })
  32 | 
```