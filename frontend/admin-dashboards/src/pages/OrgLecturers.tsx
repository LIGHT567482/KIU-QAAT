import { useState } from 'react'
import { api } from '../lib/api'
import { useQuery } from '../lib/useApi'

// Shared HOD/Dean dashboard: lists the lecturers in the viewer's department (HOD) or school (Dean)
// with their teaching progress (patrolled sessions found TEACHING vs total), and lets them notify
// lecturers (bulk or one) and message the DQA/Admin — all via the existing app-notifications system.

interface LecturerRow {
  staff_id: string; full_name: string; department?: string; school?: string
  taught_count: number; patrolled_count: number; unit_count: number
}
interface Resp { scope: { department: string; school: string }; lecturers: LecturerRow[]; message?: string }

export default function OrgLecturers({ level }: { level: 'hod' | 'dean' }) {
  const { status, data } = useQuery<Resp>(() => api.get(`/api/v1/${level}/lecturers`), [level])
  const all = data?.lecturers ?? []
  // A dean's list spans every department in the college, so it needs a way to narrow to one —
  // otherwise the only way to answer "how is Computer Science doing" is to read the whole college.
  // Pre-set from ?department= so the Departments page can link straight into a department.
  const [dept, setDept] = useState(
    () => new URLSearchParams(window.location.search).get('department') ?? '')
  const departments = Array.from(new Set(all.map(l => l.department).filter(Boolean) as string[])).sort()
  const rows = dept ? all.filter(l => l.department === dept) : all
  const scopeLabel = level === 'hod' ? (data?.scope.department || 'your department') : (data?.scope.school || 'your school')

  const [compose, setCompose] = useState<null | { audience: string; target?: string; who: string }>(null)

  return (
    <div>
      <h2 style={{ margin: '0 0 4px' }}>{level === 'hod' ? 'Department' : 'School'} — Lecturer Oversight</h2>
      <p style={{ color: 'var(--muted)', margin: '0 0 16px', fontSize: 13 }}>
        Scope: <b>{scopeLabel}</b>. Progress = patrolled sessions the lecturer was found teaching, this term.
      </p>

      <div style={{ display: 'flex', gap: 8, marginBottom: 16, flexWrap: 'wrap' }}>
        <button style={btn} onClick={() => setCompose({ audience: 'LECTURERS', who: `all lecturers in ${scopeLabel}` })}>✉ Notify all lecturers</button>
        {level === 'dean' && departments.length > 1 && (
          <select value={dept} onChange={e => setDept(e.target.value)} style={selectSm}>
            <option value="">All departments</option>
            {departments.map(d => <option key={d} value={d}>{d}</option>)}
          </select>
        )}
        {/* Upward and downward, so the chain is a channel in both directions. */}
        {level === 'dean' && (
          <button style={btnGhost} onClick={() => setCompose({ audience: 'HODS', who: 'every head of department in your college' })}>Message HODs</button>
        )}
        {level === 'hod' && (
          <button style={btnGhost} onClick={() => setCompose({ audience: 'DEAN', who: 'your dean' })}>Message Dean</button>
        )}
        <button style={btnGhost} onClick={() => setCompose({ audience: 'DQA', who: 'the DQA director(s)' })}>Message DQA</button>
        <button style={btnGhost} onClick={() => setCompose({ audience: 'ADMIN', who: 'the admin(s)' })}>Message Admin</button>
      </div>

      {data?.message && <div style={note}>{data.message}</div>}
      {status === 'loading' && <div style={{ color: 'var(--muted)' }}>Loading…</div>}
      {status === 'ok' && rows.length === 0 && !data?.message && <div style={{ color: 'var(--muted)' }}>No lecturers found in this scope yet.</div>}

      {rows.length > 0 && (
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
          <thead>
            <tr style={{ textAlign: 'left', color: 'var(--muted)', fontSize: 12 }}>
              <th style={th}>Lecturer</th><th style={th}>Staff ID</th>
              {level === 'dean' && <th style={th}>Department</th>}
              <th style={th}>Units</th><th style={th}>Taught / Patrolled</th><th style={th}></th>
            </tr>
          </thead>
          <tbody>
            {rows.map(l => {
              const pct = l.patrolled_count ? Math.round((l.taught_count / l.patrolled_count) * 100) : null
              return (
                <tr key={l.staff_id} style={{ borderTop: '1px solid #e2e8f0' }}>
                  <td style={td}><b>{l.full_name}</b></td>
                  <td style={td}>{l.staff_id}</td>
                  {level === 'dean' && <td style={td}>{l.department || '—'}</td>}
                  <td style={td}>{l.unit_count}</td>
                  <td style={td}>
                    {l.taught_count}/{l.patrolled_count}
                    {pct !== null && <span style={{ marginLeft: 8, fontWeight: 700, color: pct >= 75 ? '#15803d' : '#b91c1c' }}>{pct}%</span>}
                  </td>
                  <td style={td}><button style={btnSm} onClick={() => setCompose({ audience: 'LECTURER', target: l.staff_id, who: l.full_name })}>Notify</button></td>
                </tr>
              )
            })}
          </tbody>
        </table>
      )}

      {compose && <ComposeDialog audience={compose.audience} target={compose.target} who={compose.who} onClose={() => setCompose(null)} />}
    </div>
  )
}

function ComposeDialog({ audience, target, who, onClose }: { audience: string; target?: string; who: string; onClose: () => void }) {
  const [subject, setSubject] = useState('')
  const [body, setBody] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [sent, setSent] = useState<number | null>(null)

  async function send() {
    setBusy(true); setErr(null)
    try {
      const res = await api.post<{ recipients: number }>('/api/v1/app-notifications', { audience, target_id: target, subject, body })
      setSent(res.recipients ?? 0)
    } catch (e) { setErr(e instanceof Error ? e.message : 'Failed to send') }
    finally { setBusy(false) }
  }

  return (
    <div style={overlay} onClick={onClose}>
      <div style={modal} onClick={e => e.stopPropagation()}>
        <h3 style={{ margin: '0 0 4px' }}>Notify {who}</h3>
        {sent !== null ? (
          <>
            <div style={{ ...note, background: '#f0fdf4', color: '#15803d' }}>Sent to {sent} recipient(s).</div>
            <button style={btn} onClick={onClose}>Close</button>
          </>
        ) : (
          <>
            {err && <div style={note}>{err}</div>}
            <input placeholder="Subject" value={subject} onChange={e => setSubject(e.target.value)} style={input} />
            <textarea placeholder="Message…" value={body} onChange={e => setBody(e.target.value)} style={{ ...input, height: 120, resize: 'vertical' }} />
            <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
              <button style={btnGhost} onClick={onClose}>Cancel</button>
              <button style={btn} disabled={busy || !subject.trim()} onClick={send}>{busy ? 'Sending…' : 'Send'}</button>
            </div>
          </>
        )}
      </div>
    </div>
  )
}

const th: React.CSSProperties = { padding: '6px 10px' }
const td: React.CSSProperties = { padding: '10px' }
const btn: React.CSSProperties = { padding: '8px 14px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const btnGhost: React.CSSProperties = { padding: '8px 14px', background: '#fff', color: '#1e293b', border: '1px solid #cbd5e1', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const btnSm: React.CSSProperties = { padding: '4px 10px', background: '#fff', color: '#1e293b', border: '1px solid #cbd5e1', borderRadius: 6, cursor: 'pointer', fontSize: 12 }
const selectSm: React.CSSProperties = { padding: '8px 12px', background: '#fff', color: '#1e293b', border: '1px solid #cbd5e1', borderRadius: 6, cursor: 'pointer', fontSize: 13 }
const note: React.CSSProperties = { background: '#fef2f2', color: '#b91c1c', padding: '8px 12px', borderRadius: 6, margin: '8px 0', fontSize: 13 }
const overlay: React.CSSProperties = { position: 'fixed', inset: 0, background: 'rgba(0,0,0,.4)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 50 }
const modal: React.CSSProperties = { background: '#fff', borderRadius: 12, padding: 20, width: 'min(480px, 92vw)' }
const input: React.CSSProperties = { width: '100%', padding: '10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box', margin: '6px 0' }
