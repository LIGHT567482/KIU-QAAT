import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'
import { Kpi, KpiRow, Section } from '../../components/Kpi'
import OverviewAnalytics from '../../components/OverviewAnalytics'
import DQATrends from './DQATrends'

/**
 * The DQA director's home.
 *
 * They used to land on /dqa/thresholds — a settings form, and the only oversight
 * role whose front door was a config screen. This is the home the admin already
 * had: the state of the institution first, then shortcuts into the reports.
 *
 * WHAT IS DELIBERATELY ABSENT. The admin home's six "Manage" tiles — Schools &
 * Departments, Courses, Students, Lecturers, Employees, Coordinators — are not
 * here. The DQA directs quality assurance; they do not maintain the registry, and
 * every one of those screens is still ADMIN-only behind its route guard. Copying
 * the tiles across would have produced six links that render an Unauthorized page.
 */
interface TenantInfo {
  name: string
  motto: string
  active_academic_year: string
  active_semester: number
}

interface Overview {
  total_students?: number
  total_units?: number
  sessions_held?: number
  below_threshold?: number
  attendance_rate?: number
}

export default function DQAHome() {
  const { data: info } = useQuery<TenantInfo>(() => api.get('/api/v1/branding'))
  // The same institution-wide rollup the course-health page reads; showing it here
  // means the director sees the number before choosing which report explains it.
  const { data: ov, status } = useQuery<Overview>(() => api.get('/api/v1/org/overview'))

  const links: { label: string; href: string; desc: string }[] = [
    { label: 'Reports',            href: '/dqa/reports',             desc: 'Every report the directorate can generate' },
    { label: 'Eligibility',        href: '/dqa/eligibility',         desc: 'Who may sit exams, and who is short' },
    { label: 'Unit Attendance',    href: '/dqa/unit-attendance',     desc: 'Teaching and turnout for the same unit' },
    { label: 'At-risk Students',   href: '/dqa/at-risk',             desc: 'Who is falling below the bar you set' },
    { label: 'Course Health',      href: '/dqa/course-health',       desc: 'Attendance by course and unit' },
    { label: 'By College',         href: '/dqa/org',                 desc: 'Roll-up per college, then per department' },
    { label: 'Punctuality',        href: '/dqa/punctuality',         desc: 'Late starts and short lectures' },
    { label: 'Lecturer Attendance',href: '/dqa/lecturer-attendance', desc: 'Coordinator record and QA monitor record' },
    { label: 'QA Monitor Coverage', href: '/dqa/monitor-coverage',   desc: 'How much of the week the QA round reached' },
    { label: 'Student Attendance', href: '/dqa/student-attendance',  desc: 'Per-student detail and corrections' },
    { label: 'Employee Attendance',href: '/dqa/employee-attendance', desc: 'Support-staff terminal records' },
    { label: 'QA Reports',         href: '/dqa/qa-reports',          desc: 'Submissions filed by QA staff' },
    { label: 'Timetable',          href: '/dqa/timetable',           desc: 'The published week, read-only' },
  ]

  return (
    <div style={{ color: 'var(--text)' }}>
      <h2 style={{ margin: '0 0 4px' }}>{info?.name ?? 'Quality Assurance'}</h2>
      <p style={{ color: 'var(--muted)', marginTop: 0 }}>{info?.motto || 'Directorate of Quality Assurance'}</p>

      <div style={{ fontSize: 13, color: 'var(--muted)', margin: '8px 0 20px' }}>
        Active period: {info?.active_academic_year
          ? <strong style={{ color: 'var(--text)' }}>{info.active_academic_year}</strong>
          : <span style={{ color: '#b45309' }}>not set</span>}
        {info?.active_semester ? ` · Semester ${info.active_semester}` : ''}
      </div>

      <Section title="The institution right now">
        <KpiRow>
          <Kpi label="Students"        value={status === 'ok' ? (ov?.total_students ?? 0) : '—'} />
          <Kpi label="Units"           value={status === 'ok' ? (ov?.total_units ?? 0) : '—'} />
          <Kpi label="Sessions held"   value={status === 'ok' ? (ov?.sessions_held ?? 0) : '—'} />
          <Kpi label="Below 75%"       value={status === 'ok' ? (ov?.below_threshold ?? 0) : '—'} />
        </KpiRow>
      </Section>

      {/* Direction and location, beneath the four scalars that say only "what". The directorate's
          scope is the institution, so these cover every college. */}
      <OverviewAnalytics />

      <h3 style={{ margin: '26px 0 10px', fontSize: 16 }}>Reports</h3>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: 16 }}>
        {links.map(l => (
          <a key={l.href} href={l.href}
            style={{
              display: 'block', padding: 16, borderRadius: 12, textDecoration: 'none',
              background: 'var(--surface, #fff)', border: '1px solid var(--border, #e2e8f0)',
              color: 'var(--text)',
            }}>
            <div style={{ fontWeight: 700, marginBottom: 4 }}>{l.label}</div>
            <div style={{ fontSize: 12, color: 'var(--muted)' }}>{l.desc}</div>
          </a>
        ))}
      </div>

      {/* Trends used to be its own sidebar entry, which put the question "how is attendance
          moving" one click away from the four scalars that answer "where is it now" — and the
          two are only meaningful read together. A single figure with no direction is a number
          the director has to remember last week's value to interpret. */}
      <div style={{ marginTop: 30 }}>
        <DQATrends />
      </div>
    </div>
  )
}
