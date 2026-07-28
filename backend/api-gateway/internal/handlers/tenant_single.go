package handlers

import (
	"context"
	"os"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Single-institution deployment: there is exactly ONE real tenant, so no user ever supplies an
// institution/org. singleTenantID resolves it as DEFAULT_TENANT_ID (env) or the one non-platform
// tenant row, and caches the first non-empty result.
var (
	tenantMu     sync.Mutex
	cachedTenant string
)

func singleTenantID(ctx context.Context, adminPool *pgxpool.Pool) string {
	tenantMu.Lock()
	defer tenantMu.Unlock()
	if cachedTenant != "" {
		return cachedTenant
	}
	if env := os.Getenv("DEFAULT_TENANT_ID"); env != "" {
		cachedTenant = env
		return cachedTenant
	}
	// The platform sentinel (all-zero UUID) is the retired super-admin home — exclude it.
	_ = adminPool.QueryRow(ctx,
		`SELECT tenant_id::text FROM tenants
		  WHERE tenant_id <> '00000000-0000-0000-0000-000000000000'
		  ORDER BY created_at LIMIT 1`).Scan(&cachedTenant)
	return cachedTenant
}
