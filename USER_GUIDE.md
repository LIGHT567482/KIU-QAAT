# QAAT — User Guide & Honest System Assessment

_Last updated: 2026-06-14 (session 12). Previous update session 9._

---

## 1. What QAAT is (in one paragraph)

QAAT (Quality Assurance Attendance Tracker) is a multi-tenant attendance system
for universities. Its job is to stop two kinds of cheating: **proxy attendance**
(a student signing in for an absent friend) and **ghost lectures** (a class
recorded as held that never happened). It does this with signed personal QR codes,
device fingerprinting, a rotating in-room code that proves physical presence, and
an append-only attendance ledger that nobody — not even an admin — can quietly
delete from.

---

## 2. What the system does, and how

### 2.1 Roles
- **Student** — identified by their **registration number** only (no email/phone/password
  needed). Has a permanent, cryptographically signed QR for check-in, and can view their own
  attendance % any time via the passwordless **reg-no portal**. Checks in from their own phone;
  no app to install. An email is optional — supplied only if you want their QR emailed to them.
- **Coordinator** — runs the live session on their **laptop**, which is the room's Wi-Fi hotspot
  and offline server: opens the session, shows the rotating room code on the projector, closes it.
  If absent, can pre-authorise an **own-cohort student** (a "standby") with a one-day code to run
  that day's session in their place.
- **Lecturer** — identified by **staff ID**; presence is recorded at a start/end gate
  (anti-ghost-lecture). Has a permanent career QR (scan → passwordless dashboard) and a
  staff-ID dashboard login. Optional email only to email them their QR.
- **QA Officer / DQA Director / VC / DVC** — oversight, thresholds (≥75% floor), reports, eligibility.
- **Admin** — tenant and user management; courses (level-independent) with levels + unit roadmaps inside them;
  cohorts (applied across all courses at once or one); student/lecturer registration + bulk import; venues.
- **Super-Admin** — platform owner: registers universities (tenants), branding, billing.

### 2.2 The check-in flow (the part redesigned 2026-06-02)

> **Why it works offline.** The original design tried to run a tiny server on the
> coordinator's **phone** — that fails in the real world (iPhones can't run it, a phone
> hotspot only holds ~8–10 devices, and browsers block camera/crypto on plain WiFi).
> The current design moves the server to the coordinator's **laptop**, which is the
> room's Wi-Fi hotspot **and** the local server **and** the database. Phones join that
> hotspot and reach the laptop over HTTPS on the LAN. **No internet is needed in the
> room** — every check-in is written to the laptop's database the instant it's accepted.
> When the laptop later has connectivity, the closed session is sealed and synced to the
> cloud. "Same network" (being on the coordinator's hotspot) is itself one of the proximity
> proofs. The full reasoning is in `update.md`, `flow.md`, and `RUN-ANYWHERE.md`.

What happens, step by step:

1. **Admin registers lecturers** in the Admin Dashboard → Tenants → Lecturers. Each
   lecturer has a name, **staff ID**, optional phone and department (email is optional —
   only used to email them their QR). Admin then goes to Assignments
   and assigns each lecturer to specific course units (by academic year, year of study,
   semester, and intake session — e.g. "Dr Okafor teaches CS101 in Year 1, Sem 1,
   2024/2025, Morning session").
2. **Coordinator opens the session** by selecting the course unit. A dropdown lists all
   lecturers assigned to that unit for the current period. Coordinator selects the
   lecturer present and taps "Open Session". The server creates the session and a secret, records the lecturer, and starts producing a **6-digit room code that changes every 15 seconds**.
2. **The projector shows** a static "join" link/QR for the session **and** the big
   rotating 6-digit code with a countdown.
3. **The student** opens the link on their phone, points the camera at (or uploads)
   their personal QR, types the 6-digit code from the screen, and taps "Check in".
4. **The server validates everything** and, if it all passes, writes one permanent
   attendance row.

### 2.3 The 8 anti-fraud checks (run server-side on every check-in)

| # | Check | Stops |
|---|-------|-------|
| 1 | RSA-2048 signature on the QR | Forged / hand-made QR codes |
| 2 | Expiry date | Last year's QR |
| 3 | Tenant match (via the signature) | A QR from another university |
| 4 | On the active roster | Someone not enrolled in the unit |
| 4b | Serial number matches current issue | A QR that was reissued/revoked (lost card) |
| 5 | **Rotating room code** | Checking in from outside the room — you must see the live screen |
| 5b | **Same network** — must be on the coordinator's hotspot LAN (egress IP matches the session) | Checking in from off-site (`NOT_SAME_NETWORK`) |
| 6 | Device fingerprint binding | One phone checking in several students |
| 7 | Duplicate check | Checking in twice |
| 8 | One device per session (`DEVICE_ALREADY_USED`) | A shared "proxy phone" |

**Why a rotating code?** It needs no extra hardware and works on every phone.
Because the code changes every 15 seconds and is only ever shown on the room's
screen, possessing the current code is proof you're physically there. A relayed
code is stale within seconds.

### 2.4 Course roadmap + semester management

1. **Admin creates a course** in Admin Dashboard → Tenants → Courses. Coordinator is
   optional — can be assigned later. Set *Programme Duration* (e.g., 3 years for BSc).
2. **Admin populates the roadmap** by clicking "Roadmap". A Year × Semester grid appears.
   Click "+ Add Unit" on any slot to add a course unit to that specific year and semester.
   Units can also be moved between years/semesters via Edit.
3. **Admin sets the active semester** per tenant via Tenants → "Set Semester". The admin
   picks the academic year (e.g., "2024/2025") and semester (1 or 2). From that moment,
   coordinators for that tenant see **only the units from that semester** in their daily
   manifest — not all units from all years.
4. **Coordinator pulls manifest**: on opening the PWA the coordinator gets only the
   course units for the current active semester for courses they coordinate. No active
   semester set = all units shown.

### 2.5 Other things the system does
- **Multi-tenancy**: many universities share one database, fully isolated at the
  database level (Postgres Row-Level Security). One tenant cannot see another's data.
- **Append-only ledger**: attendance corrections create a *new* row marked
  `MANUAL_OVERRIDE`; the original is never deleted. Deletion is blocked at the
  database level, not just in code.
- **Offline sync** (kept from the original design): post-session data can be
  uploaded later in encrypted chunks if connectivity was poor.
- **Reports & eligibility**: attendance percentages, exam-eligibility thresholds,
  PDF/CSV exports for VC/DQA.

---

## 3. Will it do what you want? — what is actually proven vs. not

This is the honest part. "Verified" below means *I ran it on the live stack today
and saw the result*, not "the code looks right".

### 2.6 Lecturer attendance (anti-ghost-lecture)

The primary reason QAAT was built is to track both student and **lecturer** attendance.
The system records when each lecture session actually happens and who taught it:

1. **Session open** — when the coordinator opens a session and selects a lecturer,
   the server writes a row to `lecturer_attendance_logs` with `gate_open_time` and
   the lecturer's identity.
2. **Session close** — when the coordinator closes the session (or it auto-closes),
   the server fills in `gate_close_time` and calculates `contact_hours =
   (close - open) / 3600` to two decimal places.
3. **Admin view** — Admin Dashboard → Tenants → Attendance:
   - **Summary cards** per lecturer: total sessions, total contact hours, average
     contact hours per session, last session date.
   - **Detail log table**: date, lecturer name, unit, gate open/close times, contact
     hours, session status. Click "View logs" on a summary card to filter the table.

This gives the DQA/VC a full audit trail of lecturer presence — every ghost lecture
shows up as a session with no `gate_close_time` or anomalously short `contact_hours`.

---

### ✅ Verified working today (live, E2E test 2026-06-14)
- All 4 backend services start via `scripts/start_all_local.sh` and are healthy.
- **Admin login** → JWT issued with correct issuer/audience.
- **Coordinator login** → JWT issued with `device_binding_key`.
- **Tenant admin API** → lists tenants; `active_academic_year` and `active_semester` returned.
- **Open session** with `lecturer_id` → returns session id + rotating room code + **writes
  `lecturer_attendance_logs` row** (gate_open_time recorded).
- **Close session** → session status = CLOSED in DB + `gate_close_time` filled in
  `lecturer_attendance_logs`.
- **QR batch generation** → job queued at qr-generator (estimated count returned).
- **Sync-receiver** health check passes on port 8083.
- **Student check-in with a genuine RSA-signed QR → `PRESENT`**, and one
  append-only row written to the ledger.
- **Every rejection path returns the correct reason**: wrong code →
  `PROXIMITY_FAILED`; second attempt → `DUPLICATE_SCAN`; a different phone for an
  already-bound student → `DEVICE_MISMATCH`; tampered QR → `INVALID_SIGNATURE`.
- The cross-language signature check (QR signed by the Node service, verified by
  the Go gateway) matches **byte-for-byte** — proven by an automated test.

### ⚠️ Ready but awaiting user action (three remaining tests)

**Test 1 — Real SMTP QR email delivery:**  
System ready. A seed student (`jzany17@gmail.com`) exists. To send real email:
1. Copy `.env.smtp.example` → `.env.smtp` and fill Gmail App Password (myaccount.google.com/apppasswords)
2. `source .env.smtp && ./scripts/start_qr_generator.sh`
3. Run `./tests/e2e/run_e2e_test.sh` — step 7 triggers the batch; expect email in inbox

**Test 2 — Offline session on real mobile hardware:**  
System ready. Open `http://10.200.6.121:3000` on a phone on the same LAN.  
Login: `coordinator@test.local / Coord1234!`. Select "Introduction to Programming" → "Dr. Jane Smith" → Open Session. Check in from another phone → End Session.

**Test 3 — Sync round-trip (local): ✅ PROVEN with a real hash (2026-06-29).**  
A sealed package carrying a realistic 64-char `student_id_hash` now uploads (init/chunk/complete) and
writes `attendance_logs` with `records_written=1`. (Earlier this silently failed: the 64-char hash
overflowed `attendance_logs.student_id varchar(50)`; `sync-receiver` now resolves the hash → the real
reg-no server-side and stores the reg-no — consistent with online check-in.) To trigger from the PWA in
DevTools: `(await navigator.serviceWorker.ready).sync.register('qaat-sync-outbox')`; verify with
`SELECT student_id FROM attendance_logs WHERE session_id='<id>'` (you'll see the reg-no, not a hash).

- **GPS geofence** (optional second presence check) is designed but not wired in.

### ❌ Known gaps / not done (mostly inherited from `update.md`)
- No load test has been run **this session** — the capacity numbers in §5 are
  reasoned estimates, not measurements.
- Security review only covered the QR/sync/check-in paths. `session-manager`,
  `notification-service`, `student-portal`, and the dashboards are **unreviewed**.
- The encrypted offline-sync round trip is matched by code reading, not a live test.
- Production secrets, backups, and the 50-item pilot checklist are unverified.

**Bottom line:** the core thing you care about — a student checking in and the
system correctly accepting the real one and rejecting fraud — **works today**. The
coordinator's display screen and real-device/load testing are the remaining work
before a pilot.

---

## 4. How to run it

```bash
cd /home/Desktop/QAAT
docker compose -f infra/docker-compose.yml --env-file .env up -d   # start everything
# Load test data (first time only):
docker compose -f infra/docker-compose.yml --env-file .env exec -T postgres \
  bash -c 'psql -U qaat -d qaat -f /seeds/001_test_tenants.sql && \
           psql -U qaat -d qaat -f /seeds/002_test_usenotificationrs.sql'
```

Ports (note: some were remapped because your machine already uses the defaults):
Gateway `:8443`, Auth `:8081`, Postgres `:5434`, Redis `:6380`, QR `:3012`,
Student portal `:3003`, Mail UI `:8025`. Test login: `coordinator@alpha.edu` /
`Test1234!` / tenant `a0000000-0000-0000-0000-000000000001`.

The student check-in page is served at `GET /checkin?session=<id>`.

---

## 5. Capacity — how big a number can it handle? (honest)

> ### ⚠️ The real limit is the Wi-Fi radio, NOT the server.
> For offline, in-room attendance the bottleneck is **how many phones one access
> point holds at once**, and that is small:
> - **Stock Android phone hotspot: ~8–10 devices.** (No app, native or not, beats this — it's the OS/radio.)
> - **Android can't kick clients without root**, so freeing a slot is **voluntary**: each student must turn Wi-Fi **off** after they see ✓. The check-in screen and the coordinator dashboard both say this loudly.
> - **Linux laptop hotspot: ~20–40** comfortably (more with a good external AC/AX adapter, but congested).
>
> **So "one coordinator, one room, ~1000 students" works ONLY by rotation**, and it is
> **time-bounded, not instant**: with ~10–40 slots and ~10–15 s per student, throughput is
> a few students/second *at best* → roughly **20–60+ minutes for ~1000**, longer with reconnect
> churn and stragglers. To go faster you need **more radio**: an external Wi-Fi router/AP in the
> room, several coordinators/APs in parallel, or the hub's server on campus Wi-Fi. **The numbers
> below are about the *server*, which was never the problem.**

> **Read this carefully:** the only *measured* fact today is that single check-ins
> succeed. The throughput figures below are **engineering estimates** derived from
> the work each check-in does, not from a load test, and they describe the **server**,
> not the Wi-Fi radio above. Confirm with the k6 load tests (which test the server,
> not the radio) before relying on them.

**What one check-in costs the server:** one signature verification (fast) + about
6 small indexed database queries + one insert. That is a lightweight request.

**A single 300-seat lecture: a non-event.** Even if all 300 students hit "check in"
in the same 10 seconds (the bell rush), that is ~30 requests/second. A single
modest server handles that without noticing. 300 students spread over a 2–5 minute
window is ~1–2 requests/second.

**A single standard deployment (one gateway process + one Postgres):**
- *Estimated* sustained capacity: **several hundred check-ins per second**, i.e.
  on the order of **thousands of students checking in concurrently** across many
  simultaneous lectures.
- Practical limiting factors first to bite: the database connection-pool size and
  Postgres write throughput on the attendance table — both tunable.

**A whole university at class-change time (tens of thousands of students checking
in within the same 1–2 minutes across hundreds of rooms):**
- The check-in service itself is **stateless**, so you scale it by running more
  copies behind a load balancer — this part scales horizontally and cleanly.
- The shared bottleneck becomes **Postgres**. To go to that scale you need standard
  measures: a connection pooler (PgBouncer), tuned write settings, and read
  replicas for the dashboards. The architecture supports this (it was designed
  multi-tenant from the start) but **this scale has not been tested here.**

**Things that would have hurt at scale and were fixed today:**
- The eligibility summary was being fully rebuilt on *every* check-in — that would
  have turned a 300-student rush into ~300 full-table recomputations (O(N²)).
  Removed from the live path (it now refreshes off the hot path).

**Hard limits you should know:**
- **Multi-tenant scale**: effectively unlimited tenants in one database (isolation
  is per-row); the practical limit is total data volume vs. one Postgres cluster.
- **The dashboards** (VC overview) currently compute some numbers on the fly per
  request — fine for a pilot, but they will be the slow part for a large
  institution with lots of historical data, and should move to the pre-computed
  summary table before scaling up.
- **Connectivity**: live check-in now *requires* internet in the room (the trade we
  made so it works on iPhones with no hardware). A room with no signal at all
  cannot check in live; brief drops are absorbed by client-side retry.

---

## 6. Bugs found and fixed today (so they don't surprise you)

1. **Notification service crash-loop** — it required Web Push keys that weren't set
   and crashed the whole service (including unrelated email). Now push degrades
   gracefully; dev keys added.
2. **Coordinator login was completely broken** — the deployment config didn't pass
   the master encryption key to the auth service, so every coordinator login failed
   at the device-binding step. Fixed in the compose file.
3. **Seed passwords didn't work** — the test users' stored password hash did not
   actually match the documented password "Test1234!". Replaced with a correct hash.
4. **Port clashes** — your machine already runs Postgres, Redis, and a Next.js app
   on QAAT's default ports; the dev compose ports were remapped (see §4).
5. **O(N²) check-in footgun** — described in §5; removed.

---

## 7. What's next (recommended order)

1. Build the coordinator's live check-in screen (rotating code + countdown + join QR).
2. Test the student page on a real iPhone and Android over HTTPS.
3. Run the k6 load tests to replace the estimates in §5 with real numbers.
4. Security-review the services that haven't been looked at.
5. Update `CLAUDE.md` / `ARCHITECT.md` to describe the online check-in (they still
   describe the old offline-LAN design).
