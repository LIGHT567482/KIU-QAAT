import { useState, useEffect, type FormEvent } from 'react'

const API = (import.meta as unknown as { env: { VITE_API_URL?: string } }).env.VITE_API_URL ?? 'http://localhost:8443'

interface Unit {
  unit_id:               string
  unit_name:             string
  sessions_held:         number
  sessions_attended:     number
  attendance_percentage: number
  threshold:             number
  status:                'ELIGIBLE' | 'EXAM_INELIGIBLE'
  deficit_sessions?:     number
}

interface EligibilityData {
  student_id:    string
  academic_year: string
  semester:      number
  units:         Unit[]
}

type AuthState = { token: string; studentId: string } | null

export default function App() {
  const [auth, setAuth] = useState<AuthState>(null)

  if (!auth) return <LoginScreen onLogin={setAuth} />
  return <StatusScreen auth={auth} />
}

// ─── Login ────────────────────────────────────────────────────────────────────

function LoginScreen({ onLogin }: { onLogin: (a: AuthState) => void }) {
  const [form, setForm] = useState({ email: '', password: '', tenant_id: '' })
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setLoading(true); setError(null)
    try {
      const res = await fetch(`${API}/api/v1/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(form),
      })
      const data = await res.json()
      if (!res.ok) { setError(data.message ?? 'Login failed'); return }
      onLogin({ token: data.access_token, studentId: data.user_id })
    } catch { setError('Network error') }
    finally { setLoading(false) }
  }

  return (
    <div style={pageCenter}>
      <div style={card}>
        <h2 style={{ marginBottom: 4 }}>QAAT Student Portal</h2>
        <p style={{ color: '#64748b', marginBottom: 24 }}>Check your attendance and exam eligibility</p>
        {error && <div style={errorBox}>{error}</div>}
        <form onSubmit={submit} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <input placeholder="Institution ID" value={form.tenant_id} required
            onChange={e => setForm(f => ({ ...f, tenant_id: e.target.value }))} style={inp} />
          <input type="email" placeholder="Student email" value={form.email} required autoComplete="username"
            onChange={e => setForm(f => ({ ...f, email: e.target.value }))} style={inp} />
          <input type="password" placeholder="Password" value={form.password} required autoComplete="current-password"
            onChange={e => setForm(f => ({ ...f, password: e.target.value }))} style={inp} />
          <button type="submit" disabled={loading} style={btn}>
            {loading ? 'Signing in…' : 'View My Attendance'}
          </button>
        </form>
      </div>
    </div>
  )
}

// ─── Status Screen ────────────────────────────────────────────────────────────

function StatusScreen({ auth }: { auth: NonNullable<AuthState> }) {
  const [data, setData]     = useState<EligibilityData | null>(null)
  const [error, setError]   = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetch(`${API}/api/v1/eligibility/${auth.studentId}`, {
      headers: { Authorization: `Bearer ${auth.token}` },
    })
      .then(r => r.json())
      .then(setData)
      .catch(() => setError('Could not load your attendance data.'))
      .finally(() => setLoading(false))
  }, [auth])

  if (loading) return <div style={pageCenter}><p style={{ color: '#94a3b8' }}>Loading your attendance…</p></div>
  if (error || !data) return <div style={pageCenter}><p style={{ color: '#b91c1c' }}>{error}</p></div>

  const allEligible = data.units.every(u => u.status === 'ELIGIBLE')

  return (
    <div style={{ maxWidth: 600, margin: '0 auto', padding: '24px 16px', fontFamily: 'system-ui' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 20 }}>
        <div>
          <h2 style={{ margin: 0 }}>My Attendance</h2>
          <p style={{ color: '#64748b', margin: '4px 0 0' }}>{data.academic_year} · Semester {data.semester}</p>
        </div>
        <div style={{
          padding: '8px 16px', borderRadius: 8, fontWeight: 700, fontSize: 14,
          background: allEligible ? '#f0fdf4' : '#fef2f2',
          color:      allEligible ? '#166534' : '#b91c1c',
        }}>
          {allEligible ? 'All units eligible' : 'Action required'}
        </div>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        {data.units.map(u => <UnitCard key={u.unit_id} unit={u} />)}
      </div>

      {data.units.length === 0 && (
        <p style={{ color: '#94a3b8', textAlign: 'center', marginTop: 48 }}>
          No attendance records found yet. Check back after your first session.
        </p>
      )}
    </div>
  )
}

function UnitCard({ unit: u }: { unit: Unit }) {
  const eligible = u.status === 'ELIGIBLE'
  const pct = u.attendance_percentage

  return (
    <div style={{
      background: '#fff', borderRadius: 10, padding: '16px 20px',
      border: eligible ? '1px solid #e2e8f0' : '2px solid #fca5a5',
      boxShadow: '0 1px 4px rgba(0,0,0,.05)',
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 10 }}>
        <div>
          <div style={{ fontWeight: 700, fontSize: 15 }}>{u.unit_name}</div>
          <div style={{ fontSize: 12, color: '#94a3b8' }}>{u.unit_id}</div>
        </div>
        <div style={{ textAlign: 'right' }}>
          <div style={{ fontSize: 24, fontWeight: 800, color: eligible ? '#16a34a' : '#ef4444' }}>
            {pct}%
          </div>
          <div style={{ fontSize: 11, color: '#94a3b8' }}>threshold {u.threshold}%</div>
        </div>
      </div>

      {/* Progress bar */}
      <div style={{ background: '#f1f5f9', borderRadius: 99, height: 8, overflow: 'hidden' }}>
        <div style={{
          width: `${Math.min(pct, 100)}%`, height: '100%', borderRadius: 99,
          background: eligible ? '#22c55e' : '#ef4444',
          transition: 'width 0.4s ease',
        }} />
      </div>

      <div style={{ marginTop: 8, fontSize: 13, color: '#64748b' }}>
        {u.sessions_attended} of {u.sessions_held} sessions attended
        {!eligible && u.deficit_sessions !== undefined && (
          <span style={{ color: '#ef4444', fontWeight: 600, marginLeft: 8 }}>
            · Need {u.deficit_sessions} more
          </span>
        )}
      </div>
    </div>
  )
}

const pageCenter: React.CSSProperties = {
  minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center',
  background: '#f8fafc', fontFamily: 'system-ui',
}
const card:     React.CSSProperties = { background: '#fff', borderRadius: 12, padding: 40, width: 360, boxShadow: '0 4px 24px rgba(0,0,0,.08)' }
const inp:      React.CSSProperties = { padding: '10px 12px', fontSize: 15, borderRadius: 6, border: '1px solid #e2e8f0', width: '100%', boxSizing: 'border-box' }
const btn:      React.CSSProperties = { padding: 12, fontSize: 15, fontWeight: 600, borderRadius: 6, background: '#1e293b', color: '#fff', border: 'none', cursor: 'pointer', marginTop: 4 }
const errorBox: React.CSSProperties = { background: '#fef2f2', color: '#b91c1c', padding: '10px 14px', borderRadius: 6, marginBottom: 16 }
