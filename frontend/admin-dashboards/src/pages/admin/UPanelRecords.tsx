import { useMemo, useState, type CSSProperties } from 'react'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

export type UPanelKind = 'student' | 'lecturer' | 'admin'

export interface UPanelRow {
  id: string
  kind: UPanelKind | string
  person_id: string
  person_name: string
  staff_id: string
  student_id?: string
  full_name?: string
  present: boolean
  verified?: boolean
  event_type: string
  course: string
  timestamp: string
  closed_at?: string
  list_id: string
  session_id: string
  lecturer: string
  lecturer_name?: string
  room: string
  unit: string
  unit_name?: string
  year: string
  sem: string
  semester?: string
  session?: string
  session_date?: string
  program: string
}

export interface UPanelPayload {
  source: string
  base_url: string
  configured: boolean
  list_count: number
  session_count: number
  record_count: number
  student_count: number
  lecturer_count: number
  admin_count: number
  stored: number
  records: UPanelRow[]
  fetched_via: string
  from_cache?: boolean
  message?: string
}

export default function UPanelRecords({ kind }: { kind?: UPanelKind }) {
  const path = kind
    ? `/api/v1/dashboard/upanel/attendance?kind=${kind}`
    : '/api/v1/dashboard/upanel/attendance'
  const { data, status, message, refetch } = useQuery<UPanelPayload>(() => api.get(path), [kind ?? ''])
  const [q, setQ] = useState('')
  const [present, setPresent] = useState('')
  const [lecturer, setLecturer] = useState('')

  const rows = status === 'ok' ? (data?.records ?? []) : []
  const lecturers = useMemo(
    () => Array.from(new Set(rows.map(r => r.lecturer || r.person_name).filter(Boolean))).sort(),
    [rows],
  )
  const filtered = rows.filter(r => {
    if (present === 'yes' && !r.present) return false
    if (present === 'no' && r.present) return false
    if (lecturer && (r.lecturer || r.person_name) !== lecturer) return false
    if (!q) return true
    const hay = `${r.person_id} ${r.person_name} ${r.full_name ?? ''} ${r.staff_id} ${r.student_id ?? ''} ${r.course} ${r.unit} ${r.unit_name ?? ''} ${r.room} ${r.lecturer} ${r.lecturer_name ?? ''} ${r.session ?? ''} ${r.semester ?? ''} ${r.event_type}`.toLowerCase()
    return hay.includes(q.toLowerCase())
  })

  const columns = columnsFor(kind)

  return (
    <div style={{ color: 'var(--text)' }}>
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 12 }}>
        <button type="button" onClick={refetch} style={btnGhost}>
          {kind === 'student' ? 'Refresh sessions' : 'Refresh'}
        </button>
      </div>

      {status === 'ok' && data && (
        <p style={{ color: 'var(--muted)', fontSize: 13, margin: '0 0 14px' }}>
          {kind
            ? `${data.record_count} ${kind === 'student' ? 'student session' : kind} records`
            : `${data.student_count} students · ${data.lecturer_count} lecturers · ${data.admin_count} admin`}
          {data.stored ? ` · ${data.stored} upserted` : ''}
          {data.from_cache ? ' · last stored copy' : ''}
          {data.base_url ? ` · ${data.base_url}` : ''}
        </p>
      )}
      {status === 'ok' && data?.message && (
        <p style={{ color: '#b45309', fontSize: 13, margin: '0 0 14px' }}>{data.message}</p>
      )}

      <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', marginBottom: 14 }}>
        <label style={lab}>
          Search
          <input value={q} onChange={e => setQ(e.target.value)} placeholder="name, id, course, room…" style={inp} />
        </label>
        {kind !== 'admin' && kind !== 'lecturer' && (
          <label style={lab}>
            Present
            <select value={present} onChange={e => setPresent(e.target.value)} style={inp}>
              <option value="">All</option>
              <option value="yes">Present</option>
              <option value="no">Absent</option>
            </select>
          </label>
        )}
        {kind !== 'admin' && lecturers.length > 0 && (
          <label style={lab}>
            Lecturer
            <select value={lecturer} onChange={e => setLecturer(e.target.value)} style={inp}>
              <option value="">All lecturers</option>
              {lecturers.map(n => <option key={n} value={n}>{n}</option>)}
            </select>
          </label>
        )}
      </div>

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading attendance…</p>}
      {status === 'error' && (
        <p style={{ color: '#b91c1c' }}>
          {message || 'Could not load attendance records.'}
        </p>
      )}

      {status === 'ok' && (
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
          <thead>
            <tr style={{ background: 'var(--surface,#f8fafc)' }}>
              {columns.map(h => (
                <th key={h} style={th}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {filtered.map(r => (
              <tr key={r.id || `${r.kind}_${r.session_id}_${r.person_id}_${r.timestamp}`} style={{ borderBottom: '1px solid var(--border,#f1f5f9)' }}>
                {renderCells(kind, r)}
              </tr>
            ))}
            {filtered.length === 0 && (
              <tr>
                <td colSpan={columns.length} style={{ ...td, color: 'var(--muted)' }}>No attendance records match.</td>
              </tr>
            )}
          </tbody>
        </table>
      )}
    </div>
  )
}

function columnsFor(kind?: UPanelKind) {
  if (kind === 'lecturer') return ['Lecturer', 'Event', 'Unit', 'Room', 'Session', 'Year', 'Semester', 'Started', 'Ended']
  if (kind === 'admin') return ['Staff ID', 'Name', 'Event', 'Role / title', 'When']
  if (kind === 'student') return ['Reg No.', 'Name', 'Present', 'Course', 'Unit', 'Session', 'Year', 'Semester', 'Lecturer', 'Room', 'When']
  return ['Kind', 'Person', 'Event', 'Course / unit', 'Lecturer', 'Room', 'When']
}

function renderCells(kind: UPanelKind | undefined, r: UPanelRow) {
  if (kind === 'lecturer') {
    return (
      <>
        <td style={td}>{r.lecturer_name || r.person_name || r.lecturer || r.person_id || '—'}</td>
        <td style={td}>{labelEvent(r.event_type)}</td>
        <td style={td}>{r.unit_name || r.unit || r.course || '—'}</td>
        <td style={td}>{r.room || '—'}</td>
        <td style={td}>{r.session || r.program || '—'}</td>
        <td style={td}>{r.year || '—'}</td>
        <td style={td}>{r.semester || r.sem || '—'}</td>
        <td style={{ ...td, whiteSpace: 'nowrap' }}>{formatWhen(r.timestamp)}</td>
        <td style={{ ...td, whiteSpace: 'nowrap' }}>{formatWhen(r.closed_at ?? '')}</td>
      </>
    )
  }
  if (kind === 'admin') {
    return (
      <>
        <td style={{ ...td, fontFamily: 'monospace', fontSize: 12 }}>{r.staff_id || r.person_id || '—'}</td>
        <td style={td}>{r.full_name || r.person_name || '—'}</td>
        <td style={td}>{labelEvent(r.event_type)}</td>
        <td style={td}>{r.course || '—'}</td>
        <td style={{ ...td, whiteSpace: 'nowrap' }}>{formatWhen(r.timestamp)}</td>
      </>
    )
  }
  if (kind === 'student') {
    return (
      <>
        <td style={{ ...td, fontFamily: 'monospace', fontSize: 12 }}>{r.student_id || r.person_id}</td>
        <td style={td}>{r.full_name || r.person_name || '—'}</td>
        <td style={td}>{r.present ? 'Present' : 'Absent'}</td>
        <td style={td}>{r.course || '—'}</td>
        <td style={td}>{r.unit_name || r.unit || '—'}</td>
        <td style={td}>{r.session || r.program || '—'}</td>
        <td style={td}>{r.year || '—'}</td>
        <td style={td}>{r.semester || r.sem || '—'}</td>
        <td style={td}>{r.lecturer_name || r.lecturer || '—'}</td>
        <td style={td}>{r.room || '—'}</td>
        <td style={{ ...td, whiteSpace: 'nowrap' }}>{formatWhen(r.timestamp)}</td>
      </>
    )
  }
  return (
    <>
      <td style={td}>{r.kind}</td>
      <td style={td}>{r.person_name || r.student_id || r.person_id || '—'}</td>
      <td style={td}>{labelEvent(r.event_type) || (r.present ? 'Present' : 'Absent')}</td>
      <td style={td}>{r.unit || r.course || '—'}</td>
      <td style={td}>{r.lecturer || '—'}</td>
      <td style={td}>{r.room || '—'}</td>
      <td style={{ ...td, whiteSpace: 'nowrap' }}>{formatWhen(r.timestamp)}</td>
    </>
  )
}

function labelEvent(raw: string) {
  switch ((raw || '').toUpperCase()) {
    case 'IN': return 'Arrived'
    case 'OUT': return 'Left'
    case 'LECTURE': return 'Lecture'
    case 'LECTURER_SIGN': return 'Signed in'
    case 'PRESENT': return 'Present'
    case 'ABSENT': return 'Absent'
    default: return raw || '—'
  }
}

function formatWhen(raw: string) {
  if (!raw) return '—'
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return raw
  return d.toLocaleString()
}

const lab: CSSProperties = { display: 'flex', flexDirection: 'column', gap: 4, fontSize: 12, color: 'var(--muted)' }
const inp: CSSProperties = { padding: '6px 8px', border: '1px solid var(--border,#e2e8f0)', borderRadius: 8, fontSize: 13, minWidth: 160, background: 'var(--surface,#fff)', color: 'inherit' }
const th: CSSProperties = { padding: '8px 10px', textAlign: 'left', borderBottom: '1px solid var(--border,#e2e8f0)', whiteSpace: 'nowrap' }
const td: CSSProperties = { padding: '8px 10px' }
const btnGhost: CSSProperties = { padding: '8px 12px', borderRadius: 8, border: '1px solid var(--border,#e2e8f0)', background: 'var(--surface,#fff)', cursor: 'pointer', fontWeight: 600 }
