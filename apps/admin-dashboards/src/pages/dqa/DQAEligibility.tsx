import { useState } from 'react'
import { api, type EligibilityRecord } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

export default function DQAEligibility() {
  const [studentId, setStudentId] = useState('')
  const [query, setQuery] = useState('')

  const { status, data } = useQuery<EligibilityRecord>(
    () => api.get(`/api/v1/eligibility/${query}`),
    [query],
  )

  return (
    <div style={{ maxWidth: 640 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 4 }}>
        <h2 style={{ margin: 0 }}>Exam Eligibility</h2>
        <a href={`${import.meta.env.VITE_API_URL ?? 'http://localhost:8443'}/api/v1/reports/dqa/eligibility.csv`}
          style={{ padding: '6px 14px', borderRadius: 6, border: '1px solid #e2e8f0', background: '#fff', fontSize: 13, textDecoration: 'none', color: '#1e293b' }}>
          Export CSV
        </a>
      </div>
      <p style={{ color: '#64748b', marginBottom: 20 }}>Look up a student's attendance and eligibility status.</p>

      <div style={{ display: 'flex', gap: 8, marginBottom: 24 }}>
        <input
          value={studentId}
          onChange={e => setStudentId(e.target.value)}
          placeholder="Student registration number"
          style={{ flex: 1, padding: '9px 12px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 15 }}
          onKeyDown={e => e.key === 'Enter' && setQuery(studentId)}
        />
        <button
          onClick={() => setQuery(studentId)}
          style={{ padding: '9px 18px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600 }}
        >
          Look up
        </button>
      </div>

      {query && status === 'loading' && <p style={{ color: '#94a3b8' }}>Loading…</p>}
      {query && status === 'error'   && <p style={{ color: '#b91c1c' }}>Student not found.</p>}

      {status === 'ok' && data && (
        <div>
          <div style={{ marginBottom: 16 }}>
            <strong>{data.student_id}</strong>
            <span style={{ color: '#94a3b8', marginLeft: 8 }}>AY {data.academic_year} · Sem {data.semester}</span>
          </div>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
            <thead>
              <tr style={{ background: '#f8fafc', textAlign: 'left' }}>
                {['Unit', 'Held', 'Attended', '%', 'Threshold', 'Status'].map(h => (
                  <th key={h} style={{ padding: '8px 12px', borderBottom: '1px solid #e2e8f0' }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {data.units.map(u => (
                <tr key={u.unit_id} style={{ borderBottom: '1px solid #f1f5f9' }}>
                  <td style={{ padding: '8px 12px' }}><div style={{ fontWeight: 600 }}>{u.unit_name}</div><div style={{ fontSize: 12, color: '#94a3b8' }}>{u.unit_id}</div></td>
                  <td style={{ padding: '8px 12px' }}>{u.sessions_held}</td>
                  <td style={{ padding: '8px 12px' }}>{u.sessions_attended}</td>
                  <td style={{ padding: '8px 12px', fontWeight: 600 }}>{u.attendance_percentage}%</td>
                  <td style={{ padding: '8px 12px', color: '#94a3b8' }}>{u.threshold}%</td>
                  <td style={{ padding: '8px 12px' }}>
                    <StatusPill status={u.status} deficit={u.deficit_sessions} />
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

function StatusPill({ status, deficit }: { status: string; deficit?: number }) {
  const ok = status === 'ELIGIBLE'
  return (
    <span style={{
      display: 'inline-block', padding: '2px 10px', borderRadius: 999, fontSize: 12, fontWeight: 600,
      background: ok ? '#f0fdf4' : '#fef2f2',
      color:      ok ? '#166534' : '#b91c1c',
    }}>
      {ok ? 'Eligible' : `Ineligible${deficit ? ` (−${deficit})` : ''}`}
    </span>
  )
}
