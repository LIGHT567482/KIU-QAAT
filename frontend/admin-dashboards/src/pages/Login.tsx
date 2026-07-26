import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth, type Role } from '../contexts/AuthContext'
import { useTheme, ThemeToggle } from '../theme'

const API = import.meta.env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')

const ROLE_REDIRECT: Record<Role, string> = {
  VC:           '/vc',
  DQA_DIRECTOR: '/dqa/thresholds',
  QA_OFFICER:   '/qa/live',
  COORDINATOR:  '/coordinator',
  ADMIN:        '/admin',
  LECTURER:     '/lecturer',
}

export default function Login() {
  const { login } = useAuth()
  const { theme, toggle } = useTheme()
  const navigate = useNavigate()
  const [form, setForm] = useState({ email: '', password: '', totp_code: '', institution_id: '' })
  const [needsMFA, setNeedsMFA] = useState(false)
  const [resolvedTenantId, setResolvedTenantId] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  // Lecturers sign in passwordless: institution + staff ID (they are QR-only and
  // hold no password), the same trust model as the read-only lecturer portal.
  const [lecturerMode, setLecturerMode] = useState(false)
  const [staffId, setStaffId] = useState('')
  const [lecturerOrg, setLecturerOrg] = useState('')

  async function handleLecturerSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null); setLoading(true)
    try {
      const res = await fetch(`${API}/api/v1/auth/lecturer-login`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ staff_id: staffId.trim(), org: lecturerOrg.trim() }),
      })
      const data = await res.json()
      if (!res.ok) { setError(data.message ?? 'Login failed'); setLoading(false); return }
      // The JWT carries sub (user_id), tenant_id, role and exp.
      const p = JSON.parse(atob(data.access_token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')))
      login(data.access_token, { userId: p.sub, tenantId: p.tenant_id, role: p.role as Role, expiresAt: p.exp })
      navigate(ROLE_REDIRECT[p.role as Role] ?? '/lecturer')
    } catch {
      setError('Network error')
    } finally {
      setLoading(false)
    }
  }

  async function resolveTenant(email: string): Promise<string> {
    const res = await fetch(`${API}/api/v1/auth/tenant-lookup?email=${encodeURIComponent(email)}`)
    if (!res.ok) throw new Error('No account found for that email address.')
    const data = await res.json()
    return data.tenant_id as string
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      let tid = resolvedTenantId
      if (!tid) {
        try {
          tid = await resolveTenant(form.email)
          setResolvedTenantId(tid)
        } catch {
          setError('No account found for that email address.')
          setLoading(false)
          return
        }
      }

      const res = await fetch(`${API}/api/v1/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...form, tenant_id: tid }),
      })
      const data = await res.json()

      if (res.status === 403 && data.error === 'MFA_REQUIRED') {
        setNeedsMFA(true)
        setLoading(false)
        return
      }
      if (!res.ok) {
        setError(data.message ?? 'Login failed')
        setLoading(false)
        return
      }

      login(data.access_token, {
        userId:    data.user_id,
        tenantId:  tid,
        role:      data.role as Role,
        expiresAt: Math.floor(Date.now() / 1000) + data.expires_in,
      })
      navigate(ROLE_REDIRECT[data.role as Role] ?? '/')
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
        <h1 style={{ marginBottom: 4 }}>QAAT</h1>
        <p style={{ color: 'var(--muted)', marginBottom: 28 }}>{lecturerMode ? 'Lecturer sign-in' : 'Sign in to your dashboard'}</p>

        {error && (
          <div style={{ background: '#fee2e2', color: '#b91c1c', padding: '10px 14px', borderRadius: 6, marginBottom: 16 }}>
            {error}
          </div>
        )}

        {lecturerMode ? (
          <form onSubmit={handleLecturerSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <input type="text" placeholder="Institution ID" value={lecturerOrg} autoComplete="organization" autoFocus
              onChange={e => setLecturerOrg(e.target.value)} required style={inp} />
            <input type="text" placeholder="Staff ID" value={staffId} autoComplete="username"
              onChange={e => setStaffId(e.target.value)} required style={inp} />
            <div style={{ fontSize: 11, color: 'var(--muted)', margin: '-4px 2px 0' }}>
              No password needed — sign in with your institution and staff ID. (You can also scan your personal lecturer QR.)
            </div>
            <button type="submit" disabled={loading} style={btn}>{loading ? 'Signing in…' : 'Sign In'}</button>
          </form>
        ) : (
          <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <input type="email" placeholder="Email" value={form.email} autoComplete="username"
              onChange={e => { setForm(f => ({ ...f, email: e.target.value })); setResolvedTenantId('') }}
              required style={inp} />
            <input type="password" placeholder="Password" value={form.password} autoComplete="current-password"
              onChange={e => setForm(f => ({ ...f, password: e.target.value }))} required style={inp} />
            <div>
              <input type="text" placeholder="Institution ID" value={form.institution_id} autoComplete="off"
                onChange={e => setForm(f => ({ ...f, institution_id: e.target.value }))} style={inp} />
              <div style={{ fontSize: 11, color: 'var(--muted)', margin: '4px 2px 0' }}>
                Required for institution administrators (provided by your platform owner).
              </div>
            </div>
            {needsMFA && (
              <input type="text" inputMode="numeric" pattern="\d{6}" placeholder="Authenticator code"
                value={form.totp_code} onChange={e => setForm(f => ({ ...f, totp_code: e.target.value }))}
                required autoFocus style={inp} />
            )}
            <button type="submit" disabled={loading} style={btn}>
              {loading ? 'Signing in…' : needsMFA ? 'Verify' : 'Sign In'}
            </button>
          </form>
        )}

        <button onClick={() => { setLecturerMode(m => !m); setError(null) }}
          style={{ marginTop: 16, background: 'none', border: 'none', color: 'var(--brand)', cursor: 'pointer', fontSize: 13, padding: 0, textDecoration: 'underline' }}>
          {lecturerMode ? '← Back to staff/admin sign-in' : 'Lecturer? Sign in with your Staff ID'}
        </button>
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
