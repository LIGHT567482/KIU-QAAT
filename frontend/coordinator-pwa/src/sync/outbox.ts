// Outbox Queue Manager — technicaldoc.md §8 + ARCHITECT.md §8
// Drives the PENDING → UPLOADING → SYNCED / FAILED state machine.
// Called by the Service Worker Background Sync event (tag: qaat-sync-outbox).

import { db } from '../db/vault'
import { sealSessionPackage } from './sealer'
import { useAuthStore } from '../store/auth'

const API = import.meta.env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')
const CHUNK_SIZE = 65536

// Enqueue a closed session for sync.
export async function enqueueSession(sessionId: string): Promise<void> {
  const existing = await db.outbox_queue.where('session_id').equals(sessionId).first()
  if (existing) return  // already queued

  const pkg = await sealSessionPackage(sessionId)

  await db.outbox_queue.add({
    session_id:    sessionId,
    status:        'PENDING',
    chunks_acked:  [],
    total_chunks:  pkg.totalChunks,
    attempt_count: 0,
    created_at:    new Date().toISOString(),
  })

  // Ask the Service Worker to register a background sync.
  if ('serviceWorker' in navigator) {
    const reg = await navigator.serviceWorker.ready
    await (reg as ServiceWorkerRegistration & { sync: { register(tag: string): Promise<void> } })
      .sync.register('qaat-sync-outbox')
  }
}

// processOutboxQueue is called by the Service Worker on sync events.
// Also callable manually (e.g. user taps "Sync now" when online).
export async function processOutboxQueue(): Promise<void> {
  const pending = await db.outbox_queue
    .where('status').anyOf(['PENDING', 'UPLOADING'])
    .toArray()

  for (const entry of pending) {
    try {
      await uploadEntry(entry.id!)
    } catch (err) {
      await db.outbox_queue.update(entry.id!, {
        status:        'FAILED',
        attempt_count: (entry.attempt_count ?? 0) + 1,
        last_error:    err instanceof Error ? err.message : String(err),
      })
    }
  }
}

async function uploadEntry(entryId: number): Promise<void> {
  const entry = await db.outbox_queue.get(entryId)
  if (!entry) return

  const { token } = useAuthStore.getState()
  const pkg = await sealSessionPackage(entry.session_id)

  await db.outbox_queue.update(entryId, { status: 'UPLOADING' })

  let uploadId = entry.upload_id

  // ── Init (or resume if upload_id already exists) ─────────────────────────
  if (!uploadId) {
    const initRes = await fetch(`${API}/api/v1/sync/init`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify({
        coordinator_id:   useAuthStore.getState().userId,
        session_ids:      [entry.session_id],
        total_chunks:     pkg.totalChunks,
        package_checksum: pkg.packageChecksum,
        package_hmac:     pkg.hmac,
      }),
    })
    if (!initRes.ok) throw new Error(`sync/init failed: ${initRes.status}`)
    const initData = await initRes.json()
    uploadId = initData.upload_id
    await db.outbox_queue.update(entryId, { upload_id: uploadId })
  } else {
    // Resume: find out which chunk to start from.
    const resumeRes = await fetch(`${API}/api/v1/sync/resume/${uploadId}`, {
      headers: { Authorization: `Bearer ${token}` },
    })
    if (resumeRes.ok) {
      const resumeData = await resumeRes.json()
      const chunksAcked: number[] = resumeData.received_chunks ?? []
      await db.outbox_queue.update(entryId, { chunks_acked: chunksAcked })
    }
  }

  // Reload entry to get updated chunks_acked.
  const fresh = await db.outbox_queue.get(entryId)
  const ackedSet = new Set(fresh?.chunks_acked ?? [])

  // ── Upload chunks ─────────────────────────────────────────────────────────
  const payload = pkg.encryptedPayload
  for (let i = 0; i < pkg.totalChunks; i++) {
    if (ackedSet.has(i)) continue  // already uploaded

    const start  = i * CHUNK_SIZE
    const chunk  = payload.slice(start, start + CHUNK_SIZE)
    const chunkBytes = new TextEncoder().encode(chunk)

    const chunkRes = await fetch(`${API}/api/v1/sync/chunk/${uploadId}/${i}`, {
      method:  'POST',
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/octet-stream' },
      body:    chunkBytes,
    })
    if (!chunkRes.ok) throw new Error(`chunk ${i} failed: ${chunkRes.status}`)

    ackedSet.add(i)
    await db.outbox_queue.update(entryId, { chunks_acked: Array.from(ackedSet) })
  }

  // ── Complete ──────────────────────────────────────────────────────────────
  const completeRes = await fetch(`${API}/api/v1/sync/complete/${uploadId}`, {
    method:  'POST',
    headers: { Authorization: `Bearer ${token}` },
  })
  if (!completeRes.ok) throw new Error(`sync/complete failed: ${completeRes.status}`)

  await db.outbox_queue.update(entryId, {
    status:    'SYNCED',
    synced_at: new Date().toISOString(),
  })
}
