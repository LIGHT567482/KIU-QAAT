import { useMemo, useState } from 'react'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'
import ExportButtons from '../../components/ExportButtons'
import RecordTabs from '../../components/RecordTabs'
import UPanelRecords from '../admin/UPanelRecords'

/**
 * Employee attendance, from the biometric terminal's daily export.
 *
 * The old page showed five fields — staff ID, name, contact, and a prose "comment"
 * that buried days-worked in a sentence — for the whole institution, with no way to
 * narrow it. The terminal exports 29 columns per person per day, so almost
 * everything anyone wanted to know was either hidden in that sentence or absent.
 *
 * The question is never "show me everybody". It is "who in Finance was late last
 * week", so the filters come first and the export carries whatever they selected —
 * the same query string reaches the export endpoint, which re-runs this exact
 * handler, so a printed report can never disagree with the table it came from.
 *
 * Read-only on purpose: the sheet is HR's, uploaded by the administrator. Everyone
 * else — VC, DVC, DQA, QA — traces it rather than edits it.
 */
interface Day {
  ac_no: string; emp_no: string; full_name: string; department: string
  date: string; timetable: string
  on_duty: string; off_duty: string; clock_in: string; clock_out: string
  late: string; early: string; absent: boolean
  ot_time: string; work_time: string; att_time: string; exception: string
  ndays: number | null; ndays_ot: number | null
  checked_in_late: boolean; checked_out_early: boolean
}

export default function EmployeeAttendance() {
  return (
    <RecordTabs title="Employee Attendance" tabs={[
      {
        id: 'terminal',
        label: 'Terminal sheet',
        hint: 'From the biometric terminal daily export.',
        render: () => <TerminalEmployeeDays />,
      },
      {
        id: 'upanel',
        label: 'U-Panel campus',
        hint: 'Admin and staff campus arrival/departure fetched from U-Panel and stored in QAAT.',
        render: () => <UPanelRecords kind="admin" />,
      },
    ]} />
  )
}

function TerminalEmployeeDays() {
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [dept, setDept] = useState('')
  const [q, setQ] = useState('')
  const [flag, setFlag] = useState('')   // '' | late | early | absent | exception

  // Applied filters are separate from the inputs, so typing a name does not fire a
  // query per keystroke against a table that can hold a year of rows.
  const [applied, setApplied] = useState({ from: '', to: '', department: '', q: '', flag: '' })

  const query = useMemo(() => {
    const p: Record<string, string> = {}
    if (applied.from) p.from = applied.from
    if (applied.to) p.to = applied.to
    if (applied.department) p.department = applied.department
    if (applied.q) p.q = applied.q
    if (applied.flag) p[applied.flag] = 'true'
    return p
  }, [applied])

  const qs = new URLSearchParams(query).toString()
  const { data, status } = useQuery<{ days: Day[]; departments: string[]; count: number }>(
    () => api.get(`/api/v1/dashboard/employee-days${qs ? `?${qs}` : ''}`), [qs])

  const days = data?.days ?? []
  const apply = () => setApplied({ from, to, department: dept, q, flag })
  const clear = () => {
    setFrom(''); setTo(''); setDept(''); setQ(''); setFlag('')
    setApplied({ from: '', to: '', department: '', q: '', flag: '' })
  }

  return (
    <div>

      <div style={{
        display: 'flex', gap: 10, alignItems: 'flex-end', flexWrap: 'wrap',
        background: 'var(--surface-2,#f8fafc)', border: '1px solid var(--border,#e2e8f0)',
        borderRadius: 10, padding: 14, marginBottom: 16,
      }}>
        <label><div style={lbl}>From</div>
          <input type="date" value={from} onChange={e => setFrom(e.target.value)} style={inp} /></label>
        <label><div style={lbl}>To</div>
          <input type="date" value={to} onChange={e => setTo(e.target.value)} style={inp} /></label>
        <label><div style={lbl}>Department</div>
          <select value={dept} onChange={e => setDept(e.target.value)} style={{ ...inp, minWidth: 180 }}>
            <option value="">All departments</option>
            {(data?.departments ?? []).map(d => <option key={d} value={d}>{d}</option>)}
          </select></label>
        <label><div style={lbl}>Name or AC-No.</div>
          <input value={q} onChange={e => setQ(e.target.value)} placeholder="e.g. ANUMOLU, or 2385"
            style={{ ...inp, minWidth: 190 }} /></label>
        <label><div style={lbl}>Show only</div>
          <select value={flag} onChange={e => setFlag(e.target.value)} style={{ ...inp, minWidth: 160 }}>
            <option value="">Everything</option>
            <option value="late">Late check-ins</option>
            <option value="early">Early check-outs</option>
            <option value="absent">Absences</option>
            <option value="exception">Exceptions</option>
          </select></label>
        <button onClick={apply} style={btnPrimary}>Apply</button>
        <button onClick={clear} style={btnGhost}>Clear</button>
      </div>

      <div style={{ display: 'flex', gap: 10, alignItems: 'center', marginBottom: 10, flexWrap: 'wrap' }}>
        <span style={{ fontSize: 13, color: 'var(--muted)' }}>
          {status === 'loading' ? 'Loading…' : `${data?.count ?? 0} row${(data?.count ?? 0) === 1 ? '' : 's'}`}
        </span>
        {/* The export inherits the current filters, so it is the same set of rows. */}
        <ExportButtons base="/api/v1/dashboard/employee-days/export"
          filename="employee-attendance" query={qs} disabled={days.length === 0} />
      </div>

      <div style={{ overflowX: 'auto' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12.5, whiteSpace: 'nowrap' }}>
          <thead>
            <tr>
              {['AC-No.', 'Name', 'Department', 'Date', 'Shift', 'On duty', 'Off duty',
                'Clock In', 'Clock Out', 'Work', 'OT', 'Flags'].map(h => (
                <th key={h} style={th}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {days.map(d => (
              <tr key={`${d.ac_no}-${d.date}`}>
                <td style={{ ...td, fontFamily: 'monospace' }}>{d.ac_no}</td>
                <td style={{ ...td, fontWeight: 600 }}>{d.full_name}</td>
                <td style={td}>{d.department || '—'}</td>
                <td style={td}>{d.date}</td>
                <td style={td}>{d.timetable || '—'}</td>
                <td style={td}>{d.on_duty || '—'}</td>
                <td style={td}>{d.off_duty || '—'}</td>
                {/* The two cells that carry the exception, coloured rather than left to be
                    spotted by comparing them with the duty times by eye. */}
                <td style={{ ...td, color: d.checked_in_late ? '#b91c1c' : undefined, fontWeight: d.checked_in_late ? 700 : 400 }}>
                  {d.clock_in || '—'}
                </td>
                <td style={{ ...td, color: d.checked_out_early ? '#b45309' : undefined, fontWeight: d.checked_out_early ? 700 : 400 }}>
                  {d.clock_out || '—'}
                </td>
                <td style={td}>{d.work_time || '—'}</td>
                <td style={td}>{d.ot_time || '—'}</td>
                <td style={td}>
                  {d.absent && <Pill text="Absent" bg="#fee2e2" fg="#991b1b" />}
                  {d.checked_in_late && <Pill text="Late" bg="#fef3c7" fg="#92400e" />}
                  {d.checked_out_early && <Pill text="Left early" bg="#ffedd5" fg="#9a3412" />}
                  {d.exception && <Pill text={d.exception} bg="#e0e7ff" fg="#3730a3" />}
                </td>
              </tr>
            ))}
            {days.length === 0 && (
              <tr><td colSpan={12} style={{ ...td, color: 'var(--muted)' }}>
                {status === 'loading' ? 'Loading…'
                  : 'No rows. Upload the terminal export under Administration → Employee Attendance, or widen the filters.'}
              </td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function Pill({ text, bg, fg }: { text: string; bg: string; fg: string }) {
  return (
    <span style={{
      background: bg, color: fg, padding: '2px 8px', borderRadius: 999,
      fontSize: 11, fontWeight: 700, marginRight: 4, display: 'inline-block',
    }}>{text}</span>
  )
}

const lbl: React.CSSProperties = { fontSize: 11, fontWeight: 700, color: 'var(--muted)', marginBottom: 3 }
const inp: React.CSSProperties = { padding: '8px 10px', borderRadius: 8, border: '1px solid var(--border,#e2e8f0)', fontSize: 13 }
const th: React.CSSProperties = { padding: '8px 10px', textAlign: 'left', borderBottom: '1px solid var(--border,#e2e8f0)', color: 'var(--muted)', fontWeight: 600 }
const td: React.CSSProperties = { padding: '7px 10px', borderBottom: '1px solid var(--border,#f1f5f9)' }
const btnPrimary: React.CSSProperties = { padding: '8px 16px', background: 'var(--brand)', color: 'var(--brand-contrast,#fff)', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const btnGhost: React.CSSProperties = { padding: '8px 12px', background: 'transparent', color: 'var(--text,#334155)', border: '1px solid var(--border,#e2e8f0)', borderRadius: 6, cursor: 'pointer', fontSize: 13 }
