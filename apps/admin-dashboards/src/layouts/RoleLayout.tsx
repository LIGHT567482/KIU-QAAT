import { Navigate, Outlet } from 'react-router-dom'
import { useAuth, type Role } from '../contexts/AuthContext'

interface RoleLayoutProps {
  allowedRoles: Role[]
}

// Wraps a route group — redirects to /login if unauthenticated or wrong role.
export function RoleLayout({ allowedRoles }: RoleLayoutProps) {
  const { user, isAuthenticated } = useAuth()

  if (!isAuthenticated || !user) {
    return <Navigate to="/login" replace />
  }
  if (!allowedRoles.includes(user.role)) {
    return <Navigate to="/unauthorized" replace />
  }
  return (
    <div style={{ display: 'flex', minHeight: '100vh', fontFamily: 'system-ui' }}>
      <Sidebar role={user.role} />
      <main style={{ flex: 1, padding: 24, background: '#f8fafc' }}>
        <Outlet />
      </main>
    </div>
  )
}

const NAV: Record<Role, { label: string; path: string }[]> = {
  VC: [
    { label: 'Overview', path: '/vc' },
    { label: 'Ghost Lectures', path: '/vc/ghost-lectures' },
    { label: 'Audit Reports', path: '/vc/reports' },
  ],
  DQA_DIRECTOR: [
    { label: 'Thresholds',     path: '/dqa/thresholds' },
    { label: 'Eligibility',    path: '/dqa/eligibility' },
  ],
  QA_OFFICER: [
    { label: 'Live Sessions',  path: '/qa/live' },
    { label: 'Device Reset',   path: '/qa/device-reset' },
  ],
  COORDINATOR: [
    { label: 'Session Status', path: '/coordinator' },
    { label: 'Sync Queue', path: '/coordinator/sync' },
  ],
  ADMIN: [
    { label: 'Tenants', path: '/admin/tenants' },
    { label: 'Users', path: '/admin/users' },
  ],
}

function Sidebar({ role }: { role: Role }) {
  const { logout } = useAuth()
  const links = NAV[role] ?? []

  return (
    <aside style={{
      width: 220, background: '#1e293b', color: '#e2e8f0',
      display: 'flex', flexDirection: 'column', padding: '24px 0',
    }}>
      <div style={{ padding: '0 20px 24px', borderBottom: '1px solid #334155' }}>
        <strong style={{ fontSize: 18 }}>QAAT</strong>
        <div style={{ fontSize: 12, color: '#94a3b8', marginTop: 4 }}>{role.replace('_', ' ')}</div>
      </div>
      <nav style={{ flex: 1, padding: '16px 0' }}>
        {links.map(l => (
          <a key={l.path} href={l.path} style={{
            display: 'block', padding: '10px 20px', color: '#cbd5e1',
            textDecoration: 'none', fontSize: 14,
          }}>
            {l.label}
          </a>
        ))}
      </nav>
      <button onClick={logout} style={{
        margin: '0 16px', padding: '10px', background: '#334155',
        color: '#e2e8f0', border: 'none', borderRadius: 6, cursor: 'pointer',
      }}>
        Sign out
      </button>
    </aside>
  )
}
