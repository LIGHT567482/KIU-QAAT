# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: api.spec.ts >> API — Auth endpoints >> GET /manifest/daily returns 403 for QA Officer (wrong role)
- Location: specs/api.spec.ts:46:7

# Error details

```
Error: expect(received).toBe(expected) // Object.is equality

Expected: 403
Received: 401
```

# Test source

```ts
  1   | import { test, expect } from '@playwright/test'
  2   | 
  3   | const API      = process.env.BASE_URL ?? 'http://localhost:8443'
  4   | const TENANT_A = 'a0000000-0000-0000-0000-000000000001'
  5   | 
  6   | // Helper: obtain a JWT for API tests.
  7   | async function getToken(request: Parameters<typeof test.beforeEach>[0]['request'], role: 'QA_OFFICER' | 'COORDINATOR') {
  8   |   const emailMap = {
  9   |     QA_OFFICER:  'qa.officer@alpha.edu',
  10  |     COORDINATOR: 'coordinator@alpha.edu',
  11  |   }
  12  |   const res = await request.post(`${API}/api/v1/auth/login`, {
  13  |     data: { email: emailMap[role], password: 'Test1234!', tenant_id: TENANT_A },
  14  |   })
  15  |   if (!res.ok()) return ''
  16  |   return (await res.json()).access_token as string
  17  | }
  18  | 
  19  | test.describe('API — Auth endpoints', () => {
  20  | 
  21  |   test('POST /auth/login returns JWT for valid credentials', async ({ request }) => {
  22  |     const res = await request.post(`${API}/api/v1/auth/login`, {
  23  |       data: { email: 'qa.officer@alpha.edu', password: 'Test1234!', tenant_id: TENANT_A },
  24  |     })
  25  |     expect(res.status()).toBe(200)
  26  |     const body = await res.json()
  27  |     expect(body.access_token).toBeTruthy()
  28  |     expect(body.role).toBe('QA_OFFICER')
  29  |   })
  30  | 
  31  |   test('POST /auth/login returns 401 for wrong password', async ({ request }) => {
  32  |     const res = await request.post(`${API}/api/v1/auth/login`, {
  33  |       data: { email: 'qa.officer@alpha.edu', password: 'WrongPW!', tenant_id: TENANT_A },
  34  |     })
  35  |     expect(res.status()).toBe(401)
  36  |     expect((await res.json()).error).toBe('INVALID_CREDENTIALS')
  37  |   })
  38  | 
  39  |   test('GET /manifest/daily returns 401 without token', async ({ request }) => {
  40  |     const res = await request.get(`${API}/api/v1/manifest/daily`, {
  41  |       headers: { 'X-Device-Fingerprint': 'fp-test' },
  42  |     })
  43  |     expect(res.status()).toBe(401)
  44  |   })
  45  | 
  46  |   test('GET /manifest/daily returns 403 for QA Officer (wrong role)', async ({ request }) => {
  47  |     const token = await getToken(request, 'QA_OFFICER')
  48  |     const res = await request.get(`${API}/api/v1/manifest/daily`, {
  49  |       headers: {
  50  |         Authorization: `Bearer ${token}`,
  51  |         'X-Device-Fingerprint': 'fp-test',
  52  |       },
  53  |     })
> 54  |     expect(res.status()).toBe(403)
      |                          ^ Error: expect(received).toBe(expected) // Object.is equality
  55  |   })
  56  | 
  57  |   test('GET /manifest/daily returns 200 for Coordinator', async ({ request }) => {
  58  |     const token = await getToken(request, 'COORDINATOR')
  59  |     if (!token) { test.skip(); return }
  60  |     const res = await request.get(`${API}/api/v1/manifest/daily`, {
  61  |       headers: {
  62  |         Authorization: `Bearer ${token}`,
  63  |         'X-Device-Fingerprint': 'fp-test',
  64  |       },
  65  |     })
  66  |     expect([200, 404]).toContain(res.status()) // 404 if no sessions today — that's fine
  67  |   })
  68  | 
  69  | })
  70  | 
  71  | test.describe('API — Security headers', () => {
  72  | 
  73  |   test('Responses include Strict-Transport-Security', async ({ request }) => {
  74  |     const res = await request.get(`${API}/health`)
  75  |     const hsts = res.headers()['strict-transport-security']
  76  |     expect(hsts).toBeTruthy()
  77  |     expect(hsts).toContain('max-age=63072000')
  78  |   })
  79  | 
  80  |   test('Responses include X-Frame-Options: DENY', async ({ request }) => {
  81  |     const res = await request.get(`${API}/health`)
  82  |     expect(res.headers()['x-frame-options']).toBe('DENY')
  83  |   })
  84  | 
  85  |   test('Responses include X-Content-Type-Options: nosniff', async ({ request }) => {
  86  |     const res = await request.get(`${API}/health`)
  87  |     expect(res.headers()['x-content-type-options']).toBe('nosniff')
  88  |   })
  89  | 
  90  | })
  91  | 
  92  | test.describe('API — Cross-tenant isolation', () => {
  93  | 
  94  |   test('Coordinator from Tenant A cannot access Tenant B data via RBAC', async ({ request }) => {
  95  |     // Tenant A coordinator token should only see their own manifest.
  96  |     const token = await getToken(request, 'COORDINATOR')
  97  |     if (!token) { test.skip(); return }
  98  | 
  99  |     // Try to hit an eligibility endpoint for a Tenant B student ID.
  100 |     const res = await request.get(`${API}/api/v1/eligibility/REG-B-9999`, {
  101 |       headers: { Authorization: `Bearer ${token}` },
  102 |     })
  103 |     // Either 403 or 404 — both are acceptable (tenant isolation).
  104 |     expect([403, 404]).toContain(res.status())
  105 |   })
  106 | 
  107 | })
  108 | 
```