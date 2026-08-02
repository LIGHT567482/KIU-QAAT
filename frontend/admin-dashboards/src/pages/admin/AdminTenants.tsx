import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'
import { useAuth } from '../../contexts/AuthContext'
import { Kpi, KpiRow, Section } from '../../components/Kpi'

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
    { label: 'Schools & Departments', href: `/admin/tenants/${tenantId}/schools`, desc: 'Add schools/colleges and their departments' },
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

      {/* The state of the institution, not just links to the screens that manage it. */}
      <AdminPulse tenantId={tenantId} />

      <h3 style={{ margin: '26px 0 10px', fontSize: 16 }}>Manage</h3>
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

interface Overview {
  accounts_by_role: Record<string, number>
  setup: Record<string, number>
  activity: Record<string, number>
  gaps: Record<string, number>
}

/**
 * What the institution is actually doing, and what is quietly broken in it.
 *
 * The admin home used to be six navigation tiles — it told you where the screens were and nothing
 * about whether anything was working. The "Needs attention" row is the reason this exists: every
 * number in it is a SILENT failure somewhere else in the system. An unstaffed unit shows a blank
 * lecturer on the student's timetable and reaches the patrol manifest with nobody named against it.
 * A student with no cohort is invisible to their own coordinator's roster. A cohort with no
 * coordinator can never have a session opened for it at all. None of these raise an error
 * anywhere — they just do nothing — so the only way anyone finds them is by being shown them.
 */
function AdminPulse({ tenantId }: { tenantId: string }) {
  const { status, data } = useQuery<Overview>(() => api.get('/api/v1/admin/overview'))
  if (status === 'loading') return <p style={{ color: 'var(--muted)', fontSize: 13 }}>Loading institution status…</p>
  if (status === 'error' || !data) return null

  const g = data.gaps ?? {}
  const a = data.activity ?? {}
  const s = data.setup ?? {}

  // Ordered by how badly each one silently breaks something, worst first.
  const gapTiles: { label: string; key: string; sub: string; href?: string }[] = [
    { label: 'Units with no lecturer', key: 'units_unstaffed', sub: 'blank on timetables & patrol', href: `/admin/tenants/${tenantId}/lecturer-assignments` },
    { label: 'Cohorts with no coordinator', key: 'cohorts_uncoordinated', sub: 'no session can be opened', href: `/admin/tenants/${tenantId}/coordinators` },
    // The org tree breaking away from the academic data. Both directions are silent: the HOD sees
    // a blank dashboard, the dean sees a department that reports nothing, and neither can tell why.
    { label: 'Departments with no HOD', key: 'departments_no_hod', sub: 'nobody answerable for them', href: `/admin/tenants/${tenantId}/users` },
    { label: 'Departments not on any course', key: 'departments_unlinked', sub: 'their HOD sees a blank dashboard', href: `/admin/tenants/${tenantId}/courses` },
    { label: 'Courses naming an unknown department', key: 'courses_orphan_department', sub: 'they belong to no HOD or dean', href: `/admin/tenants/${tenantId}/courses` },
    { label: 'Students with no cohort', key: 'students_no_cohort', sub: 'invisible to their coordinator', href: `/admin/tenants/${tenantId}/students` },
    { label: 'Org roles with no unit', key: 'org_roles_unscoped', sub: 'their dashboards show nothing', href: `/admin/tenants/${tenantId}/users` },
    { label: 'Still on default password', key: 'accounts_default_password', sub: 'never signed in and changed it' },
    { label: 'Patrollers with no handset', key: 'patrollers_unbound', sub: 'cannot start a round' },
    { label: 'Sessions not synced', key: 'sessions_unsynced', sub: 'attendance still on a phone' },
  ]
  const openGaps = gapTiles.filter(t => (g[t.key] ?? 0) > 0)

  return (
    <>
      <Section title="Today" hint="live activity across the institution">
        <KpiRow>
          <Kpi label="Sessions live now" value={a.sessions_live ?? 0} tone={(a.sessions_live ?? 0) > 0 ? 'good' : 'neutral'} />
          <Kpi label="Sessions today" value={a.sessions_today ?? 0} />
          <Kpi label="Check-ins today" value={a.checkins_today ?? 0} sub="student attendance marks" />
          <Kpi label="Lecturer gate-ins today" value={a.lecturer_gates_today ?? 0} sub="lecturers who started a class" />
          <Kpi label="Sessions this week" value={a.sessions_week ?? 0} />
          <Kpi label="Patrols this week" value={a.patrols_week ?? 0} sub="QA spot-checks" />
        </KpiRow>
      </Section>

      <Section
        title="Needs attention"
        hint={openGaps.length === 0 ? 'nothing outstanding' : 'each of these silently breaks something else'}
      >
        {openGaps.length === 0 ? (
          <div style={{
            background: 'rgba(22,101,52,.07)', border: '1px solid rgba(22,101,52,.25)',
            borderRadius: 10, padding: '12px 14px', fontSize: 13, color: '#166534',
          }}>
            Nothing outstanding — every unit is staffed, every cohort has a coordinator, and every
            student is on one.
          </div>
        ) : (
          <KpiRow>
            {openGaps.map(t => (
              <Kpi
                key={t.key} label={t.label} value={g[t.key]} tone="bad" sub={t.sub}
                onClick={t.href ? () => { window.location.href = t.href! } : undefined}
              />
            ))}
          </KpiRow>
        )}
      </Section>

      <Section title="Set up" hint="what exists in the institution">
        <KpiRow>
          <Kpi label="Students" value={s.students ?? 0} sub="active enrolments" />
          <Kpi label="Lecturers" value={s.lecturers ?? 0} />
          <Kpi label="Employees" value={s.employees ?? 0} sub="non-teaching staff" />
          <Kpi label="Courses" value={s.courses ?? 0} />
          <Kpi label="Course units" value={s.units ?? 0} />
          <Kpi label="Cohorts" value={s.cohorts ?? 0} sub="course offerings" />
          <Kpi label="Schools" value={s.schools ?? 0} sub={`${s.departments ?? 0} departments`} />
          <Kpi label="Timetable slots" value={s.timetable_slots ?? 0} sub={`${s.rooms ?? 0} rooms`} />
        </KpiRow>
      </Section>
    </>
  )
}
