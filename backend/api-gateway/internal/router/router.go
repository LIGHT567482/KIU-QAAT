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
	QRGenerator    string
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
	qrProxy := mustProxy(upstreams.QRGenerator)
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
	r.Get("/metrics", promhttp.Handler().ServeHTTP) // Prometheus scrape endpoint

	// Student check-in is public: students have no JWT. It is authenticated by
	// the signed QR + rotating room code inside the handler. A per-IP limiter
	// blunts scripted abuse / room-code brute force.
	r.With(middleware.PublicIPRateLimit(5, 10)).
		Post("/api/v1/checkin", handlers.Checkin(pool))
	r.Get("/checkin", handlers.CheckinPage)

	// Passwordless portal login by scanning the student's personal QR (public:
	// the QR is the credential, cryptographically verified inside the handler).
	r.With(middleware.PublicIPRateLimit(5, 20)).
		Post("/api/v1/student/qr-login", handlers.StudentQRLogin(adminPool))

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

	// Lecturer passwordless dashboard login by scanning their personal QR (public:
	// the HMAC-signed QR is the credential, verified inside the handler).
	r.With(middleware.PublicIPRateLimit(10, 60)).
		Post("/api/v1/lecturer/qr-login", handlers.LecturerQRLogin(adminPool))

	// Lecturer PORTAL — passwordless, read-only. Enter institution + staff ID and
	// search the attendance logs of your own units (rate-limited to slow enumeration).
	r.With(middleware.PublicIPRateLimit(30, 60)).
		Get("/api/v1/lecturer-portal/overview", handlers.LecturerPortalOverview(adminPool))
	r.With(middleware.PublicIPRateLimit(30, 60)).
		Get("/api/v1/lecturer-portal/attendance", handlers.LecturerPortalAttendance(adminPool))

	// Coordinator passwordless dashboard login by scanning their personal QR (public:
	// the HMAC-signed QR encodes coordinator_id|offering_id|tenant_id).
	r.With(middleware.PublicIPRateLimit(10, 60)).
		Post("/api/v1/coordinator/qr-login", handlers.CoordinatorQRLogin(adminPool))

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

		// ── QR Management (→ qr-generator) ────────────────────────────────────
		r.With(middleware.RequireRole(middleware.RoleDQADirector, middleware.RoleAdmin)).
			Post("/api/v1/qr/generate/batch", qrProxy)

		r.With(middleware.RequireRole(middleware.RoleQAOfficer)).
			Post("/api/v1/qr/reissue", qrProxy)

		// Live QR token for display in the admin students table (no email).
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin)).
			Post("/api/v1/qr/token", qrProxy)

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
		// Student's own QR code (signed token + image) for download from the portal.
		r.With(middleware.RequireRole(middleware.RoleStudent)).
			Get("/api/v1/student/my-qr", handlers.StudentMyQR(adminPool))

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
		r.With(middleware.RequireRole(middleware.RoleDQADirector, middleware.RoleQAOfficer)).
			Post("/api/v1/messages", handlers.SendQAMessage(adminPool))
		r.With(middleware.RequireRole(middleware.RoleDQADirector, middleware.RoleQAOfficer)).
			Get("/api/v1/messages", handlers.ListQAMessages(adminPool))
		r.With(middleware.RequireRole(middleware.RoleDQADirector, middleware.RoleQAOfficer)).
			Get("/api/v1/messages/unread-count", handlers.UnreadQAMessageCount(adminPool))
		r.With(middleware.RequireRole(middleware.RoleDQADirector)).
			Get("/api/v1/messages/audiences", handlers.QAAudiences(adminPool))
		r.With(middleware.RequireRole(middleware.RoleDQADirector, middleware.RoleQAOfficer)).
			Post("/api/v1/messages/{id}/read", handlers.MarkQAMessageRead(adminPool))
		r.With(middleware.RequireRole(middleware.RoleDQADirector, middleware.RoleQAOfficer)).
			Get("/api/v1/messages/{id}/attachment", handlers.QAMessageAttachment(adminPool))

		// ── Branding (any authenticated role: own tenant's logo + motto) ─────
		// Feeds the top-left header on every dashboard.
		r.Get("/api/v1/branding", handlers.GetBranding(pool))

		// ── Super-admin ELIMINATED (single-institution build) ────────────────
		// Tenant lifecycle + branding routes are gone: there is ONE institution, and
		// branding now comes from brand.json (served by GetBranding/GetPublicBranding),
		// not a runtime editor. The SUPER_ADMIN role + its handlers are left dormant.

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

		// ── Per-tenant admin sub-resources (ADMIN for own tenant; SUPER_ADMIN any).
		// RequireOwnTenant blocks a tenant ADMIN from managing another tenant by
		// changing the {tenant_id} in the path (these run on the no-RLS adminPool).
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/users", handlers.ListTenantUsers(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/users", handlers.CreateUser(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/courses", handlers.ListCourses(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/courses", handlers.CreateCourse(adminPool))
		// Curriculum bulk import/export (courses · units/roadmap · lecturer mapping).
		// Each export.xlsx doubles as the import template.
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/courses/import", handlers.ImportCourses(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/courses/export.xlsx", handlers.ExportCoursesXLSX(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/course-units/import", handlers.ImportCourseUnits(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/course-units/export.xlsx", handlers.ExportCourseUnitsXLSX(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/lecturer-assignments/import", handlers.ImportLecturerAssignmentsFile(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/lecturer-assignments/export.xlsx", handlers.ExportLecturerAssignmentsXLSX(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Patch("/api/v1/admin/tenants/{tenant_id}/academic-period", handlers.UpdateTenantAcademicPeriod(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/academic-period/advance", handlers.AdvanceAcademicPeriod(adminPool))
		// End-of-semester clear: intake-scoped, archive-first wipe (password-gated).
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/clear-semester-data", handlers.ClearSemesterData(adminPool))
		// Semester archives (zips created by the clear) — list / download / delete.
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/semester-archives", handlers.ListSemesterArchives(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/semester-archives/{archive_id}/download", handlers.DownloadSemesterArchive(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Delete("/api/v1/admin/tenants/{tenant_id}/semester-archives/{archive_id}", handlers.DeleteSemesterArchive(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/venues", handlers.ListVenues(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/venues", handlers.CreateVenue(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/students", handlers.ListStudents(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/students", handlers.CreateStudent(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Patch("/api/v1/admin/tenants/{tenant_id}/students", handlers.UpdateStudent(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Delete("/api/v1/admin/tenants/{tenant_id}/students", handlers.DeleteStudent(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/students/export.xlsx", handlers.ExportStudentsXLSX(adminPool))

		// Coordinators directory (contacts + course/level/session) + Excel import/export.
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/coordinators", handlers.ListCoordinators(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/coordinators/export.xlsx", handlers.ExportCoordinatorsXLSX(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/coordinators/import", handlers.ImportCoordinators(adminPool))
		// The coordinator's personal QR (scan → passwordless dashboard login scoped to their cohort).
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/coordinators/{user_id}/qr", handlers.AdminCoordinatorQR(adminPool))

		// Offerings = (program + study session), each with its own coordinator (own tenant).
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/offerings", handlers.ListOfferings(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/offerings", handlers.CreateOffering(adminPool))
		// Apply one cohort to EVERY course at once (coordinators assigned later).
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/cohorts/apply-all", handlers.ApplyCohortAllCourses(adminPool))

		// Admin — lecturers + assignments (own tenant)
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/lecturers", handlers.ListLecturers(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/lecturers", handlers.CreateLecturer(adminPool))
		// Bulk lecturer import (CSV/XLSX) + filtered export.
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/lecturers/import", handlers.ImportLecturers(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/lecturers/export.xlsx", handlers.ExportLecturersXLSX(adminPool))
		// Issue a one-time biometric-enrolment link the lecturer opens on their phone.
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/lecturers/{lecturer_id}/enroll-link", handlers.AdminLecturerEnrollLink(adminPool, rdb))
		// The lecturer's personal QR (scan → passwordless dashboard login).
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/lecturers/{lecturer_id}/qr", handlers.AdminLecturerQR(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/lecturer-assignments", handlers.ListLecturerAssignments(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Post("/api/v1/admin/tenants/{tenant_id}/lecturer-assignments", handlers.CreateLecturerAssignment(adminPool))

		// Admin — resource-by-id routes (no {tenant_id} in path). The resource IDs
		// are unguessable and cannot be enumerated across tenants via the now
		// own-tenant-scoped listing endpoints above.
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin)).
			Patch("/api/v1/admin/users/{user_id}/status", handlers.SetUserStatus(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin)).
			Patch("/api/v1/admin/users/{user_id}", handlers.UpdateUser(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin)).
			Delete("/api/v1/admin/users/{user_id}", handlers.DeleteUser(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin)).
			Get("/api/v1/admin/courses/{course_id}/units", handlers.ListCourseUnits(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin)).
			Post("/api/v1/admin/courses/{course_id}/units", handlers.CreateCourseUnit(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin)).
			Patch("/api/v1/admin/courses/{course_id}", handlers.UpdateCourse(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin)).
			Patch("/api/v1/admin/courses/{course_id}/units/{unit_id}", handlers.UpdateCourseUnit(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin)).
			Get("/api/v1/admin/courses/{course_id}/roadmap", handlers.GetCourseRoadmap(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin)).
			Delete("/api/v1/admin/lecturer-assignments/{assignment_id}", handlers.DeleteLecturerAssignment(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin)).
			Delete("/api/v1/admin/offerings/{offering_id}", handlers.DeleteOffering(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin)).
			Patch("/api/v1/admin/offerings/{offering_id}", handlers.UpdateOffering(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin)).
			Patch("/api/v1/admin/lecturers/{lecturer_id}", handlers.UpdateLecturer(adminPool))

			// Admin — employees (general staff) registry + tablet attendance (own tenant).
			r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
				Get("/api/v1/admin/tenants/{tenant_id}/employees", handlers.ListEmployees(adminPool))
			r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
				Post("/api/v1/admin/tenants/{tenant_id}/employees", handlers.CreateEmployee(adminPool))
			r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
				Post("/api/v1/admin/tenants/{tenant_id}/employees/import", handlers.ImportEmployees(adminPool))
			r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
				Get("/api/v1/admin/tenants/{tenant_id}/employees/export.xlsx", handlers.ExportEmployeesXLSX(adminPool))
			r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin)).
				Patch("/api/v1/admin/employees/{id}", handlers.UpdateEmployee(adminPool))
			r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin)).
				Delete("/api/v1/admin/employees/{id}", handlers.DeleteEmployee(adminPool))
			// Tablet punch import + the admin attendance report (with auto-comments).
			r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
				Post("/api/v1/admin/tenants/{tenant_id}/employee-attendance/import", handlers.ImportEmployeePunches(adminPool))
			r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
				Get("/api/v1/admin/tenants/{tenant_id}/employee-attendance", handlers.EmployeeAttendanceReport(adminPool))

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

		// Admin — lecturer attendance logs + summary (own tenant)
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
			Get("/api/v1/admin/tenants/{tenant_id}/lecturer-attendance", handlers.GetLecturerAttendanceLogs(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
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
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin), middleware.RequireOwnTenant).
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
		// Multi-slot weekly timetable grid (one slot per unit per day, with room).
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleQAOfficer)).
			Get("/api/v1/dashboard/timetable/slots", handlers.GetTimetableSlots(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleQAOfficer)).
			Put("/api/v1/dashboard/timetable/slots", handlers.UpsertTimetableSlot(pool, rdb))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleQAOfficer)).
			Delete("/api/v1/dashboard/timetable/slots/{slot_id}", handlers.DeleteTimetableSlot(pool))

		r.With(middleware.RequireRole(middleware.RoleQAOfficer)).
			Get("/api/v1/dashboard/qa/live-sessions", handlers.QALiveSessions(pool))
		r.With(middleware.RequireRole(middleware.RoleQAOfficer)).
			Post("/api/v1/dashboard/qa/device-reset", handlers.QADeviceReset(pool))
		r.With(middleware.RequireRole(middleware.RoleQAOfficer)).
			Post("/api/v1/dashboard/qa/attendance-correction", handlers.QAManualCorrection(pool))
		r.With(middleware.RequireRole(middleware.RoleQAOfficer)).
			Get("/api/v1/dashboard/qa/coordinator-health", handlers.QACoordinatorHealth(pool))

		// Student attendance (ADMIN + QA + VC + DQA): filterable summary + Excel export/import.
		// ADMIN reaches this from the Reports hub; it supports course_id/unit_id/session/
		// year/semester filters so the report drills into real courses and their units.
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin, middleware.RoleQAOfficer, middleware.RoleVC, middleware.RoleDVC, middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/qa/student-attendance", handlers.QAStudentAttendance(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin, middleware.RoleQAOfficer, middleware.RoleVC, middleware.RoleDVC, middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/qa/student-attendance/export.xlsx", handlers.QAStudentAttendanceExport(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleSuperAdmin, middleware.RoleQAOfficer)).
			Post("/api/v1/dashboard/qa/student-attendance/import", handlers.QAStudentAttendanceImport(pool))

		// Lecturer attendance for the oversight dashboards (QA, VC, DQA Director).
		r.With(middleware.RequireRole(middleware.RoleQAOfficer, middleware.RoleVC, middleware.RoleDVC, middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/lecturer-attendance", handlers.LecturerAttendanceLogsForCaller(pool))
		r.With(middleware.RequireRole(middleware.RoleQAOfficer, middleware.RoleVC, middleware.RoleDVC, middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/lecturer-attendance/summary", handlers.LecturerAttendanceSummaryForCaller(pool))

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
