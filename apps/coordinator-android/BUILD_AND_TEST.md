# From zero: build the coordinator app, then test the whole system WITH it

This is the **phone‑hub** path: the coordinator's Android phone *is* the server + database + Wi‑Fi
hotspot — no laptop in the room, no PWA. It answers two things you asked: **how to build the app**, and
**how to test the system with it** (and why each step exists).

> **Why a build step at all?** The PWA is only a *browser screen*; in the laptop model the laptop's
> Docker backend is the server. To make a **phone** be the server, the logic has to be **compiled into
> an Android app** and installed. That compile + install is the "build" you're asking about. The app's
> security core is already written and proven; you're assembling + installing it, not writing it.

---

## Part 1 — Understand the two modes (so you pick on purpose)
- **Laptop‑hub (PWA):** laptop runs the backend; phones hit it; coordinator uses the PWA. Works today,
  ~20–40 phones/AP. Best for your **first** real‑student test — see [../../docs/SYSTEM_TEST_GUIDE.md](../../docs/SYSTEM_TEST_GUIDE.md).
- **Phone‑hub (this app):** the phone runs everything. No laptop. **~10 phones/AP on a stock phone**,
  students rotate (turn Wi‑Fi off after ✓). This document.
- The **student/lecturer experience is identical** in both. So everything you validate on the laptop
  carries over; the phone‑hub adds only "the phone is the server".

**Recommendation:** do the laptop‑hub test once (de‑risks everything), *then* this. But if you want to
go straight to the app, continue below.

---

## Part 2 — BUILD the app (one‑time, on a dev machine with internet)

### 2.1 Install the tools (each is needed because…)
- **Android Studio** (latest) + **Android SDK API 34** — the compiler/toolchain that turns the Kotlin
  into an installable APK. (I can't produce the APK for you — it needs the SDK + your acceptance of
  its licenses.)
- **JDK 17** (Android Studio ships one) — Kotlin/Android compile target.
- A **real Android phone** with **Developer Options + USB debugging** on — because the **hotspot only
  works on real hardware** (the emulator has no Wi‑Fi radio; emulator is fine for everything *except*
  the hotspot — see [EMULATOR_TESTING.md](EMULATOR_TESTING.md)).

### 2.2 Open + sync the project
`Android Studio ▸ Open ▸ apps/coordinator-android`. Let **Gradle sync** finish — it downloads the
Android Gradle Plugin, Kotlin, Compose, Ktor, Room (this needs internet, once). Modules:
`:crypto-core` + `:engine` (the proven logic) and `:app` (the Android shell).

### 2.3 The wiring is already done — you only set the backend URL
Both pieces are now implemented (login → manifest → open‑session → live roster → close → sync):
- **Networking/cert:** `net/Net.kt` (Ktor client trusts the self‑signed cert in **debug**) +
  `app/src/debug/.../network_security_config.xml` + a debug manifest. **You only set** the backend URL:
  `app/build.gradle.kts` → `buildConfigField API_BASE` (default `https://10.0.2.2:8443` for the emulator;
  use your laptop/server **LAN IP** for a real phone).
- **Login + open‑session:** `LoginScreen` (mirrors the PWA: tenant‑lookup → login → stores the token +
  **device binding key**), `ManifestClient` (pulls + parses `/api/v1/manifest/daily`, caches the roster),
  and `SessionController.open(...)` (builds the `ActiveSession` + room‑code secret, sets the live session
  on the embedded server, starts the rotating‑code ticker) / `close()` (seals + uploads via `SyncClient`).
  `onCheckin` writes the live‑roster row; the analytics screens read from Room.

> **One backend prerequisite for phone‑hub *sync*** (not needed for laptop‑hub, not needed for in‑room
> check‑in): a phone‑generated `session_id` won't exist in the **central** DB, and `attendance_logs`
> has an FK to `sessions`. So the central `sync-receiver` must **create the session from the synced
> package** before inserting rows (or the app must register the session online at open). Until that's
> added, in‑room check‑in + local storage work fully; the *upload* of a brand‑new offline session will
> be rejected centrally. (Laptop‑hub has no such gap — the gateway creates the session.) Ask me to add
> the small `sync-receiver` session‑upsert + I'll verify it live, same as the hash→reg‑no fix.

### 2.4 Compile + install on the phone
- Plug in the phone (USB) → it appears in Android Studio's device dropdown.
- Press **Run ▶**. Studio compiles the APK and installs it. **Expect to fix a few compile nits** (an
  import, a Material3/Ktor API tweak) — that's normal for a scaffold compiled the first time; the
  *logic* is sound, the *presentation* may need small touch‑ups.
- Grant permissions when asked: **Location / Nearby‑Wi‑Fi** (to start the hotspot) and **Notifications**.
- For sharing later: **Build ▸ Build APK** → install the `.apk` on other coordinator phones.

### 2.5 Prove the build is sane before the room (no students yet)
- Launch the app; tap **Start session** → the **foreground service** starts and (on a real phone) the
  **hotspot comes up** with an OS‑generated SSID/password shown on screen.
- Do your **dev open‑session** (2.3.2) so a session is ACTIVE; the room code shows + rotates.
- From a **second phone**, join that hotspot, open the student check‑in URL, scan a test QR, enter the
  code → you should get **✓ Present** and the "turn Wi‑Fi off" screen.
- *(You can also run the proven logic with no phone at all: `crypto-core/verify.sh` — 9 checks.)*

---

## Part 3 — TEST the system with the app (real students)
The backend/DB/admin setup + QR issuance are **the same as the laptop guide** — do
[../../docs/SYSTEM_TEST_GUIDE.md](../../docs/SYSTEM_TEST_GUIDE.md) **§1–§4** first (bring up the cloud
backend once so the phone can pull its manifest; create the institution, course, cohort, lecturers,
students; issue QRs). Then, in the room:

1. **Coordinator phone:** open the app → **Start session** → the app shows the **Wi‑Fi name/password**
   (and a join QR). Students join *that* Wi‑Fi. *Why:* the phone is now the room server + hotspot.
2. **Do the open‑session** (pick unit/lecturer) → the room code rotates.
3. **Lecturer:** scan the coordinator's **gate QR** → staff ID + live code → **START**.
4. **Students, in batches of ≤~10:** join the phone's Wi‑Fi → scan **own QR** → type code → **✓** →
   **turn Wi‑Fi OFF** so the next student can join. Watch the app's **live roster** fill in.
5. **Lecturer:** scan again → **END** (needs the student quorum). **Coordinator:** close.
6. **Sync:** when the phone next has internet, the closed session uploads to the central backend
   (the sealed‑package round‑trip is already proven). Verify rows as in SYSTEM_TEST_GUIDE §7.

**Deliberately test the rejections** (same as laptop): off‑network → `NOT_SAME_NETWORK`; one phone for
two students → `DEVICE_ALREADY_USED`; stale code → rejected.

---

## Part 4 — The honest limits of the phone‑hub (plan around them)
- **~10 students at once** on a stock phone; no auto‑kick → **manual rotation** (turn Wi‑Fi off after
  ✓). A class of 60 ≈ 6 rotations ≈ several minutes; 1000 in one room is **not** a stock‑phone scenario.
- For bigger rooms: have the phone + students join a cheap **external Wi‑Fi router** (the phone is still
  the server on that LAN; the router's radio carries 50–150+), or run **several coordinator phones in
  parallel**, each its own cohort.
- Capacity testing (the ramp 1 → 10 → 40 → class) is in [../../docs/DEVICE_TESTING.md](../../docs/DEVICE_TESTING.md).

---

## TL;DR
**Build once** (Android Studio: open → sync → add the 2 wirings → Run ▶ to install on a phone), then
**test** exactly like the laptop guide except the **coordinator's phone is the hotspot+server** and you
work in **batches of ~10**. Same students, same QRs, same backend setup — only the hub moved from a
laptop to the phone. Do the laptop‑hub run first if you want to de‑risk everything but the phone part.
