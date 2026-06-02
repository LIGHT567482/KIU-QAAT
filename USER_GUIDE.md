# QAAT — User Guide & Honest System Assessment

_Last updated: 2026-06-02. This guide is deliberately blunt about what works, what
doesn't, and how much load it can take. Where a number is an estimate rather than
a measured result, it says so._

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
- **Student** — has a personal, cryptographically signed QR code (emailed to them).
  Checks in from their own phone. No app to install.
- **Coordinator** — runs the live session: opens it, shows the rotating room code
  on the projector, closes it.
- **Lecturer** — presence is recorded (anti-ghost-lecture).
- **QA Officer / DQA Director / VC** — oversight, thresholds, reports, eligibility.
- **Admin** — tenant and user management.

### 2.2 The check-in flow (the part redesigned 2026-06-02)

> **Why it changed.** The original design had each student's phone connect to a
> small server running on the coordinator's phone over local WiFi. We proved this
> cannot work in the real world: iPhones can't run that kind of server, a phone
> hotspot only holds ~8–10 devices (a lecture hall has 300), and the browser
> security features the page needs (camera, crypto) are blocked on plain-WiFi
> connections. So live check-in now runs **online over HTTPS**, which works
> identically on every iPhone and Android. The full reasoning is in
> `update.md` and the plan file `~/.claude/plans/async-napping-petal.md`.

What happens, step by step:

1. **Coordinator opens the session** (one tap). The server creates the session and
   a secret, and starts producing a **6-digit room code that changes every 15
   seconds**.
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
| 5 | **Rotating room code** (replaces the old Bluetooth check) | Checking in from outside the room — you must see the live screen |
| 6 | Device fingerprint binding | One phone checking in several students |
| 7 | Duplicate check | Checking in twice |
| 8 | One device per session | A shared "proxy phone" |

**Why the rotating code instead of Bluetooth?** A student's web browser cannot read
Bluetooth signal strength (iPhones block it entirely). The rotating code is the
practical equivalent: because it changes every 15 seconds and is only shown on the
room's screen, possessing the current code is proof you're physically there. A
relayed code is stale within seconds.

### 2.4 Other things the system does
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

### ✅ Verified working today (live, end-to-end)
- The whole stack boots and is healthy (Postgres, Redis, auth, gateway, QR
  generator, notifications, student portal, mail).
- **Coordinator login** → JWT issued (after fixing two real bugs, see §6).
- **Open session** → returns a session id and a live rotating code.
- **Student check-in with a genuine RSA-signed QR → `PRESENT`**, and one
  append-only row written to the ledger.
- **Every rejection path returns the correct reason**: wrong code →
  `PROXIMITY_FAILED`; second attempt → `DUPLICATE_SCAN`; a different phone for an
  already-bound student → `DEVICE_MISMATCH`; tampered QR → `INVALID_SIGNATURE`.
- The cross-language signature check (QR signed by the Node service, verified by
  the Go gateway) matches **byte-for-byte** — proven by an automated test, because
  this is the easiest thing to get subtly wrong.
- The rotating-code maths (generate, validate, clock-skew window, reject stale
  codes) — automated unit tests pass.

### ⚠️ Built but not yet verified end-to-end
- **Coordinator's on-screen display** (the page that shows the rotating code and
  countdown) is **not built yet** — the *server endpoints it needs* are built and
  tested, but the screen itself is the next task.
- **Student page on a real phone**: the page is served and its logic is standard,
  but I tested the API directly with a signed QR, not yet by photographing a QR on
  an actual iPhone/Android. (It relies on standard browser features that work on
  current iOS/Android over HTTPS.)
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
cd /home/profy/Desktop/QAAT
docker compose -f infra/docker-compose.yml --env-file .env up -d   # start everything
# Load test data (first time only):
docker compose -f infra/docker-compose.yml --env-file .env exec -T postgres \
  bash -c 'psql -U qaat -d qaat -f /seeds/001_test_tenants.sql && \
           psql -U qaat -d qaat -f /seeds/002_test_users.sql'
```

Ports (note: some were remapped because your machine already uses the defaults):
Gateway `:8443`, Auth `:8081`, Postgres `:5434`, Redis `:6380`, QR `:3012`,
Student portal `:3003`, Mail UI `:8025`. Test login: `coordinator@alpha.edu` /
`Test1234!` / tenant `a0000000-0000-0000-0000-000000000001`.

The student check-in page is served at `GET /checkin?session=<id>`.

---

## 5. Capacity — how big a number can it handle? (honest)

> **Read this carefully:** the only *measured* fact today is that single check-ins
> succeed. The throughput figures below are **engineering estimates** derived from
> the work each check-in does, not from a load test. Treat them as "what the design
> should support if hardware and tuning are reasonable", and confirm with the k6
> load tests before relying on them.

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
