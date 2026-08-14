import { useState } from 'react'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'
import ExportButtons from '../../components/ExportButtons'

/**
 * HOW MUCH OF THE WEEK THE QA ROUND ACTUALLY SAW.
 *
 * Every patrol-derived figure in this dashboard is a fraction whose denominator was missing. A 90%
 * "taught" rate is excellent if the monitors covered the whole timetable and close to meaningless
 * if they covered a fifth of it — and until this page there was no way to tell which, because the
 * system recorded what the round FOUND and never what it MISSED.
 *
 * The distinction the page is built around, and the reason the two numbers are never added
 * together: a slot nobody patrolled is a ROTA problem, and a slot patrolled and found empty is a
 * TEACHING problem. They look identical in a total and call for opposite conversations.
 */

interface ScopeRow { name: string; expected: number; patrolled: number; not_taught: number; coverage_pct: number }
interface GapRow { unit_id: string; unit_name: string; room: string; school: string; day_of_week: number; scheduled_time: string; missed: number }
interface MonitorRow { name: string; ticks: number; rooms: number; last_at: string }
interface Coverage {
  days: number
  expected: number
  patrolled: number
  coverage_pct: number
  taught: number
  not_taught: number
  by_school: ScopeRow[]
  gaps: GapRow[]
  monitors: MonitorRow[]
}

const DAYS = ['', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']

// Coverage is a rota judgement, not a pass mark, so the bands are deliberately coarse. Anything
// under half the timetable means the round is sampling rather than covering, and the report should
// say so in a colour rather than leave a director to work it out from a decimal.
function band(p: number) {
  if (p >= 80) return '#16a34a'
  if (p >= 50) return '#f59e0b'
  return '#dc2626'
}

export default function DQAPatrolCoverage() {
  const [days, setDays] = useState(30)
  const { status, data, refetch } = useQuery<Coverage>(
    () => api.get(`/api/v1/dashboard/dqa/patrol-coverage?days=${days}`),
    [days],
  )

  const d = status === 'ok' ? data : undefined
  const missed = d ? d.expected - d.patrolled : 0

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 6, flexWrap: 'wrap', gap: 12 }}>
        <div>
          <h2 style={{ margin: 0 }}>QA Monitor Coverage</h2>
          <p style={{ color: 'var(--muted)', margin: '4px 0 0', fontSize: 13 }}>
            How much of the published timetable the QA round reached — the denominator behind every
            other monitor figure.
          </p>
        </div>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
          <select value={days} onChange={e => setDays(Number(e.target.value))} style={sel}>
            <option value={7}>Last 7 days</option>
            <option value={30}>Last 30 days</option>
            <option value={90}>This semester (90 days)</option>
          </select>
          {/* The SAME window the table is showing — the download must not silently be a
              different report from the one the director just read. */}
          {/* The endpoint keeps its original path — renaming the feature is a labelling change,
              and the URL is a contract the exports and any saved link already depend on. */}
          <ExportButtons base="/api/v1/dashboard/dqa/patrol-coverage/export"
            filename="qa-monitor-coverage" query={`days=${days}`} />
          <button onClick={refetch} style={btn}>Refresh</button>
        </div>
      </div>

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}
      {status === 'error' && <p style={{ color: '#b91c1c' }}>Failed to load QA monitor coverage.</p>}

      {d && d.expected === 0 && (
        <p style={{ color: 'var(--muted)', marginTop: 40, textAlign: 'center' }}>
          No timetabled slots in this window, so there is nothing the round could have covered.
          Publish a timetable first — coverage is measured against it.
        </p>
      )}

      {d && d.expected > 0 && (
        <>
          <div style={{ display: 'flex', gap: 12, margin: '20px 0 8px', flexWrap: 'wrap' }}>
            <Card label="Timetabled slots" value={d.expected} hint="what the round could have seen" color="#475569" />
            <Card label="Monitored" value={d.patrolled} hint={`${d.coverage_pct.toFixed(0)}% coverage`} color={band(d.coverage_pct)} />
            <Card label="Never visited" value={missed} hint="a rota gap, not a teaching one" color="#dc2626" />
            <Card label="Found not taught" value={d.not_taught} hint="visited, and no lecture" color="#b45309" />
          </div>

          {/* The one sentence a director should be able to read without interpreting anything. */}
          <div style={{
            background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 8,
            padding: '10px 14px', fontSize: 13, color: 'var(--text)', marginBottom: 24,
          }}>
            The round reached <strong>{d.patrolled}</strong> of <strong>{d.expected}</strong> timetabled
            slots in the last {d.days} days. Of those it reached, <strong>{d.not_taught}</strong> had
            no lecture. The remaining <strong>{missed}</strong> were never visited — nothing is known
            about them either way.
          </div>

          <Section title="By college" subtitle="Where the round is thin. A low percentage here is a rota to rebalance, not a lecturer to chase.">
            <table style={table}>
              <thead>
                <tr style={{ background: '#f8fafc' }}>
                  {['College', 'Timetabled', 'Monitored', 'Coverage', 'Not taught'].map(h => (
                    <th key={h} style={th}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {d.by_school.map(s => (
                  <tr key={s.name} style={{ borderBottom: '1px solid #f1f5f9' }}>
                    <td style={{ ...td, fontWeight: 600 }}>{s.name}</td>
                    <td style={{ ...td, textAlign: 'center' }}>{s.expected}</td>
                    <td style={{ ...td, textAlign: 'center' }}>{s.patrolled}</td>
                    <td style={{ ...td, textAlign: 'center' }}>
                      <span style={{ color: band(s.coverage_pct), fontWeight: 700 }}>
                        {s.coverage_pct.toFixed(0)}%
                      </span>
                    </td>
                    <td style={{ ...td, textAlign: 'center', color: s.not_taught > 0 ? '#b45309' : 'var(--muted)' }}>
                      {s.not_taught}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </Section>

          <Section
            title="Slots the round never reaches"
            subtitle="Ranked by how often the SAME weekly slot was missed. One missed Tuesday is a busy day; nine is a rota that never gets to that corridor."
          >
            {d.gaps.length === 0 ? (
              <p style={{ color: '#16a34a', fontSize: 13 }}>✓ Every timetabled slot in this window was visited at least once.</p>
            ) : (
              <table style={table}>
                <thead>
                  <tr style={{ background: '#f8fafc' }}>
                    {['Unit', 'Room', 'College', 'When', 'Times missed'].map(h => <th key={h} style={th}>{h}</th>)}
                  </tr>
                </thead>
                <tbody>
                  {d.gaps.map((g, i) => (
                    <tr key={`${g.unit_id}-${g.day_of_week}-${g.scheduled_time}-${i}`} style={{ borderBottom: '1px solid #f1f5f9' }}>
                      <td style={{ ...td, fontWeight: 600 }}>
                        {g.unit_name}
                        <div style={{ fontSize: 11, color: 'var(--muted)' }}>{g.unit_id}</div>
                      </td>
                      <td style={td}>{g.room}</td>
                      <td style={{ ...td, color: 'var(--muted)' }}>{g.school}</td>
                      <td style={td}>{DAYS[g.day_of_week] ?? '—'} {g.scheduled_time}</td>
                      <td style={{ ...td, textAlign: 'center' }}>
                        <span style={{
                          background: '#fef2f2', color: '#991b1b', padding: '2px 10px',
                          borderRadius: 999, fontSize: 12, fontWeight: 700,
                        }}>{g.missed}</span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </Section>

          <Section title="Who walked the rounds" subtitle="Because the answer to “why is that corridor never covered” is usually that one person covers it and was away.">
            {d.monitors.length === 0 ? (
              <p style={{ color: 'var(--muted)', fontSize: 13 }}>No QA monitor activity recorded in this window.</p>
            ) : (
              <table style={table}>
                <thead>
                  <tr style={{ background: '#f8fafc' }}>
                    {['Monitor', 'Findings filed', 'Distinct rooms', 'Last seen'].map(h => <th key={h} style={th}>{h}</th>)}
                  </tr>
                </thead>
                <tbody>
                  {d.monitors.map(m => (
                    <tr key={m.name} style={{ borderBottom: '1px solid #f1f5f9' }}>
                      <td style={{ ...td, fontWeight: 600 }}>{m.name}</td>
                      <td style={{ ...td, textAlign: 'center' }}>{m.ticks}</td>
                      <td style={{ ...td, textAlign: 'center' }}>{m.rooms}</td>
                      <td style={{ ...td, color: 'var(--muted)' }}>
                        {m.last_at ? new Date(m.last_at).toLocaleDateString() : '—'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </Section>
        </>
      )}
    </div>
  )
}

function Card({ label, value, hint, color }: { label: string; value: number; hint: string; color: string }) {
  return (
    <div style={{ background: '#fff', borderRadius: 10, padding: '14px 18px', border: `1px solid ${color}44`, minWidth: 150 }}>
      <div style={{ fontSize: 28, fontWeight: 700, color }}>{value}</div>
      <div style={{ fontSize: 12, fontWeight: 600 }}>{label}</div>
      <div style={{ fontSize: 11, color: 'var(--muted)' }}>{hint}</div>
    </div>
  )
}

function Section({ title, subtitle, children }: { title: string; subtitle: string; children: React.ReactNode }) {
  return (
    <div style={{ marginBottom: 28 }}>
      <h3 style={{ margin: '0 0 2px', fontSize: 15 }}>{title}</h3>
      <p style={{ color: 'var(--muted)', margin: '0 0 10px', fontSize: 12 }}>{subtitle}</p>
      {children}
    </div>
  )
}

const table: React.CSSProperties = { width: '100%', borderCollapse: 'collapse', fontSize: 14 }
const th: React.CSSProperties = { padding: '8px 12px', textAlign: 'left', borderBottom: '1px solid #e2e8f0', whiteSpace: 'nowrap' }
const td: React.CSSProperties = { padding: '10px 12px' }
const btn: React.CSSProperties = { padding: '7px 16px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const sel: React.CSSProperties = { padding: '7px 10px', borderRadius: 6, border: '1px solid #cbd5e1', fontSize: 13, background: '#fff' }
