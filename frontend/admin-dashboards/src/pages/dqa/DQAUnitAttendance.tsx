import { useMemo, useState } from 'react'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'
import ExportButtons from '../../components/ExportButtons'

/**
 * BOTH ATTENDANCE RECORDS, ONE ROW PER UNIT.
 *
 * The system keeps two independent accounts of every lecture — whether the lecturer was there, and
 * whether the students were — and they have only ever been readable on separate pages. A director
 * asking "is this unit in trouble?" had to note figures off one screen, open another, and line them
 * up by hand.
 *
 * THE INTERESTING CASES ARE THE DISAGREEMENTS, and they are exactly what that manual reconciliation
 * loses. Two of them are called out here rather than left to be spotted:
 *
 *   - Sessions with NO lecturer record. Students were marked present at a lecture the system has no
 *     evidence anybody taught. That is either a lecturer who never scanned or attendance recorded
 *     for a class that did not happen, and both are worth a question.
 *   - Turnout far below teaching. The lectures happened and nobody came, which is a different
 *     problem from a lecturer who never showed up and needs a different conversation.
 *
 * MANUAL ENTRIES ARE COUNTED, NEVER FILTERED OUT — a student marked present on paper by their
 * coordinator attended the lecture. They are shown in their own column only so that a unit whose
 * entire register was typed in after the fact is visible as such, which is a fact about the
 * evidence rather than about the attendance.
 */

interface Row {
  unit_id: string; unit_name: string; course: string
  department: string; school: string; lecturer_name: string
  sessions_held: number; lectures_recorded: number; contact_hours: number
  students_enrolled: number; student_checkins: number; manual_checkins: number
  student_attendance_pct: number
  sessions_without_lecturer_record: number
}

export default function DQAUnitAttendance() {
  const [days, setDays] = useState(90)
  const [q, setQ] = useState('')
  const [onlyFlagged, setOnlyFlagged] = useState(false)

  const { status, data, refetch } = useQuery<Row[]>(
    () => api.get(`/api/v1/dashboard/dqa/unit-attendance?days=${days}`),
    [days],
  )
  const rows = status === 'ok' ? (data ?? []) : []

  const shown = useMemo(() => {
    const needle = q.trim().toLowerCase()
    return rows.filter(r => {
      if (onlyFlagged && r.sessions_without_lecturer_record === 0) return false
      if (!needle) return true
      return [r.unit_id, r.unit_name, r.course, r.department, r.school, r.lecturer_name]
        .some(v => (v ?? '').toLowerCase().includes(needle))
    })
  }, [rows, q, onlyFlagged])

  const flagged = rows.filter(r => r.sessions_without_lecturer_record > 0).length

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 12, flexWrap: 'wrap' }}>
        <div>
          <h2 style={{ margin: 0 }}>Unit Attendance</h2>
          <p style={{ color: 'var(--muted)', margin: '4px 0 0', fontSize: 13 }}>
            Was the unit taught, and did the cohort turn up — the lecturer record and the student
            record side by side.
          </p>
        </div>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
          <select value={days} onChange={e => setDays(Number(e.target.value))} style={sel}>
            <option value={30}>Last 30 days</option>
            <option value={90}>This semester (90 days)</option>
            <option value={365}>This year</option>
          </select>
          <ExportButtons base="/api/v1/dashboard/dqa/unit-attendance/export"
            filename="unit-attendance" query={`days=${days}`} />
          <button onClick={refetch} style={btn}>Refresh</button>
        </div>
      </div>

      <div style={{ display: 'flex', gap: 10, alignItems: 'center', margin: '16px 0 12px', flexWrap: 'wrap' }}>
        <input value={q} onChange={e => setQ(e.target.value)} placeholder="Filter by unit, course, department or lecturer"
          style={{ ...sel, minWidth: 320, flex: '1 1 320px' }} />
        <label style={{ fontSize: 13, display: 'flex', gap: 6, alignItems: 'center', cursor: 'pointer' }}>
          <input type="checkbox" checked={onlyFlagged} onChange={e => setOnlyFlagged(e.target.checked)} />
          Only units with sessions missing a lecturer record{flagged > 0 ? ` (${flagged})` : ''}
        </label>
      </div>

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}
      {status === 'error' && <p style={{ color: '#b91c1c' }}>Failed to load unit attendance.</p>}

      {status === 'ok' && rows.length === 0 && (
        <p style={{ color: 'var(--muted)', marginTop: 40, textAlign: 'center' }}>
          No sessions in this window.
        </p>
      )}

      {shown.length > 0 && (
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
            <thead>
              <tr style={{ background: '#f8fafc' }}>
                {['Unit', 'Course / Dept', 'Lecturer', 'Sessions', 'Lectures recorded',
                  'No lecturer record', 'Contact hrs', 'Enrolled', 'Check-ins', 'Attendance'].map(h => (
                  <th key={h} style={th}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {shown.map(r => (
                <tr key={r.unit_id} style={{ borderBottom: '1px solid #f1f5f9' }}>
                  <td style={td}>
                    <div style={{ fontWeight: 600 }}>{r.unit_name}</div>
                    <div style={{ fontSize: 11, color: 'var(--muted)' }}>{r.unit_id}</div>
                  </td>
                  <td style={{ ...td, color: 'var(--muted)' }}>
                    <div>{r.course}</div>
                    <div style={{ fontSize: 11 }}>{[r.department, r.school].filter(Boolean).join(' · ')}</div>
                  </td>
                  <td style={td}>{r.lecturer_name || <span style={{ color: '#b45309' }}>unassigned</span>}</td>
                  <td style={{ ...td, textAlign: 'center' }}>{r.sessions_held}</td>
                  <td style={{ ...td, textAlign: 'center' }}>{r.lectures_recorded}</td>
                  <td style={{ ...td, textAlign: 'center' }}>
                    {r.sessions_without_lecturer_record > 0 ? (
                      <span style={{ background: '#fef2f2', color: '#991b1b', padding: '2px 9px', borderRadius: 999, fontWeight: 700 }}>
                        {r.sessions_without_lecturer_record}
                      </span>
                    ) : <span style={{ color: '#16a34a' }}>0</span>}
                  </td>
                  <td style={{ ...td, textAlign: 'center' }}>{r.contact_hours.toFixed(1)}</td>
                  <td style={{ ...td, textAlign: 'center' }}>{r.students_enrolled}</td>
                  <td style={{ ...td, textAlign: 'center' }}>
                    {r.student_checkins}
                    {r.manual_checkins > 0 && (
                      <div style={{ fontSize: 10, color: 'var(--muted)' }}>{r.manual_checkins} manual</div>
                    )}
                  </td>
                  <td style={{ ...td, textAlign: 'center' }}>
                    {r.students_enrolled > 0 && r.sessions_held > 0
                      ? <span style={{ fontWeight: 700, color: pctColour(r.student_attendance_pct) }}>
                          {r.student_attendance_pct.toFixed(0)}%
                        </span>
                      // No enrolment or no sessions means there is nothing to divide by. Showing 0%
                      // would read as total absence, which is a different and false claim.
                      : <span style={{ color: 'var(--muted)' }}>—</span>}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {status === 'ok' && rows.length > 0 && shown.length === 0 && (
        <p style={{ color: 'var(--muted)', marginTop: 24, textAlign: 'center' }}>
          No units match this filter.
        </p>
      )}
    </div>
  )
}

function pctColour(p: number) {
  if (p >= 75) return '#16a34a'
  if (p >= 50) return '#f59e0b'
  return '#dc2626'
}

const th: React.CSSProperties = { padding: '8px 10px', textAlign: 'left', borderBottom: '1px solid #e2e8f0', whiteSpace: 'nowrap' }
const td: React.CSSProperties = { padding: '9px 10px' }
const btn: React.CSSProperties = { padding: '7px 16px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const sel: React.CSSProperties = { padding: '7px 10px', borderRadius: 6, border: '1px solid #cbd5e1', fontSize: 13, background: '#fff' }
