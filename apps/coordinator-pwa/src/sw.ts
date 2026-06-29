/// <reference lib="webworker" />
import { cleanupOutdatedCaches, precacheAndRoute } from 'workbox-precaching'
import { registerRoute } from 'workbox-routing'
import { NetworkFirst } from 'workbox-strategies'
import { BackgroundSyncPlugin } from 'workbox-background-sync'

declare const self: ServiceWorkerGlobalScope

cleanupOutdatedCaches()
precacheAndRoute(self.__WB_MANIFEST)

// ─── Outbox background sync ───────────────────────────────────────────────────
// Fired by the app via: navigator.serviceWorker.ready.then(r => r.sync.register('qaat-sync-outbox'))

const bgSyncPlugin = new BackgroundSyncPlugin('qaat-sync-queue', {
  maxRetentionTime: 7 * 24 * 60, // 7 days in minutes
})

registerRoute(
  ({ url }) => url.pathname.startsWith('/api/v1/sync/'),
  new NetworkFirst({ plugins: [bgSyncPlugin] }),
  'POST',
)

// ─── Manifest caching (NetworkFirst, 24h TTL) ─────────────────────────────────
registerRoute(
  ({ url }) => url.pathname.startsWith('/api/v1/manifest/'),
  new NetworkFirst({ cacheName: 'manifest-cache' }),
)

// ─── Local LAN server (port 8080) ────────────────────────────────────────────
// Intercepts student scan requests from the local network.

let activeLANSessionId: string | null = null

self.addEventListener('message', (event) => {
  if (event.data?.type === 'SKIP_WAITING') self.skipWaiting()
  if (event.data?.type === 'LAN_SERVER_ACTIVATE') activeLANSessionId = event.data.sessionId
  if (event.data?.type === 'LAN_SERVER_DEACTIVATE') activeLANSessionId = null
})

// Intercept requests to localhost:8080 (student scan page).
self.addEventListener('fetch', (event: FetchEvent) => {
  const url = new URL(event.request.url)
  if (url.port !== '8080' && url.hostname !== 'localhost') return

  if (url.pathname === '/scan') {
    event.respondWith(studentScanPage(url))
  } else if (url.pathname === '/submit' && event.request.method === 'POST') {
    event.respondWith(handleStudentSubmit(event.request))
  }
})

function studentScanPage(url: URL): Response {
  const rawSession = url.searchParams.get('session') ?? activeLANSessionId ?? 'unknown'
  // The session id is reflected into the page's inline JS. Restrict it to a
  // safe character set so a crafted ?session= value cannot break out of the
  // string literal and inject script (reflected XSS).
  const sessionId = /^[A-Za-z0-9._-]{1,64}$/.test(rawSession) ? rawSession : 'unknown'
  const html = `<!DOCTYPE html>
<html lang="en"><head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>QAAT Attendance</title>
  <style>body{font-family:system-ui;max-width:400px;margin:40px auto;padding:16px;text-align:center}</style>
</head><body>
  <h2>Attendance Check-in</h2>
  <p>Scan or upload your QR code from <strong>this phone</strong>:</p>
  <input type="file" accept="image/*" id="qr" capture="environment">
  <div id="result" style="margin-top:20px;font-size:18px;font-weight:700"></div>
  <script>
    // Compute a device fingerprint ON THE STUDENT'S PHONE so the bind / one-device
    // checks reflect this handset, not the Coordinator's (F-01).
    async function sha256Hex(s){
      const b = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(s));
      return Array.from(new Uint8Array(b)).map(x=>x.toString(16).padStart(2,'0')).join('');
    }
    async function fingerprint(){
      const c = document.createElement('canvas'), ctx = c.getContext('2d');
      ctx.textBaseline='top'; ctx.font='14px Arial'; ctx.fillText('QAAT-fingerprint-probe',2,2);
      const canvasHash = await sha256Hex(c.toDataURL());
      return sha256Hex([navigator.userAgent, screen.width+'x'+screen.height,
        String(screen.colorDepth), String(window.devicePixelRatio),
        String(navigator.hardwareConcurrency), canvasHash].join('|'));
    }
    document.getElementById('qr').addEventListener('change', async e => {
      const file = e.target.files[0];
      if (!file) return;
      const el = document.getElementById('result');
      el.textContent = 'Checking…'; el.style.color = '#334155';
      const fd = new FormData();
      fd.append('qr', file);
      fd.append('session_id', '${sessionId}');
      fd.append('fingerprint', await fingerprint());
      const res = await fetch('/submit', { method: 'POST', body: fd });
      const data = await res.json();
      el.textContent = data.status === 'PRESENT' ? '✓ Checked in' : '✗ ' + (data.reason ?? 'Rejected');
      el.style.color = data.status === 'PRESENT' ? '#166534' : '#b91c1c';
    });
  </script>
</body></html>`
  return new Response(html, { headers: { 'Content-Type': 'text/html' } })
}

// ─── Student submission relay ────────────────────────────────────────────────
// The SW can't run the full validator (no DOM / session context). It
// relays the submitted QR image to the main thread over a BroadcastChannel and
// returns the validation result the main thread sends back.

const validationChannel = new BroadcastChannel('qaat-lan-validation')
const pendingValidations = new Map<string, (r: { status: string; reason?: string }) => void>()

validationChannel.addEventListener('message', (event) => {
  const data = event.data
  if (data?.type === 'QAAT_LAN_RESULT' && pendingValidations.has(data.requestId)) {
    pendingValidations.get(data.requestId)!({ status: data.status, reason: data.reason })
    pendingValidations.delete(data.requestId)
  }
})

function requestValidation(
  sessionId: string,
  image: ArrayBuffer,
  fingerprintHash: string,
): Promise<{ status: string; reason?: string }> {
  const requestId = crypto.randomUUID()
  return new Promise((resolve) => {
    const timer = setTimeout(() => {
      pendingValidations.delete(requestId)
      // No main-thread session is listening (PWA closed or no ACTIVE session).
      resolve({ status: 'REJECTED', reason: 'SESSION_NOT_ACTIVE' })
    }, 8000)
    pendingValidations.set(requestId, (r) => {
      clearTimeout(timer)
      resolve(r)
    })
    validationChannel.postMessage({
      type: 'QAAT_LAN_VALIDATE', requestId, sessionId, image, fingerprintHash,
    })
  })
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

async function handleStudentSubmit(req: Request): Promise<Response> {
  let form: FormData
  try {
    form = await req.formData()
  } catch {
    return jsonResponse({ status: 'REJECTED', reason: 'INVALID_REQUEST' }, 400)
  }

  const file = form.get('qr')
  const sessionId = String(form.get('session_id') ?? activeLANSessionId ?? '')
  // The fingerprint is measured on the student's phone and travels with the
  // submission so the validator binds THIS handset, not the Coordinator's.
  const fingerprintHash = String(form.get('fingerprint') ?? '')
  if (!(file instanceof Blob) || !sessionId || !fingerprintHash) {
    return jsonResponse({ status: 'REJECTED', reason: 'INVALID_REQUEST' }, 400)
  }

  const image = await file.arrayBuffer()
  const result = await requestValidation(sessionId, image, fingerprintHash)
  return jsonResponse(result)
}
