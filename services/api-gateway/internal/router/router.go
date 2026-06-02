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
	qrProxy      := mustProxy(upstreams.QRGenerator)
	sessionProxy := mustProxy(upstreams.SessionManager)
	syncProxy    := mustProxy(upstreams.SyncReceiver)

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
	r.Get("/health",    handlers.Health)
	r.Get("/api/v1/health", handlers.Health)
	r.Get("/metrics",   promhttp.Handler().ServeHTTP)  // Prometheus scrape endpoint

	// Student check-in is public: students have no JWT. It is authenticated by
	// the signed QR + rotating room code inside the handler. A per-IP limiter
	// blunts scripted abuse / room-code brute force.
	r.With(middleware.PublicIPRateLimit(5, 10)).
		Post("/api/v1/checkin", handlers.Checkin(pool))
	r.Get("/checkin", handlers.CheckinPage)

	// ─── Authenticated routes ─────────────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(middleware.JWTAuth(publicKey, jwtIssuer, jwtAudience, rdb))
		r.Use(middleware.SetTenant(pool))
		r.Use(middleware.CoordinatorRateLimit())
		r.Use(middleware.AuditLog(pool))

		// ── Daily Manifest ────────────────────────────────────────────────────
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Get("/api/v1/manifest/daily", handlers.ManifestDaily(pool, rdb))

		// ── QR Management (→ qr-generator) ────────────────────────────────────
		r.With(middleware.RequireRole(middleware.RoleDQADirector, middleware.RoleAdmin)).
			Post("/api/v1/qr/generate/batch", qrProxy)

		r.With(middleware.RequireRole(middleware.RoleQAOfficer)).
			Post("/api/v1/qr/reissue", qrProxy)

		// ── Session Management (→ session-manager) ────────────────────────────
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Post("/api/v1/sessions/warden-link", sessionProxy)

		// ── Online check-in session lifecycle (handled in-gateway) ────────────
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Post("/api/v1/sessions/open", handlers.OpenSession(pool))
		r.With(middleware.RequireRole(middleware.RoleCoordinator)).
			Get("/api/v1/sessions/{session_id}/checkin-code", handlers.CheckinCode(pool))

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
		r.With(middleware.RequireRole(middleware.RoleQAOfficer, middleware.RoleDQADirector, middleware.RoleVC)).
			Get("/api/v1/eligibility/{student_id}", handlers.GetEligibility(pool))

		r.With(middleware.RequireRole(middleware.RoleDQADirector)).
			Post("/api/v1/eligibility/clearance-token", sessionProxy)

		// ── Admin (tenant + user management) ─────────────────────────────────
		// These are platform-level, cross-tenant operations gated by the ADMIN
		// role, so they run on the privileged adminPool (see router.New / C1).
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Get("/api/v1/admin/tenants", handlers.ListTenants(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Post("/api/v1/admin/tenants", handlers.CreateTenant(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Patch("/api/v1/admin/tenants/{tenant_id}/status", handlers.SetTenantStatus(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Get("/api/v1/admin/tenants/{tenant_id}/users", handlers.ListTenantUsers(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Post("/api/v1/admin/tenants/{tenant_id}/users", handlers.CreateUser(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Patch("/api/v1/admin/users/{user_id}/status", handlers.SetUserStatus(adminPool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Post("/api/v1/admin/tenants/{tenant_id}/beacons", handlers.RegisterBeacon(adminPool))

		// ── Exports ──────────────────────────────────────────────────────────
		r.With(middleware.RequireRole(middleware.RoleVC)).
			Get("/api/v1/reports/vc/audit.pdf", handlers.VCAuditPDF(pool))
		r.With(middleware.RequireRole(middleware.RoleDQADirector)).
			Get("/api/v1/reports/dqa/eligibility.csv", handlers.DQAEligibilityCSV(pool))

		// ── SIS Import ───────────────────────────────────────────────────────
		r.With(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleDQADirector)).
			Post("/api/v1/import/csv", handlers.ImportCSV(pool))
		r.With(middleware.RequireRole(middleware.RoleAdmin)).
			Post("/api/v1/import/trigger", handlers.ImportTrigger(pool))

		// ── Dashboards ────────────────────────────────────────────────────────
		r.With(middleware.RequireRole(middleware.RoleVC)).
			Get("/api/v1/dashboard/vc/overview", handlers.VCOverview(pool))

		r.With(middleware.RequireRole(middleware.RoleDQADirector)).
			Get("/api/v1/dashboard/dqa/thresholds", handlers.GetThresholds(pool))
		r.With(middleware.RequireRole(middleware.RoleDQADirector)).
			Put("/api/v1/dashboard/dqa/thresholds", handlers.PutThresholds(pool, rdb))

		r.With(middleware.RequireRole(middleware.RoleQAOfficer)).
			Get("/api/v1/dashboard/qa/live-sessions", handlers.QALiveSessions(pool))
		r.With(middleware.RequireRole(middleware.RoleQAOfficer)).
			Post("/api/v1/dashboard/qa/device-reset", handlers.QADeviceReset(pool))
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

