import { useState, type FormEvent } from 'react'
import { useAuthStore } from '../store/auth'
import { useTheme, ThemeToggle } from '../theme'

const API = import.meta.env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')

export default function Login() {
  const login = useAuthStore(s => s.login)
  const { theme, toggle } = useTheme()
  const [form, setForm] = useState({ email: '', password: '', totp_code: '' })
  const [needsMFA, setNeedsMFA] = useState(false)
  const [resolvedTenantId, setResolvedTenantId] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  // Emergency standby: a student runs the session with a code from the coordinator.
  const [standbyMode, setStandbyMode] = useState(false)
  const [standby, setStandby] = useState({ code: '', reg: '' })

  async function handleStandby(e: FormEvent) {
    e.preventDefault()
    setError(null); setLoading(true)
    try {
      const res = await fetch(`${API}/api/v1/auth/coordinator-standby-login`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code: standby.code.trim(), reg: standby.reg.trim() }),
      })
      const data = await res.json()
      if (!res.ok) { setError(data.message ?? 'Standby sign-in failed'); setLoading(false); return }
      await login({
        access_token: data.access_token, jti: data.jti, role: data.role,
        user_id: data.user_id, tenant_id: data.tenant_id, expires_in: data.expires_in,
      })
    } catch {
      setError('Network error — are you offline?')
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

      await login({
        access_token:       data.access_token,
        jti:                data.jti,
        role:               data.role,
        user_id:            data.user_id,
        tenant_id:          tid,
        expires_in:         data.expires_in,
        device_binding_key: data.device_binding_key,
      })
    } catch {
      setError('Network error — are you offline?')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{ maxWidth: 400, margin: '80px auto', padding: 24, fontFamily: 'system-ui', color: 'var(--text)' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
        <h1 style={{ margin: 0 }}>QAAT Coordinator</h1>
        <ThemeToggle theme={theme} toggle={toggle} />
      </div>
      <p style={{ color: 'var(--muted)', marginBottom: 24 }}>{standbyMode ? 'Standby coordinator sign-in' : 'Sign in to start a session'}</p>

      {error && (
        <div style={{ background: '#fee2e2', color: '#b91c1c', padding: '10px 14px', borderRadius: 6, marginBottom: 16 }}>
          {error}
        </div>
      )}

      {standbyMode ? (
        <form onSubmit={handleStandby} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <p style={{ color: 'var(--muted)', fontSize: 13, margin: 0 }}>
            Standing in for an absent coordinator? Enter the standby code they gave you and your own registration number.
          </p>
          <input type="text" placeholder="Standby code" value={standby.code} autoFocus autoCapitalize="characters"
            onChange={e => setStandby(s => ({ ...s, code: e.target.value.toUpperCase() }))} required style={inputStyle} />
          <input type="text" placeholder="Your registration number" value={standby.reg}
            onChange={e => setStandby(s => ({ ...s, reg: e.target.value }))} required style={inputStyle} />
          <button type="submit" disabled={loading} style={buttonStyle}>{loading ? 'Signing in…' : 'Sign in as standby'}</button>
        </form>
      ) : (
        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <input
            type="email"
            placeholder="Email"
            value={form.email}
            onChange={e => { setForm(f => ({ ...f, email: e.target.value })); setResolvedTenantId('') }}
            required
            autoComplete="username"
            style={inputStyle}
          />
          <input
            type="password"
            placeholder="Password"
            value={form.password}
            onChange={e => setForm(f => ({ ...f, password: e.target.value }))}
            required
            autoComplete="current-password"
            style={inputStyle}
          />
          {needsMFA && (
            <input
              type="text"
              inputMode="numeric"
              pattern="\d{6}"
              placeholder="6-digit authenticator code"
              value={form.totp_code}
              onChange={e => setForm(f => ({ ...f, totp_code: e.target.value }))}
              required
              autoFocus
              style={inputStyle}
            />
          )}
          <button type="submit" disabled={loading} style={buttonStyle}>
            {loading ? 'Signing in…' : needsMFA ? 'Verify & Sign In' : 'Sign In'}
          </button>
        </form>
      )}

      <button onClick={() => { setStandbyMode(m => !m); setError(null) }}
        style={{ marginTop: 16, background: 'none', border: 'none', color: 'var(--brand)', cursor: 'pointer', fontSize: 13, padding: 0, textDecoration: 'underline' }}>
        {standbyMode ? '← Back to coordinator sign-in' : 'Standing in for an absent coordinator? Use a standby code'}
      </button>
    </div>
  )
}

const inputStyle: React.CSSProperties = {
  padding: '10px 12px', fontSize: 16, borderRadius: 6,
  border: '1px solid var(--border)', outline: 'none', width: '100%', boxSizing: 'border-box',
}

const buttonStyle: React.CSSProperties = {
  padding: '12px', fontSize: 16, fontWeight: 600, borderRadius: 6,
  background: 'var(--brand)', color: 'var(--brand-contrast)', border: 'none', cursor: 'pointer',
}
