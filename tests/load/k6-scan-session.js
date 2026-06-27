// k6 Load Test — Session Scan (plan.md Week 12)
// Target: 300 concurrent student scans per minute without data loss.
// Run: k6 run --env BASE_URL=http://localhost:8443 k6-scan-session.js

import http from 'k6/http'
import { check, sleep } from 'k6'
import { Counter, Rate, Trend } from 'k6/metrics'

const scanSuccess  = new Counter('scan_success')
const scanFailure  = new Counter('scan_failure')
const scanErrorRate = new Rate('scan_error_rate')
const scanLatency  = new Trend('scan_latency_ms', true)

export const options = {
  scenarios: {
    // Ramp to 300 scans/min (5/s), hold for 5 min, ramp down.
    scan_load: {
      executor: 'ramping-arrival-rate',
      startRate: 0,
      timeUnit: '1s',
      preAllocatedVUs: 50,
      maxVUs: 200,
      stages: [
        { target: 5,  duration: '30s' },  // ramp up to 5/s (300/min)
        { target: 5,  duration: '5m'  },  // hold
        { target: 0,  duration: '30s' },  // ramp down
      ],
    },
  },
  thresholds: {
    // plan.md NFR: 300 concurrent scans/min without data loss.
    scan_error_rate:   ['rate<0.01'],      // <1% error rate
    scan_latency_ms:   ['p(95)<2000'],     // 95th percentile < 2s
    http_req_duration: ['p(99)<5000'],     // 99th percentile < 5s
  },
}

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8443'

// Pre-generated test tokens (populate via setup() in real run).
// For local testing, set COORDINATOR_TOKEN env var.
const COORDINATOR_TOKEN = __ENV.COORDINATOR_TOKEN || 'test-token'
const SESSION_ID        = __ENV.SESSION_ID        || 'test-session-id'

export function setup() {
  // Authenticate as coordinator and return token for use in default().
  const loginRes = http.post(
    `${BASE_URL}/api/v1/auth/login`,
    JSON.stringify({
      email:     __ENV.COORDINATOR_EMAIL    || 'coordinator@alpha.edu',
      password:  __ENV.COORDINATOR_PASSWORD || 'Test1234!',
      tenant_id: __ENV.TENANT_ID            || 'a0000000-0000-0000-0000-000000000001',
    }),
    { headers: { 'Content-Type': 'application/json' } },
  )

  if (loginRes.status !== 200) {
    console.error('Setup login failed:', loginRes.body)
    return { token: '' }
  }
  const body = JSON.parse(loginRes.body)
  return { token: body.access_token }
}

export default function (data) {
  const token = data.token || COORDINATOR_TOKEN
  const start = Date.now()

  // Simulate a student QR scan by hitting the sync init endpoint
  // (real QR scan validation happens offline in the PWA; this tests the API tier).
  const res = http.get(
    `${BASE_URL}/api/v1/manifest/daily`,
    {
      headers: {
        Authorization:        `Bearer ${token}`,
        'X-Device-Fingerprint': `fp-${__VU}-${__ITER}`,
        'X-Correlation-ID':     `k6-${__VU}-${__ITER}`,
      },
    },
  )

  const latency = Date.now() - start
  scanLatency.add(latency)

  const ok = check(res, {
    'status 200 or 304': r => r.status === 200 || r.status === 304,
    'has manifest_version': r => {
      try { return JSON.parse(r.body).manifest_version !== undefined } catch { return false }
    },
  })

  if (ok) {
    scanSuccess.add(1)
    scanErrorRate.add(false)
  } else {
    scanFailure.add(1)
    scanErrorRate.add(true)
  }

  sleep(0.1) // brief pause between iterations per VU
}

export function teardown(data) {
  // Logout to blacklist the token.
  if (data.token) {
    http.post(`${BASE_URL}/api/v1/auth/logout`, null, {
      headers: { Authorization: `Bearer ${data.token}` },
    })
  }
}
