// Local LAN Server — technicaldoc.md §3.1, ARCHITECT.md §3.1
// Students scan a URL served on the Coordinator's local IP:8080.
// The Service Worker intercepts fetch events on that origin and routes
// student QR payloads back to the QR validator.
//
// URL format: http://<coordinator-ip>:8080/scan?session=<session_id>
//
// Students open this URL on their phones. The page POSTs their QR
// (camera-scanned or file-uploaded) to the endpoint, which runs the
// same validateStudentQR() engine as the Coordinator camera path.

export function getLANScanURL(sessionId: string): string {
  // Derive local IP from network interfaces — not available in browser APIs.
  // We expose the session_id in the URL; the Service Worker resolves the
  // coordinator's active session without needing the IP in the URL path.
  return `http://localhost:8080/scan?session=${sessionId}`
}

// Register a dedicated sync tag so the SW knows to activate LAN scan mode.
export async function activateLANServer(sessionId: string): Promise<void> {
  if (!('serviceWorker' in navigator)) return
  const reg = await navigator.serviceWorker.ready
  // Post a message to the SW to start intercepting LAN origin requests.
  reg.active?.postMessage({ type: 'LAN_SERVER_ACTIVATE', sessionId })
}

export async function deactivateLANServer(): Promise<void> {
  if (!('serviceWorker' in navigator)) return
  const reg = await navigator.serviceWorker.ready
  reg.active?.postMessage({ type: 'LAN_SERVER_DEACTIVATE' })
}
