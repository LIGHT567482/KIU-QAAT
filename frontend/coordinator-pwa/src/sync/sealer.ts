// Session Package Sealer — technicaldoc.md §3.5
// Seals a closed session into an AES-256-GCM encrypted package and places
// it in the IndexedDB outbox queue for the Service Worker to upload.

import { db } from '../db/vault'
import { encrypt, hmacSign } from '../crypto/vault-crypto'
import { useAuthStore } from '../store/auth'

const CHUNK_SIZE = 65536 // must match Sync Receiver

export interface SealedPackage {
  sessionId: string
  encryptedPayload: string   // base64(iv + ciphertext)
  hmac: string               // HMAC-SHA256 of plaintext payload
  totalChunks: number
  packageChecksum: string    // SHA-256 of the full encrypted payload (hex)
}

export async function sealSessionPackage(sessionId: string): Promise<SealedPackage> {
  const session = await db.sessions.get(sessionId)
  if (!session) throw new Error(`session ${sessionId} not found in vault`)

  const records = await db.attendance_records.where('session_id').equals(sessionId).toArray()

  const { userId: coordinatorId } = useAuthStore.getState()

  const plaintext = JSON.stringify({
    session,
    attendance_records: records,
    sealed_at:         new Date().toISOString(),
    coordinator_id:    coordinatorId,
    package_version:   '1.0',
  })

  const encPayload = await encrypt(plaintext)

  // Authenticate the ciphertext with the device-bound HMAC key. Keyed by a
  // secret (not by the payload), so it actually proves the package originated
  // from this coordinator device and was not tampered with in transit.
  const hmac = await hmacSign(encPayload)

  // Compute SHA-256 of the encrypted payload for server-side integrity check.
  const encBytes = new TextEncoder().encode(encPayload)
  const hashBuf  = await crypto.subtle.digest('SHA-256', encBytes)
  const packageChecksum = Array.from(new Uint8Array(hashBuf))
    .map(b => b.toString(16).padStart(2, '0'))
    .join('')

  const totalChunks = Math.ceil(encPayload.length / CHUNK_SIZE)

  return { sessionId, encryptedPayload: encPayload, hmac, totalChunks, packageChecksum }
}
