import type { CSSProperties } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { orgKpis, type OrgOverviewResp } from '../lib/orgOverview'
import { useQuery } from '../lib/useApi'
import { Kpi, KpiRow, Section } from './Kpi'

/**
 * Institution (or org-unit) scalars from GET /api/v1/org/overview.
 * Shared by Admin Home and DQA Home so both read `data.kpis`, not a flat
 * document the gateway has never returned.
 */
export default function OrgKpiHeader({ atRiskPath }: { atRiskPath: string }) {
  const nav = useNavigate()
  const { status, data, message, refetch } = useQuery<OrgOverviewResp>(
    () => api.get('/api/v1/org/overview'),
  )

  if (status === 'loading') {
    return <p style={{ color: 'var(--muted)', fontSize: 13 }}>Loading attendance…</p>
  }
  if (status === 'error') {
    return (
      <p style={{ color: '#b91c1c', fontSize: 13 }}>
        Could not load overview{message ? `: ${message}` : ''}.{' '}
        <button type="button" onClick={refetch} style={retry}>Retry</button>
      </p>
    )
  }

  const k = orgKpis(data)
  if (!k) {
    return <p style={{ color: 'var(--muted)', fontSize: 13 }}>Nothing to show yet.</p>
  }

  const windowDays = data?.window_days ?? 90
  const threshold = k.threshold
  const attendanceTone = k.avg_attendance >= threshold ? 'good' : 'bad'
  const taughtTone = k.taught_rate >= 90 ? 'good' : k.taught_rate >= 70 ? 'warn' : 'bad'

  return (
    <>
      <Section title="The institution right now" hint={`last ${windowDays} days · ${data?.scope?.label || 'institution-wide'}`}>
        <KpiRow>
          <Kpi label="Students" value={k.students} sub="active enrolments" />
          <Kpi label="Lecturers" value={k.lecturers} />
          <Kpi label="Courses" value={k.courses} />
          <Kpi label="Course units" value={k.units} />
        </KpiRow>
      </Section>

      <Section title="Is teaching happening?" hint="sessions actually held against the timetable, plus U-Panel roll-ups when QAAT sessions are empty">
        <KpiRow>
          <Kpi
            label="Classes taught" tone={taughtTone}
            value={`${k.taught_rate.toFixed(0)}%`}
            sub={`${k.sessions_held} held of ~${k.sessions_planned} timetabled`}
          />
          <Kpi label="Sessions held" value={k.sessions_held} sub={`last ${windowDays} days`} />
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
          <button type="button" style={linkBtn} onClick={() => nav(atRiskPath)}>
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
            onClick={() => nav(atRiskPath)}
          />
        </KpiRow>
      </Section>
    </>
  )
}

const retry: CSSProperties = {
  border: 'none', background: 'transparent', color: 'var(--brand)',
  cursor: 'pointer', fontSize: 13, fontWeight: 600, padding: 0,
}
const linkBtn: CSSProperties = retry
