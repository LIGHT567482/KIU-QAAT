import { useNavigate } from 'react-router-dom'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'
import { Kpi, KpiRow, Section } from '../../components/Kpi'

/**
 * The header every org-scoped dashboard now opens with — HOD, dean, and both QA rep roles.
 *
 * Previously each of these roles landed straight on a list of lecturers with no sense of whether
 * the unit they are responsible for was actually working. These are the numbers they are judged on
 * and can act on, in the order the questions get asked: how big is my unit, is teaching happening,
 * and who is about to fail.
 *
 * The scope is NOT chosen here. It comes from the caller's own account server-side, so a dean
 * cannot point this at another college — see resolveOrgScope on the gateway.
 */

interface Kpis {
  lecturers: number; students: number; courses: number; units: number
  units_unstaffed: number
  sessions_held: number; sessions_planned: number
  taught_rate: number; avg_attendance: number
  at_risk: number; threshold: number
}
interface Resp {
  scope?: { department: string; school: string; label: string }
  window_days?: number
  kpis?: Kpis
  unset?: boolean
  message?: string
}

export default function OrgOverview({ level }: { level: 'hod' | 'dean' | 'qa-dept' | 'qa-school' }) {
  const nav = useNavigate()
  const { status, data } = useQuery<Resp>(() => api.get('/api/v1/org/overview'), [])

  const isDept = level === 'hod' || level === 'qa-dept'
  const unitWord = isDept ? 'department' : 'college / school'

  if (status === 'loading') return <p style={{ color: 'var(--muted)' }}>Loading…</p>

  // An org role with no unit set matches nothing rather than everything — say so plainly instead
  // of showing a page of zeroes that looks like an empty institution.
  if (data?.unset) {
    return (
      <div style={warnBox}>
        <strong>Your account has no {unitWord} set.</strong>
        <p style={{ margin: '6px 0 0', fontSize: 13 }}>{data.message}</p>
      </div>
    )
  }

  const k = data?.kpis
  if (!k) return <p style={{ color: 'var(--muted)' }}>Nothing to show yet.</p>

  const threshold = k.threshold || 75
  const attendanceTone = k.avg_attendance >= threshold ? 'good' : 'bad'
  const taughtTone = k.taught_rate >= 90 ? 'good' : k.taught_rate >= 70 ? 'warn' : 'bad'

  return (
    <div>
      <h2 style={{ margin: '0 0 4px' }}>{data?.scope?.label || `Your ${unitWord}`}</h2>
      <p style={{ color: 'var(--muted)', margin: '0 0 18px', fontSize: 13 }}>
        Everything below is scoped to your {unitWord}. Teaching figures cover the last{' '}
        {data?.window_days ?? 90} days.
      </p>

      <Section
        title="Your unit" hint="who and what falls under you"
        right={
          // A dean manages through their heads of department, so the first thing their overview
          // offers is the way in to them — not another flat list of lecturers.
          !isDept
            ? <button style={linkBtn} onClick={() => nav(`/${level}/departments`)}>Departments &amp; HODs →</button>
            : <button style={linkBtn} onClick={() => nav(`/${level}/lecturers`)}>My lecturers →</button>
        }
      >
        <KpiRow>
          <Kpi label="Lecturers" value={k.lecturers} />
          <Kpi label="Students" value={k.students} sub="active enrolments" />
          <Kpi label="Courses" value={k.courses} />
          <Kpi label="Course units" value={k.units} />
        </KpiRow>
      </Section>

      <Section title="Is teaching happening?" hint="sessions actually held against the timetable">
        <KpiRow>
          <Kpi
            label="Classes taught" tone={taughtTone}
            value={`${k.taught_rate.toFixed(0)}%`}
            sub={`${k.sessions_held} held of ~${k.sessions_planned} timetabled`}
          />
          <Kpi label="Sessions held" value={k.sessions_held} sub={`last ${data?.window_days ?? 90} days`} />
          {/* An unstaffed unit is invisible everywhere else: blank lecturer on the student's
              timetable, nobody named on the patrol manifest. It is a gap only someone at this
              level will notice, so it is a tile rather than a footnote. */}
          <Kpi
            label="Units with no lecturer"
            value={k.units_unstaffed}
            tone={k.units_unstaffed > 0 ? 'bad' : 'good'}
            sub={k.units_unstaffed > 0 ? 'nobody assigned to teach these' : 'every unit is staffed'}
          />
        </KpiRow>
      </Section>

      <Section
        title="Who is at risk?"
        hint={`the exam-eligibility bar is ${threshold}%`}
        right={
          <button style={linkBtn} onClick={() => nav(`/${level}/at-risk`)}>
            See the watchlist →
          </button>
        }
      >
        <KpiRow>
          <Kpi
            label="Average attendance" tone={attendanceTone}
            value={`${k.avg_attendance.toFixed(1)}%`}
            sub={`threshold ${threshold}%`}
          />
          <Kpi
            label="Students below the bar"
            value={k.at_risk}
            tone={k.at_risk > 0 ? 'bad' : 'good'}
            sub={k.at_risk > 0 ? 'will lose exam eligibility' : 'nobody is below the threshold'}
            onClick={() => nav(`/${level}/at-risk`)}
          />
        </KpiRow>
      </Section>
    </div>
  )
}

const warnBox: React.CSSProperties = {
  background: 'rgba(180,83,9,.08)', border: '1px solid rgba(180,83,9,.3)',
  borderRadius: 10, padding: '14px 16px', color: '#92400e',
}
const linkBtn: React.CSSProperties = {
  border: 'none', background: 'transparent', color: 'var(--brand)',
  cursor: 'pointer', fontSize: 13, fontWeight: 600, padding: 0,
}
