import { useState } from 'react'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

// Lecturer attendance for the oversight dashboards (QA / VC / DQA). One row per
// lecturer with an inline "View logs" expander — same shape as the admin page but
// fed by the caller-tenant endpoints.

interface SummaryRow {
  lecturer_id: string; lecturer_name: string; department: string; email: string
  total_sessions: number; total_contact_hours: number; avg_contact_hours: number; last_session_date: string
}
interface LogRow {
  log_id: string; lecturer_id: string; unit_id: string; unit_name: string
  session_date: string; gate_open_time: string; gate_close_time: string; contact_hours: number; session_status: string
}

export default function DashLecturerAttendance() {
  const { data: summary, status } = useQuery<SummaryRow[]>(() => api.get('/api/v1/dashboard/lecturer-attendance/summary'))
  const { data: logs } = useQuery<LogRow[]>(() => api.get('/api/v1/dashboard/lecturer-attendance'))
  const [open, setOpen] = useState<Set<string>>(new Set())
  const toggle = (id: string) => setOpen(p => { const n = new Set(p); n.has(id) ? n.delete(id) : n.add(id); return n })
  const logsFor = (id: string) => (logs ?? []).filter(l => l.lecturer_id === id)
  const [search, setSearch] = useState('')
  const sq = search.trim().toLowerCase()
  const visibleSummary = (summary ?? []).filter(s =>
    !sq || [s.lecturer_name, s.department, s.email].some(v => (v || '').toLowerCase().includes(sq)))
  const fmt = (iso: string) => iso ? new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : '—'
  const fmtDate = (d: string) => d ? new Date(d + 'T00:00:00').toLocaleDateString() : '—'

  return (
    <div style={{ color: 'var(--text)' }}>
      <h2 style={{ margin: '0 0 4px' }}>Lecturer Attendance</h2>
      <p style={{ color: 'var(--muted)', margin: '0 0 16px', fontSize: 13 }}>Proof-of-presence — one row per lecturer; click “View logs” for each session.</p>
      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}
      {status === 'ok' && (summary ?? []).length === 0 && <p style={{ color: 'var(--muted)' }}>No lecturer attendance recorded yet.</p>}
      <input value={search} onChange={e => setSearch(e.target.value)} placeholder="Search by lecturer name, department or email…"
        style={{ width: '100%', maxWidth: 420, padding: '8px 12px', borderRadius: 8, border: '1px solid var(--border,#e2e8f0)', fontSize: 14, marginBottom: 12, boxSizing: 'border-box' }} />
      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
        <thead><tr style={{ background: 'var(--surface,#f8fafc)' }}>
          {['Lecturer', 'Sessions', 'Total hrs', 'Avg hrs', 'Last session', ''].map(h =>
            <th key={h} style={{ padding: '8px 12px', textAlign: 'left', borderBottom: '1px solid var(--border,#e2e8f0)', whiteSpace: 'nowrap' }}>{h}</th>)}
        </tr></thead>
        <tbody>
          {visibleSummary.map(s => (
            <>
              <tr key={s.lecturer_id} style={{ borderBottom: open.has(s.lecturer_id) ? 'none' : '1px solid var(--border,#f1f5f9)' }}>
                <td style={{ padding: '10px 12px' }}><div style={{ fontWeight: 700 }}>{s.lecturer_name}</div>{s.department && <div style={{ fontSize: 11, color: 'var(--muted)' }}>{s.department}</div>}</td>
                <td style={{ padding: '10px 12px', fontWeight: 700 }}>{s.total_sessions}</td>
                <td style={{ padding: '10px 12px' }}>{Number(s.total_contact_hours).toFixed(1)}</td>
                <td style={{ padding: '10px 12px' }}>{Number(s.avg_contact_hours).toFixed(1)}</td>
                <td style={{ padding: '10px 12px', color: 'var(--muted)' }}>{s.last_session_date ? fmtDate(s.last_session_date) : '—'}</td>
                <td style={{ padding: '10px 12px' }}><button onClick={() => toggle(s.lecturer_id)} style={btn}>{open.has(s.lecturer_id) ? 'Hide logs' : 'View logs'}</button></td>
              </tr>
              {open.has(s.lecturer_id) && (
                <tr key={`${s.lecturer_id}-l`}><td colSpan={6} style={{ padding: '0 12px 14px', background: 'var(--surface,#f8fafc)' }}>
                  <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12.5 }}>
                    <thead><tr>{['Date', 'Unit', 'Open', 'Close', 'Contact Hrs', 'Status'].map(h => <th key={h} style={{ padding: '6px 10px', textAlign: 'left', color: 'var(--muted)' }}>{h}</th>)}</tr></thead>
                    <tbody>
                      {logsFor(s.lecturer_id).map(l => (
                        <tr key={l.log_id} style={{ borderTop: '1px solid var(--border,#e8eef4)' }}>
                          <td style={{ padding: '6px 10px', fontWeight: 600 }}>{fmtDate(l.session_date)}</td>
                          <td style={{ padding: '6px 10px' }}>{l.unit_name} <span style={{ color: 'var(--muted)', fontFamily: 'monospace', fontSize: 11 }}>{l.unit_id}</span></td>
                          <td style={{ padding: '6px 10px' }}>{fmt(l.gate_open_time)}</td>
                          <td style={{ padding: '6px 10px' }}>{l.gate_close_time ? fmt(l.gate_close_time) : 'In progress'}</td>
                          <td style={{ padding: '6px 10px' }}>{l.contact_hours > 0 ? `${Number(l.contact_hours).toFixed(2)} h` : '—'}</td>
                          <td style={{ padding: '6px 10px' }}>{l.session_status.replace('_', ' ')}</td>
                        </tr>
                      ))}
                      {logsFor(s.lecturer_id).length === 0 && <tr><td colSpan={6} style={{ padding: 12, color: 'var(--muted)' }}>No session logs.</td></tr>}
                    </tbody>
                  </table>
                </td></tr>
              )}
            </>
          ))}
        </tbody>
      </table>
    </div>
  )
}
const btn: React.CSSProperties = { padding: '4px 10px', background: 'var(--surface,#fff)', border: '1px solid var(--border,#e2e8f0)', borderRadius: 4, cursor: 'pointer', fontSize: 12, color: 'inherit' }
