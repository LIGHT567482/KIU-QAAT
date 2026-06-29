import { useState } from 'react'
import { api, type WorkloadRecord } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

const oneMonthAgo = new Date(Date.now() - 30 * 86400_000).toISOString().split('T')[0]
const today       = new Date().toISOString().split('T')[0]

export default function VCLecturerWorkload() {
  const [dateFrom, setDateFrom] = useState(oneMonthAgo)
  const [dateTo,   setDateTo]   = useState(today)
  const [applied,  setApplied]  = useState({ dateFrom, dateTo })

  const { status, data, refetch } = useQuery<{ date_from: string; date_to: string; workload: WorkloadRecord[] }>(
    () => api.get(`/api/v1/dashboard/vc/lecturer-workload?date_from=${applied.dateFrom}&date_to=${applied.dateTo}`),
    [applied],
  )

  const records = status === 'ok' ? (data?.workload ?? []) : []

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <h2 style={{ margin: 0 }}>Lecturer / Coordinator Workload</h2>
      </div>

      {/* Date filter */}
      <div style={{ display: 'flex', gap: 8, marginBottom: 20, alignItems: 'flex-end' }}>
        <label style={labelStyle}>
          From
          <input type="date" value={dateFrom} style={inp} onChange={e => setDateFrom(e.target.value)} />
        </label>
        <label style={labelStyle}>
          To
          <input type="date" value={dateTo} style={inp} onChange={e => setDateTo(e.target.value)} />
        </label>
        <button onClick={() => { setApplied({ dateFrom, dateTo }); refetch() }} style={btn}>Apply</button>
      </div>

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}
      {status === 'error'   && <p style={{ color: '#b91c1c' }}>Failed to load workload data.</p>}

      {records.length === 0 && status === 'ok' && (
        <p style={{ color: 'var(--muted)' }}>No session data found for this period.</p>
      )}

      {records.length > 0 && (
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
          <thead>
            <tr style={{ background: '#f8fafc' }}>
              {['Coordinator', 'Units Taught', 'Scheduled Sessions', 'Actual Sessions', 'Contact Hours', 'Delivery Rate'].map(h => (
                <th key={h} style={{ padding: '8px 12px', textAlign: 'left', borderBottom: '1px solid #e2e8f0', whiteSpace: 'nowrap' }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {records.map(r => {
              const rate = r.attendance_rate_pct
              return (
                <tr key={r.coordinator_id} style={{ borderBottom: '1px solid #f1f5f9' }}>
                  <td style={{ padding: '10px 12px', fontWeight: 600 }}>
                    {r.coordinator_name}
                    <div style={{ fontSize: 11, color: 'var(--muted)' }}>{r.coordinator_id}</div>
                  </td>
                  <td style={{ padding: '10px 12px', textAlign: 'center' }}>{r.distinct_units}</td>
                  <td style={{ padding: '10px 12px', textAlign: 'center' }}>{r.scheduled_sessions}</td>
                  <td style={{ padding: '10px 12px', textAlign: 'center' }}>{r.actual_sessions}</td>
                  <td style={{ padding: '10px 12px', textAlign: 'center' }}>{r.total_contact_hours_actual.toFixed(1)} h</td>
                  <td style={{ padding: '10px 12px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <div style={{ flex: 1, background: '#f1f5f9', borderRadius: 99, height: 8, overflow: 'hidden' }}>
                        <div style={{ width: `${Math.min(rate, 100)}%`, height: '100%', borderRadius: 99, background: rate >= 80 ? '#22c55e' : rate >= 60 ? '#f59e0b' : '#ef4444' }} />
                      </div>
                      <span style={{ fontSize: 13, fontWeight: 600, minWidth: 40 }}>{rate.toFixed(0)}%</span>
                    </div>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      )}
    </div>
  )
}

const labelStyle: React.CSSProperties = { display: 'flex', flexDirection: 'column', gap: 4, fontSize: 12, color: 'var(--muted)', fontWeight: 600 }
const inp: React.CSSProperties = { padding: '6px 10px', fontSize: 13, borderRadius: 6, border: '1px solid #e2e8f0' }
const btn: React.CSSProperties = { padding: '7px 16px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
