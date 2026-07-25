import { useCallback, useEffect, useState } from 'react'
import { useAuthStore } from '../store/auth'
import { useTheme } from '../theme'
import SyncAudit from '../components/SyncAudit'
import ChronicAbsentee from '../components/ChronicAbsentee'
import Trends from '../components/Trends'
import { processOutboxQueue } from '../sync/outbox'
import { db } from '../db/vault'
import { decrypt } from '../crypto/vault-crypto'

const API = import.meta.env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')
const DAYS = ['', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']
// Colored weekly-grid timetable, matching the admin dashboard's institution-PDF look.
const KIU_GREEN = '#1a7a3f'
const TT_DAYS = ['', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday']
// A weekend cohort runs Sat–Sun; everyone else Mon–Fri (same rule as the admin dashboard).
const daysFor = (sessionType: string): number[] =>
  (sessionType || '').toLowerCase().includes('weekend') ? [6, 7] : [1, 2, 3, 4, 5]
// How many one-hour rows a session covers (08:00–11:00 = 180 min → 3 rows).
const spanOf = (mins: number) => Math.max(1, Math.ceil((mins || 60) / 60))
const hourOf = (hhmm: string) => parseInt((hhmm || '').split(':')[0] || '0', 10)
// 24h "HH:MM" → "h:MM AM/PM".
function ampm(hhmm: string): string {
  if (!hhmm) return ''
  const [h, m] = hhmm.split(':').map(Number)
  const suffix = h < 12 ? 'AM' : 'PM'
  const h12 = h % 12 === 0 ? 12 : h % 12
  return `${h12}:${String(m).padStart(2, '0')} ${suffix}`
}
function endHHMM(start: string, mins: number): string {
  if (!start) return ''
  const [h, m] = start.split(':').map(Number)
  const t = h * 60 + m + (mins || 60)
  return `${String(Math.floor(t / 60) % 24).padStart(2, '0')}:${String(t % 60).padStart(2, '0')}`
}

interface Unit {
  unit_id: string; name: string; year: number; semester: number
  day_of_week: number; session_start: string; session_duration_minutes: number; lecturers: string; room?: string; lecturer_phone?: string
}
interface Slot {
  unit_id: string; unit_name: string; day_of_week: number; start_time: string
  duration_minutes: number; room: string; lecturer_name: string; lecturer_phone?: string
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
  const { token, logout, fullName } = useAuthStore(s => ({ token: s.token, logout: s.logout, fullName: s.fullName }))
  const displayName = (fullName && fullName.trim()) || 'Coordinator'
  const { theme, toggle } = useTheme()
  const [overview, setOverview] = useState<{ offering: Offering | null; units: Unit[]; slots?: Slot[] } | null>(null)
  const [students, setStudents] = useState<Student[]>([])
  const [last, setLast] = useState<LastRoster | null>(null)
  const [brand, setBrand] = useState<{ name: string; logo_url: string; motto: string; slogan: string } | null>(null)
  const [tab, setTab] = useState<'timetable' | 'units' | 'students' | 'roster' | 'trends'>('timetable')
  const [pwOpen, setPwOpen] = useState(false)
  const [active, setActive] = useState<ActiveSession[]>([])
  const [closingId, setClosingId] = useState<string | null>(null)
  const [refreshing, setRefreshing] = useState(false)
  const [refreshMsg, setRefreshMsg] = useState<string | null>(null)

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

  const loadAll = useCallback(async () => {
    if (!token) return
    const h = { Authorization: `Bearer ${token}` }
    const j = (r: Response) => (r.ok ? r.json() : null)

    // Try API — if any fetch fails, fall back to the cached daily manifest.
    const [overviewData, studentsData, rosterData, brandData] = await Promise.all([
      fetch(`${API}/api/v1/coordinator/overview`, { headers: h }).then(j),
      fetch(`${API}/api/v1/coordinator/students`, { headers: h }).then(j),
      fetch(`${API}/api/v1/coordinator/last-roster`, { headers: h }).then(j),
      fetch(`${API}/api/v1/branding`, { headers: h }).then(j),
    ])

    if (overviewData) {
      setOverview(overviewData)
    } else {
      // Offline fallback: derive overview from cached manifest
      const cached = await loadManifestCached()
      const sessions = (cached as Record<string, unknown>)?.sessions as Array<Record<string, unknown>> | undefined
      if (sessions?.length) {
        const units = sessions.map((s: Record<string, unknown>) => ({
          unit_id: String(s.unit_id ?? ''), name: String(s.unit_name ?? ''), year: 0, semester: 0,
          day_of_week: Number(s.day_of_week ?? 0), session_start: String(s.scheduled_start ?? ''),
          session_duration_minutes: Number(s.session_duration_minutes ?? 60),
          lecturers: String(s.lecturer_name ?? ''),
          room: String(s.venue_id ?? ''),
        }))
        setOverview({ offering: null, units })
      }
    }

    if (studentsData) {
      setStudents(studentsData)
    } else {
      // Offline fallback: derive student list from manifest roster
      const cached = await loadManifestCached()
      const roster = (cached as Record<string, unknown>)?.roster as Record<string, Array<Record<string, string>>> | undefined
      if (roster) {
        const allStudents: Student[] = []
        const seen = new Set<string>()
        for (const unitRoster of Object.values(roster)) {
          for (const entry of unitRoster) {
            const hash = entry.student_id_hash
            if (!seen.has(hash)) {
              seen.add(hash)
              allStudents.push({
                student_id: hash, full_name: entry.qr_serial_number,
                email: '', current_year: 0, semester: 0, intake_session: '', enrollment_status: 'ACTIVE',
              })
            }
          }
        }
        setStudents(allStudents)
      }
    }

    if (rosterData) {
      setLast(rosterData)
    } else {
      // Offline fallback: build last-roster from IndexedDB attendance
      const localSessions = await db.sessions.toArray()
      const recent = localSessions.sort((a, b) => ((b.created_at ?? '') > (a.created_at ?? '') ? 1 : -1))[0]
      if (recent) {
        const records = await db.attendance_records.where('session_id').equals(recent.session_id ?? '').toArray()
        setLast({
          session: { unit_name: recent.unit_name ?? '', session_date: recent.date ?? '', closed_at: recent.created_at ?? '', present: records.length, total: records.length },
          roster: records.map(r => ({ student_id: r.student_id_hash, full_name: '', status: 'PRESENT' as const, checkin_time: r.checkin_timestamp })),
        })
      }
    }

    if (brandData) {
      setBrand(brandData)
    } else {
      const cached = await loadManifestCached()
      if (cached?.institution_public_key) {
        setBrand({ name: 'QAAT', logo_url: '', motto: 'Offline mode', slogan: 'Attendance will sync when online' })
      }
    }
  }, [token])

  useEffect(() => { loadAll() }, [loadAll])

  // Pull current data + upload any attendances still waiting to sync — mirrors the
  // phone app's "Refresh data & sync" so both devices behave the same.
  async function refreshAndSync() {
    setRefreshing(true); setRefreshMsg(null)
    await loadAll(); loadActive()
    try { await processOutboxQueue() } catch { /* offline — will retry */ }
    setRefreshing(false); setRefreshMsg('Data refreshed and pending attendance synced.')
    setTimeout(() => setRefreshMsg(null), 4000)
  }

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
            <div style={{ fontWeight: 800, fontSize: 17 }}>{brand?.name ?? 'QAAT'}</div>
            {brand?.slogan && <div style={{ fontSize: 12, color: 'var(--muted)', fontStyle: 'italic' }}>{brand.slogan}</div>}
            {brand?.motto && <div style={{ fontSize: 11, color: 'var(--muted)' }}>{brand.motto}</div>}
          </div>
        </div>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <button onClick={toggle} title={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`} aria-label="Toggle light or dark theme"
            style={{ background: 'none', border: '1px solid var(--border,#e2e8f0)', color: 'inherit', borderRadius: 6, padding: '7px 11px', cursor: 'pointer', fontSize: 15, lineHeight: 1 }}>
            {theme === 'dark' ? '☀' : '☾'}
          </button>
          <button onClick={() => setPwOpen(true)} style={{ background: 'none', border: '1px solid var(--border,#e2e8f0)', color: 'inherit', borderRadius: 6, padding: '7px 12px', cursor: 'pointer', fontSize: 13 }}>Change password</button>
          <button onClick={logout} style={{ background: 'none', border: 'none', color: 'var(--brand)', cursor: 'pointer' }}>Sign out</button>
        </div>
      </header>
      {pwOpen && <ChangePasswordModal token={token} onClose={() => setPwOpen(false)} />}

      {/* Welcome + refresh — greets the coordinator by the name on their credential and
          re-pulls data + syncs pending attendance, mirroring the phone app's Home. */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12, marginBottom: 14, flexWrap: 'wrap' }}>
        <div>
          <div style={{ fontSize: 20, fontWeight: 800 }}>Welcome, {displayName} 👋</div>
          <div style={{ fontSize: 12, color: 'var(--muted)' }}>Ready when you are — refresh today's data, then take attendance.</div>
        </div>
        <button onClick={refreshAndSync} disabled={refreshing}
          style={{ ...btnPrimary, background: 'var(--brand)', opacity: refreshing ? 0.6 : 1 }}>
          {refreshing ? 'Refreshing…' : '⟳ Refresh data & sync'}
        </button>
      </div>
      {refreshMsg && <div style={{ background: '#ecfdf5', color: '#065f46', padding: '8px 12px', borderRadius: 8, marginBottom: 14, fontSize: 13 }}>{refreshMsg}</div>}

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

      {/* Primary action — opens the Attendance feature (session flow). */}
      {onTakeAttendance && (
        <button onClick={onTakeAttendance} style={{ ...btnPrimary, width: '100%', padding: 14, fontSize: 15, marginBottom: 14 }}>
          Attendance
        </button>
      )}

      <div style={{ display: 'flex', gap: 6, marginBottom: 14, flexWrap: 'wrap' }}>
        {(['timetable', 'units', 'students', 'roster', 'trends'] as const).map(t => (
          <button key={t} onClick={() => setTab(t)} style={{ ...tabBtn, ...(tab === t ? tabActive : {}) }}>
            {t === 'timetable' ? 'Timetable' : t === 'units' ? `Units (${overview?.units.length ?? 0})` : t === 'students' ? `Students (${students.length})` : t === 'roster' ? 'Last roster' : 'Trends'}
          </button>
        ))}
      </div>

      {tab === 'timetable' && <Timetable sessionType={off?.session_type ?? ''} units={
        (overview?.slots?.length
          ? overview.slots.map(s => ({ unit_id: s.unit_id, name: s.unit_name, year: 0, semester: 0, day_of_week: s.day_of_week, session_start: s.start_time, session_duration_minutes: s.duration_minutes, lecturers: s.lecturer_name, room: s.room, lecturer_phone: s.lecturer_phone }))
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
        // The rest (year/semester, intake, level) is inherited from the coordinator's
        // own cohort, so the roster only needs reg-no, name and status.
        <Table head={['Reg No.', 'Student name', 'Status']}
          rows={students.map(s => [s.student_id, s.full_name, s.enrollment_status])}
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

      {tab === 'trends' && <Trends />}

      <ChronicAbsentee />
      <SyncAudit />
    </div>
  )
}

// Weekly timetable — groups the coordinator's units by weekday so they can see,
// at a glance, which sessions run on which day and at what time. Today's column
// is highlighted so "is there a session today?" is unambiguous.
function Timetable({ units, sessionType }: { units: Unit[]; sessionType: string }) {
  // JS getDay(): 0=Sun..6=Sat → our day_of_week 1=Mon..7=Sun.
  const jsDay = new Date().getDay()
  const todayDow = jsDay === 0 ? 7 : jsDay

  const scheduled = units.filter(u => u.day_of_week >= 1 && u.day_of_week <= 7 && u.session_start)
  const unscheduled = units.filter(u => !(u.day_of_week >= 1 && u.day_of_week <= 7 && u.session_start))

  // Weekend cohort → Sat/Sun columns; otherwise Mon–Fri (same as the admin dashboard).
  const days = daysFor(sessionType)

  // Hour rows: widened to cover the WHOLE duration of every session so a multi-hour
  // block has enough rows to span.
  let lo = 8, hi = 19
  for (const u of scheduled) { const h = hourOf(u.session_start); if (h < lo) lo = h; if (h + spanOf(u.session_duration_minutes) > hi) hi = h + spanOf(u.session_duration_minutes) }
  const rows = Array.from({ length: Math.max(1, hi - lo) }, (_, i) => lo + i)

  // Where each unit STARTS + which lower cells it COVERS, so a unit spans all the cells
  // for its full running time (rowSpan) instead of sitting in one cell.
  const covered = new Set<string>()
  const startAt = new Map<string, Unit[]>()
  for (const u of scheduled) {
    const sh = hourOf(u.session_start); const k = `${u.day_of_week}-${sh}`
    startAt.set(k, [...(startAt.get(k) ?? []), u])
    for (let i = 1; i < spanOf(u.session_duration_minutes); i++) covered.add(`${u.day_of_week}-${sh + i}`)
  }

  if (scheduled.length === 0 && unscheduled.length === 0) {
    return <Empty msg="No units in this session yet. Ask your admin to add units, then set each unit's day & time when you open it." />
  }

  return (
    <div>
      <div style={{ border: `2px solid ${KIU_GREEN}`, borderRadius: 10, overflow: 'hidden', background: '#fff', color: '#0f172a' }}>
        <div style={{ textAlign: 'center', padding: '10px 12px', color: KIU_GREEN }}>
          <div style={{ fontWeight: 800, fontSize: 15 }}>Lectures Timetable</div>
        </div>
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', tableLayout: 'fixed', minWidth: 480 }}>
            <thead>
              <tr style={{ background: KIU_GREEN, color: '#fff' }}>
                <th style={{ ...ttTh, width: 84 }}>Time</th>
                {days.map(d => <th key={d} style={ttTh}>{TT_DAYS[d]}{d === todayDow ? ' •' : ''}</th>)}
              </tr>
            </thead>
            <tbody>
              {rows.map(hour => (
                <tr key={hour}>
                  <td style={ttTimeCell}>{ampm(`${String(hour).padStart(2, '0')}:00`)}</td>
                  {days.map(d => {
                    const key = `${d}-${hour}`
                    if (covered.has(key)) return null // a block above spans into this cell
                    const here = startAt.get(key) ?? []
                    const span = here.length ? Math.max(...here.map(u => spanOf(u.session_duration_minutes))) : 1
                    return (
                      <td key={d} rowSpan={span} style={{ ...ttCell, background: d === todayDow ? `color-mix(in srgb, ${KIU_GREEN} 6%, #fff)` : '#fff' }}>
                        {here.map(u => (
                          <div key={u.unit_id} style={ttCard}>
                            <div style={{ fontWeight: 700, fontSize: 12, color: KIU_GREEN }}>{u.name}</div>
                            <div style={{ fontSize: 11, color: '#334155' }}>
                              {ampm(u.session_start)}{u.session_duration_minutes ? `–${ampm(endHHMM(u.session_start, u.session_duration_minutes))}` : ''}{u.room ? ` · ${u.room}` : ''}
                            </div>
                            {u.lecturers && <div style={{ fontSize: 10, color: '#64748b' }}>{u.lecturers}{u.lecturer_phone ? ` · ☎ ${u.lecturer_phone}` : ''}</div>}
                          </div>
                        ))}
                      </td>
                    )
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
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

const ttTh: React.CSSProperties = { padding: '8px 8px', textAlign: 'center', fontSize: 12, borderRight: '1px solid rgba(255,255,255,.25)' }
const ttTimeCell: React.CSSProperties = { padding: '6px 6px', textAlign: 'center', fontSize: 10, fontWeight: 700, color: KIU_GREEN, background: '#f0fdf4', borderBottom: '1px solid #e2e8f0', borderRight: `1px solid ${KIU_GREEN}`, whiteSpace: 'nowrap', verticalAlign: 'top' }
const ttCell: React.CSSProperties = { padding: 4, borderBottom: '1px solid #e2e8f0', borderRight: '1px solid #e2e8f0', verticalAlign: 'top', height: 56 }
const ttCard: React.CSSProperties = { background: '#fff', border: `1px solid ${KIU_GREEN}`, borderLeft: `4px solid ${KIU_GREEN}`, borderRadius: 6, padding: '4px 6px', marginBottom: 4 }

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

// Load the cached daily manifest from IndexedDB (decrypts it in the process).
async function loadManifestCached(): Promise<Record<string, unknown> | null> {
  const userId = useAuthStore.getState().userId
  if (!userId) return null
  const today = new Date().toISOString().split('T')[0]
  const manifestId = `${today}-${userId}`
  const cached = await db.daily_manifest.get(manifestId)
  if (!cached) return null
  try {
    const json = await decrypt(cached.encrypted_blob)
    return JSON.parse(json)
  } catch {
    return null
  }
}

const btnPrimary: React.CSSProperties = { padding: '8px 16px', background: 'var(--brand)', color: '#fff', border: 'none', borderRadius: 8, cursor: 'pointer', fontWeight: 700, fontSize: 13 }
const tabBtn: React.CSSProperties = { padding: '7px 14px', border: '1px solid var(--border,#e2e8f0)', background: 'var(--surface,#fff)', borderRadius: 8, cursor: 'pointer', fontSize: 13, fontWeight: 600, color: 'inherit' }
const tabActive: React.CSSProperties = { background: 'var(--brand)', color: '#fff', borderColor: 'var(--brand)' }
