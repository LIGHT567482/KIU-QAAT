import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

interface SummaryRow {
  lecturer_id:         string
  lecturer_name:       string
  department:          string
  email:               string
  total_sessions:      number
  total_contact_hours: number
  avg_contact_hours:   number
  last_session_date:   string
}

interface LogRow {
  log_id:         string
  lecturer_id:    string
  lecturer_name:  string
  department:     string
  unit_id:        string
  unit_name:      string
  session_date:   string
  gate_open_time: string
  gate_close_time: string
  contact_hours:  number
  session_status: string
}

export default function AdminLecturerAttendance() {
  const { tenantId } = useParams<{ tenantId: string }>()

  const { status: sumStatus, data: summary } = useQuery<SummaryRow[]>(
    () => api.get(`/api/v1/admin/tenants/${tenantId}/lecturer-attendance/summary`),
    [tenantId],
  )
  const { status: logStatus, data: logs } = useQuery<LogRow[]>(
    () => api.get(`/api/v1/admin/tenants/${tenantId}/lecturer-attendance`),
    [tenantId],
  )

  // Which lecturers' session logs are expanded inline (one row per lecturer; the
  // repetitive per-session rows only appear when you click "View logs").
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const toggle = (id: string) => setExpanded(prev => {
    const next = new Set(prev)
    next.has(id) ? next.delete(id) : next.add(id)
    return next
  })
  const logsFor = (id: string) => (logs ?? []).filter(l => l.lecturer_id === id)

  const [search, setSearch] = useState('')
  const sq = search.trim().toLowerCase()
  const visibleSummary = (summary ?? []).filter(s =>
    !sq || [s.lecturer_name, s.department, s.email].some(v => (v || '').toLowerCase().includes(sq)))

  const fmt = (iso: string) => iso ? new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : '—'
  const fmtDate = (d: string) => d ? new Date(d + 'T00:00:00').toLocaleDateString() : '—'

  function statusBadge(s: string) {
    const map: Record<string, { bg: string; color: string }> = {
      ACTIVE:          { bg: '#dcfce7', color: '#166534' },
      CLOSED:          { bg: '#eff6ff', color: '#1d4ed8' },
      AUTO_CLOSED:     { bg: '#fef9c3', color: '#854d0e' },
      PENDING_LECTURER:{ bg: '#fef2f2', color: '#b91c1c' },
    }
    const style = map[s] ?? { bg: '#f1f5f9', color: '#475569' }
    return (
      <span style={{ background: style.bg, color: style.color, padding: '2px 8px', borderRadius: 999, fontSize: 11, fontWeight: 600 }}>
        {s.replace('_', ' ')}
      </span>
    )
  }

  return (
    <div>
      <div style={{ marginBottom: 20 }}>
        <a href="/admin/tenants" style={{ color: 'var(--muted)', fontSize: 13, textDecoration: 'none' }}>← Tenants</a>
        <h2 style={{ margin: '4px 0 2px' }}>Lecturer Attendance</h2>
        <p style={{ color: 'var(--muted)', margin: 0, fontSize: 13 }}>Tenant: {tenantId}</p>
      </div>

      {sumStatus === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}

      <input value={search} onChange={e => setSearch(e.target.value)} placeholder="Search by lecturer name, department or email…"
        style={{ width: '100%', maxWidth: 420, padding: '8px 12px', borderRadius: 8, border: '1px solid #e2e8f0', fontSize: 14, marginBottom: 12, boxSizing: 'border-box' }} />

      {(summary ?? []).length === 0 && sumStatus === 'ok' && (
        <div style={{ padding: '20px 0', color: 'var(--muted)', fontSize: 14 }}>
          No attendance records yet. Records are created when a coordinator opens a session with a lecturer assigned.
        </div>
      )}

      {/* One row per lecturer; "View logs" expands that lecturer's sessions inline,
          so the same name is never repeated across many rows. */}
      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
        <thead>
          <tr style={{ background: '#f8fafc' }}>
            {['Lecturer', 'Sessions', 'Total hrs', 'Avg hrs', 'Last session', ''].map(h => (
              <th key={h} style={{ padding: '8px 12px', textAlign: 'left', borderBottom: '1px solid #e2e8f0', whiteSpace: 'nowrap', color: '#475569', fontWeight: 600 }}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {visibleSummary.map(s => (
            <>
              <tr key={s.lecturer_id} style={{ borderBottom: expanded.has(s.lecturer_id) ? 'none' : '1px solid #f1f5f9' }}>
                <td style={{ padding: '10px 12px' }}>
                  <div style={{ fontWeight: 700 }}>{s.lecturer_name}</div>
                  {s.department && <div style={{ fontSize: 11, color: 'var(--muted)' }}>{s.department}</div>}
                </td>
                <td style={{ padding: '10px 12px', fontWeight: 700 }}>{s.total_sessions}</td>
                <td style={{ padding: '10px 12px' }}>{Number(s.total_contact_hours).toFixed(1)}</td>
                <td style={{ padding: '10px 12px' }}>{Number(s.avg_contact_hours).toFixed(1)}</td>
                <td style={{ padding: '10px 12px', color: '#475569' }}>{s.last_session_date ? fmtDate(s.last_session_date) : '—'}</td>
                <td style={{ padding: '10px 12px' }}>
                  <button onClick={() => toggle(s.lecturer_id)} style={btnSmall}>
                    {expanded.has(s.lecturer_id) ? 'Hide logs' : 'View logs'}
                  </button>
                </td>
              </tr>
              {expanded.has(s.lecturer_id) && (
                <tr key={`${s.lecturer_id}-logs`}>
                  <td colSpan={6} style={{ padding: '0 12px 14px', background: '#f8fafc' }}>
                    <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12.5 }}>
                      <thead>
                        <tr>
                          {['Date', 'Unit', 'Gate Open', 'Gate Close', 'Contact Hrs', 'Status'].map(h => (
                            <th key={h} style={{ padding: '6px 10px', textAlign: 'left', color: 'var(--muted)', fontWeight: 600 }}>{h}</th>
                          ))}
                        </tr>
                      </thead>
                      <tbody>
                        {logsFor(s.lecturer_id).map(l => (
                          <tr key={l.log_id} style={{ borderTop: '1px solid #e8eef4' }}>
                            <td style={{ padding: '6px 10px', fontWeight: 600 }}>{fmtDate(l.session_date)}</td>
                            <td style={{ padding: '6px 10px' }}>{l.unit_name} <span style={{ color: 'var(--muted)', fontFamily: 'monospace', fontSize: 11 }}>{l.unit_id}</span></td>
                            <td style={{ padding: '6px 10px', color: '#475569' }}>{fmt(l.gate_open_time)}</td>
                            <td style={{ padding: '6px 10px', color: l.gate_close_time ? '#475569' : 'var(--muted)' }}>{l.gate_close_time ? fmt(l.gate_close_time) : 'In progress'}</td>
                            <td style={{ padding: '6px 10px' }}>{l.contact_hours > 0 ? `${Number(l.contact_hours).toFixed(2)} h` : '—'}</td>
                            <td style={{ padding: '6px 10px' }}>{statusBadge(l.session_status)}</td>
                          </tr>
                        ))}
                        {logsFor(s.lecturer_id).length === 0 && (
                          <tr><td colSpan={6} style={{ padding: 12, color: 'var(--muted)' }}>No session logs.</td></tr>
                        )}
                      </tbody>
                    </table>
                  </td>
                </tr>
              )}
            </>
          ))}
        </tbody>
      </table>
      {logStatus === 'loading' && <p style={{ color: 'var(--muted)', marginTop: 8 }}>Loading logs…</p>}
    </div>
  )
}

const btnSmall: React.CSSProperties = { padding: '4px 10px', background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 4, cursor: 'pointer', fontSize: 12 }
