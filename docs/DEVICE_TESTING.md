# QAAT — Real-device testing guide (do this before any external use)

The server is easy to test; the **room** is not. The things that break in the field are the
**Wi-Fi radio capacity**, the **self-signed certificate**, and **iOS quirks** — none of which a
unit test or the k6 load test covers. Walk these steps with real phones first.

> **The one fact to internalise:** a phone hotspot holds ~**10** devices, a laptop hotspot ~**20–40**,
> and a stock phone **cannot kick** anyone off. So students must **turn Wi-Fi off after ✓** to free a
> slot. Test that the rotation actually happens — it's the whole ballgame for big rooms.

## 0. Bring up the hub
- **Laptop hub (recommended for testing):** `./setup.sh`, then start the hotspot
  (`sudo nmcli device wifi hotspot ifname wlan0 ssid QAAT-Attendance password qaat12345`).
  Laptop is `10.42.0.1`.
- Have an admin create a tenant + a coordinator + a unit/offering + at least 2–3 test students with QRs.

## 1. Cert trust (the #1 first-contact failure)
- On each test phone, join `QAAT-Attendance`, open `https://10.42.0.1:3000` (and `:8443/health`).
- You'll get a security warning (self-signed). Tap **Advanced ▸ Proceed** — once per port.
  Better: install/trust `infra/certs/qaat.crt` on the device.
- ✅ Pass = the page loads and the gateway answers. ❌ If camera/crypto features fail, the cert/secure
  context is the cause.

## 2. Single-device end-to-end (one phone)
1. Coordinator opens a session (laptop/PWA) → room code appears + counts down.
2. Lecturer phone: scan the coordinator's gate QR → staff ID + live code (+ fingerprint) → **START**.
3. Student phone: open their **own** QR with the camera → check-in page opens → type the room code →
   **✓ You're marked present** + the **"Turn Wi-Fi OFF now"** screen with the countdown appears.
4. Lecturer scans again → **END** (needs the student quorum).
5. Coordinator closes the session → confirm it seals and, once the laptop is online, syncs (a row
   reaches the central DB; eligibility updates).

## 3. iOS + Android matrix
- Repeat step 2 on **Safari (iPhone)** and **Chrome (Android)**.
- Confirm the **camera app** opens the QR link (this is the OS camera, not in-browser) and the
  room-code form submits.
- iOS **Private Wi-Fi Address**: on by default; fine here (we don't rely on MAC), but note it.

## 4. Concurrency ramp — find YOUR real ceiling
- Add phones to the hotspot: **5 → 10 → 20 → 40**. After each step, have them all check in.
- Record where it breaks: phones that **can't associate** (hotspot full), slow page loads, timeouts.
- That number is your **true per-AP capacity** — not the server's req/s. Write it down per device
  used as the hub.

## 5. Rotation behaviour (the big-room test)
- Fill the hotspot to its cap, have everyone check in, and **watch whether students actually turn
  Wi-Fi off** when told. Time how long a fresh batch takes to cycle through.
- Extrapolate honestly: `students ÷ slots × seconds_per_student` ≈ total time. If it's unacceptable,
  you need more radio (external AP/router) or parallel coordinators — not more code.

## 6. Failure paths (must all behave)
- **Off-network device** (on mobile data, not the hotspot) → `NOT_SAME_NETWORK`.
- **Two students on one phone** in a session → second is `DEVICE_ALREADY_USED`.
- **Stale/old room code** → rejected; current code works.
- **Hub goes offline mid-session** (airplane-mode the laptop's uplink, keep the hotspot) → check-ins
  **still record locally**; closing seals the package; it syncs when the uplink returns.
- **Wrong person's QR** → the page shows the name + a warning; coordinator can refuse.

## What this guide does NOT prove
- It does not prove 1000-at-once: no single AP does that. Prove your *rotation time* and *per-AP cap*
  instead, then decide between rotation, more APs, or campus Wi-Fi.
- The k6 load test (`tests/load`) stresses the **server**, which is not the bottleneck — don't mistake
  a green k6 run for "the room scales".
