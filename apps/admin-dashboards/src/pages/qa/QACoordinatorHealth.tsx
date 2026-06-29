import { api, type CoordinatorHealth } from '../../lib/api'
import { usePoll } from '../../lib/useApi'

export default function QACoordinatorHealth() {
  const { status, data } = usePoll<CoordinatorHealth[]>(
    () => api.get('/api/v1/dashboard/qa/coordinator-health'),
    30_000,
  )

  const records = status === 'ok' ? (data ?? []) : []

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <div>
          <h2 style={{ margin: 0 }}>Coordinator Health</h2>
          <p style={{ color: 'var(--muted)', margin: '4px 0 0', fontSize: 13 }}>Sync latency and manual-override ratio — last 30 days. Refreshes every 30s.</p>
        </div>
      </div>

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}
      {status === 'error'   && <p style={{ color: '#b91c1c' }}>Failed to load coordinator health data.</p>}

      {records.length === 0 && status === 'ok' && (
        <p style={{ color: 'var(--muted)', textAlign: 'center', marginTop: 48 }}>No coordinator data in the last 30 days.</p>
      )}

      <div style={{ display: 'grid', gap: 12 }}>
        {records.map(r => {
          const syncOk   = r.sync_success_rate >= 90
          const override = r.manual_override_ratio > 20
          return (
            <div key={r.coordinator_id} style={{
              background: '#fff', borderRadius: 10, padding: '16px 20px',
              border: override ? '2px solid #f59e0b' : '1px solid #e2e8f0',
              boxShadow: '0 1px 4px rgba(0,0,0,.04)',
            }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 12 }}>
                <div>
                  <div style={{ fontWeight: 700, fontSize: 15 }}>{r.coordinator_name}</div>
                  <div style={{ fontSize: 11, color: 'var(--muted)' }}>{r.coordinator_id}</div>
                </div>
                <div style={{ display: 'flex', gap: 8 }}>
                  <Pill label="Sessions" value={String(r.total_sessions)} />
                  {override && <Pill label="High Override" value={`${r.manual_override_ratio}%`} color="#f59e0b" />}
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3,1fr)', gap: 10 }}>
                <Metric label="Sync Success" value={`${r.sync_success_rate}%`} ok={syncOk} />
                <Metric label="Synced" value={String(r.synced_sessions)} ok />
                <Metric label="Failed" value={String(r.failed_sessions)} ok={r.failed_sessions === 0} />
                <Metric label="Pending" value={String(r.pending_sessions)} ok={r.pending_sessions < 3} />
                <Metric label="Manual Overrides" value={String(r.manual_overrides)} ok={!override} />
                <Metric label="Override Ratio" value={`${r.manual_override_ratio}%`} ok={!override} />
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function Metric({ label, value, ok }: { label: string; value: string; ok: boolean }) {
  return (
    <div style={{ background: '#f8fafc', borderRadius: 8, padding: '10px 12px' }}>
      <div style={{ fontSize: 18, fontWeight: 700, color: ok ? '#1e293b' : '#f59e0b' }}>{value}</div>
      <div style={{ fontSize: 11, color: 'var(--muted)', marginTop: 2 }}>{label}</div>
    </div>
  )
}

function Pill({ label, value, color = '#1e293b' }: { label: string; value: string; color?: string }) {
  return (
    <div style={{ background: '#f8fafc', borderRadius: 6, padding: '4px 10px', fontSize: 12, textAlign: 'center' }}>
      <div style={{ fontWeight: 700, color }}>{value}</div>
      <div style={{ color: 'var(--muted)', fontSize: 10 }}>{label}</div>
    </div>
  )
}
