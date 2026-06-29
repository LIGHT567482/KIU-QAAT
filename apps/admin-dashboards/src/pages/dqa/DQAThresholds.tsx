import { useState, useEffect } from 'react'
import { api, type ThresholdConfig } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

export default function DQAThresholds() {
  const { status, data, refetch } = useQuery<ThresholdConfig>(
    () => api.get('/api/v1/dashboard/dqa/thresholds'),
  )

  const [form, setForm] = useState<ThresholdConfig | null>(null)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)

  useEffect(() => {
    if (status === 'ok' && data) setForm(data)
  }, [status, data])

  async function handleSave() {
    if (!form) return
    setSaving(true); setSaved(false); setSaveError(null)
    try {
      await api.put('/api/v1/dashboard/dqa/thresholds', form)
      setSaved(true)
      refetch()
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : 'Save failed')
    } finally {
      setSaving(false)
    }
  }

  if (status === 'loading') return <p style={{ color: 'var(--muted)' }}>Loading…</p>
  if (status === 'error' || !form) return <p style={{ color: '#b91c1c' }}>Failed to load thresholds.</p>

  return (
    <div style={{ maxWidth: 520 }}>
      <h2 style={{ marginBottom: 4 }}>Policy Thresholds</h2>
      <p style={{ color: 'var(--muted)', marginBottom: 24 }}>
        Changes apply immediately to all new sessions. The Daily Manifest cache is invalidated automatically.
      </p>

      <div style={{ display: 'grid', gap: 20 }}>
        <Field
          label="Attendance threshold (%)"
          hint="Minimum % required for exam eligibility — fixed at a 75% floor; can be raised, never lowered below 75."
          value={form.attendance_threshold}
          min={75} max={100}
          onChange={v => setForm(f => f && ({ ...f, attendance_threshold: Math.max(75, v) }))}
        />
        <Field
          label="Check-in window (minutes)"
          hint="How long students can scan after gate-open"
          value={form.checkin_window_minutes}
          min={10} max={360}
          onChange={v => setForm(f => f && ({ ...f, checkin_window_minutes: v }))}
        />
        <Field
          label="Auto-kill timer (minutes)"
          hint="Session auto-closes this many minutes after initialisation"
          value={form.auto_kill_minutes}
          min={30} max={480}
          onChange={v => setForm(f => f && ({ ...f, auto_kill_minutes: v }))}
        />
      </div>

      {saved      && <div style={successBox}>Saved — manifest cache cleared.</div>}
      {saveError  && <div style={errorBox}>{saveError}</div>}

      <button onClick={handleSave} disabled={saving} style={{ ...saveBtn, opacity: saving ? 0.6 : 1 }}>
        {saving ? 'Saving…' : 'Save Changes'}
      </button>
    </div>
  )
}

function Field({ label, hint, value, min, max, onChange }: {
  label: string; hint: string; value: number; min: number; max: number
  onChange: (v: number) => void
}) {
  return (
    <label style={{ display: 'block' }}>
      <div style={{ fontWeight: 600, marginBottom: 2 }}>{label}</div>
      <div style={{ fontSize: 12, color: 'var(--muted)', marginBottom: 6 }}>{hint}</div>
      <input
        type="number" min={min} max={max} value={value}
        onChange={e => onChange(Number(e.target.value))}
        style={{ padding: '8px 12px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 16, width: 120 }}
      />
    </label>
  )
}

const successBox: React.CSSProperties = { background: '#f0fdf4', color: '#166534', padding: '10px 14px', borderRadius: 6, marginTop: 20 }
const errorBox:   React.CSSProperties = { background: '#fef2f2', color: '#b91c1c', padding: '10px 14px', borderRadius: 6, marginTop: 20 }
const saveBtn:    React.CSSProperties = { marginTop: 24, padding: '12px 24px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 8, fontWeight: 700, fontSize: 15, cursor: 'pointer' }
