import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { api } from '../../lib/api'
import { useAuth } from '../../contexts/AuthContext'

const REASON_CODES = ['LOST_PHONE', 'DAMAGED_DEVICE', 'OTHER'] as const
type ReasonCode = typeof REASON_CODES[number]

interface MonitorBinding {
  user_id:      string
  full_name:    string
  email:        string
  staff_id:     string
  bound_at:     string
  last_seen_at: string
}

export default function QADeviceReset() {
  const { user } = useAuth()
  const [form, setForm] = useState({ student_id: '', reason_code: 'LOST_PHONE' as ReasonCode, reason_text: '' })
  const [result, setResult] = useState<'success' | 'error' | null>(null)
  const [message, setMessage] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setLoading(true); setResult(null)
    try {
      await api.post('/api/v1/dashboard/qa/device-reset', {
        ...form,
        officer_id: user?.userId,
      })
      setResult('success')
      setMessage(`Hardware fingerprint for ${form.student_id} has been reset. The student's next scan will re-bind their device.`)
      setForm(f => ({ ...f, student_id: '', reason_text: '' }))
    } catch (e) {
      setResult('error')
      setMessage(e instanceof Error ? e.message : 'Reset failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{ maxWidth: 720 }}>
      <div style={{ maxWidth: 480 }}>
      <h2 style={{ marginBottom: 4 }}>Student device binding</h2>
      <p style={{ color: 'var(--muted)', marginBottom: 24 }}>
        Clears a student's hardware fingerprint so they can re-bind on their next scan.
        This action is logged to the audit trail.
      </p>

      {result === 'success' && <div style={successBox}>{message}</div>}
      {result === 'error'   && <div style={errorBox}>{message}</div>}

      <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        <label>
          <div style={labelStyle}>Student Registration Number</div>
          <input
            value={form.student_id} required
            onChange={e => setForm(f => ({ ...f, student_id: e.target.value }))}
            placeholder="e.g. REG-2024-0001"
            style={inputStyle}
          />
        </label>

        <label>
          <div style={labelStyle}>Reason</div>
          <select
            value={form.reason_code}
            onChange={e => setForm(f => ({ ...f, reason_code: e.target.value as ReasonCode }))}
            style={inputStyle}
          >
            {REASON_CODES.map(r => <option key={r} value={r}>{r.replace(/_/g, ' ')}</option>)}
          </select>
        </label>

        <label>
          <div style={labelStyle}>Additional notes (optional)</div>
          <textarea
            value={form.reason_text}
            onChange={e => setForm(f => ({ ...f, reason_text: e.target.value }))}
            rows={3}
            style={{ ...inputStyle, resize: 'vertical' }}
          />
        </label>

        <button type="submit" disabled={loading} style={{ ...saveBtn, opacity: loading ? 0.6 : 1 }}>
          {loading ? 'Resetting…' : 'Reset Device Binding'}
        </button>
      </form>
      </div>

      <MonitorHandsets />
    </div>
  )
}

/**
 * The monitor's side of the same problem.
 *
 * A monitor account records itself against the first handset it is used from. When that phone is
 * lost, replaced, reinstalled, or simply reports a different fingerprint, the monitor can be
 * refused mid-round and cannot free themselves — releasing a binding is deliberately not
 * self-service, or the lock would mean nothing. That made an institution administrator the only
 * way out of a QA field problem. This is the same release, on the desk of the officer whose
 * monitors they are.
 *
 * The fingerprint itself is never shown: a QA officer needs to know that a handset is claimed and
 * when it was last used, never the value that would let them impersonate it.
 */
function MonitorHandsets() {
  const [rows, setRows] = useState<MonitorBinding[] | null>(null)
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState<string | null>(null)
  const [note, setNote] = useState('')

  const load = useCallback(async () => {
    setErr('')
    try {
      const r = await api.get<{ bindings: MonitorBinding[] }>('/api/v1/dashboard/qa/patrol-bindings')
      setRows(r.bindings ?? [])
    } catch (e) {
      setRows([])
      setErr(e instanceof Error ? e.message : 'Could not load monitor handsets')
    }
  }, [])

  useEffect(() => { void load() }, [load])

  async function release(b: MonitorBinding) {
    const who = b.full_name || b.staff_id || b.email || 'this monitor'
    if (!confirm(`Release ${who}'s handset?\n\nThey will claim whichever phone they next sign in on. This is recorded in the audit trail.`)) return
    setBusy(b.user_id); setNote(''); setErr('')
    try {
      await api.delete(`/api/v1/dashboard/qa/patrol-bindings/${b.user_id}`)
      setNote(`${who} can now claim a new handset on their next sign-in.`)
      await load()
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'Release failed')
    } finally {
      setBusy(null)
    }
  }

  const when = (iso: string) => {
    if (!iso) return '—'
    const d = new Date(iso.replace(' ', 'T'))
    return isNaN(d.getTime()) ? iso : d.toLocaleString()
  }

  return (
    <section style={{ marginTop: 44, borderTop: '1px solid #e2e8f0', paddingTop: 28 }}>
      <h2 style={{ marginBottom: 4 }}>QA monitor handsets</h2>
      <p style={{ color: 'var(--muted)', marginBottom: 20 }}>
        Which phone each QA monitor is registered on. Release one when a monitor has changed
        phone or is locked out of their round — they claim the new handset on their next sign-in.
        Every release is written to the audit trail.
      </p>

      {note && <div style={successBox}>{note}</div>}
      {err  && <div style={errorBox}>{err}</div>}

      {rows === null && <div style={{ color: 'var(--muted)' }}>Loading…</div>}
      {rows !== null && rows.length === 0 && !err && (
        <div style={{ color: 'var(--muted)' }}>No QA monitor has claimed a handset yet.</div>
      )}

      {rows !== null && rows.length > 0 && (
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
            <thead>
              <tr style={{ textAlign: 'left', color: 'var(--muted)' }}>
                <th style={th}>QA Monitor</th>
                <th style={th}>Staff ID</th>
                <th style={th}>Claimed</th>
                <th style={th}>Last used</th>
                <th style={th} />
              </tr>
            </thead>
            <tbody>
              {rows.map(b => (
                <tr key={b.user_id} style={{ borderTop: '1px solid #e2e8f0' }}>
                  <td style={td}>
                    <div style={{ fontWeight: 600 }}>{b.full_name || '—'}</div>
                    <div style={{ color: 'var(--muted)', fontSize: 12 }}>{b.email}</div>
                  </td>
                  <td style={td}>{b.staff_id || '—'}</td>
                  <td style={td}>{when(b.bound_at)}</td>
                  <td style={td}>{when(b.last_seen_at)}</td>
                  <td style={{ ...td, textAlign: 'right' }}>
                    <button
                      onClick={() => void release(b)}
                      disabled={busy === b.user_id}
                      style={{ ...releaseBtn, opacity: busy === b.user_id ? 0.6 : 1 }}
                    >
                      {busy === b.user_id ? 'Releasing…' : 'Release handset'}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}

const labelStyle: React.CSSProperties = { fontWeight: 600, marginBottom: 4, fontSize: 14 }
const inputStyle: React.CSSProperties = { width: '100%', padding: '9px 12px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }
const saveBtn:    React.CSSProperties = { padding: '12px 24px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 8, fontWeight: 700, fontSize: 15, cursor: 'pointer' }
const successBox: React.CSSProperties = { background: '#f0fdf4', color: '#166534', padding: '10px 14px', borderRadius: 6, marginBottom: 20 }
const errorBox:   React.CSSProperties = { background: '#fef2f2', color: '#b91c1c', padding: '10px 14px', borderRadius: 6, marginBottom: 20 }
const th:         React.CSSProperties = { padding: '8px 10px', fontWeight: 600, fontSize: 12, textTransform: 'uppercase', letterSpacing: 0.3 }
const td:         React.CSSProperties = { padding: '10px' }
const releaseBtn: React.CSSProperties = { padding: '7px 14px', background: '#fff', color: '#b91c1c', border: '1px solid #fecaca', borderRadius: 6, fontWeight: 600, fontSize: 13, cursor: 'pointer', whiteSpace: 'nowrap' }
