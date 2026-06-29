import { useCallback, useEffect, useState } from 'react'
import { useAuthStore } from '../store/auth'
import { useTheme } from '../theme'
import SyncAudit from '../components/SyncAudit'

const API = import.meta.env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')
const DAYS = ['', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']
// Weekday columns for the timetable (Mon→Sun match day_of_week 1..7).
const WEEK = [1, 2, 3, 4, 5, 6, 7]

interface Unit {
  unit_id: string; name: string; year: number; semester: number
  day_of_week: number; session_start: string; session_duration_minutes: number; lecturers: string; room?: string
}
interface Slot {
  unit_id: string; unit_name: string; day_of_week: number; start_time: string
  duration_minutes: number; room: string; lecturer_name: string
}
interface Offering {
  course_name: string; level: string; intake: string; study_year: number; semester: number; session_type: string
}
interface Student {
  student_id: string; full_name: string; email: string
  current_year: number; semester: number; intake_session: string; enrollment_status: string
}
interface ActiveSession {
  session_id: string; unit_name: string; status: string
  gate_open_time?: string; checkin_window_end?: string; present_count: number
}
interface RosterRow { student_id: string; full_name: string; status: string; checkin_time?: string }
interface LastRoster {
  session: { unit_name: string; session_date: string; closed_at: string; present: number; total: number } | null
  roster: RosterRow[]
}

export default function Dashboard({ onTakeAttendance }: { onTakeAttendance?: () => void }) {
  const { token, logout } = useAuthStore(s => ({ token: s.token, logout: s.logout }))
  const { theme, toggle } = useTheme()
  const [overview, setOverview] = useState<{ offering: Offering | null; units: Unit[]; slots?: Slot[] } | null>(null)
  const [students, setStudents] = useState<Student[]>([])
  const [last, setLast] = useState<LastRoster | null>(null)
  const [brand, setBrand] = useState<{ name: string; logo_url: string; motto: string } | null>(null)
  const [tab, setTab] = useState<'timetable' | 'units' | 'students' | 'roster'>('timetable')
  const [pwOpen, setPwOpen] = useState(false)
  const [active, setActive] = useState<ActiveSession[]>([])
  const [closingId, setClosingId] = useState<string | null>(null)

  // Live sessions are polled (and refreshed after a close) so the coordinator
  // always sees what's currently open — the same sessions that block starting a
  // new one — and can close one right here.
  const loadActive = useCallback(() => {
    if (!token) return
    fetch(`${API}/api/v1/coordinator/active-sessions`, { headers: { Authorization: `Bearer ${token}` } })
      .then(r => (r.ok ? r.json() : null))
      .then(d => setActive(d?.sessions ?? []))
      .catch(() => {})
  }, [token])

  useEffect(() => {
    if (!token) return
    const h = { Authorization: `Bearer ${token}` }
    const j = (r: Response) => (r.ok ? r.json() : null)
    fetch(`${API}/api/v1/coordinator/overview`, { headers: h }).then(j).then(setOverview).catch(() => {})
    fetch(`${API}/api/v1/coordinator/students`, { headers: h }).then(j).then((s) => setStudents(s ?? [])).catch(() => {})
    fetch(`${API}/api/v1/coordinator/last-roster`, { headers: h }).then(j).then(setLast).catch(() => {})
    fetch(`${API}/api/v1/branding`, { headers: h }).then(j).then(setBrand).catch(() => {})
  }, [token])

  useEffect(() => {
    loadActive()
    const t = setInterval(loadActive, 15000)
    return () => clearInterval(t)
  }, [loadActive])

  async function closeSession(id: string) {
    if (!token) return
    if (!confirm('Close this live session now? Students will no longer be able to check in.')) return
    setClosingId(id)
    try {
      await fetch(`${API}/api/v1/sessions/${id}/close`, { method: 'POST', headers: { Authorization: `Bearer ${token}` } })
      loadActive()
      // The closed session becomes the "last roster".
      fetch(`${API}/api/v1/coordinator/last-roster`, { headers: { Authorization: `Bearer ${token}` } })
        .then(r => (r.ok ? r.json() : null)).then(setLast).catch(() => {})
    } finally {
      setClosingId(null)
    }
  }

  const off = overview?.offering
  return (
    <div style={{ maxWidth: 760, margin: '0 auto', padding: 20, fontFamily: 'system-ui', color: 'var(--text, #0f172a)' }}>
      <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 18 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          {brand?.logo_url
            ? <img src={brand.logo_url} alt="" style={{ height: 68, width: 68, objectFit: 'contain', borderRadius: 6 }} />
            : <div style={{ height: 68, width: 68, borderRadius: 6, background: 'var(--brand)', color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', fontWeight: 700 }}>{(brand?.name ?? 'Q').slice(0, 1)}</div>}
          <div>
            <div style={{ fontWeight: 700 }}>{brand?.name ?? 'QAAT'}</div>
            {brand?.motto && <div style={{ fontSize: 11, color: 'var(--muted)' }}>{brand.motto}</div>}
          </div>
        </div>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          {onTakeAttendance && <button onClick={onTakeAttendance} style={btnPrimary}>Take attendance</button>}
          <button onClick={toggle} title={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`} aria-label="Toggle light or dark theme"
            style={{ background: 'none', border: '1px solid var(--border,#e2e8f0)', color: 'inherit', borderRadius: 6, padding: '7px 11px', cursor: 'pointer', fontSize: 15, lineHeight: 1 }}>
            {theme === 'dark' ? '☀' : '☾'}
          </button>
          <button onClick={() => setPwOpen(true)} style={{ background: 'none', border: '1px solid var(--border,#e2e8f0)', color: 'inherit', borderRadius: 6, padding: '7px 12px', cursor: 'pointer', fontSize: 13 }}>Change password</button>
          <button onClick={logout} style={{ background: 'none', border: 'none', color: 'var(--brand)', cursor: 'pointer' }}>Sign out</button>
        </div>
      </header>
      {pwOpen && <ChangePasswordModal token={token} onClose={() => setPwOpen(false)} />}

      {off ? (
        <div style={{ background: 'var(--surface, #f8fafc)', border: '1px solid var(--border, #e2e8f0)', borderRadius: 12, padding: '14px 18px', marginBottom: 16 }}>
          <div style={{ fontSize: 18, fontWeight: 800 }}>{off.course_name}</div>
          <div style={{ color: 'var(--muted)', fontSize: 13 }}>{[off.session_type, `Year ${off.study_year}`, `Sem ${off.semester}`, off.level, off.intake].filter(Boolean).join(' · ')}</div>
        </div>
      ) : (
        <div style={{ background: '#fffbeb', border: '1px solid #fde68a', borderRadius: 12, padding: 16, marginBottom: 16, color: '#92400e' }}>
          You are not yet assigned to a session. Ask your administrator to add you as a coordinator of a session.
        </div>
      )}

      {active.length > 0 && (
        <div style={{ background: '#ecfdf5', border: '1px solid #6ee7b7', borderRadius: 12, padding: '12px 16px', marginBottom: 16 }}>
          <div style={{ fontWeight: 800, color: '#065f46', marginBottom: 6, display: 'flex', alignItems: 'center', gap: 8 }}>
            <span style={{ width: 9, height: 9, borderRadius: 999, background: '#10b981', display: 'inline-block', boxShadow: '0 0 0 3px rgba(16,185,129,.25)' }} />
            Live session{active.length > 1 ? 's' : ''} in progress
          </div>
          {active.map(s => (
            <div key={s.session_id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12, padding: '8px 0', borderTop: '1px solid #d1fae5' }}>
              <div>
                <div style={{ fontWeight: 700 }}>{s.unit_name}</div>
                <div style={{ fontSize: 12, color: '#047857' }}>
                  {s.status === 'PENDING_LECTURER' ? 'Awaiting lecturer start' : 'Active'} · {s.present_count} checked in
                  {s.gate_open_time ? ` · opened ${new Date(s.gate_open_time).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}` : ''}
                </div>
              </div>
              <button onClick={() => closeSession(s.session_id)} disabled={closingId === s.session_id}
                style={{ padding: '8px 14px', background: '#dc2626', color: '#fff', border: 'none', borderRadius: 8, cursor: 'pointer', fontWeight: 700, fontSize: 13, whiteSpace: 'nowrap', opacity: closingId === s.session_id ? 0.6 : 1 }}>
                {closingId === s.session_id ? 'Closing…' : 'Close session'}
              </button>
            </div>
          ))}
        </div>
      )}

      <StandbyPanel token={token} />

      <div style={{ display: 'flex', gap: 6, marginBottom: 14, flexWrap: 'wrap' }}>
        {(['timetable', 'units', 'students', 'roster'] as const).map(t => (
          <button key={t} onClick={() => setTab(t)} style={{ ...tabBtn, ...(tab === t ? tabActive : {}) }}>
            {t === 'timetable' ? 'Timetable' : t === 'units' ? `Units (${overview?.units.length ?? 0})` : t === 'students' ? `Students (${students.length})` : 'Last roster'}
          </button>
        ))}
      </div>

      {tab === 'timetable' && <Timetable units={
        (overview?.slots?.length
          ? overview.slots.map(s => ({ unit_id: s.unit_id, name: s.unit_name, year: 0, semester: 0, day_of_week: s.day_of_week, session_start: s.start_time, session_duration_minutes: s.duration_minutes, lecturers: s.lecturer_name, room: s.room }))
          : overview?.units) ?? []
      } />}

      {tab === 'units' && (
        <Table head={['Code', 'Unit', 'Yr/Sem', 'Day & time', 'Lecturer(s)']}
          rows={(overview?.units ?? []).map(u => [
            u.unit_id,
            u.name,
            `Y${u.year}/S${u.semester}`,
            u.session_start ? `${u.day_of_week ? DAYS[u.day_of_week] + ' ' : ''}${u.session_start}${u.session_duration_minutes ? ` (${u.session_duration_minutes}m)` : ''}` : '—',
            u.lecturers || '—',
          ])} empty="No units in this program yet." />
      )}

      {tab === 'students' && (
        <Table head={['Reg No.', 'Name', 'Year/Sem', 'Intake', 'Status']}
          rows={students.map(s => [s.student_id, s.full_name, `Y${s.current_year}/S${s.semester}`, s.intake_session || '—', s.enrollment_status])}
          empty="No students enrolled in this session yet." />
      )}

      {tab === 'roster' && (
        last?.session ? (
          <div>
            <div style={{ fontSize: 13, color: 'var(--muted)', marginBottom: 8 }}>
              {last.session.unit_name} · {last.session.session_date} · <strong>{last.session.present}/{last.session.total}</strong> present
            </div>
            <Table head={['Reg No.', 'Name', 'Status', 'Checked in']}
              rows={last.roster.map(r => [r.student_id, r.full_name, r.status, r.checkin_time ? new Date(r.checkin_time).toLocaleTimeString() : '—'])}
              empty="No students on the roster." />
          </div>
        ) : <Empty msg="No closed/synced session yet. Close a session to see its roster here." />
      )}

      <SyncAudit />
    </div>
  )
}

// Weekly timetable — groups the coordinator's units by weekday so they can see,
// at a glance, which sessions run on which day and at what time. Today's column
// is highlighted so "is there a session today?" is unambiguous.
function Timetable({ units }: { units: Unit[] }) {
  // JS getDay(): 0=Sun..6=Sat → our day_of_week 1=Mon..7=Sun.
  const jsDay = new Date().getDay()
  const todayDow = jsDay === 0 ? 7 : jsDay

  const scheduled = units.filter(u => u.day_of_week >= 1 && u.day_of_week <= 7 && u.session_start)
  const unscheduled = units.filter(u => !(u.day_of_week >= 1 && u.day_of_week <= 7 && u.session_start))
  const byDay = (d: number) =>
    scheduled.filter(u => u.day_of_week === d).sort((a, b) => a.session_start.localeCompare(b.session_start))

  const endTime = (start: string, mins: number) => {
    if (!start || !mins) return ''
    const [h, m] = start.split(':').map(Number)
    const t = h * 60 + m + mins
    return `${String(Math.floor(t / 60) % 24).padStart(2, '0')}:${String(t % 60).padStart(2, '0')}`
  }

  if (scheduled.length === 0 && unscheduled.length === 0) {
    return <Empty msg="No units in this session yet. Ask your admin to add units, then set each unit's day & time when you open it." />
  }

  return (
    <div>
      <div style={{ fontSize: 12, color: 'var(--muted)', marginBottom: 10 }}>
        Your weekly schedule. Set or change a unit's day &amp; time from “Take attendance” when you open its session.
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(7, minmax(96px, 1fr))', gap: 8, overflowX: 'auto' }}>
        {WEEK.map(d => {
          const isToday = d === todayDow
          const items = byDay(d)
          return (
            <div key={d} style={{
              border: `1px solid ${isToday ? 'var(--brand)' : 'var(--border,#e2e8f0)'}`,
              borderRadius: 10, padding: 8, background: isToday ? 'color-mix(in srgb, var(--brand) 8%, transparent)' : 'var(--surface,#fff)', minWidth: 96,
            }}>
              <div style={{ fontWeight: 700, fontSize: 12, marginBottom: 6, color: isToday ? 'var(--brand)' : 'inherit' }}>
                {DAYS[d]}{isToday ? ' · today' : ''}
              </div>
              {items.length === 0
                ? <div style={{ fontSize: 11, color: 'var(--muted)' }}>—</div>
                : items.map(u => (
                    <div key={u.unit_id} style={{ background: 'var(--bg,#f1f5f9)', borderRadius: 8, padding: '6px 8px', marginBottom: 6 }}>
                      <div style={{ fontWeight: 600, fontSize: 12 }}>{u.name}</div>
                      <div style={{ fontSize: 11, color: 'var(--muted)' }}>
                        {u.session_start}{u.session_duration_minutes ? `–${endTime(u.session_start, u.session_duration_minutes)}` : ''}{u.room ? ` · ${u.room}` : ''}
                      </div>
                      {u.lecturers && <div style={{ fontSize: 10, color: 'var(--muted)' }}>{u.lecturers}</div>}
                    </div>
                  ))}
            </div>
          )
        })}
      </div>
      {unscheduled.length > 0 && (
        <div style={{ marginTop: 14 }}>
          <div style={{ fontSize: 12, fontWeight: 700, color: '#92400e', marginBottom: 6 }}>Not yet scheduled</div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
            {unscheduled.map(u => (
              <span key={u.unit_id} style={{ background: '#fffbeb', border: '1px solid #fde68a', color: '#92400e', borderRadius: 999, padding: '3px 10px', fontSize: 12 }}>
                {u.name}
              </span>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

function Table({ head, rows, empty }: { head: string[]; rows: (string | number)[][]; empty: string }) {
  if (rows.length === 0) return <Empty msg={empty} />
  return (
    <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
      <thead><tr style={{ background: 'var(--surface, #f8fafc)' }}>
        {head.map(h => <th key={h} style={{ textAlign: 'left', padding: '8px 10px', borderBottom: '1px solid var(--border,#e2e8f0)', whiteSpace: 'nowrap' }}>{h}</th>)}
      </tr></thead>
      <tbody>
        {rows.map((r, i) => (
          <tr key={i} style={{ borderBottom: '1px solid var(--border,#f1f5f9)' }}>
            {r.map((c, j) => <td key={j} style={{ padding: '8px 10px', color: j === 0 ? 'var(--muted)' : 'inherit', fontFamily: j === 0 ? 'monospace' : 'inherit', fontWeight: j === 1 ? 600 : 400 }}>{c}</td>)}
          </tr>
        ))}
      </tbody>
    </table>
  )
}
function Empty({ msg }: { msg: string }) {
  return <p style={{ color: 'var(--muted)', textAlign: 'center', padding: 24 }}>{msg}</p>
}

// Emergency standby — the coordinator authorises a student of their OWN cohort to
// run a session in their absence. Issues a code the coordinator reads out; the
// student signs in with it on the login screen.
interface Standby { delegation_id: string; code: string; deputy_reg: string; deputy_name: string; expires_at: string }
function StandbyPanel({ token }: { token: string | null }) {
  const [open, setOpen] = useState(false)
  const [reg, setReg] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [list, setList] = useState<Standby[]>([])

  const load = useCallback(() => {
    if (!token) return
    fetch(`${API}/api/v1/coordinator/standby`, { headers: { Authorization: `Bearer ${token}` } })
      .then(r => (r.ok ? r.json() : [])).then((d: Standby[]) => setList(Array.isArray(d) ? d : [])).catch(() => {})
  }, [token])
  useEffect(() => { if (open) load() }, [open, load])

  async function issue() {
    if (!reg.trim()) return
    setBusy(true); setErr(null)
    try {
      const res = await fetch(`${API}/api/v1/coordinator/standby`, {
        method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ deputy_reg: reg.trim() }),
      })
      const d = await res.json()
      if (!res.ok) throw new Error(d.message || 'Could not create standby')
      setReg(''); load()
    } catch (e) { setErr(e instanceof Error ? e.message : 'Failed') } finally { setBusy(false) }
  }
  async function revoke(id: string) {
    if (!confirm('Revoke this standby? The student will no longer be able to run your session.')) return
    await fetch(`${API}/api/v1/coordinator/standby/${id}/revoke`, { method: 'POST', headers: { Authorization: `Bearer ${token}` } }).catch(() => {})
    load()
  }

  return (
    <div style={{ border: '1px solid var(--border,#e2e8f0)', borderRadius: 12, padding: '10px 14px', marginBottom: 16, background: 'var(--surface,#fff)' }}>
      <button onClick={() => setOpen(o => !o)} style={{ width: '100%', background: 'none', border: 'none', cursor: 'pointer', display: 'flex', justifyContent: 'space-between', alignItems: 'center', color: 'inherit', fontWeight: 700, fontSize: 13, padding: 0 }}>
        <span>🆘 Emergency standby {list.length > 0 && <span style={{ color: '#b45309' }}>· {list.length} active</span>}</span>
        <span style={{ color: 'var(--muted)' }}>{open ? '▾' : '▸'}</span>
      </button>
      {open && (
        <div style={{ marginTop: 10 }}>
          <p style={{ fontSize: 12, color: 'var(--muted,#64748b)', margin: '0 0 10px' }}>
            Going to be absent? Authorise a student <strong>from your own cohort</strong> to run the session today. Read them the code — they sign in with “standby code” on the login screen. Valid until end of day.
          </p>
          {err && <div style={{ background: '#fef2f2', color: '#b91c1c', padding: '6px 10px', borderRadius: 6, marginBottom: 8, fontSize: 12 }}>{err}</div>}
          <div style={{ display: 'flex', gap: 8 }}>
            <input value={reg} onChange={e => setReg(e.target.value)} placeholder="Standby student's registration number"
              style={{ flex: 1, padding: '8px 10px', borderRadius: 8, border: '1px solid var(--border,#e2e8f0)', fontSize: 13, background: 'var(--surface,#fff)', color: 'inherit' }} />
            <button onClick={issue} disabled={busy || !reg.trim()} style={{ padding: '8px 14px', background: '#b45309', color: '#fff', border: 'none', borderRadius: 8, cursor: 'pointer', fontWeight: 700, fontSize: 13, whiteSpace: 'nowrap', opacity: busy || !reg.trim() ? 0.6 : 1 }}>{busy ? 'Issuing…' : 'Issue code'}</button>
          </div>
          {list.length > 0 && (
            <div style={{ marginTop: 12, display: 'flex', flexDirection: 'column', gap: 6 }}>
              {list.map(s => (
                <div key={s.delegation_id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 10, background: 'var(--bg,#f8fafc)', borderRadius: 8, padding: '8px 10px' }}>
                  <div style={{ minWidth: 0 }}>
                    <span style={{ fontFamily: 'monospace', fontWeight: 800, fontSize: 16, letterSpacing: 1 }}>{s.code}</span>
                    <div style={{ fontSize: 11, color: 'var(--muted,#64748b)' }}>{s.deputy_name || s.deputy_reg} · until {new Date(s.expires_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</div>
                  </div>
                  <button onClick={() => revoke(s.delegation_id)} style={{ padding: '5px 10px', background: 'none', border: '1px solid #fecaca', color: '#b91c1c', borderRadius: 6, cursor: 'pointer', fontSize: 12 }}>Revoke</button>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function ChangePasswordModal({ token, onClose }: { token: string | null; onClose: () => void }) {
  const [cur, setCur] = useState(''); const [next, setNext] = useState(''); const [confirm, setConfirm] = useState('')
  const [busy, setBusy] = useState(false); const [err, setErr] = useState<string | null>(null); const [done, setDone] = useState(false)
  async function submit() {
    if (next.length < 8) { setErr('New password must be at least 8 characters.'); return }
    if (next !== confirm) { setErr('New passwords do not match.'); return }
    setBusy(true); setErr(null)
    try {
      const res = await fetch(`${API}/api/v1/auth/change-password`, {
        method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ current_password: cur, new_password: next }),
      })
      if (!res.ok) { const d = await res.json().catch(() => ({})); throw new Error(d.message || 'Failed') }
      setDone(true)
    } catch (e) { setErr(e instanceof Error ? e.message : 'Failed') } finally { setBusy(false) }
  }
  const inp: React.CSSProperties = { width: '100%', padding: 10, borderRadius: 6, border: '1px solid var(--border,#e2e8f0)', marginBottom: 10, boxSizing: 'border-box', background: 'var(--surface,#fff)', color: 'var(--text,#0f172a)' }
  const b: React.CSSProperties = { padding: '10px 16px', background: 'var(--brand)', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 700 }
  return (
    <div onClick={onClose} style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 9999, padding: 20 }}>
      <div onClick={e => e.stopPropagation()} style={{ background: 'var(--surface,#fff)', color: 'var(--text,#0f172a)', borderRadius: 12, padding: 24, width: 340 }}>
        <h3 style={{ marginTop: 0 }}>Change password</h3>
        {done ? <><p style={{ color: '#16a34a' }}>✓ Password changed.</p><button onClick={onClose} style={b}>Close</button></> : <>
          {err && <div style={{ background: '#fef2f2', color: '#b91c1c', padding: '8px 12px', borderRadius: 6, marginBottom: 10, fontSize: 13 }}>{err}</div>}
          <input type="password" placeholder="Current password" value={cur} onChange={e => setCur(e.target.value)} style={inp} />
          <input type="password" placeholder="New password (min 8)" value={next} onChange={e => setNext(e.target.value)} style={inp} />
          <input type="password" placeholder="Confirm new password" value={confirm} onChange={e => setConfirm(e.target.value)} style={inp} />
          <div style={{ display: 'flex', gap: 8 }}>
            <button onClick={submit} disabled={busy || !cur || !next} style={b}>{busy ? 'Saving…' : 'Update'}</button>
            <button onClick={onClose} style={{ ...b, background: 'transparent', color: 'inherit', border: '1px solid var(--border,#e2e8f0)' }}>Cancel</button>
          </div>
        </>}
      </div>
    </div>
  )
}

const btnPrimary: React.CSSProperties = { padding: '8px 16px', background: 'var(--brand)', color: '#fff', border: 'none', borderRadius: 8, cursor: 'pointer', fontWeight: 700, fontSize: 13 }
const tabBtn: React.CSSProperties = { padding: '7px 14px', border: '1px solid var(--border,#e2e8f0)', background: 'var(--surface,#fff)', borderRadius: 8, cursor: 'pointer', fontSize: 13, fontWeight: 600, color: 'inherit' }
const tabActive: React.CSSProperties = { background: 'var(--brand)', color: '#fff', borderColor: 'var(--brand)' }
