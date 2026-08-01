import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import PasswordInput from '../components/PasswordInput'
import { useAuth, type Role } from '../contexts/AuthContext'
import { useTheme, ThemeToggle } from '../theme'
import brand from '../brand.json'

const API = import.meta.env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')

const ROLE_REDIRECT: Record<Role, string> = {
  VC:           '/vc',
  DQA_DIRECTOR: '/dqa/thresholds',
  QA_OFFICER:   '/qa/live',
  COORDINATOR:  '/coordinator',
  ADMIN:        '/admin',
  LECTURER:     '/lecturer',
  HOD:          '/hod',
  DEAN:         '/dean',
}

export default function Login() {
  const { login } = useAuth()
  const { theme, toggle } = useTheme()
  const navigate = useNavigate()
  const [form, setForm] = useState({ email: '', password: '', totp_code: '' })
  const [needsMFA, setNeedsMFA] = useState(false)
  const [resolvedTenantId, setResolvedTenantId] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

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

      sessionStorage.setItem('qaat_welcome', data.full_name || form.email)
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
      justifyContent: 'center', background: 'var(--bg)', fontFamily: 'system-ui', position: 'relative', overflow: 'hidden',
    }}>
      <img src={brand.logo_url} alt="" aria-hidden style={{
        position: 'absolute', width: 460, maxWidth: '80vw', opacity: 0.05,
        left: '50%', top: '50%', transform: 'translate(-50%, -50%)', pointerEvents: 'none',
      }} />
      <div style={{ position: 'absolute', top: 16, right: 16 }}>
        <ThemeToggle theme={theme} toggle={toggle} />
      </div>
      <div style={{ background: 'var(--surface)', color: 'var(--text)', borderRadius: 12, padding: 40, width: 380, boxShadow: 'var(--shadow)', border: '1px solid var(--border)' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 4 }}>
          {brand.logo_url && <img src={brand.logo_url} alt={brand.name} style={{ height: 48, width: 48, objectFit: 'contain', borderRadius: 8 }} />}
          <h1 style={{ margin: 0, fontSize: 20, lineHeight: 1.15 }}>{brand.name}</h1>
        </div>
        <p style={{ color: 'var(--muted)', marginBottom: 28 }}>Sign in to your dashboard</p>

        {error && (
          <div style={{ background: '#fee2e2', color: '#b91c1c', padding: '10px 14px', borderRadius: 6, marginBottom: 16 }}>
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <input type="email" placeholder="Email" value={form.email} autoComplete="username"
            onChange={e => { setForm(f => ({ ...f, email: e.target.value })); setResolvedTenantId('') }}
            required style={inp} />
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
