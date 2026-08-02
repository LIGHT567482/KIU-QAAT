import { useState, useRef, useEffect } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

// email is OPTIONAL — identity is the reg-no; email is only for correspondence.
const CSV_COLS = ['student_id', 'full_name', 'email', 'course_id', 'academic_year', 'current_year', 'semester', 'intake_session', 'level']
function toCSV(rows: Record<string, unknown>[], cols: string[]): string {
  const esc = (v: unknown) => { const s = String(v ?? ''); return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s }
  return [cols.join(','), ...rows.map(r => cols.map(c => esc(r[c])).join(','))].join('\n')
}
function download(name: string, content: string) {
  const url = URL.createObjectURL(new Blob([content], { type: 'text/csv' }))
  const a = document.createElement('a'); a.href = url; a.download = name; a.click(); URL.revokeObjectURL(url)
}

const INTAKE_SESSIONS = ['Morning', 'Evening', 'Weekend', 'Distance'] as const

interface Student {
  student_id: string; full_name: string; email: string
  course_id: string; course_name: string; current_year: number
  semester: number; academic_year: string; intake_session: string; enrollment_status: string
  level: string
}

interface Course {
  course_id: string; name: string
}

interface Offering {
  offering_id: string; course_id: string; course_name: string
  session_type: string; study_year: number; semester: number; level: string; intake: string
  coordinator_name: string
}

// "Day · Y1 · S1 · Degree · January — Jane" — a cohort's human label.
function cohortLabel(o: Offering): string {
  const cohort = [o.session_type, `Y${o.study_year}`, `S${o.semester}`, o.level, o.intake].filter(Boolean).join(' · ')
  return cohort + (o.coordinator_name ? ` — ${o.coordinator_name}` : '')
}

export default function AdminStudents() {
  const { tenantId } = useParams<{ tenantId: string }>()
  const { status, data: students, refetch } = useQuery<Student[]>(
    () => api.get(`/api/v1/admin/tenants/${tenantId}/students`),
    [tenantId],
  )
  const { data: courses } = useQuery<Course[]>(
    () => api.get(`/api/v1/admin/tenants/${tenantId}/courses`),
    [tenantId],
  )
  // Students register into an OFFERING (program + session); that binds their
  // course (for units) and their coordinator.
  const { data: offerings } = useQuery<Offering[]>(
    () => api.get(`/api/v1/admin/tenants/${tenantId}/offerings`),
    [tenantId],
  )
  // Students reach their own progress view from inside the app, so no share-link is
  // surfaced here — only the institution's active academic year is needed.
  const brandQ = useQuery<{ domain: string; active_academic_year?: string }>(() => api.get('/api/v1/branding'))
  // The institution's active academic year (set once in Settings). Students inherit
  // it — no need to re-type it per registration, same as the cohort-derived fields.
  const activeAY = brandQ.data?.active_academic_year ?? ''

  // Tenant-defined intakes (#1) — admin-configurable; offered at registration.
  const intakesQ = useQuery<{ intakes: string[] }>(() => api.get('/api/v1/admin/settings/intakes'), [tenantId])
  const tenantIntakes = (intakesQ.status === 'ok' ? intakesQ.data?.intakes : null) ?? []
  const intakeOptions = tenantIntakes.length ? tenantIntakes : [...INTAKE_SESSIONS]

  // Tenant-defined levels of study — asked when registering a student (the level
  // of education the student pursues in this course).
  const levelsQ = useQuery<{ levels: string[]; level_years?: Record<string, number> }>(() => api.get('/api/v1/admin/settings/levels'), [tenantId])
  const levelOptions = (levelsQ.status === 'ok' ? levelsQ.data?.levels : null) ?? ['Certificate', 'Diploma', 'Degree']
  const levelYearsMap = (levelsQ.status === 'ok' ? levelsQ.data?.level_years : null) ?? {}

  const [creating, setCreating] = useState(false)
  const [search, setSearch] = useState('')
  const [form, setForm] = useState({
    student_id: '', full_name: '', email: '',
    offering_id: '', level: '', current_year: 0, semester: 0,
    academic_year: '', intake_session: '',
  })
  // Inherit the institution's active academic year into the form once it loads.
  useEffect(() => {
    if (activeAY) setForm(f => (f.academic_year ? f : { ...f, academic_year: activeAY }))
  }, [activeAY])
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  // Sign-in details of the student just registered, shown once so the admin can pass them on.
  const [created, setCreated] = useState<{ id: string; password: string } | null>(null)
  const [editStudent, setEditStudent] = useState<Student | null>(null)

  async function handleDelete(s: Student) {
    if (!confirm(`Delete student "${s.full_name}" (${s.student_id})? This also removes their login. This cannot be undone.`)) return
    try {
      await api.delete(`/api/v1/admin/tenants/${tenantId}/students?student_id=${encodeURIComponent(s.student_id)}`)
      refetch()
    } catch (e) { alert(e instanceof Error ? e.message : 'Delete failed') }
  }
  // Course → Session picker (resolves to an offering_id). The student's Level is a
  // separate field (the level of study the student pursues in this course).
  const [pick, setPick] = useState({ course: '' })

  // ── Manage intakes (#1) ────────────────────────────────────────────────────
  const [intakeEdit, setIntakeEdit] = useState(false)
  const [intakeDraft, setIntakeDraft] = useState<string[]>([])
  const [newIntake, setNewIntake] = useState('')
  const [intakeSaving, setIntakeSaving] = useState(false)

  function openIntakeEditor() {
    setIntakeDraft(intakeOptions)
    setNewIntake('')
    setIntakeEdit(true)
  }
  async function saveIntakes() {
    setIntakeSaving(true)
    try {
      await api.put('/api/v1/admin/settings/intakes', { intakes: intakeDraft })
      setIntakeEdit(false)
      intakesQ.refetch()
    } catch (e) { alert(e instanceof Error ? e.message : 'Failed to save intakes') }
    finally { setIntakeSaving(false) }
  }

  // ── Manage levels of study ─────────────────────────────────────────────────
  const [levelEdit, setLevelEdit] = useState(false)
  const [levelDraft, setLevelDraft] = useState<string[]>([])
  const [yearsDraft, setYearsDraft] = useState<Record<string, number>>({})
  const [newLevel, setNewLevel] = useState('')
  const [levelSaving, setLevelSaving] = useState(false)
  function openLevelEditor() { setLevelDraft(levelOptions); setYearsDraft({ ...levelYearsMap }); setNewLevel(''); setLevelEdit(true) }
  async function saveLevels() {
    setLevelSaving(true)
    try {
      await api.put('/api/v1/admin/settings/levels', { levels: levelDraft, level_years: yearsDraft })
      setLevelEdit(false)
      levelsQ.refetch()
    } catch (e) { alert(e instanceof Error ? e.message : 'Failed to save levels') }
    finally { setLevelSaving(false) }
  }

  function startCreate() {
    setCreating(c => !c)
  }

  // ── Bulk import / export ───────────────────────────────────────────────────
  const fileRef = useRef<HTMLInputElement>(null)
  const [importing, setImporting] = useState(false)
  const [importMsg, setImportMsg] = useState<string | null>(null)

  async function handleImport(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    setImporting(true); setImportMsg(null)
    try {
      const fd = new FormData(); fd.append('roster', file)
      // Import-into-cohort: the currently-active filters become the target so rows
      // missing those columns land in the chosen course/year/semester/intake.
      if (filters.course)   fd.append('course_id', filters.course)
      if (filters.year)     fd.append('study_year', filters.year)
      if (filters.semester) fd.append('semester', filters.semester)
      if (filters.intake)   fd.append('intake', filters.intake)
      if (filters.academic) fd.append('academic_year', filters.academic)
      const res = await api.upload<{ inserted: number; updated: number; skipped: number; errors: string[] }>(
        '/api/v1/import/csv', fd)
      setImportMsg(`Imported: ${res.inserted} new, ${res.updated} updated, ${res.skipped} skipped${res.errors?.length ? ` · ${res.errors.length} error(s): ${res.errors.slice(0, 3).join('; ')}` : ''}`)
      refetch()
    } catch (e) { setImportMsg(e instanceof Error ? `Import failed: ${e.message}` : 'Import failed') }
    finally { setImporting(false); if (fileRef.current) fileRef.current.value = '' }
  }

  // Active filters as a query string for the server-side Excel export.
  function exportQS() {
    const p = new URLSearchParams()
    if (filters.course)   p.set('course_id', filters.course)
    if (filters.year)     p.set('study_year', filters.year)
    if (filters.semester) p.set('semester', filters.semester)
    if (filters.intake)   p.set('intake', filters.intake)
    const s = p.toString()
    return s ? `?${s}` : ''
  }

  // ── Filters ──────────────────────────────────────────────────────────────
  const [filters, setFilters] = useState({ course: '', year: '', semester: '', intake: '', academic: '', status: '' })
  function setF(k: keyof typeof filters, v: string) { setFilters(f => ({ ...f, [k]: v })) }
  const all = students ?? []
  const uniq = (xs: string[]) => Array.from(new Set(xs.filter(Boolean))).sort()
  const intakes = uniq(all.map(s => s.intake_session))
  const academics = uniq(all.map(s => s.academic_year))
  const statuses = uniq(all.map(s => s.enrollment_status))

  async function handleCreate() {
    setSaving(true); setError(null); setCreated(null)
    try {
      // Students are identified by their reg-no only — the server auto-handles a
      // hidden identity for the check-in path. Email is OPTIONAL and used solely to
      // email the student their QR; reg-no-only students need none.
      const res = await api.post<{ sign_in_id?: string; default_password?: string }>(
        `/api/v1/admin/tenants/${tenantId}/students`, {
        student_id: form.student_id, full_name: form.full_name, email: form.email,
        offering_id: form.offering_id, level: form.level,
        current_year: form.current_year, semester: form.semester,
        academic_year: form.academic_year, intake_session: form.intake_session,
      })
      // Show the credentials the new student signs in with. The admin is the one who hands them
      // over, so leaving them to guess is how "I added a student and the app says no account" starts.
      if (res?.default_password) {
        setCreated({ id: res.sign_in_id || form.student_id, password: res.default_password })
      }
      setCreating(false)
      setForm({ student_id: '', full_name: '', email: '', offering_id: '', level: '', current_year: 0, semester: 0, academic_year: activeAY, intake_session: '' })
      setPick({ course: '' })
      refetch()
    } catch (e) { setError(e instanceof Error ? e.message : 'Failed') }
    finally { setSaving(false) }
  }

  const list = all.filter(s => {
    const q = search.toLowerCase()
    const matchesSearch = !q ||
      s.full_name.toLowerCase().includes(q) ||
      s.student_id.toLowerCase().includes(q) ||
      s.course_name.toLowerCase().includes(q)
    return matchesSearch &&
      (!filters.course   || s.course_id === filters.course) &&
      (!filters.year     || String(s.current_year) === filters.year) &&
      (!filters.semester || String(s.semester) === filters.semester) &&
      (!filters.intake   || s.intake_session === filters.intake) &&
      (!filters.academic || s.academic_year === filters.academic) &&
      (!filters.status   || s.enrollment_status === filters.status)
  })

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 20 }}>
        <div>
          <h2 style={{ margin: '0' }}>Students</h2>
        </div>
        <div style={{ display: 'flex', gap: 8, alignItems: 'flex-start' }}>
          <button onClick={() => download('students_template.csv', CSV_COLS.join(',') + '\n')} style={btnGhost} title="Download a blank CSV with the required columns">Template</button>
          <button onClick={() => download(`students_${Date.now()}.csv`, toCSV(list as unknown as Record<string, unknown>[], CSV_COLS))} disabled={list.length === 0} style={btnGhost}>Export CSV</button>
          <button onClick={() => api.download(`/api/v1/admin/tenants/${tenantId}/students/export.xlsx${exportQS()}`, 'students.xlsx').catch(e => alert(e instanceof Error ? e.message : 'Export failed'))} style={btnGhost} title="Exports the currently-filtered students">Export Excel</button>
          <input ref={fileRef} type="file" accept=".csv,text/csv,.xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" onChange={handleImport} style={{ display: 'none' }} />
          <button onClick={() => fileRef.current?.click()} disabled={importing} style={btnGhost} title="Imports into the currently-filtered course/year/semester (rows may omit those columns)">{importing ? 'Importing…' : 'Import (CSV/Excel)'}</button>
          <button onClick={openIntakeEditor} style={btnGhost} title="Define the intakes offered at registration">Manage intakes</button>
          <button onClick={openLevelEditor} style={btnGhost} title="Define the levels of study offered at registration">Manage levels</button>
          <button onClick={startCreate} style={btnPrimary}>
            {creating ? 'Cancel' : '+ Register Student'}
          </button>
        </div>
      </div>

      {importMsg && (
        <div style={{ background: importMsg.startsWith('Import failed') ? '#fef2f2' : '#f0fdf4', color: importMsg.startsWith('Import failed') ? '#b91c1c' : '#166534', padding: '10px 14px', borderRadius: 8, marginBottom: 16, fontSize: 13 }}>
          {importMsg}
        </div>
      )}

      {intakeEdit && (
        <div style={{ background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 10, padding: 20, marginBottom: 24 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
            <h3 style={{ margin: 0 }}>Intakes</h3>
            <button onClick={() => setIntakeEdit(false)} style={{ ...btnGhost, padding: '4px 10px' }}>Close</button>
          </div>
          <p style={{ color: 'var(--muted)', fontSize: 13, margin: '0 0 14px' }}>
            These are the intakes (e.g. January, May, August) offered when registering a student.
          </p>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginBottom: 14 }}>
            {intakeDraft.map((it, i) => (
              <span key={`${it}-${i}`} style={{ display: 'inline-flex', alignItems: 'center', gap: 6, background: '#eef2ff', color: '#3730a3', padding: '5px 10px', borderRadius: 999, fontSize: 13, fontWeight: 600 }}>
                {it}
                <button onClick={() => setIntakeDraft(d => d.filter((_, j) => j !== i))} style={{ border: 'none', background: 'transparent', color: '#6366f1', cursor: 'pointer', fontSize: 15, lineHeight: 1 }}>×</button>
              </span>
            ))}
            {intakeDraft.length === 0 && <span style={{ color: 'var(--muted)', fontSize: 13 }}>No intakes yet — add at least one.</span>}
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <input value={newIntake} onChange={e => setNewIntake(e.target.value)} placeholder="e.g. January Intake"
              onKeyDown={e => { if (e.key === 'Enter' && newIntake.trim()) { setIntakeDraft(d => [...d, newIntake.trim()]); setNewIntake('') } }}
              style={{ flex: 1, padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }} />
            <button onClick={() => { if (newIntake.trim()) { setIntakeDraft(d => [...d, newIntake.trim()]); setNewIntake('') } }} style={btnGhost}>+ Add</button>
            <button onClick={saveIntakes} disabled={intakeSaving || intakeDraft.length === 0} style={btnPrimary}>{intakeSaving ? 'Saving…' : 'Save intakes'}</button>
          </div>
        </div>
      )}

      {levelEdit && (
        <div style={{ background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 10, padding: 20, marginBottom: 24 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
            <h3 style={{ margin: 0 }}>Levels of study</h3>
            <button onClick={() => setLevelEdit(false)} style={{ ...btnGhost, padding: '4px 10px' }}>Close</button>
          </div>
          <p style={{ color: 'var(--muted)', fontSize: 13, margin: '0 0 14px' }}>
            The levels of education (e.g. Certificate, Diploma, Degree) offered at registration, and how many <strong>years</strong> each one is studied — a Degree may be 3 years while a Masters is 2.
          </p>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginBottom: 14 }}>
            {levelDraft.map((it, i) => (
              <div key={`${it}-${i}`} style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <span style={{ flex: 1, background: '#ecfeff', color: '#155e75', padding: '6px 12px', borderRadius: 8, fontSize: 13, fontWeight: 600 }}>{it}</span>
                <label style={{ fontSize: 12, color: 'var(--muted)', display: 'flex', alignItems: 'center', gap: 6 }}>
                  Years:
                  <input type="number" min={1} max={10} value={yearsDraft[it] ?? 3}
                    onChange={e => setYearsDraft(y => ({ ...y, [it]: Number(e.target.value) }))}
                    style={{ width: 64, padding: '6px 8px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14 }} />
                </label>
                <button onClick={() => { setLevelDraft(d => d.filter((_, j) => j !== i)); setYearsDraft(y => { const n = { ...y }; delete n[it]; return n }) }}
                  style={{ border: '1px solid #fecaca', background: '#fef2f2', color: '#b91c1c', borderRadius: 6, padding: '4px 10px', cursor: 'pointer', fontSize: 12 }}>Remove</button>
              </div>
            ))}
            {levelDraft.length === 0 && <span style={{ color: 'var(--muted)', fontSize: 13 }}>No levels yet — add at least one.</span>}
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <input value={newLevel} onChange={e => setNewLevel(e.target.value)} placeholder="e.g. Masters"
              onKeyDown={e => { if (e.key === 'Enter' && newLevel.trim()) { const n = newLevel.trim(); setLevelDraft(d => [...d, n]); setYearsDraft(y => ({ ...y, [n]: y[n] ?? 3 })); setNewLevel('') } }}
              style={{ flex: 1, padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }} />
            <button onClick={() => { if (newLevel.trim()) { const n = newLevel.trim(); setLevelDraft(d => [...d, n]); setYearsDraft(y => ({ ...y, [n]: y[n] ?? 3 })); setNewLevel('') } }} style={btnGhost}>+ Add</button>
            <button onClick={saveLevels} disabled={levelSaving || levelDraft.length === 0} style={btnPrimary}>{levelSaving ? 'Saving…' : 'Save levels'}</button>
          </div>
        </div>
      )}

      {created && (
        <div style={{
          background: 'rgba(26,122,63,.08)', border: '1px solid rgba(26,122,63,.35)', borderRadius: 10,
          padding: '12px 16px', marginBottom: 16, display: 'flex', justifyContent: 'space-between', gap: 12, alignItems: 'center',
        }}>
          <span style={{ fontSize: 14 }}>
            <strong>{created.id}</strong> can now sign in to the KIU QAAT app with the registration
            number and the password <strong>{created.password}</strong>. They are asked to change it
            immediately at first sign-in.
          </span>
          <button onClick={() => setCreated(null)} aria-label="Dismiss"
            style={{ border: 'none', background: 'transparent', cursor: 'pointer', fontSize: 16, color: 'var(--muted)' }}>✕</button>
        </div>
      )}
      {creating && (
        <div style={{ background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 10, padding: 20, marginBottom: 24 }}>
          <h3 style={{ margin: '0 0 16px' }}>Register Student</h3>
          <p style={{ color: 'var(--muted)', fontSize: 13, marginTop: -8, marginBottom: 16 }}>
            Only the registration number and name are needed — the student is identified by their reg-no.
          </p>{/* no QR: check-in is by typed registration number */}
          {error && <div style={errorBox}>{error}</div>}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <Input label="Registration Number" value={form.student_id} onChange={v => setForm(f => ({ ...f, student_id: v }))} placeholder="e.g. NUT/CS/2024/001" />
            <Input label="Full Name" value={form.full_name} onChange={v => setForm(f => ({ ...f, full_name: v }))} placeholder="e.g. Jane Doe" />
            <Input label="Email (optional)" value={form.email} onChange={v => setForm(f => ({ ...f, email: v }))} placeholder="leave blank to skip" />

            {/* Course → Session resolves the offering; Level is the student's own
                attribute. */}
            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Course</div>
              <select value={pick.course} onChange={e => { setPick({ course: e.target.value }); setForm(f => ({ ...f, offering_id: '' })) }} style={selectStyle}>
                <option value="">— Select course —</option>
                {uniq((offerings ?? []).map(o => o.course_name)).map(c => <option key={c} value={c}>{c}</option>)}
              </select>
            </label>
            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Session</div>
              <select value={form.offering_id} disabled={!pick.course} onChange={e => {
                // Picking the cohort inherits its year/semester/level/intake — no need
                // to re-ask (they are properties of the cohort, not the student).
                const off = (offerings ?? []).find(o => o.offering_id === e.target.value)
                setForm(f => ({ ...f, offering_id: e.target.value,
                  ...(off ? { level: off.level, intake_session: off.intake, current_year: off.study_year, semester: off.semester } : {}) }))
              }} style={selectStyle}>
                <option value="">— Select session —</option>
                {(offerings ?? []).filter(o => o.course_name === pick.course).map(o => (
                  <option key={o.offering_id} value={o.offering_id}>{cohortLabel(o)}</option>
                ))}
              </select>
              {(offerings ?? []).length === 0 && (
                <p style={{ fontSize: 12, color: '#f59e0b', margin: '4px 0 0' }}>
                  No sessions found. Create a course and add a session (with a coordinator) first.
                </p>
              )}
            </label>
            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Level of study{form.offering_id ? ' · from cohort' : ''}</div>
              <select value={form.level} disabled={!!form.offering_id} onChange={e => setForm(f => ({ ...f, level: e.target.value }))} style={form.offering_id ? greyed : selectStyle}>
                <option value="">— Select level —</option>
                {levelOptions.map(l => <option key={l} value={l}>{l}</option>)}
              </select>
            </label>

            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Intake{form.offering_id ? ' · from cohort' : ''}</div>
              <select value={form.intake_session} disabled={!!form.offering_id} onChange={e => setForm(f => ({ ...f, intake_session: e.target.value }))} style={form.offering_id ? greyed : { ...selectStyle, color: form.intake_session ? '#1e293b' : 'var(--muted)' }}>
                <option value="">— Select —</option>
                {intakeOptions.map(s => <option key={s} value={s} style={{ color: '#1e293b' }}>{s}</option>)}
              </select>
            </label>

            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Year of Study{form.offering_id ? ' · from cohort' : ''}</div>
              <select value={form.current_year || ''} disabled={!!form.offering_id} onChange={e => setForm(f => ({ ...f, current_year: Number(e.target.value) }))} style={form.offering_id ? greyed : { ...selectStyle, color: form.current_year ? '#1e293b' : 'var(--muted)' }}>
                <option value="">— Select —</option>
                {[1, 2, 3, 4].map(y => <option key={y} value={y} style={{ color: '#1e293b' }}>Year {y}</option>)}
              </select>
            </label>

            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Semester{form.offering_id ? ' · from cohort' : ''}</div>
              <select value={form.semester || ''} disabled={!!form.offering_id} onChange={e => setForm(f => ({ ...f, semester: Number(e.target.value) }))} style={form.offering_id ? greyed : { ...selectStyle, color: form.semester ? '#1e293b' : 'var(--muted)' }}>
                <option value="">— Select —</option>
                <option value={1} style={{ color: '#1e293b' }}>Semester 1</option>
                <option value={2} style={{ color: '#1e293b' }}>Semester 2</option>
              </select>
            </label>

            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Academic Year{activeAY ? ' · institution year' : ''}</div>
              <input value={form.academic_year} disabled={!!activeAY}
                onChange={e => setForm(f => ({ ...f, academic_year: e.target.value }))}
                placeholder="e.g. 2024/2025" style={activeAY ? greyed : selectStyle} />
              {!activeAY && <p style={{ fontSize: 11, color: '#f59e0b', margin: '4px 0 0' }}>Tip: set the institution's active academic year in Settings so this fills in automatically.</p>}
            </label>
          </div>
          <p style={{ fontSize: 12, color: 'var(--muted)', margin: '12px 0 0' }}>
            The student is identified by their registration number — that is what they type to check in and to open their progress view in the app.
          </p>
          <button
            onClick={handleCreate}
            disabled={saving || !form.student_id || !form.full_name || !form.offering_id || !form.academic_year}
            style={{ ...btnPrimary, marginTop: 16 }}>
            {saving ? 'Registering…' : 'Register Student'}
          </button>
        </div>
      )}

      <div style={{ marginBottom: 12 }}>
        <input value={search} onChange={e => setSearch(e.target.value)}
          placeholder="Search by name, registration number or course…"
          style={{ width: '100%', padding: '9px 12px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }} />
      </div>

      {/* Filter bar */}
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginBottom: 16, alignItems: 'center' }}>
        <FilterSelect label="Course" value={filters.course} onChange={v => setF('course', v)}
          options={(courses ?? []).map(c => ({ value: c.course_id, label: c.name }))} />
        <FilterSelect label="Year" value={filters.year} onChange={v => setF('year', v)}
          options={[1, 2, 3, 4, 5, 6].map(y => ({ value: String(y), label: `Year ${y}` }))} />
        <FilterSelect label="Semester" value={filters.semester} onChange={v => setF('semester', v)}
          options={[{ value: '1', label: 'Sem 1' }, { value: '2', label: 'Sem 2' }]} />
        <FilterSelect label="Intake" value={filters.intake} onChange={v => setF('intake', v)}
          options={intakes.map(i => ({ value: i, label: i }))} />
        <FilterSelect label="Academic Yr" value={filters.academic} onChange={v => setF('academic', v)}
          options={academics.map(a => ({ value: a, label: a }))} />
        <FilterSelect label="Status" value={filters.status} onChange={v => setF('status', v)}
          options={statuses.map(s => ({ value: s, label: s }))} />
        {(Object.values(filters).some(Boolean) || search) && (
          <button onClick={() => { setFilters({ course: '', year: '', semester: '', intake: '', academic: '', status: '' }); setSearch('') }}
            style={{ padding: '7px 12px', border: '1px solid #e2e8f0', borderRadius: 6, background: '#fff', cursor: 'pointer', fontSize: 12 }}>
            Clear all
          </button>
        )}
      </div>

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}

      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
        <thead>
          <tr style={{ background: '#f8fafc' }}>
            {['Reg No.', 'Name', 'Course', 'Year/Sem', 'Intake', 'Status', 'Actions'].map(h => (
              <th key={h} style={{ padding: '8px 12px', textAlign: 'left', borderBottom: '1px solid #e2e8f0' }}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {list.map(s => (
            <tr key={s.student_id} style={{ borderBottom: '1px solid #f1f5f9' }}>
              <td style={{ padding: '10px 12px', fontFamily: 'monospace', fontSize: 12 }}>{s.student_id}</td>
              <td style={{ padding: '10px 12px', fontWeight: 600 }}>{s.full_name}</td>
              <td style={{ padding: '10px 12px', color: 'var(--muted)' }}>{s.course_name}</td>
              <td style={{ padding: '10px 12px', textAlign: 'center' }}>Y{s.current_year}/S{s.semester}</td>
              <td style={{ padding: '10px 12px', color: 'var(--muted)', fontSize: 12 }}>{s.intake_session}</td>
              <td style={{ padding: '10px 12px' }}>
                <span style={{
                  background: s.enrollment_status === 'ACTIVE' ? '#f0fdf4' : '#fef2f2',
                  color:      s.enrollment_status === 'ACTIVE' ? '#166534' : '#b91c1c',
                  padding: '2px 8px', borderRadius: 999, fontSize: 11, fontWeight: 600,
                }}>
                  {s.enrollment_status}
                </span>
              </td>
              <td style={{ padding: '10px 12px', whiteSpace: 'nowrap' }}>
                <button onClick={() => setEditStudent(s)}
                  style={{ padding: '4px 10px', border: '1px solid #c7d2fe', color: '#3730a3', borderRadius: 4, cursor: 'pointer', fontSize: 12, background: '#eef2ff', marginRight: 6 }}>
                  Edit
                </button>
                <button onClick={() => handleDelete(s)}
                  style={{ padding: '4px 10px', border: '1px solid #fecaca', color: '#b91c1c', borderRadius: 4, cursor: 'pointer', fontSize: 12, background: '#fef2f2' }}>
                  Delete
                </button>
              </td>
            </tr>
          ))}
          {status === 'ok' && list.length === 0 && (
            <tr>
              <td colSpan={7} style={{ padding: 32, textAlign: 'center', color: 'var(--muted)' }}>
                {(students ?? []).length === 0
                  ? 'No students registered yet. Click "+ Register Student" to add one.'
                  : 'No students match the search.'}
              </td>
            </tr>
          )}
        </tbody>
      </table>

      {status === 'ok' && (students ?? []).length > 0 && (
        <p style={{ color: 'var(--muted)', fontSize: 12, marginTop: 12 }}>
          Showing {list.length} of {(students ?? []).length} students
        </p>
      )}

      {editStudent && (
        <EditStudentModal
          tenantId={tenantId!}
          student={editStudent}
          offerings={offerings ?? []}
          intakeOptions={intakeOptions}
          onClose={() => setEditStudent(null)}
          onSaved={() => { setEditStudent(null); refetch() }}
        />
      )}
    </div>
  )
}

// Edit an existing student. Sends the full editable field set to PATCH
// /students (keyed by student_id in the body, since reg numbers may contain '/').
function EditStudentModal({ tenantId, student, offerings, intakeOptions, onClose, onSaved }: {
  tenantId: string
  student: Student
  offerings: Offering[]
  intakeOptions: string[]
  onClose: () => void
  onSaved: () => void
}) {
  const [f, setF] = useState({
    full_name: student.full_name,
    current_year: student.current_year,
    semester: student.semester,
    academic_year: student.academic_year,
    intake_session: student.intake_session,
    enrollment_status: student.enrollment_status || 'ACTIVE',
    offering_id: '',
  })
  const [saving, setSaving] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  async function save() {
    setSaving(true); setErr(null)
    try {
      await api.patch(`/api/v1/admin/tenants/${tenantId}/students`, { student_id: student.student_id, ...f })
      onSaved()
    } catch (e) { setErr(e instanceof Error ? e.message : 'Update failed') }
    finally { setSaving(false) }
  }
  return (
    <div onClick={onClose} style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.6)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 9999, padding: 20 }}>
      <div onClick={e => e.stopPropagation()} style={{ background: '#fff', borderRadius: 14, padding: 24, maxWidth: 460, width: '100%', maxHeight: '90vh', overflowY: 'auto' }}>
        <h3 style={{ margin: '0 0 2px' }}>Edit student</h3>
        <div style={{ fontSize: 12, color: 'var(--muted)', fontFamily: 'monospace', marginBottom: 16 }}>{student.student_id}</div>
        {err && <div style={errorBox}>{err}</div>}
        <Input label="Full Name" value={f.full_name} onChange={v => setF(s => ({ ...s, full_name: v }))} />
        <div style={{ display: 'flex', gap: 10 }}>
          <div style={{ flex: 1 }}>
            <label style={editLabel}>Year</label>
            <select value={f.current_year} onChange={e => setF(s => ({ ...s, current_year: Number(e.target.value) }))} style={editSelect}>
              {[1, 2, 3, 4, 5, 6].map(y => <option key={y} value={y}>Year {y}</option>)}
            </select>
          </div>
          <div style={{ flex: 1 }}>
            <label style={editLabel}>Semester</label>
            <select value={f.semester} onChange={e => setF(s => ({ ...s, semester: Number(e.target.value) }))} style={editSelect}>
              {[1, 2].map(y => <option key={y} value={y}>Sem {y}</option>)}
            </select>
          </div>
        </div>
        <Input label="Academic Year" value={f.academic_year} onChange={v => setF(s => ({ ...s, academic_year: v }))} placeholder="e.g. 2024/2025" />
        <label style={editLabel}>Intake</label>
        <select value={f.intake_session} onChange={e => setF(s => ({ ...s, intake_session: e.target.value }))} style={editSelect}>
          {Array.from(new Set([f.intake_session, ...intakeOptions])).filter(Boolean).map(i => <option key={i} value={i}>{i}</option>)}
        </select>
        <label style={editLabel}>Status</label>
        <select value={f.enrollment_status} onChange={e => setF(s => ({ ...s, enrollment_status: e.target.value }))} style={editSelect}>
          {['ACTIVE', 'INACTIVE', 'SUSPENDED', 'GRADUATED'].map(s => <option key={s} value={s}>{s}</option>)}
        </select>
        <label style={editLabel}>Reassign session (optional)</label>
        <select value={f.offering_id} onChange={e => setF(s => ({ ...s, offering_id: e.target.value }))} style={editSelect}>
          <option value="">— keep current —</option>
          {offerings.map(o => <option key={o.offering_id} value={o.offering_id}>{o.course_name} · {cohortLabel(o)}</option>)}
        </select>
        <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 18 }}>
          <button onClick={onClose} style={btnGhost}>Cancel</button>
          <button onClick={save} disabled={saving} style={btnPrimary}>{saving ? 'Saving…' : 'Save changes'}</button>
        </div>
      </div>
    </div>
  )
}

const editLabel: React.CSSProperties = { display: 'block', fontSize: 12, fontWeight: 600, color: '#475569', margin: '12px 0 4px' }
const editSelect: React.CSSProperties = { width: '100%', padding: '9px 10px', border: '1px solid #cbd5e1', borderRadius: 8, fontSize: 14, boxSizing: 'border-box', background: '#fff' }

function FilterSelect({ label, value, onChange, options }: {
  label: string; value: string; onChange: (v: string) => void; options: { value: string; label: string }[]
}) {
  return (
    <select value={value} onChange={e => onChange(e.target.value)}
      style={{
        padding: '7px 10px', borderRadius: 6, fontSize: 13,
        border: value ? '1px solid #1d4ed8' : '1px solid #e2e8f0',
        background: value ? '#eff6ff' : '#fff', color: '#0f172a', cursor: 'pointer',
      }}>
      <option value="">{label}: all</option>
      {options.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
    </select>
  )
}

function Input({ label, value, onChange, placeholder, type = 'text' }: {
  label: string; value: string; onChange: (v: string) => void; placeholder?: string; type?: string
}) {
  return (
    <label style={{ display: 'block' }}>
      <div style={labelStyle}>{label}</div>
      <input type={type} value={value} placeholder={placeholder} onChange={e => onChange(e.target.value)}
        style={{ width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }} />
    </label>
  )
}

const btnPrimary: React.CSSProperties = { padding: '8px 16px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const btnGhost: React.CSSProperties = { padding: '8px 12px', background: '#fff', color: '#334155', border: '1px solid #e2e8f0', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const labelStyle: React.CSSProperties = { fontSize: 13, fontWeight: 600, marginBottom: 4, color: '#475569' }
const selectStyle: React.CSSProperties = { width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14 }
// Fields inherited from the chosen cohort: shown filled but locked (no need to re-enter).
const greyed: React.CSSProperties = { ...selectStyle, background: '#eef2f7', color: '#64748b', cursor: 'not-allowed' }
const errorBox:   React.CSSProperties = { background: '#fef2f2', color: '#b91c1c', padding: '8px 12px', borderRadius: 6, marginBottom: 12, fontSize: 13 }
