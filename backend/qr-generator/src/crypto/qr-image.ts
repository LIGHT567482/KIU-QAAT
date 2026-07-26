import QRCode from 'qrcode'
import type { SignedQRPayload } from './rsa-keys.js'

// Base URL of the student check-in portal. The QR encodes a full URL so that
// scanning with the phone camera opens the page directly (captive portal pattern).
// STUDENT_CHECKIN_BASE_URL is the dedicated student origin, kept independent from
// the coordinator/admin console (e.g. http://attend.uni.edu). CHECKIN_BASE_URL is
// honoured as a legacy fallback; if neither is set a relative path is used (works
// only when scanning from the same origin).
const CHECKIN_BASE_URL = (
  process.env.STUDENT_CHECKIN_BASE_URL || process.env.CHECKIN_BASE_URL || ''
).replace(/\/$/, '')

// Preferred destination: the student portal. Scanning the QR with the phone's
// native camera/browser opens the portal at /?qr=<token>, where the student is
// logged in passwordlessly (the portal exchanges the token for a session) and can
// both attend live sessions and track their attendance — no app to install.
const STUDENT_PORTAL_URL = (process.env.STUDENT_PORTAL_URL || '').replace(/\/$/, '')

// Renders a 1024×1024 PNG buffer from a signed QR payload.
export async function renderQRImage(payload: SignedQRPayload, org?: string): Promise<Buffer> {
  const token = Buffer.from(JSON.stringify(payload)).toString('base64url')
  // Open the full student portal when configured; otherwise fall back to the
  // single-session captive check-in page on the API origin.
  // Encode only the short, globally-unique serial so the QR stays low-density and
  // scans reliably; the portal resolves it server-side at /student/qr-login.
  // &org lets the portal resolve the institution for the attendance lookup even when
  // the QR is scanned off any coordinator hotspot (the public progress call needs it).
  const orgQS = org ? `&org=${encodeURIComponent(org)}` : ''
  const qrValue = STUDENT_PORTAL_URL
    ? `${STUDENT_PORTAL_URL}/?qr=${payload.serial_number}${orgQS}`
    : `${CHECKIN_BASE_URL}/checkin?t=${token}`
  return QRCode.toBuffer(qrValue, {
    width: 1024,
    errorCorrectionLevel: 'H',
    margin: 2,
    color: { dark: '#000000', light: '#ffffff' },
  })
}
