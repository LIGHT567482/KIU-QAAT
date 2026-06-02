import { useState, type FormEvent } from 'react'
import { useAuthStore } from '../store/auth'

const API = import.meta.env.VITE_API_URL ?? 'http://localhost:8443'

export default function Login() {
  const login = useAuthStore(s => s.login)
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

      await login({
        access_token:       data.access_token,
        jti:                data.jti,
        role:               data.role,
        user_id:            data.user_id,
        tenant_id:          form.tenant_id,
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
    <div style={{ maxWidth: 400, margin: '80px auto', padding: 24, fontFamily: 'system-ui' }}>
      <h1 style={{ marginBottom: 8 }}>QAAT Coordinator</h1>
      <p style={{ color: '#666', marginBottom: 24 }}>Sign in to start a session</p>

      {error && (
        <div style={{ background: '#fde8e8', color: '#b91c1c', padding: '10px 14px', borderRadius: 6, marginBottom: 16 }}>
          {error}
        </div>
      )}

      <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        <input
          type="text"
          placeholder="Institution ID (tenant)"
          value={form.tenant_id}
          onChange={e => setForm(f => ({ ...f, tenant_id: e.target.value }))}
          required
          style={inputStyle}
        />
        <input
          type="email"
          placeholder="Email"
          value={form.email}
          onChange={e => setForm(f => ({ ...f, email: e.target.value }))}
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
    </div>
  )
}

const inputStyle: React.CSSProperties = {
  padding: '10px 12px', fontSize: 16, borderRadius: 6,
  border: '1px solid #d1d5db', outline: 'none', width: '100%', boxSizing: 'border-box',
}

const buttonStyle: React.CSSProperties = {
  padding: '12px', fontSize: 16, fontWeight: 600, borderRadius: 6,
  background: '#1a73e8', color: '#fff', border: 'none', cursor: 'pointer',
}
