import { describe, expect, it } from 'vitest'
import { orgKpis, type OrgOverviewResp } from './orgOverview'

describe('orgKpis', () => {
  it('reads numbers from data.kpis, not the document root', () => {
    const data: OrgOverviewResp = {
      kpis: {
        lecturers: 12,
        students: 400,
        courses: 8,
        units: 40,
        units_unstaffed: 2,
        sessions_held: 90,
        sessions_planned: 100,
        taught_rate: 90,
        avg_attendance: 81.5,
        at_risk: 17,
        threshold: 75,
      },
    }
    const k = orgKpis(data)
    expect(k?.students).toBe(400)
    expect(k?.at_risk).toBe(17)
    expect(k?.avg_attendance).toBe(81.5)
  })

  it('does not invent KPIs from a missing kpis object (the old DQA home bug)', () => {
    const data = { total_students: 400, below_threshold: 17 } as OrgOverviewResp
    expect(orgKpis(data)).toBeUndefined()
  })

  it('defaults threshold to 75 when the API omits it', () => {
    const k = orgKpis({
      kpis: {
        lecturers: 0, students: 0, courses: 0, units: 0, units_unstaffed: 0,
        sessions_held: 0, sessions_planned: 0, taught_rate: 0, avg_attendance: 0,
        at_risk: 0, threshold: 0,
      },
    })
    expect(k?.threshold).toBe(75)
  })
})
