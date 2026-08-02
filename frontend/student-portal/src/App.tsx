import { useEffect, useState, useRef, type FormEvent } from 'react'
import { useTheme, ThemeToggle, applyPalette } from './theme'
import brandDefault from './brand.json'
import WelcomeToast from './components/WelcomeToast'

const API = (import.meta as unknown as { env: { VITE_API_URL?: string } }).env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')

interface Brand { name: string; logo_url: string; motto: string; brand_color: string; sidebar_color: string; background_color: string; footer_color: string }

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

interface Progress {
  student_id:    string
  full_name?:    string
  institution?:  string
  academic_year: string
  semester:      number
  units:         Unit[]
}

type View = 'login' | 'progress'

export default function App() {
  const { theme, toggle } = useTheme()
  const [reg, setReg]         = useState('')
  const [brand, setBrand]     = useState<Brand | null>(null)
  const [data, setData]       = useState<Progress | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError]     = useState<string | null>(null)

  const [view, setView]       = useState<View>('login')

  useEffect(() => {
    // Single institution — no org needed; the backend serves brand.json.
    fetch(`${API}/api/v1/branding/public`)
      .then(r => (r.ok ? r.json() : null))
      .then((b: Brand | null) => { if (b) { setBrand(b); applyPalette(b) } })
      .catch(() => {})
  }, [])

  async function submit(e: FormEvent) {
    e.preventDefault()
    const r = reg.trim()
    if (!r) return
    setLoading(true); setError(null); setData(null)
    try {
      const res = await fetch(`${API}/api/v1/student/progress?reg=${encodeURIComponent(r)}`)
      if (!res.ok) {
        const d = await res.json().catch(() => ({}))
        throw new Error(d.message || 'No record found for that registration number.')
      }
      const prog = await res.json()
      setData(prog)
      sessionStorage.setItem('qaat_welcome', prog.full_name || r)
      setView('progress')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not load your attendance.')
    } finally {
      setLoading(false)
    }
  }

  const title = brand?.name || data?.institution || 'QAAT Student Portal'

  return (
    <div style={{ minHeight: '100vh', display: 'flex', flexDirection: 'column', background: 'var(--app-bg)' }}>
      <WelcomeToast key={view} />
      <img src={brandDefault.logo_url} alt="" aria-hidden style={{
        position: 'fixed', width: 440, maxWidth: '80vw', opacity: 0.05,
        left: '50%', top: '55%', transform: 'translate(-50%, -50%)', pointerEvents: 'none', zIndex: 0,
      }} />
      <header style={{ background: 'var(--sidebar, var(--brand))', color: '#fff', padding: '12px 16px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, position: 'relative', zIndex: 1 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, minWidth: 0 }}>
          {brand?.logo_url
            ? <img src={brand.logo_url} alt="" style={{ height: 44, width: 44, objectFit: 'contain', background: '#fff', borderRadius: 6, padding: 2, flexShrink: 0 }} />
            : <div style={{ height: 44, width: 44, borderRadius: 6, background: 'rgba(255,255,255,.2)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontWeight: 800, flexShrink: 0 }}>{title.slice(0, 1)}</div>}
          <div style={{ minWidth: 0 }}>
            <div style={{ fontWeight: 800, fontSize: 16, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{title}</div>
            {brand?.motto && <div style={{ fontSize: 11, opacity: .85 }}>{brand.motto}</div>}
          </div>
        </div>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          {view !== 'login' && (
            <button onClick={() => { setView('login'); setError(null) }}
              style={navBtn}>Back</button>
          )}
          <ThemeToggle theme={theme} toggle={toggle} />
        </div>
      </header>

      <div style={{ flex: 1, maxWidth: 600, margin: '0 auto', width: '100%', padding: '24px 16px', boxSizing: 'border-box', fontFamily: 'system-ui', color: 'var(--text)' }}>
        {view === 'login' && (
          <form onSubmit={submit} style={{ marginBottom: 20 }}>
            <div style={{ display: 'flex', gap: 8 }}>
              <input value={reg} onChange={e => setReg(e.target.value)} autoFocus
                placeholder="Enter your registration number" style={{ ...inp, flex: 1 }} />
              <button type="submit" disabled={loading || !reg.trim()} style={{ ...btn, marginTop: 0, whiteSpace: 'nowrap' }}>
                {loading ? 'Checking\u2026' : 'View progress'}
              </button>
            </div>
          </form>
        )}

        {error && <div style={errorBox}>{error}</div>}

        {view === 'login' && !data && !error && !loading && (
          <p style={{ color: 'var(--muted)', textAlign: 'center', marginTop: 48 }}>
            Type your registration number above and submit to see your attendance and exam eligibility.
          </p>
        )}

        {data && <Results data={data} onBack={() => { setData(null); setError(null); setReg(''); setView('login') }} />}
      </div>

      <footer style={{ background: 'var(--footer)', color: 'var(--footer-text)', padding: '10px 16px', fontSize: 11, textAlign: 'center' }}>
        Powered by LIGHT TECHNOLOGIES
      </footer>
    </div>
  )
}

function Results({ data, onBack }: { data: Progress; onBack?: () => void }) {
  const allEligible = data.units.length > 0 && data.units.every(u => u.status === 'ELIGIBLE')
  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
        {onBack && (
          <button onClick={onBack} style={{
            background: 'none', border: 'none', color: 'var(--brand, #2563eb)', cursor: 'pointer',
            fontSize: 13, fontWeight: 600, padding: 0, display: 'flex', alignItems: 'center', gap: 4,
          }}>
            \u2190 Check another number
          </button>
        )}
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 16, gap: 12 }}>
        <div>
          <div style={{ fontWeight: 800, fontSize: 18 }}>{data.full_name || data.student_id}</div>
          <div style={{ color: 'var(--muted)', fontSize: 13 }}>
            {data.student_id}{data.academic_year ? ` \u00b7 ${data.academic_year}` : ''} \u00b7 Semester {data.semester}
          </div>
        </div>
        {data.units.length > 0 && (
          <div style={{ flexShrink: 0, padding: '8px 16px', borderRadius: 8, fontWeight: 700, fontSize: 14, background: allEligible ? '#f0fdf4' : '#fef2f2', color: allEligible ? '#166534' : '#b91c1c' }}>
            {allEligible ? 'All units eligible' : 'Action required'}
          </div>
        )}
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        {data.units.map(u => <UnitCard key={u.unit_id} unit={u} />)}
      </div>

      {data.units.length === 0 && (
        <p style={{ color: 'var(--muted)', textAlign: 'center', marginTop: 48 }}>
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
      background: 'var(--surface)', borderRadius: 10, padding: '16px 20px',
      border: eligible ? '1px solid var(--border)' : '2px solid #fca5a5',
      boxShadow: '0 1px 4px rgba(0,0,0,.05)',
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 10 }}>
        <div>
          <div style={{ fontWeight: 700, fontSize: 15 }}>{u.unit_name}</div>
          <div style={{ fontSize: 12, color: 'var(--muted)' }}>{u.unit_id}</div>
        </div>
        <div style={{ textAlign: 'right' }}>
          <div style={{ fontSize: 24, fontWeight: 800, color: eligible ? '#16a34a' : '#ef4444' }}>{pct}%</div>
          <div style={{ fontSize: 11, color: 'var(--muted)' }}>threshold {u.threshold}%</div>
        </div>
      </div>

      <div style={{ marginBottom: 8 }}>
        <span style={{
          display: 'inline-block', padding: '2px 10px', borderRadius: 999, fontSize: 12, fontWeight: 700,
          background: eligible ? '#f0fdf4' : '#fef2f2',
          color:      eligible ? '#166534' : '#b91c1c',
        }}>
          {eligible ? 'ELIGIBLE' : 'INELIGIBLE'}
        </span>
      </div>

      <div style={{ background: '#f1f5f9', borderRadius: 99, height: 8, overflow: 'hidden' }}>
        <div style={{
          width: `${Math.min(pct, 100)}%`, height: '100%', borderRadius: 99,
          background: eligible ? '#22c55e' : '#ef4444', transition: 'width 0.4s ease',
        }} />
      </div>

      <div style={{ marginTop: 8, fontSize: 13, color: 'var(--muted)' }}>
        {u.sessions_attended} of {u.sessions_held} sessions attended
        {!eligible && u.deficit_sessions !== undefined && (
          <span style={{ color: '#ef4444', fontWeight: 600, marginLeft: 8 }}>
            \u00b7 Need {u.deficit_sessions} more to reach {u.threshold}%
          </span>
        )}
      </div>
    </div>
  )
}

const inp:      React.CSSProperties = { padding: '10px 12px', fontSize: 15, borderRadius: 6, border: '1px solid var(--border)', boxSizing: 'border-box', background: 'var(--surface)', color: 'var(--text)' }
const btn:      React.CSSProperties = { padding: '12px 20px', fontSize: 15, fontWeight: 600, borderRadius: 6, background: 'var(--brand)', color: 'var(--brand-contrast)', border: 'none', cursor: 'pointer' }
const navBtn:   React.CSSProperties = { padding: '6px 14px', fontSize: 13, fontWeight: 600, borderRadius: 6, background: 'rgba(255,255,255,.15)', color: '#fff', border: 'none', cursor: 'pointer' }
const errorBox: React.CSSProperties = { background: '#fef2f2', color: '#b91c1c', padding: '10px 14px', borderRadius: 6, marginBottom: 16 }
