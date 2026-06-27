// Tenant RSA key store — reads/writes tenant key pairs from PostgreSQL.
// In production, private keys would live in an HSM; here they are AES-256
// encrypted at rest using a server-side master key from an env variable.

import { createCipheriv, createDecipheriv, randomBytes } from 'node:crypto'
import type { Queryable } from '../db.js'

// Fail fast: a missing/short KEK must never silently fall back to a known key
// (e.g. all-zeros), which would render every tenant's private key effectively
// plaintext. Require a 32-byte (64 hex char) key supplied via the environment.
function loadMasterKey(): Buffer {
  const hex = process.env.KEY_ENCRYPTION_KEY
  if (!hex || !/^[0-9a-fA-F]{64}$/.test(hex)) {
    throw new Error(
      'KEY_ENCRYPTION_KEY must be set to a 64-character hex string (32 bytes / AES-256)',
    )
  }
  return Buffer.from(hex, 'hex')
}

const MASTER_KEY = loadMasterKey()

// AES-256-GCM provides confidentiality AND integrity. The tenant_id is bound as
// Additional Authenticated Data so a ciphertext row cannot be moved/replayed
// between tenants without failing decryption.
function encryptKey(pem: string, tenantId: string): string {
  const iv = randomBytes(12)
  const cipher = createCipheriv('aes-256-gcm', MASTER_KEY, iv)
  cipher.setAAD(Buffer.from(tenantId, 'utf8'))
  const encrypted = Buffer.concat([cipher.update(pem, 'utf8'), cipher.final()])
  const tag = cipher.getAuthTag()
  return [iv.toString('hex'), tag.toString('hex'), encrypted.toString('hex')].join(':')
}

function decryptKey(stored: string, tenantId: string): string {
  const parts = stored.split(':')

  // Legacy format: "iv:ciphertext" (AES-256-CBC, no auth tag, no AAD). Older
  // rows were written before the switch to GCM; decrypt them transparently so
  // existing tenants keep working. New rows are always re-encrypted as GCM on
  // the next storeTenantKeys() call.
  if (parts.length === 2) {
    const [ivHex, ctHex] = parts
    const decipher = createDecipheriv('aes-256-cbc', MASTER_KEY, Buffer.from(ivHex, 'hex'))
    return Buffer.concat([decipher.update(Buffer.from(ctHex, 'hex')), decipher.final()]).toString('utf8')
  }

  // Current format: "iv:tag:ciphertext" (AES-256-GCM, tenant_id as AAD).
  const [ivHex, tagHex, ctHex] = parts
  const decipher = createDecipheriv('aes-256-gcm', MASTER_KEY, Buffer.from(ivHex, 'hex'))
  decipher.setAAD(Buffer.from(tenantId, 'utf8'))
  decipher.setAuthTag(Buffer.from(tagHex, 'hex'))
  return Buffer.concat([decipher.update(Buffer.from(ctHex, 'hex')), decipher.final()]).toString('utf8')
}

export interface TenantKeys {
  privateKeyPem: string
  publicKeyPem: string
  hmacSecret: string
}

// db must be a connection already scoped to tenantId (see db.withTenant), because
// tenant_rsa_keys is RLS-protected under the qaat_app role.
export async function getTenantKeys(db: Queryable, tenantId: string): Promise<TenantKeys | null> {
  const res = await db.query(
    `SELECT rsa_private_key_enc, rsa_public_key_pem, hmac_secret_enc
     FROM tenant_rsa_keys WHERE tenant_id = $1 AND revoked_at IS NULL
     ORDER BY created_at DESC LIMIT 1`,
    [tenantId],
  )
  if (res.rows.length === 0) return null
  const row = res.rows[0]
  return {
    privateKeyPem: decryptKey(row.rsa_private_key_enc, tenantId),
    publicKeyPem:  row.rsa_public_key_pem,
    hmacSecret:    decryptKey(row.hmac_secret_enc, tenantId),
  }
}

export async function storeTenantKeys(
  db: Queryable,
  tenantId: string,
  privateKeyPem: string,
  publicKeyPem: string,
): Promise<void> {
  const hmacSecret = randomBytes(32).toString('hex')
  await db.query(
    `INSERT INTO tenant_rsa_keys (tenant_id, rsa_private_key_enc, rsa_public_key_pem, hmac_secret_enc)
     VALUES ($1, $2, $3, $4)`,
    [tenantId, encryptKey(privateKeyPem, tenantId), publicKeyPem, encryptKey(hmacSecret, tenantId)],
  )
}
