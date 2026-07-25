import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'
import { useAuth } from '../../contexts/AuthContext'

// Tenant ADMIN home — scoped to the admin's OWN institution (tenant_id from JWT).
// The academic period control lives on the Administration page (less accidental
// change); here the ADMIN sees a greeting + day-to-day shortcuts.
interface TenantInfo {
  tenant_id: string
  name: string
  motto: string
  active_academic_year: string
  active_semester: number
}

export default function AdminHome() {
  const { user } = useAuth()
  const tenantId = user?.tenantId ?? ''
  const { status, data } = useQuery<TenantInfo>(() => api.get('/api/v1/branding'))
  const info = status === 'ok' ? data : undefined

  const links: { label: string; href: string; desc: string }[] = [
    { label: 'Courses',     href: `/admin/tenants/${tenantId}/courses`,      desc: 'Courses, levels & cohorts' },
    { label: 'Students',    href: `/admin/tenants/${tenantId}/students`,     desc: 'Enrolment records' },
    { label: 'Lecturers',   href: `/admin/tenants/${tenantId}/lecturers`,    desc: 'Lecturer directory' },
    { label: 'Coordinators',href: `/admin/tenants/${tenantId}/coordinators`, desc: 'Directory, contacts & cohorts' },
    { label: 'Employees',   href: `/admin/tenants/${tenantId}/employees`,    desc: 'Staff registry & tablet attendance' },
  ]

  const now = new Date()
  const hr = now.getHours()
  const partOfDay = hr < 12 ? 'morning' : hr < 17 ? 'afternoon' : hr < 21 ? 'evening' : 'night'
  const dateStr = now.toLocaleDateString(undefined, { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })
  const timeStr = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })

  return (
    <div style={{ color: 'var(--text)' }}>
      <div style={{ background: 'linear-gradient(135deg, var(--brand), color-mix(in srgb, var(--brand) 70%, #000))', color: '#fff', borderRadius: 14, padding: '20px 24px', marginBottom: 20 }}>
        <h2 style={{ margin: 0, fontSize: 22 }}>Good {partOfDay} 👋</h2>
        <p style={{ margin: '6px 0 0', opacity: .9, fontSize: 14 }}>
          Welcome back to {info?.name ?? 'your institution'}. It's {dateStr} · {timeStr}.
        </p>
      </div>

      <h2 style={{ margin: '0 0 4px' }}>{info?.name ?? 'My Institution'}</h2>
      <p style={{ color: 'var(--muted)', marginTop: 0 }}>{info?.motto || 'Manage your institution'}</p>

      <div style={{ fontSize: 13, color: 'var(--muted)', margin: '8px 0 20px' }}>
        Active period: {info?.active_academic_year
          ? <strong style={{ color: 'var(--text)' }}>{info.active_academic_year}</strong>
          : <span style={{ color: '#b45309' }}>not set</span>} · advanced by semester under <strong>Administration</strong>.
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: 16 }}>
        {links.map(l => (
          <a key={l.href} href={l.href} style={{
            display: 'block', padding: 18, background: 'var(--surface)', border: '1px solid var(--border)',
            borderRadius: 10, textDecoration: 'none', color: 'var(--text)',
          }}>
            <div style={{ fontWeight: 700, fontSize: 15 }}>{l.label}</div>
            <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 4 }}>{l.desc}</div>
          </a>
        ))}
      </div>
    </div>
  )
}
