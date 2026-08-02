import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'
import { Kpi, KpiRow, Section } from '../../components/Kpi'

// Lecturer dashboard — every course unit the lecturer is assigned to is listed, each
// collapsed to a single row. Expanding one reveals its session logs: the student
// attendance MATRIX (students × session dates, ✓ = present), split per COHORT so the
// Day, Evening and Weekend runs of a shared unit never bleed into one another.

interface Unit {
  unit_id: string; name: string; year: number; semester: number
  course_name: string; session_count: number; last_session_date: string
}
interface Sess { session_id: string; session_date: string; coordinator_name: string; session_status: string }
interface Student { student_id: string; full_name: string; present: boolean[]; guest: boolean }
interface Cohort {
  offering_id: string; label: string; session_type: string; level: string
  intake: string; coordinator_name: string; sessions: Sess[]; students: Student[]
}

const BLUE = '#2f5fb3'

export default function LecturerDashboard() {
  const { data, status } = useQuery<{ lecturer: { full_name: string } | null; units: Unit[] }>(
    () => api.get('/api/v1/lecturer/overview'))
  const units = data?.units ?? []

  // Which units are expanded. The first unit opens itself once the list arrives, so
  // the lecturer lands on something useful instead of an all-closed page.
  const [open, setOpen] = useState<Set<string>>(new Set())
  const [autoOpened, setAutoOpened] = useState(false)
  useEffect(() => {
    if (autoOpened || units.length === 0) return
    setOpen(new Set([units[0].unit_id]))
    setAutoOpened(true)
  }, [units, autoOpened])

  const toggle = (id: string) => setOpen(p => {
    const n = new Set(p)
    n.has(id) ? n.delete(id) : n.add(id)
    return n
  })

  return (
    <div style={{ color: 'var(--text, #0f172a)' }}>
      <h2 style={{ margin: '0 0 4px' }}>My Teaching</h2>
      <p style={{ color: 'var(--muted,#64748b)', margin: '0 0 16px', fontSize: 13 }}>
        {data?.lecturer?.full_name ? `Signed in as ${data.lecturer.full_name}. ` : ''}
        Your own attendance this month, then every unit you teach — open one to see its session logs.
      </p>

      {/* The lecturer's OWN record, which is the thing they are actually assessed on and the one
          thing this page never showed. The unit panels below are about their students. */}
      <MyTeachingMonth />

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}
      {status === 'ok' && units.length === 0 && (
        <div style={{ background: '#fffbeb', border: '1px solid #fde68a', color: '#92400e', borderRadius: 10, padding: 16 }}>
          You have no assigned units yet. Ask the administrator to assign you to a course unit.
        </div>
      )}

      {units.length > 1 && (
        <div style={{ display: 'flex', gap: 8, marginBottom: 12 }}>
          <button onClick={() => setOpen(new Set(units.map(u => u.unit_id)))} style={linkBtn}>Expand all</button>
          <button onClick={() => setOpen(new Set())} style={linkBtn}>Collapse all</button>
        </div>
      )}

      <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        {units.map(u => (
          <UnitPanel key={u.unit_id} unit={u} open={open.has(u.unit_id)} onToggle={() => toggle(u.unit_id)} />
        ))}
      </div>
    </div>
  )
}

interface CalEvent {
  date: string; unit_id: string; unit_name: string; start_time: string; room: string
  held: boolean; enrolled: number; present: number; pct: number
  lecturer_present: boolean; contact_hours: number
}

/**
 * The lecturer's own month: what they were timetabled to teach, and what they actually taught.
 *
 * The web dashboard showed only their STUDENTS' attendance — the one number a lecturer is himself
 * judged on was visible nowhere but the phone app. `lecturer_present` comes from the gate record in
 * `lecturer_attendance_logs`, so it means "I was there", not merely "a session existed": a
 * coordinator can open a room around a lecturer who never arrived.
 *
 * A slot in the past with no gate record is a MISS. Today is never counted as one — the class may
 * still be ahead of them.
 */
function MyTeachingMonth() {
  const now = new Date()
  const [offset, setOffset] = useState(0)
  const shown = new Date(now.getFullYear(), now.getMonth() + offset, 1)
  const from = `${shown.getFullYear()}-${String(shown.getMonth() + 1).padStart(2, '0')}-01`
  const last = new Date(shown.getFullYear(), shown.getMonth() + 1, 0)
  const to = `${last.getFullYear()}-${String(last.getMonth() + 1).padStart(2, '0')}-${String(last.getDate()).padStart(2, '0')}`

  const { data, status } = useQuery<{ events: CalEvent[] }>(
    () => api.get(`/api/v1/lecturer/calendar?from=${from}&to=${to}`), [from, to])

  const events = data?.events ?? []
  const todayIso = new Date().toISOString().slice(0, 10)
  const taught = events.filter(e => e.lecturer_present).length
  const missed = events.filter(e => !e.lecturer_present && e.date < todayIso).length
  const upcoming = events.length - taught - missed
  const rate = taught + missed > 0 ? Math.round((taught / (taught + missed)) * 100) : 0
  // Students who turned up to the classes that ran, across every unit.
  const studentPct = (() => {
    const held = events.filter(e => e.held && e.enrolled > 0)
    const enrolled = held.reduce((s, e) => s + e.enrolled, 0)
    return enrolled > 0 ? (held.reduce((s, e) => s + e.present, 0) / enrolled) * 100 : null
  })()

  const label = shown.toLocaleDateString(undefined, { month: 'long', year: 'numeric' })

  return (
    <Section
      title="My teaching"
      hint={label}
      right={
        <span style={{ display: 'flex', gap: 6 }}>
          <button onClick={() => setOffset(o => o - 1)} style={linkBtn}>‹ Prev</button>
          <button onClick={() => setOffset(o => o + 1)} style={linkBtn}>Next ›</button>
        </span>
      }
    >
      {status === 'loading' && <p style={{ color: 'var(--muted)', fontSize: 13 }}>Loading…</p>}
      {status === 'ok' && events.length === 0 && (
        <p style={{ color: 'var(--muted)', fontSize: 13 }}>Nothing timetabled for you in {label}.</p>
      )}
      {events.length > 0 && (
        <>
          <KpiRow>
            <Kpi label="Classes taught" value={taught} tone="good" sub={`of ${events.length} timetabled`} />
            <Kpi label="Missed" value={missed} tone={missed > 0 ? 'bad' : 'good'}
              sub={missed > 0 ? 'no gate record' : 'none so far'} />
            {upcoming > 0 && <Kpi label="Still to come" value={upcoming} sub="later this month" />}
            <Kpi label="Attendance rate" value={`${rate}%`} tone={rate >= 90 ? 'good' : rate >= 70 ? 'warn' : 'bad'}
              sub="your own, so far this month" />
            {studentPct !== null && (
              <Kpi label="Your students" value={`${studentPct.toFixed(0)}%`}
                tone={studentPct >= 75 ? 'good' : 'bad'} sub="turned up to your classes" />
            )}
          </KpiRow>

          {missed > 0 && (
            <details style={{ fontSize: 13 }}>
              <summary style={{ cursor: 'pointer', color: '#b91c1c', fontWeight: 600 }}>
                Show the {missed} class{missed === 1 ? '' : 'es'} with no gate record
              </summary>
              <ul style={{ margin: '8px 0 0', paddingLeft: 20, color: 'var(--muted)' }}>
                {events.filter(e => !e.lecturer_present && e.date < todayIso).map(e => (
                  <li key={`${e.unit_id}-${e.date}-${e.start_time}`} style={{ marginBottom: 3 }}>
                    <strong style={{ color: 'var(--text)' }}>{e.unit_id}</strong> — {' '}
                    {new Date(e.date).toLocaleDateString(undefined, { weekday: 'short', day: 'numeric', month: 'short' })}
                    {e.start_time ? ` at ${e.start_time}` : ''}
                    {e.held ? ' (a session was opened, but you never gated in)' : ''}
                  </li>
                ))}
              </ul>
            </details>
          )}
        </>
      )}
    </Section>
  )
}

// One collapsible unit. Its session logs are fetched only once it is first opened,
// so a lecturer with a dozen units doesn't pay for twelve matrices up front.
function UnitPanel({ unit, open, onToggle }: { unit: Unit; open: boolean; onToggle: () => void }) {
  const [loaded, setLoaded] = useState(false)
  useEffect(() => { if (open) setLoaded(true) }, [open])

  const { data, status } = useQuery<{ cohorts: Cohort[] }>(
    () => (loaded
      ? api.get(`/api/v1/lecturer/attendance?unit_id=${encodeURIComponent(unit.unit_id)}`)
      : Promise.resolve({ cohorts: [] as Cohort[] })),
    [loaded, unit.unit_id])
  const cohorts = (data?.cohorts ?? []).filter(c => c.sessions.length > 0 || c.students.length > 0)
  // Until `loaded` flips, the query above resolves an empty placeholder and reports 'ok'.
  // Treat that as busy, or the first expand flashes "No sessions held" before it fetches.
  const busy = !loaded || status === 'loading'

  return (
    <div style={{ border: '1px solid var(--border,#e2e8f0)', borderRadius: 10, overflow: 'hidden', background: 'var(--surface,#fff)' }}>
      <button onClick={onToggle} aria-expanded={open} style={{
        width: '100%', display: 'flex', alignItems: 'center', gap: 12, padding: '12px 16px',
        background: open ? 'color-mix(in srgb, var(--brand) 10%, transparent)' : 'transparent',
        border: 'none', borderBottom: open ? '1px solid var(--border,#e2e8f0)' : 'none',
        cursor: 'pointer', textAlign: 'left', color: 'inherit', font: 'inherit',
      }}>
        <span style={{
          display: 'inline-block', width: 14, flexShrink: 0, fontSize: 12, color: 'var(--muted)',
          transform: open ? 'rotate(90deg)' : 'none', transition: 'transform .15s',
        }}>▶</span>
        <span style={{ flex: 1, minWidth: 0 }}>
          <span style={{ fontWeight: 700 }}>{unit.name}</span>
          <span style={{ color: 'var(--muted)', fontSize: 12, marginLeft: 8, fontFamily: 'monospace' }}>{unit.unit_id}</span>
          <div style={{ color: 'var(--muted)', fontSize: 12, marginTop: 2 }}>
            {unit.course_name} · Y{unit.year}/S{unit.semester}
            {unit.last_session_date ? ` · last session ${fmtDate(unit.last_session_date)}` : ''}
          </div>
        </span>
        <span style={{
          flexShrink: 0, fontSize: 12, fontWeight: 700, borderRadius: 999, padding: '3px 10px',
          background: unit.session_count > 0 ? '#eff6ff' : '#f1f5f9',
          color: unit.session_count > 0 ? '#1d4ed8' : '#64748b',
        }}>
          {unit.session_count} session{unit.session_count === 1 ? '' : 's'}
        </span>
      </button>

      {open && (
        <div style={{ padding: 14 }}>
          {busy && <p style={{ color: 'var(--muted)', margin: 0 }}>Loading session logs…</p>}
          {!busy && status === 'error' && <p style={{ color: '#b91c1c', margin: 0 }}>Could not load this unit's session logs.</p>}
          {!busy && status === 'ok' && cohorts.length === 0 && (
            <p style={{ color: 'var(--muted)', margin: 0 }}>No sessions held for this unit yet.</p>
          )}
          {!busy && status === 'ok' && cohorts.map(c => (
            <CohortLogs key={c.offering_id || 'unassigned'} cohort={c} multiple={cohorts.length > 1} />
          ))}
        </div>
      )}
    </div>
  )
}

// One cohort's session logs. A unit shared across study sessions produces one of
// these per cohort, each with only its own students — a Weekend student is never
// listed under the Day cohort's sessions.
function CohortLogs({ cohort: c, multiple }: { cohort: Cohort; multiple: boolean }) {
  return (
    <div style={{ marginBottom: 18 }}>
      {multiple && (
        <div style={{ fontWeight: 700, fontSize: 13, margin: '0 0 6px', color: BLUE }}>
          {c.label || c.session_type || 'Cohort'}
          <span style={{ color: 'var(--muted)', fontWeight: 400, marginLeft: 8 }}>
            {c.students.length} student{c.students.length === 1 ? '' : 's'} · {c.sessions.length} session{c.sessions.length === 1 ? '' : 's'}
          </span>
        </div>
      )}
      {c.sessions.length === 0 ? (
        <div style={{ color: 'var(--muted)', fontSize: 13 }}>No sessions held for this cohort yet.</div>
      ) : (
        <div style={{ overflowX: 'auto', border: '1px solid var(--border,#e2e8f0)', borderRadius: 8 }}>
          <table style={{ borderCollapse: 'collapse', fontSize: 13, minWidth: 600 }}>
            <thead>
              <tr style={{ background: BLUE, color: '#fff' }}>
                <th style={{ ...hcell, position: 'sticky', left: 0, background: BLUE, textAlign: 'left', minWidth: 200 }}>Name</th>
                <th style={{ ...hcell, minWidth: 120 }}>Reg No.</th>
                {c.sessions.map(s => (
                  <th key={s.session_id} style={{ ...hcell, minWidth: 64 }}
                    title={s.coordinator_name ? `Coordinator: ${s.coordinator_name}` : ''}>
                    {fmtDate(s.session_date)}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {c.students.map((st, ri) => (
                <tr key={st.student_id} style={{ background: ri % 2 ? '#eef3fb' : '#dbe6f6' }}>
                  <td style={{ ...bcell, position: 'sticky', left: 0, background: ri % 2 ? '#eef3fb' : '#dbe6f6', textAlign: 'left', fontWeight: 600 }}>
                    {st.full_name}
                    {st.guest && (
                      <span title="Attended this cohort's session but is enrolled elsewhere"
                        style={{ marginLeft: 6, fontSize: 10, fontWeight: 700, color: '#92400e', background: '#fffbeb', border: '1px solid #fde68a', borderRadius: 999, padding: '1px 6px' }}>
                        guest
                      </span>
                    )}
                  </td>
                  <td style={{ ...bcell, fontFamily: 'monospace', fontSize: 12 }}>{st.student_id}</td>
                  {st.present.map((p, ci) => (
                    <td key={ci} style={{ ...bcell, textAlign: 'center', color: p ? '#15803d' : '#cbd5e1', fontWeight: 700 }}>{p ? '✓' : ''}</td>
                  ))}
                </tr>
              ))}
              {c.students.length === 0 && (
                <tr><td colSpan={2 + c.sessions.length} style={{ padding: 16, color: 'var(--muted)' }}>No students enrolled in this cohort.</td></tr>
              )}
            </tbody>
            {/* Coordinator-of-record row, so the lecturer can see who ran each session. */}
            <tfoot>
              <tr style={{ background: '#f8fafc' }}>
                <td style={{ ...bcell, position: 'sticky', left: 0, background: '#f8fafc', textAlign: 'left', fontWeight: 700, color: BLUE }}>Coordinator</td>
                <td style={bcell}></td>
                {c.sessions.map(s => (
                  <td key={s.session_id} style={{ ...bcell, textAlign: 'center', fontSize: 10, color: 'var(--muted)' }} title={s.coordinator_name}>
                    {s.coordinator_name ? s.coordinator_name.split(' ')[0] : '—'}
                  </td>
                ))}
              </tr>
            </tfoot>
          </table>
        </div>
      )}
    </div>
  )
}

const fmtDate = (d: string) => d ? new Date(d + 'T00:00:00').toLocaleDateString(undefined, { day: '2-digit', month: 'short' }) : '—'

const hcell: React.CSSProperties = { padding: '8px 10px', textAlign: 'center', borderRight: '1px solid rgba(255,255,255,.25)', whiteSpace: 'nowrap', fontWeight: 700 }
const bcell: React.CSSProperties = { padding: '7px 10px', borderRight: '1px solid #c7d6ee', borderTop: '1px solid #c7d6ee', whiteSpace: 'nowrap' }
const linkBtn: React.CSSProperties = { background: 'none', border: 'none', color: 'var(--brand,#2563eb)', cursor: 'pointer', fontSize: 12, fontWeight: 600, padding: 0 }
