import { useState } from 'react'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

/**
 * Assigning lecturers to units — the head of department's screen.
 *
 * This used to be ADMIN-only, on a route that takes the tenant from the URL and
 * trusts it. A head of department does the same job for their own department, so
 * the page talks to /api/v1/hod/*, where the scope is resolved from the caller's
 * own user row and a unit outside it is refused by the server. Nothing here sends
 * a department: the page cannot widen its own reach, only display what it is given.
 *
 * The assignment is not cosmetic. It is what files a lecturer under a department,
 * what puts their name on the coordinator's dashboard and the monitor manifest, and
 * what the QA record is matched against — a unit with nobody assigned shows a
 * blank lecturer on every surface in the system.
 */
interface Assignment {
  assignment_id: string
  lecturer_id: string; lecturer_name: string; staff_id: string
  unit_id: string; unit_name: string
  course_id: string; course_name: string; department: string
  academic_year: string; year: number; semester: number; intake_session: string
}
interface UnitOpt {
  unit_id: string; unit_name: string; course_id: string; course_name: string
  year: number; semester: number; assigned: number
}
interface LecOpt { lecturer_id: string; full_name: string; staff_id: string }

export default function LecturerAssignments() {
  const list = useQuery<{ scope?: string; unset?: boolean; message?: string; assignments: Assignment[] }>(
    () => api.get('/api/v1/hod/assignments'))
  const opts = useQuery<{ unset?: boolean; units: UnitOpt[]; lecturers: LecOpt[] }>(
    () => api.get('/api/v1/hod/assignable'))

  const [unit, setUnit] = useState('')
  const [lecturer, setLecturer] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [q, setQ] = useState('')

  const rows = (list.data?.assignments ?? []).filter(a => {
    const s = q.trim().toLowerCase()
    return !s || [a.lecturer_name, a.staff_id, a.unit_id, a.unit_name, a.course_name]
      .some(v => (v || '').toLowerCase().includes(s))
  })

  async function assign() {
    if (!unit || !lecturer) return
    setBusy(true); setErr(null)
    try {
      await api.post('/api/v1/hod/assignments', { unit_id: unit, lecturer_id: lecturer })
      setUnit(''); setLecturer('')
      list.refetch(); opts.refetch()
    } catch (e) { setErr(e instanceof Error ? e.message : 'Could not assign') }
    finally { setBusy(false) }
  }

  async function unassign(id: string, who: string, what: string) {
    if (!confirm(`Remove ${who} from ${what}?`)) return
    try {
      await api.delete(`/api/v1/hod/assignments/${id}`)
      list.refetch(); opts.refetch()
    } catch (e) { alert(e instanceof Error ? e.message : 'Could not remove') }
  }

  // A bounded role with no department set matches nothing. The server says so
  // explicitly rather than returning an empty list, because an empty table looks
  // like an empty institution and sends people looking for the wrong problem.
  if (list.data?.unset) {
    return (
      <div>
        <h2 style={{ margin: '0 0 6px' }}>Assignments</h2>
        <p style={{ color: '#b45309' }}>{list.data.message}</p>
      </div>
    )
  }

  const units = opts.data?.units ?? []
  const lecturers = opts.data?.lecturers ?? []
  const unstaffed = units.filter(u => u.assigned === 0).length

  return (
    <div>
      <div style={{ marginBottom: 14 }}>
        <h2 style={{ margin: '0 0 2px' }}>Assignments</h2>
        <p style={{ color: 'var(--muted)', margin: 0, fontSize: 13 }}>
          Who teaches what in {list.data?.scope || 'your department'}.
          {unstaffed > 0 && <> <strong style={{ color: '#b45309' }}>{unstaffed} unit{unstaffed === 1 ? '' : 's'} still have nobody assigned</strong> — their lecturer shows blank on every dashboard until you set one.</>}
        </p>
      </div>

      <div style={{ display: 'flex', gap: 10, alignItems: 'flex-end', flexWrap: 'wrap', background: 'var(--surface-2,#f8fafc)', border: '1px solid var(--border,#e2e8f0)', borderRadius: 10, padding: 14, marginBottom: 16 }}>
        <label style={{ display: 'block' }}>
          <div style={lbl}>Unit</div>
          <select value={unit} onChange={e => setUnit(e.target.value)} style={sel}>
            <option value="">— select a unit —</option>
            {units.map(u => (
              <option key={u.unit_id} value={u.unit_id}>
                {u.unit_id} — {u.unit_name}{u.assigned === 0 ? '  (unassigned)' : ''}
              </option>
            ))}
          </select>
        </label>
        <label style={{ display: 'block' }}>
          <div style={lbl}>Lecturer</div>
          <select value={lecturer} onChange={e => setLecturer(e.target.value)} style={sel}>
            <option value="">— select a lecturer —</option>
            {lecturers.map(l => (
              <option key={l.lecturer_id} value={l.lecturer_id}>
                {l.full_name}{l.staff_id ? ` (${l.staff_id})` : ''}
              </option>
            ))}
          </select>
        </label>
        <button onClick={assign} disabled={!unit || !lecturer || busy} style={btnPrimary}>
          {busy ? 'Assigning…' : 'Assign'}
        </button>
        {err && <span style={{ color: '#b91c1c', fontSize: 12 }}>{err}</span>}
      </div>

      <input value={q} onChange={e => setQ(e.target.value)} placeholder="Search a lecturer, unit or course…"
        style={{ width: '100%', maxWidth: 380, padding: '8px 12px', borderRadius: 8, border: '1px solid var(--border,#e2e8f0)', fontSize: 14, marginBottom: 12, boxSizing: 'border-box' }} />

      <div style={{ overflowX: 'auto' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
          <thead>
            <tr>
              {['Lecturer', 'Staff ID', 'Unit', 'Course', 'Year/Sem', ''].map(h => (
                <th key={h} style={th}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map(a => (
              <tr key={a.assignment_id}>
                <td style={{ ...td, fontWeight: 600 }}>{a.lecturer_name}</td>
                <td style={td}>{a.staff_id || '—'}</td>
                <td style={td}>{a.unit_id} — {a.unit_name}</td>
                <td style={td}>{a.course_name}</td>
                <td style={td}>Y{a.year} S{a.semester}</td>
                <td style={td}>
                  <button onClick={() => unassign(a.assignment_id, a.lecturer_name, a.unit_id)} style={btnGhost}>Remove</button>
                </td>
              </tr>
            ))}
            {rows.length === 0 && (
              <tr><td colSpan={6} style={{ ...td, color: 'var(--muted)' }}>
                {list.status === 'loading' ? 'Loading…' : 'Nothing assigned yet.'}
              </td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}

const lbl: React.CSSProperties = { fontSize: 11, fontWeight: 700, color: 'var(--muted)', marginBottom: 3 }
const sel: React.CSSProperties = { padding: '8px 10px', borderRadius: 8, border: '1px solid var(--border,#e2e8f0)', fontSize: 13, minWidth: 220 }
const th: React.CSSProperties = { padding: '8px 12px', textAlign: 'left', borderBottom: '1px solid var(--border,#e2e8f0)', color: 'var(--muted)', fontWeight: 600, whiteSpace: 'nowrap' }
const td: React.CSSProperties = { padding: '8px 12px', borderBottom: '1px solid var(--border,#f1f5f9)' }
const btnPrimary: React.CSSProperties = { padding: '8px 16px', background: 'var(--brand)', color: 'var(--brand-contrast,#fff)', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const btnGhost: React.CSSProperties = { padding: '5px 10px', background: 'transparent', color: 'var(--text,#334155)', border: '1px solid var(--border,#e2e8f0)', borderRadius: 6, cursor: 'pointer', fontSize: 12 }
