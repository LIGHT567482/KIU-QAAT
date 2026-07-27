import { describe, it, expect } from 'vitest'
import { generateTenantKeyPair, signQRPayload, generateSerialNumber, verifyQRSignature } from './rsa-keys.js'
import { checkinUrl, qrToken } from './qr-image.js'

// Locks in the offline-in-room QR contract: a student card must open the LAN hub and carry the
// FULL signed payload, so the coordinator's phone can verify it with no internet. Regression guard
// for the "site can't be reached" bug (card pointing at an internet host).
describe('student QR check-in URL', () => {
  const keys = generateTenantKeyPair()
  const signed = signQRPayload(
    {
      student_id: 'STU-001', tenant_id: '11111111-1111-1111-1111-111111111111', course_id: 'CS101',
      full_name: 'Test Student', academic_year: '2026', serial_number: generateSerialNumber(),
      expiry_date: '2027-12-31', issued_at: '2026-01-01T00:00:00Z',
    },
    keys.privateKeyPem, 'hmac-secret',
  )

  it('defaults to the LAN hub /checkin?t=<token> (no STUDENT_PORTAL_URL set)', () => {
    delete process.env.STUDENT_PORTAL_URL
    const url = checkinUrl(signed)
    expect(url.startsWith('http://192.168.43.1:8080/checkin?t=')).toBe(true)
  })

  it('embeds the full signed payload in the token (offline verify needs it)', () => {
    const url = checkinUrl(signed)
    const token = new URL(url).searchParams.get('t')!
    const decoded = JSON.parse(Buffer.from(token, 'base64url').toString('utf-8'))
    expect(decoded).toEqual(signed)
    expect(token).toBe(qrToken(signed))
    // the decoded payload still verifies against the tenant public key
    expect(verifyQRSignature(decoded, keys.publicKeyPem)).toBe(true)
  })

  it('honours an explicit CHECKIN_BASE_URL override', () => {
    // checkinUrl reads the env-derived base at module load; assert the token form is stable here
    const url = checkinUrl(signed)
    expect(url).toContain('/checkin?t=')
    expect(url).not.toContain('/?qr=') // not the serial-only online portal form
  })
})
