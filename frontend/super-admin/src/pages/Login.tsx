import { useState, type FormEvent } from 'react'
import { setSession, PLATFORM_TENANT_ID, type Session } from '../auth'
import PasswordInput from '../components/PasswordInput'
import { ThemeToggle, type Theme } from '../theme'

const API = import.meta.env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')

// Super-admin login. The platform tenant is resolved from the email (tenant-lookup),
// falling back to the well-known sentinel tenant so no tenant typing is needed.
export default function Login({ onLogin, theme, toggle }: { onLogin: (s: Session) => void; theme: Theme; toggle: () => void }) {
  const [form, setForm] = useState({ email: '', password: '', totp_code: '' })
  const [needsMFA, setNeedsMFA] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  async function resolveTenant(email: string): Promise<string> {
    try {
      const res = await fetch(`${API}/api/v1/auth/tenant-lookup?email=${encodeURIComponent(email)}`)
      if (res.ok) return ((await res.json()).tenant_id as string) || PLATFORM_TENANT_ID
    } catch { /* fall through */ }
    return PLATFORM_TENANT_ID
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      const tid = await resolveTenant(form.email)
      const res = await fetch(`${API}/api/v1/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...form, tenant_id: tid }),
      })
      const data = await res.json()

      if (res.status === 403 && data.error === 'MFA_REQUIRED') {
        setNeedsMFA(true); setLoading(false); return
      }
      if (!res.ok) { setError(data.message ?? 'Login failed'); setLoading(false); return }
      if (data.role !== 'SUPER_ADMIN') {
        setError('This console is for the platform owner only.'); setLoading(false); return
      }

      setSession({
        token: data.access_token,
        userId: data.user_id,
        tenantId: data.tenant_id ?? tid,
        role: data.role,
        expiresAt: Math.floor(Date.now() / 1000) + data.expires_in,
      })
      onLogin({
        token: data.access_token, userId: data.user_id, tenantId: data.tenant_id ?? tid,
        role: data.role, expiresAt: Math.floor(Date.now() / 1000) + data.expires_in,
      })
    } catch {
      setError('Network error')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{
      minHeight: '100vh', display: 'flex', alignItems: 'center',
      justifyContent: 'center', background: 'var(--bg)', fontFamily: 'system-ui', position: 'relative',
    }}>
      <div style={{ position: 'absolute', top: 16, right: 16 }}>
        <ThemeToggle theme={theme} toggle={toggle} />
      </div>
      <div style={{ background: 'var(--surface)', color: 'var(--text)', borderRadius: 12, padding: 40, width: 380, boxShadow: 'var(--shadow)', border: '1px solid var(--border)' }}>
        <h1 style={{ marginBottom: 4 }}>QAAT Platform</h1>
        <p style={{ color: 'var(--muted)', marginBottom: 28 }}>Super-admin console</p>

        {error && (
          <div style={{ background: '#fef2f2', color: '#b91c1c', padding: '10px 14px', borderRadius: 6, marginBottom: 16 }}>
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <input type="email" placeholder="Email" value={form.email} autoComplete="username"
            onChange={e => setForm(f => ({ ...f, email: e.target.value }))} required style={inp} />
          <PasswordInput placeholder="Password" value={form.password} autoComplete="current-password"
            onChange={e => setForm(f => ({ ...f, password: e.target.value }))} required style={inp} />
          {needsMFA && (
            <input type="text" inputMode="numeric" pattern="\d{6}" placeholder="Authenticator code"
              value={form.totp_code} onChange={e => setForm(f => ({ ...f, totp_code: e.target.value }))}
              required autoFocus style={inp} />
          )}
          <button type="submit" disabled={loading} style={btn}>
            {loading ? 'Signing in…' : needsMFA ? 'Verify' : 'Sign In'}
          </button>
        </form>
      </div>
    </div>
  )
}

const inp: React.CSSProperties = {
  padding: '10px 12px', fontSize: 15, borderRadius: 6,
  border: '1px solid var(--border)', width: '100%', boxSizing: 'border-box',
}
const btn: React.CSSProperties = {
  padding: 12, fontSize: 15, fontWeight: 600, borderRadius: 6,
  background: 'var(--brand)', color: 'var(--brand-contrast)', border: 'none', cursor: 'pointer', marginTop: 4,
}
