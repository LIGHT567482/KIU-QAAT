import { useEffect, useState } from 'react'
import { Navigate, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useAuth, type Role } from '../contexts/AuthContext'
import { api } from '../lib/api'
import PasswordInput from '../components/PasswordInput'
import { useTheme, ThemeToggle, applyPalette } from '../theme'
import brandDefault from '../brand.json'

interface Branding {
  name: string; logo_url: string; motto: string
  brand_color: string; sidebar_color: string; background_color: string; footer_color: string
  text_color_light?: string; text_color_dark?: string
}

// The bundled brand.json is the single source of truth for identity, so the sidebar
// shows the real KIU logo + name from the first frame (never the "Q" letter). The
// backend /branding fetch below simply confirms it.
const BUNDLED_BRAND = brandDefault as unknown as Branding

interface RoleLayoutProps {
  allowedRoles: Role[]
}

// Wraps a route group — redirects to /login if unauthenticated or wrong role.
export function RoleLayout({ allowedRoles }: RoleLayoutProps) {
  const { user, isAuthenticated } = useAuth()
  const [brand, setBrand] = useState<Branding | null>(BUNDLED_BRAND)
  const navigate = useNavigate()
  const location = useLocation()

  // Start on the bundled brand (instant logo + palette), then let the backend
  // /branding fetch confirm/refresh it. On failure we keep the bundled brand.
  useEffect(() => {
    applyPalette(BUNDLED_BRAND)
    if (!isAuthenticated) return
    api.get<Branding>('/api/v1/branding').then(b => { setBrand(b); applyPalette(b) }).catch(() => {})
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
        flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', background: 'var(--app-bg)', position: 'relative',
      }}>
        {/* Faint institution logo watermark, behind every page's content. */}
        {brand?.logo_url && (
          <img src={brand.logo_url} alt="" aria-hidden style={{
            position: 'fixed', top: '50%', left: 'calc(50% + 110px)', transform: 'translate(-50%, -50%)',
            width: '42vmin', maxWidth: 520, opacity: 0.05, pointerEvents: 'none', zIndex: 0, userSelect: 'none',
          }} />
        )}
        <main style={{ flex: 1, padding: 24, color: 'var(--text)', position: 'relative', zIndex: 1 }}>
          <GoBack navigate={navigate} location={location} />
          <Outlet />
        </main>
        <footer style={{ background: 'var(--footer)', color: 'var(--footer-text)', padding: '12px 24px', fontSize: 12, textAlign: 'center', position: 'relative', zIndex: 1 }}>
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
    { label: 'QA Reports',          path: '/dqa/qa-reports' },
    { label: 'Messages',            path: '/dqa/messages' },
  ],
  QA_OFFICER: [
    { label: 'QA Reports',          path: '/qa/reports' },
    { label: 'Timetable',           path: '/qa/timetable' },
    { label: 'Student Attendance',  path: '/qa/student-attendance' },
    { label: 'Lecturer Attendance', path: '/qa/lecturer-attendance' },
    { label: 'Manual Correction',   path: '/qa/correction' },
    { label: 'Coordinator Health',  path: '/qa/coordinator-health' },
    { label: 'Device Reset',        path: '/qa/device-reset' },
    { label: 'Messages',            path: '/qa/messages' },
  ],
  COORDINATOR: [],
  ADMIN: [], // built per-tenant in adminNav() — needs the admin's tenant_id.
  LECTURER: [
    { label: 'My Attendance', path: '/lecturer' },
  ],
  HOD: [
    { label: 'Department Lecturers', path: '/hod' },
  ],
  DEAN: [
    { label: 'School Lecturers', path: '/dean' },
  ],
  QA_DEPT_REP: [
    { label: 'My Department',  path: '/qa-dept' },
    { label: 'File QA Report', path: '/qa-dept/report' },
    { label: 'Messages',       path: '/qa-dept/messages' },
  ],
  QA_SCHOOL_HANDLER: [
    { label: 'School Overview', path: '/qa-school' },
    { label: 'Lecturers',       path: '/qa-school/lecturers' },
    { label: 'QA Reports',      path: '/qa-school/reports' },
    { label: 'Messages',        path: '/qa-school/messages' },
  ],
}

// The ADMIN sidebar lists EVERY management page so nothing is buried behind the
// home grid. The sub-resource pages are keyed by the admin's own tenant_id.
function adminNav(tenantId: string): NavLink[] {
  const t = `/admin/tenants/${tenantId}`
  return [
    { label: 'Home',                path: '/admin' },
    { label: 'Administration',      path: `${t}/users` },
    { label: 'Schools & Departments', path: `${t}/schools` },
    { label: 'Courses & Sessions',  path: `${t}/courses` },
    { label: 'Timetable',           path: '/admin/timetable' },
    { label: 'Students',            path: `${t}/students` },
    { label: 'Coordinators',        path: `${t}/coordinators` },
    { label: 'Lecturers',           path: `${t}/lecturers` },
    { label: 'Assignments',         path: `${t}/lecturer-assignments` },
    { label: 'Lecturer Attendance', path: `${t}/lecturer-attendance` },
    { label: 'Employees',           path: `${t}/employees` },
    { label: 'Reports',             path: '/admin/reports' },
    { label: 'Rooms & Codes',       path: `${t}/rooms` },
    { label: 'Settings',            path: '/admin/settings' },
  ]
}

function Sidebar({ role, brand }: { role: Role; brand: Branding | null }) {
  const { user, logout } = useAuth()
  const { theme, toggle } = useTheme()
  const [pwOpen, setPwOpen] = useState(false)
  const links = role === 'ADMIN' ? adminNav(user?.tenantId ?? '') : (NAV[role] ?? [])
  const current = typeof window !== 'undefined' ? window.location.pathname : ''

  // Unread-messages badge for the roles that have a Messages inbox. Polls, and refetches
  // on navigation (so it clears after the user reads their inbox).
  const [unread, setUnread] = useState(0)
  useEffect(() => {
    // Every role with a Messages inbox polls the badge — the three QA field roles share the
    // DQA channel, so they all have one.
    const withInbox: Role[] = ['DQA_DIRECTOR', 'QA_OFFICER', 'QA_DEPT_REP', 'QA_SCHOOL_HANDLER']
    if (!withInbox.includes(role)) return
    let alive = true
    const fetchUnread = () => api.get<{ unread: number }>('/api/v1/messages/unread-count')
      .then(r => { if (alive) setUnread(r.unread || 0) }).catch(() => {})
    fetchUnread()
    const id = setInterval(fetchUnread, 20000)
    return () => { alive = false; clearInterval(id) }
  }, [role, current])

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
              display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8,
              padding: '10px 20px', color: 'var(--sidebar-text)',
              opacity: active ? 1 : .82, textDecoration: 'none', fontSize: 14,
              fontWeight: active ? 700 : 400,
              background: active ? 'rgba(255,255,255,.16)' : 'transparent',
              borderLeft: active ? '3px solid var(--brand)' : '3px solid transparent',
            }}>
              <span>{l.label}</span>
              {l.path.endsWith('/messages') && unread > 0 && (
                <span style={{
                  background: '#ef4444', color: '#fff', borderRadius: 999, fontSize: 10,
                  fontWeight: 700, padding: '1px 7px', minWidth: 18, textAlign: 'center',
                }}>{unread > 99 ? '99+' : unread}</span>
              )}
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
// previous page. Hidden on the base role route (e.g. /vc, /qa/reports, /admin).
function GoBack({ navigate: nav, location: loc }: { navigate: ReturnType<typeof useNavigate>; location: ReturnType<typeof useLocation> }) {
  const baseRoutes = ['/vc', '/dqa/thresholds', '/qa/reports', '/admin', '/lecturer', '/hod', '/dean', '/qa-dept', '/qa-school']
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
