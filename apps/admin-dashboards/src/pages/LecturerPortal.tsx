import { useMemo, useState, type FormEvent } from 'react'

// Public, READ-ONLY lecturer portal. A lecturer enters their institution + staff ID
// (no password) and searches the attendance logs of the units they teach, filtering
// by course, cohort (coordinator) and day. Nothing here can be edited.

const API = (import.meta as unknown as { env: { VITE_API_URL?: string } }).env.VITE_API_URL
  ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')
const URL_ORG = typeof location !== 'undefined' ? (new URLSearchParams(location.search).get('org') ?? '').trim() : ''

interface Unit { unit_id: string; name: string; year: number; semester: number; course_id: string; course_name: string }
interface Sess { session_id: string; session_date: string; coordinator_name: string }
interface Student { student_id: string; full_name: string; present: boolean[] }
interface Coord { coordinator_id: string; coordinator_name: string }
interface Attendance { unit_id: string; sessions: Sess[]; students: Student[]; coordinators: Coord[] }

const DOW = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']

export default function LecturerPortal() {
  const [org, setOrg] = useState(URL_ORG)
  const [staffId, setStaffId] = useState('')
  const [lecturer, setLecturer] = useState<{ full_name: string } | null>(null)
  const [units, setUnits] = useState<Unit[]>([])
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const [course, setCourse] = useState('')
  const [unitId, setUnitId] = useState('')
  const [coord, setCoord] = useState('')
  const [day, setDay] = useState('')
  const [search, setSearch] = useState('')
  const [att, setAtt] = useState<Attendance | null>(null)

  async function doSearch(e?: FormEvent) {
    e?.preventDefault()
    if (!org.trim() || !staffId.trim()) return
    setLoading(true); setErr(null); setLecturer(null); setUnits([]); setAtt(null); setUnitId(''); setCourse(''); setCoord(''); setDay('')
    try {
      const r = await fetch(`${API}/api/v1/lecturer-portal/overview?staff_id=${encodeURIComponent(staffId.trim())}&org=${encodeURIComponent(org.trim())}`)
      if (!r.ok) { const d = await r.json().catch(() => ({})); throw new Error(d.message || 'No lecturer found for that staff ID.') }
      const d = await r.json()
      setLecturer(d.lecturer); setUnits(d.units ?? [])
    } catch (e) { setErr(e instanceof Error ? e.message : 'Failed') } finally { setLoading(false) }
  }

  async function loadUnit(uid: string, coordId = '') {
    setUnitId(uid); setCoord(coordId); setAtt(null); setErr(null); setDay(''); setSearch('')
    if (!uid) return
    setLoading(true)
    try {
      const qs = new URLSearchParams({ staff_id: staffId.trim(), org: org.trim(), unit_id: uid })
      if (coordId) qs.set('coordinator', coordId)
      const r = await fetch(`${API}/api/v1/lecturer-portal/attendance?${qs}`)
      if (!r.ok) { const d = await r.json().catch(() => ({})); throw new Error(d.message || 'Could not load attendance.') }
      setAtt(await r.json())
    } catch (e) { setErr(e instanceof Error ? e.message : 'Failed') } finally { setLoading(false) }
  }

  const courses = useMemo(() => Array.from(new Set(units.map(u => u.course_name).filter(Boolean))).sort(), [units])
  const shownUnits = units.filter(u => !course || u.course_name === course)

  const visible = useMemo(() => {
    if (!att) return [] as { s: Sess; i: number }[]
    return att.sessions.map((s, i) => ({ s, i }))
      .filter(({ s }) => !day || String(new Date(s.session_date + 'T00:00:00').getDay()) === day)
  }, [att, day])

  const shownStudents = (att?.students ?? []).filter(st =>
    !search || st.full_name.toLowerCase().includes(search.toLowerCase()) || st.student_id.toLowerCase().includes(search.toLowerCase()))

  return (
    <div style={{ minHeight: '100vh', background: '#f1f5f9', fontFamily: 'system-ui', color: '#0f172a' }}>
      <header style={{ background: '#1a7a3f', color: '#fff', padding: '14px 18px' }}>
        <div style={{ maxWidth: 1000, margin: '0 auto', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12 }}>
          <div style={{ fontWeight: 800, fontSize: 17 }}>Lecturer Portal</div>
          <span style={{ background: 'rgba(255,255,255,.2)', padding: '3px 10px', borderRadius: 999, fontSize: 12, fontWeight: 700 }}>READ ONLY</span>
        </div>
      </header>

      <div style={{ maxWidth: 1000, margin: '0 auto', padding: '20px 16px' }}>
        <form onSubmit={doSearch} style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 16 }}>
          {!URL_ORG && (
            <input value={org} onChange={e => setOrg(e.target.value)} placeholder="Institution (e.g. kiu.ac.ug)" style={inp} />
          )}
          <input value={staffId} onChange={e => setStaffId(e.target.value)} placeholder="Your staff ID" style={{ ...inp, flex: 1, minWidth: 200 }} />
          <button type="submit" disabled={loading || !staffId.trim() || !org.trim()} style={btn}>{loading ? 'Searching…' : 'Search'}</button>
        </form>

        {err && <div style={{ background: '#fef2f2', color: '#b91c1c', padding: '10px 14px', borderRadius: 8, marginBottom: 14, fontSize: 13 }}>{err}</div>}

        {lecturer && (
          <>
            <div style={{ fontWeight: 800, fontSize: 16, marginBottom: 4 }}>{lecturer.full_name}</div>
            <div style={{ color: '#334155', fontSize: 13, marginBottom: 14 }}>{units.length} unit(s) you teach. Pick a unit to see its attendance.</div>

            <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', marginBottom: 14 }}>
              <Sel label="Course" value={course} onChange={v => { setCourse(v); setUnitId(''); setAtt(null) }} options={courses.map(c => ({ v: c, l: c }))} allLabel="All courses" />
              <Sel label="Unit" value={unitId} onChange={v => loadUnit(v)} options={shownUnits.map(u => ({ v: u.unit_id, l: `${u.name} (${u.unit_id})` }))} allLabel="Select a unit…" />
              {att && att.coordinators.length > 1 && (
                <Sel label="Cohort (coordinator)" value={coord} onChange={v => loadUnit(unitId, v)} options={att.coordinators.map(c => ({ v: c.coordinator_id, l: c.coordinator_name || c.coordinator_id }))} allLabel="All cohorts" />
              )}
              {att && (
                <Sel label="Day" value={day} onChange={setDay} options={DOW.map((d, i) => ({ v: String(i), l: d }))} allLabel="All days" />
              )}
              {att && (
                <label style={{ fontSize: 12 }}>
                  <div style={{ color: '#334155', marginBottom: 3 }}>Search student</div>
                  <input value={search} onChange={e => setSearch(e.target.value)} placeholder="name or reg-no" style={{ ...inp, minWidth: 160 }} />
                </label>
              )}
            </div>

            {loading && <p style={{ color: '#334155' }}>Loading…</p>}

            {att && !loading && (
              visible.length === 0 || shownStudents.length === 0 ? (
                <p style={{ color: '#334155', textAlign: 'center', padding: 24 }}>
                  {att.sessions.length === 0 ? 'No sessions have been run for this unit yet.' : 'No records match these filters.'}
                </p>
              ) : (
                <div style={{ overflowX: 'auto', border: '1px solid #e2e8f0', borderRadius: 10, background: '#fff' }}>
                  <table style={{ borderCollapse: 'collapse', fontSize: 12, width: '100%' }}>
                    <thead>
                      <tr style={{ background: '#f0fdf4' }}>
                        <th style={{ ...th, position: 'sticky', left: 0, background: '#f0fdf4', textAlign: 'left', minWidth: 180 }}>Student</th>
                        {visible.map(({ s, i }) => (
                          <th key={i} style={th} title={s.coordinator_name}>
                            {s.session_date.slice(5)}<br /><span style={{ fontWeight: 400, color: '#64748b' }}>{DOW[new Date(s.session_date + 'T00:00:00').getDay()].slice(0, 3)}</span>
                          </th>
                        ))}
                        <th style={th}>%</th>
                      </tr>
                    </thead>
                    <tbody>
                      {shownStudents.map(st => {
                        const present = visible.filter(({ i }) => st.present[i]).length
                        const pct = visible.length ? Math.round(100 * present / visible.length) : 0
                        return (
                          <tr key={st.student_id} style={{ borderTop: '1px solid #f1f5f9' }}>
                            <td style={{ ...td, position: 'sticky', left: 0, background: '#fff' }}>
                              <div style={{ fontWeight: 600 }}>{st.full_name}</div>
                              <div style={{ color: '#64748b', fontFamily: 'monospace', fontSize: 11 }}>{st.student_id}</div>
                            </td>
                            {visible.map(({ i }) => (
                              <td key={i} style={{ ...td, textAlign: 'center', color: st.present[i] ? '#16a34a' : '#cbd5e1', fontWeight: 700 }}>
                                {st.present[i] ? '✓' : '·'}
                              </td>
                            ))}
                            <td style={{ ...td, textAlign: 'center', fontWeight: 700, color: pct >= 75 ? '#16a34a' : '#ef4444' }}>{pct}%</td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                </div>
              )
            )}
          </>
        )}
      </div>
    </div>
  )
}

function Sel({ label, value, onChange, options, allLabel }: { label: string; value: string; onChange: (v: string) => void; options: { v: string; l: string }[]; allLabel: string }) {
  return (
    <label style={{ fontSize: 12 }}>
      <div style={{ color: '#334155', marginBottom: 3 }}>{label}</div>
      <select value={value} onChange={e => onChange(e.target.value)} style={{ ...inp, minWidth: 170 }}>
        <option value="">{allLabel}</option>
        {options.map(o => <option key={o.v} value={o.v}>{o.l}</option>)}
      </select>
    </label>
  )
}

const inp: React.CSSProperties = { padding: '9px 11px', borderRadius: 8, border: '1px solid #cbd5e1', fontSize: 14, background: '#fff', color: '#0f172a', boxSizing: 'border-box' }
const btn: React.CSSProperties = { padding: '9px 18px', background: '#1a7a3f', color: '#fff', border: 'none', borderRadius: 8, cursor: 'pointer', fontWeight: 700, fontSize: 14 }
const th: React.CSSProperties = { padding: '7px 8px', textAlign: 'center', whiteSpace: 'nowrap', borderBottom: '1px solid #e2e8f0', fontSize: 11, color: '#0f172a' }
const td: React.CSSProperties = { padding: '7px 8px', whiteSpace: 'nowrap' }
