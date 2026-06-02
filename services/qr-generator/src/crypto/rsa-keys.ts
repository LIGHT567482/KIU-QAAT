import { createSign, createVerify, generateKeyPairSync, randomBytes } from 'node:crypto'

export interface TenantKeyPair {
  privateKeyPem: string
  publicKeyPem: string
}

// Generates a fresh RSA-2048 key pair for a tenant.
// In production this would be stored in HSM; here we store in DB (encrypted).
export function generateTenantKeyPair(): TenantKeyPair {
  const { privateKey, publicKey } = generateKeyPairSync('rsa', {
    modulusLength: 2048,
    publicKeyEncoding:  { type: 'spki',  format: 'pem' },
    privateKeyEncoding: { type: 'pkcs8', format: 'pem' },
  })
  return { privateKeyPem: privateKey, publicKeyPem: publicKey }
}

export interface QRPayload {
  student_id: string
  tenant_id: string
  academic_year: string
  serial_number: string
  expiry_date: string        // ISO date "YYYY-MM-DD"
  issued_at: string          // ISO timestamp
}

export interface SignedQRPayload extends QRPayload {
  signature: string          // RSA-SHA256 base64
  hmac: string               // HMAC-SHA256 hex (uses tenant HMAC secret)
}

import { createHmac } from 'node:crypto'

export function signQRPayload(payload: QRPayload, privateKeyPem: string, hmacSecret: string): SignedQRPayload {
  const body = JSON.stringify(payload)

  const sign = createSign('RSA-SHA256')
  sign.update(body)
  const signature = sign.sign(privateKeyPem, 'base64')

  const hmac = createHmac('sha256', hmacSecret).update(body).digest('hex')

  return { ...payload, signature, hmac }
}

export function verifyQRSignature(signed: SignedQRPayload, publicKeyPem: string): boolean {
  const { signature, hmac, ...payload } = signed
  const body = JSON.stringify(payload)
  const verify = createVerify('RSA-SHA256')
  verify.update(body)
  return verify.verify(publicKeyPem, signature, 'base64')
}

export function generateSerialNumber(): string {
  const year = new Date().getFullYear()
  // 12 hex chars (48 bits) from a CSPRNG — unpredictable and collision-resistant,
  // unlike Math.random() over a 10^6 space. Serials gate QR validation after a
  // reissue, so they must not be guessable.
  const rand = randomBytes(6).toString('hex').toUpperCase()
  return `SN-${year}-${rand}`
}
