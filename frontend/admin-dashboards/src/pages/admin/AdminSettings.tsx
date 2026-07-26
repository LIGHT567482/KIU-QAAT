import { useState, useEffect } from 'react'
import { api, type ThresholdConfig } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

// Tenant ADMIN settings — attendance threshold & policy for the admin's own tenant.
// Backed by the same handlers as the DQA Director thresholds page, so both roles
// can edit (tenant-scoped via JWT). See /api/v1/admin/settings/thresholds.
export default function AdminSettings() {
  const { status, data, refetch } = useQuery<ThresholdConfig>(
    () => api.get('/api/v1/admin/settings/thresholds'),
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
      await api.put('/api/v1/admin/settings/thresholds', form)
      setSaved(true); refetch()
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : 'Save failed')
    } finally {
      setSaving(false)
    }
  }

  if (status === 'loading') return <p style={{ color: 'var(--muted)' }}>Loading…</p>
  if (status === 'error' || !form) return <p style={{ color: '#b91c1c' }}>Failed to load settings.</p>

  return (
    <div style={{ maxWidth: 520 }}>
      <h2 style={{ marginBottom: 4 }}>Attendance Policy</h2>
      <p style={{ color: 'var(--muted)', marginBottom: 24 }}>
        Settings for your institution. Changes apply immediately to new sessions; the
        Daily Manifest cache is invalidated automatically.
      </p>

      <div style={{ display: 'grid', gap: 20 }}>
        <Field label="Attendance threshold (%)" hint="Fixed at a 75% floor — can be raised, never set below 75."
          value={form.attendance_threshold} min={75} max={100}
          onChange={v => setForm(f => f && ({ ...f, attendance_threshold: Math.max(75, v) }))} />
        <Field label="Check-in window (minutes)" hint="How long students can scan after gate-open"
          value={form.checkin_window_minutes} min={10} max={360}
          onChange={v => setForm(f => f && ({ ...f, checkin_window_minutes: v }))} />
        <Field label="Auto-kill timer (minutes)" hint="Session auto-closes this many minutes after initialisation"
          value={form.auto_kill_minutes} min={30} max={480}
          onChange={v => setForm(f => f && ({ ...f, auto_kill_minutes: v }))} />
      </div>

      {saved     && <div style={successBox}>Saved — manifest cache cleared.</div>}
      {saveError && <div style={errorBox}>{saveError}</div>}

      <button onClick={handleSave} disabled={saving} style={{ ...saveBtn, opacity: saving ? 0.6 : 1 }}>
        {saving ? 'Saving…' : 'Save Changes'}
      </button>

      <SessionWindowEditor />
    </div>
  )
}

const DAYS = [
  { iso: 1, label: 'Mon' }, { iso: 2, label: 'Tue' }, { iso: 3, label: 'Wed' },
  { iso: 4, label: 'Thu' }, { iso: 5, label: 'Fri' }, { iso: 6, label: 'Sat' }, { iso: 7, label: 'Sun' },
]

interface WindowCfg { start: string; end: string; active_days: number[] }

// Daily session window — coordinators may only begin a session within these
// hours/active days; outside it they see nothing to start (default 08:00–17:00).
function SessionWindowEditor() {
  const { status, data, refetch } = useQuery<WindowCfg>(() => api.get('/api/v1/admin/settings/session-window'))
  const [w, setW] = useState<WindowCfg | null>(null)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => { if (status === 'ok' && data) setW(data) }, [status, data])
  if (!w) return null

  function toggleDay(iso: number) {
    setW(c => c && ({ ...c, active_days: c.active_days.includes(iso) ? c.active_days.filter(d => d !== iso) : [...c.active_days, iso].sort() }))
  }
  async function save() {
    if (!w) return
    setSaving(true); setSaved(false); setErr(null)
    try { await api.put('/api/v1/admin/settings/session-window', w); setSaved(true); refetch() }
    catch (e) { setErr(e instanceof Error ? e.message : 'Save failed') }
    finally { setSaving(false) }
  }

  return (
    <div style={{ marginTop: 40, paddingTop: 24, borderTop: '1px solid var(--border)' }}>
      <h2 style={{ marginBottom: 4 }}>Daily Session Window</h2>
      <p style={{ color: 'var(--muted)', marginBottom: 20 }}>
        Coordinators can only begin a lecture session within this window. Outside these
        hours (or on inactive days) no sessions appear to start.
      </p>
      <div style={{ display: 'flex', gap: 20, flexWrap: 'wrap', alignItems: 'flex-end' }}>
        <label style={{ display: 'block' }}>
          <div style={{ fontWeight: 600, marginBottom: 6 }}>Opens at</div>
          <input type="time" value={w.start} onChange={e => setW(c => c && ({ ...c, start: e.target.value }))}
            style={{ padding: '8px 12px', borderRadius: 6, border: '1px solid var(--border)', fontSize: 16 }} />
        </label>
        <label style={{ display: 'block' }}>
          <div style={{ fontWeight: 600, marginBottom: 6 }}>Closes at</div>
          <input type="time" value={w.end} onChange={e => setW(c => c && ({ ...c, end: e.target.value }))}
            style={{ padding: '8px 12px', borderRadius: 6, border: '1px solid var(--border)', fontSize: 16 }} />
        </label>
      </div>
      <div style={{ marginTop: 18 }}>
        <div style={{ fontWeight: 600, marginBottom: 8 }}>Active days</div>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          {DAYS.map(d => {
            const on = w.active_days.includes(d.iso)
            return (
              <button key={d.iso} onClick={() => toggleDay(d.iso)}
                style={{ padding: '8px 14px', borderRadius: 999, fontWeight: 700, fontSize: 13, cursor: 'pointer',
                  border: `1px solid ${on ? 'var(--brand)' : 'var(--border)'}`,
                  background: on ? 'var(--brand)' : 'transparent',
                  color: on ? 'var(--brand-contrast, #fff)' : 'var(--muted)' }}>
                {d.label}
              </button>
            )
          })}
        </div>
      </div>
      {saved && <div style={successBox}>Session window saved.</div>}
      {err   && <div style={errorBox}>{err}</div>}
      <button onClick={save} disabled={saving} style={{ ...saveBtn, opacity: saving ? 0.6 : 1 }}>
        {saving ? 'Saving…' : 'Save Window'}
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
      <input type="number" min={min} max={max} value={value}
        onChange={e => onChange(Number(e.target.value))}
        style={{ padding: '8px 12px', borderRadius: 6, border: '1px solid var(--border)', fontSize: 16, width: 120 }} />
    </label>
  )
}

const successBox: React.CSSProperties = { background: '#f0fdf4', color: '#166534', padding: '10px 14px', borderRadius: 6, marginTop: 20 }
const errorBox:   React.CSSProperties = { background: '#fef2f2', color: '#b91c1c', padding: '10px 14px', borderRadius: 6, marginTop: 20 }
const saveBtn:    React.CSSProperties = { marginTop: 24, padding: '12px 24px', background: 'var(--brand)', color: 'var(--brand-contrast)', border: 'none', borderRadius: 8, fontWeight: 700, fontSize: 15, cursor: 'pointer' }
