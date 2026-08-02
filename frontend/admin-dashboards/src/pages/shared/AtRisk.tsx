import { useMemo, useState } from 'react'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'
import { Kpi, KpiRow, RateBar } from '../../components/Kpi'

/**
 * The exam-eligibility watchlist: students below the attendance threshold, worst first.
 *
 * WHY IT IS ITS OWN PAGE, SHARED BY FIVE ROLES. This information already existed — inside the DQA's
 * eligibility CSV export. That meant the one person who could download it was the one person least
 * able to do anything about it, while the people who actually see these students weekly (the head
 * of department, the dean, the QA rep who visits the class) could not see it at all. The gateway
 * scopes the same endpoint by the caller's own org unit, so this is one page rather than five.
 *
 * DEFICIT IS THE POINT. A percentage tells you someone is failing; the deficit tells you what to do
 * about it — how many more sessions this student must attend to climb back over the bar. Sorted so
 * the recoverable cases and the hopeless ones separate themselves.
 */

interface Risk {
  student_id: string; full_name: string; email: string
  course_name: string; department: string; school: string
  unit_id: string; unit_name: string
  sessions_held: number; sessions_attended: number
  attendance_percentage: number; threshold: number; deficit_sessions: number
}
interface Resp { scope?: string; students?: Risk[]; distinct_students?: number; unset?: boolean; message?: string }

export default function AtRisk() {
  const { status, data } = useQuery<Resp>(() => api.get('/api/v1/org/at-risk?limit=1000'), [])
  const [q, setQ] = useState('')
  const [course, setCourse] = useState('')

  const rows = data?.students ?? []
  const courses = useMemo(
    () => Array.from(new Set(rows.map(r => r.course_name).filter(Boolean))).sort(),
    [rows],
  )

  const filtered = useMemo(() => {
    const needle = q.trim().toLowerCase()
    return rows.filter(r =>
      (!course || r.course_name === course) &&
      (!needle || [r.student_id, r.full_name, r.unit_id, r.unit_name].some(v => (v || '').toLowerCase().includes(needle))))
  }, [rows, q, course])

  // A student failing four units is one person to talk to, not four rows of work.
  const people = useMemo(() => new Set(filtered.map(r => r.student_id)).size, [filtered])
  const threshold = rows[0]?.threshold ?? 75
  // Beyond recovery: the sessions they would need already outnumber the ones left in a normal
  // ~14-week term. Worth separating, because the intervention is a different conversation.
  const severe = filtered.filter(r => r.attendance_percentage < threshold / 2).length

  function exportCsv() {
    const head = ['student_id', 'full_name', 'email', 'course', 'unit_id', 'unit_name',
      'sessions_held', 'sessions_attended', 'attendance_pct', 'threshold', 'sessions_needed']
    const body = filtered.map(r => [
      r.student_id, r.full_name, r.email, r.course_name, r.unit_id, r.unit_name,
      r.sessions_held, r.sessions_attended, r.attendance_percentage, r.threshold, r.deficit_sessions,
    ])
    const csv = [head, ...body]
      .map(cols => cols.map(c => `"${String(c ?? '').replace(/"/g, '""')}"`).join(','))
      .join('\n')
    const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv' }))
    const a = document.createElement('a')
    a.href = url; a.download = 'at-risk-students.csv'; a.click()
    URL.revokeObjectURL(url)
  }

  if (status === 'loading') return <p style={{ color: 'var(--muted)' }}>Loading…</p>
  if (data?.unset) {
    return (
      <div style={warnBox}>
        <strong>Your account has no department or college set.</strong>
        <p style={{ margin: '6px 0 0', fontSize: 13 }}>{data.message}</p>
      </div>
    )
  }

  return (
    <div>
      <h2 style={{ margin: '0 0 4px' }}>At-risk students</h2>
      <p style={{ color: 'var(--muted)', margin: '0 0 18px', fontSize: 13 }}>
        Below the {threshold}% attendance bar for exam eligibility{data?.scope ? <> in <b>{data.scope}</b></> : null}.
        “Needs” is how many more sessions of that unit the student must attend to recover.
      </p>

      <KpiRow>
        <Kpi label="Students at risk" value={people} tone={people > 0 ? 'bad' : 'good'} sub="distinct people" />
        <Kpi label="Student–unit rows" value={filtered.length} sub="one per failing unit" />
        <Kpi label="Severely behind" value={severe} tone={severe > 0 ? 'bad' : 'good'} sub={`under ${(threshold / 2).toFixed(0)}%`} />
        <Kpi label="Eligibility bar" value={`${threshold}%`} sub="set by the DQA" />
      </KpiRow>

      <div style={{ display: 'flex', gap: 8, marginBottom: 14, flexWrap: 'wrap' }}>
        <input
          value={q} onChange={e => setQ(e.target.value)}
          placeholder="Search by name, reg-no or unit…"
          style={{ ...inp, flex: 1, minWidth: 220 }}
        />
        <select value={course} onChange={e => setCourse(e.target.value)} style={inp}>
          <option value="">All courses</option>
          {courses.map(c => <option key={c} value={c}>{c}</option>)}
        </select>
        <button onClick={exportCsv} disabled={filtered.length === 0} style={btn}>Export CSV</button>
      </div>

      {filtered.length === 0 ? (
        <p style={{ color: 'var(--muted)' }}>
          {rows.length === 0
            ? 'Nobody is below the attendance threshold. '
            : 'No student matches those filters.'}
        </p>
      ) : (
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13, minWidth: 760 }}>
            <thead>
              <tr>
                {['Student', 'Reg no.', 'Course', 'Unit', 'Attended', 'Attendance', 'Needs'].map(h => (
                  <th key={h} style={th}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {filtered.map(r => (
                <tr key={`${r.student_id}-${r.unit_id}`}>
                  <td style={td}>
                    {r.full_name}
                    {r.email && <div style={{ fontSize: 11, color: 'var(--muted)' }}>{r.email}</div>}
                  </td>
                  <td style={{ ...td, fontFamily: 'monospace', fontSize: 12 }}>{r.student_id}</td>
                  <td style={td}>{r.course_name}</td>
                  <td style={td}>
                    {r.unit_name}
                    <div style={{ fontSize: 11, color: 'var(--muted)' }}>{r.unit_id}</div>
                  </td>
                  <td style={td}>{r.sessions_attended} / {r.sessions_held}</td>
                  <td style={{ ...td, minWidth: 110 }}>
                    <RateBar pct={r.attendance_percentage} threshold={r.threshold} />
                  </td>
                  <td style={{ ...td, fontWeight: 700, color: '#b91c1c' }}>
                    {r.deficit_sessions > 0 ? `+${r.deficit_sessions}` : '—'}
                    <div style={{ fontSize: 10, fontWeight: 400, color: 'var(--muted)' }}>sessions</div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

const th: React.CSSProperties = {
  textAlign: 'left', padding: '8px 10px', borderBottom: '2px solid var(--border)',
  fontSize: 11, textTransform: 'uppercase', letterSpacing: '.04em', color: 'var(--muted)',
}
const td: React.CSSProperties = { padding: '8px 10px', borderBottom: '1px solid var(--border)', verticalAlign: 'top' }
const inp: React.CSSProperties = {
  padding: '8px 10px', borderRadius: 8, border: '1px solid var(--border)',
  fontSize: 13, background: 'var(--surface)', color: 'var(--text)',
}
const btn: React.CSSProperties = {
  padding: '8px 14px', borderRadius: 8, border: '1px solid var(--border)',
  background: 'var(--surface)', color: 'var(--text)', cursor: 'pointer', fontSize: 13,
}
const warnBox: React.CSSProperties = {
  background: 'rgba(180,83,9,.08)', border: '1px solid rgba(180,83,9,.3)',
  borderRadius: 10, padding: '14px 16px', color: '#92400e',
}
