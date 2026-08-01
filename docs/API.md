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
- **Offerings & coordinators:** `GET/POST/DELETE /…/offerings` (cohorts; supports apply-across-all-courses), `GET /…/coordinators[/export.xlsx]`, `POST /…/coordinators/import`. (BLE `beacons` endpoints were removed — migration 039.)
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
| GET | `/api/v1/eligibility/{student_id}` | Own attendance/eligibility — **path id is ignored for STUDENT; forced to caller** |

## 6. Governance — VC / DVC / DQA_DIRECTOR / QA_OFFICER

- **VC/DVC:** `GET /api/v1/dashboard/vc/{overview,lecturer-workload}`, `GET /api/v1/reports/vc/audit.pdf`
- **DQA_DIRECTOR:** `GET /api/v1/dashboard/dqa/{course-health,ineligible,punctuality,trends}`, `GET/PUT /api/v1/dashboard/dqa/thresholds`, `GET /api/v1/reports/dqa/eligibility.csv`, `POST /api/v1/eligibility/clearance-token`
- **QA_OFFICER:** `GET /api/v1/dashboard/qa/{coordinator-health,live-sessions,student-attendance[/export.xlsx]}`, `POST /api/v1/dashboard/qa/{attendance-correction,device-reset,student-attendance/import}`, `POST /api/v1/qr/reissue`
- **Shared lecturer-attendance view:** `GET /api/v1/dashboard/lecturer-attendance[/summary]` (QA/VC/DVC/DQA)
- **Shared lecturer-teaching report:** `GET /api/v1/reports/lecturer-teaching?from=&to=&school=&department=&lecturer=&unit=&status=` — aggregates `lecturer_patrol_logs` (patroller observations **and** QA-rep workbook uploads). Open to QA/DQA/VC/DVC/ADMIN unscoped; for HOD, DEAN and the QA rep roles the caller's own department/school is applied **on top of** the query filters, so an org-scoped caller cannot read another unit by naming it.
- **Rooms picker:** `GET /api/v1/dashboard/rooms` — the tenant's active rooms (ADMIN/QA/DQA/COORDINATOR/HOD/DEAN/QA reps)
- **Any authenticated role:** `GET /api/v1/branding`, `POST /api/v1/auth/{change-password,change-email}`

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
