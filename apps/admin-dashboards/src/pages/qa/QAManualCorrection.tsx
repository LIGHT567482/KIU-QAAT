import { useState } from 'react'
import { api } from '../../lib/api'

const REASON_CODES = [
  'LATE_ARRIVAL_APPROVED',
  'SYSTEM_SYNC_FAILURE',
  'QR_HARDWARE_FAULT',
  'STUDENT_EXEMPTION',
  'OTHER',
]

export default function QAManualCorrection() {
  const [form, setForm] = useState({ session_id: '', student_id: '', reason_code: REASON_CODES[0], reason_notes: '' })
  const [saving, setSaving] = useState(false)
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setSaving(true); setResult(null)
    try {
      const res = await api.post<{ status: string; log_id: string }>('/api/v1/dashboard/qa/attendance-correction', {
        session_id: form.session_id.trim(),
        student_id: form.student_id.trim(),
        reason: `${form.reason_code}: ${form.reason_notes}`.trim(),
      })
      setResult({ ok: true, message: `Recorded. Log ID: ${res.log_id}` })
      setForm(f => ({ ...f, session_id: '', student_id: '', reason_notes: '' }))
    } catch (e) {
      setResult({ ok: false, message: e instanceof Error ? e.message : 'Failed' })
    } finally {
      setSaving(false)
    }
  }

  return (
    <div style={{ maxWidth: 560 }}>
      <h2 style={{ marginBottom: 4 }}>Manual Attendance Correction</h2>
      <p style={{ color: 'var(--muted)', marginBottom: 24, fontSize: 13 }}>
        Insert a MANUAL_OVERRIDE attendance record. Only use this when automatic check-in failed due to a verified system or hardware issue. All corrections are fully audited.
      </p>

      {result && (
        <div style={{
          padding: '10px 14px', borderRadius: 8, marginBottom: 20,
          background: result.ok ? '#f0fdf4' : '#fef2f2',
          color:      result.ok ? '#166534' : '#b91c1c',
          fontWeight: 600,
        }}>
          {result.ok ? '✓ ' : '✗ '}{result.message}
        </div>
      )}

      <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        <Field label="Session ID (UUID)" required>
          <input value={form.session_id} required placeholder="e.g. 3f6a…"
            onChange={e => setForm(f => ({ ...f, session_id: e.target.value }))} style={inp} />
        </Field>

        <Field label="Student Registration Number" required>
          <input value={form.student_id} required placeholder="e.g. NUT/CS/2024/001"
            onChange={e => setForm(f => ({ ...f, student_id: e.target.value }))} style={inp} />
        </Field>

        <Field label="Reason Code" required>
          <select value={form.reason_code} onChange={e => setForm(f => ({ ...f, reason_code: e.target.value }))} style={inp}>
            {REASON_CODES.map(c => <option key={c} value={c}>{c.replace(/_/g, ' ')}</option>)}
          </select>
        </Field>

        <Field label="Additional Notes">
          <textarea value={form.reason_notes} rows={3} placeholder="Describe the circumstances…"
            onChange={e => setForm(f => ({ ...f, reason_notes: e.target.value }))}
            style={{ ...inp, resize: 'vertical' }} />
        </Field>

        <div style={{ background: '#fffbeb', border: '1px solid #fbbf24', borderRadius: 8, padding: '10px 14px', fontSize: 13 }}>
          <strong>Audit notice:</strong> This action is logged against your officer ID, timestamp, and reason. Corrections cannot be deleted.
        </div>

        <button type="submit" disabled={saving} style={btn}>
          {saving ? 'Saving…' : 'Record Correction'}
        </button>
      </form>
    </div>
  )
}

function Field({ label, children, required }: { label: string; children: React.ReactNode; required?: boolean }) {
  return (
    <label style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
      <span style={{ fontSize: 13, fontWeight: 600, color: '#475569' }}>
        {label}{required && <span style={{ color: '#ef4444', marginLeft: 2 }}>*</span>}
      </span>
      {children}
    </label>
  )
}

const inp: React.CSSProperties = { padding: '9px 12px', fontSize: 14, borderRadius: 6, border: '1px solid #e2e8f0', width: '100%', boxSizing: 'border-box' }
const btn: React.CSSProperties = { padding: '11px', fontSize: 15, fontWeight: 600, borderRadius: 6, background: '#1e293b', color: '#fff', border: 'none', cursor: 'pointer' }
