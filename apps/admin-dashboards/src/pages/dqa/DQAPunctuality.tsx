import { api, type PunctualityRecord } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

export default function DQAPunctuality() {
  const { status, data, refetch } = useQuery<{ period: string; records: PunctualityRecord[] }>(
    () => api.get('/api/v1/dashboard/dqa/punctuality'),
  )

  const records = status === 'ok' ? (data?.records ?? []) : []
  const period  = status === 'ok' ? data?.period : ''

  const totalLate = records.reduce((s, r) => s + r.late_open_count, 0)
  const totalNoOpen = records.reduce((s, r) => s + r.no_gate_open_count, 0)

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <div>
          <h2 style={{ margin: 0 }}>Lecturer Punctuality</h2>
          <p style={{ color: 'var(--muted)', margin: '4px 0 0', fontSize: 13 }}>
            Gate-open wait time vs scheduled session start — {period}.
          </p>
        </div>
        <button onClick={refetch} style={btn}>Refresh</button>
      </div>

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}
      {status === 'error'   && <p style={{ color: '#b91c1c' }}>Failed to load punctuality data.</p>}

      {records.length > 0 && (
        <>
          {/* Summary */}
          <div style={{ display: 'flex', gap: 12, marginBottom: 20 }}>
            <SummaryCard label="Late opens (>15 min)" value={totalLate} color="#f59e0b" />
            <SummaryCard label="No gate-open recorded" value={totalNoOpen} color="#ef4444" />
          </div>

          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
            <thead>
              <tr style={{ background: '#f8fafc' }}>
                {['Coordinator', 'Total Sessions', 'Gate Opened', 'Never Opened', 'Avg Wait (min)', 'Late Opens (>15min)'].map(h => (
                  <th key={h} style={{ padding: '8px 12px', textAlign: 'left', borderBottom: '1px solid #e2e8f0', whiteSpace: 'nowrap' }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {records.map(r => (
                <tr key={r.coordinator_id} style={{ borderBottom: '1px solid #f1f5f9' }}>
                  <td style={{ padding: '10px 12px', fontWeight: 600 }}>
                    {r.coordinator_name}
                    <div style={{ fontSize: 11, color: 'var(--muted)' }}>{r.coordinator_id}</div>
                  </td>
                  <td style={{ padding: '10px 12px', textAlign: 'center' }}>{r.total_sessions}</td>
                  <td style={{ padding: '10px 12px', textAlign: 'center' }}>{r.gate_opened_count}</td>
                  <td style={{ padding: '10px 12px', textAlign: 'center' }}>
                    <span style={{ color: r.no_gate_open_count > 0 ? '#ef4444' : 'var(--muted)', fontWeight: r.no_gate_open_count > 0 ? 700 : 400 }}>
                      {r.no_gate_open_count}
                    </span>
                  </td>
                  <td style={{ padding: '10px 12px', textAlign: 'center' }}>
                    <span style={{ color: r.avg_wait_minutes > 15 ? '#f59e0b' : '#22c55e', fontWeight: 600 }}>
                      {r.avg_wait_minutes} min
                    </span>
                  </td>
                  <td style={{ padding: '10px 12px', textAlign: 'center' }}>
                    {r.late_open_count > 0 ? (
                      <span style={{ background: '#fffbeb', color: '#92400e', padding: '2px 8px', borderRadius: 999, fontSize: 12, fontWeight: 600 }}>
                        ⚠ {r.late_open_count}
                      </span>
                    ) : (
                      <span style={{ color: '#22c55e', fontWeight: 600 }}>✓ 0</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}

      {records.length === 0 && status === 'ok' && (
        <p style={{ color: 'var(--muted)', textAlign: 'center', marginTop: 48 }}>No session data in the last 30 days.</p>
      )}
    </div>
  )
}

function SummaryCard({ label, value, color }: { label: string; value: number; color: string }) {
  return (
    <div style={{ background: '#fff', borderRadius: 10, padding: '12px 18px', border: `1px solid ${color}44`, display: 'flex', gap: 12, alignItems: 'center' }}>
      <div style={{ fontSize: 28, fontWeight: 700, color }}>{value}</div>
      <div style={{ fontSize: 12, color: 'var(--muted)' }}>{label}</div>
    </div>
  )
}

const btn: React.CSSProperties = { padding: '7px 16px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
