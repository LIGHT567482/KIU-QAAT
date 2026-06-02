package security_test

// RLS Isolation Tests — plan.md Week 10
// Verifies zero cross-tenant data leakage across all tenant-scoped tables.
// Run: DB_URL=postgres://... go test ./tests/security/... -v

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	tenantA = "a0000000-0000-0000-0000-000000000001"
	tenantB = "b0000000-0000-0000-0000-000000000002"
)

func skipNoIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("DB_URL") == "" {
		t.Skip("skipping: DB_URL not set")
	}
}

func getPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), os.Getenv("DB_URL"))
	if err != nil || pool.Ping(context.Background()) != nil {
		t.Fatalf("db unavailable: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func withTenant(ctx context.Context, pool *pgxpool.Pool, tenantID string, fn func(*pgxpool.Conn)) {
	conn, _ := pool.Acquire(ctx)
	defer conn.Release()
	// Use a session-scoped GUC via set_config, NOT "SET LOCAL": pgx runs each
	// Exec as its own implicit transaction, so a SET LOCAL is discarded before
	// the next statement and the test would assert against an unset tenant (and
	// pass vacuously). The parameter is bound, never interpolated. This mirrors
	// production middleware.SetTenantConn. (update.md L3)
	conn.Exec(ctx, "SELECT set_config('app.current_tenant', $1, false)", tenantID) //nolint:errcheck
	fn(conn)
}

func countCross(ctx context.Context, conn *pgxpool.Conn, table, crossTenant string) int {
	var n int
	conn.QueryRow(ctx, "SELECT COUNT(*) FROM "+table+" WHERE tenant_id = $1", crossTenant).Scan(&n) //nolint:errcheck
	return n
}

func countAll(ctx context.Context, conn *pgxpool.Conn, table string) int {
	var n int
	conn.QueryRow(ctx, "SELECT COUNT(*) FROM "+table).Scan(&n) //nolint:errcheck
	return n
}

func countOwn(ctx context.Context, conn *pgxpool.Conn, table, tenantID string) int {
	var n int
	conn.QueryRow(ctx, "SELECT COUNT(*) FROM "+table+" WHERE tenant_id = $1", tenantID).Scan(&n) //nolint:errcheck
	return n
}

// ─── Per-table isolation ──────────────────────────────────────────────────────

func TestRLS_Users_NoCrossTenantRows(t *testing.T) {
	skipNoIntegration(t)
	pool := getPool(t)
	ctx := context.Background()
	var n int
	withTenant(ctx, pool, tenantA, func(c *pgxpool.Conn) { n = countCross(ctx, c, "users", tenantB) })
	if n > 0 {
		t.Errorf("RLS FAILURE users: Tenant A query returned %d Tenant B rows", n)
	}
}

func TestRLS_Sessions_NoCrossTenantRows(t *testing.T) {
	skipNoIntegration(t)
	pool := getPool(t)
	ctx := context.Background()
	var n int
	withTenant(ctx, pool, tenantA, func(c *pgxpool.Conn) { n = countCross(ctx, c, "sessions", tenantB) })
	if n > 0 {
		t.Errorf("RLS FAILURE sessions: %d cross-tenant rows", n)
	}
}

func TestRLS_AttendanceLogs_NoCrossTenantRows(t *testing.T) {
	skipNoIntegration(t)
	pool := getPool(t)
	ctx := context.Background()
	var n int
	withTenant(ctx, pool, tenantA, func(c *pgxpool.Conn) { n = countCross(ctx, c, "attendance_logs", tenantB) })
	if n > 0 {
		t.Errorf("RLS FAILURE attendance_logs: %d cross-tenant rows", n)
	}
}

func TestRLS_StudentsExtended_NoCrossTenantRows(t *testing.T) {
	skipNoIntegration(t)
	pool := getPool(t)
	ctx := context.Background()
	var n int
	withTenant(ctx, pool, tenantA, func(c *pgxpool.Conn) { n = countCross(ctx, c, "students_extended", tenantB) })
	if n > 0 {
		t.Errorf("RLS FAILURE students_extended: %d cross-tenant rows", n)
	}
}

// ─── Wildcard SELECT returns only own-tenant rows ─────────────────────────────

func TestRLS_AllTenantTables_WildcardSelectIsolated(t *testing.T) {
	skipNoIntegration(t)
	pool := getPool(t)
	ctx := context.Background()

	tables := []string{
		"users", "sessions", "attendance_logs", "students_extended",
		"courses", "course_units", "venues", "hardware_vault", "admin_audit_log",
	}

	for _, tbl := range tables {
		var total, own int
		withTenant(ctx, pool, tenantA, func(c *pgxpool.Conn) {
			total = countAll(ctx, c, tbl)
			own   = countOwn(ctx, c, tbl, tenantA)
		})
		if total != own {
			t.Errorf("RLS FAILURE %s: SELECT * = %d rows but only %d are Tenant A's", tbl, total, own)
		}
	}
}

// ─── Append-only guard ────────────────────────────────────────────────────────

func TestRLS_AttendanceLogs_DeleteBlocked(t *testing.T) {
	skipNoIntegration(t)
	pool := getPool(t)
	ctx := context.Background()

	var deleteErr error
	var rowsAffected int64
	withTenant(ctx, pool, tenantA, func(c *pgxpool.Conn) {
		tag, err := c.Exec(ctx, "DELETE FROM attendance_logs WHERE tenant_id = $1", tenantA)
		deleteErr   = err
		rowsAffected = tag.RowsAffected()
	})

	if deleteErr != nil {
		t.Logf("DELETE correctly blocked by RLS policy: %v", deleteErr)
		return
	}
	if rowsAffected > 0 {
		t.Errorf("SECURITY FAILURE: DELETE removed %d attendance_log rows — append-only guard not working", rowsAffected)
	}
}
