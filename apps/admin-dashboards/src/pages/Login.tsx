import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth, type Role } from '../contexts/AuthContext'

const API = import.meta.env.VITE_API_URL ?? 'http://localhost:8443'

const ROLE_REDIRECT: Record<Role, string> = {
  VC:           '/vc',
  DQA_DIRECTOR: '/dqa/thresholds',
  QA_OFFICER:   '/qa/live',
  COORDINATOR:  '/coordinator',
  ADMIN:        '/admin/tenants',
}

export default function Login() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const [form, setForm] = useState({ email: '', password: '', tenant_id: '', totp_code: '' })
  const [needsMFA, setNeedsMFA] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      const res = await fetch(`${API}/api/v1/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(form),
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
        tenantId:  form.tenant_id,
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
      justifyContent: 'center', background: '#f1f5f9', fontFamily: 'system-ui',
    }}>
      <div style={{ background: '#fff', borderRadius: 12, padding: 40, width: 380, boxShadow: '0 4px 24px rgba(0,0,0,.08)' }}>
        <h1 style={{ marginBottom: 4 }}>QAAT Admin</h1>
        <p style={{ color: '#64748b', marginBottom: 28 }}>Sign in to your dashboard</p>

        {error && (
          <div style={{ background: '#fef2f2', color: '#b91c1c', padding: '10px 14px', borderRadius: 6, marginBottom: 16 }}>
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <input type="text"  placeholder="Institution ID" value={form.tenant_id}
            onChange={e => setForm(f => ({ ...f, tenant_id: e.target.value }))} required style={inp} />
          <input type="email" placeholder="Email" value={form.email} autoComplete="username"
            onChange={e => setForm(f => ({ ...f, email: e.target.value }))} required style={inp} />
          <input type="password" placeholder="Password" value={form.password} autoComplete="current-password"
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
  border: '1px solid #e2e8f0', width: '100%', boxSizing: 'border-box',
}
const btn: React.CSSProperties = {
  padding: 12, fontSize: 15, fontWeight: 600, borderRadius: 6,
  background: '#1e293b', color: '#fff', border: 'none', cursor: 'pointer', marginTop: 4,
}
