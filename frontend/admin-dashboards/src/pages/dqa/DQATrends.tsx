import {
  LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip,
  ResponsiveContainer, Legend,
} from 'recharts'
import { api, type TrendPoint } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

export default function DQATrends() {
  const { status, data, refetch } = useQuery<{ trend: TrendPoint[] }>(
    () => api.get('/api/v1/dashboard/dqa/trends'),
  )

  const trend = status === 'ok' ? (data?.trend ?? []) : []

  const formatted = trend.map(p => ({
    ...p,
    week: new Date(p.week_start).toLocaleDateString('en-GB', { day: '2-digit', month: 'short' }),
  }))

  // Week-over-week change for the most recent two weeks.
  const wow: string | null = (() => {
    if (formatted.length < 2) return null
    const curr = formatted[formatted.length - 1].unique_students
    const prev = formatted[formatted.length - 2].unique_students
    if (prev === 0) return null
    const change = ((curr - prev) / prev * 100).toFixed(1)
    return `${change.startsWith('-') ? '' : '+'}${change}%`
  })()

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <div>
          <h2 style={{ margin: 0 }}>Attendance Trends</h2>
          <p style={{ color: 'var(--muted)', margin: '4px 0 0', fontSize: 13 }}>
            Week-over-week attendance for the last 12 weeks.
            {wow && (
              <span style={{ marginLeft: 8, fontWeight: 700, color: wow.startsWith('+') ? '#22c55e' : '#ef4444' }}>
                WoW: {wow}
              </span>
            )}
          </p>
        </div>
        <button onClick={refetch} style={btn}>Refresh</button>
      </div>

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}
      {status === 'error'   && <p style={{ color: '#b91c1c' }}>Failed to load trend data.</p>}

      {formatted.length === 0 && status === 'ok' && (
        <p style={{ color: 'var(--muted)', textAlign: 'center', marginTop: 48 }}>No session data in the last 12 weeks.</p>
      )}

      {formatted.length > 0 && (
        <>
          {/* Summary cards */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(140px,1fr))', gap: 12, marginBottom: 24 }}>
            <SummaryCard label="Total sessions (12w)" value={trend.reduce((s, p) => s + p.sessions_held, 0)} />
            <SummaryCard label="Total check-ins (12w)" value={trend.reduce((s, p) => s + p.total_checkins, 0)} />
            <SummaryCard label="Unique students (12w)" value={trend.reduce((s, p) => s + p.unique_students, 0)} />
          </div>

          <h3 style={{ marginBottom: 12 }}>Unique Students per Week</h3>
          <ResponsiveContainer width="100%" height={220}>
            <LineChart data={formatted} margin={{ top: 4, right: 16, left: 0, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
              <XAxis dataKey="week" tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} />
              <Tooltip />
              <Legend />
              <Line type="monotone" dataKey="unique_students" name="Unique Students" stroke="#1e293b" strokeWidth={2} dot={{ r: 3 }} />
              <Line type="monotone" dataKey="total_checkins"  name="Total Check-ins"  stroke="#3b82f6" strokeWidth={2} dot={{ r: 3 }} strokeDasharray="4 2" />
            </LineChart>
          </ResponsiveContainer>

          <h3 style={{ marginTop: 28, marginBottom: 12 }}>Sessions Held per Week</h3>
          <ResponsiveContainer width="100%" height={160}>
            <LineChart data={formatted} margin={{ top: 4, right: 16, left: 0, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
              <XAxis dataKey="week" tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} />
              <Tooltip />
              <Line type="monotone" dataKey="sessions_held" name="Sessions Held" stroke="#22c55e" strokeWidth={2} dot={{ r: 3 }} />
            </LineChart>
          </ResponsiveContainer>
        </>
      )}
    </div>
  )
}

function SummaryCard({ label, value }: { label: string; value: number }) {
  return (
    <div style={{ background: '#fff', borderRadius: 10, padding: '14px 18px', border: '1px solid #e2e8f0', boxShadow: '0 1px 3px rgba(0,0,0,.04)' }}>
      <div style={{ fontSize: 24, fontWeight: 700, color: '#1e293b' }}>{value.toLocaleString()}</div>
      <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 4 }}>{label}</div>
    </div>
  )
}

const btn: React.CSSProperties = { padding: '7px 16px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
