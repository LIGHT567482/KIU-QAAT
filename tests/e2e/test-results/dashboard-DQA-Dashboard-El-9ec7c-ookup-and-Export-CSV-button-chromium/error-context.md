# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: dashboard.spec.ts >> DQA Dashboard >> Eligibility page has lookup and Export CSV button
- Location: specs/dashboard.spec.ts:67:7

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
  3  | const DASHBOARD = process.env.DASHBOARD_URL ?? 'http://localhost:3001'
  4  | const TENANT_A  = 'a0000000-0000-0000-0000-000000000001'
  5  | 
  6  | // Helper: log in as QA Officer and return authenticated page.
  7  | async function loginAsQA(page: Parameters<typeof test.beforeEach>[0]['page']) {
  8  |   await page.goto(`${DASHBOARD}/login`)
  9  |   await page.fill('[placeholder="Institution ID"]', TENANT_A)
  10 |   await page.fill('[placeholder="Email"]',           'qa.officer@alpha.edu')
  11 |   await page.fill('[placeholder="Password"]',        'Test1234!')
  12 |   await page.click('button[type="submit"]')
  13 |   await page.waitForURL(/\/qa\/live/)
  14 | }
  15 | 
  16 | async function loginAsDQA(page: Parameters<typeof test.beforeEach>[0]['page']) {
  17 |   await page.goto(`${DASHBOARD}/login`)
> 18 |   await page.fill('[placeholder="Institution ID"]', TENANT_A)
     |              ^ Error: page.fill: Test timeout of 30000ms exceeded.
  19 |   await page.fill('[placeholder="Email"]',           'dqa.director@alpha.edu')
  20 |   await page.fill('[placeholder="Password"]',        'Test1234!')
  21 |   await page.click('button[type="submit"]')
  22 |   // DQA has TOTP — skip TOTP for basic dashboard render test if enrolled
  23 | }
  24 | 
  25 | test.describe('QA Officer Dashboard', () => {
  26 | 
  27 |   test('Live Sessions page renders', async ({ page }) => {
  28 |     await loginAsQA(page)
  29 |     await expect(page.locator('h2')).toContainText('Live Sessions')
  30 |     await expect(page.locator('text=Auto-refreshes every 10s')).toBeVisible()
  31 |   })
  32 | 
  33 |   test('Device Reset form renders with required fields', async ({ page }) => {
  34 |     await loginAsQA(page)
  35 |     await page.goto(`${DASHBOARD}/qa/device-reset`)
  36 |     await expect(page.locator('h2')).toContainText('Device Binding Reset')
  37 |     await expect(page.locator('[placeholder="e.g. REG-2024-0001"]')).toBeVisible()
  38 |     await expect(page.locator('select')).toBeVisible()
  39 |   })
  40 | 
  41 |   test('Device Reset form validates empty submission', async ({ page }) => {
  42 |     await loginAsQA(page)
  43 |     await page.goto(`${DASHBOARD}/qa/device-reset`)
  44 |     await page.click('button[type="submit"]')
  45 |     // HTML5 required validation should prevent submission.
  46 |     await expect(page).toHaveURL(/\/qa\/device-reset/)
  47 |   })
  48 | 
  49 | })
  50 | 
  51 | test.describe('DQA Dashboard', () => {
  52 | 
  53 |   test('Thresholds page renders form fields', async ({ page }) => {
  54 |     await loginAsDQA(page)
  55 |     // If TOTP required, the form won't be visible — that's fine for CI (seed user has no TOTP enrolled)
  56 |     await page.goto(`${DASHBOARD}/dqa/thresholds`)
  57 |     // If redirected to /login (TOTP not enrolled), skip gracefully.
  58 |     if (page.url().includes('/login')) {
  59 |       test.skip()
  60 |       return
  61 |     }
  62 |     await expect(page.locator('h2')).toContainText('Policy Thresholds')
  63 |     await expect(page.locator('text=Attendance threshold')).toBeVisible()
  64 |     await expect(page.locator('text=Save Changes')).toBeVisible()
  65 |   })
  66 | 
  67 |   test('Eligibility page has lookup and Export CSV button', async ({ page }) => {
  68 |     await loginAsDQA(page)
  69 |     await page.goto(`${DASHBOARD}/dqa/eligibility`)
  70 |     if (page.url().includes('/login')) { test.skip(); return }
  71 |     await expect(page.locator('h2')).toContainText('Exam Eligibility')
  72 |     await expect(page.locator('text=Export CSV')).toBeVisible()
  73 |   })
  74 | 
  75 | })
  76 | 
```