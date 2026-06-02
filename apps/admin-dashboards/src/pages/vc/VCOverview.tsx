import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, Cell } from 'recharts'
import { api, type VCOverview as VCOverviewData } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

export default function VCOverview() {
  const { status, data, refetch } = useQuery<VCOverviewData>(
    () => api.get('/api/v1/dashboard/vc/overview'),
  )

  if (status === 'loading') return <Spinner />
  if (status === 'error')   return <ErrorBox message={(data as unknown as { message: string })?.message ?? 'Failed to load'} onRetry={refetch} />

  const d = (status === 'ok' ? data : null)!
  const eligChart = [
    { name: 'Eligible',   value: d.eligibility_summary.eligible,   fill: '#22c55e' },
    { name: 'Ineligible', value: d.eligibility_summary.ineligible,  fill: '#ef4444' },
    { name: 'Pending',    value: d.eligibility_summary.pending,      fill: '#f59e0b' },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h2 style={{ margin: 0 }}>VC Overview</h2>
        <div style={{ display: 'flex', gap: 8 }}>
          <button onClick={refetch} style={btnStyle}>Refresh</button>
          <a href={`${import.meta.env.VITE_API_URL ?? 'http://localhost:8443'}/api/v1/reports/vc/audit.pdf`}
            style={{ ...btnStyle, background: '#1e293b', color: '#fff', textDecoration: 'none', padding: '6px 14px', borderRadius: 6, fontSize: 13, display: 'inline-block' }}>
            Export PDF
          </a>
        </div>
      </div>

      {/* KPI Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(160px,1fr))', gap: 16, marginBottom: 32 }}>
        <KPI label="Sessions today"    value={d.total_sessions_today} />
        <KPI label="Students today"    value={d.total_students_today} />
        <KPI label="Avg attendance"    value={`${d.avg_attendance_pct}%`} />
        <KPI label="Ghost lectures"    value={d.ghost_lecture_count} alert={d.ghost_lecture_count > 0} />
      </div>

      {/* Eligibility chart */}
      <h3 style={{ marginBottom: 12 }}>Exam Eligibility Summary</h3>
      <ResponsiveContainer width="100%" height={200}>
        <BarChart data={eligChart} layout="vertical">
          <XAxis type="number" />
          <YAxis type="category" dataKey="name" width={80} />
          <Tooltip />
          <Bar dataKey="value" radius={[0, 4, 4, 0]}>
            {eligChart.map((e, i) => <Cell key={i} fill={e.fill} />)}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}

function KPI({ label, value, alert }: { label: string; value: number | string; alert?: boolean }) {
  return (
    <div style={{ background: '#fff', borderRadius: 10, padding: '16px 20px', boxShadow: '0 1px 4px rgba(0,0,0,.06)', border: alert ? '2px solid #ef4444' : '1px solid #e2e8f0' }}>
      <div style={{ fontSize: 28, fontWeight: 700, color: alert ? '#ef4444' : '#1e293b' }}>{value}</div>
      <div style={{ fontSize: 13, color: '#64748b', marginTop: 4 }}>{label}</div>
    </div>
  )
}

function Spinner() { return <p style={{ color: '#94a3b8' }}>Loading…</p> }
function ErrorBox({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div style={{ background: '#fef2f2', color: '#b91c1c', padding: 16, borderRadius: 8 }}>
      {message} <button onClick={onRetry} style={{ marginLeft: 8 }}>Retry</button>
    </div>
  )
}

const btnStyle: React.CSSProperties = {
  padding: '6px 14px', borderRadius: 6, border: '1px solid #e2e8f0',
  background: '#fff', cursor: 'pointer', fontSize: 13,
}
