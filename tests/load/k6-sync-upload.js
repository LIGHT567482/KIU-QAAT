// k6 Load Test — Sync Upload (plan.md Week 12)
// Target: 10,000 concurrent sync operations on API.
// Run: k6 run --env BASE_URL=http://localhost:8443 k6-sync-upload.js

import http from 'k6/http'
import { check, sleep } from 'k6'
import { Counter, Rate, Trend } from 'k6/metrics'

const syncSuccess  = new Counter('sync_success')
const syncFailure  = new Counter('sync_failure')
const syncErrorRate = new Rate('sync_error_rate')
const syncLatency  = new Trend('sync_latency_ms', true)

export const options = {
  scenarios: {
    sync_load: {
      executor: 'constant-arrival-rate',
      rate: 167,          // ~10,000 ops/min
      timeUnit: '1s',
      duration: '5m',
      preAllocatedVUs: 200,
      maxVUs: 500,
    },
  },
  thresholds: {
    sync_error_rate: ['rate<0.005'],   // <0.5% failures
    sync_latency_ms: ['p(95)<3000'],   // 95th percentile < 3s
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
  const token = data.token
  if (!token) { sleep(1); return }

  const start = Date.now()

  // Phase 1: Init upload.
  const initRes = http.post(
    `${BASE_URL}/api/v1/sync/init`,
    JSON.stringify({
      coordinator_id:   `coord-${__VU}`,
      session_ids:      [`session-${__VU}-${__ITER}`],
      total_chunks:     1,
      package_checksum: `checksum-${__VU}-${__ITER}`,
    }),
    { headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` } },
  )

  if (initRes.status !== 200) {
    syncFailure.add(1)
    syncErrorRate.add(true)
    return
  }

  const uploadId = JSON.parse(initRes.body).upload_id

  // Phase 2: Upload single chunk.
  const chunkPayload = new Uint8Array(1024).fill(0xAA)  // 1 KiB dummy encrypted chunk
  const chunkRes = http.post(
    `${BASE_URL}/api/v1/sync/chunk/${uploadId}/0`,
    chunkPayload.buffer,
    { headers: { 'Content-Type': 'application/octet-stream', Authorization: `Bearer ${token}` } },
  )

  if (chunkRes.status !== 200) {
    syncFailure.add(1)
    syncErrorRate.add(true)
    return
  }

  // Phase 3: Complete.
  const completeRes = http.post(
    `${BASE_URL}/api/v1/sync/complete/${uploadId}`,
    null,
    { headers: { Authorization: `Bearer ${token}` } },
  )

  const latency = Date.now() - start
  syncLatency.add(latency)

  const ok = check(completeRes, {
    'sync complete 200': r => r.status === 200,
    'status SYNCED':     r => {
      try { return JSON.parse(r.body).status === 'SYNCED' } catch { return false }
    },
  })

  if (ok) {
    syncSuccess.add(1)
    syncErrorRate.add(false)
  } else {
    syncFailure.add(1)
    syncErrorRate.add(true)
  }

  sleep(0.05)
}
