import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'
import ExportButtons from '../../components/ExportButtons'
import RecordTabs from '../../components/RecordTabs'
import CompensationTag from '../../components/CompensationTag'
import ProvisionTag from '../../components/ProvisionTag'
import MonitorLecturerAttendance from '../shared/MonitorLecturerAttendance'

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
  /** How many students were marked present in that session, counted from the check-in ledger. */
  students_attended: number
  /** The cohort that sat in it, as YEAR:SEMESTER — "2:1". */
  class_group: string
  /** The QA monitor who independently witnessed the same lecture, if one did. */
  qa_monitor: string
  qa_monitor_staff_id: string
  /** Recorded by that monitor as making good an earlier lecture. */
  is_compensation: boolean
  compensation_for: string
  /** The room it was actually taught in, and whether that was the timetabled one. */
  room: string
  room_is_provision: boolean
  provision_note: string
}

// Two independent records of the same lectures, on two pages of one feature — the coordinator's
// session log and the QA monitor's spot-check. Kept apart so a disagreement stays visible.
export default function AdminLecturerAttendance() {
  return (
    <RecordTabs title="Lecturer Attendance" tabs={[
      { id: 'coordinator', label: 'Coordinator record', render: () => <CoordinatorRecord /> },
      { id: 'monitor',      label: 'QA monitor record',  render: () => <MonitorLecturerAttendance /> },
    ]} />
  )
}

function CoordinatorRecord() {
  const { tenantId } = useParams<{ tenantId: string }>()

  const { status: sumStatus, data: summary } = useQuery<SummaryRow[]>(
    () => api.get(`/api/v1/admin/lecturer-attendance/summary`),
    [tenantId],
  )
  const { status: logStatus, data: logs } = useQuery<LogRow[]>(
    () => api.get(`/api/v1/admin/lecturer-attendance`),
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
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 12, flexWrap: 'wrap', marginBottom: 14 }}>
        <p style={{ color: 'var(--muted)', margin: 0, fontSize: 13, maxWidth: 620 }}>
          Proof-of-presence from the session itself — the lecturer's own start/end and the contact
          hours between them.
        </p>
        <ExportButtons base="/api/v1/dashboard/lecturer-attendance/export" filename="lecturer-attendance"
          disabled={(summary ?? []).length === 0} />
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
                          {['Date', 'Unit', 'Class/Group', 'Room', 'Students', 'Gate Open', 'Gate Close', 'Contact Hrs', 'QA Monitor', 'Status'].map(h => (
                            <th key={h} style={{ padding: '6px 10px', textAlign: 'left', color: 'var(--muted)', fontWeight: 600 }}>{h}</th>
                          ))}
                        </tr>
                      </thead>
                      <tbody>
                        {logsFor(s.lecturer_id).map(l => (
                          <tr key={l.log_id} style={{ borderTop: '1px solid #e8eef4' }}>
                            <td style={{ padding: '6px 10px', fontWeight: 600 }}>{fmtDate(l.session_date)}</td>
                            <td style={{ padding: '6px 10px' }}>
                              {l.unit_name} <span style={{ color: 'var(--muted)', fontFamily: 'monospace', fontSize: 11 }}>{l.unit_id}</span>
                              {l.is_compensation && <CompensationTag forDate={l.compensation_for} />}
                            </td>
                            <td style={{ padding: '6px 10px', fontFamily: 'monospace' }}>{l.class_group || '—'}</td>
                            <td style={{ padding: '6px 10px' }}>
                              {l.room || '—'}
                              {l.room_is_provision && <ProvisionTag note={l.provision_note} />}
                            </td>
                            {/* How many actually came. A lecture the lecturer started and nobody
                                attended is a very different event from a full room, and the log
                                could not tell them apart. */}
                            <td style={{ padding: '6px 10px', fontWeight: 600 }}>{l.students_attended}</td>
                            <td style={{ padding: '6px 10px', color: '#475569' }}>{fmt(l.gate_open_time)}</td>
                            <td style={{ padding: '6px 10px', color: l.gate_close_time ? '#475569' : 'var(--muted)' }}>{l.gate_close_time ? fmt(l.gate_close_time) : 'In progress'}</td>
                            <td style={{ padding: '6px 10px' }}>{l.contact_hours > 0 ? `${Number(l.contact_hours).toFixed(2)} h` : '—'}</td>
                            <td style={{ padding: '6px 10px' }}>
                              {l.qa_monitor
                                ? <>{l.qa_monitor}{l.qa_monitor_staff_id && <span style={{ color: 'var(--muted)', fontFamily: 'monospace', fontSize: 11 }}> {l.qa_monitor_staff_id}</span>}</>
                                : <span style={{ color: 'var(--muted)' }}>Not visited</span>}
                            </td>
                            <td style={{ padding: '6px 10px' }}>{statusBadge(l.session_status)}</td>
                          </tr>
                        ))}
                        {logsFor(s.lecturer_id).length === 0 && (
                          <tr><td colSpan={10} style={{ padding: 12, color: 'var(--muted)' }}>No session logs.</td></tr>
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
