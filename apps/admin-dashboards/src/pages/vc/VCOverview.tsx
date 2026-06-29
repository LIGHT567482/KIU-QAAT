import { useState } from 'react'
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, Cell } from 'recharts'
import { api, type VCOverview as VCOverviewData } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

const today = new Date().toISOString().split('T')[0]

export default function VCOverview() {
  const [filters, setFilters] = useState({
    date_from: today, date_to: today,
    course_id: '', unit_id: '', department: '',
  })
  const [applied, setApplied] = useState(filters)

  const queryPath = () => {
    const p = new URLSearchParams()
    if (applied.date_from) p.set('date_from', applied.date_from)
    if (applied.date_to)   p.set('date_to',   applied.date_to)
    if (applied.course_id) p.set('course_id',  applied.course_id)
    if (applied.unit_id)   p.set('unit_id',    applied.unit_id)
    if (applied.department)p.set('department', applied.department)
    return `/api/v1/dashboard/vc/overview?${p.toString()}`
  }

  const { status, data, refetch } = useQuery<VCOverviewData>(
    () => api.get(queryPath()),
    [applied],
  )

  const d = status === 'ok' ? data : null

  const eligChart = d ? [
    { name: 'Eligible',   value: d.eligibility_summary.eligible,   fill: '#22c55e' },
    { name: 'Ineligible', value: d.eligibility_summary.ineligible,  fill: '#ef4444' },
    { name: 'Pending',    value: d.eligibility_summary.pending,      fill: '#f59e0b' },
  ] : []

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <h2 style={{ margin: 0 }}>VC Overview</h2>
        <div style={{ display: 'flex', gap: 8 }}>
          <button onClick={refetch} style={btnStyle}>Refresh</button>
          <a href={`${import.meta.env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')}/api/v1/reports/vc/audit.pdf`}
            style={{ ...btnStyle, background: '#1e293b', color: '#fff', textDecoration: 'none', padding: '6px 14px', borderRadius: 6, fontSize: 13, display: 'inline-block' }}>
            Export PDF
          </a>
        </div>
      </div>

      {/* Filter row */}
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginBottom: 20, padding: '12px 16px', background: '#fff', borderRadius: 8, border: '1px solid #e2e8f0' }}>
        <label style={labelStyle}>
          From
          <input type="date" value={filters.date_from} style={inp}
            onChange={e => setFilters(f => ({ ...f, date_from: e.target.value }))} />
        </label>
        <label style={labelStyle}>
          To
          <input type="date" value={filters.date_to} style={inp}
            onChange={e => setFilters(f => ({ ...f, date_to: e.target.value }))} />
        </label>
        <label style={labelStyle}>
          Course ID
          <input placeholder="e.g. CS-DEPT" value={filters.course_id} style={inp}
            onChange={e => setFilters(f => ({ ...f, course_id: e.target.value }))} />
        </label>
        <label style={labelStyle}>
          Unit ID
          <input placeholder="e.g. CS101" value={filters.unit_id} style={inp}
            onChange={e => setFilters(f => ({ ...f, unit_id: e.target.value }))} />
        </label>
        <label style={labelStyle}>
          Department
          <input placeholder="e.g. Computer Science" value={filters.department} style={inp}
            onChange={e => setFilters(f => ({ ...f, department: e.target.value }))} />
        </label>
        <div style={{ display: 'flex', alignItems: 'flex-end' }}>
          <button onClick={() => setApplied({ ...filters })} style={{ ...btnStyle, background: '#1e293b', color: '#fff' }}>
            Apply Filters
          </button>
        </div>
      </div>

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}
      {status === 'error' && (
        <div style={{ background: '#fef2f2', color: '#b91c1c', padding: 16, borderRadius: 8, marginBottom: 20 }}>
          Failed to load. <button onClick={refetch} style={{ marginLeft: 8 }}>Retry</button>
        </div>
      )}

      {d && (
        <>
          {/* KPI Cards */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(150px,1fr))', gap: 16, marginBottom: 24 }}>
            <KPI label="Scheduled sessions"  value={d.total_sessions_scheduled} />
            <KPI label="Actual sessions"     value={d.total_sessions_actual} />
            <KPI label="Students present"    value={d.total_students_today} />
            <KPI label="Avg attendance"      value={`${d.avg_attendance_pct}%`} />
            <KPI label="IER"                 value={`${d.ier}%`}
              tooltip="Institutional Efficiency Ratio = (Actual / Scheduled) × Avg Attendance" />
            <KPI label="Ghost lectures"      value={d.ghost_lecture_count} alert={d.ghost_lecture_count > 0} />
          </div>

          {/* Eligibility chart */}
          <h3 style={{ marginBottom: 12 }}>Exam Eligibility Summary</h3>
          <ResponsiveContainer width="100%" height={180}>
            <BarChart data={eligChart} layout="vertical">
              <XAxis type="number" />
              <YAxis type="category" dataKey="name" width={80} />
              <Tooltip />
              <Bar dataKey="value" radius={[0, 4, 4, 0]}>
                {eligChart.map((e, i) => <Cell key={i} fill={e.fill} />)}
              </Bar>
            </BarChart>
          </ResponsiveContainer>

          {/* Ghost lecture list */}
          {d.ghost_sessions.length > 0 && (
            <div style={{ marginTop: 28 }}>
              <h3 style={{ marginBottom: 12, color: '#b91c1c' }}>Ghost Lectures Flagged</h3>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
                <thead>
                  <tr style={{ background: '#fef2f2' }}>
                    {['Unit', 'Date', 'Students Present'].map(h => (
                      <th key={h} style={{ padding: '8px 12px', textAlign: 'left', borderBottom: '1px solid #fca5a5' }}>{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {d.ghost_sessions.map(gs => (
                    <tr key={gs.session_id} style={{ borderBottom: '1px solid #f1f5f9' }}>
                      <td style={{ padding: '8px 12px', fontWeight: 600 }}>{gs.unit_name}
                        <span style={{ color: 'var(--muted)', fontSize: 12, marginLeft: 6 }}>{gs.unit_id}</span></td>
                      <td style={{ padding: '8px 12px', color: 'var(--muted)' }}>{gs.session_date}</td>
                      <td style={{ padding: '8px 12px', color: '#b91c1c', fontWeight: 600 }}>{gs.student_count}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </div>
  )
}

function KPI({ label, value, alert, tooltip }: { label: string; value: number | string; alert?: boolean; tooltip?: string }) {
  return (
    <div title={tooltip} style={{
      background: '#fff', borderRadius: 10, padding: '16px 20px',
      boxShadow: '0 1px 4px rgba(0,0,0,.06)',
      border: alert ? '2px solid #ef4444' : '1px solid #e2e8f0',
      cursor: tooltip ? 'help' : 'default',
    }}>
      <div style={{ fontSize: 28, fontWeight: 700, color: alert ? '#ef4444' : '#1e293b' }}>{value}</div>
      <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 4 }}>{label}</div>
    </div>
  )
}

const btnStyle: React.CSSProperties = {
  padding: '6px 14px', borderRadius: 6, border: '1px solid #e2e8f0',
  background: '#fff', cursor: 'pointer', fontSize: 13,
}
const labelStyle: React.CSSProperties = {
  display: 'flex', flexDirection: 'column', gap: 4, fontSize: 12, color: 'var(--muted)', fontWeight: 600,
}
const inp: React.CSSProperties = {
  padding: '6px 10px', fontSize: 13, borderRadius: 6,
  border: '1px solid #e2e8f0', background: '#fff',
}
