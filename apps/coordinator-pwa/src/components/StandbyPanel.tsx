import { useCallback, useEffect, useState } from 'react'

const API = import.meta.env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')

// Emergency standby — the coordinator authorises a student of their OWN cohort to
// run a session in their absence. Issues a code the coordinator reads out; the
// student signs in with it on the login screen. Lives inside the Attendance
// feature (session screen), since it is part of running attendance for the day.
interface Standby { delegation_id: string; code: string; deputy_reg: string; deputy_name: string; expires_at: string }

export default function StandbyPanel({ token }: { token: string | null }) {
  const [open, setOpen] = useState(false)
  const [reg, setReg] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [list, setList] = useState<Standby[]>([])

  const load = useCallback(() => {
    if (!token) return
    fetch(`${API}/api/v1/coordinator/standby`, { headers: { Authorization: `Bearer ${token}` } })
      .then(r => (r.ok ? r.json() : [])).then((d: Standby[]) => setList(Array.isArray(d) ? d : [])).catch(() => {})
  }, [token])
  useEffect(() => { if (open) load() }, [open, load])

  async function issue() {
    if (!reg.trim()) return
    setBusy(true); setErr(null)
    try {
      const res = await fetch(`${API}/api/v1/coordinator/standby`, {
        method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ deputy_reg: reg.trim() }),
      })
      const d = await res.json()
      if (!res.ok) throw new Error(d.message || 'Could not create standby')
      setReg(''); load()
    } catch (e) { setErr(e instanceof Error ? e.message : 'Failed') } finally { setBusy(false) }
  }
  async function revoke(id: string) {
    if (!confirm('Revoke this standby? The student will no longer be able to run your session.')) return
    await fetch(`${API}/api/v1/coordinator/standby/${id}/revoke`, { method: 'POST', headers: { Authorization: `Bearer ${token}` } }).catch(() => {})
    load()
  }

  return (
    <div style={{ border: '1px solid var(--border,#e2e8f0)', borderRadius: 12, padding: '10px 14px', marginBottom: 16, background: 'var(--surface,#fff)' }}>
      <button onClick={() => setOpen(o => !o)} style={{ width: '100%', background: 'none', border: 'none', cursor: 'pointer', display: 'flex', justifyContent: 'space-between', alignItems: 'center', color: 'inherit', fontWeight: 700, fontSize: 13, padding: 0 }}>
        <span>🆘 Emergency standby {list.length > 0 && <span style={{ color: '#b45309' }}>· {list.length} active</span>}</span>
        <span style={{ color: 'var(--muted)' }}>{open ? '▾' : '▸'}</span>
      </button>
      {open && (
        <div style={{ marginTop: 10 }}>
          <p style={{ fontSize: 12, color: 'var(--muted,#64748b)', margin: '0 0 10px' }}>
            Going to be absent? Authorise a student <strong>from your own cohort</strong> to run the session today. Read them the code — they sign in with “standby code” on the login screen. Valid until end of day.
          </p>
          {err && <div style={{ background: '#fef2f2', color: '#b91c1c', padding: '6px 10px', borderRadius: 6, marginBottom: 8, fontSize: 12 }}>{err}</div>}
          <div style={{ display: 'flex', gap: 8 }}>
            <input value={reg} onChange={e => setReg(e.target.value)} placeholder="Standby student's registration number"
              style={{ flex: 1, padding: '8px 10px', borderRadius: 8, border: '1px solid var(--border,#e2e8f0)', fontSize: 13, background: 'var(--surface,#fff)', color: 'inherit' }} />
            <button onClick={issue} disabled={busy || !reg.trim()} style={{ padding: '8px 14px', background: '#b45309', color: '#fff', border: 'none', borderRadius: 8, cursor: 'pointer', fontWeight: 700, fontSize: 13, whiteSpace: 'nowrap', opacity: busy || !reg.trim() ? 0.6 : 1 }}>{busy ? 'Issuing…' : 'Issue code'}</button>
          </div>
          {list.length > 0 && (
            <div style={{ marginTop: 12, display: 'flex', flexDirection: 'column', gap: 6 }}>
              {list.map(s => (
                <div key={s.delegation_id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 10, background: 'var(--bg,#f8fafc)', borderRadius: 8, padding: '8px 10px' }}>
                  <div style={{ minWidth: 0 }}>
                    <span style={{ fontFamily: 'monospace', fontWeight: 800, fontSize: 16, letterSpacing: 1 }}>{s.code}</span>
                    <div style={{ fontSize: 11, color: 'var(--muted,#64748b)' }}>{s.deputy_name || s.deputy_reg} · until {new Date(s.expires_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</div>
                  </div>
                  <button onClick={() => revoke(s.delegation_id)} style={{ padding: '5px 10px', background: 'none', border: '1px solid #fecaca', color: '#b91c1c', borderRadius: 6, cursor: 'pointer', fontSize: 12 }}>Revoke</button>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
