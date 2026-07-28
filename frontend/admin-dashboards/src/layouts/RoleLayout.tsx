import { useEffect, useState } from 'react'
import { Navigate, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useAuth, type Role } from '../contexts/AuthContext'
import { api } from '../lib/api'
import PasswordInput from '../components/PasswordInput'
import { useTheme, ThemeToggle, applyPalette } from '../theme'

interface Branding {
  name: string; logo_url: string; motto: string
  brand_color: string; sidebar_color: string; background_color: string; footer_color: string
  text_color_light?: string; text_color_dark?: string
}

interface RoleLayoutProps {
  allowedRoles: Role[]
}

// Wraps a route group — redirects to /login if unauthenticated or wrong role.
export function RoleLayout({ allowedRoles }: RoleLayoutProps) {
  const { user, isAuthenticated } = useAuth()
  const [brand, setBrand] = useState<Branding | null>(null)
  const navigate = useNavigate()
  const location = useLocation()

  // Institution branding + colour palette, fetched once and applied app-wide.
  useEffect(() => {
    if (!isAuthenticated) return
    api.get<Branding>('/api/v1/branding').then(b => { setBrand(b); applyPalette(b) }).catch(() => setBrand(null))
  }, [isAuthenticated])

  if (!isAuthenticated || !user) {
    return <Navigate to="/login" replace />
  }
  if (!allowedRoles.includes(user.role)) {
    return <Navigate to="/unauthorized" replace />
  }
  return (
    // app-shell = min-height:100dvh (with 100vh fallback) so the footer always sits at
    // the bottom of the visible viewport, on phones too. No margins → full-bleed.
    <div className="app-shell" style={{ display: 'flex', fontFamily: 'system-ui' }}>
      <Sidebar role={user.role} brand={brand} />
      {/* Content column fills the rest and carries the tenant background colour. */}
      <div style={{
        flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', background: 'var(--app-bg)',
      }}>
        <main style={{ flex: 1, padding: 24, color: 'var(--text)' }}>
          <GoBack navigate={navigate} location={location} />
          <Outlet />
        </main>
        <footer style={{ background: 'var(--footer)', color: 'var(--footer-text)', padding: '12px 24px', fontSize: 12, textAlign: 'center' }}>
          {brand?.name ? `${brand.name} · ` : ''}Powered by LIGHT TECHNOLOGIES
        </footer>
      </div>
    </div>
  )
}

type NavLink = { label: string; path: string }

const NAV: Record<Role, NavLink[]> = {
  VC: [
    { label: 'Overview',            path: '/vc' },
    { label: 'Lecturer Workload',   path: '/vc/lecturer-workload' },
    { label: 'Lecturer Attendance', path: '/vc/lecturer-attendance' },
    { label: 'Student Attendance',  path: '/vc/student-attendance' },
  ],
  DQA_DIRECTOR: [
    { label: 'Thresholds',          path: '/dqa/thresholds' },
    { label: 'Eligibility',         path: '/dqa/eligibility' },
    { label: 'Course Health',       path: '/dqa/course-health' },
    { label: 'Trends',              path: '/dqa/trends' },
    { label: 'Punctuality',         path: '/dqa/punctuality' },
    { label: 'Lecturer Attendance', path: '/dqa/lecturer-attendance' },
    { label: 'Student Attendance',  path: '/dqa/student-attendance' },
  ],
  QA_OFFICER: [
    { label: 'Live Sessions',       path: '/qa/live' },
    { label: 'Timetable',           path: '/qa/timetable' },
    { label: 'Student Attendance',  path: '/qa/student-attendance' },
    { label: 'Lecturer Attendance', path: '/qa/lecturer-attendance' },
    { label: 'Manual Correction',   path: '/qa/correction' },
    { label: 'Coordinator Health',  path: '/qa/coordinator-health' },
    { label: 'Device Reset',        path: '/qa/device-reset' },
  ],
  COORDINATOR: [],
  ADMIN: [], // built per-tenant in adminNav() — needs the admin's tenant_id.
  LECTURER: [
    { label: 'My Attendance', path: '/lecturer' },
  ],
}

// The ADMIN sidebar lists EVERY management page so nothing is buried behind the
// home grid. The sub-resource pages are keyed by the admin's own tenant_id.
function adminNav(tenantId: string): NavLink[] {
  const t = `/admin/tenants/${tenantId}`
  return [
    { label: 'Home',                path: '/admin' },
    { label: 'Administration',      path: `${t}/users` },
    { label: 'Courses & Sessions',  path: `${t}/courses` },
    { label: 'Timetable',           path: '/admin/timetable' },
    { label: 'Students',            path: `${t}/students` },
    { label: 'Coordinators',        path: `${t}/coordinators` },
    { label: 'Lecturers',           path: `${t}/lecturers` },
    { label: 'Assignments',         path: `${t}/lecturer-assignments` },
    { label: 'Lecturer Attendance', path: `${t}/lecturer-attendance` },
    { label: 'Employees',           path: `${t}/employees` },
    { label: 'Reports',             path: '/admin/reports' },
    { label: 'Venues',              path: `${t}/venues` },
    { label: 'Settings',            path: '/admin/settings' },
  ]
}

function Sidebar({ role, brand }: { role: Role; brand: Branding | null }) {
  const { user, logout } = useAuth()
  const { theme, toggle } = useTheme()
  const [pwOpen, setPwOpen] = useState(false)
  const links = role === 'ADMIN' ? adminNav(user?.tenantId ?? '') : (NAV[role] ?? [])
  const current = typeof window !== 'undefined' ? window.location.pathname : ''

  const name = brand?.name || 'QAAT'

  return (
    <aside style={{
      width: 220, flexShrink: 0, background: 'var(--sidebar)', color: 'var(--sidebar-text)',
      display: 'flex', flexDirection: 'column', padding: '24px 0',
      position: 'sticky', top: 0, alignSelf: 'flex-start', height: '100dvh', overflowY: 'auto',
    }}>
      <div style={{ padding: '0 20px 24px', borderBottom: '1px solid rgba(255,255,255,.12)' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          {brand?.logo_url
            ? <img src={brand.logo_url} alt={name} style={{ height: 64, width: 64, objectFit: 'contain', borderRadius: 6 }} />
            : <div style={{
                height: 64, width: 64, borderRadius: 6, background: 'var(--brand)',
                color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', fontWeight: 700,
              }}>{name.slice(0, 1)}</div>}
          <strong style={{ fontSize: 16 }}>{name}</strong>
        </div>
        {brand?.motto && <div style={{ fontSize: 11, opacity: .75, marginTop: 6, fontStyle: 'italic' }}>{brand.motto}</div>}
        <div style={{ fontSize: 12, opacity: .75, marginTop: 4 }}>{role.replace('_', ' ')}</div>
      </div>
      <nav style={{ flex: 1, padding: '16px 0', overflowY: 'auto' }}>
        {links.map(l => {
          const active = l.path === '/admin'
            ? current === '/admin'
            : current === l.path || current.startsWith(l.path + '/')
          return (
            <a key={l.path} href={l.path} style={{
              display: 'block', padding: '10px 20px', color: 'var(--sidebar-text)',
              opacity: active ? 1 : .82, textDecoration: 'none', fontSize: 14,
              fontWeight: active ? 700 : 400,
              background: active ? 'rgba(255,255,255,.16)' : 'transparent',
              borderLeft: active ? '3px solid var(--brand)' : '3px solid transparent',
            }}>
              {l.label}
            </a>
          )
        })}
      </nav>
      <div style={{ margin: '0 16px 8px' }}>
        <button onClick={() => setPwOpen(true)} style={{
          width: '100%', padding: '9px', background: 'transparent', color: 'var(--sidebar-text)',
          border: '1px solid rgba(255,255,255,.25)', borderRadius: 6, cursor: 'pointer', fontSize: 13,
        }}>
          Change password
        </button>
      </div>
      <div style={{ display: 'flex', gap: 8, margin: '0 16px', alignItems: 'center' }}>
        <button onClick={logout} style={{
          flex: 1, padding: '10px', background: 'rgba(255,255,255,.15)',
          color: 'var(--sidebar-text)', border: 'none', borderRadius: 6, cursor: 'pointer',
        }}>
          Sign out
        </button>
        <ThemeToggle theme={theme} toggle={toggle} />
      </div>
      {pwOpen && <ChangePasswordModal onClose={() => setPwOpen(false)} />}
    </aside>
  )
}

// Self-service password change — works for any signed-in role (POST verifies the
// current password and updates the hash everywhere).
function ChangePasswordModal({ onClose }: { onClose: () => void }) {
  const [cur, setCur] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [done, setDone] = useState(false)

  async function submit() {
    if (next.length < 8) { setErr('New password must be at least 8 characters.'); return }
    if (next !== confirm) { setErr('New passwords do not match.'); return }
    setBusy(true); setErr(null)
    try {
      await api.post('/api/v1/auth/change-password', { current_password: cur, new_password: next })
      setDone(true)
    } catch (e) { setErr(e instanceof Error ? e.message : 'Failed to change password') }
    finally { setBusy(false) }
  }

  return (
    <div onClick={onClose} style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 9999 }}>
      <div onClick={e => e.stopPropagation()} style={{ background: 'var(--surface,#fff)', color: 'var(--text,#0f172a)', borderRadius: 12, padding: 24, width: 360 }}>
        <h3 style={{ margin: '0 0 16px' }}>Change password</h3>
        {done ? (
          <>
            <p style={{ color: '#16a34a' }}>✓ Password changed. Use it next time you sign in.</p>
            <button onClick={onClose} style={pwBtn}>Close</button>
          </>
        ) : (
          <>
            {err && <div style={{ background: '#fef2f2', color: '#b91c1c', padding: '8px 12px', borderRadius: 6, marginBottom: 12, fontSize: 13 }}>{err}</div>}
            <PasswordInput placeholder="Current password" value={cur} onChange={e => setCur(e.target.value)} style={pwInp} />
            <PasswordInput placeholder="New password (min 8)" value={next} onChange={e => setNext(e.target.value)} style={pwInp} />
            <PasswordInput placeholder="Confirm new password" value={confirm} onChange={e => setConfirm(e.target.value)} style={pwInp} />
            <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
              <button onClick={submit} disabled={busy || !cur || !next} style={pwBtn}>{busy ? 'Saving…' : 'Update password'}</button>
              <button onClick={onClose} style={{ ...pwBtn, background: 'transparent', color: 'var(--text,#334155)', border: '1px solid var(--border,#e2e8f0)' }}>Cancel</button>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
const pwInp: React.CSSProperties = { width: '100%', padding: '10px', borderRadius: 6, border: '1px solid var(--border,#e2e8f0)', fontSize: 14, marginBottom: 10, boxSizing: 'border-box', background: 'var(--surface,#fff)', color: 'var(--text,#0f172a)' }
const pwBtn: React.CSSProperties = { padding: '10px 16px', background: 'var(--brand)', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }

// A back-button shown on every sub-page so the user can always go back to the
// previous page. Hidden on the base role route (e.g. /vc, /qa/live, /admin).
function GoBack({ navigate: nav, location: loc }: { navigate: ReturnType<typeof useNavigate>; location: ReturnType<typeof useLocation> }) {
  const baseRoutes = ['/vc', '/dqa/thresholds', '/qa/live', '/admin', '/lecturer']
  const isBase = baseRoutes.some(b => loc.pathname === b || loc.pathname === b + '/')
  if (isBase) return null
  return (
    <button onClick={() => nav(-1)} style={{
      background: 'none', border: 'none', color: 'var(--brand, #2563eb)', cursor: 'pointer',
      fontSize: 13, fontWeight: 600, padding: 0, marginBottom: 12, display: 'flex', alignItems: 'center', gap: 4,
    }}>
      ← Back
    </button>
  )
}
