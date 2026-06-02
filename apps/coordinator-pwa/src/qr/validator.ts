// QR Validation Engine — technicaldoc.md §3.4
// Implements all 8 validation steps in order. Any failure returns immediately.

import { db } from '../db/vault'
import { hmacHex } from '../crypto/vault-crypto'

export type ValidationStatus =
  | 'PRESENT'
  | 'REJECTED'

export type RejectionReason =
  | 'INVALID_SIGNATURE'
  | 'QR_EXPIRED'
  | 'TENANT_MISMATCH'
  | 'NOT_ON_ROSTER'
  | 'SERIAL_REVOKED'
  | 'PROXIMITY_FAILED'
  | 'DEVICE_MISMATCH'
  | 'DUPLICATE_SCAN'
  | 'DEVICE_ALREADY_USED'
  | 'SESSION_NOT_ACTIVE'
  | 'GATE_NOT_OPEN'

export interface ValidationResult {
  status: ValidationStatus
  reason?: RejectionReason
  studentIdHash?: string
  rssi?: number
  auditFlag?: string
}

export interface ActiveSession {
  sessionId: string
  tenantId: string
  beaconUUID: string
  academicYear: string
  institutionPublicKey: string    // PEM
  studentHashKey: string          // per-tenant HMAC key (from manifest, F-07)
  rosterHashes: string[]          // hmac(student_id) for enrolled students
  rosterSerials: Map<string, string> // hash → qr_serial_number
  policy: {
    rssiThresholdDBM: number
  }
}

// DeviceContext carries the proximity + device-identity signals measured on the
// *student's* handset (when checking in via the LAN page) and forwarded with the
// scan. For the Coordinator camera fallback it carries the Coordinator device's
// own values. Binding these to the submitting device — rather than recomputing
// them on whatever device runs the validator — is what makes the fingerprint and
// proximity checks meaningful (F-01, F-06).
export interface DeviceContext {
  fingerprintHash: string
  rssi: number | null
}

interface QRPayload {
  student_id: string
  tenant_id: string
  academic_year: string
  serial_number: string
  expiry_date: string
  issued_at: string
  signature: string
  hmac: string
}

export async function validateStudentQR(
  rawQR: string,
  session: ActiveSession,
  device: DeviceContext,
): Promise<ValidationResult> {

  // ── Step 1: RSA-2048 signature verification ────────────────────────────────
  let payload: QRPayload
  try {
    payload = JSON.parse(rawQR)
  } catch {
    return { status: 'REJECTED', reason: 'INVALID_SIGNATURE' }
  }

  const signatureValid = await verifyRSASignature(payload, session.institutionPublicKey)
  if (!signatureValid) {
    return { status: 'REJECTED', reason: 'INVALID_SIGNATURE' }
  }

  // ── Step 2: Expiry check ───────────────────────────────────────────────────
  // An unparseable date yields NaN; treat that (and any past date) as expired
  // rather than letting a malformed payload slip through.
  const expiryMs = new Date(payload.expiry_date).getTime()
  if (Number.isNaN(expiryMs) || expiryMs < Date.now()) {
    return { status: 'REJECTED', reason: 'QR_EXPIRED' }
  }

  // ── Step 3: Tenant ID match ────────────────────────────────────────────────
  if (payload.tenant_id !== session.tenantId) {
    return { status: 'REJECTED', reason: 'TENANT_MISMATCH' }
  }

  // ── Step 4: Roster lookup (keyed hash of student_id vs cached roster) ──────
  // HMAC-SHA256 with the per-tenant key so the stored hash is not reversible
  // from a low-entropy registration number (F-07).
  const studentIdHash = await hmacHex(session.studentHashKey, payload.student_id)
  if (!session.rosterHashes.includes(studentIdHash)) {
    return { status: 'REJECTED', reason: 'NOT_ON_ROSTER' }
  }

  // ── Step 4b: Serial number must match the current issued serial ───────────
  // Rejects superseded QR codes after a reissue (lost/stolen/compromised card).
  // Without this check, revocation via reissueQR has no effect.
  const currentSerial = session.rosterSerials.get(studentIdHash)
  if (!currentSerial || payload.serial_number !== currentSerial) {
    return { status: 'REJECTED', reason: 'SERIAL_REVOKED' }
  }

  // ── Step 5: BLE proximity check ───────────────────────────────────────────
  // The RSSI is the rolling average measured on the submitting device (the
  // student's handset on the LAN path), not on whatever device runs this code
  // (F-06). A null reading means the device could not hear the beacon.
  const rssi = device.rssi
  if (rssi === null || rssi < session.policy.rssiThresholdDBM) {
    return { status: 'REJECTED', reason: 'PROXIMITY_FAILED' }
  }

  // ── Step 6: Hardware fingerprint check ────────────────────────────────────
  // Bind the *submitting* device's fingerprint (computed on that device and sent
  // with the scan), so a proxy on a different handset is caught (F-01).
  const deviceFingerprint = device.fingerprintHash
  const storedBinding = await db.hardware_vault.get(studentIdHash)

  if (storedBinding) {
    if (storedBinding.fingerprint_hash !== deviceFingerprint) {
      return {
        status: 'REJECTED',
        reason: 'DEVICE_MISMATCH',
        auditFlag: 'DEVICE_MISMATCH',
      }
    }
  } else {
    // First scan — bind this device to the student for the academic year.
    await db.hardware_vault.put({
      student_id_hash: studentIdHash,
      fingerprint_hash: deviceFingerprint,
      academic_year:    session.academicYear,
      first_bound_at:   new Date().toISOString(),
    })
  }

  // ── Step 7: Duplicate scan check ──────────────────────────────────────────
  const existing = await db.attendance_records
    .where('[session_id+student_id_hash]')
    .equals([session.sessionId, studentIdHash])
    .first()
  if (existing) {
    return { status: 'REJECTED', reason: 'DUPLICATE_SCAN' }
  }

  // ── Step 8: One-device-per-session check ──────────────────────────────────
  // A device that already recorded another student in this session is blocked.
  const deviceUsed = await db.attendance_records
    .where('session_id')
    .equals(session.sessionId)
    .filter(r => r.device_fingerprint_hash === deviceFingerprint && r.student_id_hash !== studentIdHash)
    .first()
  if (deviceUsed) {
    return { status: 'REJECTED', reason: 'DEVICE_ALREADY_USED' }
  }

  // ── All checks passed → record attendance ─────────────────────────────────
  const nextSeq = await getNextSequence(session.sessionId)
  await db.attendance_records.add({
    log_id:                 crypto.randomUUID(),
    session_id:             session.sessionId,
    student_id_hash:        studentIdHash,
    device_fingerprint_hash: deviceFingerprint,
    beacon_rssi:            Math.round(rssi),
    sequence_number:        nextSeq,
    checkin_timestamp:      new Date().toISOString(),
    entry_method:           'QR_SCAN',
  })

  return { status: 'PRESENT', studentIdHash, rssi: Math.round(rssi) }
}

// ─── RSA-2048 signature verification via SubtleCrypto ─────────────────────────
// We use SubtleCrypto (native browser) instead of jsrsasign to avoid
// loading a large library on the critical validation path.

async function verifyRSASignature(payload: QRPayload, publicKeyPEM: string): Promise<boolean> {
  try {
    const { signature, hmac, ...body } = payload
    const bodyStr = JSON.stringify(body)
    const encoded = new TextEncoder().encode(bodyStr)

    const keyData = pemToArrayBuffer(publicKeyPEM)
    const cryptoKey = await crypto.subtle.importKey(
      'spki',
      keyData,
      { name: 'RSASSA-PKCS1-v1_5', hash: 'SHA-256' },
      false,
      ['verify'],
    )

    const sigBytes = base64ToArrayBuffer(signature)
    return await crypto.subtle.verify('RSASSA-PKCS1-v1_5', cryptoKey, sigBytes, encoded)
  } catch {
    return false
  }
}

function pemToArrayBuffer(pem: string): ArrayBuffer {
  const b64 = pem
    .replace(/-----BEGIN PUBLIC KEY-----/, '')
    .replace(/-----END PUBLIC KEY-----/, '')
    .replace(/\s+/g, '')
  return base64ToArrayBuffer(b64)
}

function base64ToArrayBuffer(b64: string): ArrayBuffer {
  const bin = atob(b64)
  const buf = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) buf[i] = bin.charCodeAt(i)
  return buf.buffer
}

async function getNextSequence(sessionId: string): Promise<number> {
  const records = await db.attendance_records
    .where('session_id')
    .equals(sessionId)
    .count()
  return records + 1
}
