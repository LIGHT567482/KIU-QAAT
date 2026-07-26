// Minimal session store for the super-admin console (sessionStorage-backed).
// Mirrors the admin-dashboards approach: token kept in sessionStorage, cleared on
// expiry. Only the SUPER_ADMIN role is allowed past login.

export interface Session {
  token: string
  userId: string
  tenantId: string
  role: string
  expiresAt: number // unix seconds
}

// Well-known sentinel "platform" tenant that owns the SUPER_ADMIN user (see
// db/migrations/015 + db/seeds/004). Used as a fallback if tenant-lookup fails.
export const PLATFORM_TENANT_ID = '00000000-0000-0000-0000-000000000000'

const KEY = 'qaat_superadmin_token'

export function getSession(): Session | null {
  try {
    const raw = sessionStorage.getItem(KEY)
    if (!raw) return null
    const s: Session = JSON.parse(raw)
    if (Math.floor(Date.now() / 1000) >= s.expiresAt) return null
    return s
  } catch {
    return null
  }
}

export function setSession(s: Session): void {
  sessionStorage.setItem(KEY, JSON.stringify(s))
}

export function clearSession(): void {
  sessionStorage.removeItem(KEY)
}
