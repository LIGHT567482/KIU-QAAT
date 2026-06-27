package middleware

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	"github.com/jackc/pgx/v5/pgxpool"
)

// tenantIDPattern matches a canonical UUID — the only valid form for a tenant
// id. Validating before use prevents SQL injection through the tenant GUC.
var tenantIDPattern = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ValidTenantID reports whether s is a syntactically valid tenant UUID.
func ValidTenantID(s string) bool {
	return tenantIDPattern.MatchString(s)
}

// SetTenantConn activates PostgreSQL RLS for tenantID on conn.
//
// It uses set_config(..., is_local=false) rather than "SET LOCAL": pgx runs
// each Exec/Query as its own implicit transaction, so a SET LOCAL would be
// discarded before the next statement and RLS would silently never apply.
// A session-scoped GUC persists across those statements for the life of the
// connection; the pool's BeforeAcquire hook clears it on checkout so it cannot
// leak between tenants. The tenant id is bound as a parameter and UUID-checked,
// never interpolated into SQL.
func SetTenantConn(ctx context.Context, conn *pgxpool.Conn, tenantID string) error {
	if !ValidTenantID(tenantID) {
		return fmt.Errorf("invalid tenant id")
	}
	_, err := conn.Exec(ctx, "SELECT set_config('app.current_tenant', $1, false)", tenantID)
	return err
}

// SetTenant injects the tenant_id from the JWT into each database connection
// via "SET LOCAL app.current_tenant = '<uuid>'" so PostgreSQL RLS policies
// activate automatically for every query in that transaction.
//
// The pool is stored in context for handlers to retrieve with GetDB(ctx).
func SetTenant(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := GetTenantID(r.Context())
			if tenantID == "" {
				// Should never reach here — JWTAuth runs first.
				http.Error(w, `{"error":"MISSING_TENANT"}`, http.StatusInternalServerError)
				return
			}
			if !ValidTenantID(tenantID) {
				// Malformed tenant claim — refuse rather than risk an unscoped query.
				http.Error(w, `{"error":"INVALID_TENANT"}`, http.StatusForbidden)
				return
			}

			ctx := context.WithValue(r.Context(), ctxDB, &tenantDB{pool: pool, tenantID: tenantID})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type contextDBKey string

const ctxDB contextDBKey = "tenant_db"

type tenantDB struct {
	pool     *pgxpool.Pool
	tenantID string
}

// GetDB retrieves a helper that automatically sets app.current_tenant on any
// acquired connection before returning it to the caller.
func GetDB(ctx context.Context) *tenantDB {
	v, _ := ctx.Value(ctxDB).(*tenantDB)
	return v
}

// Exec runs a query with RLS active for the request's tenant.
func (t *tenantDB) Exec(ctx context.Context, sql string, args ...interface{}) error {
	conn, err := t.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	if err := SetTenantConn(ctx, conn, t.tenantID); err != nil {
		return fmt.Errorf("set tenant: %w", err)
	}
	_, err = conn.Exec(ctx, sql, args...)
	return err
}

// Query runs a SELECT with RLS active and returns the pgx rows.
func (t *tenantDB) Query(ctx context.Context, sql string, args ...interface{}) (interface{ Close() }, error) {
	conn, err := t.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire conn: %w", err)
	}
	// Note: caller must call rows.Close() which will also release conn.
	if err := SetTenantConn(ctx, conn, t.tenantID); err != nil {
		conn.Release()
		return nil, fmt.Errorf("set tenant: %w", err)
	}
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		conn.Release()
		return nil, err
	}
	return rows, nil
}
