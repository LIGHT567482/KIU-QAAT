import { useState } from 'react'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

// QA reports: (1) cross-dimension lecturer-teaching report from patrol data (filterable), and
// (2) today's employee biometric no-shows with a one-click email + WhatsApp notify.

interface TeachRow { lecturer_id: string; full_name: string; department: string; school: string; taught: number; patrolled: number }
interface TeachResp { rows: TeachRow[]; total_taught: number; total_patrolled: number }
interface NoShow { staff_id: string; name: string; email: string; phone: string }
interface NoShowResp { date: string; no_shows: NoShow[] }

export default function QAReports() {
  const [f, setF] = useState({ from: '', to: '', school: '', department: '', lecturer: '', status: 'all' })
  const qs = new URLSearchParams(Object.entries(f).filter(([, v]) => v && v !== 'all')).toString()
  const rep = useQuery<TeachResp>(() => api.get(`/api/v1/reports/lecturer-teaching${qs ? '?' + qs : ''}`), [qs])
  const noshow = useQuery<NoShowResp>(() => api.get('/api/v1/qa/employee-no-shows'), [])
  const rows = rep.data?.rows ?? []
  const [notifyMsg, setNotifyMsg] = useState<string | null>(null)
  const [notifying, setNotifying] = useState(false)

  async function notifyNoShows() {
    setNotifying(true); setNotifyMsg(null)
    try {
      const res = await api.post<{ notified: number }>('/api/v1/qa/notify-no-shows', {})
      setNotifyMsg(`Notified ${res.notified} no-show(s) by email + WhatsApp.`)
    } catch (e) { setNotifyMsg(e instanceof Error ? e.message : 'Failed') }
    finally { setNotifying(false) }
  }

  return (
    <div>
      <h2 style={{ margin: '0 0 16px' }}>QA Reports</h2>

      {/* ── Lecturer teaching report ─────────────────────────────────────── */}
      <h3 style={{ margin: '0 0 8px' }}>Lecturer teaching (from patrols)</h3>
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 12 }}>
        <Field label="From"><input type="date" value={f.from} onChange={e => setF({ ...f, from: e.target.value })} style={inp} /></Field>
        <Field label="To"><input type="date" value={f.to} onChange={e => setF({ ...f, to: e.target.value })} style={inp} /></Field>
        <Field label="School"><input value={f.school} onChange={e => setF({ ...f, school: e.target.value })} style={inp} placeholder="any" /></Field>
        <Field label="Department"><input value={f.department} onChange={e => setF({ ...f, department: e.target.value })} style={inp} placeholder="any" /></Field>
        <Field label="Lecturer (staff id)"><input value={f.lecturer} onChange={e => setF({ ...f, lecturer: e.target.value })} style={inp} placeholder="any" /></Field>
        <Field label="Status">
          <select value={f.status} onChange={e => setF({ ...f, status: e.target.value })} style={inp}>
            <option value="all">All</option><option value="taught">Taught</option><option value="not_taught">Not taught</option>
          </select>
        </Field>
      </div>
      {rep.status === 'loading' ? <div style={muted}>Loading…</div> : rows.length === 0 ? <div style={muted}>No patrol data for these filters.</div> : (
        <>
          <div style={{ ...muted, marginBottom: 6 }}>Overall: {rep.data?.total_taught}/{rep.data?.total_patrolled} patrols found teaching.</div>
          <table style={table}>
            <thead><tr style={htr}><th style={th}>Lecturer</th><th style={th}>Staff ID</th><th style={th}>Department</th><th style={th}>School</th><th style={th}>Taught / Patrolled</th></tr></thead>
            <tbody>
              {rows.map(l => {
                const pct = l.patrolled ? Math.round((l.taught / l.patrolled) * 100) : null
                return (
                  <tr key={l.lecturer_id} style={tr}>
                    <td style={td}><b>{l.full_name || l.lecturer_id}</b></td>
                    <td style={td}>{l.lecturer_id}</td>
                    <td style={td}>{l.department || '—'}</td>
                    <td style={td}>{l.school || '—'}</td>
                    <td style={td}>{l.taught}/{l.patrolled}{pct !== null && <span style={{ marginLeft: 8, fontWeight: 700, color: pct >= 75 ? '#15803d' : '#b91c1c' }}>{pct}%</span>}</td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </>
      )}

      {/* ── Employee biometric no-shows ──────────────────────────────────── */}
      <h3 style={{ margin: '28px 0 8px' }}>Employee no-shows today (no check-in by 09:30)</h3>
      {notifyMsg && <div style={{ ...muted, color: '#15803d', marginBottom: 8 }}>{notifyMsg}</div>}
      <button style={btn} disabled={notifying} onClick={notifyNoShows}>{notifying ? 'Sending…' : '✉ Notify no-shows (email + WhatsApp)'}</button>
      <div style={{ marginTop: 12 }}>
        {noshow.status === 'loading' ? <div style={muted}>Loading…</div> : (noshow.data?.no_shows ?? []).length === 0 ? <div style={muted}>Everyone checked in. 🎉</div> : (
          <table style={table}>
            <thead><tr style={htr}><th style={th}>Name</th><th style={th}>Staff ID</th><th style={th}>Email</th><th style={th}>Phone</th></tr></thead>
            <tbody>
              {(noshow.data?.no_shows ?? []).map(n => (
                <tr key={n.staff_id} style={tr}><td style={td}><b>{n.name}</b></td><td style={td}>{n.staff_id}</td><td style={td}>{n.email || '—'}</td><td style={td}>{n.phone || '—'}</td></tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <label style={{ fontSize: 12, color: '#475569' }}><div style={{ marginBottom: 2 }}>{label}</div>{children}</label>
}
const inp: React.CSSProperties = { padding: '7px 9px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14 }
const table: React.CSSProperties = { width: '100%', borderCollapse: 'collapse', fontSize: 14 }
const htr: React.CSSProperties = { textAlign: 'left', color: '#64748b', fontSize: 12 }
const th: React.CSSProperties = { padding: '6px 10px' }
const tr: React.CSSProperties = { borderTop: '1px solid #e2e8f0' }
const td: React.CSSProperties = { padding: '10px' }
const muted: React.CSSProperties = { color: '#64748b', fontSize: 13 }
const btn: React.CSSProperties = { padding: '8px 14px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
