# QAAT — Stakeholders, Users & Audiences

Everyone who touches the Quality Assurance Attendance Tracking system: who they are, what they see,
and how they get in.

> **On credentials.** This file documents **default / seeded** credentials and the *mechanism* each
> group authenticates with. It contains no real user passwords — those are bcrypt hashes in the
> database and cannot be read back. Every default below is a **first-login credential that must be
> changed**. See [§7 Credential hygiene](#7-credential-hygiene).

Roles are the values of the `user_role_enum` type in Postgres (13 as of migration 064). A user's
role is carried in their RS256 JWT and enforced by `RequireRole` at the API gateway.

---

## 1. At a glance

| # | Stakeholder | Role enum | Where they work | How they sign in |
|---|---|---|---|---|
| 1 | Institution IT administrator | `ADMIN` | Admin dashboard | Email + password |
| 2 | Vice-Chancellor | `VC` | VC dashboard | Email + password **+ TOTP** |
| 3 | Deputy Vice-Chancellor | `DVC` | VC dashboard | Email + password |
| 4 | Director of Quality Assurance | `DQA_DIRECTOR` | DQA dashboard | Email + password **+ TOTP** |
| 5 | QA Officer | `QA_OFFICER` | QA dashboard | Email + password |
| 6 | QA School Handler | `QA_SCHOOL_HANDLER` | QA school dashboard | Email + password |
| 7 | QA Department Rep | `QA_DEPT_REP` | QA department dashboard | Email + password |
| 8 | QA Patroller | `QA_PATROLLER` | **KIU QAAT Patrol** (Android) | Staff ID / email + password |
| 9 | Dean | `DEAN` | Dean dashboard | Email + password |
| 10 | Head of Department | `HOD` | HOD dashboard | Email + password |
| 11 | Course Coordinator | `COORDINATOR` | **KIU QAAT** (Android) | Password or coordinator code |
| 12 | Lecturer | `LECTURER` | Lecturer dashboard / gate scan | Passwordless (staff ID) or password |
| 13 | Student | `STUDENT` | Student portal / KIU QAAT app | Reg-no (portal) or password (app) |
| — | Employee (non-teaching staff) | *no account* | Biometric tablet | Fingerprint / staff ID |
| — | Standby coordinator | borrows `COORDINATOR` | KIU QAAT (Android) | One-day code + reg-no |

**MFA is mandatory for `VC` and `DQA_DIRECTOR` only** (`models.Role.MFARequired()`), bypassable in
development with `DISABLE_MFA=true`.

---

## 2. Governance & oversight

> **The platform-owner role is gone.** `SUPER_ADMIN` was removed in migration 064 — from the code,
> the route guards and the `user_role_enum` type. QAAT is a single-institution deployment: the
> institution's own `ADMIN` performs every administrative action, and `RequireOwnTenant` now has no
> cross-tenant exemption at all.

### 2.1 Vice-Chancellor — `VC` · Deputy Vice-Chancellor — `DVC`

Institution-wide executive view: attendance overview, lecturer workload, ghost-lecture count, and a
30-day PDF audit report. Read-only — they change no records.

- **Dashboard:** `/vc` → Overview · Lecturer Workload · Lecturer Attendance · Student Attendance
- **Credentials:** issued by the institution ADMIN. **VC additionally enrols TOTP MFA** and must
  supply a 6-digit code at every login. DVC does not.

### 2.2 Director of Quality Assurance — `DQA_DIRECTOR`

Owns the quality bar: sets the attendance threshold, check-in window and auto-kill timer, and reads
every QA report. Broadcasts to QA staff by department or school.

- **Dashboard:** `/dqa/thresholds` → Thresholds · Eligibility · Course Health · Trends ·
  Punctuality · Lecturer Attendance · Student Attendance · QA Reports · Messages
- **Credentials:** issued by the ADMIN. **TOTP MFA is mandatory.**

---

## 3. Quality-assurance field staff

All four QA roles below share one inbox: the DQA's `ALL_QA` / `DEPARTMENT` / `SCHOOL` broadcasts
reach whichever of them match, and all of them reply up to the DQA.

### 3.1 QA Officer — `QA_OFFICER`

Institution-wide QA operations: watches live sessions, corrects attendance by hand, resets a
student's device binding, and reissues QR codes.

- **Dashboard:** lands on `/qa/reports` → Live Sessions · QA Reports · Timetable · Student Attendance ·
  Lecturer Attendance · Manual Correction · Coordinator Health · Device Reset · Messages
- **Account requirement:** a **department is mandatory** on the account.

### 3.2 QA School Handler — `QA_SCHOOL_HANDLER`

Oversees quality across **one school/college**. Sees every department beneath it, which have filed
reports and which have not, and files reports of their own.

- **Dashboard:** `/qa-school` → School Overview · Lecturers · QA Reports · Messages
- **Account requirement:** a **college/school is mandatory** — it is the scope of everything they
  see. The API rejects creating this role without one.

### 3.3 QA Department Rep — `QA_DEPT_REP`

The QA ambassador inside **one department**. Uploads the monitoring workbook they already fill in by
hand; recognised rows become teaching observations and the workbook is kept as the evidence.

- **Dashboard:** `/qa-dept` → My Department · File QA Report · Messages
- **Account requirement:** a **department is mandatory.**
- **Scope guard:** a workbook row naming a unit outside their own department is refused, and a QA
  patroller's live field observation is never overwritten by a spreadsheet filled in afterwards.

### 3.4 QA Patroller — `QA_PATROLLER`

Walks room to room and records whether the timetabled lecturer is actually teaching. Works offline
and syncs when a network returns.

- **App:** **KIU QAAT Patrol** (`ug.qaat.patroller`, `KIU QAAT Patrol.apk`) — a separate Android app
  from the coordinator's, installable alongside it.
- **Sign-in:** `POST /api/v1/auth/app-login` with staff ID or email + password. Accounts are created
  by the ADMIN with a `staff_id`.
- **No web dashboard** — patrollers work entirely from the phone.

---

## 4. Academic management

### 4.1 Dean — `DEAN`

Oversees **one school/college**: the lecturers teaching in any of its departments and how much of
their timetabled teaching was actually observed. Can notify those lecturers, or message the DQA/ADMIN.

- **Dashboard:** `/dean` → School Lecturers
- **Account requirement:** a **college/school is mandatory.**

### 4.2 Head of Department — `HOD`

The same, narrowed to **one department**.

- **Dashboard:** `/hod` → Department Lecturers
- **Account requirement:** a **department is mandatory.**

### 4.3 Institution IT Administrator — `ADMIN`

Runs the institution's data: staff accounts, schools & departments, courses, cohorts, timetable,
students, lecturers, employees, rooms, branding and the academic period. Creates **every other
account in the institution.**

- **Dashboard:** `/admin` → Home · Administration · Schools & Departments · Courses & Sessions ·
  Timetable · Students · Coordinators · Lecturers · Assignments · Lecturer Attendance · Employees ·
  Reports · Rooms & Codes · Settings
- **Extra gate:** the Administration (Users) page sits behind a per-tenant **Users passcode** on top
  of the login, configured in Settings.
- **Constraint:** every account they create must use the institution's email domain.

---

## 5. Delivery staff

### 5.1 Course Coordinator — `COORDINATOR`

Runs the actual session: opens the gate, displays the rotating room code, admits the lecturer, and
closes the session. Works offline; the phone is the hub.

- **App:** **KIU QAAT** (`KIU QAAT.apk`). *(The former coordinator PWA has been removed from the
  repository — the Android app supersedes it.)*
- **Known gap:** the web login still redirects `COORDINATOR` to `/coordinator`, a route that no
  longer exists in `admin-dashboards`. A coordinator who signs in on the web lands on a dead route
  and is bounced to `/login`. They should use the Android app; worth either restoring a landing page
  or telling them plainly at sign-in.
- **Sign-in, either:**
  - Email + password (`/api/v1/auth/login`)
  - Unified app login by coordinator code (`/api/v1/auth/app-login`)
- **Issued identifier:** a unique **coordinator code**, auto-generated at account creation and shown
  to the ADMIN once — give it to the coordinator.

### 5.2 Standby coordinator *(a student, temporarily)*

If a coordinator is absent, they — and only they — may pre-authorise a student **of their own
cohort** to run that day's sessions.

- **Sign-in:** `POST /api/v1/auth/coordinator-standby-login` with a **one-day code + registration
  number**. Mints a `COORDINATOR` token under the absent coordinator's identity, scoped to that one
  cohort and expiring at end of day.
- Revocable at any time by the coordinator who issued it.

### 5.3 Lecturer — `LECTURER`

Teaches, and proves presence by scanning the coordinator's gate QR — an HMAC-signed token the
coordinator's screen displays, which is separate from the retired personal-QR logins and still in
use. Has a read-only dashboard of their own attendance.

- **Dashboard:** `/lecturer` → My Attendance
- **Sign-in, either:**
  - **Passwordless:** institution + staff ID (`/api/v1/auth/lecturer-login`)
  - Password via the unified app — **default `Lecturer`** for accounts that have never signed in
    (migration 052), with a forced change at first login (migration 053)
- **Also:** a read-only **Lecturer Portal** needing no account at all — institution + staff ID.
- **Optional:** WebAuthn phone biometric, enrolled once, verified per gate scan.

---

## 6. End users & non-account audiences

### 6.1 Student — `STUDENT`

Checks in to lectures and watches their own eligibility. **Identified by registration number only** —
students have no institutional email or password requirement.

- **Portal:** the student portal — enter a registration number, see attendance and eligibility.
  Passwordless (`/api/v1/student/progress`).
- **Check-in:** signed in through the KIU QAAT app, then `/api/v1/student/checkin` with the room
  code shown on the coordinator's screen. *(The public captive-portal check-in and the personal-QR
  login were removed with the QR subsystem — migration 063.)*
- **Unified app:** password **default `Student`** for accounts that have never signed in
  (migration 052), forced change at first login (migration 053). The device is bound once at
  onboarding via `/api/v1/student/register-device`.
- **One device, one student:** a hardware fingerprint prevents a phone being reused by a second
  student in the same session.

### 6.2 Employees / non-teaching staff — *no login*

Finance, ICT, library, admissions and other support staff. They **do not have dashboard accounts**.
They check in on a biometric tablet, and the system reports their no-shows to QA each morning and
notifies them by email + WhatsApp.

- Managed by the ADMIN under **Employees**.
- Support departments live under a department of `kind = 'SUPPORT'`.

### 6.3 Reporting audiences — *recipients, not users*

People who receive output without signing in:

- **Lecturers and students** receiving in-app notifications from a coordinator, HOD, dean or QA rep.
- **Employees** receiving no-show emails / WhatsApp messages.
- **Exam boards** consuming eligibility exports and clearance tokens.
- **The DQA** receiving QA-rep workbooks and their parsed observations.

---

## 7. Credential hygiene

### Defaults shipped by the system

| Who | Default | Set by | Must change |
|---|---|---|---|
| Students (never signed in) | password `Student` | migration 052 | Forced at first login (053) |
| Lecturers (never signed in) | password `Lecturer` | migration 052 | Forced at first login (053) |
| Test seed users (**dev only**) | `Test1234!` | `db/seeds/002_test_users.sql` | Never load in production |

Migrations 052/053 only touch accounts with `last_login_at IS NULL`, so a password the user has
already changed is never clobbered.

### Rules the system enforces

- Every account must use the **institution's email domain** (a sub-domain is allowed).
- Passwords are **bcrypt, cost 12**; minimum 8 characters.
- **MFA (TOTP) is mandatory for `VC` and `DQA_DIRECTOR`.**
- Account lockout after repeated failures; unknown accounts get a constant-time dummy bcrypt so the
  login endpoint cannot be used to enumerate users.
- Login is rate-limited **per IP** — 5 requests/second sustained, burst 60
  (`PublicIPRateLimit(perSec, burst)`).
- Every org-scoped role (`HOD`, `DEAN`, `QA_DEPT_REP`, `QA_SCHOOL_HANDLER`, `QA_OFFICER`) **cannot be
  created without its department or school** — that field is the scope of everything they see, and
  an account without one matches nothing rather than everything.
- Self-service **Change password** is available from every dashboard sidebar.

### Who creates whom

```
ADMIN (the institution's own administrator) creates every account:
    VC · DVC · DQA_DIRECTOR · QA_OFFICER · QA_SCHOOL_HANDLER · QA_DEPT_REP ·
    QA_PATROLLER · DEAN · HOD · COORDINATOR · LECTURER · STUDENT · (employees: no account)

There is no tier above ADMIN. The institution (tenant) is provisioned at deploy time.
```

---

## 8. Access surfaces

| Surface | Who uses it | Source |
|---|---|---|
| Admin dashboards (web) | ADMIN, VC, DVC, DQA_DIRECTOR, QA_OFFICER, QA_SCHOOL_HANDLER, QA_DEPT_REP, DEAN, HOD, LECTURER | `frontend/admin-dashboards` |
| Student portal (web) | Students (passwordless) | `frontend/student-portal` |
| **KIU QAAT** (Android) | Coordinators, lecturers, students, standby coordinators — including student check-in | `frontend/coordinator-android` (`:app`) |
| **KIU QAAT Patrol** (Android) | QA patrollers | `frontend/coordinator-android` (`:patroller`) |
| Biometric tablet | Employees | external device → employee attendance ingest |

All web traffic reaches one public service, the **api-gateway**, which verifies the JWT, resolves the
tenant and enforces the role. Clients never talk to Postgres.

---

## 9. Tenancy

Every stakeholder is confined to **one institution (tenant)** — without exception. Isolation is
enforced in the database by row-level security keyed on `app.current_tenant`, which the gateway sets
from the JWT's `tenant_id` on every request. The gateway strips any inbound `X-Tenant-ID`,
`X-User-ID` or `X-Role` header before setting its own, so a client cannot assert its own identity.
