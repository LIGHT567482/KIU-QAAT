import QRCode from 'qrcode'
import type { SignedQRPayload } from './rsa-keys.js'

// Renders a 1024×1024 PNG buffer from a signed QR payload.
export async function renderQRImage(payload: SignedQRPayload): Promise<Buffer> {
  return QRCode.toBuffer(JSON.stringify(payload), {
    width: 1024,
    errorCorrectionLevel: 'H',
    margin: 2,
    color: { dark: '#000000', light: '#ffffff' },
  })
}
