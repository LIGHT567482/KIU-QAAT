import { useState, type FormEvent } from 'react'
import { api } from '../../lib/api'
import { useAuth } from '../../contexts/AuthContext'

const REASON_CODES = ['LOST_PHONE', 'DAMAGED_DEVICE', 'OTHER'] as const
type ReasonCode = typeof REASON_CODES[number]

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
    <div style={{ maxWidth: 480 }}>
      <h2 style={{ marginBottom: 4 }}>Device Binding Reset</h2>
      <p style={{ color: '#64748b', marginBottom: 24 }}>
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
  )
}

const labelStyle: React.CSSProperties = { fontWeight: 600, marginBottom: 4, fontSize: 14 }
const inputStyle: React.CSSProperties = { width: '100%', padding: '9px 12px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }
const saveBtn:    React.CSSProperties = { padding: '12px 24px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 8, fontWeight: 700, fontSize: 15, cursor: 'pointer' }
const successBox: React.CSSProperties = { background: '#f0fdf4', color: '#166534', padding: '10px 14px', borderRadius: 6, marginBottom: 20 }
const errorBox:   React.CSSProperties = { background: '#fef2f2', color: '#b91c1c', padding: '10px 14px', borderRadius: 6, marginBottom: 20 }
