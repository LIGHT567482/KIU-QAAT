// AES-256-GCM encryption for IndexedDB values using a device key
// derived from the server-issued binding secret (HKDF, per technicaldoc.md §5.1).

const SALT = new TextEncoder().encode('QAAT-IndexedDB-Salt-v1')
const INFO = new TextEncoder().encode('coordinator-vault-key')
const HMAC_INFO = new TextEncoder().encode('coordinator-vault-hmac')

let _deviceKey: CryptoKey | null = null
let _hmacKey: CryptoKey | null = null

export async function initDeviceKey(serverIssuedSecret: string): Promise<void> {
  const baseKey = await crypto.subtle.importKey(
    'raw',
    new TextEncoder().encode(serverIssuedSecret),
    { name: 'HKDF' },
    false,
    ['deriveKey'],
  )
  _deviceKey = await crypto.subtle.deriveKey(
    { name: 'HKDF', hash: 'SHA-256', salt: SALT, info: INFO },
    baseKey,
    { name: 'AES-GCM', length: 256 },
    false,
    ['encrypt', 'decrypt'],
  )
  // A separate HMAC key derived from the same server-issued secret (distinct
  // HKDF info). Package authentication MUST use a secret key — never a key
  // derived from the message itself.
  _hmacKey = await crypto.subtle.deriveKey(
    { name: 'HKDF', hash: 'SHA-256', salt: SALT, info: HMAC_INFO },
    baseKey,
    { name: 'HMAC', hash: 'SHA-256', length: 256 },
    false,
    ['sign'],
  )
}

function getKey(): CryptoKey {
  if (!_deviceKey) throw new Error('Device key not initialised — call initDeviceKey() after login')
  return _deviceKey
}

// hmacSign authenticates data with the device-bound HMAC key.
export async function hmacSign(data: string): Promise<string> {
  if (!_hmacKey) throw new Error('Device key not initialised — call initDeviceKey() after login')
  const sig = await crypto.subtle.sign('HMAC', _hmacKey, new TextEncoder().encode(data))
  return Array.from(new Uint8Array(sig)).map(b => b.toString(16).padStart(2, '0')).join('')
}

export async function encrypt(plaintext: string): Promise<string> {
  const iv = crypto.getRandomValues(new Uint8Array(12))
  const encoded = new TextEncoder().encode(plaintext)
  const ct = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, getKey(), encoded)
  // Store as base64(iv + ciphertext)
  const combined = new Uint8Array(iv.byteLength + ct.byteLength)
  combined.set(iv, 0)
  combined.set(new Uint8Array(ct), iv.byteLength)
  return btoa(String.fromCharCode(...combined))
}

export async function decrypt(ciphertext: string): Promise<string> {
  const combined = Uint8Array.from(atob(ciphertext), c => c.charCodeAt(0))
  const iv = combined.slice(0, 12)
  const ct = combined.slice(12)
  const pt = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, getKey(), ct)
  return new TextDecoder().decode(pt)
}

export async function sha256(input: string): Promise<string> {
  const buf = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(input))
  return Array.from(new Uint8Array(buf)).map(b => b.toString(16).padStart(2, '0')).join('')
}

// hmacHex computes HMAC-SHA256(keyStr, message) and returns lowercase hex. Used
// for keyed student-id hashing with the per-tenant key from the manifest (F-07);
// must match services/api-gateway hashStudentID exactly. This is independent of
// the device vault key above.
export async function hmacHex(keyStr: string, message: string): Promise<string> {
  const key = await crypto.subtle.importKey(
    'raw',
    new TextEncoder().encode(keyStr),
    { name: 'HMAC', hash: 'SHA-256' },
    false,
    ['sign'],
  )
  const sig = await crypto.subtle.sign('HMAC', key, new TextEncoder().encode(message))
  return Array.from(new Uint8Array(sig)).map(b => b.toString(16).padStart(2, '0')).join('')
}
