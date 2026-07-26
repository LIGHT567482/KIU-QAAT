// QR Scanner component — uses the browser's camera via BarcodeDetector API
// (Chrome 83+) with a fallback message for unsupported browsers.
// Fires onScan(rawQRString) when a QR code is detected.

import { useEffect, useRef, useState } from 'react'

interface Props {
  onScan: (raw: string) => void
  active: boolean
}

declare const BarcodeDetector: {
  new(options: { formats: string[] }): {
    detect(source: HTMLVideoElement): Promise<Array<{ rawValue: string }>>
  }
  getSupportedFormats(): Promise<string[]>
}

export default function QRScanner({ onScan, active }: Props) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const [supported, setSupported] = useState(true)
  const [scanning, setScanning] = useState(false)

  useEffect(() => {
    if (!active) return
    if (!('BarcodeDetector' in window)) {
      setSupported(false)
      return
    }

    let stream: MediaStream | null = null
    let animFrame: number

    const detector = new BarcodeDetector({ formats: ['qr_code'] })

    async function start() {
      stream = await navigator.mediaDevices.getUserMedia({
        video: { facingMode: 'environment' },
      })
      if (!videoRef.current) return
      videoRef.current.srcObject = stream
      await videoRef.current.play()
      setScanning(true)
      scan()
    }

    async function scan() {
      if (!videoRef.current || !active) return
      try {
        const codes = await detector.detect(videoRef.current)
        if (codes.length > 0) {
          onScan(codes[0].rawValue)
        }
      } catch { /* ignore frame errors */ }
      animFrame = requestAnimationFrame(scan)
    }

    start().catch(() => setSupported(false))

    return () => {
      cancelAnimationFrame(animFrame)
      stream?.getTracks().forEach(t => t.stop())
      setScanning(false)
    }
  }, [active, onScan])

  if (!supported) {
    return (
      <div style={container}>
        <p style={{ color: '#b91c1c' }}>
          Camera scanning not supported on this browser.<br />
          Use Chrome 83+ on Android.
        </p>
      </div>
    )
  }

  return (
    <div style={container}>
      <video
        ref={videoRef}
        playsInline
        muted
        style={{ width: '100%', borderRadius: 8, background: '#000' }}
      />
      {!scanning && <p style={{ color: 'var(--muted)', textAlign: 'center' }}>Starting camera…</p>}
    </div>
  )
}

const container: React.CSSProperties = {
  width: '100%', maxWidth: 360, margin: '0 auto',
}
