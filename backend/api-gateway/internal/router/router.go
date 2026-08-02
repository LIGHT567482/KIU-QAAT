package router

import (
	"crypto/rsa"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/qaat/api-gateway/internal/handlers"
	"github.com/qaat/api-gateway/internal/middleware"
	"github.com/qaat/api-gateway/internal/proxy"
)

// Upstreams holds the base URLs of the downstream microservices the gateway
// reverse-proxies to.
type Upstreams struct {
	AuthService    string
	SessionManager string
	SyncReceiver   string
}

// New builds and returns the fully wired router.
// All routes, middleware stacks, and role guards are defined here.
// pool is the tenant-scoped data plane (RLS-enforced qaat_app role). adminPool is
// a privileged connection used ONLY by the ADMIN-gated, cross-tenant handlers.
func New(publicKey *rsa.PublicKey, jwtIssuer, jwtAudience string, rdb *redis.Client, pool, adminPool *pgxpool.Pool, corsOrigins []string, env string, upstreams Upstreams) http.Handler { //nolint:cyclop
	r := chi.NewRouter()

	// ─── Downstream service proxies ───────────────────────────────────────────
	// JWT + tenant are verified at the gateway; the proxy forwards identity as
	// headers (X-Tenant-ID / X-User-ID / X-Role) to the service.
	authProxy := mustProxy(upstreams.AuthService)
	sessionProxy := mustProxy(upstreams.SessionManager)
	syncProxy := mustProxy(upstreams.SyncReceiver)

	// ─── Global middleware ────────────────────────────────────────────────────
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.PrometheusMiddleware)
	r.Use(correlationID)
	r.Use(middleware.RequireHTTPS(env))
	r.Use(middleware.SecurityHeaders(env))
	r.Use(middleware.CORS(corsOrigins))

	// ─── Public routes (no auth) ──────────────────────────────────────────────
	r.Get("/health", handlers.Health)
	r.Get("/api/v1/health", handlers.Health)

	// Scheduled cron (Render Cron Job) → notify employee no-shows across all tenants. Public route,
	// but gated by the X-Cron-Secret header (== CRON_SECRET env), so it's not openly callable.
	r.Post("/internal/cron/notify-no-shows", handlers.CronNotifyNoShows(adminPool))
	r.Get("/metrics", promhttp.Handler().ServeHTTP) // Prometheus scrape endpoint

	// Single passwordless student progress portal: enter reg-no → see attendance.
	r.With(middleware.PublicIPRateLimit(10, 40)).
		Get("/api/v1/student/progress", handlers.StudentProgressByReg(adminPool))

	// Native student app: bind this device to the student's reg number (one-device-one-student,
	// global) at one-time onboarding. Public; reg + org → tenant, self-scoped on adminPool.
	r.With(middleware.PublicIPRateLimit(5, 20)).
		Post("/api/v1/student/register-device", handlers.RegisterDevice(adminPool))

	// Unified KIU QAAT app sign-in: identifier (email / reg-no / staff-id) + password + org →
	// resolves to the account, reuses auth-service /auth/login, augments with student_id/staff_id.
	r.With(middleware.PublicIPRateLimit(5, 60)).
		Post("/api/v1/auth/app-login", handlers.AppLogin(adminPool))

	// Lecturer gate scan is public — authenticated by HMAC-signed QR token issued
	// by the coordinator. A per-IP limiter blunts brute-force / replay attempts.
	r.With(middleware.PublicIPRateLimit(5, 60)).
		Post("/api/v1/lecturer/gate-scan", handlers.LecturerGateScan(adminPool, rdb))
	r.Get("/lecturer/checkin", handlers.LecturerCheckinPage)
	r.Get("/lecturer/enroll", handlers.LecturerEnrollPage)
	r.With(middleware.PublicIPRateLimit(10, 60)).
		Get("/api/v1/lecturer/session-info", handlers.LecturerSessionInfo(adminPool))

	// Lecturer biometric (WebAuthn phone passkey) — enrol once, verify per scan.
	r.With(middleware.PublicIPRateLimit(10, 60)).Post("/api/v1/lecturer/webauthn/enroll/begin", handlers.LecturerEnrollBegin(adminPool, rdb))
	r.With(middleware.PublicIPRateLimit(10, 60)).Post("/api/v1/lecturer/webauthn/enroll/finish", handlers.LecturerEnrollFinish(adminPool, rdb))
	r.With(middleware.PublicIPRateLimit(20, 60)).Post("/api/v1/lecturer/webauthn/assert/begin", handlers.LecturerAssertBegin(adminPool, rdb))
	r.With(middleware.PublicIPRateLimit(20, 60)).Post("/api/v1/lecturer/webauthn/assert/finish", handlers.LecturerAssertFinish(adminPool, rdb))

	// Lecturer PORTAL — passwordless, read-only. Enter institution + staff ID and
	// search the attendance logs of your own units (rate-limited to slow enumeration).
	r.With(middleware.PublicIPRateLimit(30, 60)).
		Get("/api/v1/lecturer-portal/overview", handlers.LecturerPortalOverview(adminPool))
	r.With(middleware.PublicIPRateLimit(30, 60)).
		Get("/api/v1/lecturer-portal/attendance", handlers.LecturerPortalAttendance(adminPool))

	// Emergency standby coordinator login: a student exchanges code + reg-no for a
	// COORDINATOR token (issued for the absent coordinator) to run that day's session.
	r.With(middleware.PublicIPRateLimit(5, 20)).
		Post("/api/v1/auth/coordinator-standby-login", handlers.CoordinatorStandbyLogin(adminPool))

	// Public branding lookup for the captive portals (no JWT). Display-safe
	// fields only, keyed by the tenant_id carried in the student/lecturer QR.
	r.Get("/api/v1/branding/public", handlers.GetPublicBranding(adminPool))

	// ── Auth (→ auth-service) ───────────────────────────────────────────────
	// Login happens before a JWT exists, so it must stay outside the JWTAuth
	// group. Rate-limited per-IP to blunt credential-stuffing / brute force.
	r.With(middleware.PublicIPRateLimit(5, 60)).
		Post("/api/v1/auth/login", authProxy)

	// Lecturer dashboard login is passwordless: lecturers hold no usable password
	// (they are QR-only), so institution + staff ID resolve the lecturer — the same
	// trust model as the read-only lecturer portal — and a read-only LECTURER token
	// is minted. Rate-limited per-IP to slow staff-ID enumeration.
	r.With(middleware.PublicIPRateLimit(10, 60)).
		Post("/api/v1/auth/lecturer-login", handlers.LecturerPasswordlessLogin(adminPool))

	// Tenant lookup by student email — used by the check-in page to auto-resolve
	// tenant_id so students never need to know or type it.
	r.Get("/api/v1/auth/tenant-lookup", handlers.TenantLookup(adminPool))

	// ─── Authenticated routes ─────────────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(middleware.JWTAuth(publicKey, jwtIssuer, jwtAudience, rdb))
		r.Use(middleware.SetTenant(pool))
		r.Use(middleware.CoordinatorRateLimit())
		r.Use(middleware.AuditLog(pool))

		// ── Self-service account changes (any authenticated role) ─────────────
		r.Post("/api/v1/auth/change-password", authProxy)
		r.Post("/api/v1/auth/change-email", authProxy)

		// ── Daily Manifest ────────────────────────────────────────────────────
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Get("/api/v1/manifest/daily", handlers.ManifestDaily(pool, rdb))

		// ── QA Patroller (offline-first lecturer-presence patrol) ─────────────
		// Beyond the role check, all three are bound to ONE handset per patroller (migration 069):
		// bind-device claims it, the other two refuse any call that arrives from a different phone.
		r.With(middleware.RequireRole(middleware.RolePatroller)).
			Post("/api/v1/patrol/bind-device", handlers.BindPatrolDevice(pool))
		r.With(middleware.RequireRole(middleware.RolePatroller)).
			Get("/api/v1/patrol/manifest", handlers.PatrolManifest(pool))
		r.With(middleware.RequireRole(middleware.RolePatroller)).
			Post("/api/v1/patrol/sync", handlers.PatrolSync(pool))
		// The lost-phone path: an administrator releases a binding so the patroller can claim a
		// new handset. Audited by the AuditLog middleware already on this group.
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Get("/api/v1/admin/patrol-bindings", handlers.ListPatrolBindings(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Delete("/api/v1/admin/patrol-bindings/{user_id}", handlers.ReleasePatrolBinding(pool))

		// ── HOD + Dean (org-scoped lecturer oversight) ────────────────────────
		r.With(middleware.RequireRole(middleware.RoleHOD)).
			Get("/api/v1/hod/lecturers", handlers.HODLecturers(pool))
		r.With(middleware.RequireRole(middleware.RoleDean)).
			Get("/api/v1/dean/lecturers", handlers.DeanLecturers(pool))

		// ── QA reps (dept rep / school handler) ───────────────────────────────
		// Their scope comes from the JWT role + their own user row, never the request.
		qaRep := middleware.RequireRole(middleware.RoleQADeptRep, middleware.RoleQASchool)
		// The oversight roles read submissions across the whole institution.
		qaRepOrOversight := middleware.RequireRole(middleware.RoleQADeptRep, middleware.RoleQASchool,
			middleware.RoleQAOfficer, middleware.RoleDQADirector, middleware.RoleVC, middleware.RoleDVC, middleware.RoleAdmin)

		r.With(qaRep).Get("/api/v1/qa-rep/scope", handlers.QARepScope(pool))
		r.With(qaRep).Get("/api/v1/qa-rep/lecturers", handlers.QARepLecturers(pool))
		r.With(qaRepOrOversight).Get("/api/v1/qa-rep/departments", handlers.QARepDepartments(pool))
		r.With(qaRepOrOversight).Get("/api/v1/qa-rep/submissions", handlers.QAListSubmissions(pool))
		r.With(qaRep).Post("/api/v1/qa-rep/submissions", handlers.QASubmitReport(pool))
		r.With(qaRepOrOversight).Get("/api/v1/qa-rep/submissions/{id}/file", handlers.QASubmissionFile(pool))
		r.With(qaRepOrOversight).Delete("/api/v1/qa-rep/submissions/{id}", handlers.QADeleteSubmission(pool))
		r.With(qaRep).Get("/api/v1/qa-rep/template.xlsx", handlers.QARepTemplate(pool))

		// ── Cross-dimension lecturer-teaching report (QA oversight) ───────────
		r.With(middleware.RequireRole(middleware.RoleQAOfficer, middleware.RoleDQADirector, middleware.RoleVC, middleware.RoleDVC, middleware.RoleAdmin, middleware.RoleHOD, middleware.RoleDean, middleware.RoleQASchool, middleware.RoleQADeptRep)).
			Get("/api/v1/reports/lecturer-teaching", handlers.LecturerTeachingReport(pool))
		// Same report, same filters, taken away as a spreadsheet or a PDF.
		r.With(middleware.RequireRole(middleware.RoleQAOfficer, middleware.RoleDQADirector, middleware.RoleVC, middleware.RoleDVC, middleware.RoleAdmin, middleware.RoleHOD, middleware.RoleDean, middleware.RoleQASchool, middleware.RoleQADeptRep)).
			Get("/api/v1/reports/lecturer-teaching/export.xlsx", handlers.LecturerTeachingExport(pool, "xlsx"))
		r.With(middleware.RequireRole(middleware.RoleQAOfficer, middleware.RoleDQADirector, middleware.RoleVC, middleware.RoleDVC, middleware.RoleAdmin, middleware.RoleHOD, middleware.RoleDean, middleware.RoleQASchool, middleware.RoleQADeptRep)).
			Get("/api/v1/reports/lecturer-teaching/export.csv", handlers.LecturerTeachingExport(pool, "csv"))
		r.With(middleware.RequireRole(middleware.RoleQAOfficer, middleware.RoleDQADirector, middleware.RoleVC, middleware.RoleDVC, middleware.RoleAdmin, middleware.RoleHOD, middleware.RoleDean, middleware.RoleQASchool, middleware.RoleQADeptRep)).
			Get("/api/v1/reports/lecturer-teaching/export.pdf", handlers.LecturerTeachingExport(pool, "pdf"))

		// ── Employee biometric no-shows (detect + notify by email + WhatsApp) ─
		r.With(middleware.RequireRole(middleware.RoleQAOfficer, middleware.RoleDQADirector, middleware.RoleAdmin)).
			Get("/api/v1/qa/employee-no-shows", handlers.EmployeeNoShows(adminPool))
		r.With(middleware.RequireRole(middleware.RoleQAOfficer, middleware.RoleDQADirector, middleware.RoleAdmin)).
			Post("/api/v1/qa/notify-no-shows", handlers.NotifyNoShows(adminPool))

		// ── Session Management (→ session-manager) ────────────────────────────
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Post("/api/v1/sessions/warden-link", sessionProxy)

		// ── Online check-in session lifecycle (handled in-gateway) ────────────
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Post("/api/v1/sessions/open", handlers.OpenSession(pool))
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Get("/api/v1/sessions/{session_id}/checkin-code", handlers.CheckinCode(pool))

		// ── Student authenticated check-in ────────────────────────────────────
		// Identity = JWT (email → student_id lookup). Proximity = room code.
		// No QR needed: the student's authenticated account IS the identity proof.
		r.With(middleware.RequireRole(middleware.RoleStudent)).
			Post("/api/v1/student/checkin", handlers.StudentCheckin(pool))
		// Live/active sessions the student may attend right now (#4a).
		r.With(middleware.RequireRole(middleware.RoleStudent)).
			Get("/api/v1/student/live-sessions", handlers.StudentLiveSessions(pool))
		// The student app's Home tab: profile, their cohort's units and its weekly timetable.
		r.With(middleware.RequireRole(middleware.RoleStudent)).
			Get("/api/v1/student/home", handlers.StudentHome(adminPool))
		// ── Synchronisation (→ sync-receiver) ─────────────────────────────────
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Post("/api/v1/sync/init", syncProxy)
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Post("/api/v1/sync/chunk/{upload_id}/{chunk_index}", syncProxy)
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Get("/api/v1/sync/resume/{upload_id}", syncProxy)
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Post("/api/v1/sync/complete/{upload_id}", syncProxy)

		// ── Eligibility ───────────────────────────────────────────────────────
		r.With(middleware.RequireRole(middleware.RoleQAOfficer, middleware.RoleDQADirector, middleware.RoleVC, middleware.RoleDVC, middleware.RoleStudent)).
			Get("/api/v1/eligibility/{student_id}", handlers.GetEligibility(pool))

		r.With(middleware.RequireRole(middleware.RoleDQADirector)).
			Post("/api/v1/eligibility/clearance-token", sessionProxy)

		// ── DQA ⇄ QA-officer messaging (in-app inbox) ────────────────────────
		// The DQA director shares reports/notifications to QA officers (all / by
		// department / by college-school); QA officers reply to the DQA. Optional
		// file attachment. adminPool + explicit tenant scoping in the handlers.
		r.With(middleware.RequireRole(middleware.RoleDQADirector, middleware.RoleQAOfficer, middleware.RoleQADeptRep, middleware.RoleQASchool)).
			Post("/api/v1/messages", handlers.SendQAMessage(adminPool))
		r.With(middleware.RequireRole(middleware.RoleDQADirector, middleware.RoleQAOfficer, middleware.RoleQADeptRep, middleware.RoleQASchool)).
			Get("/api/v1/messages", handlers.ListQAMessages(adminPool))
		r.With(middleware.RequireRole(middleware.RoleDQADirector, middleware.RoleQAOfficer, middleware.RoleQADeptRep, middleware.RoleQASchool)).
			Get("/api/v1/messages/unread-count", handlers.UnreadQAMessageCount(adminPool))
		r.With(middleware.RequireRole(middleware.RoleDQADirector)).
			Get("/api/v1/messages/audiences", handlers.QAAudiences(adminPool))
		r.With(middleware.RequireRole(middleware.RoleDQADirector, middleware.RoleQAOfficer, middleware.RoleQADeptRep, middleware.RoleQASchool)).
			Post("/api/v1/messages/{id}/read", handlers.MarkQAMessageRead(adminPool))
		r.With(middleware.RequireRole(middleware.RoleDQADirector, middleware.RoleQAOfficer, middleware.RoleQADeptRep, middleware.RoleQASchool)).
			Get("/api/v1/messages/{id}/attachment", handlers.QAMessageAttachment(adminPool))
		// Dismiss (the ✕) — clears it from YOUR inbox only.
		r.With(middleware.RequireRole(middleware.RoleDQADirector, middleware.RoleQAOfficer, middleware.RoleQADeptRep, middleware.RoleQASchool)).
			Delete("/api/v1/messages/{id}", handlers.DismissQAMessage(adminPool))

		// ── Branding (any authenticated role: own tenant's logo + motto) ─────
		// Feeds the top-left header on every dashboard.
		r.Get("/api/v1/branding", handlers.GetBranding(pool))

		// ── Super-admin ELIMINATED (single-institution build) ────────────────
		// Tenant lifecycle + branding routes are gone: there is ONE institution, and
		// branding now comes from brand.json (served by GetBranding/GetPublicBranding),
		// not a runtime editor.

		// ── Tenant ADMIN: own-tenant settings ────────────────────────────────
		// Attendance threshold stays editable inside the tenant — by the tenant
		// ADMIN here and by the DQA Director page below (same handlers).
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Get("/api/v1/admin/settings/thresholds", handlers.GetThresholds(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Put("/api/v1/admin/settings/thresholds", handlers.PutThresholds(pool, rdb))

		// Tenant-configurable intakes (#1) — admin-defined, used at student registration.
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Get("/api/v1/admin/settings/intakes", handlers.GetIntakes(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Put("/api/v1/admin/settings/intakes", handlers.PutIntakes(pool))

		// Tenant-configurable course levels + study sessions (admin-defined).
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Get("/api/v1/admin/settings/levels", handlers.GetLevels(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Put("/api/v1/admin/settings/levels", handlers.PutLevels(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Get("/api/v1/admin/settings/study-sessions", handlers.GetStudySessions(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Put("/api/v1/admin/settings/study-sessions", handlers.PutStudySessions(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Get("/api/v1/admin/settings/staff-id-prefix", handlers.GetStaffIDPrefix(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Put("/api/v1/admin/settings/staff-id-prefix", handlers.PutStaffIDPrefix(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Get("/api/v1/admin/settings/titles", handlers.GetTitles(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Put("/api/v1/admin/settings/titles", handlers.PutTitles(pool))

		// Tenant-configurable daily session window (coordinators may only begin a
		// session inside it; default 08:00–17:00 Mon–Sat).
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Get("/api/v1/admin/settings/session-window", handlers.GetSessionWindow(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Put("/api/v1/admin/settings/session-window", handlers.PutSessionWindow(pool))

		// Separate fixed passcode gating the admin Users page.
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Get("/api/v1/admin/settings/users-passcode", handlers.GetUsersPasscodeStatus(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Put("/api/v1/admin/settings/users-passcode", handlers.PutUsersPasscode(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Post("/api/v1/admin/settings/users-passcode/verify", handlers.VerifyUsersPasscode(pool))

		// ── Per-tenant admin sub-resources (ADMIN, confined to their own tenant).
		// RequireOwnTenant blocks a tenant ADMIN from managing another tenant by
		// changing the {tenant_id} in the path (these run on the no-RLS adminPool).
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/users", handlers.ListTenantUsers(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/users", handlers.CreateUser(adminPool))
		// ── Schools/colleges + departments (org foundation) ───────────────────
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/schools", handlers.ListSchools(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/schools", handlers.CreateSchool(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Delete("/api/v1/admin/tenants/{tenant_id}/schools/{school_id}", handlers.DeleteSchool(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/departments", handlers.ListDepartments(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/departments", handlers.CreateDepartment(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Delete("/api/v1/admin/tenants/{tenant_id}/departments/{department_id}", handlers.DeleteDepartment(adminPool))

		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/courses", handlers.ListCourses(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/courses", handlers.CreateCourse(adminPool))
		// Curriculum bulk import/export (courses · units/roadmap · lecturer mapping).
		// Each export.xlsx doubles as the import template.
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/courses/import", handlers.ImportCourses(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/courses/export.xlsx", handlers.ExportCoursesXLSX(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/course-units/import", handlers.ImportCourseUnits(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/course-units/export.xlsx", handlers.ExportCourseUnitsXLSX(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/lecturer-assignments/import", handlers.ImportLecturerAssignmentsFile(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/lecturer-assignments/export.xlsx", handlers.ExportLecturerAssignmentsXLSX(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Patch("/api/v1/admin/tenants/{tenant_id}/academic-period", handlers.UpdateTenantAcademicPeriod(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/academic-period/advance", handlers.AdvanceAcademicPeriod(adminPool))
		// End-of-semester clear: intake-scoped, archive-first wipe (password-gated).
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/clear-semester-data", handlers.ClearSemesterData(adminPool))
		// Semester archives (zips created by the clear) — list / download / delete.
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/semester-archives", handlers.ListSemesterArchives(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/semester-archives/{archive_id}/download", handlers.DownloadSemesterArchive(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Delete("/api/v1/admin/tenants/{tenant_id}/semester-archives/{archive_id}", handlers.DeleteSemesterArchive(adminPool))
		// ── Rooms / room codes (the venues registry) ──────────────────────────
		// /venues stays as the legacy alias of /rooms so older clients keep working.
		adminOwn := chi.Middlewares{middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant}
		for _, base := range []string{"/api/v1/admin/tenants/{tenant_id}/rooms", "/api/v1/admin/tenants/{tenant_id}/venues"} {
			r.With(adminOwn...).Get(base, handlers.ListRooms(adminPool))
			r.With(adminOwn...).Post(base, handlers.CreateRoom(adminPool))
			r.With(adminOwn...).Patch(base+"/{room_code}", handlers.UpdateRoom(adminPool))
			r.With(adminOwn...).Delete(base+"/{room_code}", handlers.DeleteRoom(adminPool))
			r.With(adminOwn...).Post(base+"/import", handlers.ImportRooms(adminPool))
			r.With(adminOwn...).Get(base+"/export.xlsx", handlers.ExportRoomsXLSX(adminPool))
		}
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/students", handlers.ListStudents(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/students", handlers.CreateStudent(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Patch("/api/v1/admin/tenants/{tenant_id}/students", handlers.UpdateStudent(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Delete("/api/v1/admin/tenants/{tenant_id}/students", handlers.DeleteStudent(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/students/export.xlsx", handlers.ExportStudentsXLSX(adminPool))

		// Coordinators directory (contacts + course/level/session) + Excel import/export.
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/coordinators", handlers.ListCoordinators(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/coordinators/export.xlsx", handlers.ExportCoordinatorsXLSX(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/coordinators/import", handlers.ImportCoordinators(adminPool))

		// Offerings = (program + study session), each with its own coordinator (own tenant).
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/offerings", handlers.ListOfferings(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/offerings", handlers.CreateOffering(adminPool))
		// Apply one cohort to EVERY course at once (coordinators assigned later).
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/cohorts/apply-all", handlers.ApplyCohortAllCourses(adminPool))

		// Admin — lecturers + assignments (own tenant)
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/lecturers", handlers.ListLecturers(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/lecturers", handlers.CreateLecturer(adminPool))
		// Bulk lecturer import (CSV/XLSX) + filtered export.
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/lecturers/import", handlers.ImportLecturers(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/lecturers/export.xlsx", handlers.ExportLecturersXLSX(adminPool))
		// Issue a one-time biometric-enrolment link the lecturer opens on their phone.
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/lecturers/{lecturer_id}/enroll-link", handlers.AdminLecturerEnrollLink(adminPool, rdb))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/lecturer-assignments", handlers.ListLecturerAssignments(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/lecturer-assignments", handlers.CreateLecturerAssignment(adminPool))

		// Admin — resource-by-id routes (no {tenant_id} in path). The resource IDs
		// are unguessable and cannot be enumerated across tenants via the now
		// own-tenant-scoped listing endpoints above.
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Patch("/api/v1/admin/users/{user_id}/status", handlers.SetUserStatus(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Patch("/api/v1/admin/users/{user_id}", handlers.UpdateUser(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Delete("/api/v1/admin/users/{user_id}", handlers.DeleteUser(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Get("/api/v1/admin/courses/{course_id}/units", handlers.ListCourseUnits(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Post("/api/v1/admin/courses/{course_id}/units", handlers.CreateCourseUnit(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Patch("/api/v1/admin/courses/{course_id}", handlers.UpdateCourse(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Patch("/api/v1/admin/courses/{course_id}/units/{unit_id}", handlers.UpdateCourseUnit(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Get("/api/v1/admin/courses/{course_id}/roadmap", handlers.GetCourseRoadmap(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Delete("/api/v1/admin/lecturer-assignments/{assignment_id}", handlers.DeleteLecturerAssignment(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Delete("/api/v1/admin/offerings/{offering_id}", handlers.DeleteOffering(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Patch("/api/v1/admin/offerings/{offering_id}", handlers.UpdateOffering(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Patch("/api/v1/admin/lecturers/{lecturer_id}", handlers.UpdateLecturer(adminPool))

		// Admin — employees (general staff) registry + tablet attendance (own tenant).
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/employees", handlers.ListEmployees(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/employees", handlers.CreateEmployee(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/employees/import", handlers.ImportEmployees(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/employees/export.xlsx", handlers.ExportEmployeesXLSX(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Patch("/api/v1/admin/employees/{id}", handlers.UpdateEmployee(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Delete("/api/v1/admin/employees/{id}", handlers.DeleteEmployee(adminPool))
		// Tablet punch import + the admin attendance report (with auto-comments).
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/employee-attendance/import", handlers.ImportEmployeePunches(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/employee-attendance", handlers.EmployeeAttendanceReport(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/employee-attendance/export.xlsx", handlers.EmployeeAttendanceExport(adminPool, "xlsx"))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/employee-attendance/export.csv", handlers.EmployeeAttendanceExport(adminPool, "csv"))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/employee-attendance/export.pdf", handlers.EmployeeAttendanceExport(adminPool, "pdf"))

		// Coordinator — lecturer dropdown for a course unit
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Get("/api/v1/coordinator/units/{unit_id}/lecturers", handlers.GetUnitLecturers(pool))

		// Coordinator — per-unit session schedule (set once, then locked)
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Get("/api/v1/coordinator/units/{unit_id}/schedule", handlers.GetUnitSchedule(pool))
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Put("/api/v1/coordinator/units/{unit_id}/schedule", handlers.SetUnitSchedule(pool, rdb))

		// Coordinator — close a session (writes gate_close_time + contact_hours)
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Post("/api/v1/sessions/{session_id}/close", handlers.CloseSession(pool))

		// Coordinator — generate a signed lecturer gate QR for the active session
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Get("/api/v1/sessions/{session_id}/lecturer-gate-qr", handlers.LecturerGateQR(pool))

		// Coordinator dashboard (their offering: units+lecturers+schedule, students, last roster)
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Get("/api/v1/coordinator/overview", handlers.CoordinatorOverview(pool))
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Get("/api/v1/coordinator/students", handlers.CoordinatorStudents(pool))
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Get("/api/v1/coordinator/last-roster", handlers.CoordinatorLastRoster(pool))
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Get("/api/v1/coordinator/attendance", handlers.CoordinatorAttendance(pool))
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Get("/api/v1/coordinator/trends", handlers.CoordinatorTrends(pool))
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Get("/api/v1/coordinator/active-sessions", handlers.CoordinatorActiveSessions(pool))

		// Emergency standby coordinator — the coordinator delegates their cohort's
		// session to a student of that cohort (coordinator-only; never an admin).
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Post("/api/v1/coordinator/standby", handlers.CreateCoordinatorStandby(adminPool))
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Get("/api/v1/coordinator/standby", handlers.ListCoordinatorStandby(adminPool))
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Post("/api/v1/coordinator/standby/{id}/revoke", handlers.RevokeCoordinatorStandby(adminPool))

		// Lecturer dashboard (their assigned units + student attendance matrix).
		r.With(middleware.RequireRole(middleware.RoleLecturer)).
			Get("/api/v1/lecturer/overview", handlers.LecturerOverview(adminPool))
		r.With(middleware.RequireRole(middleware.RoleLecturer)).
			Get("/api/v1/lecturer/attendance", handlers.LecturerAttendance(adminPool))
		// Unit-centric teaching calendar: timetabled sessions per course unit, with the cohorts
		// attending each as sub-tags and their attendance broken out per cohort.
		r.With(middleware.RequireRole(middleware.RoleLecturer)).
			Get("/api/v1/lecturer/calendar", handlers.LecturerCalendar(adminPool))
		// Lecturer roster & analytics: enrolled/attended students across cohorts, sessions,
		// and per-session present/absent — all sortable/filterable by the app.
		r.With(middleware.RequireRole(middleware.RoleLecturer)).
			Get("/api/v1/lecturer/roster", handlers.LecturerRoster(adminPool))
		r.With(middleware.RequireRole(middleware.RoleLecturer)).
			Get("/api/v1/lecturer/sessions", handlers.LecturerSessions(adminPool))
		r.With(middleware.RequireRole(middleware.RoleLecturer)).
			Get("/api/v1/lecturer/sessions/{session_id}/students", handlers.LecturerSessionStudents(adminPool))

		// ── Cross-role in-app notifications ──────────────────────────────────
		// Lecturer → his students / the coordinator; Coordinator → his students / the
		// lecturers. Students, coordinators and lecturers all read their own inbox.
		r.With(middleware.RequireRole(middleware.RoleLecturer, middleware.RoleCoordinator, middleware.RoleHOD, middleware.RoleDean, middleware.RoleQADeptRep, middleware.RoleQASchool)).
			Post("/api/v1/app-notifications", handlers.SendAppNotification(adminPool))
		r.With(middleware.RequireRole(middleware.RoleStudent, middleware.RoleCoordinator, middleware.RoleLecturer, middleware.RoleHOD, middleware.RoleDean, middleware.RoleQADeptRep, middleware.RoleQASchool)).
			Get("/api/v1/app-notifications", handlers.ListAppNotifications(adminPool))
		r.With(middleware.RequireRole(middleware.RoleStudent, middleware.RoleCoordinator, middleware.RoleLecturer, middleware.RoleHOD, middleware.RoleDean, middleware.RoleQADeptRep, middleware.RoleQASchool)).
			Get("/api/v1/app-notifications/unread-count", handlers.UnreadAppNotificationCount(adminPool))
		r.With(middleware.RequireRole(middleware.RoleStudent, middleware.RoleCoordinator, middleware.RoleLecturer, middleware.RoleHOD, middleware.RoleDean, middleware.RoleQADeptRep, middleware.RoleQASchool)).
			Post("/api/v1/app-notifications/{id}/read", handlers.MarkAppNotificationRead(adminPool))
		// Dismiss (the ✕ on every alert) — clears it from YOUR inbox, not everyone else's.
		r.With(middleware.RequireRole(middleware.RoleStudent, middleware.RoleLecturer, middleware.RoleCoordinator,
			middleware.RoleAdmin, middleware.RoleQAOfficer, middleware.RoleDQADirector, middleware.RoleVC, middleware.RoleDVC)).
			Delete("/api/v1/app-notifications/{id}", handlers.DismissAppNotification(adminPool))
		// Lecturers of the coordinator's course units (for the "notify a specific lecturer" picker).
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Get("/api/v1/coordinator/lecturers", handlers.CoordinatorLecturers(adminPool))

		// Admin — lecturer attendance logs + summary (own tenant)
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/lecturer-attendance", handlers.GetLecturerAttendanceLogs(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/lecturer-attendance/summary", handlers.GetLecturerAttendanceSummary(adminPool))

		// ── Exports ──────────────────────────────────────────────────────────
		r.With(middleware.RequireRole(middleware.RoleVC, middleware.RoleDVC)).
			Get("/api/v1/reports/vc/audit.pdf", handlers.VCAuditPDF(pool))
		r.With(middleware.RequireRole(middleware.RoleDQADirector)).
			Get("/api/v1/reports/dqa/eligibility.csv", handlers.DQAEligibilityCSV(pool))

		// ── SIS Import ───────────────────────────────────────────────────────
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleDQADirector)).
			Post("/api/v1/import/csv", handlers.ImportCSV(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Post("/api/v1/import/trigger", handlers.ImportTrigger(pool))
		// Bulk timetable import (CSV/XLSX → weekly slots, auto-resolves offering/unit/lecturer).
		r.With(middleware.RequireRole(middleware.RoleAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/timetable/import", handlers.ImportTimetable(adminPool))

		// ── Dashboards ────────────────────────────────────────────────────────
		r.With(middleware.RequireRole(middleware.RoleVC, middleware.RoleDVC)).
			Get("/api/v1/dashboard/vc/overview", handlers.VCOverview(pool))
		r.With(middleware.RequireRole(middleware.RoleVC, middleware.RoleDVC)).
			Get("/api/v1/dashboard/vc/lecturer-workload", handlers.VCLecturerWorkload(pool))

		r.With(middleware.RequireRole(middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/dqa/thresholds", handlers.GetThresholds(pool))
		r.With(middleware.RequireRole(middleware.RoleDQADirector)).
			Put("/api/v1/dashboard/dqa/thresholds", handlers.PutThresholds(pool, rdb))
		r.With(middleware.RequireRole(middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/dqa/course-health", handlers.DQACourseHealth(pool))
		r.With(middleware.RequireRole(middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/dqa/trends", handlers.DQATrends(pool))
		r.With(middleware.RequireRole(middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/dqa/punctuality", handlers.DQAPunctuality(pool))
		r.With(middleware.RequireRole(middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/dqa/ineligible", handlers.DQABulkIneligible(pool))
		r.With(middleware.RequireRole(middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/dqa/eligibility-all", handlers.DQAAllEligibility(pool))

		// Timetable (ADMIN + QA OFFICER): view the coordinator-filled weekly schedule
		// of every offering's units, with override power on the PUT.
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleQAOfficer)).
			Get("/api/v1/dashboard/timetable", handlers.TimetableOverview(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleQAOfficer)).
			Put("/api/v1/dashboard/timetable", handlers.SetTimetableSchedule(pool, rdb))
		// The tenant's active rooms, for the timetable grid's room picker — read-only and
		// RLS-scoped, so every dashboard role that shows a room can ask for the list.
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleQAOfficer, middleware.RoleDQADirector,
			middleware.RoleCoordinator, middleware.RoleHOD, middleware.RoleDean,
			middleware.RoleQASchool, middleware.RoleQADeptRep)).
			Get("/api/v1/dashboard/rooms", handlers.DashboardRooms(pool))

		// Multi-slot weekly timetable grid (one slot per unit per day, with room).
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleQAOfficer)).
			Get("/api/v1/dashboard/timetable/slots", handlers.GetTimetableSlots(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleQAOfficer)).
			Put("/api/v1/dashboard/timetable/slots", handlers.UpsertTimetableSlot(pool, rdb))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleQAOfficer)).
			Delete("/api/v1/dashboard/timetable/slots/{slot_id}", handlers.DeleteTimetableSlot(pool))

		r.With(middleware.RequireRole(middleware.RoleQAOfficer)).
			Post("/api/v1/dashboard/qa/device-reset", handlers.QADeviceReset(pool))
		r.With(middleware.RequireRole(middleware.RoleQAOfficer)).
			Post("/api/v1/dashboard/qa/attendance-correction", handlers.QAManualCorrection(pool))
		r.With(middleware.RequireRole(middleware.RoleQAOfficer)).
			Get("/api/v1/dashboard/qa/coordinator-health", handlers.QACoordinatorHealth(pool))

		// Student attendance (ADMIN + QA + VC + DQA): filterable summary + Excel export/import.
		// ADMIN reaches this from the Reports hub; it supports course_id/unit_id/session/
		// year/semester filters so the report drills into real courses and their units.
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleQAOfficer, middleware.RoleVC, middleware.RoleDVC, middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/qa/student-attendance", handlers.QAStudentAttendance(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleQAOfficer, middleware.RoleVC, middleware.RoleDVC, middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/qa/student-attendance/export.xlsx", handlers.QAStudentAttendanceReport(pool, "xlsx"))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleQAOfficer, middleware.RoleVC, middleware.RoleDVC, middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/qa/student-attendance/export.csv", handlers.QAStudentAttendanceReport(pool, "csv"))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleQAOfficer, middleware.RoleVC, middleware.RoleDVC, middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/qa/student-attendance/export.pdf", handlers.QAStudentAttendanceReport(pool, "pdf"))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleQAOfficer)).
			Post("/api/v1/dashboard/qa/student-attendance/import", handlers.QAStudentAttendanceImport(pool))

		// Lecturer attendance for the oversight dashboards (QA, VC, DQA Director).
		r.With(middleware.RequireRole(middleware.RoleQAOfficer, middleware.RoleVC, middleware.RoleDVC, middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/lecturer-attendance", handlers.LecturerAttendanceLogsForCaller(pool))
		r.With(middleware.RequireRole(middleware.RoleQAOfficer, middleware.RoleVC, middleware.RoleDVC, middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/lecturer-attendance/summary", handlers.LecturerAttendanceSummaryForCaller(pool))
		// ADMIN is included here (unlike the JSON routes above, which the admin console
		// reaches through its own /admin/tenants/{id} pair): the export is caller-tenant
		// scoped from the JWT, so an admin downloading it can only ever get their own.
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleQAOfficer, middleware.RoleVC, middleware.RoleDVC, middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/lecturer-attendance/export.xlsx", handlers.LecturerAttendanceExport(pool, "xlsx"))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleQAOfficer, middleware.RoleVC, middleware.RoleDVC, middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/lecturer-attendance/export.csv", handlers.LecturerAttendanceExport(pool, "csv"))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleQAOfficer, middleware.RoleVC, middleware.RoleDVC, middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/lecturer-attendance/export.pdf", handlers.LecturerAttendanceExport(pool, "pdf"))

		// The SECOND, independent record of the same lectures: the QA patroller's spot-checks.
		// Kept on its own endpoint (and its own page) so it can contradict the coordinator's.
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleQAOfficer, middleware.RoleVC, middleware.RoleDVC, middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/lecturer-attendance/patrol", handlers.LecturerPatrolAttendance(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleQAOfficer, middleware.RoleVC, middleware.RoleDVC, middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/lecturer-attendance/patrol/export.xlsx", handlers.LecturerPatrolExport(pool, "xlsx"))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleQAOfficer, middleware.RoleVC, middleware.RoleDVC, middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/lecturer-attendance/patrol/export.csv", handlers.LecturerPatrolExport(pool, "csv"))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleQAOfficer, middleware.RoleVC, middleware.RoleDVC, middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/lecturer-attendance/patrol/export.pdf", handlers.LecturerPatrolExport(pool, "pdf"))

		// ── Session roster (coordinator: who is present in a live session) ────
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Get("/api/v1/sessions/{session_id}/roster", handlers.SessionRoster(pool))
	})

	return r
}

// mustProxy builds a reverse proxy to the given upstream URL or panics at
// startup — an unparseable upstream URL is a fatal misconfiguration.
func mustProxy(target string) http.HandlerFunc {
	h, err := proxy.New(target)
	if err != nil {
		panic("invalid upstream URL " + target + ": " + err.Error())
	}
	return h
}

// correlationID passes X-Correlation-ID through from request or generates one.
func correlationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Correlation-ID")
		if id == "" {
			id = chimiddleware.GetReqID(r.Context())
		}
		w.Header().Set("X-Correlation-ID", id)
		next.ServeHTTP(w, r)
	})
}
