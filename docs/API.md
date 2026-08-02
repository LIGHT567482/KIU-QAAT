# QAAT — API Endpoint Catalog

**Platform version:** `1.0.0` (single source: `/VERSION`, `services/api-gateway/internal/version`, frontend `VITE_APP_VERSION`).
**API version:** `v1` — every JSON endpoint is namespaced under `/api/v1/…`. Bump the path prefix (`/api/v2`) for breaking changes; additive changes stay in `v1`.
**Base URL:** the API gateway behind Caddy (HTTPS). Health probe: `GET /health` and `GET /api/v1/health` → `{status, service, version, timestamp}`.

All requests are TLS-terminated by Caddy → `api-gateway`. The gateway verifies the RS256 JWT, applies RBAC (`RequireRole`), tenant ownership (`RequireOwnTenant`), sets the Postgres RLS tenant GUC (`app.current_tenant`), rate-limits, and audit-logs before proxying to internal services. Internal services (auth, qr-generator, session-manager, sync-receiver, notification) are **not** publicly reachable and independently re-verify the JWT.

---

## Access tiers (who can call what)

| Tier | Roles | Auth proof |
|------|-------|-----------|
| Public | none | none, or a signed artefact verified in-handler (student QR, lecturer gate token) |
| Super-admin **plane** | `SUPER_ADMIN` | password login against the **platform tenant** (`00000000-…-0`) |
| Tenant admin | `ADMIN` (own tenant only) | email + password + **Institution ID** |
| Coordinator | `COORDINATOR` | email + password |
| Governance | `VC`, `DVC`, `DQA_DIRECTOR`, `QA_OFFICER` | email + password |
| Student | `STUDENT` | personal **QR** (passwordless) or email + password |

> **The super-admin is intentionally outside the tenant system.** It owns no academic data; it exists only to register/suspend tenants, set their identity/branding, and (by design) gate monetization for this standalone SaaS. It lives in its own app (`apps/super-admin`, its own origin/port) and a sentinel "platform" tenant. Tenant admins are confined to their own tenant by `RequireOwnTenant`; they cannot create, list, or touch other tenants.

---

## 1. Public endpoints (no JWT)

Authenticated by a signed artefact inside the handler and protected by a per-IP rate limiter (`PublicIPRateLimit(burst, perMin)`).

| Method | Path | Purpose | In-handler proof |
|--------|------|---------|------------------|
| GET | `/health`, `/api/v1/health` | Liveness + version | — |
| GET | `/metrics` | Prometheus scrape | — (should be ClusterIP-only in prod) |
| POST | `/api/v1/checkin` | **Student self-scan check-in** | RSA-signed student QR + rotating room code + device fingerprint |
| GET | `/checkin` | Student captive page (server-rendered; no app/login) | — |
| POST | `/api/v1/student/qr-login` | Passwordless student session from their QR | RSA-signed student QR |
| POST | `/api/v1/lecturer/gate-scan` | Lecturer **START/END** of a lecture | staff ID + live 10s code + fingerprint + (LAN) + (WebAuthn) |
| GET | `/lecturer/checkin`, `/lecturer/enroll` | Lecturer captive + biometric-enrol pages | — |
| GET | `/api/v1/lecturer/session-info` | Display fields for the lecturer captive page | session_id |
| POST | `/api/v1/lecturer/webauthn/{enroll,assert}/{begin,finish}` | Phone-passkey biometric enrol/verify | WebAuthn ceremony |
| GET | `/api/v1/branding/public` | Display-safe tenant branding for captive portals | tenant_id (display fields only) |
| POST | `/api/v1/auth/login` | Login (admins: email + institution_id; → auth-service) | credentials |
| GET | `/api/v1/auth/tenant-lookup` | Resolve tenant_id from a student email | — |
| GET | `/api/v1/student/progress` | **Passwordless reg-no progress portal** (read-only %; needs `?reg=` + `?org=`) | reg-no resolved only within its institution |
| POST | `/api/v1/lecturer/qr-login` | Passwordless lecturer dashboard from their career QR | HMAC-signed lecturer QR |
| POST | `/api/v1/auth/lecturer-login` | Lecturer staff-ID + password login (no email) | staff ID |
| POST | `/api/v1/auth/coordinator-standby-login` | Standby deputy login: `{code, reg}` → coordinator token (end-of-day) | one-day delegation code + own-cohort reg-no |

## 2. Super-admin plane — `RequireRole(SUPER_ADMIN)`

Tenant lifecycle + identity. Runs on the no-RLS admin pool because it is cross-tenant by design.

| Method | Path | Purpose |
|--------|------|---------|
| GET / POST | `/api/v1/admin/tenants` | List / create tenants |
| PATCH | `/api/v1/admin/tenants/{tenant_id}/status` | Activate / suspend |
| PATCH | `/api/v1/admin/tenants/{tenant_id}/branding` | Logo, motto, colours, address |
| DELETE | `/api/v1/admin/tenants/{tenant_id}` | Delete tenant (platform tenant is protected) |

## 3. Tenant admin — `RequireRole(ADMIN[,SUPER_ADMIN]) + RequireOwnTenant`

Sub-resources under `/api/v1/admin/tenants/{tenant_id}/…` are confined to the caller's own tenant (`RequireOwnTenant` rejects a mismatched `{tenant_id}`; SUPER_ADMIN may act on any).

- **Users:** `GET/POST /…/{tenant_id}/users`, `PATCH /api/v1/admin/users/{user_id}`, `PATCH /api/v1/admin/users/{user_id}/status`, `DELETE /api/v1/admin/users/{user_id}`
- **Courses & units:** `GET/POST /…/{tenant_id}/courses`, `PATCH /api/v1/admin/courses/{course_id}`, `GET /api/v1/admin/courses/{course_id}/units`, `POST /…/units`, `PATCH /api/v1/admin/courses/{course_id}/units/{unit_id}`, `GET /api/v1/admin/courses/{course_id}/roadmap`
- **Students:** `GET/POST/PATCH/DELETE /…/{tenant_id}/students`, `GET /…/students/export.xlsx`, `POST /api/v1/import/csv`, `POST /api/v1/import/trigger`
- **Lecturers & assignments:** `GET/POST /…/{tenant_id}/lecturers`, `PATCH /api/v1/admin/lecturers/{lecturer_id}`, `POST /…/lecturers/{lecturer_id}/enroll-link`, `GET/POST /…/{tenant_id}/lecturer-assignments`, `DELETE /api/v1/admin/lecturer-assignments/{assignment_id}`, `GET /…/{tenant_id}/lecturer-attendance[/summary]`
- **Offerings & coordinators:** `GET/POST/DELETE /…/offerings` (cohorts; supports apply-across-all-courses), `GET /…/coordinators[/export.xlsx]`, `POST /…/coordinators/import`.
- **Schools & departments:** `GET/POST /…/{tenant_id}/schools`, `DELETE /…/schools/{school_id}`, `GET/POST /…/{tenant_id}/departments` (GET accepts `?school_id=`), `DELETE /…/departments/{department_id}`
- **Rooms / room codes:** `GET/POST /…/{tenant_id}/rooms`, `PATCH|DELETE /…/rooms/{room_code}`, `POST /…/rooms/import` (multipart `roster`, xlsx/csv), `GET /…/rooms/export.xlsx`. Backed by the `venues` table — `venue_id` **is** the room code — extended in migration 062 with `school_id`/`department_id`/`room_type`/`is_active`. `…/venues` remains a **legacy alias** of every `…/rooms` route. A room still referenced by the timetable or past sessions cannot be deleted; deactivate it. Room codes are unique **platform-wide**, not per tenant (a quirk of migration 001).
- **Permanent QRs + email dispatch:** `GET /…/{tenant_id}/lecturers/{lecturer_id}/qr` (career-QR token + scan URL). On create/import, if an optional `email` is supplied, the lecturer/student QR is emailed via qr-generator `POST /api/v1/qr/email-link` (lecturers) / `POST /api/v1/qr/issue` (students).
- **Academic year / period:** `PATCH /…/{tenant_id}/academic-period` sets the institution-wide **active academic year** (`active_academic_year`, shared by all intakes; rolled onto every active student). `active_semester` is **optional** — there is **no single institution-wide semester**: within one academic year different intakes/cohorts sit in different semesters at once (yr1/sem1, yr1/sem2, yr2/sem1 …), so semester lives on each `course_offerings`/`students_extended` row and the coordinator manifest is scoped by the coordinator's **own offering** (year+semester+level), never a global semester. `POST /…/academic-period/advance` (body may include `intakes: []` to advance only those intakes — each student advances along their **own semester** as the root: yr1/sem2 → yr2/sem1, etc., and their **cohorts** (`course_offerings`, which carry the intake) advance by the same rule so the coordinator's catalog stays in lock-step; final year graduates/completes. Other intakes and the shared academic year are untouched. Omit/empty = whole-institution advance).
- **End-of-semester clear + archives:** `POST /…/{tenant_id}/clear-semester-data` (password-gated; body `intakes: []` is **required** — at least one; optional `academic_year`). It first zips the in-scope attendance/sessions/lecturer logs into a **semester archive**, then deletes only that intake's attendance and the sessions that become empty (shared sessions of a continuing intake are kept). Archives: `GET /…/{tenant_id}/semester-archives`, `GET /…/semester-archives/{archive_id}/download` (zip of CSVs), `DELETE /…/semester-archives/{archive_id}`.
- **Settings (`/api/v1/admin/settings/*`, own tenant via JWT):** `thresholds`, `intakes`, `levels`, `titles`, `session-window`, `study-sessions`, `staff-id-prefix`, `users-passcode` (+ `/verify`) — each `GET` + `PUT`.

## 4. Coordinator — `RequireRole(COORDINATOR)`

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/v1/manifest/daily` | Offline daily manifest |
| POST | `/api/v1/sessions/open` | Open an ACTIVE session (captures coordinator IP for LAN gate) |
| POST | `/api/v1/sessions/{session_id}/close` | Close session |
| GET | `/api/v1/sessions/{session_id}/checkin-code` | Live rotating room code |
| GET | `/api/v1/sessions/{session_id}/lecturer-gate-qr` | Lecturer gate QR for the screen |
| GET | `/api/v1/sessions/{session_id}/roster` | Who is present |
| GET | `/api/v1/coordinator/{overview,students,last-roster}` | Coordinator dashboards |
| GET/PUT | `/api/v1/coordinator/units/{unit_id}/schedule` | Set-once/locked unit schedule |
| GET | `/api/v1/coordinator/units/{unit_id}/lecturers` | Assigned lecturers |
| POST/GET | `/api/v1/sync/{init,chunk/…,complete/…,resume/…}` | Offline sync (→ sync-receiver) |
| POST | `/api/v1/sessions/warden-link` | LAN warden link (→ session-manager) |
| GET/POST | `/api/v1/coordinator/standby` | List / issue a **standby delegation** (own-cohort student + one-day code) |
| POST | `/api/v1/coordinator/standby/{id}/revoke` | Revoke a standby delegation |

## 5. Student — `RequireRole(STUDENT)` (JWT path) + the public QR path above

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/api/v1/student/checkin` | Authenticated check-in (room code) |
| GET | `/api/v1/student/live-sessions` | Live sessions for the student's cohort |
| GET | `/api/v1/student/home` | The app's Home tab: profile, cohort, programme units, weekly timetable |
| GET | `/api/v1/eligibility/{student_id}` | Own attendance/eligibility — **path id is ignored for STUDENT; forced to caller** |

`student/home` returns **every unit on the student's programme**, each carrying its `year`,
`semester` and a `current` flag for the year/semester they are sitting. It deliberately does not
filter to the current semester: a cohort whose year/semester has no units tagged against it yet
would otherwise be told it has no units at all. The `timetable` array is scoped to the student's own
`offering_id`, so a Weekend student never sees the Day run of a shared unit.

## 5b. Lecturer — `RequireRole(LECTURER)`

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/v1/lecturer/overview` | The units they are assigned to, with session counts |
| GET | `/api/v1/lecturer/calendar?from=&to=&unit_id=` | Unit-centric teaching calendar, cohorts as sub-tags |
| GET | `/api/v1/lecturer/roster?scope=enrolled\|attended&unit_id=` | Students across their units |
| GET | `/api/v1/lecturer/sessions[?unit_id=]` | Sessions held, per cohort |
| GET | `/api/v1/lecturer/sessions/{session_id}/students` | One session's present/absent list |
| GET | `/api/v1/lecturer/attendance?unit_id=` | The unit's attendance matrix |

Every one of these is driven by **`lecturer_assignments`** — the row that says this lecturer teaches
this unit. A unit with no assignment row is invisible to all of them, shows a blank lecturer on the
student's timetable, and reaches the patrol manifest with nobody named against it. The admin
roadmap (`GET /api/v1/admin/courses/{course_id}/roadmap`) therefore returns `lecturer_names` and
`slot_count` per unit, so that gap is visible on the screen that governs it.

### The calendar, and the three different things it reports

`timetable_slots` holds one row per cohort per **weekly** slot ("CSE 2420, Mondays, 08:00"), so the
handler expands those slots across the requested range: a unit taught every Monday yields an entry
on *every* Monday in the window, not one entry. Each entry then carries three facts that are
routinely confused, and which the app renders separately:

| Field | Means | Source |
|---|---|---|
| `held` | A session was opened for that unit and date | `sessions` |
| `lecturer_present` (+ `contact_hours`) | **The lecturer actually gated in** | `lecturer_attendance_logs` |
| `pct` / `cohorts[].pct` | How many *students* attended | `attendance_logs` |

`held` is not presence: a coordinator can open a room around a lecturer who never arrived. Only
`lecturer_attendance_logs` — written when the lecturer physically STARTs at the coordinator's gate —
answers "was I there?", which is what the phone app's month grid colours each day by (taught /
missed / partly / scheduled). A day with a timetabled slot, in the past, with no gate record, is a
**miss**, and the grid must show it: the earlier agenda-style view listed only days that had a
class, so a missed Monday and a Monday that was never timetabled looked identical — both absent
from the screen.

## 6. Governance — VC / DVC / DQA_DIRECTOR / QA_OFFICER

- **VC/DVC:** `GET /api/v1/dashboard/vc/{overview,lecturer-workload}`, `GET /api/v1/reports/vc/audit.pdf`
- **DQA_DIRECTOR:** `GET /api/v1/dashboard/dqa/{course-health,ineligible,punctuality,trends}`, `GET/PUT /api/v1/dashboard/dqa/thresholds`, `GET /api/v1/reports/dqa/eligibility.csv`, `POST /api/v1/eligibility/clearance-token`
- **QA_OFFICER:** `GET /api/v1/dashboard/qa/{coordinator-health,live-sessions,student-attendance[/export.xlsx]}`, `POST /api/v1/dashboard/qa/{attendance-correction,device-reset,student-attendance/import}`, `POST /api/v1/qr/reissue`
- **Shared lecturer-attendance view:** `GET /api/v1/dashboard/lecturer-attendance[/summary]` (QA/VC/DVC/DQA)
- **Shared lecturer-teaching report:** `GET /api/v1/reports/lecturer-teaching?from=&to=&school=&department=&lecturer=&unit=&status=` — aggregates `lecturer_patrol_logs` (patroller observations **and** QA-rep workbook uploads). Open to QA/DQA/VC/DVC/ADMIN unscoped; for HOD, DEAN and the QA rep roles the caller's own department/school is applied **on top of** the query filters, so an org-scoped caller cannot read another unit by naming it.
- **Rooms picker:** `GET /api/v1/dashboard/rooms` — the tenant's active rooms (ADMIN/QA/DQA/COORDINATOR/HOD/DEAN/QA reps)
- **Any authenticated role:** `GET /api/v1/branding`, `POST /api/v1/auth/{change-password,change-email}`

## 6a. QA patrol — QA_PATROLLER

The patroller works from the **KIU QAAT** Android app (the same one everyone installs — the former
standalone `ug.qaat.patroller` APK was deleted and its screens became a role branch).

| Method | Path | Notes |
|---|---|---|
| POST | `/api/v1/patrol/bind-device` | Claims this handset for the caller. First call binds; a repeat from the same phone is a no-op |
| GET | `/api/v1/patrol/manifest` | Today's timetabled slots — unit ↔ lecturer ↔ room ↔ time |
| POST | `/api/v1/patrol/sync` | Batch of observations; idempotent per (tenant, unit, date, scheduled time) |

**Handset binding.** All three require an `X-Device-Fingerprint` header matching the row in
`patroller_device_bindings` (migration 069), on top of the `QA_PATROLLER` role check. A patrol
record accuses a named lecturer and feeds the teaching reports as the independent second record, so
a valid token is deliberately not sufficient — the call must come from the bound phone. Anything
else, including a call with no fingerprint at all and a call from an as-yet-unbound account, is
refused `403 DEVICE_NOT_BOUND`. Claiming a handset already held by another patroller is refused
`403 DEVICE_IN_USE`: one phone serves exactly one patrol account. The handset each tick came from
is stored on the record itself (`lecturer_patrol_logs.patroller_device_hash`).

**Releasing a binding** (lost or replaced phone) is an ADMIN action, recorded by the audit-log
middleware:

| Method | Path | Notes |
|---|---|---|
| GET | `/api/v1/admin/patrol-bindings` | Who is bound to a handset, when it was claimed and last used. The fingerprint value itself is never returned |
| DELETE | `/api/v1/admin/patrol-bindings/{user_id}` | Frees that patroller to claim a new phone |

## 6b. Org-scoped oversight — HOD / DEAN / QA_SCHOOL_HANDLER / QA_DEPT_REP

Every one of these roles is bounded by the org unit **on its own user record** — `users.department` for HOD and QA_DEPT_REP, `users.school` for DEAN and QA_SCHOOL_HANDLER. The scope is always re-derived server-side from the JWT's role + user id; it is never accepted from the request. An account with no department/school set matches **nothing** (never everything). `POST /…/users` rejects creating these roles without the matching org unit.

| Method | Path | Role | Purpose |
|--------|------|------|---------|
| GET | `/api/v1/hod/lecturers` | HOD | Lecturers of own department + teaching progress |
| GET | `/api/v1/dean/lecturers` | DEAN | Same, across own school |
| GET | `/api/v1/qa-rep/scope` | QA reps | Who am I / what do I cover / may I file |
| GET | `/api/v1/qa-rep/lecturers` | QA reps | Same query as HOD/Dean, scoped by the caller's role |
| GET | `/api/v1/qa-rep/departments` | QA reps + oversight | Per-department roll-up incl. last report filed |
| GET | `/api/v1/qa-rep/template.xlsx` | QA reps | Monitoring workbook pre-filled with own timetabled sessions |
| POST | `/api/v1/qa-rep/submissions` | QA reps | **Upload a monitoring workbook** (multipart `report`, ≤8 MB, xlsx or csv; plus `period_label`, `period_from`, `period_to`, `notes`) |
| GET | `/api/v1/qa-rep/submissions` | QA reps + oversight | Submissions in scope (oversight sees all) |
| GET | `/api/v1/qa-rep/submissions/{id}/file` | QA reps + oversight | Download the original workbook |
| DELETE | `/api/v1/qa-rep/submissions/{id}` | QA reps + oversight | Withdraw (a rep may remove only their own) |

**Upload semantics.** Columns: `unit_id`, `unit_name`, `course_code`, `lecturer_staff_id`, `lecturer_name`, `room`, `date`, `time`, `taught`, `remarks` (header synonyms and Excel serial dates/times are accepted; only `unit_id`, `date` and `taught` are required). Recognised rows are written to `lecturer_patrol_logs` with `entry_method = 'QA_REP_UPLOAD'` so they feed every existing teaching report, and the workbook itself is stored on `qa_rep_submissions.file_bytes` as the evidence behind them. Two invariants:

1. A row naming a unit outside the rep's own department/school is refused.
2. The upsert only overwrites rows that are themselves `QA_REP_UPLOAD` — **a QA patroller's live field observation is never overwritten by a spreadsheet filled in afterwards**; such rows are reported as skipped.

Withdrawing a submission removes the observations derived from it (FK cascade) and nothing else.

**Messaging & notifications.** The QA rep roles share the QA officer's channels: `/api/v1/messages*` (they receive the DQA's `ALL_QA`/`DEPARTMENT`/`SCHOOL` broadcasts matching their own unit, and reply up to the DQA) and `/api/v1/app-notifications` (audiences `LECTURERS`, `LECTURER`, `DQA`, `ADMIN` — the lecturer audiences resolve **only** within the sender's org unit).

### Org-scoped dashboards — scope is never a parameter

| Method | Path | Roles |
|--------|------|-------|
| GET | `/api/v1/org/overview` | HOD · DEAN · QA_DEPT_REP · QA_SCHOOL_HANDLER · QA_OFFICER · DQA · VC · DVC · ADMIN |
| GET | `/api/v1/org/at-risk?limit=&course=` | same |
| GET | `/api/v1/org/departments` | same — one row per department, naming its HOD |
| GET | `/api/v1/admin/overview` | ADMIN |
| GET | `/api/v1/admin/audit?action=&actor=&from=&to=&limit=` | ADMIN |

`resolveOrgScope` reads the caller's org unit **from their own account** — `users.department` for
HOD/QA_DEPT_REP, `users.school` for DEAN/QA_SCHOOL_HANDLER — and there is deliberately **no query
parameter for it**, so a dean cannot read another college by naming one. The institution-wide roles
(DQA, QA officer, VC/DVC, ADMIN) are `Unbounded` and get the same data unfiltered, which is what
makes the at-risk watchlist one page instead of five.

A bounded role whose account carries **no unit matches nothing, not everything** — an empty scope
that matched every department would silently hand the head of one department the whole institution.
The same rule already governs `resolveRecipients`, and `qaFiltersScoped` applies it to
`/api/v1/dashboard/qa/student-attendance` so HOD and DEAN read that shared endpoint scoped. Its
`_scope_*` keys are underscored precisely because they are not, and must never become, query
parameters.

### The org tree vs the academic data — the seam everything hinges on

There are **two** representations of the same hierarchy, joined only by string equality:

- `schools` → `departments` — id-linked, authoritative, edited under *Schools & Departments*.
- `courses.school` / `courses.department` — free-text **names**, and what every scoped query
  actually filters on.

`/api/v1/org/departments` is deliberately driven by the **`departments` table**, not by
`courses.department`. Deriving the list from courses would silently omit a department that exists
but has no course attached — precisely the one a dean most needs to ask about. Everything else
LEFT JOINs onto that spine, which is what lets the two real failure modes be *reported* instead of
absorbed:

| Fault | What it looks like to the person affected |
|---|---|
| Department with **no HOD** | Nobody answerable; it appears on the dean's page as an unowned card |
| Department **on no course** (`unlinked`) | Its HOD opens a working, blank dashboard with no clue why |
| Course naming an **unknown department** | Its lecturers and students belong to no HOD and to no dean's college |

All three are counted on `/api/v1/admin/overview` under `gaps`, because closing them is the
administrator's job and nothing else in the system raises so much as a warning about them.

### The management chain is a channel in both directions

`resolveRecipients` gained three audiences so the layer between a dean and a lecturer stops being a
dead end:

| Audience | Sender | Resolves to |
|---|---|---|
| `HODS` | DEAN · QA_SCHOOL_HANDLER | every HOD of a department in **their** school |
| `HOD` + `target_id` | DEAN · QA_SCHOOL_HANDLER | one HOD, **only** if their department is in that school |
| `DEAN` | HOD · QA_DEPT_REP | the dean of the school **their own department** sits under |

Scope still comes from the sender's account, never the request, so `HODS` can only ever mean the
heads of the sender's own college. Before this a dean's only bulk option was `LECTURERS` — every
lecturer in the college at once, routing straight past the person responsible for them.

### The audit trail

`admin_audit_log` has existed since migration 001 and **nothing wrote to it** until `auditAdmin`.
Writes are best-effort by design — an audit failure must never abort the action it is recording, a
half-done device release with a clean log being worse than a completed one with a missing line —
but they are logged. Recorded today: `USER_DELETED`, `STUDENT_DEVICE_RESET`,
`PATROL_DEVICE_RELEASED`, `PATROL_PIN_RESET`. The `payload` JSONB carries what cannot be recovered
afterwards (a deleted account's email and role, the reason given for a reset); it must never carry
credentials, a PIN, or a token.

### Patroller PIN (second factor)

| Method | Path | Role |
|--------|------|------|
| GET | `/api/v1/patrol/pin` | QA_PATROLLER — `{pin_set, locked, attempts_left}` |
| POST | `/api/v1/patrol/pin` | QA_PATROLLER — set (first time) or change (`current_pin` required) |
| POST | `/api/v1/patrol/pin/verify` | QA_PATROLLER — opens the round |
| DELETE | `/api/v1/admin/patrol-pins/{user_id}` | ADMIN — clears it; audited |

Verification is server-side on purpose: a secret a stolen handset can check for itself is a delay,
not a factor. The PIN is never returned by any endpoint — `pin_set` is the only fact the app is
told — and an administrator can clear one but never set or read it. 5 failures → a 15-minute
lockout, counted in a single `UPDATE … RETURNING` so two racing attempts cannot both read 4.

### Dismissing an alert

`DELETE /api/v1/app-notifications/{id}` and `DELETE /api/v1/messages/{id}` clear an item from **the
caller's own inbox**. Neither deletes anything: an alert fanned out to a cohort is one row with many
recipients, so a student clearing their copy must not remove anyone else's. The sender's record and
the audit trail are untouched.

Three properties the clients depend on, and which any new inbox surface must preserve:

- **Idempotent.** Dismissing something already dismissed returns `200`, not `404`. Clients restore
  the card when a dismissal fails, so a `404` on the second press (a double-tap, a retry over a
  flaky link, a second signed-in device) would make the alert pop back — the exact opposite of what
  the ✕ asked for. Only an id that is not in the caller's inbox at all returns `404`.
- **Every reader may clear.** The dismiss route carries the *same* role list as the list/read
  routes (`inboxRoles` in the router). They were once written out separately and drifted, leaving
  `HOD`/`DEAN`/`QA_DEPT_REP`/`QA_SCHOOL_HANDLER` able to read an inbox but not clear it: the ✕
  returned `403`, the card vanished locally and returned on the next refresh.
- **Dismissed is gone from the badge too.** `GET /api/v1/app-notifications/unread-count` filters
  `dismissed_at IS NULL`, or the ✕ would leave an unread count pointing at rows that are no longer
  listed anywhere they could be opened.

**Pop-up delivery.** There is no FCM in this deployment (no Play Services; the backend is the
institution's own and the app is offline-first), so the Android app **polls** these endpoints and
raises the Android notification itself — every ~45s while the process is alive, and every 15 minutes
via WorkManager after it has been closed. Dismissing an alert cancels its pop-up and blocks it for
good on that handset.

## 7. QR management (→ qr-generator)

| Method | Path | Role |
|--------|------|------|
| POST | `/api/v1/qr/generate/batch` | DQA_DIRECTOR, ADMIN |
| POST | `/api/v1/qr/reissue` | QA_OFFICER |
| POST | `/api/v1/qr/token` | ADMIN, SUPER_ADMIN |
| (internal) | `/api/v1/qr/issue` | server-to-server on student registration |

---

## Versioning policy

- **Single version string** `1.0.0` in `/VERSION`, mirrored by the Go `version` package (surfaced in `/health`) and baked into every frontend at build time as `VITE_APP_VERSION` (shown in each app footer next to "Powered by LIGHT TECHNOLOGIES").
- **API contract** is versioned by the `/api/v1` path prefix. Additive fields and new endpoints ship under `v1`; breaking changes require `/api/v2` and a deprecation window.
- Internal Node services carry their own `package.json` `version` (kept at the platform version).
