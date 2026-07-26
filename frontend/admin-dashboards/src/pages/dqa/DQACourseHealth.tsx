import { api, type CourseHealthUnit } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

export default function DQACourseHealth() {
  const { status, data, refetch } = useQuery<{ units: CourseHealthUnit[]; total: number }>(
    () => api.get('/api/v1/dashboard/dqa/course-health'),
  )

  const units = status === 'ok' ? (data?.units ?? []) : []

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <div>
          <h2 style={{ margin: 0 }}>Course Health</h2>
          <p style={{ color: 'var(--muted)', margin: '4px 0 0', fontSize: 13 }}>Aggregate attendance compliance per course unit — sorted by lowest first.</p>
        </div>
        <button onClick={refetch} style={btn}>Refresh</button>
      </div>

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}
      {status === 'error'   && <p style={{ color: '#b91c1c' }}>Failed to load course health data.</p>}

      {/* Summary pills */}
      {units.length > 0 && (
        <div style={{ display: 'flex', gap: 12, marginBottom: 20 }}>
          {(['HEALTHY', 'AT_RISK', 'CRITICAL'] as const).map(level => {
            const count = units.filter(u => u.risk_level === level).length
            const colors = { HEALTHY: '#22c55e', AT_RISK: '#f59e0b', CRITICAL: '#ef4444' }
            const bgs    = { HEALTHY: '#f0fdf4', AT_RISK: '#fffbeb', CRITICAL: '#fef2f2' }
            return (
              <div key={level} style={{ background: bgs[level], borderRadius: 8, padding: '10px 16px', textAlign: 'center', minWidth: 80 }}>
                <div style={{ fontSize: 22, fontWeight: 700, color: colors[level] }}>{count}</div>
                <div style={{ fontSize: 11, color: 'var(--muted)', fontWeight: 600 }}>{level.replace('_', ' ')}</div>
              </div>
            )
          })}
        </div>
      )}

      {/* Heatmap grid */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(200px,1fr))', gap: 10 }}>
        {units.map(u => {
          const pct = u.avg_attendance_pct
          const bg  = pct >= u.threshold ? '#f0fdf4' : pct >= u.threshold * 0.85 ? '#fffbeb' : '#fef2f2'
          const accent = pct >= u.threshold ? '#22c55e' : pct >= u.threshold * 0.85 ? '#f59e0b' : '#ef4444'
          return (
            <div key={u.unit_id} style={{ background: bg, borderRadius: 10, padding: '14px 16px', border: `1px solid ${accent}22` }}>
              <div style={{ fontSize: 13, fontWeight: 700, marginBottom: 2 }}>{u.unit_name}</div>
              <div style={{ fontSize: 11, color: 'var(--muted)', marginBottom: 8 }}>{u.course_name}</div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <div style={{ fontSize: 28, fontWeight: 800, color: accent }}>{pct}%</div>
                <div style={{ fontSize: 11, color: 'var(--muted)' }}>
                  <div>{u.enrolled_count} students</div>
                  <div style={{ color: accent }}>{u.below_threshold_count} below {u.threshold}%</div>
                </div>
              </div>
              {/* Progress bar */}
              <div style={{ background: '#e2e8f0', borderRadius: 99, height: 6, marginTop: 8, overflow: 'hidden' }}>
                <div style={{ width: `${Math.min(pct, 100)}%`, height: '100%', borderRadius: 99, background: accent }} />
              </div>
            </div>
          )
        })}
      </div>

      {units.length === 0 && status === 'ok' && (
        <p style={{ color: 'var(--muted)', textAlign: 'center', marginTop: 48 }}>No course units found.</p>
      )}
    </div>
  )
}

const btn: React.CSSProperties = { padding: '7px 16px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
