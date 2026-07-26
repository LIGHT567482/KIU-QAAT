# Testing the QAAT Coordinator app in Android Studio (from scratch)

> ## ⚠️ What the emulator CAN and CANNOT do
> A Wi‑Fi hotspot needs a real radio. **`WifiManager.startLocalOnlyHotspot()` fails on the emulator**,
> so you **cannot** test the hotspot, the ~10‑client cap, the manual‑rotation "kick", or LAN‑proximity
> on the emulator — those need **≥2 physical phones** (see [../../docs/DEVICE_TESTING.md](../../docs/DEVICE_TESTING.md)).
>
> The emulator IS perfect for everything else: the **embedded Ktor server**, the **check‑in validation**,
> **Room** storage, the **daily‑manifest sync** (config inheritance), the **sealed upload** to your
> backend, **announcements (SSE)**, and the whole UI. The app is built so the **server starts even if the
> hotspot fails**, so it runs fine on the emulator.

---

## 0. Install (once)
- **Android Studio** (latest) → SDK Manager: install **Android 14 (API 34)** platform + build‑tools.
- Create an **AVD**: Device Manager → Pixel 7, system image **API 34** (x86_64). Start it.
- You also need the QAAT backend running on your machine: `./setup.sh` (or `docker compose -f infra/docker-compose.yml up -d`).

## 1. The one networking fact: `10.0.2.2`
From inside the emulator, **`10.0.2.2` = your host machine's `localhost`**. So:
- The app reaches the QAAT **gateway** at `https://10.0.2.2:8443` (set this as the API base — see §3).
- The app's **own embedded server** runs at `http://127.0.0.1:8080` *inside the emulator* — reachable
  from the **emulator's own Chrome** and (from your laptop) via `adb forward tcp:8080 tcp:8080`.
- Other devices **cannot** reach the emulator's server (NAT) — that's why multi‑phone needs real hardware.

## 2. Open + sync the project
`File ▸ Open ▸ apps/coordinator-android`. Gradle sync downloads AGP 8.5 / Kotlin 2.0 / Compose / Ktor /
Room. Modules: `:crypto-core` + `:engine` (plain Kotlin/JVM, already verified) and `:app` (Android).

## 3. Two small dev wirings to make the emulator useful
Both are normal app work (the engine they call is already verified):

**(a) API base + trust the dev cert.** Point the manifest/sync clients at the host and accept the
self‑signed cert in debug:
- Build with `VITE`‑style base `https://10.0.2.2:8443` (pass to `ManifestClient`/`SyncClient`).
- Add `app/src/debug/res/xml/network_security_config.xml` trusting a cleartext/user‑CA dev host, and
  reference it from the debug manifest — OR configure the Ktor `HttpClient(CIO)` to trust all certs in
  debug only. (The gateway cert is self‑signed; this is a dev‑only allowance.)

**(b) A "dev: open session" action.** Until the real session‑open screen exists, add a debug button that
hydrates an `ActiveSession` and calls `SessionService.server.setLive(...)`, e.g.:
```kotlin
// pull the manifest for a test coordinator, pick a unit, build the session context
val raw = ManifestClient("https://10.0.2.2:8443", coordinatorJwt).fetchRaw()   // inherits config
// …parse roster/policy/publicKey/hashKey (kotlinx.serialization against manifest.go)…
val session = ActiveSession(sessionId, tenantId, year, pubKeyPem, hashKey, rosterHashes, rosterSerials)
val secret  = /* per-session room-code secret */ "dev-secret".toByteArray()
SessionService.server.setLive(InRoomServer.Live(session, secret,
    gateContext = { LecturerGateContext(staffId, secret, GateState.NOT_STARTED, attended, enrolled, 0.5, false) },
    onGate = { /* update SessionManager + persist gate-open/close */ }))
```
Without this, `/submit` correctly returns `SESSION_NOT_ACTIVE`.

## 4. Run the verified engine tests (no emulator needed)
The logic is plain JVM — run it straight from Gradle:
```bash
# the authoritative proof (9 checks incl. live backend round-trip):
apps/coordinator-android/crypto-core/verify.sh
```
(Optionally fold the `engine/src/test` mains into JUnit so `./gradlew :engine:test` runs them in Studio.)

## 5. Drive the in‑room flow on the emulator
1. Run `:app` on the AVD (`Shift+F10`). Grant Location / Nearby‑Wifi / Notifications when asked.
2. Tap **Start session** → the foreground service starts; the hotspot attempt **fails silently on the
   emulator** (expected) but the **Ktor server is up on :8080** and the room code shows in the app.
3. Trigger your **dev open‑session** (§3b) so a session is live.
4. In the **emulator's Chrome**, open the student page against the embedded server:
   `http://127.0.0.1:8080/attend?qr=<url-encoded signed QR JSON>`
   (get a real signed QR from the running `qr-generator`, or your test fixtures). Type the room code
   shown in the app → **Confirm** → you should see **PRESENT** + the "turn Wi‑Fi off" rotation screen.
   Try the rejection cases (wrong code, second student same "device", etc.) — they mirror the verified engine.
5. **Sync:** close the session → the app seals the package and uploads to `https://10.0.2.2:8443/api/v1/sync/*`.
   Verify on the host: `docker exec infra-postgres-1 psql -U qaat -d qaat -c "SELECT student_id FROM attendance_logs WHERE session_id='<id>'"` → you'll see the **reg‑no** (server resolves the hash).
6. **Announcements:** `curl` or a dev button → `POST http://127.0.0.1:8080/announce` (form `type`,`message`)
   → the open `/attend` page shows the coloured banner (SSE).

## 6. Inspect things
- **Logcat** for the service + Ktor logs. **App Inspection ▸ Database Inspector** to view Room
  (`attendance_logs`, `device_bindings`, `roster`).
- `adb forward tcp:8080 tcp:8080` then hit `http://localhost:8080/status` from your laptop to see the
  live room code + active flag.

## 7. When you're ready for the real thing
Hotspot start, the ~10‑client cap, manual rotation, LAN‑proximity, and cross‑device check‑in are **only
testable on physical phones** — follow [../../docs/DEVICE_TESTING.md](../../docs/DEVICE_TESTING.md)
(cert trust per device, concurrency ramp, rotation timing, failure paths).
