// LAN validation host (main thread side).
//
// Students submit their QR image to the Service Worker's LAN /submit endpoint.
// The SW cannot run the 8-step validator itself — that needs the live session
// context (manifest, machine state), the BLE rolling average, IndexedDB and a
// DOM for QR decoding, all of which live on the main thread. So the SW relays
// each submission here over a BroadcastChannel; we decode the QR, run
// validateStudentQR, and post the result back for the SW to return to the
// student's page.
//
// The QR is decoded here, but the device-identity and proximity signals are
// measured on the *student's* handset (the LAN scan page) and forwarded in the
// submission, so the fingerprint and BLE checks bind the student's device rather
// than the Coordinator's (F-01, F-06).

import type { DeviceContext, ValidationResult } from '../qr/validator'

const CHANNEL = 'qaat-lan-validation'

type Validator = (raw: string, sessionId: string, device: DeviceContext) => Promise<ValidationResult>

interface ValidateRequest {
  type: 'QAAT_LAN_VALIDATE'
  requestId: string
  sessionId: string
  image: ArrayBuffer
  fingerprintHash: string
  rssi: number | null
}

declare const BarcodeDetector: {
  new (options: { formats: string[] }): {
    detect(source: ImageBitmapSource): Promise<Array<{ rawValue: string }>>
  }
}

let channel: BroadcastChannel | null = null
let activeValidator: Validator | null = null

// startLANValidationHost registers the validator used to process student
// submissions and returns a disposer. Call it while the session is ACTIVE.
export function startLANValidationHost(validator: Validator): () => void {
  activeValidator = validator
  if (!channel) {
    channel = new BroadcastChannel(CHANNEL)
    channel.addEventListener('message', onMessage)
  }
  return () => {
    activeValidator = null
  }
}

async function onMessage(event: MessageEvent): Promise<void> {
  const data = event.data as ValidateRequest | undefined
  if (data?.type !== 'QAAT_LAN_VALIDATE' || !channel) return

  let result: { status: string; reason?: string }
  try {
    if (!activeValidator) {
      result = { status: 'REJECTED', reason: 'SESSION_NOT_ACTIVE' }
    } else {
      const raw = await decodeQR(data.image)
      result = raw === null
        ? { status: 'REJECTED', reason: 'UNREADABLE_QR' }
        : await activeValidator(raw, data.sessionId, {
            fingerprintHash: data.fingerprintHash,
            rssi: data.rssi,
          })
    }
  } catch {
    result = { status: 'REJECTED', reason: 'VALIDATION_ERROR' }
  }

  channel.postMessage({
    type: 'QAAT_LAN_RESULT',
    requestId: data.requestId,
    status: result.status,
    reason: result.reason,
  })
}

async function decodeQR(image: ArrayBuffer): Promise<string | null> {
  if (!('BarcodeDetector' in self)) return null
  try {
    const bitmap = await createImageBitmap(new Blob([image]))
    const detector = new BarcodeDetector({ formats: ['qr_code'] })
    const codes = await detector.detect(bitmap)
    bitmap.close()
    return codes.length > 0 ? codes[0].rawValue : null
  } catch {
    return null
  }
}
