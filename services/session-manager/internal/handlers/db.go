package handlers

import (
	"context"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5/pgxpool"
)

// tenantIDPattern matches a canonical UUID — the only valid tenant id form.
var tenantIDPattern = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// setTenantConn activates PostgreSQL RLS for tenantID on conn using a
// parameterised, session-scoped GUC (NOT "SET LOCAL", which pgx discards as its
// own implicit transaction, and NOT fmt.Sprintf, which would be SQL injection).
// Mirrors api-gateway middleware.SetTenantConn. Fixes update.md (clearance.go
// injection + inert SET LOCAL).
func setTenantConn(ctx context.Context, conn *pgxpool.Conn, tenantID string) error {
	if !tenantIDPattern.MatchString(tenantID) {
		return fmt.Errorf("invalid tenant id")
	}
	_, err := conn.Exec(ctx, "SELECT set_config('app.current_tenant', $1, false)", tenantID)
	return err
}
