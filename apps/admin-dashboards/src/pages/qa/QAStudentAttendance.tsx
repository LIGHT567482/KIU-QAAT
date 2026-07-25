import { useState, useRef, useMemo } from 'react'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'
import { useAuth } from '../../contexts/AuthContext'

interface Row {
  student_id: string; full_name: string; course: string; level: string
  session: string; year: number; semester: number
  sessions_held: number; sessions_attended: number; attendance_percentage: number
}
interface CourseOpt { course_id: string; name: string }
interface UnitOpt { unit_id: string; name: string; year: number; semester: number }

export default function QAStudentAttendance() {
  const { user } = useAuth()
  // ADMIN gets real, cascading, server-side filters (existing courses → their units
  // for the cohort). Other oversight roles keep the lightweight client-side filters.
  const isAdmin = user?.role === 'ADMIN'
  const tenantId = user?.tenantId ?? ''

  // ── Admin: server-side filters wired to the endpoint's course_id/unit_id/… ──
  const [sf, setSf] = useState({ course_id: '', unit_id: '', session: '', year: '', semester: '' })
  const qs = useMemo(() => {
    if (!isAdmin) return ''
    const p = new URLSearchParams()
    Object.entries(sf).forEach(([k, v]) => { if (v) p.set(k, v) })
    const s = p.toString(); return s ? `?${s}` : ''
  }, [isAdmin, sf])

  const { data, status } = useQuery<Row[]>(() => api.get(`/api/v1/dashboard/qa/student-attendance${qs}`), [qs])
  const all = status === 'ok' ? (data ?? []) : []

  // Real course list (admin) → drives the Course dropdown and the cascading units.
  const coursesQ = useQuery<CourseOpt[]>(
    () => (isAdmin && tenantId ? api.get(`/api/v1/admin/tenants/${tenantId}/courses`) : Promise.resolve([] as CourseOpt[])),
    [isAdmin, tenantId])
  const courses = coursesQ.status === 'ok' ? (coursesQ.data ?? []) : []
  const unitsQ = useQuery<UnitOpt[]>(
    () => (isAdmin && sf.course_id ? api.get(`/api/v1/admin/courses/${sf.course_id}/units`) : Promise.resolve([] as UnitOpt[])),
    [isAdmin, sf.course_id])
  const units = unitsQ.status === 'ok' ? (unitsQ.data ?? []) : []

  // ── Non-admin: client-derived filters from the returned rows (unchanged) ──
  const [filters, setFilters] = useState({ course: '', session: '', year: '', semester: '' })
  const uniq = (xs: string[]) => Array.from(new Set(xs.filter(Boolean))).sort()
  const courseNames = useMemo(() => uniq(all.map(r => r.course)), [all])
  const sessions = useMemo(() => uniq(all.map(r => r.session)), [all])
  const years = useMemo(() => uniq(all.map(r => String(r.year))), [all])

  const list = isAdmin ? all : all.filter(r =>
    (!filters.course || r.course === filters.course) &&
    (!filters.session || r.session === filters.session) &&
    (!filters.year || String(r.year) === filters.year) &&
    (!filters.semester || String(r.semester) === filters.semester))

  const [importing, setImporting] = useState(false)
  const [msg, setMsg] = useState<string | null>(null)
  const fileRef = useRef<HTMLInputElement>(null)

  async function handleImport(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    setImporting(true); setMsg(null)
    try {
      const fd = new FormData(); fd.append('roster', file)
      const r = await api.upload<{ inserted: number; skipped: number; errors: string[] }>(
        '/api/v1/dashboard/qa/student-attendance/import', fd)
      setMsg(`Imported: ${r.inserted} attendance records added, ${r.skipped} skipped${r.errors?.length ? ` · ${r.errors.length} error(s)` : ''}`)
    } catch (e) { setMsg(e instanceof Error ? `Import failed: ${e.message}` : 'Import failed') }
    finally { setImporting(false); if (fileRef.current) fileRef.current.value = '' }
  }

  function exportXlsx() {
    const suffix = isAdmin ? qs : (() => {
      const p = new URLSearchParams()
      if (filters.session) p.set('session', filters.session)
      if (filters.year) p.set('year', filters.year)
      if (filters.semester) p.set('semester', filters.semester)
      return p.toString() ? `?${p}` : ''
    })()
    api.download(`/api/v1/dashboard/qa/student-attendance/export.xlsx${suffix}`, 'student-attendance.xlsx')
      .catch(e => alert(e instanceof Error ? e.message : 'Export failed'))
  }

  const anyAdminFilter = isAdmin && (sf.course_id || sf.unit_id || sf.session || sf.year || sf.semester)

  return (
    <div style={{ color: 'var(--text)' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 12 }}>
        <div>
          <h2 style={{ margin: '0 0 4px' }}>Student Attendance</h2>
          <p style={{ color: 'var(--muted)', margin: 0, fontSize: 13 }}>Summarised progress per student. Filter by course, its units, session, year &amp; semester.</p>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button onClick={exportXlsx} style={btnGhost}>Export Excel</button>
          <input ref={fileRef} type="file" accept=".csv,.xlsx,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" onChange={handleImport} style={{ display: 'none' }} />
          <button onClick={() => fileRef.current?.click()} disabled={importing} style={btnGhost} title="Import attendance records (columns: session_id, student_id)">{importing ? 'Importing…' : 'Import (CSV/Excel)'}</button>
        </div>
      </div>

      {msg && <div style={{ background: msg.startsWith('Import failed') ? '#fef2f2' : '#f0fdf4', color: msg.startsWith('Import failed') ? '#b91c1c' : '#166534', padding: '10px 14px', borderRadius: 8, marginBottom: 12, fontSize: 13 }}>{msg}</div>}

      {isAdmin ? (
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', marginBottom: 14 }}>
          <SelRaw label="Course" value={sf.course_id} onChange={v => setSf(f => ({ ...f, course_id: v, unit_id: '' }))}
            options={courses.map(c => ({ value: c.course_id, label: c.name }))} allLabel="All courses" />
          <SelRaw label="Course unit" value={sf.unit_id} onChange={v => setSf(f => ({ ...f, unit_id: v }))}
            options={units.map(u => ({ value: u.unit_id, label: `${u.name}${u.year ? ` · Y${u.year}` : ''}${u.semester ? ` S${u.semester}` : ''}` }))}
            allLabel={sf.course_id ? 'All units in course' : 'Select a course first'} disabled={!sf.course_id} />
          <SelRaw label="Session" value={sf.session} onChange={v => setSf(f => ({ ...f, session: v }))}
            options={sessions.map(s => ({ value: s, label: s }))} allLabel="All sessions" />
          <SelRaw label="Year" value={sf.year} onChange={v => setSf(f => ({ ...f, year: v }))}
            options={years.map(y => ({ value: y, label: y }))} allLabel="All years" />
          <SelRaw label="Semester" value={sf.semester} onChange={v => setSf(f => ({ ...f, semester: v }))}
            options={[{ value: '1', label: '1' }, { value: '2', label: '2' }]} allLabel="All" />
          {anyAdminFilter && <button onClick={() => setSf({ course_id: '', unit_id: '', session: '', year: '', semester: '' })} style={{ ...btnGhost, alignSelf: 'flex-end' }}>Clear</button>}
        </div>
      ) : (
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', marginBottom: 14 }}>
          <Sel label="Course" value={filters.course} onChange={v => setFilters(f => ({ ...f, course: v }))} options={courseNames} />
          <Sel label="Session" value={filters.session} onChange={v => setFilters(f => ({ ...f, session: v }))} options={sessions} />
          <Sel label="Year" value={filters.year} onChange={v => setFilters(f => ({ ...f, year: v }))} options={years} />
          <Sel label="Semester" value={filters.semester} onChange={v => setFilters(f => ({ ...f, semester: v }))} options={['1', '2']} />
          {(filters.course || filters.session || filters.year || filters.semester) &&
            <button onClick={() => setFilters({ course: '', session: '', year: '', semester: '' })} style={{ ...btnGhost, alignSelf: 'flex-end' }}>Clear</button>}
        </div>
      )}

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}
      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
        <thead><tr style={{ background: 'var(--surface,#f8fafc)' }}>
          {['Reg No.', 'Name', 'Course', 'Level', 'Session', 'Year', 'Sem', 'Held', 'Attended', 'Progress'].map(h =>
            <th key={h} style={{ padding: '8px 10px', textAlign: 'left', borderBottom: '1px solid var(--border,#e2e8f0)', whiteSpace: 'nowrap' }}>{h}</th>)}
        </tr></thead>
        <tbody>
          {list.map(r => {
            const ok = r.attendance_percentage >= 75
            return (
              <tr key={r.student_id} style={{ borderBottom: '1px solid var(--border,#f1f5f9)' }}>
                <td style={{ padding: '8px 10px', fontFamily: 'monospace', fontSize: 12 }}>{r.student_id}</td>
                <td style={{ padding: '8px 10px', fontWeight: 600 }}>{r.full_name}</td>
                <td style={{ padding: '8px 10px' }}>{r.course}</td>
                <td style={{ padding: '8px 10px' }}>{r.level || '—'}</td>
                <td style={{ padding: '8px 10px' }}>{r.session || '—'}</td>
                <td style={{ padding: '8px 10px', textAlign: 'center' }}>{r.year}</td>
                <td style={{ padding: '8px 10px', textAlign: 'center' }}>{r.semester}</td>
                <td style={{ padding: '8px 10px', textAlign: 'center' }}>{r.sessions_held}</td>
                <td style={{ padding: '8px 10px', textAlign: 'center' }}>{r.sessions_attended}</td>
                <td style={{ padding: '8px 10px' }}>
                  <span style={{ fontWeight: 700, color: ok ? '#16a34a' : '#ef4444' }}>{r.attendance_percentage}%</span>
                  <div style={{ background: '#f1f5f9', borderRadius: 99, height: 6, marginTop: 3, overflow: 'hidden', width: 90 }}>
                    <div style={{ width: `${Math.min(r.attendance_percentage, 100)}%`, height: '100%', background: ok ? '#22c55e' : '#ef4444' }} />
                  </div>
                </td>
              </tr>
            )
          })}
          {status === 'ok' && list.length === 0 && (
            <tr><td colSpan={10} style={{ padding: 28, textAlign: 'center', color: 'var(--muted)' }}>No matching students.</td></tr>
          )}
        </tbody>
      </table>
      {status === 'ok' && <p style={{ color: 'var(--muted)', fontSize: 12, marginTop: 10 }}>Showing {list.length}{isAdmin ? '' : ` of ${all.length}`} students</p>}
    </div>
  )
}

function Sel({ label, value, onChange, options }: { label: string; value: string; onChange: (v: string) => void; options: string[] }) {
  return (
    <label style={{ fontSize: 12 }}>
      <div style={{ color: 'var(--muted)', marginBottom: 3 }}>{label}</div>
      <select value={value} onChange={e => onChange(e.target.value)} style={selStyle}>
        <option value="">{label}: all</option>
        {options.map(o => <option key={o} value={o}>{o}</option>)}
      </select>
    </label>
  )
}

// SelRaw carries a separate value/label per option (e.g. course_id vs course name).
function SelRaw({ label, value, onChange, options, allLabel, disabled }: {
  label: string; value: string; onChange: (v: string) => void; options: { value: string; label: string }[]; allLabel: string; disabled?: boolean
}) {
  return (
    <label style={{ fontSize: 12, opacity: disabled ? 0.55 : 1 }}>
      <div style={{ color: 'var(--muted)', marginBottom: 3 }}>{label}</div>
      <select value={value} disabled={disabled} onChange={e => onChange(e.target.value)} style={selStyle}>
        <option value="">{allLabel}</option>
        {options.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
      </select>
    </label>
  )
}

const selStyle: React.CSSProperties = { padding: '7px 10px', borderRadius: 6, border: '1px solid var(--border,#e2e8f0)', fontSize: 13, background: 'var(--surface,#fff)', color: 'var(--text,#0f172a)', minWidth: 150 }
const btnGhost: React.CSSProperties = { padding: '8px 12px', background: 'var(--surface,#fff)', color: 'var(--text,#334155)', border: '1px solid var(--border,#e2e8f0)', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
