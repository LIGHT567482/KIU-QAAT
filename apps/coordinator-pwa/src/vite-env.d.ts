/// <reference types="vite/client" />
/// <reference types="vite-plugin-pwa/client" />

interface ImportMetaEnv {
  readonly VITE_API_URL: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

// ─── Web Bluetooth (experimental advertisement scanning) ──────────────────────
// Minimal ambient declarations: requestLEScan and the advertisementreceived
// event are not yet in the standard TS DOM lib but are used by the BLE engine.
interface BluetoothAdvertisingEvent extends Event {
  readonly rssi: number
  readonly device: { id: string; name?: string }
}

interface BluetoothLEScan {
  stop(): void
}

interface Bluetooth extends EventTarget {
  requestLEScan(options?: {
    filters?: Array<{ services?: string[] }>
    keepRepeatedDevices?: boolean
    acceptAllAdvertisements?: boolean
  }): Promise<BluetoothLEScan>
}

interface Navigator {
  readonly bluetooth: Bluetooth
}
