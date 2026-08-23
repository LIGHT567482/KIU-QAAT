/** Shape of GET /api/v1/org/overview. The numbers live under `kpis`, not at the root. */

export interface OrgOverviewKpis {
  lecturers: number
  students: number
  courses: number
  units: number
  units_unstaffed: number
  sessions_held: number
  sessions_planned: number
  taught_rate: number
  avg_attendance: number
  at_risk: number
  threshold: number
}

export interface OrgOverviewResp {
  scope?: { department: string; school: string; label: string }
  window_days?: number
  kpis?: OrgOverviewKpis
  unset?: boolean
  message?: string
}

export function orgKpis(data?: OrgOverviewResp | null): OrgOverviewKpis | undefined {
  const k = data?.kpis
  if (!k) return undefined
  return {
    lecturers: k.lecturers ?? 0,
    students: k.students ?? 0,
    courses: k.courses ?? 0,
    units: k.units ?? 0,
    units_unstaffed: k.units_unstaffed ?? 0,
    sessions_held: k.sessions_held ?? 0,
    sessions_planned: k.sessions_planned ?? 0,
    taught_rate: k.taught_rate ?? 0,
    avg_attendance: k.avg_attendance ?? 0,
    at_risk: k.at_risk ?? 0,
    threshold: k.threshold || 75,
  }
}
