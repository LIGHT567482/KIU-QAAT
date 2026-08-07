import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import PasswordInput from '../components/PasswordInput'
import { useAuth, type Role } from '../contexts/AuthContext'
import { useTheme, ThemeToggle } from '../theme'
import brand from '../brand.json'

const API = import.meta.env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')

// Where each role lands after signing in. A role missing from this map has no web dashboard at
// all — see NO_WEB_DASHBOARD below.
const ROLE_REDIRECT: Partial<Record<Role, string>> = {
  VC:           '/vc',
  DQA_DIRECTOR: '/dqa',
  QA_OFFICER:   '/qa/reports',
  ADMIN:        '/admin',
  LECTURER:     '/lecturer',
  HOD:          '/hod',
  DEAN:         '/dean',
  QA_SCHOOL_HANDLER: '/qa-school',
  QA_DEPT_REP:       '/qa-dept',
  TLC:               '/tlc',
}

// Roles that work from a phone rather than this console. Their credentials are valid — sending
// them to a route that does not exist would bounce them back here looking like a failed login, so
// say plainly which app to open instead.
const NO_WEB_DASHBOARD: Partial<Record<Role, string>> = {
  COORDINATOR: 'Coordinators run sessions from the KIU QAAT mobile app — there is no web dashboard for this account. Sign in there with the same details, or with your coordinator code.',
}

/**
 * Fetch that survives a sleeping backend, and reports what actually happened.
 *
 * THE BUG THIS REPLACES. handleSubmit called `await res.json()` before looking at the status. The
 * services run on a free tier that hibernates after ~15 minutes idle, and a hibernating one
 * answers with an HTML holding page (502) or a plain-text throttle (429,
 * `x-render-routing: hibernate-rate-limited`). Calling .json() on either THROWS, the throw landed
 * in the outer catch, and every one of those became **"Network error"** — on a machine with a
 * perfectly good network, for a user whose password was never even checked. There was no way to
 * tell that from a real outage, and the honest answer ("the server is waking up, wait a moment")
 * was never shown.
 *
 * So: read the body as TEXT first, decide from the status and the content, and retry through the
 * wake-up rather than reporting it as a failure. The phone app has done this since the beginning
 * (AuthClient.appLogin) — the console simply never learned.
 *
 * @param onWaking called when a wake-up is detected, so the form can say so instead of freezing.
 */
async function fetchJSON(
  url: string, init: RequestInit, onWaking: (msg: string) => void, attempts = 5,
): Promise<{ status: number; data: Record<string, unknown> }> {
  let lastText = ''
  for (let i = 0; i < attempts; i++) {
    let res: Response
    try {
      res = await fetch(url, init)
    } catch {
      // A genuine network failure — no server reached at all. This, and only this, is what
      // "network error" should ever have meant.
      if (i === attempts - 1) throw new Error('Could not reach the server. Check your connection.')
      await new Promise(r => setTimeout(r, 1500 * (i + 1)))
      continue
    }
    lastText = await res.text()
    const looksJSON = lastText.trimStart().startsWith('{') || lastText.trimStart().startsWith('[')

    // 429 with a plain-text body, or 5xx with an HTML holding page: the service is asleep and
    // spinning up. Not an error the user can act on — wait it out.
    const waking = res.status === 429 || (res.status >= 500 && !looksJSON)
    if (waking && i < attempts - 1) {
      const after = Number(res.headers.get('Retry-After')) || 0
      const wait = Math.min(Math.max(after * 1000, 2000 * (i + 1)), 10000)
      onWaking('The server is waking up — this takes up to a minute on the first sign-in of the day. Still trying…')
      await new Promise(r => setTimeout(r, wait))
      continue
    }
    if (!looksJSON) {
      throw new Error(res.status === 429
        ? 'The server is busy waking up. Wait about a minute and try again.'
        : `The server is not answering properly yet (HTTP ${res.status}). Wait a moment and try again.`)
    }
    return { status: res.status, data: JSON.parse(lastText) as Record<string, unknown> }
  }
  throw new Error('The server is still waking up. Wait about a minute and try again.')
}

export default function Login() {
  const { login } = useAuth()
  const { theme, toggle } = useTheme()
  const navigate = useNavigate()
  const [form, setForm] = useState({ email: '', password: '', totp_code: '' })
  const [needsMFA, setNeedsMFA] = useState(false)
  const [resolvedTenantId, setResolvedTenantId] = useState('')
  const [error, setError] = useState<string | null>(null)
  // Said out loud while a hibernating service spins up, so the button does not just sit there
  // looking broken for a minute.
  const [waking, setWaking] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  async function resolveTenant(email: string): Promise<string> {
    const { status, data } = await fetchJSON(
      `${API}/api/v1/auth/tenant-lookup?email=${encodeURIComponent(email)}`, {}, setWaking)
    if (status !== 200 || !data.tenant_id) throw new Error('No account found for that email address.')
    return data.tenant_id as string
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      let tid = resolvedTenantId
      if (!tid) {
        try {
          tid = await resolveTenant(form.email)
          setResolvedTenantId(tid)
        } catch {
          setError('No account found for that email address.')
          setLoading(false)
          return
        }
      }

      const { status: loginStatus, data } = await fetchJSON(`${API}/api/v1/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...form, tenant_id: tid }),
      }, setWaking)
      setWaking(null)
      const res = { status: loginStatus, ok: loginStatus >= 200 && loginStatus < 300 }

      if (res.status === 403 && data.error === 'MFA_REQUIRED') {
        setNeedsMFA(true)
        setLoading(false)
        return
      }
      if (!res.ok) {
        setError((data.message as string) ?? 'Login failed')
        setLoading(false)
        return
      }

      // Resolve the landing page BEFORE storing the session: a role with no web dashboard should
      // be told so, not signed in and then bounced back to this page by the catch-all route.
      const dest = ROLE_REDIRECT[data.role as Role]
      if (!dest) {
        setError(NO_WEB_DASHBOARD[data.role as Role]
          ?? 'This account has no dashboard in the web console.')
        setLoading(false)
        return
      }

      sessionStorage.setItem('qaat_welcome', (data.full_name as string) || form.email)
      login(data.access_token as string, {
        userId:    data.user_id as string,
        tenantId:  tid,
        role:      data.role as Role,
        expiresAt: Math.floor(Date.now() / 1000) + (data.expires_in as number),
        // Carried so every dashboard can greet the person by title and name,
        // rather than only the four-second toast that fires once at sign-in.
        fullName:  data.full_name as string,
        title:     data.title as string,
      })
      navigate(dest)
    } catch (e) {
      // The real reason, whatever it is — never a blanket "Network error".
      setError(e instanceof Error ? e.message : 'Sign-in failed')
    } finally {
      setLoading(false); setWaking(null)
    }
  }

  return (
    <div style={{
      minHeight: '100vh', display: 'flex', alignItems: 'center',
      justifyContent: 'center', background: 'var(--bg)', fontFamily: 'system-ui', position: 'relative', overflow: 'hidden',
    }}>
      <img src={brand.logo_url} alt="" aria-hidden style={{
        position: 'absolute', width: 460, maxWidth: '80vw', opacity: 0.05,
        left: '50%', top: '50%', transform: 'translate(-50%, -50%)', pointerEvents: 'none',
      }} />
      <div style={{ position: 'absolute', top: 16, right: 16 }}>
        <ThemeToggle theme={theme} toggle={toggle} />
      </div>
      <div style={{ background: 'var(--surface)', color: 'var(--text)', borderRadius: 12, padding: 40, width: 380, boxShadow: 'var(--shadow)', border: '1px solid var(--border)' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 4 }}>
          {brand.logo_url && <img src={brand.logo_url} alt={brand.name} style={{ height: 48, width: 48, objectFit: 'contain', borderRadius: 8 }} />}
          <h1 style={{ margin: 0, fontSize: 20, lineHeight: 1.15 }}>{brand.name}</h1>
        </div>
        <p style={{ color: 'var(--muted)', marginBottom: 28 }}>Sign in to your dashboard</p>

        {waking && (
          <div style={{ background: '#fffbeb', color: '#92400e', padding: '10px 14px', borderRadius: 6, marginBottom: 16, fontSize: 13 }}>
            {waking}
          </div>
        )}

        {error && (
          <div style={{ background: '#fee2e2', color: '#b91c1c', padding: '10px 14px', borderRadius: 6, marginBottom: 16 }}>
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <input type="email" placeholder="Email" value={form.email} autoComplete="username"
            onChange={e => { setForm(f => ({ ...f, email: e.target.value })); setResolvedTenantId('') }}
            required style={inp} />
          <PasswordInput placeholder="Password" value={form.password} autoComplete="current-password"
            onChange={e => setForm(f => ({ ...f, password: e.target.value }))} required style={inp} />
          {needsMFA && (
            <input type="text" inputMode="numeric" pattern="\d{6}" placeholder="Authenticator code"
              value={form.totp_code} onChange={e => setForm(f => ({ ...f, totp_code: e.target.value }))}
              required autoFocus style={inp} />
          )}
          <button type="submit" disabled={loading} style={btn}>
            {loading ? (waking ? 'Waking the server…' : 'Signing in…') : needsMFA ? 'Verify' : 'Sign In'}
          </button>
        </form>
      </div>
    </div>
  )
}

const inp: React.CSSProperties = {
  padding: '10px 12px', fontSize: 15, borderRadius: 6,
  border: '1px solid var(--border)', width: '100%', boxSizing: 'border-box',
}
const btn: React.CSSProperties = {
  padding: 12, fontSize: 15, fontWeight: 600, borderRadius: 6,
  background: 'var(--brand)', color: 'var(--brand-contrast)', border: 'none', cursor: 'pointer', marginTop: 4,
}
