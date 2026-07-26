import { useState, useEffect, useMemo, useRef } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'
import { useAuth } from '../../contexts/AuthContext'

interface Unit {
  unit_id:          string
  name:             string
  year:             number
  semester:         number
  level:            string
  academic_year:    string
  default_venue_id: string
  session_start:           string
  session_duration_minutes: number
  schedule_locked:         boolean
}

interface Roadmap {
  course_id:        string
  tenant_id:        string
  name:             string
  department:       string
  school:           string
  coordinator_name: string
  total_years:      number
  level_years?:     Record<string, number>
  roadmap:          Record<string, Record<string, Unit[]>>
}

interface Venue { venue_id: string; name: string; building: string }

// Default form pre-filled when user clicks "+ Add Unit" for a specific year/semester
const blankForm = { unit_id: '', name: '', year: 1, semester: 1, level: '', academic_year: '', default_venue_id: '' }
const NO_LEVEL = '— No level —'

export default function AdminCourseUnits() {
  const { courseId } = useParams<{ courseId: string }>()
  const { status, data: roadmap, refetch } = useQuery<Roadmap>(
    () => api.get(`/api/v1/admin/courses/${courseId}/roadmap`),
    [courseId],
  )
  const levelsQ = useQuery<{ levels: string[]; level_years?: Record<string, number> }>(() => api.get('/api/v1/admin/settings/levels'))
  const tenantLevels = (levelsQ.status === 'ok' ? levelsQ.data?.levels : null) ?? []
  const levelYears = (levelsQ.status === 'ok' ? levelsQ.data?.level_years : null) ?? {}

  // Units are per LEVEL — a Degree's units differ from a Masters'/PhD's. The admin
  // switches level here; each level keeps its own curriculum.
  const [level, setLevel] = useState<string>('')
  const presentLevels = useMemo(() => {
    const set = new Set<string>()
    const m = roadmap?.roadmap ?? {}
    for (const y of Object.values(m)) for (const sem of Object.values(y)) for (const u of sem) set.add(u.level || '')
    return set
  }, [roadmap])
  // Switcher options: tenant levels + any level actually present (incl. '' = no level).
  const levelOptions = useMemo(() => {
    const arr = Array.from(new Set([...tenantLevels, ...presentLevels]))
    return arr.length ? arr : ['']
  }, [tenantLevels, presentLevels])
  // Pick a sensible initial level once data is in.
  useEffect(() => {
    if (level === '' && !presentLevels.has('') && levelOptions.length) setLevel(levelOptions[0])
  }, [levelOptions]) // eslint-disable-line react-hooks/exhaustive-deps

  const [venues, setVenues] = useState<Venue[]>([])

  useEffect(() => {
    if (!roadmap?.tenant_id) return
    api.get<Venue[]>(`/api/v1/admin/tenants/${roadmap.tenant_id}/venues`)
      .then(v => setVenues(v))
      .catch(() => setVenues([]))
  }, [roadmap?.tenant_id])

  // Add-unit panel state
  const [addForm,  setAddForm]  = useState<typeof blankForm | null>(null)
  const [saving,   setSaving]   = useState(false)
  const [addError, setAddError] = useState<string | null>(null)

  // Edit-unit state
  const [editUnit,   setEditUnit]   = useState<Unit | null>(null)
  const [editForm,   setEditForm]   = useState<Partial<Unit>>({})
  const [editSaving, setEditSaving] = useState(false)
  const [editError,  setEditError]  = useState<string | null>(null)

  async function handleAdd() {
    if (!addForm) return
    setSaving(true); setAddError(null)
    try {
      await api.post(`/api/v1/admin/courses/${courseId}/units`, addForm)
      setAddForm(null)
      refetch()
    } catch (e) { setAddError(e instanceof Error ? e.message : 'Failed') }
    finally { setSaving(false) }
  }

  async function handleEditSave() {
    if (!editUnit) return
    setEditSaving(true); setEditError(null)
    try {
      await api.patch(`/api/v1/admin/courses/${courseId}/units/${editUnit.unit_id}`, editForm)
      setEditUnit(null)
      refetch()
    } catch (e) { setEditError(e instanceof Error ? e.message : 'Failed') }
    finally { setEditSaving(false) }
  }

  function startEdit(u: Unit) {
    setEditUnit(u)
    setEditForm({ name: u.name, level: u.level, default_venue_id: u.default_venue_id, session_start: u.session_start, session_duration_minutes: u.session_duration_minutes })
  }

  const courseLevelYears = roadmap?.level_years ?? {}
  const totalYears = courseLevelYears[level] || levelYears[level] || roadmap?.total_years || 3
  // Build year range 1..totalYears
  const years = Array.from({ length: totalYears }, (_, i) => i + 1)

  if (status === 'loading') return <p style={{ padding: 24, color: 'var(--muted)' }}>Loading roadmap…</p>
  if (status === 'error')   return <p style={{ padding: 24, color: '#b91c1c' }}>Could not load roadmap.</p>

  const unitMap = roadmap?.roadmap ?? {}

  return (
    <div>
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 24 }}>
        <div>
          <a href="#" onClick={() => history.back()} style={{ color: 'var(--muted)', fontSize: 13, textDecoration: 'none' }}>← Back to Courses</a>
          <h2 style={{ margin: '4px 0 2px' }}>{roadmap?.name ?? courseId}</h2>
          <div style={{ fontSize: 13, color: 'var(--muted)', display: 'flex', gap: 16 }}>
            {roadmap?.department && <span>{roadmap.department}</span>}
            {roadmap?.school && <span>· {roadmap.school}</span>}
            {roadmap?.coordinator_name && <span>· Coordinator: <strong>{roadmap.coordinator_name}</strong></span>}
          </div>
          <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 4 }}>
            Course ID: <code>{courseId}</code> &nbsp;·&nbsp; {totalYears}-year programme
          </div>
        </div>
        <div style={{ display: 'flex', gap: 12, alignItems: 'flex-end' }}>
          <label style={{ display: 'block', minWidth: 160 }}>
            <div style={{ ...labelStyle, textAlign: 'right' }}>Level (each has its own units)</div>
            <select value={level} onChange={e => setLevel(e.target.value)} style={selectStyle}>
              {levelOptions.map(l => <option key={l} value={l}>{l === '' ? NO_LEVEL : l}</option>)}
            </select>
          </label>
          {level !== '' && <LevelYearsControl courseId={courseId!} level={level} allYears={courseLevelYears} fallback={levelYears[level] || 3} onSaved={refetch} />}
        </div>
      </div>

      {/* Bulk import/export this course's units (the roadmap) */}
      <UnitsImportBar courseId={courseId!} onDone={refetch} />

      {/* Add-unit panel */}
      {addForm && (
        <div style={{ background: '#f0f9ff', border: '1px solid #bae6fd', borderRadius: 10, padding: 20, marginBottom: 24 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 14 }}>
            <strong style={{ fontSize: 14 }}>
              Add Unit — {addForm.level || NO_LEVEL} · Year {addForm.year}, Semester {addForm.semester}
            </strong>
            <button onClick={() => setAddForm(null)} style={btnGhost}>✕ Cancel</button>
          </div>
          {addError && <div style={errorBox}>{addError}</div>}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <Field label="Unit ID *" value={addForm.unit_id} onChange={v => setAddForm(f => f && ({ ...f, unit_id: v }))} placeholder="e.g. CS201" />
            <Field label="Unit Name *" value={addForm.name} onChange={v => setAddForm(f => f && ({ ...f, name: v }))} placeholder="e.g. Data Structures" />
            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Default Venue (optional)</div>
              <select value={addForm.default_venue_id} onChange={e => setAddForm(f => f && ({ ...f, default_venue_id: e.target.value }))} style={selectStyle}>
                <option value="">— No default venue —</option>
                {venues.map(v => <option key={v.venue_id} value={v.venue_id}>{v.name} {v.building ? `(${v.building})` : ''}</option>)}
              </select>
            </label>
          </div>
          <button
            disabled={saving || !addForm.unit_id || !addForm.name}
            onClick={handleAdd}
            style={{ ...btnPrimary, marginTop: 14, opacity: (!addForm.unit_id || !addForm.name) ? 0.5 : 1 }}>
            {saving ? 'Adding…' : 'Add Unit'}
          </button>
        </div>
      )}

      {/* Edit-unit panel */}
      {editUnit && (
        <div style={{ background: '#fefce8', border: '1px solid #fde68a', borderRadius: 10, padding: 20, marginBottom: 24 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 14 }}>
            <strong style={{ fontSize: 14 }}>Edit Unit: {editUnit.unit_id}</strong>
            <button onClick={() => setEditUnit(null)} style={btnGhost}>✕ Cancel</button>
          </div>
          {editError && <div style={errorBox}>{editError}</div>}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <Field label="Name" value={editForm.name ?? ''} onChange={v => setEditForm(f => ({ ...f, name: v }))} />
            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Year of Study</div>
              <select value={editForm.year ?? editUnit.year} onChange={e => setEditForm(f => ({ ...f, year: Number(e.target.value) }))} style={selectStyle}>
                {years.map(y => <option key={y} value={y}>Year {y}</option>)}
              </select>
            </label>
            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Semester</div>
              <select value={editForm.semester ?? editUnit.semester} onChange={e => setEditForm(f => ({ ...f, semester: Number(e.target.value) }))} style={selectStyle}>
                <option value={1}>Semester 1</option>
                <option value={2}>Semester 2</option>
              </select>
            </label>
            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Level</div>
              <select value={editForm.level ?? editUnit.level ?? ''} onChange={e => setEditForm(f => ({ ...f, level: e.target.value }))} style={selectStyle}>
                {levelOptions.map(l => <option key={l} value={l}>{l === '' ? NO_LEVEL : l}</option>)}
              </select>
            </label>
            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Default Venue</div>
              <select value={editForm.default_venue_id ?? editUnit.default_venue_id ?? ''} onChange={e => setEditForm(f => ({ ...f, default_venue_id: e.target.value }))} style={selectStyle}>
                <option value="">— No default venue —</option>
                {venues.map(v => <option key={v.venue_id} value={v.venue_id}>{v.name}</option>)}
              </select>
            </label>
            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Session start {editUnit.schedule_locked && <span style={{ color: 'var(--muted)', fontWeight: 400 }}>(set by coordinator — you can override)</span>}</div>
              <input type="time" value={editForm.session_start ?? ''} onChange={e => setEditForm(f => ({ ...f, session_start: e.target.value }))}
                style={{ width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }} />
            </label>
            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Session length (min)</div>
              <input type="number" min={5} max={600} value={editForm.session_duration_minutes ?? 0} onChange={e => setEditForm(f => ({ ...f, session_duration_minutes: Number(e.target.value) }))}
                style={{ width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }} />
            </label>
          </div>
          <button disabled={editSaving} onClick={handleEditSave} style={{ ...btnPrimary, marginTop: 14, background: '#92400e' }}>
            {editSaving ? 'Saving…' : 'Save Changes'}
          </button>
        </div>
      )}

      {/* Roadmap grid — Year > Semester > Units */}
      {years.map(year => (
        <div key={year} style={{ marginBottom: 32 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 12 }}>
            <h3 style={{ margin: 0, fontSize: 16, color: '#1e293b' }}>Year {year}</h3>
            <div style={{ flex: 1, height: 1, background: '#e2e8f0' }} />
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
            {[1, 2].map(sem => {
              const units: Unit[] = (unitMap[year]?.[sem] ?? []).filter(u => (u.level || '') === level)
              return (
                <div key={sem} style={{ border: '1px solid #e2e8f0', borderRadius: 10, overflow: 'hidden' }}>
                  {/* Semester header */}
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '10px 14px', background: '#f8fafc', borderBottom: '1px solid #e2e8f0' }}>
                    <span style={{ fontWeight: 700, fontSize: 13, color: '#475569' }}>Semester {sem}</span>
                    <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
                      <span style={{ fontSize: 11, color: 'var(--muted)' }}>{units.length} unit{units.length !== 1 ? 's' : ''}</span>
                      <button
                        onClick={() => {
                          setAddForm({ ...blankForm, year, semester: sem, level })
                          setEditUnit(null)
                        }}
                        style={{ ...btnSmall, fontSize: 11 }}>
                        + Add Unit
                      </button>
                    </div>
                  </div>

                  {/* Unit list */}
                  <div>
                    {units.length === 0 && (
                      <div style={{ padding: '16px 14px', color: 'var(--muted)', fontSize: 13, textAlign: 'center' }}>
                        No units yet for this semester.
                      </div>
                    )}
                    {units.map((u, idx) => (
                      <div key={u.unit_id} style={{
                        display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                        padding: '10px 14px',
                        borderBottom: idx < units.length - 1 ? '1px solid #f1f5f9' : 'none',
                        background: editUnit?.unit_id === u.unit_id ? '#fefce8' : '#fff',
                      }}>
                        <div>
                          <div style={{ fontWeight: 600, fontSize: 13 }}>{u.name}</div>
                          <div style={{ fontSize: 11, color: 'var(--muted)', marginTop: 2 }}>
                            {u.unit_id}
                            {u.academic_year && ` · ${u.academic_year}`}
                            {u.default_venue_id && ` · venue: ${u.default_venue_id}`}
                          </div>
                        </div>
                        <button
                          onClick={() => startEdit(u)}
                          style={{ ...btnSmall, fontSize: 11 }}>
                          Edit
                        </button>
                      </div>
                    ))}
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      ))}

      {years.length === 0 && (
        <p style={{ color: 'var(--muted)', textAlign: 'center', padding: 32 }}>
          No years configured. Set total_years on the course to define the roadmap.
        </p>
      )}
    </div>
  )
}

// Sets how many YEARS this level runs FOR THIS COURSE (a Masters may be 3 years in
// CS but 2 in SWE). Saved onto the course's level_years map.
function LevelYearsControl({ courseId, level, allYears, fallback, onSaved }: {
  courseId: string; level: string; allYears: Record<string, number>; fallback: number; onSaved: () => void
}) {
  const [years, setYears] = useState(allYears[level] || fallback)
  const [busy, setBusy] = useState(false)
  useEffect(() => { setYears(allYears[level] || fallback) }, [level, allYears, fallback])
  async function save() {
    setBusy(true)
    try {
      await api.patch(`/api/v1/admin/courses/${courseId}`, { level_years: { ...allYears, [level]: years } })
      onSaved()
    } catch (e) { alert(e instanceof Error ? e.message : 'Failed') }
    finally { setBusy(false) }
  }
  const dirty = years !== (allYears[level] || fallback)
  return (
    <label style={{ display: 'block' }}>
      <div style={{ ...labelStyle, textAlign: 'right' }}>Years for {level}</div>
      <div style={{ display: 'flex', gap: 6 }}>
        <input type="number" min={1} max={10} value={years} onChange={e => setYears(Number(e.target.value))}
          style={{ width: 64, padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14 }} />
        <button onClick={save} disabled={busy || !dirty} style={{ padding: '8px 12px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontSize: 13, opacity: !dirty ? 0.5 : 1 }}>{busy ? '…' : 'Save'}</button>
      </div>
    </label>
  )
}

function Field({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (v: string) => void; placeholder?: string }) {
  return (
    <label style={{ display: 'block' }}>
      <div style={labelStyle}>{label}</div>
      <input value={value} placeholder={placeholder} onChange={e => onChange(e.target.value)}
        style={{ width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }} />
    </label>
  )
}

// Import/export this course's units. The import sends course_id so rows that omit
// it are filed under this course; the export downloads the units in the same
// columns (template). Unknown columns are ignored.
function UnitsImportBar({ courseId, onDone }: { courseId: string; onDone: () => void }) {
  const { user } = useAuth()
  const tenantId = user?.tenantId ?? ''
  const ref = useRef<HTMLInputElement | null>(null)
  const [busy, setBusy] = useState(false)
  const [msg, setMsg] = useState<string | null>(null)
  type Transfer = { unit_id: string; name: string; from_course: string; to_course: string }
  type ImportResp = { inserted: number; updated: number; skipped: number; errors: string[]; transfers?: Transfer[] }
  function runImport(file: File, confirmTransfers: boolean) {
    const fd = new FormData()
    fd.append('roster', file)
    fd.append('target_course', courseId) // this page's course is the destination
    if (confirmTransfers) fd.append('confirm_transfers', 'true')
    return api.upload<ImportResp>(`/api/v1/admin/tenants/${tenantId}/course-units/import`, fd)
  }
  const summarise = (r: ImportResp) => `✓ ${r.inserted} new, ${r.updated} updated, ${r.skipped} skipped${r.errors?.length ? ` · ${r.errors.slice(0, 2).join('; ')}` : ''}`

  async function doImport(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    setBusy(true); setMsg(null)
    try {
      const r = await runImport(file, false)
      // Some units already belong to another course — moving them needs consent.
      if (r.transfers && r.transfers.length) {
        const lines = r.transfers.map(t => `• ${t.unit_id} (${t.name}) — currently in "${t.from_course}", move to "${t.to_course}"`).join('\n')
        const ok = window.confirm(
          `${r.transfers.length} unit(s) already belong to another course.\n\n` +
          `Reason: these unit codes already exist under a different course in this tenant. ` +
          `Importing them here will MOVE them to this course:\n\n${lines}\n\nMove them here?`)
        if (ok) {
          setMsg(summarise(await runImport(file, true)))
        } else {
          setMsg(`Imported ${r.inserted} new, ${r.updated} updated · ${r.transfers.length} cross-course unit(s) left where they are.`)
        }
      } else {
        setMsg(summarise(r))
      }
      onDone()
    } catch (err) { setMsg(`✗ ${err instanceof Error ? err.message : 'Import failed'}`) }
    finally { setBusy(false); if (ref.current) ref.current.value = '' }
  }
  return (
    <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap', marginBottom: 16, padding: '8px 12px', background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 8 }}>
      <span style={{ fontSize: 13, fontWeight: 600, color: '#475569' }}>Bulk units:</span>
      <span style={{ fontSize: 11, color: 'var(--muted)', flex: '1 1 200px' }}>columns — unit_id, name, year, semester, level · units land in THIS course (a unit already under another course asks before moving)</span>
      <button onClick={() => api.download(`/api/v1/admin/tenants/${tenantId}/course-units/export.xlsx`, 'course-units.xlsx').catch(e => alert(e instanceof Error ? e.message : 'Export failed'))} style={{ ...btnGhost, border: '1px solid #e2e8f0', padding: '6px 12px', borderRadius: 6 }}>Template / Export</button>
      <input ref={ref} type="file" accept=".csv,text/csv,.xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" onChange={doImport} style={{ display: 'none' }} />
      <button onClick={() => ref.current?.click()} disabled={busy || !tenantId} style={btnPrimary}>{busy ? 'Importing…' : 'Import units'}</button>
      {msg && <div style={{ flexBasis: '100%', fontSize: 12, color: msg.startsWith('✗') ? '#b91c1c' : '#166534' }}>{msg}</div>}
    </div>
  )
}

const btnPrimary: React.CSSProperties = { padding: '8px 16px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const btnSmall:   React.CSSProperties = { padding: '4px 10px', background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 4, cursor: 'pointer', fontSize: 12 }
const btnGhost:   React.CSSProperties = { background: 'none', border: 'none', cursor: 'pointer', color: 'var(--muted)', fontSize: 13 }
const labelStyle: React.CSSProperties = { fontSize: 13, fontWeight: 600, marginBottom: 4, color: '#475569' }
const selectStyle: React.CSSProperties = { width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, background: '#fff', boxSizing: 'border-box' }
const errorBox:   React.CSSProperties = { background: '#fef2f2', color: '#b91c1c', padding: '8px 12px', borderRadius: 6, marginBottom: 12, fontSize: 13 }
