# QAAT — Testing the whole system with real students & devices (from zero)

This is the end‑to‑end runbook: bring up the **backend**, verify the **database**, drive the
**frontends**, hand QR codes to **real students**, and run a **real attendance session** on real
phones — then confirm the data. It is written for a **pilot on one Linux laptop** that is both the
server and the room Wi‑Fi.

> **Laptop‑hub vs phone‑hub — and where the PWA fits.** In *this* guide the **server + database is the
> laptop's Docker stack**; the **PWA is only the coordinator's browser screen** (a client of that
> backend) — the PWA is *not* the server. That's why it works here even though a PWA can't itself be a
> server. The **native Android app** is the *other* mode: it makes the **phone** the server (so no
> laptop). Students' experience is identical either way. Use this laptop‑hub path for your **first**
> real‑student test (least to go wrong); to test with the **app as the hub**, build it first and follow
> [../apps/coordinator-android/BUILD_AND_TEST.md](../apps/coordinator-android/BUILD_AND_TEST.md).

---

## 0. The mental model (what you're testing)
```
   ┌─────────────────────────── ONE LINUX LAPTOP ───────────────────────────┐
   │  Docker stack:  api-gateway · auth · qr-generator · sync-receiver       │
   │                 postgres · redis · caddy (HTTPS) · mailhog              │
   │  + it is also the room's Wi-Fi HOTSPOT  (laptop = 10.42.0.1)            │
   └───────────────▲───────────────────────────▲───────────────────────────┘
                   │ join Wi-Fi "QAAT-Attendance"│
        ┌──────────┴─────────┐        ┌──────────┴──────────┐
        │ Student phones     │        │ Lecturer phone      │
        │ (scan own QR →     │        │ (scan coordinator   │
        │  captive page)     │        │  gate QR)           │
        └────────────────────┘        └─────────────────────┘
   Coordinator runs the PWA in a browser (on the laptop, or a phone on the hotspot).
```
**Golden rule:** taking attendance is **fully offline** — phones reach the laptop over the hotspot;
nothing needs the internet. Internet is only needed once up front (to pull/build) and later if you
sync to a *separate* central server (not needed for a one‑laptop pilot — the laptop IS the database).

You will test, in order: **Backend → Database → Frontends (config) → QR delivery → Offline session
with real phones → Verify results.**

---

## 1. Backend — bring it up and prove it's healthy
On the laptop (internet on for this step):
```bash
cd QAAT
./setup.sh          # builds images, starts everything, runs migrations + seeds the platform super-admin
```
When it finishes it prints the URLs + the super‑admin login. **Verify each service:**
```bash
# Gateway health (200 = up)
curl -sk https://localhost:8443/health -w '\n%{http_code}\n'

# All containers should be "Up"
docker compose -f infra/docker-compose.yml ps          # (or: docker ps)

# Auth service
curl -s http://localhost:8081/health
```
Ports (browser, on the laptop): Coordinator PWA `https://localhost:3000` · Admin `:3001` ·
Super‑Admin `:3002` · Student portal `:3003` · Mailhog (test emails) `http://localhost:8025`.
> First visit to each `https://` URL shows a self‑signed‑cert warning → **Advanced ▸ Proceed**.

**✅ Backend pass:** gateway returns `200`, all containers `Up`, the four UIs load.

---

## 2. Database — how to look inside and verify
The DB is Postgres in the `infra-postgres-1` container (user `qaat`, db `qaat`).
```bash
# Open a SQL shell
docker exec -it infra-postgres-1 psql -U qaat -d qaat

# Handy one-liners (-t -A = clean output):
docker exec infra-postgres-1 psql -U qaat -d qaat -c "\dt"                       # list tables
docker exec infra-postgres-1 psql -U qaat -d qaat -c "SELECT name FROM tenants;" # your institutions
docker exec infra-postgres-1 psql -U qaat -d qaat -c "SELECT student_id,full_name FROM students_extended LIMIT 5;"
docker exec infra-postgres-1 psql -U qaat -d qaat -c "SELECT COUNT(*) FROM attendance_logs;"
```
The tables you'll watch during a test: `tenants`, `students_extended`, `lecturers`, `course_units`,
`course_offerings` (cohorts), `sessions`, **`attendance_logs`** (the result), `lecturer_attendance_logs`.

> **Multi‑tenant isolation:** every row has a `tenant_id` and Row‑Level Security enforces it. To query
> as the app does, the gateway sets the tenant per request; in `psql` you're the superuser so you see
> all — fine for verification.

**✅ DB pass:** you can open `psql`, list tables, and read your tenant/students.

---

## 3. Frontends — configure the institution (super‑admin → admin)
1. **Super‑Admin** (`https://localhost:3002`): log in with the printed creds
   (`superadmin@qaat.platform` / `Super1234!` — change it). Create your **institution (tenant)**:
   name, Institution ID, branding, and its **Admin** user.
2. **Admin dashboard** (`https://localhost:3001`): log in as that admin (email + institution ID).
   Set up, in this order:
   - **Course** → add **levels** inside it → add **units** per level/year/semester.
   - **Cohort/offering** (session · year · semester · level · intake), assign a **coordinator**.
   - **Lecturers** (staff ID; optional email/phone) and **assign** them to units.
   - **Students** — register by **registration number** (bulk import a CSV is fastest; an optional
     email lets the system email them their QR).
   - Confirm the **attendance threshold** (≥75%) and the **daily session window**.

**✅ Frontend pass:** the admin lists show your courses, units, cohort, lecturers, and students.

---

## 4. Get QR codes to the students (and the lecturer)
Each student has **one permanent QR** (encodes a signed token + a URL to the check‑in page); each
lecturer has **one permanent career QR**.
- **Get them:** Admin → Students → **Show QR** (display/print), or supply emails so the QR is emailed
  (check Mailhog at `:8025` in dev). Same for Lecturers → **QR**.
- **⚠️ Critical for offline:** the QR encodes a **host**. For the offline hotspot it must resolve to
  the laptop — use the `qaat.local` name (setup.sh maps `qaat.local → 10.42.0.1` on the hotspot) or
  the hotspot IP. If your QRs were generated for a different/public host, re‑point first (see
  [RUN-ANYWHERE.md](../RUN-ANYWHERE.md) / the LAN re‑point note) and re‑issue, or the camera will open
  a URL the phones can't reach on the hotspot.
- Tell students to **save the QR image to their phone** (gallery) — no app to install.

**✅ QR pass:** scanning a student QR with a phone camera opens a URL like
`https://qaat.local:8443/checkin?t=…` (it will only load once the phone is on the hotspot — next step).

---

## 5. Go offline — start the room hotspot
On the laptop (Linux + an AP‑capable Wi‑Fi card):
```bash
echo 'address=/qaat.local/10.42.0.1' | sudo tee /etc/NetworkManager/dnsmasq-shared.d/qaat.conf
sudo nmcli device wifi hotspot ifname wlan0 ssid QAAT-Attendance password qaat12345
# stop later with:  sudo nmcli connection down Hotspot
```
Now the laptop is `10.42.0.1` and serves Wi‑Fi **QAAT‑Attendance** (`qaat12345`). Phones that join can
reach `https://qaat.local:3000` etc. **No internet required.**
> **Cert trust:** on each phone, the first HTTPS visit warns about the self‑signed cert →
> **Advanced ▸ Proceed** (once per port), or install `infra/certs/qaat.crt`. The QR scan + room‑code
> form work after that.

**✅ Hotspot pass:** a phone on QAAT‑Attendance can open `https://qaat.local:3000` and `:8443/health`.

---

## 6. Run a REAL session (coordinator + students + lecturer)
1. **Coordinator** opens the PWA (`https://…:3000`), logs in, picks today's **unit**, selects the
   **lecturer**, taps **Open Session**. A big **6‑digit room code** appears and rotates every ~10s.
2. **Lecturer** (phone on the hotspot) scans the **coordinator's gate QR** → enters **staff ID** +
   the **live code** (+ fingerprint if enrolled) → **START**. (This proves a real lecture; without it
   students can't be marked present.)
3. **Students**, in small batches (the hotspot holds ~10 at once — see §8):
   - Join Wi‑Fi **QAAT‑Attendance**, open their **own QR** (camera) → the check‑in page opens.
   - Type the **room code** on the projector → **Confirm** → they see **✓ Present** and a big
     **"turn Wi‑Fi OFF now"** screen → they **disconnect** so the next student can connect.
4. Watch the coordinator's **live roster** fill in (present count, names).
5. **Lecturer** scans again → **END** (counts only if enough students attended — the quorum).
6. **Coordinator** taps **Close session**.

**What to deliberately try (failure paths should be rejected):**
- A phone **not on the hotspot** (mobile data) → `NOT_SAME_NETWORK`.
- **Two students on one phone** in the same session → `DEVICE_ALREADY_USED`.
- An **old/expired** room code → rejected.
- Someone else's QR on your phone → device‑mismatch / proxy rejection.

**✅ Session pass:** present students get ✓, the listed failure cases are blocked, the lecturer
START/END recorded.

---

## 7. Verify the results (database + dashboards)
```bash
# attendance rows for the session (student_id is the REG-NO)
docker exec infra-postgres-1 psql -U qaat -d qaat -c \
 "SELECT student_id, checkin_timestamp, entry_method FROM attendance_logs ORDER BY checkin_timestamp DESC LIMIT 20;"

# lecturer start/end + contact hours
docker exec infra-postgres-1 psql -U qaat -d qaat -c \
 "SELECT lecturer_id, gate_open_time, gate_close_time, contact_hours FROM lecturer_attendance_logs ORDER BY gate_open_time DESC LIMIT 5;"
```
- **Student portal** (`https://…:3003/?org=<your-institution>`): a student types their **reg‑no** and
  sees their attendance % and exam eligibility.
- **Admin / DQA / QA / VC dashboards** (`:3001`): eligibility (≥75%), lecturer attendance/workload,
  per‑unit health, timetable.

**✅ Results pass:** the present students appear in `attendance_logs` (as reg‑nos), the lecturer log
has contact hours, and the dashboards + student portal reflect it.

*(If you deploy a separate central server later, the offline‑sealed sync is what carries a closed
session to it — that round‑trip is proven; for a one‑laptop pilot the data is already in this DB.)*

---

## 8. The capacity reality + a phased rollout (do this, in order)
One Wi‑Fi access point is the real limit — **~10 phones on a stock phone hotspot, ~20–40 on a laptop**.
There is **no auto‑kick on a stock device**, so students must **turn Wi‑Fi off after ✓** to free a
slot (the screen tells them). So run the pilot in stages:

| Stage | Who | What you're proving |
|---|---|---|
| **A. Smoke** | you + **1 phone** | cert trust, QR opens, one full check‑in writes a row, lecturer gate, close |
| **B. Small** | **5–10 phones** | concurrent check‑ins, the live roster, the failure paths |
| **C. Ramp** | **20 → 40 phones** | where association/throughput breaks on *your* hardware = your real per‑AP cap |
| **D. Real class** | a whole class, **batched** | rotation timing (≈ slots ÷ ~12s per student); plan ~ that many minutes |

For a big hall: add an **external Wi‑Fi router/AP** (students join *it*; the laptop is the server on
that LAN — handles 50–150+), or run **several coordinators/APs in parallel**. The Go server handles
thousands; the **radio** is what you're really testing.

Record, per device used as the hotspot: max concurrent phones, time to cycle a batch, any failures.
(See [DEVICE_TESTING.md](DEVICE_TESTING.md) for the device matrix and [PILOT_CHECKLIST.md](PILOT_CHECKLIST.md).)

---

## 9. Troubleshooting
| Symptom | Likely cause → fix |
|---|---|
| Phone can't open `qaat.local`/`10.42.0.1` | Not on the hotspot, or dnsmasq line not set → rejoin Wi‑Fi; re‑run the `address=/qaat.local/` line; use the IP `https://10.42.0.1:3000` |
| "Your connection is not private" | Self‑signed cert → **Advanced ▸ Proceed** (per port), or install `infra/certs/qaat.crt` |
| QR opens a URL that won't load | QR built for the wrong host → re‑point to `qaat.local`/hotspot IP and re‑issue QRs (§4) |
| Camera won't scan / no check‑in page | Use the phone's normal **camera app** to open the QR (not an in‑browser scanner); ensure good lighting |
| `NOT_SAME_NETWORK` for a student in the room | Their phone fell back to **mobile data** → turn data off, ensure they're on QAAT‑Attendance |
| `DEVICE_ALREADY_USED` | That phone already checked in a different student this session (one device = one student) — expected |
| New phones can't connect mid‑session | Hotspot at its ~10 cap → earlier students must **turn Wi‑Fi off** (rotation); or add an external AP |
| No attendance rows after a session | Check the gateway logs `docker logs infra-api-gateway-1`; confirm the session was **ACTIVE** and the code matched |
| Emails (QR) not arriving in dev | Dev uses **Mailhog** — check `http://localhost:8025`, not a real inbox |

---

## TL;DR order
1. `./setup.sh` → verify health (§1) · 2. peek at the DB (§2) · 3. super‑admin → admin sets up
course/cohort/lecturers/students (§3) · 4. issue + distribute QRs, host‑correct (§4) · 5. start the
hotspot (§5) · 6. coordinator opens a session; lecturer START; students scan+code+✓+disconnect;
lecturer END; close (§6) · 7. verify `attendance_logs` + dashboards + reg‑no portal (§7) · 8. ramp
1 → 10 → 40 → a class (§8).
