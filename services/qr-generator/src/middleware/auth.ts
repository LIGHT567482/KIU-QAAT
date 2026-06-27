// JWT verification for qr-generator.
//
// Although the API gateway authenticates + RBAC-checks these routes, qr-generator
// mints RSA-signed QR codes — the identity credential the whole check-in trusts —
// so it MUST NOT be exploitable if reached directly (SSRF / in-cluster lateral
// movement / a misapplied NetworkPolicy). It verifies the RS256 token itself and
// derives the tenant from the verified claim, overwriting any client-supplied
// X-Tenant-ID. Mirrors the Go services' auth packages. (update.md H1)

import type { NextFunction, Request, Response } from 'express'
import { createVerify } from 'node:crypto'
import { readFileSync } from 'node:fs'

const PUBLIC_KEY_PEM = loadPublicKey()
const ISSUER = process.env.JWT_ISSUER ?? 'qaat-auth'
const AUDIENCE = process.env.JWT_AUDIENCE ?? 'qaat-api'

function loadPublicKey(): string {
  const path = process.env.RSA_PUBLIC_KEY_PATH ?? 'keys/auth_public.pem'
  return readFileSync(path, 'utf8')
}

function b64urlToBuffer(s: string): Buffer {
  return Buffer.from(s.replace(/-/g, '+').replace(/_/g, '/'), 'base64')
}

interface Claims {
  sub?: string
  iss?: string
  aud?: string | string[]
  exp?: number
  tenant_id?: string
  role?: string
}

function verifyRS256(token: string): Claims | null {
  const parts = token.split('.')
  if (parts.length !== 3) return null
  const [headerB64, payloadB64, sigB64] = parts

  let header: { alg?: string }
  let claims: Claims
  try {
    header = JSON.parse(b64urlToBuffer(headerB64).toString('utf8'))
    claims = JSON.parse(b64urlToBuffer(payloadB64).toString('utf8'))
  } catch {
    return null
  }

  // Reject anything but RS256 — prevents alg=none / HS256 confusion.
  if (header.alg !== 'RS256') return null

  const verifier = createVerify('RSA-SHA256')
  verifier.update(`${headerB64}.${payloadB64}`)
  verifier.end()
  let ok = false
  try {
    ok = verifier.verify(PUBLIC_KEY_PEM, b64urlToBuffer(sigB64))
  } catch {
    return null
  }
  if (!ok) return null

  // Standard claim checks.
  if (claims.iss !== ISSUER) return null
  const aud = Array.isArray(claims.aud) ? claims.aud : [claims.aud]
  if (!aud.includes(AUDIENCE)) return null
  if (typeof claims.exp === 'number' && claims.exp * 1000 <= Date.now()) return null

  return claims
}

// requireJWT verifies the Bearer token and pins the tenant context from the
// verified claim. On success it overwrites X-Tenant-ID so downstream code
// (callerTenant) can only ever see the authenticated tenant.
export function requireJWT(req: Request, res: Response, next: NextFunction): void {
  const authz = req.header('authorization') ?? ''
  if (!authz.startsWith('Bearer ')) {
    res.status(401).json({ error: 'MISSING_TOKEN', message: 'Authorization: Bearer <token> required' })
    return
  }
  const claims = verifyRS256(authz.slice('Bearer '.length))
  if (!claims || !claims.tenant_id) {
    res.status(401).json({ error: 'TOKEN_INVALID', message: 'token is invalid or expired' })
    return
  }
  // Pin verified identity; never trust an inbound copy.
  req.headers['x-tenant-id'] = claims.tenant_id
  if (claims.role) req.headers['x-role'] = claims.role
  if (claims.sub) req.headers['x-user-id'] = claims.sub
  next()
}
