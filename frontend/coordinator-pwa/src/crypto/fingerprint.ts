// Hardware fingerprint engine — technicaldoc.md §3.3
// Computes a deterministic device fingerprint and binds it to a student on
// first scan. SHA-256 hash is stored; raw components are never persisted.

import { sha256 } from './vault-crypto'

async function getCanvasHash(): Promise<string> {
  const canvas = document.createElement('canvas')
  const ctx = canvas.getContext('2d')!
  ctx.textBaseline = 'top'
  ctx.font = '14px Arial'
  ctx.fillText('QAAT-fingerprint-probe', 2, 2)
  return sha256(canvas.toDataURL())
}

export async function computeHardwareFingerprint(): Promise<string> {
  const components = [
    navigator.userAgent,
    `${screen.width}x${screen.height}`,
    String(screen.colorDepth),
    String(window.devicePixelRatio),
    String(navigator.hardwareConcurrency),
    await getCanvasHash(),
  ]
  return sha256(components.join('|'))
}
