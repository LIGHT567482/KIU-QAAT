import { useState, useRef } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

// A course is just a course. Coordinators + students attach to an OFFERING
// (course + study session), so one course can run several sessions, each with its
// own coordinator. The student's level of study is recorded on the STUDENT.

interface Course {
  course_id:     string
  name:          string
  department:    string
  school:        string
  unit_count:    number
  total_years?:  number
  level_years?:   Record<string, number>
  offering_count: number
}

interface Offering {
  offering_id:      string
  course_id:        string
  course_name:      string
  session_type:     string
  study_year:       number
  semester:         number
  level:            string
  intake:           string
  coordinator_id:   string
  coordinator_name: string
  coordinator_code: string
  student_count:    number
}

interface User { user_id: string; full_name: string; email: string; role: string }

export default function AdminCourses() {
  const { tenantId } = useParams<{ tenantId: string }>()
  const { status, data: courses, refetch } = useQuery<Course[]>(
    () => api.get(`/api/v1/admin/tenants/${tenantId}/courses`), [tenantId])
  const offeringsQ = useQuery<Offering[]>(
    () => api.get(`/api/v1/admin/tenants/${tenantId}/offerings`), [tenantId])
  const usersQ = useQuery<User[]>(
    () => api.get(`/api/v1/admin/tenants/${tenantId}/users`), [tenantId])
  const users = usersQ.data
  const brandQ = useQuery<{ domain: string }>(() => api.get('/api/v1/branding'))
  const domain = brandQ.status === 'ok' ? (brandQ.data?.domain ?? '') : ''
  const sessionsQ = useQuery<{ study_sessions: string[] }>(() => api.get('/api/v1/admin/settings/study-sessions'), [tenantId])
  const levelsQ = useQuery<{ levels: string[]; level_years?: Record<string, number> }>(() => api.get('/api/v1/admin/settings/levels'), [tenantId])
  const intakesQ = useQuery<{ intakes: string[] }>(() => api.get('/api/v1/admin/settings/intakes'), [tenantId])
  const titlesQ = useQuery<{ titles: string[] }>(() => api.get('/api/v1/admin/settings/titles'), [tenantId])
  const titles = (titlesQ.status === 'ok' ? titlesQ.data?.titles : null) ?? []

  const coordinators = (users ?? []).filter(u => u.role === 'COORDINATOR')
  const sessions = (sessionsQ.status === 'ok' ? sessionsQ.data?.study_sessions : null) ?? ['Day']
  const levels = (levelsQ.status === 'ok' ? levelsQ.data?.levels : null) ?? ['Certificate', 'Diploma', 'Degree']
  const levelYears = (levelsQ.status === 'ok' ? levelsQ.data?.level_years : null) ?? {}
  const intakes = (intakesQ.status === 'ok' ? intakesQ.data?.intakes : null) ?? []
  const offerings = offeringsQ.status === 'ok' ? (offeringsQ.data ?? []) : []

  function refetchAll() { refetch(); offeringsQ.refetch() }

  const [creating, setCreating] = useState(false)
  const [search, setSearch] = useState('')
  const [filters, setFilters] = useState({ department: '', school: '', level: '' })
  const setF = (k: keyof typeof filters, v: string) => setFilters(f => ({ ...f, [k]: v }))
  const cq = search.trim().toLowerCase()
  const uniq = (xs: string[]) => Array.from(new Set(xs.filter(Boolean))).sort()
  const deptOpts = uniq((courses ?? []).map(c => c.department))
  const schoolOpts = uniq((courses ?? []).map(c => c.school))
  const levelOpts = uniq(offerings.map(o => o.level))
  // course_id → set of levels it runs (derived from its offerings) for the Level filter.
  const courseLevels = new Map<string, Set<string>>()
  offerings.forEach(o => { if (!courseLevels.has(o.course_id)) courseLevels.set(o.course_id, new Set()); courseLevels.get(o.course_id)!.add(o.level) })
  const anyFilter = !!cq || !!filters.department || !!filters.school || !!filters.level
  const visibleCourses = (courses ?? []).filter(c =>
    (!cq || [c.name, c.course_id, c.department, c.school].some(v => (v || '').toLowerCase().includes(cq))) &&
    (!filters.department || c.department === filters.department) &&
    (!filters.school || c.school === filters.school) &&
    (!filters.level || courseLevels.get(c.course_id)?.has(filters.level)))
  const [form, setForm] = useState({ course_id: '', name: '', department: '', school: '' })
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [editId, setEditId] = useState<string | null>(null)
  const [editForm, setEditForm] = useState<Partial<Course>>({})
  const [openOfferings, setOpenOfferings] = useState<string | null>(null)

  async function handleCreate() {
    setSaving(true); setError(null)
    try {
      await api.post(`/api/v1/admin/tenants/${tenantId}/courses`, form)
      setCreating(false)
      setForm({ course_id: '', name: '', department: '', school: '' })
      refetch()
    } catch (e) { setError(e instanceof Error ? e.message : 'Failed') }
    finally { setSaving(false) }
  }

  // Shared cohort — create this cohort's offering for EVERY course in one go.
  const [cohortOpen, setCohortOpen] = useState(false)
  const [cohortForm, setCohortForm] = useState({ session_type: '', study_year: '', semester: '', level: '', intake: '' })
  const [cohortBusy, setCohortBusy] = useState(false)
  const [cohortMsg, setCohortMsg] = useState<string | null>(null)
  const cohortReady = cohortForm.session_type && cohortForm.study_year && cohortForm.semester && cohortForm.level
  async function applyCohort() {
    setCohortBusy(true); setCohortMsg(null)
    try {
      const res = await api.post<{ created: number; skipped: number; total_courses: number }>(
        `/api/v1/admin/tenants/${tenantId}/cohorts/apply-all`, {
          session_type: cohortForm.session_type, study_year: Number(cohortForm.study_year),
          semester: Number(cohortForm.semester), level: cohortForm.level, intake: cohortForm.intake,
        })
      setCohortMsg(`Created ${res.created} cohort offering(s) across ${res.total_courses} course(s)${res.skipped ? `; ${res.skipped} already existed` : ''}. Assign coordinators per course below.`)
      refetchAll()
    } catch (e) { setCohortMsg(e instanceof Error ? e.message : 'Failed') }
    finally { setCohortBusy(false) }
  }

  function startEdit(c: Course) {
    setEditId(c.course_id)
    setEditForm({ name: c.name, department: c.department, school: c.school })
  }
  async function handleEditSave() {
    if (!editId) return
    try { await api.patch(`/api/v1/admin/courses/${editId}`, editForm); setEditId(null); refetch() }
    catch (e) { alert(e instanceof Error ? e.message : 'Failed') }
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <div>
          <a href="/admin/tenants" style={{ color: 'var(--muted)', fontSize: 13, textDecoration: 'none' }}>← Tenants</a>
          <h2 style={{ margin: '4px 0 0' }}>Courses & Sessions</h2>
          <p style={{ color: 'var(--muted)', margin: '4px 0 0', fontSize: 13 }}>A course can run several sessions (e.g. Morning, Evening), each with its own coordinator. A student's level of study is set when registering the student.</p>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button onClick={() => { setCohortOpen(o => !o); setCohortMsg(null) }} style={btnSmall}>{cohortOpen ? 'Cancel' : '+ New cohort (all courses)'}</button>
          <button onClick={() => { setCreating(c => !c); setEditId(null) }} style={btnPrimary}>{creating ? 'Cancel' : '+ New Course'}</button>
        </div>
      </div>

      {cohortOpen && (
        <div style={panel}>
          <h3 style={{ margin: '0 0 6px' }}>New cohort across all courses</h3>
          <p style={{ color: 'var(--muted)', fontSize: 13, margin: '0 0 14px' }}>
            Creates this cohort's session for <strong>every course</strong> at once, instead of adding it to each course by hand. Coordinators are attached afterwards, per course, in each course's sessions panel.
          </p>
          {cohortMsg && <div style={{ background: cohortMsg.startsWith('Created') ? '#f0fdf4' : '#fef2f2', color: cohortMsg.startsWith('Created') ? '#166534' : '#b91c1c', padding: '8px 12px', borderRadius: 6, marginBottom: 12, fontSize: 13 }}>{cohortMsg}</div>}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))', gap: 10 }}>
            <Select label="Session" value={cohortForm.session_type} onChange={v => setCohortForm(f => ({ ...f, session_type: v }))} options={sessions} />
            <Select label="Level" value={cohortForm.level} onChange={v => setCohortForm(f => ({ ...f, level: v }))} options={levels} />
            <Select label="Year of study" value={cohortForm.study_year} onChange={v => setCohortForm(f => ({ ...f, study_year: v }))} options={['1', '2', '3', '4', '5', '6']} />
            <Select label="Semester" value={cohortForm.semester} onChange={v => setCohortForm(f => ({ ...f, semester: v }))} options={['1', '2']} />
            <Select label="Intake" value={cohortForm.intake} onChange={v => setCohortForm(f => ({ ...f, intake: v }))} options={intakes} />
          </div>
          <button onClick={applyCohort} disabled={cohortBusy || !cohortReady} style={{ ...btnPrimary, marginTop: 16, opacity: cohortReady ? 1 : 0.5 }}>
            {cohortBusy ? 'Applying…' : 'Apply to all courses'}
          </button>
        </div>
      )}

      {/* Manage study sessions (used when adding sessions to a course) */}
      <div style={{ display: 'flex', gap: 24, flexWrap: 'wrap', marginBottom: 20 }}>
        <ListEditor title="Study sessions" hint="e.g. Morning, Evening, Weekend" values={sessions}
          onSave={v => api.put('/api/v1/admin/settings/study-sessions', { study_sessions: v }).then(() => sessionsQ.refetch())} />
      </div>

      {creating && (
        <div style={panel}>
          <h3 style={{ margin: '0 0 16px' }}>Create Course</h3>
          {error && <div style={errorBox}>{error}</div>}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <Input label="Course ID *" value={form.course_id} onChange={v => setForm(f => ({ ...f, course_id: v }))} placeholder="e.g. SWE" />
            <Input label="Course Name *" value={form.name} onChange={v => setForm(f => ({ ...f, name: v }))} placeholder="e.g. Software Engineering" />
            <Input label="Department" value={form.department} onChange={v => setForm(f => ({ ...f, department: v }))} />
            <Input label="School / Faculty" value={form.school} onChange={v => setForm(f => ({ ...f, school: v }))} />
          </div>
          <button onClick={handleCreate} disabled={saving || !form.course_id || !form.name} style={{ ...btnPrimary, marginTop: 16, opacity: (!form.course_id || !form.name) ? 0.5 : 1 }}>
            {saving ? 'Creating…' : 'Create Course'}
          </button>
        </div>
      )}

      {/* Bulk curriculum import — load an existing catalogue instead of typing it all. */}
      <CurriculumImport tenantId={tenantId!} onDone={refetchAll} />

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}

      <input value={search} onChange={e => setSearch(e.target.value)} placeholder="Search courses by name, ID, department or school…"
        style={{ width: '100%', maxWidth: 420, padding: '8px 12px', borderRadius: 8, border: '1px solid #e2e8f0', fontSize: 14, marginBottom: 12, boxSizing: 'border-box' }} />

      {/* Field filters */}
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center', marginBottom: 12 }}>
        <FilterSelect label="Department" value={filters.department} onChange={v => setF('department', v)} options={deptOpts} />
        <FilterSelect label="School" value={filters.school} onChange={v => setF('school', v)} options={schoolOpts} />
        <FilterSelect label="Level" value={filters.level} onChange={v => setF('level', v)} options={levelOpts} />
        {anyFilter && <button onClick={() => { setFilters({ department: '', school: '', level: '' }); setSearch('') }}
          style={{ ...btnSmall, color: 'var(--muted)' }}>Clear all</button>}
      </div>

      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
        <thead>
          <tr style={{ background: '#f8fafc' }}>
            {['Course', 'Dept', 'Units', 'Sessions', ''].map(h => (
              <th key={h} style={{ padding: '8px 12px', textAlign: 'left', borderBottom: '1px solid #e2e8f0', whiteSpace: 'nowrap' }}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {visibleCourses.map(c => (
            <>
              <tr key={c.course_id} style={{ borderBottom: editId === c.course_id || openOfferings === c.course_id ? 'none' : '1px solid #f1f5f9' }}>
                <td style={{ padding: '10px 12px' }}>
                  <div style={{ fontWeight: 600 }}>{c.name}</div>
                  <div style={{ fontSize: 11, color: 'var(--muted)' }}>{c.course_id}</div>
                </td>
                <td style={{ padding: '10px 12px', color: 'var(--muted)' }}>{c.department || '—'}</td>
                <td style={{ padding: '10px 12px', textAlign: 'center' }}><span style={pill('#e0e7ff', '#3730a3')}>{c.unit_count}</span></td>
                <td style={{ padding: '10px 12px', textAlign: 'center' }}><span style={pill('#f0fdf4', '#166534')}>{c.offering_count}</span></td>
                <td style={{ padding: '10px 12px', whiteSpace: 'nowrap' }}>
                  <a href={`/admin/courses/${c.course_id}/units`} style={{ ...btnSmall, textDecoration: 'none', display: 'inline-block', marginRight: 4 }}>Roadmap</a>
                  <button onClick={() => setOpenOfferings(openOfferings === c.course_id ? null : c.course_id)} style={{ ...btnSmall, marginRight: 4 }}>Sessions</button>
                  <button onClick={() => editId === c.course_id ? setEditId(null) : startEdit(c)} style={btnSmall}>{editId === c.course_id ? 'Cancel' : 'Edit'}</button>
                </td>
              </tr>

              {editId === c.course_id && (
                <tr key={`${c.course_id}-edit`}><td colSpan={5} style={{ padding: '0 12px 16px', background: '#fefce8' }}>
                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 10, paddingTop: 12 }}>
                    <Input label="Name" value={editForm.name ?? ''} onChange={v => setEditForm(f => ({ ...f, name: v }))} />
                    <Input label="Department" value={editForm.department ?? ''} onChange={v => setEditForm(f => ({ ...f, department: v }))} />
                    <Input label="School / Faculty" value={editForm.school ?? ''} onChange={v => setEditForm(f => ({ ...f, school: v }))} />
                  </div>
                  <button onClick={handleEditSave} style={{ ...btnPrimary, marginTop: 10, background: '#92400e' }}>Save Changes</button>
                </td></tr>
              )}

              {openOfferings === c.course_id && (
                <tr key={`${c.course_id}-off`}><td colSpan={5} style={{ padding: '0 12px 16px', background: '#f8fafc' }}>
                  <OfferingsPanel course={c} offerings={offerings.filter(o => o.course_id === c.course_id)}
                    sessions={sessions} levels={levels} levelYears={levelYears} intakes={intakes} coordinators={coordinators}
                    domain={domain} titles={titles} onCoordinatorsChanged={() => usersQ.refetch()} tenantId={tenantId!} onChange={refetchAll} />
                </td></tr>
              )}
            </>
          ))}
          {status === 'ok' && visibleCourses.length === 0 && (
            <tr><td colSpan={5} style={{ padding: 32, textAlign: 'center', color: 'var(--muted)' }}>{cq ? 'No courses match your search.' : 'No courses yet. Create one, then add sessions (with coordinators) to it.'}</td></tr>
          )}
        </tbody>
      </table>
    </div>
  )
}

function OfferingsPanel({ course, offerings, sessions, levels, levelYears, intakes, coordinators, domain, titles, onCoordinatorsChanged, tenantId, onChange }: {
  course: Course; offerings: Offering[]; sessions: string[]; levels: string[]; levelYears: Record<string, number>; intakes: string[]; coordinators: User[]; domain: string; titles: string[]; onCoordinatorsChanged: () => void; tenantId: string; onChange: () => void
}) {
  const yearsFor = (lvl: string) => Array.from({ length: course.level_years?.[lvl] || levelYears[lvl] || 3 }, (_, i) => String(i + 1))
  const [form, setForm] = useState({ session_type: '', study_year: '', semester: '', level: '', intake: '', coordinator_id: '' })
  const setF = (k: keyof typeof form, v: string) => setForm(f => ({ ...f, [k]: v }))
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  async function add() {
    setBusy(true); setErr(null)
    try {
      await api.post(`/api/v1/admin/tenants/${tenantId}/offerings`, {
        course_id: course.course_id, session_type: form.session_type,
        study_year: Number(form.study_year), semester: Number(form.semester),
        level: form.level, intake: form.intake, coordinator_id: form.coordinator_id,
      })
      setForm(f => ({ ...f, coordinator_id: '' })); onChange()
    } catch (e) { setErr(e instanceof Error ? e.message : 'Failed') }
    finally { setBusy(false) }
  }
  async function del(o: Offering) {
    if (!confirm(`Remove the ${o.session_type} · Year ${o.study_year} · Sem ${o.semester} cohort of ${o.course_name}?`)) return
    try { await api.delete(`/api/v1/admin/offerings/${o.offering_id}`); onChange() }
    catch (e) { alert(e instanceof Error ? e.message : 'Failed') }
  }

  const [edit, setEdit] = useState<Offering | null>(null)
  const [editForm, setEditForm] = useState({ session_type: '', study_year: '1', semester: '1', level: '', intake: '', coordinator_id: '' })
  function startEdit(o: Offering) {
    setEdit(o)
    setEditForm({ session_type: o.session_type, study_year: String(o.study_year), semester: String(o.semester), level: o.level, intake: o.intake, coordinator_id: o.coordinator_id })
  }
  async function saveEdit() {
    if (!edit) return
    setBusy(true); setErr(null)
    try {
      await api.patch(`/api/v1/admin/offerings/${edit.offering_id}`, {
        session_type: editForm.session_type, study_year: Number(editForm.study_year), semester: Number(editForm.semester),
        level: editForm.level, intake: editForm.intake, coordinator_id: editForm.coordinator_id,
      })
      setEdit(null); onChange()
    } catch (e) { setErr(e instanceof Error ? e.message : 'Failed') }
    finally { setBusy(false) }
  }

  return (
    <div style={{ paddingTop: 12 }}>
      <div style={{ fontWeight: 700, fontSize: 13, marginBottom: 8 }}>Cohorts of {course.name}</div>
      <div style={{ fontSize: 12, color: 'var(--muted)', marginBottom: 10 }}>A cohort = session · year · semester · level · intake. Each has its own coordinator + timetable. Cohorts exist with or without a coordinator/students.</div>
      {offerings.length === 0 && <div style={{ color: 'var(--muted)', fontSize: 13, marginBottom: 8 }}>No cohorts yet.</div>}
      {offerings.map(o => (
        <div key={o.offering_id}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '6px 0', borderBottom: '1px solid #eef2f7' }}>
            <span style={pill('#f0fdf4', '#166534')}>{[o.session_type, `Y${o.study_year}`, `S${o.semester}`, o.level, o.intake].filter(Boolean).join(' · ')}</span>
            <span style={{ fontSize: 13 }}>{o.coordinator_name || <span style={{ color: '#f59e0b' }}>⚠ no coordinator</span>}{o.coordinator_code && <span style={{ color: '#0369a1', fontFamily: 'monospace', fontSize: 11, marginLeft: 6 }}>{o.coordinator_code}</span>}</span>
            <span style={{ fontSize: 12, color: 'var(--muted)' }}>· {o.student_count} students</span>
            <button onClick={() => edit?.offering_id === o.offering_id ? setEdit(null) : startEdit(o)} style={{ ...btnSmall, marginLeft: 'auto' }}>{edit?.offering_id === o.offering_id ? 'Cancel' : 'Edit'}</button>
            <button onClick={() => del(o)} style={{ ...btnSmall, color: '#b91c1c', borderColor: '#fecaca', background: '#fef2f2' }}>Remove</button>
          </div>
          {edit?.offering_id === o.offering_id && (
            <div style={{ background: '#fefce8', borderRadius: 8, padding: 12, margin: '6px 0' }}>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 8 }}>
                <Select label="Session" value={editForm.session_type} onChange={v => setEditForm(f => ({ ...f, session_type: v }))} options={sessions} />
                <Select label="Year" value={editForm.study_year} onChange={v => setEditForm(f => ({ ...f, study_year: v }))} options={yearsFor(editForm.level)} />
                <Select label="Semester" value={editForm.semester} onChange={v => setEditForm(f => ({ ...f, semester: v }))} options={['1', '2']} />
                <Select label="Level" value={editForm.level} onChange={v => setEditForm(f => ({ ...f, level: v }))} options={levels} />
                <Select label="Intake" value={editForm.intake} onChange={v => setEditForm(f => ({ ...f, intake: v }))} options={intakes.length ? intakes : ['—']} />
                <CoordinatorPicker value={editForm.coordinator_id} onChange={v => setEditForm(f => ({ ...f, coordinator_id: v }))}
                  coordinators={coordinators} domain={domain} titles={titles} tenantId={tenantId} onCreated={onCoordinatorsChanged} />
              </div>
              <button onClick={saveEdit} disabled={busy} style={{ ...btnPrimary, marginTop: 10, background: '#92400e' }}>{busy ? 'Saving…' : 'Save cohort'}</button>
            </div>
          )}
        </div>
      ))}
      {err && <div style={{ ...errorBox, marginTop: 8 }}>{err}</div>}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 8, marginTop: 10 }}>
        <Select label="Session" value={form.session_type} onChange={v => setF('session_type', v)} options={sessions} />
        <Select label="Year" value={form.study_year} onChange={v => setF('study_year', v)} options={yearsFor(form.level)} />
        <Select label="Semester" value={form.semester} onChange={v => setF('semester', v)} options={['1', '2']} />
        <Select label="Level" value={form.level} onChange={v => setF('level', v)} options={levels} />
        <Select label="Intake" value={form.intake} onChange={v => setF('intake', v)} options={intakes.length ? intakes : ['—']} />
        <CoordinatorPicker value={form.coordinator_id} onChange={v => setF('coordinator_id', v)}
          coordinators={coordinators} domain={domain} titles={titles} tenantId={tenantId} onCreated={onCoordinatorsChanged} />
      </div>
      <button onClick={add} disabled={busy || !form.session_type || !form.study_year || !form.semester} style={{ ...btnPrimary, marginTop: 10, opacity: (!form.session_type || !form.study_year || !form.semester) ? 0.5 : 1 }}>{busy ? 'Adding…' : '+ Add cohort'}</button>
    </div>
  )
}

// Coordinator chooser for a cohort: pick an existing coordinator OR create a brand
// new one inline (account + auto-generated coordinator code) without leaving the form.
function CoordinatorPicker({ value, onChange, coordinators, domain, titles, tenantId, onCreated }: {
  value: string; onChange: (v: string) => void; coordinators: User[]; domain: string; titles: string[]; tenantId: string; onCreated: () => void
}) {
  const GENDERS = ['', 'Male', 'Female', 'Other']
  const [adding, setAdding] = useState(false)
  const blank = { title: '', full_name: '', gender: '', local: '', password: '', registration_number: '', phone: '', whatsapp: '' }
  const [nf, setNf] = useState(blank)
  const setN = (k: keyof typeof blank, v: string) => setNf(f => ({ ...f, [k]: v }))
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [code, setCode] = useState<string | null>(null)

  async function create() {
    setBusy(true); setErr(null)
    try {
      if (!nf.full_name.trim() || !nf.local.trim() || !nf.password.trim()) throw new Error('Name, email and password are required.')
      const email = `${nf.local.trim().toLowerCase()}@${domain}`
      const res = await api.post(`/api/v1/admin/tenants/${tenantId}/users`, {
        email, password: nf.password, role: 'COORDINATOR', full_name: nf.full_name.trim(),
        title: nf.title, gender: nf.gender, registration_number: nf.registration_number.trim(),
        phone: nf.phone.trim(), whatsapp: nf.whatsapp.trim(),
      }) as { user_id?: string; coordinator_code?: string }
      onCreated()                       // refresh the coordinators list
      if (res?.user_id) onChange(res.user_id)  // select the new coordinator for this cohort
      setCode(res?.coordinator_code ?? null)
      setAdding(false); setNf(blank)
    } catch (e) { setErr(e instanceof Error ? e.message : 'Failed') }
    finally { setBusy(false) }
  }

  if (adding) {
    return (
      <div style={{ gridColumn: '1 / -1', border: '1px dashed #c7d2fe', background: '#eef2ff', borderRadius: 8, padding: 10 }}>
        <div style={{ fontSize: 12, fontWeight: 700, color: '#3730a3', marginBottom: 8 }}>New coordinator — their course / level / session / year are inherited from this cohort</div>
        {err && <div style={{ ...errorBox, marginBottom: 8 }}>{err}</div>}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 8 }}>
          <Select label="Title" value={nf.title} onChange={v => setN('title', v)} options={['', ...titles]} />
          <Input label="Full name *" value={nf.full_name} onChange={v => setN('full_name', v)} placeholder="e.g. Jane Doe" />
          <Select label="Gender" value={nf.gender} onChange={v => setN('gender', v)} options={GENDERS} />
          <label style={{ display: 'block' }}>
            <div style={labelStyle}>Email *</div>
            <div style={{ display: 'flex', alignItems: 'center' }}>
              <input value={nf.local} placeholder="username" onChange={e => setN('local', e.target.value)}
                style={{ flex: 1, padding: '8px 10px', borderRadius: '6px 0 0 6px', border: '1px solid #e2e8f0', borderRight: 0, fontSize: 14, boxSizing: 'border-box' }} />
              <span style={{ padding: '8px 8px', borderRadius: '0 6px 6px 0', border: '1px solid #e2e8f0', background: '#f1f5f9', color: 'var(--muted)', fontSize: 12, whiteSpace: 'nowrap' }}>@{domain || '…'}</span>
            </div>
          </label>
          <Input label="Password *" value={nf.password} onChange={v => setN('password', v)} placeholder="temporary password" />
          <Input label="Registration No." value={nf.registration_number} onChange={v => setN('registration_number', v)} placeholder="staff/registration no." />
          <Input label="Phone" value={nf.phone} onChange={v => setN('phone', v)} />
          <Input label="WhatsApp" value={nf.whatsapp} onChange={v => setN('whatsapp', v)} />
        </div>
        <div style={{ display: 'flex', gap: 8, marginTop: 10 }}>
          <button onClick={create} disabled={busy} style={btnPrimary}>{busy ? 'Creating…' : 'Create & assign'}</button>
          <button onClick={() => { setAdding(false); setErr(null) }} style={btnSmall}>Cancel</button>
        </div>
      </div>
    )
  }

  return (
    <label style={{ display: 'block' }}>
      <div style={labelStyle}>Coordinator</div>
      <div style={{ display: 'flex', gap: 6 }}>
        <select value={value} onChange={e => onChange(e.target.value)} style={{ ...selectStyle, flex: 1 }}>
          <option value="">— Assign later —</option>
          {coordinators.map(co => <option key={co.user_id} value={co.user_id}>{co.full_name} ({co.email})</option>)}
        </select>
        <button type="button" onClick={() => { setAdding(true); setCode(null) }} style={{ ...btnSmall, whiteSpace: 'nowrap' }}>+ New</button>
      </div>
      {code && <div style={{ fontSize: 11, color: '#065f46', marginTop: 4 }}>Created · coordinator ID <strong style={{ fontFamily: 'monospace' }}>{code}</strong></div>}
    </label>
  )
}

function ListEditor({ title, hint, values, onSave }: { title: string; hint: string; values: string[]; onSave: (v: string[]) => Promise<unknown> }) {
  const [open, setOpen] = useState(false)
  const [draft, setDraft] = useState<string[]>(values)
  const [add, setAdd] = useState('')
  const [busy, setBusy] = useState(false)
  return (
    <div style={{ border: '1px solid #e2e8f0', borderRadius: 8, padding: '10px 14px', minWidth: 240 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <strong style={{ fontSize: 13 }}>{title}</strong>
        <button onClick={() => { setDraft(values); setOpen(o => !o) }} style={{ ...btnSmall }}>{open ? 'Close' : 'Manage'}</button>
      </div>
      {!open
        ? <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 6 }}>{values.join(' · ')}</div>
        : (<div style={{ marginTop: 8 }}>
            <div style={{ fontSize: 11, color: 'var(--muted)', marginBottom: 6 }}>{hint}</div>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginBottom: 8 }}>
              {draft.map((v, i) => (
                <span key={`${v}-${i}`} style={{ ...pill('#eef2ff', '#3730a3'), display: 'inline-flex', gap: 6, alignItems: 'center' }}>{v}
                  <button onClick={() => setDraft(d => d.filter((_, j) => j !== i))} style={{ border: 'none', background: 'transparent', color: '#6366f1', cursor: 'pointer' }}>×</button>
                </span>
              ))}
            </div>
            <div style={{ display: 'flex', gap: 6 }}>
              <input value={add} onChange={e => setAdd(e.target.value)} placeholder="Add…" onKeyDown={e => { if (e.key === 'Enter' && add.trim()) { setDraft(d => [...d, add.trim()]); setAdd('') } }}
                style={{ flex: 1, padding: '6px 8px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 13 }} />
              <button onClick={() => { if (add.trim()) { setDraft(d => [...d, add.trim()]); setAdd('') } }} style={btnSmall}>+</button>
              <button onClick={async () => { setBusy(true); try { await onSave(draft); setOpen(false) } finally { setBusy(false) } }} disabled={busy || draft.length === 0} style={btnPrimary}>{busy ? '…' : 'Save'}</button>
            </div>
          </div>)}
    </div>
  )
}

// Bulk import the whole curriculum from the university's own export. Three files in
// order — courses → units (the roadmap) → lecturer assignments. Each "Template"
// downloads the current data in the exact import columns (export == template), so
// importing a course brings its units, and assignments bring the units' lecturers.
const CURRICULUM_KINDS = [
  { key: 'courses',              label: '1. Courses',     cols: 'course_id, name, department, school' },
  { key: 'course-units',         label: '2. Units (roadmap)', cols: 'unit_id, course_id, name, year, semester, level' },
  { key: 'lecturer-assignments', label: '3. Lecturer mapping', cols: 'unit_id, lecturer_staff_id, lecturer_name, academic_year, intake_session, year, semester' },
] as const

function CurriculumImport({ tenantId, onDone }: { tenantId: string; onDone: () => void }) {
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState<string | null>(null)
  const [msg, setMsg] = useState<Record<string, string>>({})
  const refs = useRef<Record<string, HTMLInputElement | null>>({})

  async function doImport(kind: string, e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    setBusy(kind); setMsg(m => ({ ...m, [kind]: '' }))
    try {
      const fd = new FormData(); fd.append('roster', file)
      const r = await api.upload<{ inserted: number; updated: number; skipped: number; errors: string[] }>(
        `/api/v1/admin/tenants/${tenantId}/${kind}/import`, fd)
      setMsg(m => ({ ...m, [kind]: `✓ ${r.inserted} new, ${r.updated} updated, ${r.skipped} skipped${r.errors?.length ? ` · ${r.errors.slice(0, 3).join('; ')}` : ''}` }))
      onDone()
    } catch (err) { setMsg(m => ({ ...m, [kind]: `✗ ${err instanceof Error ? err.message : 'Import failed'}` })) }
    finally { setBusy(null); if (refs.current[kind]) refs.current[kind]!.value = '' }
  }

  return (
    <div style={{ ...panel, marginBottom: 16, padding: 16 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <strong style={{ fontSize: 14 }}>📥 Bulk import curriculum (courses · units · lecturer mapping)</strong>
        <button onClick={() => setOpen(o => !o)} style={btnSmall}>{open ? 'Hide' : 'Open'}</button>
      </div>
      {open && (
        <div style={{ marginTop: 12 }}>
          <div style={{ fontSize: 12, color: 'var(--muted)', marginBottom: 10 }}>
            Import your existing catalogue instead of typing it. Do them in order. Each <b>Template</b> downloads
            the current data in the exact columns to fill in (CSV or Excel). Unknown lecturers in step 3 are
            auto‑created and mapped to their units.
          </div>
          {CURRICULUM_KINDS.map(k => (
            <div key={k.key} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 0', borderTop: '1px solid #eef2f7', flexWrap: 'wrap' }}>
              <div style={{ minWidth: 160, fontWeight: 600, fontSize: 13 }}>{k.label}</div>
              <code style={{ fontSize: 11, color: 'var(--muted)', flex: '1 1 240px' }}>{k.cols}</code>
              <button onClick={() => api.download(`/api/v1/admin/tenants/${tenantId}/${k.key}/export.xlsx`, `${k.key}.xlsx`).catch(e => alert(e instanceof Error ? e.message : 'Export failed'))} style={btnSmall}>Template / Export</button>
              <input ref={el => { refs.current[k.key] = el }} type="file" accept=".csv,text/csv,.xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" onChange={e => doImport(k.key, e)} style={{ display: 'none' }} />
              <button onClick={() => refs.current[k.key]?.click()} disabled={busy === k.key} style={btnPrimary}>{busy === k.key ? 'Importing…' : 'Import'}</button>
              {msg[k.key] && <div style={{ flexBasis: '100%', fontSize: 12, color: msg[k.key].startsWith('✗') ? '#b91c1c' : '#166534' }}>{msg[k.key]}</div>}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function FilterSelect({ label, value, onChange, options }: { label: string; value: string; onChange: (v: string) => void; options: string[] }) {
  return (
    <select value={value} onChange={e => onChange(e.target.value)}
      style={{ padding: '7px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 13, background: '#fff', color: value ? '#1e293b' : 'var(--muted)', cursor: 'pointer' }}>
      <option value="">{label}: all</option>
      {options.map(o => <option key={o} value={o}>{o}</option>)}
    </select>
  )
}

function Input({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (v: string) => void; placeholder?: string }) {
  return (
    <label style={{ display: 'block' }}>
      <div style={labelStyle}>{label}</div>
      <input value={value} placeholder={placeholder} onChange={e => onChange(e.target.value)}
        style={{ width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }} />
    </label>
  )
}
function Select({ label, value, onChange, options, prefix }: { label: string; value: string; onChange: (v: string) => void; options: string[]; prefix?: string }) {
  return (
    <label style={{ display: 'block' }}>
      <div style={labelStyle}>{label}</div>
      <select value={value} onChange={e => onChange(e.target.value)} style={{ ...selectStyle, color: value ? '#1e293b' : 'var(--muted)' }}>
        <option value="">— Select —</option>
        {options.map(o => <option key={o} value={o} style={{ color: '#1e293b' }}>{`${prefix ?? ''}${o}`}</option>)}
      </select>
    </label>
  )
}

const pill = (bg: string, fg: string): React.CSSProperties => ({ background: bg, color: fg, padding: '2px 8px', borderRadius: 999, fontSize: 12, fontWeight: 600 })
const panel: React.CSSProperties = { background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 10, padding: 20, marginBottom: 24 }
const btnPrimary: React.CSSProperties = { padding: '8px 16px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const btnSmall:   React.CSSProperties = { padding: '4px 10px', background: '#fff', border: '1px solid #e2e8f0', borderRadius: 4, cursor: 'pointer', fontSize: 12 }
const labelStyle: React.CSSProperties = { fontSize: 13, fontWeight: 600, marginBottom: 4, color: '#475569' }
const selectStyle: React.CSSProperties = { width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, background: '#fff', boxSizing: 'border-box' }
const errorBox:   React.CSSProperties = { background: '#fef2f2', color: '#b91c1c', padding: '8px 12px', borderRadius: 6, marginBottom: 12, fontSize: 13 }
