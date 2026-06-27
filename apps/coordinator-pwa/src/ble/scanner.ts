// BLE Proximity Engine — technicaldoc.md §4
// Scans for a specific beacon UUID and maintains a 10-second weighted RSSI
// rolling average. More recent readings have higher weight.

interface RSSIReading {
  rssi: number
  timestamp: number
}

const rssiReadings = new Map<string, RSSIReading[]>()

// requestLEScan requires the browser flag or a specific permissions context.
// Web Bluetooth advertisement scanning is behind a flag in Chrome (--enable-experimental-web-platform-features).
export async function startBLEScan(beaconUUID: string): Promise<void> {
  // Remove dashes from UUID for service filter (Web Bluetooth format).
  const serviceUUID = beaconUUID.toLowerCase()

  rssiReadings.set(beaconUUID, [])

  await navigator.bluetooth.requestLEScan({
    filters: [{ services: [serviceUUID] }],
    keepRepeatedDevices: true,
  })

  navigator.bluetooth.addEventListener('advertisementreceived', (event: Event) => {
    const adv = event as BluetoothAdvertisingEvent
    const readings = rssiReadings.get(beaconUUID) ?? []
    readings.push({ rssi: adv.rssi, timestamp: Date.now() })
    pruneOldReadings(beaconUUID, 10_000)
    rssiReadings.set(beaconUUID, readings)
  })
}

export function stopBLEScan(): void {
  navigator.bluetooth.requestLEScan?.({ keepRepeatedDevices: false })
    .catch(() => {})
}

// Returns the 10-second weighted RSSI average, or null if no readings.
export function getBLERollingAverage(beaconUUID: string, windowMs = 10_000): number | null {
  const cutoff = Date.now() - windowMs
  const recent = (rssiReadings.get(beaconUUID) ?? []).filter(r => r.timestamp > cutoff)
  if (recent.length === 0) return null

  // Weight: index position within the window (later = higher weight).
  const { sum, weights } = recent.reduce(
    (acc, r, i) => {
      const w = (i + 1) / recent.length
      return { sum: acc.sum + r.rssi * w, weights: acc.weights + w }
    },
    { sum: 0, weights: 0 },
  )
  return sum / weights
}

export function isBLEBeaconDetected(beaconUUID: string, thresholdDBM: number): boolean {
  const avg = getBLERollingAverage(beaconUUID)
  return avg !== null && avg >= thresholdDBM
}

function pruneOldReadings(beaconUUID: string, windowMs: number): void {
  const cutoff = Date.now() - windowMs
  const readings = rssiReadings.get(beaconUUID) ?? []
  rssiReadings.set(beaconUUID, readings.filter(r => r.timestamp > cutoff))
}

// Compatibility check — Web Bluetooth is not available on all platforms.
// iOS Safari does not support it at all (plan.md §9 risk register).
export function isBLESupported(): boolean {
  return 'bluetooth' in navigator
}
