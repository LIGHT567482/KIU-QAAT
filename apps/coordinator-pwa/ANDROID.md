# QAAT Coordinator — Native Android app

> ## ⚠️ SUPERSEDED — see [`../coordinator-android/README.md`](../coordinator-android/README.md)
> The owner chose a **full native Kotlin** app (Ktor + Room + Compose), **not** the Capacitor wrapper
> described below. The current plan, status, and the **verified Phase 0 crypto-parity** live under
> [`apps/coordinator-android/`](../coordinator-android/). This file is kept only as the earlier
> Capacitor exploration; the hotspot/foreground-service ideas below still inform the native build,
> but ignore the Capacitor-specific steps.

---

Goal: let a coordinator run a session from a **phone** instead of a laptop. This wraps the existing
React coordinator PWA in a native shell (Capacitor), adds a **Wi-Fi hotspot + foreground service**,
and (later) an **on-device server** so the phone is a true offline hub.

> ## ⚠️ Read first — the honest limits (these are OS/radio walls, not bugs)
> - A **stock Android hotspot holds ~8–10 phones**. Native code cannot raise this.
> - A non-rooted app **cannot deauthenticate/kick** clients. So freeing a slot is **voluntary**:
>   each student turns Wi-Fi **off** after they see ✓. The check-in screen + coordinator dashboard
>   already say this loudly (the "manual rotation" model).
> - Therefore **"one phone, one room, ~1000 students" is rotation-over-time, not concurrent** — plan
>   for tens-of-minutes and consider an external router/AP for big halls (see
>   [../../docs/DEVICE_TESTING.md](../../docs/DEVICE_TESTING.md)).
> - **This repo's CI machine has no Android SDK and no device**, so the steps below are run by *you*
>   in Android Studio on a real phone. Nothing here is built/verified in the QAAT Docker stack.

---

## Prerequisites
- **Android Studio** (latest) + **Android SDK** (API 34+) + **JDK 17**.
- A **real Android phone** (emulators can't make a real hotspot) with USB debugging on.
- Node 20 + pnpm (already used by this app).

## Phase 2 — Native shell + managed hotspot (talks to a LAN gateway)
This delivers a real installable app with a one-tap hotspot and the rotation UX, while still using a
running QAAT **gateway** on the LAN (a laptop, or another machine) for the actual check-in logic.

### 2.1 Add Capacitor and the Android project
```bash
cd apps/coordinator-pwa
pnpm add -D @capacitor/cli
pnpm add @capacitor/core @capacitor/android @capacitor-community/keep-awake
# capacitor.config.ts already exists in this folder.
# Point the web bundle at your running gateway on the LAN:
VITE_API_URL=https://10.42.0.1:8443 pnpm build
npx cap add android        # generates apps/coordinator-pwa/android/
npx cap sync android
npx cap open android        # opens Android Studio
```

### 2.2 Permissions — `android/app/src/main/AndroidManifest.xml`
Add inside `<manifest>`:
```xml
<uses-permission android:name="android.permission.ACCESS_FINE_LOCATION"/>
<uses-permission android:name="android.permission.NEARBY_WIFI_DEVICES" android:usesPermissionFlags="neverForLocation"/>
<uses-permission android:name="android.permission.FOREGROUND_SERVICE"/>
<uses-permission android:name="android.permission.FOREGROUND_SERVICE_CONNECTED_DEVICE"/>
<uses-permission android:name="android.permission.POST_NOTIFICATIONS"/>
```

### 2.3 Hotspot plugin — `android/app/src/main/java/ug/qaat/coordinator/HotspotPlugin.kt`
Starts a **LocalOnlyHotspot** and surfaces its SSID/passphrase so the coordinator can read it out.
(LocalOnlyHotspot is the only hotspot a normal app may start; it has no internet uplink, which is
exactly what we want for offline.)
```kotlin
package ug.qaat.coordinator

import android.content.Context
import android.net.wifi.WifiManager
import com.getcapacitor.JSObject
import com.getcapacitor.Plugin
import com.getcapacitor.PluginCall
import com.getcapacitor.PluginMethod
import com.getcapacitor.annotation.CapacitorPlugin

@CapacitorPlugin(name = "Hotspot")
class HotspotPlugin : Plugin() {
    private var reservation: WifiManager.LocalOnlyHotspotReservation? = null

    @PluginMethod
    fun start(call: PluginCall) {
        val wifi = context.applicationContext.getSystemService(Context.WIFI_SERVICE) as WifiManager
        wifi.startLocalOnlyHotspot(object : WifiManager.LocalOnlyHotspotCallback() {
            override fun onStarted(res: WifiManager.LocalOnlyHotspotReservation) {
                reservation = res
                val cfg = res.softApConfiguration
                val out = JSObject()
                out.put("ssid", cfg.ssid ?: "")
                out.put("passphrase", cfg.passphrase ?: "")
                call.resolve(out)
            }
            override fun onFailed(reason: Int) { call.reject("hotspot failed: $reason") }
        }, null)
    }

    @PluginMethod
    fun stop(call: PluginCall) {
        reservation?.close(); reservation = null; call.resolve()
    }
}
```
Register it (Capacitor auto-discovers `@CapacitorPlugin` classes via annotation processing; if using
the manual path, add it in `MainActivity.onCreate` with `registerPlugin(HotspotPlugin::class.java)`).

### 2.4 Foreground service (keep the hotspot + session alive when screen sleeps)
Add a small `Service` with an ongoing notification: *"QAAT session running — tell students to turn
Wi-Fi off after ✓."* Start it when a session opens, stop it on close. This both keeps the process
alive and reinforces the **manual-rotation** message natively. (Standard Android foreground-service
boilerplate; type `connectedDevice`.)

### 2.5 Call it from the React app
The rotation UX already ships in the PWA (the coordinator dashboard banner + the served check-in
page's "Turn Wi-Fi OFF now" screen). From a session-open handler:
```ts
import { registerPlugin } from '@capacitor/core'
const Hotspot = registerPlugin<{ start(): Promise<{ssid:string; passphrase:string}>; stop(): Promise<void> }>('Hotspot')
// onOpenSession: const { ssid, passphrase } = await Hotspot.start(); // show these for students to join
// onCloseSession: await Hotspot.stop()
```

### 2.6 Build + run on a device
```bash
VITE_API_URL=https://<gateway-ip>:8443 pnpm build && npx cap sync android
# In Android Studio: Run ▶ on the connected phone.
```
**Verify on the phone:** open session → hotspot starts, SSID shown → a second phone joins → student
checks in → sees the "turn Wi-Fi off" screen → disconnects → next phone connects. Confirm the
~10-device ceiling and time a rotation (this is the real test — see DEVICE_TESTING.md).

---

## Phase 3 — On-device server + SQLite (the true phone hub) — SPIKE FIRST
Only this removes the laptop. It means **porting the in-room server** onto the phone, because today
the PWA is a *client* of the Go gateway (`src/pages/SessionPage.tsx`, `src/sync/outbox.ts`).

### 3.1 De-risking spike (do before committing to the port)
Build a throwaway that proves the unknowns on a real device:
1. LocalOnlyHotspot up + a tiny embedded HTTP server (**Ktor** or **NanoHTTPD**) serving a hello page.
2. **≥8 student phones** join and load that page over the hotspot IP.
3. Confirm the **camera-app → URL → room-code form** flow works (no in-browser camera needed) and the
   self-signed/no-cert "proceed" UX is acceptable on iOS + Android.
4. Measure real concurrency + how voluntary rotation behaves with ~30–50 phones.
→ **Go/no-go.** If the radio/UX can't sustain it, stop here and use the laptop hub + external AP.

### 3.2 The port (if the spike passes)
Embedded server (Ktor) + **Room/SQLite** reimplementing only the in-room endpoints — mirror the Go
handlers so behaviour matches exactly:
- `sessions/open`, `sessions/{id}/close`, `sessions/{id}/checkin-code` (rotating code),
- student check-in gates: **RSA QR verify**, roster lookup, room code, **one-device-one-person**,
- lecturer gate (start/end + quorum),
- **seal** a closed session = AES-256-GCM + HMAC-SHA256 + SHA-256 checksum, mirroring
  [`src/sync/sealer.ts`](src/sync/sealer.ts) so the existing **`sync-receiver`** accepts it unchanged,
- serve the **student/lecturer check-in pages** (reuse the HTML from the Go handlers
  `checkin_page.go` / `lecturer_gate.go`).
- **Pre-session (online):** pull the daily manifest (roster, tenant keys, session window) into SQLite.
- **Post-session (online):** upload sealed packages to the central `sync-receiver`.

This is multi-week. Keep the crypto + gate rules byte-for-byte compatible with the Go/TS originals so
a phone-hub session and a laptop-hub session are indistinguishable to the central server.

---

## What is and isn't verified
- **Verified in the QAAT stack:** the manual-rotation UX (the served check-in "turn Wi-Fi off" screen
  + coordinator dashboard banner) — these ship today and the app reuses them.
- **NOT verifiable here:** anything in `android/` (no SDK/device in this environment). Build + test on
  a real phone per the steps above and `docs/DEVICE_TESTING.md`.
