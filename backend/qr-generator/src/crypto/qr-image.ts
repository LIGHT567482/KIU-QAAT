import QRCode from 'qrcode'
import type { SignedQRPayload } from './rsa-keys.js'

// Base URL of the student check-in page. The QR encodes a FULL URL (host + /checkin?t=<token>)
// so scanning with the phone camera opens the page directly (captive-portal pattern).
//
// OFFLINE IN-ROOM MODEL: the hub is the coordinator's app SERVING on the coordinator's own phone
// hotspot (named after the cohort in Settings), with NO internet. The card must bake that LAN host —
// an internet host is unreachable on the offline hotspot ("site can't be reached"). The Android
// phone-hotspot gateway is 192.168.43.1 by default, so cards default to it and work offline out of
// the box. Override with STUDENT_CHECKIN_BASE_URL / CHECKIN_BASE_URL if your phones use a different
// tethering subnet (check the app's "serving at <ip>" line) or you serve from another origin.
const INROOM_HUB_BASE_URL = 'http://192.168.43.1:8080'
const CHECKIN_BASE_URL = (
  process.env.STUDENT_CHECKIN_BASE_URL || process.env.CHECKIN_BASE_URL || INROOM_HUB_BASE_URL
).replace(/\/$/, '')

// Preferred destination: the student portal. Scanning the QR with the phone's
// native camera/browser opens the portal at /?qr=<token>, where the student is
// logged in passwordlessly (the portal exchanges the token for a session) and can
// both attend live sessions and track their attendance — no app to install.
const STUDENT_PORTAL_URL = (process.env.STUDENT_PORTAL_URL || '').replace(/\/$/, '')

// The base64url token carrying the whole signed payload (what the offline hub decodes at /checkin).
export function qrToken(payload: SignedQRPayload): string {
  return Buffer.from(JSON.stringify(payload)).toString('base64url')
}

// The exact string encoded into a student's QR — the single source of truth for both the rendered
// image and any verification/preview. Offline in-room default: the LAN hub /checkin?t=<token>.
// Only when STUDENT_PORTAL_URL is explicitly set does it switch to the online serial-only portal URL.
export function checkinUrl(payload: SignedQRPayload, org?: string): string {
  const orgQS = org ? `&org=${encodeURIComponent(org)}` : ''
  return STUDENT_PORTAL_URL
    ? `${STUDENT_PORTAL_URL}/?qr=${payload.serial_number}${orgQS}`
    : `${CHECKIN_BASE_URL}/checkin?t=${qrToken(payload)}`
}

// Renders a 1024×1024 PNG buffer from a signed QR payload.
export async function renderQRImage(payload: SignedQRPayload, org?: string): Promise<Buffer> {
  return QRCode.toBuffer(checkinUrl(payload, org), {
    width: 1024,
    errorCorrectionLevel: 'H',
    margin: 2,
    color: { dark: '#000000', light: '#ffffff' },
  })
}
