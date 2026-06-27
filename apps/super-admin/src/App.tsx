import { useState } from 'react'
import { getSession, clearSession, type Session } from './auth'
import { BrandHeader } from './components/BrandHeader'
import { useTheme, ThemeToggle } from './theme'
import { api } from './lib/api'
import Login from './pages/Login'
import Tenants from './pages/Tenants'

export default function App() {
  const { theme, toggle } = useTheme()
  const [session, setSession] = useState<Session | null>(() => getSession())
  const [acctOpen, setAcctOpen] = useState(false)

  if (!session) {
    return <Login onLogin={setSession} theme={theme} toggle={toggle} />
  }

  return (
    <div style={{ minHeight: '100vh', display: 'flex', flexDirection: 'column', background: 'var(--app-bg)', fontFamily: 'system-ui' }}>
      <BrandHeader right={
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <ThemeToggle theme={theme} toggle={toggle} />
          <button onClick={() => setAcctOpen(true)} style={{
            padding: '8px 14px', background: 'rgba(255,255,255,.15)', color: 'var(--sidebar-text)',
            border: '1px solid rgba(255,255,255,.25)', borderRadius: 6, cursor: 'pointer', fontSize: 13, fontWeight: 600,
          }}>Account</button>
          <button onClick={() => { clearSession(); setSession(null) }} style={{
            padding: '8px 14px', background: 'rgba(255,255,255,.15)', color: 'var(--sidebar-text)',
            border: '1px solid rgba(255,255,255,.25)', borderRadius: 6, cursor: 'pointer', fontSize: 13, fontWeight: 600,
          }}>Sign out</button>
        </div>
      } />
      {acctOpen && <AccountModal onClose={() => setAcctOpen(false)} />}
      <div style={{ flex: 1 }}><Tenants /></div>
      <footer style={{
        background: 'var(--footer)', color: 'var(--footer-text)', padding: '14px 24px',
        fontSize: 12, textAlign: 'center',
      }}>
        Powered by LIGHT TECHNOLOGIES · Super-admin console
      </footer>
    </div>
  )
}

// Self-service account settings for the platform owner: change sign-in email and
// password. Both re-verify the current password before applying.
function AccountModal({ onClose }: { onClose: () => void }) {
  const [pw, setPw] = useState({ cur: '', next: '', confirm: '' })
  const [em, setEm] = useState({ cur: '', email: '' })
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null)
  const [busy, setBusy] = useState(false)

  async function changeEmail() {
    if (!em.email.trim()) { setMsg({ ok: false, text: 'Enter a new email.' }); return }
    setBusy(true); setMsg(null)
    try {
      await api.post('/api/v1/auth/change-email', { current_password: em.cur, new_email: em.email.trim() })
      setMsg({ ok: true, text: 'Sign-in email updated. Use it at your next login.' })
      setEm({ cur: '', email: '' })
    } catch (e) { setMsg({ ok: false, text: e instanceof Error ? e.message : 'Failed' }) }
    finally { setBusy(false) }
  }
  async function changePassword() {
    if (pw.next.length < 8) { setMsg({ ok: false, text: 'New password must be at least 8 characters.' }); return }
    if (pw.next !== pw.confirm) { setMsg({ ok: false, text: 'New passwords do not match.' }); return }
    setBusy(true); setMsg(null)
    try {
      await api.post('/api/v1/auth/change-password', { current_password: pw.cur, new_password: pw.next })
      setMsg({ ok: true, text: 'Password changed.' })
      setPw({ cur: '', next: '', confirm: '' })
    } catch (e) { setMsg({ ok: false, text: e instanceof Error ? e.message : 'Failed' }) }
    finally { setBusy(false) }
  }

  const inp: React.CSSProperties = { width: '100%', padding: 10, borderRadius: 6, border: '1px solid var(--border,#e2e8f0)', marginBottom: 10, boxSizing: 'border-box', background: 'var(--surface,#fff)', color: 'var(--text,#0f172a)' }
  const btn: React.CSSProperties = { padding: '10px 16px', background: 'var(--brand)', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 700, fontSize: 13 }

  return (
    <div onClick={onClose} style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 9999, padding: 20 }}>
      <div onClick={e => e.stopPropagation()} style={{ background: 'var(--surface,#fff)', color: 'var(--text,#0f172a)', borderRadius: 12, padding: 24, width: 380, maxHeight: '90vh', overflowY: 'auto' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 4 }}>
          <h3 style={{ margin: 0 }}>Account settings</h3>
          <button onClick={onClose} style={{ border: 'none', background: 'transparent', fontSize: 20, cursor: 'pointer', color: 'inherit' }}>×</button>
        </div>
        {msg && <div style={{ background: msg.ok ? '#f0fdf4' : '#fef2f2', color: msg.ok ? '#166534' : '#b91c1c', padding: '8px 12px', borderRadius: 6, margin: '10px 0', fontSize: 13 }}>{msg.text}</div>}

        <h4 style={{ margin: '16px 0 8px' }}>Change sign-in email</h4>
        <input type="email" placeholder="New sign-in email" value={em.email} onChange={e => setEm(s => ({ ...s, email: e.target.value }))} style={inp} />
        <input type="password" placeholder="Current password" value={em.cur} onChange={e => setEm(s => ({ ...s, cur: e.target.value }))} style={inp} />
        <button onClick={changeEmail} disabled={busy || !em.cur || !em.email} style={btn}>{busy ? 'Saving…' : 'Update email'}</button>

        <h4 style={{ margin: '20px 0 8px' }}>Change password</h4>
        <input type="password" placeholder="Current password" value={pw.cur} onChange={e => setPw(s => ({ ...s, cur: e.target.value }))} style={inp} />
        <input type="password" placeholder="New password (min 8)" value={pw.next} onChange={e => setPw(s => ({ ...s, next: e.target.value }))} style={inp} />
        <input type="password" placeholder="Confirm new password" value={pw.confirm} onChange={e => setPw(s => ({ ...s, confirm: e.target.value }))} style={inp} />
        <button onClick={changePassword} disabled={busy || !pw.cur || !pw.next} style={btn}>{busy ? 'Saving…' : 'Update password'}</button>
      </div>
    </div>
  )
}
