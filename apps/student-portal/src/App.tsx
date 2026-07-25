import { useEffect, useState, useRef, type FormEvent } from 'react'
import { useTheme, ThemeToggle, applyPalette } from './theme'

const API = (import.meta as unknown as { env: { VITE_API_URL?: string } }).env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')

const URL_ORG = typeof location !== 'undefined' ? (new URLSearchParams(location.search).get('org') ?? '').trim() : ''
const URL_QR  = typeof location !== 'undefined' ? (new URLSearchParams(location.search).get('qr') ?? '').trim() : ''

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

interface QRData {
  student_id:    string
  serial_number: string
  token:         string
  url:           string
  image:         string
}

type View = 'login' | 'progress' | 'qr' | 'scan'

export default function App() {
  const { theme, toggle } = useTheme()
  const [reg, setReg]         = useState('')
  const [org, setOrg]         = useState(URL_ORG)
  const [brand, setBrand]     = useState<Brand | null>(null)
  const [data, setData]       = useState<Progress | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError]     = useState<string | null>(null)

  // QR-login state
  const [jwt, setJwt]         = useState<string | null>(null)
  const [qrData, setQrData]   = useState<QRData | null>(null)
  const [qrLoading, setQrLoading] = useState(false)
  const [view, setView]       = useState<View>(URL_QR ? 'scan' : 'login')

  useEffect(() => {
    if (!URL_ORG) return
    fetch(`${API}/api/v1/branding/public?org=${encodeURIComponent(URL_ORG)}`)
      .then(r => (r.ok ? r.json() : null))
      .then((b: Brand | null) => { if (b) { setBrand(b); applyPalette(b) } })
      .catch(() => {})
  }, [])

  // Auto QR login: when the portal is opened via ?qr=TOKEN, exchange it for a JWT.
  useEffect(() => {
    if (!URL_QR) return
    ;(async () => {
      setLoading(true); setError(null)
      try {
        const res = await fetch(`${API}/api/v1/student/qr-login`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ qr: URL_QR }),
        })
        if (!res.ok) {
          const d = await res.json().catch(() => ({}))
          throw new Error(d.message || 'QR login failed')
        }
        const authData = await res.json()
        const token = authData.access_token
        if (!token) throw new Error('No access token returned')
        setJwt(token)
        setView('progress')
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Could not log in with QR code.')
      } finally {
        setLoading(false)
      }
    })()
  }, [])

  async function fetchProgress(token: string) {
    setLoading(true); setError(null)
    try {
      // Resolve student info from the JWT.
      const meRes = await fetch(`${API}/api/v1/student/my-qr`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!meRes.ok) throw new Error('Could not load student profile')
      const me = await meRes.json()
      const pRes = await fetch(`${API}/api/v1/student/progress?reg=${encodeURIComponent(me.student_id)}&org=${encodeURIComponent(org || URL_ORG)}`)
      if (!pRes.ok) {
        const d = await pRes.json().catch(() => ({}))
        throw new Error(d.message || 'No attendance records found.')
      }
      setData(await pRes.json())
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not load data.')
    } finally {
      setLoading(false)
    }
  }

  async function loadMyQR(token: string) {
    setQrLoading(true)
    try {
      const res = await fetch(`${API}/api/v1/student/my-qr`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error('Could not load QR code')
      setQrData(await res.json())
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not load QR code.')
    } finally {
      setQrLoading(false)
    }
  }

  useEffect(() => {
    if (jwt && view === 'progress' && !data) fetchProgress(jwt)
  }, [jwt, view])

  async function submit(e: FormEvent) {
    e.preventDefault()
    const r = reg.trim(), o = org.trim()
    if (!r || !o) return
    setLoading(true); setError(null); setData(null)
    try {
      const res = await fetch(`${API}/api/v1/student/progress?reg=${encodeURIComponent(r)}&org=${encodeURIComponent(o)}`)
      if (!res.ok) {
        const d = await res.json().catch(() => ({}))
        throw new Error(d.message || 'No record found for that registration number.')
      }
      setData(await res.json())
      setView('progress')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not load your attendance.')
    } finally {
      setLoading(false)
    }
  }

  async function handleQRLogin(jwtToken: string) {
    setJwt(jwtToken)
    setView('progress')
    fetchProgress(jwtToken)
  }

  const title = brand?.name || data?.institution || 'QAAT Student Portal'

  return (
    <div style={{ minHeight: '100vh', display: 'flex', flexDirection: 'column', background: 'var(--app-bg)' }}>
      <header style={{ background: 'var(--sidebar, var(--brand))', color: '#fff', padding: '12px 16px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12 }}>
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
          {jwt && view !== 'qr' && (
            <button onClick={() => { loadMyQR(jwt); setView('qr') }}
              style={navBtn}>My QR</button>
          )}
          {jwt && view === 'progress' && (
            <button onClick={() => { setView('qr'); loadMyQR(jwt) }}
              style={navBtn}>My QR</button>
          )}
          {view !== 'login' && !jwt && (
            <button onClick={() => { setView('login'); setError(null) }}
              style={navBtn}>Back</button>
          )}
          <ThemeToggle theme={theme} toggle={toggle} />
        </div>
      </header>

      <div style={{ flex: 1, maxWidth: 600, margin: '0 auto', width: '100%', padding: '24px 16px', boxSizing: 'border-box', fontFamily: 'system-ui', color: 'var(--text)' }}>
        {view === 'login' && !jwt && (
          <form onSubmit={submit} style={{ marginBottom: 20 }}>
            {!URL_ORG && (
              <input value={org} onChange={e => setOrg(e.target.value)}
                placeholder="Institution (e.g. kiu.ac.ug)" style={{ ...inp, width: '100%', marginBottom: 8 }} />
            )}
            <div style={{ display: 'flex', gap: 8 }}>
              <input value={reg} onChange={e => setReg(e.target.value)} autoFocus
                placeholder="Enter your registration number" style={{ ...inp, flex: 1 }} />
              <button type="submit" disabled={loading || !reg.trim() || !org.trim()} style={{ ...btn, marginTop: 0, whiteSpace: 'nowrap' }}>
                {loading ? 'Checking\u2026' : 'View progress'}
              </button>
            </div>
          </form>
        )}

        {view === 'scan' && URL_QR && (
          <div style={{ textAlign: 'center', padding: '40px 0' }}>
            <div style={{ fontSize: 40, marginBottom: 16 }}>{loading ? '\u231B' : '\u2705'}</div>
            <div style={{ fontWeight: 700, fontSize: 18, marginBottom: 8 }}>
              {loading ? 'Signing in with your QR code\u2026' : 'QR login successful'}
            </div>
            {error && <div style={errorBox}>{error}</div>}
          </div>
        )}

        {error && view !== 'scan' && <div style={errorBox}>{error}</div>}

        {view === 'qr' && qrData && (
          <div style={{ textAlign: 'center', padding: '20px 0' }}>
            <h2 style={{ fontWeight: 700, fontSize: 20, marginBottom: 8 }}>Your QR Code</h2>
            <p style={{ color: 'var(--muted)', fontSize: 13, marginBottom: 20 }}>
              Scan this QR to sign in to attendance sessions. You can download or screenshot it.
            </p>
            <div style={{
              display: 'inline-block', padding: 16, borderRadius: 12,
              background: '#fff', boxShadow: '0 2px 12px rgba(0,0,0,.1)', marginBottom: 16,
            }}>
              <img src={qrData.image} alt="Your QR code"
                style={{ width: 260, height: 260, display: 'block' }} />
            </div>
            <div style={{ fontSize: 12, color: 'var(--muted)', marginBottom: 16 }}>
              Serial: {qrData.serial_number}
            </div>
            <div style={{ display: 'flex', gap: 12, justifyContent: 'center', flexWrap: 'wrap' }}>
              <a href={qrData.image} download={`qr-${qrData.serial_number}.png`}
                style={{ ...btn, textDecoration: 'none', display: 'inline-block' }}>
                Download QR
              </a>
              <button onClick={() => { setQrData(null); setView('progress') }}
                style={{ ...btn, background: 'var(--surface)', color: 'var(--text)', border: '1px solid var(--border)' }}>
                Back to progress
              </button>
            </div>
          </div>
        )}

        {view === 'qr' && qrLoading && !qrData && (
          <div style={{ textAlign: 'center', padding: '40px 0', color: 'var(--muted)' }}>
            Loading your QR code\u2026
          </div>
        )}

        {view === 'login' && !jwt && !data && !error && !loading && (
          <p style={{ color: 'var(--muted)', textAlign: 'center', marginTop: 48 }}>
            {org ? 'Type your registration number above and submit to see your attendance and exam eligibility.'
                 : 'Open your institution\u2019s portal link, then enter your registration number to see your attendance.'}
          </p>
        )}

        {data && view !== 'qr' && <Results data={data} onBack={() => { setData(null); setError(null); setReg('') }}
          onShowQR={jwt ? () => { loadMyQR(jwt); setView('qr') } : undefined} />}
      </div>

      <footer style={{ background: 'var(--footer)', color: 'var(--footer-text)', padding: '10px 16px', fontSize: 11, textAlign: 'center' }}>
        Powered by LIGHT TECHNOLOGIES
      </footer>
    </div>
  )
}

function Results({ data, onBack, onShowQR }: { data: Progress; onBack?: () => void; onShowQR?: () => void }) {
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
        {onShowQR && (
          <button onClick={onShowQR} style={navBtn}>
            My QR Code
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
