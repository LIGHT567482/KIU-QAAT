# QAAT Coordinator — Native Android app

Replaces the coordinator PWA. The phone is the in-room hub: an embedded **Ktor** server + **Room/SQLite**
in a **foreground service** serves the student `/attend` and lecturer `/gate` pages over its own
Wi-Fi hotspot, validates locally, stores attendance, and syncs sealed sessions to the cloud when
online. It **inherits all super-admin/tenant-admin config** via the existing daily manifest
(`GET /api/v1/manifest/daily`). Spec: `coordinator's app.md`. Plan + decisions:
`~/.claude/plans/goofy-meandering-peacock.md`.

> Students keep **no app** — they scan their QR (camera app opens the URL → room-code form) for
> check-in, and use the separate reg-no **progress portal** to view attendance. The dashboards are
> untouched. This is purely the coordinator client swap.

## Status
- ✅ **Phase 0 (crypto) DONE + verified** — [`crypto-core/`](crypto-core/): a Kotlin-sealed package is
  accepted `SYNCED` by the live `sync-receiver`; RSA-2048 QR verify + roster hash match.
- ✅ **Phase A/B/D engine DONE + verified** — [`engine/`](engine/): the **full student check-in chain**
  (PRESENT + all 9 rejection paths), the **lecturer gate** (START/END + student-quorum + biometric +
  every rejection), the **rotating room code** (matches the Go server byte-for-byte), the **session
  package builder** (the exact attendance JSON the server applies — also proven to write live), and the
  **value-add analytics** (chronic-absentee detection + attendance trends) — all **pure JVM, compiled
  and tested off-device** against real signed QRs and the live backend. Run everything:
  `crypto-core/verify.sh` (**9 proofs**, self-bootstrapping toolchain).
- ✅ **Offline-sync round-trip proven + a real backend bug fixed** — a Kotlin-sealed package with a real
  64-char student hash now writes a real attendance row via the live `sync-receiver` (server resolves
  hash→reg-no; see project memory / build_status).
- ⏳ **Phase A shell + Phases B–E** — the Android `:app` module is scaffolded **and wired**: login
  (→ token + device binding key), daily-manifest pull (config inheritance), **open-session flow**
  (`SessionController` → `server.setLive` + rotating-code ticker), live-roster/absentee/trends/audit
  Compose screens, hotspot service, sync client, and a debug cert/network config. **Build-ready in
  Android Studio** (set `API_BASE` to your backend) — **not compilable in this repo's CI env** (no
  Android SDK / no device); expect minor compile touch-ups. One backend prerequisite for phone-hub
  *sync* (central session-upsert) is documented in [BUILD_AND_TEST.md](BUILD_AND_TEST.md).

## Module layout
```
crypto-core/  Kotlin/JVM — HKDF device key, AES-GCM sealer, RSA QR verify   ✅ verified
engine/       Kotlin/JVM — CheckinValidator, LecturerGate, RoomCode, SessionManager,
              SessionPackage, ChronicAbsentee, AttendanceTrends, Store iface          ✅ verified
app/          Android — Ktor server (check-in, gate, /announce SSE), Room (Store impl +
              sessions/roster/present-display), hotspot, foreground service,
              manifest/sync clients, and the Compose UI:                              ⏳ Android Studio
                ui/CoordinatorApp (bottom-nav) · SessionScreen (live roster + room
                code + announce) · AbsenteeScreen · TrendsScreen (Canvas chart) ·
                SyncAuditScreen · di/Graph + data/Repository (bridge Room→engine)
```
The verified `engine` makes every security decision; `app` is the Android wiring around it (Room
implements `engine.Store`, Ktor's `/submit` calls `CheckinValidator`, sync uses `crypto-core.Sealer`).

## Honest limits on stock Android (target = stock, non-rooted)
- Hotspot holds **~10 phones**; the OS generates the SSID/passphrase (no custom "QAAT-SESSION" name).
- The app **cannot deauth/MAC-block** a client. "Kick" = closing the HTTP socket; the **Wi-Fi slot
  frees only when the student disconnects** — the served check-in page already shows the big
  "turn Wi-Fi OFF now" screen. Big rooms = **rotation over time**, an external AP, or parallel
  coordinators. (spec §3.1 custom SSID and §5 auto-kick require device-owner/root — out of scope.)

## Phase 0 — also do the device spike (go/no-go, on a real phone)
Before the big build, prove the radio/UX on real hardware:
1. A throwaway Android app: `WifiManager.startLocalOnlyHotspot()` + a Ktor "hello" server bound to the
   hotspot IP.
2. **≥8 student phones** join and load the page; confirm the camera-app→URL→room-code flow + the
   self-signed/no-cert "proceed" UX on iOS + Android.
3. Measure real concurrency and how voluntary rotation behaves with ~30–50 phones.
→ If the radio/UX can't sustain it, stop and use the laptop hub + external AP instead.

## Build phases (each a milestone)
- **A** — Kotlin/Compose scaffold + foreground service hosting Ktor + Room (schema per spec §10);
  session state machine + auto-expiry (T+120/T+180); daily manifest sync → SQLite (+ <24h fallback);
  hotspot manager + Wi-Fi join QR.
- **B** — `/attend` + `POST /submit` (Kotlin validation chain reusing `crypto-core`), `/gate`,
  `/status`; live roster; HTTP-close "kick". Reuse the served HTML from the gateway's
  `checkin_page.go` / `lecturer_gate.go` (incl. the rotation screen).
- **C** — seal on close (`crypto-core` `Sealer`) → chunked upload to `/api/v1/sync/*`; sync audit + SYNC_OVERDUE.
- **D** — announcements (SSE), trend graphs, chronic-absentee, notifications inbox. (Two-way
  notification/escalation needs new backend + admin-dashboard work → deferred.)
- **E** — cutover (app becomes the coordinator client) + docs + flowchart re-render.

## Prerequisites to build
Android Studio + Android SDK (API 34+) + JDK 17. The **hotspot/rotation/multi-phone** parts need a
**real Android phone** (emulators can't make a real hotspot) — but the server, check-in, sync, and UI
are all **emulator-testable**: see [EMULATOR_TESTING.md](EMULATOR_TESTING.md) (from scratch, incl. the
`10.0.2.2` networking + the two small dev wirings). Device testing: [../../docs/DEVICE_TESTING.md](../../docs/DEVICE_TESTING.md).

## Branding (multi-tenant / white-label)
- **Runtime branding (implemented):** one shared app **launcher icon** for everyone; **after the
  coordinator logs in**, the app calls `GET /api/v1/branding` and adopts the **tenant's logo + colours**
  app-wide — branded `MaterialTheme` (`brand_color`) + a top bar showing the institution's logo/name
  (`ui/Brand.kt`, `net/BrandingClient.kt`). The logo is decoded from a `data:` base64 image (most
  tenants); for remote `https` logos, add Coil's `AsyncImage` in `BrandLogo`.
- **True per-tenant launcher icon (optional, "white-label"):** Android can't change the launcher icon
  at runtime, so a per-tenant icon = a **per-tenant APK** built by the admin/CI. Easiest path: a
  **Gradle product flavor per tenant** (or a small `whitelabel.sh`) that swaps `res/mipmap*/ic_launcher`
  + the app label (and optionally bakes a default `API_BASE`/institution hint), then `assembleRelease`
  produces `qaat-<tenant>.apk` for the admin to distribute. Trade-off: one APK + update per institution.
  The runtime branding above already covers the in-app experience, so white-label is only needed if the
  **home-screen icon itself** must differ per institution.

## Feature 5 (notifications) backend
Spec ready to implement: [../../docs/NOTIFICATIONS_BACKEND.md](../../docs/NOTIFICATIONS_BACKEND.md)
(migration + endpoints + RLS + app polling/outbox). It's the one feature that also touches the admin
dashboard, so it's deferred to a separate, confirmed pass.

## Verify the crypto foundation now (no device needed)
```bash
cd crypto-core && KOTLINC=<kotlinc> JAVA_HOME=<jdk21> ./verify.sh
```
See [`crypto-core/README.md`](crypto-core/README.md). Device-level testing: [`../../docs/DEVICE_TESTING.md`](../../docs/DEVICE_TESTING.md).
