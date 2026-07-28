# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: auth.spec.ts >> Authentication >> VC is redirected to /vc after login
- Location: specs/auth.spec.ts:19:7

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
  4  | const API       = process.env.BASE_URL      ?? 'http://localhost:8443'
  5  | const TENANT_A  = 'a0000000-0000-0000-0000-000000000001'
  6  | 
  7  | test.describe('Authentication', () => {
  8  | 
  9  |   test('QA Officer can log in to dashboard', async ({ page }) => {
  10 |     await page.goto(`${DASHBOARD}/login`)
  11 |     await page.fill('[placeholder="Institution ID"]', TENANT_A)
  12 |     await page.fill('[placeholder="Email"]',           'qa.officer@alpha.edu')
  13 |     await page.fill('[placeholder="Password"]',        'Test1234!')
  14 |     await page.click('button[type="submit"]')
  15 |     await expect(page).toHaveURL(/\/qa\/live/)
  16 |     await expect(page.locator('h2')).toContainText('Live Sessions')
  17 |   })
  18 | 
  19 |   test('VC is redirected to /vc after login', async ({ page }) => {
  20 |     await page.goto(`${DASHBOARD}/login`)
> 21 |     await page.fill('[placeholder="Institution ID"]', TENANT_A)
     |                ^ Error: page.fill: Test timeout of 30000ms exceeded.
  22 |     await page.fill('[placeholder="Email"]',           'vc@alpha.edu')
  23 |     await page.fill('[placeholder="Password"]',        'Test1234!')
  24 |     // VC has TOTP — fill dummy code (will fail) to confirm MFA gate renders
  25 |     await page.click('button[type="submit"]')
  26 |     await expect(page.locator('text=Verify')).toBeVisible({ timeout: 3000 })
  27 |   })
  28 | 
  29 |   test('Wrong password shows error', async ({ page }) => {
  30 |     await page.goto(`${DASHBOARD}/login`)
  31 |     await page.fill('[placeholder="Institution ID"]', TENANT_A)
  32 |     await page.fill('[placeholder="Email"]',           'qa.officer@alpha.edu')
  33 |     await page.fill('[placeholder="Password"]',        'WrongPassword!')
  34 |     await page.click('button[type="submit"]')
  35 |     await expect(page.locator('text=invalid')).toBeVisible({ timeout: 3000 })
  36 |   })
  37 | 
  38 |   test('Unauthenticated access redirects to /login', async ({ page }) => {
  39 |     await page.goto(`${DASHBOARD}/qa/live`)
  40 |     await expect(page).toHaveURL(/\/login/)
  41 |   })
  42 | 
  43 |   test('Wrong role is redirected to /unauthorized', async ({ page }) => {
  44 |     // Log in as QA Officer, then try to access VC route.
  45 |     await page.goto(`${DASHBOARD}/login`)
  46 |     await page.fill('[placeholder="Institution ID"]', TENANT_A)
  47 |     await page.fill('[placeholder="Email"]',           'qa.officer@alpha.edu')
  48 |     await page.fill('[placeholder="Password"]',        'Test1234!')
  49 |     await page.click('button[type="submit"]')
  50 |     await page.goto(`${DASHBOARD}/vc`)
  51 |     await expect(page).toHaveURL(/\/unauthorized/)
  52 |   })
  53 | 
  54 |   test('JWT health check endpoint responds', async ({ request }) => {
  55 |     const res = await request.get(`${API}/health`)
  56 |     expect(res.status()).toBe(200)
  57 |     const body = await res.json()
  58 |     expect(body.status).toBe('ok')
  59 |   })
  60 | 
  61 | })
  62 | 
```