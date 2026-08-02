import { useState } from 'react'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'
import ExportButtons from '../../components/ExportButtons'

// Lecturer attendance — QA PATROL record.
//
// The second, independent witness to the same lectures. The coordinator's page records what the
// lecturer themselves started and ended; this records what a QA patroller saw when they walked into
// the room. The two are kept apart on purpose: where they disagree — a session the coordinator
// logged but the patroller found empty — is precisely what QA is looking for, and merging them
// would hide it.

interface Summary {
  lecturer_id: string; lecturer_name: string; department: string; school: string
  patrolled: number; taught: number; missed: number; rate: number; last_patrol_date: string
}
interface Visit {
  patrol_id: string; lecturer_id: string; unit_id: string; unit_name: string
  room: string; session_date: string; scheduled_time: string; taught: boolean
  patroller_name: string; patroller_staff_id: string; taken_at: string
}

export default function PatrolLecturerAttendance() {
  const [range, setRange] = useState({ from: '', to: '' })
  const qs = (() => {
    const p = new URLSearchParams()
    if (range.from) p.set('from', range.from)
    if (range.to) p.set('to', range.to)
    return p.toString()
  })()

  const { data, status } = useQuery<{ summary: Summary[]; visits: Visit[] }>(
    () => api.get(`/api/v1/dashboard/lecturer-attendance/patrol${qs ? `?${qs}` : ''}`), [qs])
  const summary = data?.summary ?? []
  const visits = data?.visits ?? []

  const [open, setOpen] = useState<Set<string>>(new Set())
  const toggle = (id: string) => setOpen(p => { const n = new Set(p); n.has(id) ? n.delete(id) : n.add(id); return n })
  const visitsFor = (id: string) => visits.filter(v => v.lecturer_id === id)

  const [search, setSearch] = useState('')
  const sq = search.trim().toLowerCase()
  const list = summary.filter(s => !sq ||
    [s.lecturer_name, s.lecturer_id, s.department, s.school].some(v => (v || '').toLowerCase().includes(sq)))

  const totals = summary.reduce((a, s) => ({ p: a.p + s.patrolled, t: a.t + s.taught }), { p: 0, t: 0 })
  const fmtDate = (d: string) => d ? new Date(d + 'T00:00:00').toLocaleDateString() : '—'

  return (
    <div style={{ color: 'var(--text)' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 12, flexWrap: 'wrap', marginBottom: 12 }}>
        <p style={{ color: 'var(--muted)', margin: 0, fontSize: 13, maxWidth: 620 }}>
          What a QA patroller observed on walking into the room — an independent check against the
          coordinator's record. One row per lecturer; open it for every visit.
        </p>
        <ExportButtons base="/api/v1/dashboard/lecturer-attendance/patrol/export"
          filename="lecturer-attendance-patrol" query={qs} disabled={summary.length === 0} />
      </div>

      <div style={{ display: 'flex', gap: 10, alignItems: 'flex-end', flexWrap: 'wrap', marginBottom: 14 }}>
        <label style={{ fontSize: 12, color: 'var(--muted)' }}>
          <div style={{ marginBottom: 3 }}>From</div>
          <input type="date" value={range.from} onChange={e => setRange(r => ({ ...r, from: e.target.value }))} style={inp} />
        </label>
        <label style={{ fontSize: 12, color: 'var(--muted)' }}>
          <div style={{ marginBottom: 3 }}>To</div>
          <input type="date" value={range.to} onChange={e => setRange(r => ({ ...r, to: e.target.value }))} style={inp} />
        </label>
        {(range.from || range.to) && (
          <button onClick={() => setRange({ from: '', to: '' })} style={btn}>Clear dates</button>
        )}
        <input value={search} onChange={e => setSearch(e.target.value)}
          placeholder="Search lecturer, staff ID, department…"
          style={{ ...inp, flex: 1, minWidth: 220 }} />
      </div>

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}
      {status === 'ok' && summary.length === 0 && (
        <div style={{ background: '#fffbeb', border: '1px solid #fde68a', color: '#92400e', borderRadius: 10, padding: 16, fontSize: 13 }}>
          No patrols recorded yet. Records appear here once a QA patroller submits their rounds from
          the Patrol app.
        </div>
      )}

      {summary.length > 0 && (
        <div style={{ color: 'var(--muted)', fontSize: 13, marginBottom: 8 }}>
          Overall: <strong style={{ color: 'var(--text)' }}>{totals.t}</strong> of {totals.p} patrols found the lecturer teaching
          {totals.p > 0 && ` (${Math.round((totals.t / totals.p) * 100)}%)`}.
        </div>
      )}

      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
        <thead><tr style={{ background: 'var(--surface,#f8fafc)' }}>
          {['Lecturer', 'Staff ID', 'Department', 'Patrolled', 'Teaching', 'Missed', 'Rate', 'Last patrol', ''].map(h =>
            <th key={h} style={{ padding: '8px 12px', textAlign: 'left', borderBottom: '1px solid var(--border,#e2e8f0)', whiteSpace: 'nowrap' }}>{h}</th>)}
        </tr></thead>
        <tbody>
          {list.map(s => (
            <>
              <tr key={s.lecturer_id} style={{ borderBottom: open.has(s.lecturer_id) ? 'none' : '1px solid var(--border,#f1f5f9)' }}>
                <td style={{ padding: '10px 12px', fontWeight: 700 }}>{s.lecturer_name || s.lecturer_id}</td>
                <td style={{ padding: '10px 12px', fontFamily: 'monospace', fontSize: 12 }}>{s.lecturer_id || '—'}</td>
                <td style={{ padding: '10px 12px', color: 'var(--muted)' }}>{s.department || '—'}</td>
                <td style={{ padding: '10px 12px', fontWeight: 700 }}>{s.patrolled}</td>
                <td style={{ padding: '10px 12px', color: '#15803d', fontWeight: 600 }}>{s.taught}</td>
                <td style={{ padding: '10px 12px', color: s.missed > 0 ? '#b91c1c' : 'var(--muted)', fontWeight: s.missed > 0 ? 700 : 400 }}>{s.missed}</td>
                <td style={{ padding: '10px 12px' }}>
                  <span style={{ fontWeight: 700, color: s.rate >= 75 ? '#15803d' : '#b91c1c' }}>{s.rate}%</span>
                </td>
                <td style={{ padding: '10px 12px', color: 'var(--muted)' }}>{fmtDate(s.last_patrol_date)}</td>
                <td style={{ padding: '10px 12px' }}>
                  <button onClick={() => toggle(s.lecturer_id)} style={btn}>
                    {open.has(s.lecturer_id) ? 'Hide visits' : 'View visits'}
                  </button>
                </td>
              </tr>
              {open.has(s.lecturer_id) && (
                <tr key={`${s.lecturer_id}-v`}><td colSpan={9} style={{ padding: '0 12px 14px', background: 'var(--surface,#f8fafc)' }}>
                  <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12.5 }}>
                    <thead><tr>{['Date', 'Time', 'Unit', 'Room', 'Verdict', 'Patroller', 'Recorded'].map(h =>
                      <th key={h} style={{ padding: '6px 10px', textAlign: 'left', color: 'var(--muted)' }}>{h}</th>)}</tr></thead>
                    <tbody>
                      {visitsFor(s.lecturer_id).map(v => (
                        <tr key={v.patrol_id} style={{ borderTop: '1px solid var(--border,#e8eef4)' }}>
                          <td style={{ padding: '6px 10px', fontWeight: 600 }}>{fmtDate(v.session_date)}</td>
                          <td style={{ padding: '6px 10px' }}>{v.scheduled_time || '—'}</td>
                          <td style={{ padding: '6px 10px' }}>
                            {v.unit_name || v.unit_id}
                            <span style={{ color: 'var(--muted)', fontFamily: 'monospace', fontSize: 11, marginLeft: 6 }}>{v.unit_id}</span>
                          </td>
                          <td style={{ padding: '6px 10px' }}>{v.room || '—'}</td>
                          <td style={{ padding: '6px 10px' }}>
                            <span style={{
                              padding: '2px 8px', borderRadius: 999, fontSize: 11, fontWeight: 700,
                              background: v.taught ? '#f0fdf4' : '#fef2f2',
                              color: v.taught ? '#166534' : '#b91c1c',
                            }}>{v.taught ? 'TEACHING' : 'NOT TEACHING'}</span>
                          </td>
                          <td style={{ padding: '6px 10px' }}>
                            {v.patroller_name || '—'}
                            {v.patroller_staff_id && <span style={{ color: 'var(--muted)', fontSize: 11, marginLeft: 6 }}>{v.patroller_staff_id}</span>}
                          </td>
                          <td style={{ padding: '6px 10px', color: 'var(--muted)' }}>
                            {v.taken_at ? new Date(v.taken_at).toLocaleString() : '—'}
                          </td>
                        </tr>
                      ))}
                      {visitsFor(s.lecturer_id).length === 0 && (
                        <tr><td colSpan={7} style={{ padding: 12, color: 'var(--muted)' }}>No visits in this date range.</td></tr>
                      )}
                    </tbody>
                  </table>
                </td></tr>
              )}
            </>
          ))}
          {status === 'ok' && summary.length > 0 && list.length === 0 && (
            <tr><td colSpan={9} style={{ padding: 24, textAlign: 'center', color: 'var(--muted)' }}>No lecturer matches the search.</td></tr>
          )}
        </tbody>
      </table>
    </div>
  )
}

const inp: React.CSSProperties = { padding: '7px 10px', borderRadius: 6, border: '1px solid var(--border,#e2e8f0)', fontSize: 13, background: 'var(--surface,#fff)', color: 'var(--text,#0f172a)', boxSizing: 'border-box' }
const btn: React.CSSProperties = { padding: '6px 12px', background: 'var(--surface,#fff)', border: '1px solid var(--border,#e2e8f0)', borderRadius: 6, cursor: 'pointer', fontSize: 12, color: 'inherit', fontWeight: 600 }
