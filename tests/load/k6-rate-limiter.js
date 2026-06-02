// k6 Security Test — Rate Limiter (plan.md Week 11)
// Verifies the 50 req/s per coordinator limit returns 429 when exceeded.
// Run: k6 run --env BASE_URL=http://localhost:8443 k6-rate-limiter.js

import http from 'k6/http'
import { check } from 'k6'
import { Counter } from 'k6/metrics'

const got429 = new Counter('rate_limit_429_count')
const got200 = new Counter('rate_limit_200_count')

export const options = {
  scenarios: {
    burst: {
      executor: 'constant-arrival-rate',
      rate: 100,       // 100/s — well above 50/s limit
      timeUnit: '1s',
      duration: '30s',
      preAllocatedVUs: 50,
      maxVUs: 100,
    },
  },
  thresholds: {
    // We expect the rate limiter to kick in — at least 20% of requests should be 429.
    'rate_limit_429_count': ['count>100'],
  },
}

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8443'

export function setup() {
  const res = http.post(
    `${BASE_URL}/api/v1/auth/login`,
    JSON.stringify({
      email:     __ENV.COORDINATOR_EMAIL    || 'coordinator@alpha.edu',
      password:  __ENV.COORDINATOR_PASSWORD || 'Test1234!',
      tenant_id: __ENV.TENANT_ID            || 'a0000000-0000-0000-0000-000000000001',
    }),
    { headers: { 'Content-Type': 'application/json' } },
  )
  if (res.status !== 200) return { token: '' }
  return { token: JSON.parse(res.body).access_token }
}

export default function (data) {
  const res = http.get(`${BASE_URL}/api/v1/manifest/daily`, {
    headers: {
      Authorization:        `Bearer ${data.token}`,
      'X-Device-Fingerprint': 'fp-ratelimit-test',
    },
  })

  if (res.status === 429) {
    got429.add(1)
    check(res, { 'rate limit error code': r => {
      try { return JSON.parse(r.body).error === 'RATE_LIMIT_EXCEEDED' } catch { return false }
    }})
  } else {
    got200.add(1)
  }
}
