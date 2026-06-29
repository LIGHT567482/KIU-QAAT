import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

// Lecturer dashboard — the logged-in lecturer picks one of their units and sees the
// student attendance MATRIX (students × session dates, ✓ = present), plus the
// coordinator who ran each session. Styled after the attendance-grid screenshot.

interface Unit { unit_id: string; name: string; year: number; semester: number; course_name: string }
interface Sess { session_id: string; session_date: string; coordinator_name: string }
interface Student { student_id: string; full_name: string; present: boolean[] }
interface Coord { coordinator_id: string; coordinator_name: string }

const BLUE = '#2f5fb3'

export default function LecturerDashboard() {
  const { data, status } = useQuery<{ lecturer: { full_name: string } | null; units: Unit[] }>(
    () => api.get('/api/v1/lecturer/overview'))
  const units = data?.units ?? []
  const [unitId, setUnitId] = useState('')
  // A unit can be shared across coordinators/cohorts — filter by who ran the sessions.
  const [coordinator, setCoordinator] = useState('')
  useEffect(() => { if (!unitId && units.length) setUnitId(units[0].unit_id) }, [units, unitId])

  const { data: grid, status: gridStatus } = useQuery<{ sessions: Sess[]; students: Student[]; coordinators: Coord[] }>(
    () => api.get(`/api/v1/lecturer/attendance?unit_id=${encodeURIComponent(unitId)}${coordinator ? `&coordinator=${encodeURIComponent(coordinator)}` : ''}`), [unitId, coordinator])
  const sessions = grid?.sessions ?? []
  const students = grid?.students ?? []
  const coordinators = grid?.coordinators ?? []
  const fmtDate = (d: string) => d ? new Date(d + 'T00:00:00').toLocaleDateString(undefined, { day: '2-digit', month: 'short' }) : '—'

  return (
    <div style={{ color: 'var(--text, #0f172a)' }}>
      <h2 style={{ margin: '0 0 4px' }}>My Attendance</h2>
      <p style={{ color: 'var(--muted,#64748b)', margin: '0 0 16px', fontSize: 13 }}>
        {data?.lecturer?.full_name ? `Signed in as ${data.lecturer.full_name}. ` : ''}
        Pick a unit to see who attended each lecture.
      </p>

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}
      {status === 'ok' && units.length === 0 && (
        <div style={{ background: '#fffbeb', border: '1px solid #fde68a', color: '#92400e', borderRadius: 10, padding: 16 }}>
          You have no assigned units yet. Ask the administrator to assign you to a course unit.
        </div>
      )}

      {units.length > 0 && (
        <div style={{ marginBottom: 14, display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center' }}>
          <select value={unitId} onChange={e => { setUnitId(e.target.value); setCoordinator('') }}
            style={{ padding: '8px 12px', borderRadius: 8, border: '1px solid var(--border,#e2e8f0)', fontSize: 14, background: 'var(--surface,#fff)', color: 'inherit', minWidth: 320 }}>
            {units.map(u => <option key={u.unit_id} value={u.unit_id}>{u.unit_id} — {u.name} ({u.course_name}, Y{u.year}/S{u.semester})</option>)}
          </select>
          {/* Filter: this unit may be run by several coordinators (shared unit). */}
          <select value={coordinator} onChange={e => setCoordinator(e.target.value)}
            style={{ padding: '8px 12px', borderRadius: 8, border: '1px solid var(--border,#e2e8f0)', fontSize: 14, background: 'var(--surface,#fff)', color: 'inherit' }}>
            <option value="">All coordinators</option>
            {coordinators.map(c => <option key={c.coordinator_id} value={c.coordinator_id}>{c.coordinator_name || c.coordinator_id}</option>)}
          </select>
        </div>
      )}

      {unitId && gridStatus === 'ok' && (
        sessions.length === 0 ? (
          <div style={{ color: 'var(--muted)', padding: '20px 0' }}>No sessions held for this unit yet.</div>
        ) : (
          <div style={{ overflowX: 'auto', border: '1px solid var(--border,#e2e8f0)', borderRadius: 10 }}>
            <table style={{ borderCollapse: 'collapse', fontSize: 13, minWidth: 600 }}>
              <thead>
                <tr style={{ background: BLUE, color: '#fff' }}>
                  <th style={{ ...hcell, position: 'sticky', left: 0, background: BLUE, textAlign: 'left', minWidth: 200 }}>Name</th>
                  <th style={{ ...hcell, minWidth: 120 }}>Reg No.</th>
                  {sessions.map(s => (
                    <th key={s.session_id} style={{ ...hcell, minWidth: 64 }} title={s.coordinator_name ? `Coordinator: ${s.coordinator_name}` : ''}>
                      {fmtDate(s.session_date)}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {students.map((st, ri) => (
                  <tr key={st.student_id} style={{ background: ri % 2 ? '#eef3fb' : '#dbe6f6' }}>
                    <td style={{ ...bcell, position: 'sticky', left: 0, background: ri % 2 ? '#eef3fb' : '#dbe6f6', textAlign: 'left', fontWeight: 600 }}>{st.full_name}</td>
                    <td style={{ ...bcell, fontFamily: 'monospace', fontSize: 12 }}>{st.student_id}</td>
                    {st.present.map((p, ci) => (
                      <td key={ci} style={{ ...bcell, textAlign: 'center', color: p ? '#15803d' : '#cbd5e1', fontWeight: 700 }}>{p ? '✓' : ''}</td>
                    ))}
                  </tr>
                ))}
                {students.length === 0 && (
                  <tr><td colSpan={2 + sessions.length} style={{ padding: 16, color: 'var(--muted)' }}>No students enrolled in this unit.</td></tr>
                )}
              </tbody>
              {/* Coordinator-of-record row, so the lecturer can see who ran each session. */}
              <tfoot>
                <tr style={{ background: '#f8fafc' }}>
                  <td style={{ ...bcell, position: 'sticky', left: 0, background: '#f8fafc', textAlign: 'left', fontWeight: 700, color: BLUE }}>Coordinator</td>
                  <td style={bcell}></td>
                  {sessions.map(s => (
                    <td key={s.session_id} style={{ ...bcell, textAlign: 'center', fontSize: 10, color: 'var(--muted)' }} title={s.coordinator_name}>
                      {s.coordinator_name ? s.coordinator_name.split(' ')[0] : '—'}
                    </td>
                  ))}
                </tr>
              </tfoot>
            </table>
          </div>
        )
      )}
    </div>
  )
}

const hcell: React.CSSProperties = { padding: '8px 10px', textAlign: 'center', borderRight: '1px solid rgba(255,255,255,.25)', whiteSpace: 'nowrap', fontWeight: 700 }
const bcell: React.CSSProperties = { padding: '7px 10px', borderRight: '1px solid #c7d6ee', borderTop: '1px solid #c7d6ee', whiteSpace: 'nowrap' }
