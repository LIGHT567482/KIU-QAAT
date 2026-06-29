import { useState, useRef, useMemo } from 'react'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

interface Row {
  student_id: string; full_name: string; course: string; level: string
  session: string; year: number; semester: number
  sessions_held: number; sessions_attended: number; attendance_percentage: number
}

export default function QAStudentAttendance() {
  const [filters, setFilters] = useState({ course: '', session: '', year: '', semester: '' })
  // Fetched unfiltered; we filter client-side from the distinct values so QA needs
  // no admin-only course/unit endpoints. (Server filters also exist via querystring.)
  const { data, status } = useQuery<Row[]>(() => api.get('/api/v1/dashboard/qa/student-attendance'))
  const all = status === 'ok' ? (data ?? []) : []

  const uniq = (xs: string[]) => Array.from(new Set(xs.filter(Boolean))).sort()
  const courses = useMemo(() => uniq(all.map(r => r.course)), [all])
  const sessions = useMemo(() => uniq(all.map(r => r.session)), [all])
  const years = useMemo(() => uniq(all.map(r => String(r.year))), [all])

  const list = all.filter(r =>
    (!filters.course   || r.course === filters.course) &&
    (!filters.session  || r.session === filters.session) &&
    (!filters.year     || String(r.year) === filters.year) &&
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
    // Pass the server-supported filters (session/year/semester); course is filtered
    // by display name on the client so it isn't forwarded.
    const qs = new URLSearchParams()
    if (filters.session) qs.set('session', filters.session)
    if (filters.year) qs.set('year', filters.year)
    if (filters.semester) qs.set('semester', filters.semester)
    const suffix = qs.toString() ? `?${qs}` : ''
    api.download(`/api/v1/dashboard/qa/student-attendance/export.xlsx${suffix}`, 'student-attendance.xlsx')
      .catch(e => alert(e instanceof Error ? e.message : 'Export failed'))
  }

  return (
    <div style={{ color: 'var(--text)' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 12 }}>
        <div>
          <h2 style={{ margin: '0 0 4px' }}>Student Attendance</h2>
          <p style={{ color: 'var(--muted)', margin: 0, fontSize: 13 }}>Summarised progress per student. Filter by course, session, year & semester.</p>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button onClick={exportXlsx} style={btnGhost}>Export Excel</button>
          <input ref={fileRef} type="file" accept=".csv,.xlsx,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" onChange={handleImport} style={{ display: 'none' }} />
          <button onClick={() => fileRef.current?.click()} disabled={importing} style={btnGhost} title="Import attendance records (columns: session_id, student_id)">{importing ? 'Importing…' : 'Import (CSV/Excel)'}</button>
        </div>
      </div>

      {msg && <div style={{ background: msg.startsWith('Import failed') ? '#fef2f2' : '#f0fdf4', color: msg.startsWith('Import failed') ? '#b91c1c' : '#166534', padding: '10px 14px', borderRadius: 8, marginBottom: 12, fontSize: 13 }}>{msg}</div>}

      <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', marginBottom: 14 }}>
        <Sel label="Course" value={filters.course} onChange={v => setFilters(f => ({ ...f, course: v }))} options={courses} />
        <Sel label="Session" value={filters.session} onChange={v => setFilters(f => ({ ...f, session: v }))} options={sessions} />
        <Sel label="Year" value={filters.year} onChange={v => setFilters(f => ({ ...f, year: v }))} options={years} />
        <Sel label="Semester" value={filters.semester} onChange={v => setFilters(f => ({ ...f, semester: v }))} options={['1', '2']} />
        {(filters.course || filters.session || filters.year || filters.semester) &&
          <button onClick={() => setFilters({ course: '', session: '', year: '', semester: '' })} style={{ ...btnGhost, alignSelf: 'flex-end' }}>Clear</button>}
      </div>

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
      {status === 'ok' && <p style={{ color: 'var(--muted)', fontSize: 12, marginTop: 10 }}>Showing {list.length} of {all.length} students</p>}
    </div>
  )
}

function Sel({ label, value, onChange, options }: { label: string; value: string; onChange: (v: string) => void; options: string[] }) {
  return (
    <label style={{ fontSize: 12 }}>
      <div style={{ color: 'var(--muted)', marginBottom: 3 }}>{label}</div>
      <select value={value} onChange={e => onChange(e.target.value)} style={{ padding: '7px 10px', borderRadius: 6, border: '1px solid var(--border,#e2e8f0)', fontSize: 13, background: 'var(--surface,#fff)', color: 'var(--text,#0f172a)', minWidth: 130 }}>
        <option value="">{label}: all</option>
        {options.map(o => <option key={o} value={o}>{o}</option>)}
      </select>
    </label>
  )
}
const btnGhost: React.CSSProperties = { padding: '8px 12px', background: 'var(--surface,#fff)', color: 'var(--text,#334155)', border: '1px solid var(--border,#e2e8f0)', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
