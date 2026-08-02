import { useState, useEffect } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

interface Assignment {
  assignment_id: string
  lecturer_id:   string
  lecturer_name: string
  unit_id:       string
  unit_name:     string
  course_id:     string
  course_name:   string
  academic_year: string
  year:          number
  semester:      number
  intake_session: string
}

interface Lecturer { lecturer_id: string; full_name: string }
interface Course   { course_id: string;   name: string }
interface Unit     { unit_id: string;     name: string; year: number; semester: number }

// Last-resort fallback only; the real options come from the tenant's configured
// study sessions (Admin → Courses → "Manage sessions").
const SESSION_FALLBACK = ['Day', 'Evening', 'Weekend']

export default function AdminLecturerAssignments() {
  const { tenantId } = useParams<{ tenantId: string }>()

  const { status, data, refetch } = useQuery<Assignment[]>(
    () => api.get(`/api/v1/admin/tenants/${tenantId}/lecturer-assignments`)
  )

  // Study sessions (Day/Evening/…) are tenant-configurable — use the SAME list
  // everywhere so a removed/added session is reflected here too.
  const sessionsQ = useQuery<{ study_sessions: string[] }>(() => api.get('/api/v1/admin/settings/study-sessions'), [tenantId])
  const sessionOptions = (sessionsQ.status === 'ok' ? sessionsQ.data?.study_sessions : null)?.length
    ? (sessionsQ.data!.study_sessions) : SESSION_FALLBACK

  // Dropdowns data
  const [lecturers, setLecturers] = useState<Lecturer[]>([])
  const [courses,   setCourses]   = useState<Course[]>([])
  const [units,     setUnits]     = useState<Unit[]>([])

  const [creating, setCreating] = useState(false)
  const [form, setForm] = useState({
    lecturer_id: '', course_id: '', unit_id: '',
    academic_year: '', year: 1, semester: 1, intake_session: '',
  })
  const [saving,  setSaving]  = useState(false)
  const [deleting, setDeleting] = useState<string | null>(null)
  const [error,   setError]   = useState<string | null>(null)

  // Load lecturers + courses up front (needed for the grouped view + add-unit).
  useEffect(() => {
    api.get<Lecturer[]>(`/api/v1/admin/tenants/${tenantId}/lecturers`)
      .then(setLecturers).catch(() => {})
    api.get<Course[]>(`/api/v1/admin/tenants/${tenantId}/courses`)
      .then(setCourses).catch(() => {})
  }, [tenantId])

  // Open the create panel with a lecturer pre-selected ("add another unit").
  function startAddUnit(lecturerId: string) {
    setForm(f => ({ ...f, lecturer_id: lecturerId, course_id: '', unit_id: '' }))
    setCreating(true)
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  // Load units when course changes
  useEffect(() => {
    if (!form.course_id) { setUnits([]); return }
    api.get<Unit[]>(`/api/v1/admin/courses/${form.course_id}/units`)
      .then(setUnits).catch(() => setUnits([]))
  }, [form.course_id])

  async function handleCreate() {
    setSaving(true); setError(null)
    try {
      await api.post(`/api/v1/admin/tenants/${tenantId}/lecturer-assignments`, form)
      setCreating(false)
      setForm({ lecturer_id: '', course_id: '', unit_id: '', academic_year: '', year: 1, semester: 1, intake_session: '' })
      refetch()
    } catch (e) { setError(e instanceof Error ? e.message : 'Failed') }
    finally { setSaving(false) }
  }

  async function handleDelete(assignmentId: string) {
    if (!confirm('Remove this assignment?')) return
    setDeleting(assignmentId)
    try {
      await api.delete(`/api/v1/admin/lecturer-assignments/${assignmentId}`)
      refetch()
    } catch (e) { alert(e instanceof Error ? e.message : 'Delete failed') }
    finally { setDeleting(null) }
  }

  const assignments = status === 'ok' ? data : []
  const canSubmit = form.lecturer_id && form.unit_id && form.course_id && form.academic_year && form.intake_session

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <div>
          <h2 style={{ margin: '0' }}>Lecturer Assignments</h2>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <a href={`/admin/tenants/${tenantId}/lecturers`} style={{ ...btnSmall, textDecoration: 'none', display: 'inline-block' }}>
            Manage Lecturers
          </a>
          <button onClick={() => setCreating(c => !c)} style={btnPrimary}>
            {creating ? 'Cancel' : '+ New Assignment'}
          </button>
        </div>
      </div>

      {creating && (
        <div style={{ background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 10, padding: 20, marginBottom: 24 }}>
          <h3 style={{ margin: '0 0 16px' }}>Assign Lecturer to Course Unit</h3>
          {error && <div style={errorBox}>{error}</div>}

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            {/* Lecturer dropdown */}
            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Lecturer *</div>
              <select value={form.lecturer_id} onChange={e => setForm(f => ({ ...f, lecturer_id: e.target.value }))} style={selectStyle}>
                <option value="">Select lecturer…</option>
                {lecturers.map(l => <option key={l.lecturer_id} value={l.lecturer_id}>{l.full_name}</option>)}
              </select>
            </label>

            {/* Course dropdown */}
            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Course *</div>
              <select value={form.course_id} onChange={e => setForm(f => ({ ...f, course_id: e.target.value, unit_id: '' }))} style={selectStyle}>
                <option value="">Select course…</option>
                {courses.map(c => <option key={c.course_id} value={c.course_id}>{c.name}</option>)}
              </select>
            </label>

            {/* Unit dropdown — cascades from course */}
            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Course Unit *</div>
              <select value={form.unit_id} onChange={e => setForm(f => ({ ...f, unit_id: e.target.value }))} style={selectStyle} disabled={!form.course_id}>
                <option value="">Select unit…</option>
                {units.map(u => (
                  <option key={u.unit_id} value={u.unit_id}>
                    {u.name} (Yr{u.year} S{u.semester})
                  </option>
                ))}
              </select>
            </label>

            {/* Academic year */}
            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Academic Year *</div>
              <input value={form.academic_year} placeholder="2024/2025"
                onChange={e => setForm(f => ({ ...f, academic_year: e.target.value }))}
                style={{ ...selectStyle }} />
            </label>

            {/* Year of study */}
            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Year of Study</div>
              <select value={form.year} onChange={e => setForm(f => ({ ...f, year: Number(e.target.value) }))} style={selectStyle}>
                {[1, 2, 3, 4, 5].map(y => <option key={y} value={y}>Year {y}</option>)}
              </select>
            </label>

            {/* Semester */}
            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Semester</div>
              <select value={form.semester} onChange={e => setForm(f => ({ ...f, semester: Number(e.target.value) }))} style={selectStyle}>
                <option value={1}>Semester 1</option>
                <option value={2}>Semester 2</option>
              </select>
            </label>

            {/* Study session (tenant-configured: Day/Evening/…) */}
            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Study Session</div>
              <select value={form.intake_session} onChange={e => setForm(f => ({ ...f, intake_session: e.target.value }))} style={selectStyle}>
                <option value="">— select session —</option>
                {sessionOptions.map(s => <option key={s} value={s}>{s}</option>)}
              </select>
            </label>
          </div>

          <button onClick={handleCreate} disabled={saving || !canSubmit}
            style={{ ...btnPrimary, marginTop: 16, opacity: !canSubmit ? 0.5 : 1 }}>
            {saving ? 'Saving…' : 'Create Assignment'}
          </button>
        </div>
      )}

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}

      {/* Grouped by lecturer — one card per lecturer listing ALL their units, so a
          lecturer teaching 6 units appears once (not on 6 separate rows). */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        {groupByLecturer(assignments ?? []).map(g => (
          <div key={g.lecturer_id} style={{ border: '1px solid #e2e8f0', borderRadius: 10, padding: '14px 18px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 10 }}>
              <div style={{ fontWeight: 700, fontSize: 15 }}>{g.lecturer_name}
                <span style={{ marginLeft: 8, fontSize: 12, fontWeight: 600, color: 'var(--muted)' }}>
                  · {g.items.length} unit{g.items.length > 1 ? 's' : ''}
                </span>
              </div>
              <button onClick={() => startAddUnit(g.lecturer_id)} style={btnSmall}>+ Add unit</button>
            </div>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
              {g.items.map(a => (
                <span key={a.assignment_id} style={{ display: 'inline-flex', alignItems: 'center', gap: 8, background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 8, padding: '6px 10px', fontSize: 13 }}>
                  <span><strong>{a.unit_name}</strong> <span style={{ color: 'var(--muted)', fontSize: 11 }}>{a.unit_id}</span>
                    <span style={{ color: 'var(--muted)' }}> · {a.course_name} · {a.academic_year}</span></span>
                  <button onClick={() => handleDelete(a.assignment_id)} disabled={deleting === a.assignment_id}
                    title="Remove this unit" style={{ border: 'none', background: 'transparent', color: '#b91c1c', cursor: 'pointer', fontSize: 15, lineHeight: 1 }}>×</button>
                </span>
              ))}
            </div>
          </div>
        ))}
        {(assignments ?? []).length === 0 && status === 'ok' && (
          <p style={{ padding: 24, textAlign: 'center', color: 'var(--muted)' }}>
            No assignments yet. Click "+ New Assignment" to assign a lecturer to a course unit.
          </p>
        )}
      </div>
    </div>
  )
}

// groupByLecturer collapses the flat assignment list into one entry per lecturer.
function groupByLecturer(rows: Assignment[]) {
  const map = new Map<string, { lecturer_id: string; lecturer_name: string; items: Assignment[] }>()
  for (const a of rows) {
    const g = map.get(a.lecturer_id) ?? { lecturer_id: a.lecturer_id, lecturer_name: a.lecturer_name, items: [] }
    g.items.push(a)
    map.set(a.lecturer_id, g)
  }
  return Array.from(map.values()).sort((x, y) => x.lecturer_name.localeCompare(y.lecturer_name))
}

const btnPrimary: React.CSSProperties = { padding: '8px 16px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const btnSmall:   React.CSSProperties = { padding: '6px 12px', background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 4, cursor: 'pointer', fontSize: 12, color: '#1e293b' }
const errorBox:   React.CSSProperties = { background: '#fef2f2', color: '#b91c1c', padding: '8px 12px', borderRadius: 6, marginBottom: 12, fontSize: 13 }
const labelStyle: React.CSSProperties = { fontSize: 13, fontWeight: 600, marginBottom: 4, color: '#475569' }
const selectStyle: React.CSSProperties = { width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box', background: '#fff' }
