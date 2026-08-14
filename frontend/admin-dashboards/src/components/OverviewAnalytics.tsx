import { useState } from 'react'
import {
  ResponsiveContainer, LineChart, Line, BarChart, Bar, PieChart, Pie, Cell,
  XAxis, YAxis, CartesianGrid, Tooltip, Legend, LabelList,
} from 'recharts'
import { api } from '../lib/api'
import { useQuery } from '../lib/useApi'

/**
 * THE OVERVIEW'S THREE CHARTS, shared by the DQA director, deans and heads of department.
 *
 * One component for all three because the DATA is already one endpoint: /api/v1/org/analytics
 * resolves the caller's own college or department from their account, so a dean's charts cover
 * their college and the directorate's cover the institution without either page knowing which.
 *
 * WHY THESE THREE FORMS. Each was chosen from the job the reader has, not from a wish for variety:
 *
 *   Line — direction. "Is this getting better or worse" is a question about time, and a scalar
 *          cannot answer it. Two series, both percentages, so they share ONE axis; a second y-scale
 *          would invent a correlation that is not in the data.
 *   Bar  — location. "Where is the problem" is a comparison across departments (or units, for a
 *          head of one), and a bar is the form people read most accurately.
 *   Pie  — composition. The term's lectures split three ways, at a glance. Three slices, direct
 *          percentages, because a pie is only honest for part-to-whole with few segments.
 *
 * COLOR IS ASSIGNED BY JOB AND WAS VALIDATED, NOT EYEBALLED. The line's two series and the pie's
 * three slices use categorical slots 1–3 (blue / orange / aqua), which pass the colorblind
 * separation, lightness-band, chroma and contrast checks in both light and dark mode — the pie
 * against the all-pairs list, since a reader compares any two slices, not just neighbours.
 *
 * An earlier draft coloured the pie green-for-taught and red-for-not-taught, which is the obvious
 * choice and wrong: that pair measures ΔE 4.1 under deuteranopia — the two commonest slices of the
 * chart would have been the same colour for a reader with red-green colour blindness. It is exactly
 * the mistake that looks fine to the person drawing it.
 *
 * The bar chart is ONE colour for every bar. Shading bars darker-where-taller double-encodes the
 * length the reader can already see, and spends the only free channel on nothing.
 */

// Categorical slots 1–3, light and dark steps. Applied by ROLE (series identity), never by rank —
// a filter that drops a department must not repaint the survivors.
const SERIES = {
  attendance: { light: '#2a78d6', dark: '#3987e5' }, // slot 1 — blue
  taught: { light: '#eb6834', dark: '#d95926' },     // slot 2 — orange
  third: { light: '#1baf7a', dark: '#199e70' },      // slot 3 — aqua
  fourth: { light: '#eda100', dark: '#c98500' },     // slot 4 — yellow
}

// The four ways a working day can end, in the order they are stacked. The ORDER is the
// colour-safety mechanism, not a layout preference: a stacked bar is validated on the ADJACENT
// pairlist, so slots 1→4 must appear in slot order for the pairs the reader actually compares to
// be the pairs that were checked. Running the validator over exactly this sequence gives worst
// adjacent CVD ΔE 9.1 light / 8.4 dark (yellow↔aqua), clear of the ≥8 target in both modes.
const DAY_BUCKETS = [
  { key: 'on_time', label: 'On time',    slot: 'attendance' },
  { key: 'late',    label: 'Arrived late', slot: 'taught' },
  { key: 'early',   label: 'Left early',   slot: 'third' },
  { key: 'absent',  label: 'Absent',       slot: 'fourth' },
] as const

interface Week { week_start: string; sessions: number; taught: number; attendance_pct: number; taught_pct: number }
interface Group { name: string; sessions: number; attendance_pct: number }
interface Analytics {
  weeks: number
  group_by: string
  trend: Week[]
  by_group: Group[]
  outcomes: { taught: number; not_taught: number; no_record: number }
  session_total: number
  employee_time: EmployeeDept[]
  employee_days: number
  employee_on_time: number
}
interface EmployeeDept {
  name: string; days: number; employees: number
  on_time: number; late: number; early: number; absent: number
  on_time_pct: number
}

function useDark() {
  // The dashboards stamp data-theme on the root; fall back to the OS setting.
  if (typeof document === 'undefined') return false
  const stamped = document.documentElement.getAttribute('data-theme')
  if (stamped === 'dark') return true
  if (stamped === 'light') return false
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ?? false
}

export default function OverviewAnalytics() {
  const [weeks, setWeeks] = useState(12)
  const { status, data } = useQuery<Analytics>(
    () => api.get(`/api/v1/org/analytics?weeks=${weeks}`),
    [weeks],
  )
  const dark = useDark()
  const k = dark ? 'dark' : 'light'
  const ink = dark ? '#c3c2b7' : '#52514e'
  const grid = dark ? '#2e2e2c' : '#ececea'

  if (status === 'loading') return <p style={{ color: 'var(--muted)' }}>Loading charts…</p>
  if (status === 'error' || !data) return null

  const { trend, by_group: groups, outcomes, session_total: total } = data

  // A term with no sessions has nothing to plot, and three empty axes would imply otherwise.
  if (total === 0) {
    return (
      <div style={card}>
        <p style={{ color: 'var(--muted)', fontSize: 13, margin: 0 }}>
          No sessions in the last {weeks} weeks, so there is nothing to chart yet.
        </p>
      </div>
    )
  }

  const pie = [
    { name: 'Taught', value: outcomes.taught, fill: SERIES.attendance[k] },
    { name: 'Not taught', value: outcomes.not_taught, fill: SERIES.taught[k] },
    { name: 'No lecturer record', value: outcomes.no_record, fill: SERIES.third[k] },
  ].filter(s => s.value > 0)

  // Older responses predate this series, and a dashboard that throws on a field the server has
  // not shipped yet is a worse failure than one chart being absent.
  const emp = data.employee_time ?? []
  const empOnTimePct = data.employee_days > 0
    ? Math.round((data.employee_on_time / data.employee_days) * 1000) / 10
    : 0

  const axis = { stroke: grid, tick: { fill: ink, fontSize: 11 } }
  const tooltip = {
    contentStyle: {
      background: dark ? '#1a1a19' : '#fcfcfb',
      border: `1px solid ${grid}`, borderRadius: 8, fontSize: 12,
      color: dark ? '#fff' : '#0b0b0b',
    },
  }

  return (
    <div style={{ margin: '4px 0 24px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 10, flexWrap: 'wrap', gap: 8 }}>
        <h3 style={{ margin: 0, fontSize: 15 }}>Analytics</h3>
        {/* Filters in one row above the charts, and the range applies to all three so they always
            describe the same window. */}
        <select value={weeks} onChange={e => setWeeks(Number(e.target.value))} style={sel}>
          <option value={6}>Last 6 weeks</option>
          <option value={12}>Last 12 weeks</option>
          <option value={24}>Last 24 weeks</option>
        </select>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: 14 }}>
        {/* ── Line: direction over time ─────────────────────────────────── */}
        <div style={card}>
          <ChartTitle>Attendance and teaching, week by week</ChartTitle>
          <ChartNote>Both are percentages, so they share one axis.</ChartNote>
          <ResponsiveContainer width="100%" height={220}>
            <LineChart data={trend} margin={{ top: 8, right: 12, left: -18, bottom: 0 }}>
              <CartesianGrid stroke={grid} vertical={false} />
              <XAxis dataKey="week_start" {...axis} tickFormatter={shortDate} />
              <YAxis {...axis} domain={[0, 100]} unit="%" />
              <Tooltip {...tooltip} formatter={(v: number) => `${v}%`} labelFormatter={l => `Week of ${shortDate(String(l))}`} />
              <Legend wrapperStyle={{ fontSize: 12, color: ink }} />
              {/* isAnimationActive={false} on every mark in this file, and not for taste: Recharts
                  animates from a zero-height/zero-length state, so a chart captured before the
                  animation settles renders its axes and NOTHING else. That is how these three
                  charts first came out — fully drawn frames with no data in them. It also means a
                  reader waits ~1s to read a dashboard, which is the wrong trade for a report. */}
              <Line type="monotone" dataKey="attendance_pct" name="Student attendance"
                stroke={SERIES.attendance[k]} strokeWidth={2} dot={{ r: 3 }} activeDot={{ r: 5 }}
                isAnimationActive={false} />
              <Line type="monotone" dataKey="taught_pct" name="Lectures delivered"
                stroke={SERIES.taught[k]} strokeWidth={2} dot={{ r: 3 }} activeDot={{ r: 5 }}
                isAnimationActive={false} />
            </LineChart>
          </ResponsiveContainer>
        </div>

        {/* ── Pie: part-to-whole ────────────────────────────────────────── */}
        <div style={card}>
          <ChartTitle>What happened to {total} timetabled lectures</ChartTitle>
          <ChartNote>“No lecturer record” is neither taught nor missed — it is unevidenced.</ChartNote>
          <ResponsiveContainer width="100%" height={220}>
            <PieChart>
              {/* Direct percentage labels are not decoration here: aqua sits below 3:1 on the light
                  surface, and visible labels are the required relief. */}
              <Pie data={pie} dataKey="value" nameKey="name" innerRadius={44} outerRadius={78}
                paddingAngle={2} stroke={dark ? '#1a1a19' : '#fcfcfb'} strokeWidth={2}
                isAnimationActive={false}
                // A slice under 4% cannot hold a legible label inside the ring, and printing one
                // anyway is what produces the overlapping "1%" floating clear of its own arc. Small
                // slices are named in the legend and the tooltip instead.
                label={({ value }: { value: number }) =>
                  value / total >= 0.04 ? `${Math.round((value / total) * 100)}%` : ''}
                labelLine={false}>
                {pie.map(s => <Cell key={s.name} fill={s.fill} />)}
              </Pie>
              <Tooltip {...tooltip} formatter={(v: number, n) => [`${v} lecture(s)`, n]} />
              <Legend wrapperStyle={{ fontSize: 12, color: ink }} />
            </PieChart>
          </ResponsiveContainer>
        </div>

        {/* ── Bar: where ────────────────────────────────────────────────── */}
        <div style={{ ...card, gridColumn: '1 / -1' }}>
          <ChartTitle>
            Student attendance by {data.group_by === 'unit_name' ? 'unit' : 'department'}
          </ChartTitle>
          <ChartNote>One colour for every bar — the length already carries the magnitude.</ChartNote>
          <ResponsiveContainer width="100%" height={Math.max(200, groups.length * 34)}>
            <BarChart data={groups} layout="vertical" margin={{ top: 4, right: 52, left: 8, bottom: 4 }}>
              {/* Recessive: the gridline is a reading aid behind the data, and at full strength it
                  read as a mark in its own right cutting across every bar. */}
              <CartesianGrid stroke={grid} strokeOpacity={0.6} horizontal={false} />
              <XAxis type="number" domain={[0, 100]} unit="%" {...axis} />
              {/* Horizontal bars because department names are long — rotated labels are unreadable.
                  Recharts wraps a long category label onto three lines and lets it run into its
                  neighbour, which is what "Computer Science and Information Technology" did to
                  "Business Administration and Management". One line, elided, with the full name in
                  the tooltip. */}
              {/* 16 characters, arrived at by rendering and measuring rather than by guessing:
                  Recharts breaks a tick label on WORD boundaries, so "Computer Science an…" still
                  took two lines inside the 168px gutter even though it fit by width. The full name
                  is in the tooltip, and the bar's own length is the thing being read here. */}
              <YAxis type="category" dataKey="name" width={168} {...axis} interval={0}
                tickFormatter={(v: string) => (v.length > 16 ? v.slice(0, 15) + '…' : v)} />
              <Tooltip {...tooltip} formatter={(v: number, _n, p) => [`${v}% · ${p.payload.sessions} session(s)`, 'Attendance']}
                labelFormatter={(l) => String(l)} />
              <Bar dataKey="attendance_pct" name="Attendance" fill={SERIES.attendance[k]}
                radius={[0, 4, 4, 0]} barSize={16} isAnimationActive={false}>
                <LabelList dataKey="attendance_pct" position="right"
                  formatter={(v: number) => `${v}%`} style={{ fill: ink, fontSize: 11 }} />
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        </div>

        {/* ── Stacked bar: employee time accuracy ─────────────────────────
            Support staff clock in and out at a terminal, and the four ways a day can end are not
            degrees of one failure — arriving late, leaving early and not coming at all are three
            different problems with three different remedies. A single "attendance %" per
            department hides which one is happening, so each day is classified into exactly one
            bucket and the buckets are shown side by side.

            STACKED, not grouped: the four buckets are parts of one whole (every working day is
            exactly one of them), and the stack lets a reader see both the composition within a
            department and the total days worked across departments. Grouped bars would show the
            same four numbers while throwing away the fact that they sum to something meaningful.

            2px surface-coloured gap between segments so adjacent fills never touch — abutting
            colours read as one blended mark, which is the failure a stacked bar is most prone to.

            Direct labels are REQUIRED here, not decorative: two of these four colours sit below
            3:1 contrast on the light surface, and the validator's relief rule says such a palette
            ships with visible labels or a table view. The on-time percentage is printed at the end
            of every bar. */}
        <div style={{ ...card, gridColumn: '1 / -1' }}>
          <ChartTitle>Employee time accuracy by department</ChartTitle>
          <ChartNote>
            {emp.length > 0
              ? `Every clocked day in the window, sorted into one outcome each. ${empOnTimePct}% of ${data.employee_days} days across the institution were on time.`
              : 'Each clocked day counts once: on time, late in, left early, or absent.'}
          </ChartNote>
          {emp.length === 0 ? (
            /* An empty chart frame reads as "0% on time" rather than "no terminal records", and
               those are opposite conclusions for a director to draw. Say which it is. */
            <p style={{ color: 'var(--muted)', fontSize: 13, margin: '18px 0', textAlign: 'center' }}>
              No employee terminal records in the last {data.weeks} weeks — nothing to measure.
              Import them from Employee Attendance.
            </p>
          ) : (
            <ResponsiveContainer width="100%" height={Math.max(200, emp.length * 34)}>
              <BarChart data={emp} layout="vertical" margin={{ top: 4, right: 64, left: 8, bottom: 4 }}>
                <CartesianGrid stroke={grid} strokeOpacity={0.6} horizontal={false} />
                <XAxis type="number" {...axis} />
                <YAxis type="category" dataKey="name" width={168} {...axis} interval={0}
                  tickFormatter={(v: string) => (v.length > 16 ? v.slice(0, 15) + '…' : v)} />
                <Tooltip {...tooltip}
                  formatter={(v: number, n) => [`${v} day(s)`, n]}
                  labelFormatter={(l) => {
                    const d = emp.find(x => x.name === l)
                    return d ? `${l} — ${d.employees} employee(s), ${d.days} day(s)` : String(l)
                  }} />
                {/* Identity is never colour alone: a legend is always present for four series. */}
                <Legend wrapperStyle={{ fontSize: 12 }} />
                {DAY_BUCKETS.map((b, i) => (
                  <Bar key={b.key} dataKey={b.key} name={b.label} stackId="day"
                    fill={SERIES[b.slot][k]} barSize={16} isAnimationActive={false}
                    stroke={dark ? '#1a1a19' : '#fcfcfb'} strokeWidth={2}>
                    {/* Only the last segment carries a label, and it reports the figure the chart
                        exists to answer rather than the length of the segment it sits on. */}
                    {i === DAY_BUCKETS.length - 1 && (
                      <LabelList dataKey="on_time_pct" position="right"
                        formatter={(v: number) => `${v}% on time`}
                        style={{ fill: ink, fontSize: 11 }} />
                    )}
                  </Bar>
                ))}
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>
      </div>
    </div>
  )
}

function shortDate(s: string) {
  const d = new Date(s)
  return isNaN(d.getTime()) ? s : d.toLocaleDateString(undefined, { day: 'numeric', month: 'short' })
}

function ChartTitle({ children }: { children: React.ReactNode }) {
  return <div style={{ fontSize: 13, fontWeight: 700, marginBottom: 2 }}>{children}</div>
}
function ChartNote({ children }: { children: React.ReactNode }) {
  return <div style={{ fontSize: 11, color: 'var(--muted)', marginBottom: 8 }}>{children}</div>
}

const card: React.CSSProperties = {
  background: 'var(--surface, #fff)', border: '1px solid #e2e8f0',
  borderRadius: 10, padding: 14,
}
const sel: React.CSSProperties = {
  padding: '6px 10px', borderRadius: 6, border: '1px solid #cbd5e1', fontSize: 13, background: '#fff',
}
