import { test, expect } from '@playwright/test'

const API      = process.env.BASE_URL ?? 'http://localhost:8443'
const TENANT_A = 'a0000000-0000-0000-0000-000000000001'

// Helper: obtain a JWT for API tests.
async function getToken(request: Parameters<typeof test.beforeEach>[0]['request'], role: 'QA_OFFICER' | 'COORDINATOR') {
  const emailMap = {
    QA_OFFICER:  'qa.officer@alpha.edu',
    COORDINATOR: 'coordinator@alpha.edu',
  }
  const res = await request.post(`${API}/api/v1/auth/login`, {
    data: { email: emailMap[role], password: 'Test1234!', tenant_id: TENANT_A },
  })
  if (!res.ok()) return ''
  return (await res.json()).access_token as string
}

test.describe('API — Auth endpoints', () => {

  test('POST /auth/login returns JWT for valid credentials', async ({ request }) => {
    const res = await request.post(`${API}/api/v1/auth/login`, {
      data: { email: 'qa.officer@alpha.edu', password: 'Test1234!', tenant_id: TENANT_A },
    })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body.access_token).toBeTruthy()
    expect(body.role).toBe('QA_OFFICER')
  })

  test('POST /auth/login returns 401 for wrong password', async ({ request }) => {
    const res = await request.post(`${API}/api/v1/auth/login`, {
      data: { email: 'qa.officer@alpha.edu', password: 'WrongPW!', tenant_id: TENANT_A },
    })
    expect(res.status()).toBe(401)
    expect((await res.json()).error).toBe('INVALID_CREDENTIALS')
  })

  test('GET /manifest/daily returns 401 without token', async ({ request }) => {
    const res = await request.get(`${API}/api/v1/manifest/daily`, {
      headers: { 'X-Device-Fingerprint': 'fp-test' },
    })
    expect(res.status()).toBe(401)
  })

  test('GET /manifest/daily returns 403 for QA Officer (wrong role)', async ({ request }) => {
    const token = await getToken(request, 'QA_OFFICER')
    const res = await request.get(`${API}/api/v1/manifest/daily`, {
      headers: {
        Authorization: `Bearer ${token}`,
        'X-Device-Fingerprint': 'fp-test',
      },
    })
    expect(res.status()).toBe(403)
  })

  test('GET /manifest/daily returns 200 for Coordinator', async ({ request }) => {
    const token = await getToken(request, 'COORDINATOR')
    if (!token) { test.skip(); return }
    const res = await request.get(`${API}/api/v1/manifest/daily`, {
      headers: {
        Authorization: `Bearer ${token}`,
        'X-Device-Fingerprint': 'fp-test',
      },
    })
    expect([200, 404]).toContain(res.status()) // 404 if no sessions today — that's fine
  })

})

test.describe('API — Security headers', () => {

  test('Responses include Strict-Transport-Security', async ({ request }) => {
    const res = await request.get(`${API}/health`)
    const hsts = res.headers()['strict-transport-security']
    expect(hsts).toBeTruthy()
    expect(hsts).toContain('max-age=63072000')
  })

  test('Responses include X-Frame-Options: DENY', async ({ request }) => {
    const res = await request.get(`${API}/health`)
    expect(res.headers()['x-frame-options']).toBe('DENY')
  })

  test('Responses include X-Content-Type-Options: nosniff', async ({ request }) => {
    const res = await request.get(`${API}/health`)
    expect(res.headers()['x-content-type-options']).toBe('nosniff')
  })

})

test.describe('API — Cross-tenant isolation', () => {

  test('Coordinator from Tenant A cannot access Tenant B data via RBAC', async ({ request }) => {
    // Tenant A coordinator token should only see their own manifest.
    const token = await getToken(request, 'COORDINATOR')
    if (!token) { test.skip(); return }

    // Try to hit an eligibility endpoint for a Tenant B student ID.
    const res = await request.get(`${API}/api/v1/eligibility/REG-B-9999`, {
      headers: { Authorization: `Bearer ${token}` },
    })
    // Either 403 or 404 — both are acceptable (tenant isolation).
    expect([403, 404]).toContain(res.status())
  })

})
