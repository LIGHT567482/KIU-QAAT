import { api, type LiveSession } from '../../lib/api'
import { usePoll } from '../../lib/useApi'

export default function QALiveSessions() {
  const { status, data } = usePoll<LiveSession[]>(
    () => api.get('/api/v1/dashboard/qa/live-sessions'),
    10_000,  // poll every 10s
  )

  const sessions = (status === 'ok' ? data : []) ?? []

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <h2 style={{ margin: 0 }}>Live Sessions</h2>
        <span style={{ fontSize: 12, color: 'var(--muted)' }}>Auto-refreshes every 10s</span>
      </div>

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}
      {status === 'error'   && <p style={{ color: '#b91c1c' }}>Failed to load sessions.</p>}

      {sessions.length === 0 && status === 'ok' && (
        <div style={{ textAlign: 'center', padding: 48, color: 'var(--muted)' }}>
          No active sessions right now.
        </div>
      )}

      <div style={{ display: 'grid', gap: 12 }}>
        {sessions.map(s => (
          <SessionCard key={s.session_id} session={s} />
        ))}
      </div>
    </div>
  )
}

function SessionCard({ session: s }: { session: LiveSession }) {
  const hasAnomalies = s.audit_flags.length > 0

  return (
    <div style={{
      background: '#fff', borderRadius: 10, padding: '14px 18px',
      border: hasAnomalies ? '2px solid #f59e0b' : '1px solid #e2e8f0',
      boxShadow: '0 1px 4px rgba(0,0,0,.04)',
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <div>
          <div style={{ fontWeight: 700, fontSize: 15 }}>{s.unit_name}</div>
          <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 2 }}>
            {s.unit_id} · {s.venue_id} · Coordinator {s.coordinator_id}
          </div>
        </div>
        <div style={{ textAlign: 'right' }}>
          <div style={{ fontSize: 26, fontWeight: 700, color: '#1e293b' }}>{s.student_count}</div>
          <div style={{ fontSize: 11, color: 'var(--muted)' }}>students</div>
        </div>
      </div>

      {hasAnomalies && (
        <div style={{ marginTop: 10, display: 'flex', gap: 6, flexWrap: 'wrap' }}>
          {s.audit_flags.map(f => (
            <span key={f} style={{ background: '#fffbeb', color: '#92400e', fontSize: 11, padding: '2px 8px', borderRadius: 999, fontWeight: 600 }}>
              ⚠ {f.replace(/_/g, ' ')}
            </span>
          ))}
        </div>
      )}

      <div style={{ marginTop: 8, fontSize: 12, color: 'var(--muted)' }}>
        Gate open: {new Date(s.gate_open_time).toLocaleTimeString()}
      </div>
    </div>
  )
}
