import { useMemo, useState, type CSSProperties } from 'react'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'
import ExportButtons from '../../components/ExportButtons'
import type { UPanelPayload, UPanelRow } from '../admin/UPanelRecords'

type Status = 'present' | 'absent' | null

interface SessionCol {
  id: string
  ts: number
  label: string
}

interface StudentRow {
  id: string
  name: string
  detail: string
  cells: Record<string, Status>
  percent: number
}

export default function StudentAttendanceRoll() {
  const { data, status, message, refetch } = useQuery<UPanelPayload>(
    () => api.get('/api/v1/dashboard/upanel/attendance?kind=student'),
    [],
  )
  const [q, setQ] = useState('')
  const [course, setCourse] = useState('')
  const [session, setSession] = useState('')
  const [year, setYear] = useState('')
  const [semester, setSemester] = useState('')
  const [unit, setUnit] = useState('')

  const records = status === 'ok' ? (data?.records ?? []) : []
  const courses = useMemo(
    () => Array.from(new Set(records.map(r => r.course).filter(Boolean))).sort(),
    [records],
  )
  const units = useMemo(
    () => Array.from(new Set(records.map(r => r.unit_name || r.unit).filter(Boolean))).sort(),
    [records],
  )
  const sessions = useMemo(
    () => Array.from(new Set(records.map(r => r.session || r.program).filter(Boolean))).sort(),
    [records],
  )
  const years = useMemo(
    () => Array.from(new Set(records.map(r => r.year).filter(Boolean))).sort(),
    [records],
  )
  const semesters = useMemo(
    () => Array.from(new Set(records.map(r => r.semester || r.sem).filter(Boolean))).sort(),
    [records],
  )
  const scoped = useMemo(
    () => records.filter(r =>
      (!course || r.course === course) &&
      (!unit || r.unit_name === unit || r.unit === unit) &&
      (!session || r.session === session || r.program === session) &&
      (!year || r.year === year) &&
      (!semester || r.semester === semester || r.sem === semester),
    ),
    [records, course, unit, session, year, semester],
  )

  const dateCols = useMemo(() => columnsFrom(scoped), [scoped])
  const students = useMemo(() => rowsFrom(scoped, dateCols), [scoped, dateCols])
  const visible = students.filter(s => {
    if (!q.trim()) return true
    const hay = `${s.name} ${s.id} ${s.detail}`.toLowerCase()
    return hay.includes(q.trim().toLowerCase())
  })
  const exportQuery = useMemo(() => {
    const p = new URLSearchParams()
    if (course) p.set('course_id', course)
    if (unit) p.set('unit_id', unit)
    if (session) p.set('session', session)
    if (year) p.set('year', year)
    if (semester) p.set('semester', semester)
    return p.toString()
  }, [course, unit, session, year, semester])

  return (
    <div style={{ color: 'var(--text)' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end', gap: 12, flexWrap: 'wrap', marginBottom: 14 }}>
        <h2 style={{ margin: 0 }}>Student Attendance</h2>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>
          <ExportButtons
            base="/api/v1/dashboard/qa/student-attendance/export"
            filename="student-attendance"
            query={exportQuery}
            disabled={status !== 'ok' || visible.length === 0}
          />
          <button type="button" onClick={refetch} style={btnGhost}>Refresh</button>
        </div>
      </div>

      <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', marginBottom: 14 }}>
        <label style={lab}>
          Search
          <input value={q} onChange={e => setQ(e.target.value)} placeholder="name or registration no." style={inp} />
        </label>
        {courses.length > 0 && (
          <label style={lab}>
            Course
            <select value={course} onChange={e => setCourse(e.target.value)} style={inp}>
              <option value="">All courses</option>
              {courses.map(c => <option key={c} value={c}>{c}</option>)}
            </select>
          </label>
        )}
        {units.length > 0 && (
          <label style={lab}>
            Unit
            <select value={unit} onChange={e => setUnit(e.target.value)} style={inp}>
              <option value="">All units</option>
              {units.map(u => <option key={u} value={u}>{u}</option>)}
            </select>
          </label>
        )}
        {sessions.length > 0 && (
          <label style={lab}>
            Session
            <select value={session} onChange={e => setSession(e.target.value)} style={inp}>
              <option value="">All sessions</option>
              {sessions.map(s => <option key={s} value={s}>{s}</option>)}
            </select>
          </label>
        )}
        {years.length > 0 && (
          <label style={lab}>
            Year
            <select value={year} onChange={e => setYear(e.target.value)} style={inp}>
              <option value="">All years</option>
              {years.map(y => <option key={y} value={y}>{y}</option>)}
            </select>
          </label>
        )}
        {semesters.length > 0 && (
          <label style={lab}>
            Semester
            <select value={semester} onChange={e => setSemester(e.target.value)} style={inp}>
              <option value="">All semesters</option>
              {semesters.map(s => <option key={s} value={s}>{s}</option>)}
            </select>
          </label>
        )}
      </div>

      {status === 'ok' && data?.message && (
        <p style={{ color: '#b45309', fontSize: 13, margin: '0 0 12px' }}>{data.message}</p>
      )}
      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading attendance…</p>}
      {status === 'error' && <p style={{ color: '#b91c1c' }}>{message || 'Could not load attendance.'}</p>}

      {status === 'ok' && (
        <div style={{ overflowX: 'auto', border: '1px solid var(--border,#e2e8f0)', borderRadius: 8, background: 'var(--surface,#fff)' }}>
          <table style={{ borderCollapse: 'collapse', fontSize: 13, minWidth: '100%' }}>
            <thead>
              <tr>
                <th style={{ ...th, ...sticky(0, 56), textAlign: 'left', background: 'var(--surface-2,#f8fafc)', zIndex: 4 }}>%</th>
                <th style={{ ...th, ...sticky(56, 220), textAlign: 'left', background: 'var(--surface-2,#f8fafc)', zIndex: 4 }}>Student</th>
                {dateCols.map(s => (
                  <th key={s.id} style={{ ...th, textAlign: 'center', minWidth: 96 }}>{s.label}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {visible.map(st => (
                <tr key={st.id}>
                  <td style={{ ...td, ...sticky(0, 56), fontWeight: 600 }}>{st.percent}%</td>
                  <td style={{ ...td, ...sticky(56, 220) }}>
                    <div style={{ fontWeight: 600, lineHeight: 1.25 }}>{st.name}</div>
                    <div style={{ color: 'var(--muted,#64748b)', fontSize: 12, marginTop: 2, fontFamily: 'monospace' }}>{st.id}</div>
                    {st.detail && (
                      <div style={{ color: 'var(--muted,#64748b)', fontSize: 11, marginTop: 2 }}>{st.detail}</div>
                    )}
                  </td>
                  {dateCols.map(s => {
                    const cell = st.cells[s.id]
                    return (
                      <td key={s.id} style={{ ...td, textAlign: 'center' }}>
                        <StatusLabel status={cell} />
                      </td>
                    )
                  })}
                </tr>
              ))}
              {visible.length === 0 && (
                <tr>
                  <td colSpan={2 + dateCols.length} style={{ ...td, color: 'var(--muted)', textAlign: 'center', padding: 28 }}>
                    No student attendance records yet.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function StatusLabel({ status }: { status: Status }) {
  if (status === 'present') return <span style={{ color: '#16a34a' }}>present</span>
  if (status === 'absent') return <span style={{ color: '#dc2626' }}>absent</span>
  return <span style={{ color: 'var(--muted,#94a3b8)' }}>—</span>
}

function columnsFrom(records: UPanelRow[]): SessionCol[] {
  const map = new Map<string, SessionCol>()
  for (const r of records) {
    const id = (r.session_id || r.timestamp || r.id || '').trim()
    if (!id) continue
    const ts = Date.parse(r.timestamp) || 0
    const prev = map.get(id)
    if (!prev || ts > prev.ts) {
      map.set(id, { id, ts, label: formatDay(r.timestamp) })
    }
  }
  return [...map.values()].sort((a, b) => b.ts - a.ts || a.id.localeCompare(b.id))
}

function rowsFrom(records: UPanelRow[], sessions: SessionCol[]): StudentRow[] {
  const byStudent = new Map<string, { name: string; detail: string; cells: Record<string, Status> }>()
  for (const r of records) {
    const id = (r.student_id || r.person_id || '').trim()
    if (!id) continue
    const sessionId = (r.session_id || r.timestamp || r.id || '').trim()
    let row = byStudent.get(id)
    if (!row) {
      row = { name: studentName(r, id), detail: studentDetail(r), cells: {} }
      byStudent.set(id, row)
    }
    const nextName = studentName(r, id)
    if (nextName && nextName !== id) row.name = nextName
    if (!row.detail) row.detail = studentDetail(r)
    if (!sessionId) continue
    const next: Status = r.present || (r.event_type || '').toUpperCase() === 'PRESENT' ? 'present' : 'absent'
    const prev = row.cells[sessionId]
    if (prev === 'present') continue
    row.cells[sessionId] = next
  }
  const out: StudentRow[] = []
  for (const [id, row] of byStudent) {
    let present = 0
    let counted = 0
    for (const s of sessions) {
      const cell = row.cells[s.id]
      if (!cell) continue
      counted++
      if (cell === 'present') present++
    }
    out.push({
      id,
      name: row.name,
      detail: row.detail,
      cells: row.cells,
      percent: counted === 0 ? 0 : Math.round((present / counted) * 100),
    })
  }
  out.sort((a, b) => a.name.localeCompare(b.name) || a.id.localeCompare(b.id))
  return out
}

function studentName(r: UPanelRow, fallback: string) {
  return (r.full_name || r.person_name || fallback).trim()
}

function studentDetail(r: UPanelRow) {
  const year = r.year ? `Year ${r.year}` : ''
  const sem = (r.semester || r.sem) ? `Sem ${r.semester || r.sem}` : ''
  return [r.course, r.unit_name || r.unit, r.session || r.program, year, sem].filter(Boolean).join(' · ')
}

function formatDay(raw: string) {
  if (!raw) return '—'
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return raw
  const parts = new Intl.DateTimeFormat('en-GB', {
    timeZone: 'Africa/Kampala',
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
  }).formatToParts(d)
  const day = parts.find(p => p.type === 'day')?.value
  const month = parts.find(p => p.type === 'month')?.value
  const year = parts.find(p => p.type === 'year')?.value
  return `${day}/${month}/${year}`
}

function sticky(left: number, width: number): CSSProperties {
  return {
    position: 'sticky',
    left,
    width,
    minWidth: width,
    maxWidth: width,
    background: 'var(--surface,#fff)',
    zIndex: 2,
    boxShadow: left > 0 ? 'inset -1px 0 0 var(--border,#e2e8f0)' : undefined,
  }
}

const lab: CSSProperties = { display: 'flex', flexDirection: 'column', gap: 4, fontSize: 12, color: 'var(--muted)' }
const inp: CSSProperties = { padding: '6px 8px', border: '1px solid var(--border,#e2e8f0)', borderRadius: 8, fontSize: 13, minWidth: 180, background: 'var(--surface,#fff)', color: 'inherit' }
const th: CSSProperties = {
  padding: '10px 12px',
  borderBottom: '1px solid var(--border,#e2e8f0)',
  background: 'var(--surface-2,#f8fafc)',
  fontWeight: 700,
  whiteSpace: 'nowrap',
  position: 'sticky',
  top: 0,
  zIndex: 3,
}
const td: CSSProperties = {
  padding: '10px 12px',
  borderBottom: '1px solid var(--border,#f1f5f9)',
  verticalAlign: 'middle',
}
const btnGhost: CSSProperties = { padding: '8px 12px', borderRadius: 8, border: '1px solid var(--border,#e2e8f0)', background: 'var(--surface,#fff)', cursor: 'pointer', fontWeight: 600 }
