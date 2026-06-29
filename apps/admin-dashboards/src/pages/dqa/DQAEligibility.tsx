import { useMemo, useState } from 'react'
import { api, type EligibilityRecord, type AllEligibilityRecord } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

type Tab = 'all' | 'lookup'

export default function DQAEligibility() {
  const [tab, setTab] = useState<Tab>('all')

  return (
    <div style={{ maxWidth: 1100 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 4 }}>
        <h2 style={{ margin: 0 }}>Exam Eligibility</h2>
        <a href={`${import.meta.env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')}/api/v1/reports/dqa/eligibility.csv`}
          style={{ padding: '6px 14px', borderRadius: 6, border: '1px solid #e2e8f0', background: '#fff', fontSize: 13, textDecoration: 'none', color: '#1e293b' }}>
          Export CSV
        </a>
      </div>

      {/* Tab switcher */}
      <div style={{ display: 'flex', gap: 2, marginBottom: 20, marginTop: 16, borderBottom: '2px solid #e2e8f0' }}>
        {(['all', 'lookup'] as Tab[]).map(t => (
          <button key={t} onClick={() => setTab(t)} style={{
            padding: '8px 20px', fontWeight: 600, fontSize: 13,
            background: 'none', border: 'none', cursor: 'pointer',
            borderBottom: tab === t ? '2px solid #1e293b' : '2px solid transparent',
            color: tab === t ? '#1e293b' : 'var(--muted)',
            marginBottom: -2,
          }}>
            {t === 'all' ? 'All Students' : 'Student Lookup'}
          </button>
        ))}
      </div>

      {tab === 'all' ? <AllStudentsTab /> : <LookupTab />}
    </div>
  )
}

// ─── All students — every student's eligibility, with filters ─────────────────

const EMPTY_FILTERS = { course: '', unit: '', status: '', year: '', semester: '', intake: '', academic: '' }

function AllStudentsTab() {
  const { status, data, refetch } = useQuery<{
    total_count: number; eligible_count: number; ineligible_count: number; records: AllEligibilityRecord[]
  }>(() => api.get('/api/v1/dashboard/dqa/eligibility-all'))

  const records = status === 'ok' ? (data?.records ?? []) : []
  const [filters, setFilters] = useState({ ...EMPTY_FILTERS })
  const [search, setSearch] = useState('')
  const setF = (k: keyof typeof filters, v: string) => setFilters(f => ({ ...f, [k]: v }))

  const uniq = (xs: string[]) => Array.from(new Set(xs.filter(Boolean))).sort()
  const opts = useMemo(() => ({
    course:   uniq(records.map(r => r.course_name)),
    unit:     uniq(records.map(r => r.unit_name)),
    year:     uniq(records.map(r => r.current_year ? String(r.current_year) : '')),
    semester: uniq(records.map(r => r.semester ? String(r.semester) : '')),
    intake:   uniq(records.map(r => r.intake_session)),
    academic: uniq(records.map(r => r.academic_year)),
  }), [records])

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    return records.filter(r =>
      (!q ||
        r.student_name.toLowerCase().includes(q) ||
        r.student_id.toLowerCase().includes(q) ||
        r.email.toLowerCase().includes(q) ||
        r.unit_name.toLowerCase().includes(q) ||
        r.course_name.toLowerCase().includes(q)) &&
      (!filters.course   || r.course_name === filters.course) &&
      (!filters.unit     || r.unit_name === filters.unit) &&
      (!filters.status   || r.status === filters.status) &&
      (!filters.year     || String(r.current_year) === filters.year) &&
      (!filters.semester || String(r.semester) === filters.semester) &&
      (!filters.intake   || r.intake_session === filters.intake) &&
      (!filters.academic || r.academic_year === filters.academic),
    )
  }, [records, filters, search])

  const hasFilters = search !== '' || Object.values(filters).some(Boolean)

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12, flexWrap: 'wrap', gap: 8 }}>
        <p style={{ color: 'var(--muted)', margin: 0, fontSize: 13 }}>
          Every student's exam eligibility — shown in full regardless of any search; refreshed on sync.
          {status === 'ok' && data && (
            <>
              {' '}<strong>{data.total_count}</strong> record(s) ·{' '}
              <span style={{ color: '#166534', fontWeight: 600 }}>{data.eligible_count} eligible</span> ·{' '}
              <span style={{ color: '#b91c1c', fontWeight: 600 }}>{data.ineligible_count} ineligible</span>
            </>
          )}
        </p>
        <button onClick={refetch} style={btn}>Refresh</button>
      </div>

      {/* Filter bar */}
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center', marginBottom: 16 }}>
        <input value={search} onChange={e => setSearch(e.target.value)}
          placeholder="Search name, ID, email, unit or course…"
          style={{ flex: '1 1 240px', minWidth: 200, padding: '8px 12px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }} />
        <FilterSelect label="Course" value={filters.course} onChange={v => setF('course', v)} options={opts.course} />
        <FilterSelect label="Unit" value={filters.unit} onChange={v => setF('unit', v)} options={opts.unit} />
        <FilterSelect label="Status" value={filters.status} onChange={v => setF('status', v)} options={['ELIGIBLE', 'INELIGIBLE']} labels={{ ELIGIBLE: 'Eligible', INELIGIBLE: 'Ineligible' }} />
        <FilterSelect label="Year" value={filters.year} onChange={v => setF('year', v)} options={opts.year} prefix="Year " />
        <FilterSelect label="Semester" value={filters.semester} onChange={v => setF('semester', v)} options={opts.semester} prefix="Sem " />
        <FilterSelect label="Intake" value={filters.intake} onChange={v => setF('intake', v)} options={opts.intake} />
        <FilterSelect label="Academic year" value={filters.academic} onChange={v => setF('academic', v)} options={opts.academic} />
        {hasFilters && (
          <button onClick={() => { setFilters({ ...EMPTY_FILTERS }); setSearch('') }}
            style={{ padding: '8px 12px', borderRadius: 6, border: '1px solid #e2e8f0', background: '#fff', fontSize: 13, cursor: 'pointer', color: 'var(--muted)' }}>
            Clear all
          </button>
        )}
      </div>

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}
      {status === 'error'   && <p style={{ color: '#b91c1c' }}>Failed to load eligibility list.</p>}
      {status === 'ok' && filtered.length === 0 && (
        <div style={{ textAlign: 'center', padding: 48, color: 'var(--muted)' }}>
          {records.length === 0 ? 'No student attendance records yet.' : 'No students match these filters.'}
        </div>
      )}

      {filtered.length > 0 && (
        <>
          <div style={{ fontSize: 12, color: 'var(--muted)', marginBottom: 6 }}>Showing {filtered.length} of {records.length}</div>
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
              <thead>
                <tr style={{ background: '#f8fafc' }}>
                  {['Student', 'Course', 'Unit', 'Yr/Sem', 'Intake', 'Attended/Held', '%', 'Threshold', 'Status'].map(h => (
                    <th key={h} style={{ padding: '8px 10px', textAlign: 'left', borderBottom: '1px solid #e2e8f0', whiteSpace: 'nowrap' }}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {filtered.map((r, i) => {
                  const elig = r.status === 'ELIGIBLE'
                  return (
                    <tr key={`${r.student_id}-${r.unit_id}-${i}`} style={{ borderBottom: '1px solid #f1f5f9', background: i % 2 === 0 ? '#fff' : '#fafafa' }}>
                      <td style={{ padding: '8px 10px' }}>
                        <div style={{ fontWeight: 600 }}>{r.student_name}</div>
                        <div style={{ fontSize: 11, color: 'var(--muted)' }}>{r.student_id}</div>
                      </td>
                      <td style={{ padding: '8px 10px', color: 'var(--muted)' }}>{r.course_name}</td>
                      <td style={{ padding: '8px 10px' }}>
                        <div style={{ fontWeight: 600 }}>{r.unit_name}</div>
                        <div style={{ fontSize: 11, color: 'var(--muted)' }}>{r.unit_id}</div>
                      </td>
                      <td style={{ padding: '8px 10px', color: 'var(--muted)', whiteSpace: 'nowrap' }}>{r.current_year ? `Y${r.current_year}` : '—'}{r.semester ? ` / S${r.semester}` : ''}</td>
                      <td style={{ padding: '8px 10px', color: 'var(--muted)' }}>{r.intake_session || '—'}</td>
                      <td style={{ padding: '8px 10px', textAlign: 'center' }}>{r.sessions_attended}/{r.sessions_held}</td>
                      <td style={{ padding: '8px 10px', fontWeight: 700, color: elig ? '#16a34a' : '#ef4444' }}>{r.attendance_percentage}%</td>
                      <td style={{ padding: '8px 10px', color: 'var(--muted)' }}>{r.threshold}%</td>
                      <td style={{ padding: '8px 10px' }}><StatusPill status={r.status} deficit={r.deficit_sessions} /></td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  )
}

function FilterSelect({ label, value, onChange, options, prefix, labels }: {
  label: string; value: string; onChange: (v: string) => void; options: string[]
  prefix?: string; labels?: Record<string, string>
}) {
  return (
    <select value={value} onChange={e => onChange(e.target.value)}
      style={{ padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 13, background: '#fff', color: value ? '#1e293b' : 'var(--muted)', cursor: 'pointer' }}>
      <option value="">{label}: all</option>
      {options.map(o => <option key={o} value={o}>{labels?.[o] ?? `${prefix ?? ''}${o}`}</option>)}
    </select>
  )
}

// ─── Single student lookup ────────────────────────────────────────────────────

function LookupTab() {
  const [studentId, setStudentId] = useState('')
  const [query, setQuery]         = useState('')

  const { status, data } = useQuery<EligibilityRecord>(
    () => api.get(`/api/v1/eligibility/${query}`),
    [query],
  )

  return (
    <div>
      <p style={{ color: 'var(--muted)', marginBottom: 16 }}>Look up a single student's attendance and eligibility status by registration number.</p>

      <div style={{ display: 'flex', gap: 8, marginBottom: 24 }}>
        <input value={studentId} onChange={e => setStudentId(e.target.value)}
          placeholder="Student registration number (e.g. NUT/CS/2024/001)"
          style={{ flex: 1, padding: '9px 12px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 15 }}
          onKeyDown={e => e.key === 'Enter' && setQuery(studentId)} />
        <button onClick={() => setQuery(studentId)}
          style={{ padding: '9px 18px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600 }}>
          Look up
        </button>
      </div>

      {query && status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}
      {query && status === 'error'   && <p style={{ color: '#b91c1c' }}>Student not found.</p>}

      {status === 'ok' && data && (
        <div>
          <div style={{ marginBottom: 16 }}>
            <strong>{data.student_id}</strong>
            <span style={{ color: 'var(--muted)', marginLeft: 8 }}>AY {data.academic_year} · Sem {data.semester}</span>
          </div>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
            <thead>
              <tr style={{ background: '#f8fafc', textAlign: 'left' }}>
                {['Unit', 'Held', 'Attended', '%', 'Threshold', 'Status'].map(h => (
                  <th key={h} style={{ padding: '8px 12px', borderBottom: '1px solid #e2e8f0' }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {data.units.map(u => (
                <tr key={u.unit_id} style={{ borderBottom: '1px solid #f1f5f9' }}>
                  <td style={{ padding: '8px 12px' }}><div style={{ fontWeight: 600 }}>{u.unit_name}</div><div style={{ fontSize: 12, color: 'var(--muted)' }}>{u.unit_id}</div></td>
                  <td style={{ padding: '8px 12px' }}>{u.sessions_held}</td>
                  <td style={{ padding: '8px 12px' }}>{u.sessions_attended}</td>
                  <td style={{ padding: '8px 12px', fontWeight: 600 }}>{u.attendance_percentage}%</td>
                  <td style={{ padding: '8px 12px', color: 'var(--muted)' }}>{u.threshold}%</td>
                  <td style={{ padding: '8px 12px' }}><StatusPill status={u.status} deficit={u.deficit_sessions} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function StatusPill({ status, deficit }: { status: string; deficit?: number }) {
  const ok = status === 'ELIGIBLE'
  return (
    <span style={{
      display: 'inline-block', padding: '2px 10px', borderRadius: 999, fontSize: 12, fontWeight: 600,
      background: ok ? '#f0fdf4' : '#fef2f2',
      color:      ok ? '#166534' : '#b91c1c',
    }}>
      {ok ? 'Eligible' : `Ineligible${deficit ? ` (−${deficit})` : ''}`}
    </span>
  )
}

const btn: React.CSSProperties = { padding: '6px 14px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
