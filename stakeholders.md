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
| 8 | QA Patroller | `QA_PATROLLER` | **KIU QAAT** (Android) — Patrol role | Staff ID / email + password |
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

- **App:** **KIU QAAT** (`KIU QAAT.apk`) — the same single app everyone else installs. The token's
  role opens the Patrol experience. *(The separate `ug.qaat.patroller` / `KIU QAAT Patrol.apk` build
  was deleted; there is one APK to distribute, and it runs wherever the main app runs.)*
- **Sign-in:** `POST /api/v1/auth/app-login` with staff ID or email + password. Accounts are created
  by the ADMIN with a `staff_id`.
- **No web dashboard** — patrollers work entirely from the phone.
- **One patroller, one handset.** A patrol record accuses a named lecturer, so the account is bound
  to the first phone that claims it (`patroller_device_bindings`, migration 069) and every patrol
  call must carry that handset's `X-Device-Fingerprint`. A token replayed from another phone is
  refused with `DEVICE_NOT_BOUND`, and one phone cannot serve two patrol accounts. A lost or
  replaced phone is freed by the ADMIN via `DELETE /api/v1/admin/patrol-bindings/{user_id}`, which
  the audit-log middleware records.
- **No silent re-login.** Unlike every other role, the app erases the saved credentials when a
  patroller signs in, so their round can never be resumed on an unattended phone without the
  password being typed again. Signing out also wipes the cached timetable and any queued ticks.

---

## 4. Academic management

### 4.1 Dean — `DEAN`

Oversees **one school/college**: the lecturers teaching in any of its departments and how much of
their timetabled teaching was actually observed. Can notify those lecturers, or message the DQA/ADMIN.

- **Dashboard:** `/dean` → **Overview** · **Departments & HODs** · Lecturers · At-risk Students ·
  Student Attendance · Timetable (read-only) · Alerts
- **Account requirement:** a **college/school is mandatory.**

> **A dean manages through their heads of department, so that layer is now the second page.**
> `Departments & HODs` lists every department of the college with the head who runs it — name,
> contact, and whether they have ever signed in — beside the four figures the department is judged
> on: classes taught against the timetable, whether the **lecturer actually gated in** for the ones
> that ran (the ghost-lecture measure — `sessions` alone only says a room was opened), student
> attendance, and how many students are about to lose eligibility. Each card drills into that
> department's lecturers.

### 4.2 Head of Department — `HOD`

The same, narrowed to **one department**.

- **Dashboard:** `/hod` → **Overview** · Lecturers · At-risk Students · Student Attendance ·
  Timetable (read-only) · Alerts
- **Account requirement:** a **department is mandatory.**

> **Both used to have exactly one page** — a bare lecturer list — which left the two roles
> accountable for a department and a college seeing less than the QA rep who visits them. The
> Overview answers the three questions in order: how big is my unit, is teaching actually
> happening (sessions held against the timetable, plus units with **no lecturer assigned** —
> invisible on every other screen), and who is about to lose exam eligibility.
>
> **Scope is never a parameter.** It is resolved server-side from the unit on the caller's own
> account (`resolveOrgScope`), so a dean cannot read another college by naming it, and an account
> with a blank unit matches **nothing** rather than everything.

### 4.3 Institution IT Administrator — `ADMIN`

Runs the institution's data: staff accounts, schools & departments, courses, cohorts, timetable,
students, lecturers, employees, rooms, branding and the academic period. Creates **every other
account in the institution.**

- **Dashboard:** `/admin` → Home · Administration · Schools & Departments · Courses & Sessions ·
  Timetable · Students · Coordinators · Lecturers · Assignments · Lecturer Attendance · Employees ·
  **At-risk Students** · Reports · Rooms & Codes · **Audit Trail** · Settings
- **Home** is no longer a grid of links. It carries what the institution is *doing* (sessions live
  now, check-ins and lecturer gate-ins today) and — the point of it — **Needs attention**: units
  with no lecturer, cohorts with no coordinator, students on no cohort, org roles with no unit,
  accounts still on the default password, patrollers with no handset, sessions never synced. Every
  one of those is a *silent* failure elsewhere; none of them raise an error anywhere, so the only
  way anyone finds them is by being shown them.
- **Audit Trail** reads `admin_audit_log`, which existed from migration 001 and which **nothing
  ever wrote to** until now. Account deletions, student device resets, patrol handset releases and
  patrol PIN clearances are recorded with the detail that cannot be recovered afterwards (the
  deleted account's email and role, the reason given for a reset).
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
- **Dashboard:** `/lecturer` → **My Teaching** — their own month (taught / missed / still to come,
  from the gate record, not from "a session existed"), then the per-unit student attendance matrix
  split by cohort.
- **Sign-in, either:**
  - **Passwordless:** institution + staff ID (`/api/v1/auth/lecturer-login`)
  - Password via the unified app — **default `lecturer`** for accounts that have never signed in
    (migrations 052/070; either casing is accepted), with a forced change at first login (053)
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
- **Unified app:** sign in with the **registration number** and the password **default `student`**
  for accounts that have never signed in (migrations 052/070; either casing is accepted), forced
  change at first login (053). The device is bound once at onboarding via
  `/api/v1/student/register-device`.
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
| Students (never signed in) | password `student` | migrations 052 + 070, `DefaultStudentPassword` | Forced at first login (053) |
| Lecturers (never signed in) | password `lecturer` | migrations 052 + 070, `DefaultLecturerPassword` | Forced at first login (053) |
| Test seed users (**dev only**) | `Test1234!` | `db/seeds/002_test_users.sql` | Never load in production |

Migrations 052/053/070 only touch accounts with `last_login_at IS NULL`, so a password the user has
already changed is never clobbered.

**Casing does not matter for a default.** 052 seeded `Student`/`Lecturer`; the canonical spelling is
now lower-case. For an account still flagged `force_password_change`, auth-service accepts any
casing of the word it was actually seeded with (`matchesSeededDefault`) — but only after verifying
the stored hash really is that untouched default. The moment the user picks their own password the
flag clears and matching is exact again, like every other account.

**Where the default comes from.** Students and lecturers never choose a password at creation — they
are added by registration number and staff ID — so `DefaultStudentPassword` /
`DefaultLecturerPassword` (`backend/api-gateway/internal/handlers/default_passwords.go`) are seeded
for them, in every path: the admin dashboard, the SIS import, and the lazy provisioning on first
sign-in. Registering a student from the admin dashboard **used to seed a random throwaway** instead
— a leftover from the QR era, when the password was never meant to be typed. The reg-number
resolved, the account existed, and sign-in still failed with "invalid email or password". Migration
070 repairs every account left stranded by that.

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

### Org units are chosen, never typed

Department and school on an account are matched **by name** against `courses.department` /
`courses.school`. One typo — "Comp. Science" against "Computer Science" — and the account sees
nothing at all: the scoped queries return an empty set rather than an error, so it looks like an
empty institution rather than a mistake. Every form that takes them (Users, Lecturers, Employees)
therefore picks from the org units the admin created, via the shared `OrgPicker`:

- **Choosing a department fills in its school and locks it**, so the pair can never disagree.
- Choosing a school first simply narrows the department list.
- A **support department** (Finance, ICT, Library — `school_id IS NULL`, migration 066) *clears*
  the school and says so, rather than leaving a faculty attached to a department that has none.

### The patroller's second factor — a PIN

Only the QA patroller is asked for more than a password, and only because of what a patrol tick is:
an accusation that a named lecturer was or was not teaching, weighed against the coordinator's own
log precisely because it comes from an independent observer. The password is the part that gets
shared "to help cover the rounds", and once shared, anyone can mark any lecturer absent.

- **First sign-in** lands on a *Set your patrol PIN* page (4–8 digits; repeated and running digits
  refused). **Every sign-in after** asks for it before the round will open.
- Verified **server-side** (`POST /api/v1/patrol/pin/verify`) — a secret a stolen handset could
  check for itself is a delay, not a factor. So the round cannot open offline, and the screen says
  so plainly.
- 5 wrong attempts → a 15-minute lockout. An admin can **clear** a PIN
  (`DELETE /api/v1/admin/patrol-pins/{user_id}`, audited) so the patroller sets a new one; nobody
  can ever *set* or read someone else's.
- This stacks with the handset binding (migration 069): the binding proves **which phone**, the PIN
  proves **who is holding it**.

### Signing out of the phone app

One handset is passed between coordinators and lent to students, so sign-out is a full handover,
not just "forget the token". Every role's button is the same control (`SignOutButton`), and it:

1. **Refuses while a session is open.** End the session first — that is what seals the attendance
   and uploads it. Signing out drops the device-binding key, which is the only thing able to seal a
   closed session, so the room's check-ins would be stranded on the phone forever.
2. **Warns when sealed sessions have not reached the server**, naming the count, before discarding
   them. Same reason: the key goes with the sign-in.
3. Then tears everything down — the foreground service (and with it the hotspot and the in-room
   HTTP server on `:8080`), the pop-up notifications, the credentials (written **synchronously**, so
   a process death cannot resurrect the session), the cached cohort roster and check-ins in Room,
   and all in-memory state including `force_password_change`.

The mandatory change-password screen carries its own sign-out, since it is the one screen with no
navigation of its own and a user who cannot complete the change was otherwise stuck on it.

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
| **KIU QAAT** (Android) | Coordinators, lecturers, students, **QA patrollers**, standby coordinators — including student check-in | `frontend/coordinator-android` (`:app`) |
| Biometric tablet | Employees | external device → employee attendance ingest |

**The phone app serves four roles and only four:** `COORDINATOR`, `LECTURER`, `STUDENT` and
`QA_PATROLLER`. Every other role — the oversight tier, the org tier and the ADMIN — signs in
successfully but is shown a screen saying their work is on the web dashboard. It used to fall
through to the coordinator's in-room hub, which handed session-opening and roster controls to
roles that had no business seeing them; the server refused the calls, but the controls were on
screen. Unknown roles now get no role UI rather than the most powerful one.

All web traffic reaches one public service, the **api-gateway**, which verifies the JWT, resolves the
tenant and enforces the role. Clients never talk to Postgres.

---

## 9. Tenancy

Every stakeholder is confined to **one institution (tenant)** — without exception. Isolation is
enforced in the database by row-level security keyed on `app.current_tenant`, which the gateway sets
from the JWT's `tenant_id` on every request. The gateway strips any inbound `X-Tenant-ID`,
`X-User-ID` or `X-Role` header before setting its own, so a client cannot assert its own identity.
