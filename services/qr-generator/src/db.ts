// Shared PG pool + tenant-scoped execution.
//
// qr-generator connects as the non-superuser `qaat_app` role, so PostgreSQL RLS
// is enforced (update.md C1). Every query that touches a tenant-scoped table
// (students_extended, tenant_rsa_keys) must therefore run on a connection that
// has `app.current_tenant` set. withTenant() checks out one client, sets the
// GUC (parameterised — never interpolated), runs the callback on that client,
// then clears the GUC and returns the client to the pool.

import pg from 'pg'

const { Pool } = pg

const UUID_RE = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/

let _pool: pg.Pool | null = null

export function pool(): pg.Pool {
  if (!_pool) {
    _pool = new Pool({ connectionString: process.env.DB_URL })
  }
  return _pool
}

// Queryable is the subset of pg's client/pool surface the stores depend on, so
// callers can pass either a tenant-scoped client or (for non-RLS tables) the pool.
export interface Queryable {
  query: pg.Pool['query']
}

export async function withTenant<T>(
  tenantId: string,
  fn: (db: pg.PoolClient) => Promise<T>,
): Promise<T> {
  if (!UUID_RE.test(tenantId)) {
    throw new Error('invalid tenant id')
  }
  const client = await pool().connect()
  try {
    await client.query("SELECT set_config('app.current_tenant', $1, false)", [tenantId])
    return await fn(client)
  } finally {
    // Clear the GUC before the connection is reused for another tenant.
    try {
      await client.query("SELECT set_config('app.current_tenant', '', false)")
    } catch {
      /* ignore — connection is being released regardless */
    }
    client.release()
  }
}
