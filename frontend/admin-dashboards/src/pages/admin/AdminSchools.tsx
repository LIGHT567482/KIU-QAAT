import { useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

// Manage the org hierarchy: schools/colleges, and the departments under each. Courses inherit
// from these (the course form picks a school → department).
//
// SUPPORT departments — Finance, Admissions, Bursary, Library, ICT, Estates… — are
// institution-wide and belong to NO school. They get their own section below, and are stored with
// school_id NULL (migration 066) rather than parked under a fictional "Support Services" school.

// ONE file carries both, because that is the shape an institution already holds its org chart in.
// The school column repeats down a college's block exactly as a spreadsheet renders a merged cell;
// a blank department creates the college alone, and a department with no school is a standalone
// SUPPORT unit. See org_io.go.
const ORG_COLS = ['school', 'abbreviation', 'department', 'kind']
const ORG_SAMPLE = [
  ORG_COLS.join(','),
  'School of Mathematics and Computing,SOMAC,Computer Science,ACADEMIC',
  'School of Mathematics and Computing,SOMAC,Information Technology,ACADEMIC',
  'College of Economics and Management,CEM,Accounting and Finance,ACADEMIC',
  ',,Finance,SUPPORT',
].join('\n') + '\n'

function downloadText(name: string, content: string) {
  const url = URL.createObjectURL(new Blob([content], { type: 'text/csv' }))
  const a = document.createElement('a'); a.href = url; a.download = name; a.click(); URL.revokeObjectURL(url)
}

interface School { school_id: string; name: string; abbreviation: string; dept_count: number }
interface Dept { department_id: string; school_id: string; name: string; kind: string }

export default function AdminSchools() {
  const { tenantId } = useParams<{ tenantId: string }>()
  const schoolsQ = useQuery<School[]>(() => api.get(`/api/v1/admin/tenants/${tenantId}/schools`), [tenantId])
  const deptsQ = useQuery<Dept[]>(() => api.get(`/api/v1/admin/tenants/${tenantId}/departments`), [tenantId])
  const schools = schoolsQ.data ?? []
  const depts = deptsQ.data ?? []

  const [newSchool, setNewSchool] = useState('')
  const [newAbbr, setNewAbbr] = useState('')
  // Schools being renamed, keyed by id — so an institution can add the full title to a school it
  // originally created under its abbreviation.
  const [edit, setEdit] = useState<Record<string, { name: string; abbreviation: string }>>({})
  const [deptName, setDeptName] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  // Standalone support departments (no school).
  const [supportName, setSupportName] = useState('')
  // Which schools are open. Collapsed by default: an institution with a dozen colleges was one
  // flat wall of chips, and the school a person came here to work on was somewhere in the middle
  // of it. The header still carries the department count, so nothing is hidden — only folded.
  const [open, setOpen] = useState<Record<string, boolean>>({})
  const toggle = (id: string) => setOpen(o => ({ ...o, [id]: !o[id] }))

  // school_id === '' is the marker for a department that belongs to no school.
  const academicDepts = depts.filter(d => d.school_id !== '')
  const supportDepts = depts.filter(d => d.school_id === '')

  function reload() { schoolsQ.refetch(); deptsQ.refetch() }

  // ── Bulk import of the whole chart ─────────────────────────────────────────
  const fileRef = useRef<HTMLInputElement>(null)
  const [importing, setImporting] = useState(false)
  const [importMsg, setImportMsg] = useState<string | null>(null)

  async function handleImport(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    setImporting(true); setImportMsg(null); setError(null)
    try {
      const fd = new FormData(); fd.append('roster', file)
      const res = await api.upload<{ inserted: number; updated: number; skipped: number; errors: string[] }>(
        `/api/v1/admin/tenants/${tenantId}/org/import`, fd)
      setImportMsg(
        `Imported: ${res.inserted} new, ${res.updated} updated, ${res.skipped} skipped` +
        (res.errors?.length ? ` · ${res.errors.length} problem(s): ${res.errors.slice(0, 3).join('; ')}` : ''))
      reload()
    } catch (err) {
      setImportMsg(err instanceof Error ? `Import failed: ${err.message}` : 'Import failed')
    } finally { setImporting(false); if (fileRef.current) fileRef.current.value = '' }
  }

  async function addSchool() {
    if (!newSchool.trim() || !newAbbr.trim()) return
    setBusy(true); setError(null)
    try {
      await api.post(`/api/v1/admin/tenants/${tenantId}/schools`,
        { name: newSchool.trim(), abbreviation: newAbbr.trim() })
      setNewSchool(''); setNewAbbr(''); reload()
    }
    catch (e) { setError(e instanceof Error ? e.message : 'Failed to add school') }
    finally { setBusy(false) }
  }

  // Renaming is how an institution that entered the SHORT FORM as the name (which everyone did
  // before there was a field for it) fills in the full title. Nothing filed under the old value is
  // orphaned: the backend keeps both forms as aliases when it scopes a dean's dashboard.
  async function saveSchool(id: string) {
    const e = edit[id]
    if (!e || !e.name.trim() || !e.abbreviation.trim()) return
    setBusy(true); setError(null)
    try {
      await api.patch(`/api/v1/admin/tenants/${tenantId}/schools/${id}`,
        { name: e.name.trim(), abbreviation: e.abbreviation.trim() })
      setEdit(p => { const n = { ...p }; delete n[id]; return n })
      reload()
    }
    catch (err) { setError(err instanceof Error ? err.message : 'Failed to save') }
    finally { setBusy(false) }
  }
  async function delSchool(id: string) {
    if (!confirm('Delete this school and its departments?')) return
    try { await api.delete(`/api/v1/admin/tenants/${tenantId}/schools/${id}`); reload() }
    catch (e) { setError(e instanceof Error ? e.message : 'Failed to delete') }
  }
  async function addDept(schoolId: string) {
    const name = (deptName[schoolId] ?? '').trim()
    if (!name) return
    setBusy(true); setError(null)
    try {
      await api.post(`/api/v1/admin/tenants/${tenantId}/departments`, { school_id: schoolId, name, kind: 'ACADEMIC' })
      setDeptName(m => ({ ...m, [schoolId]: '' })); reload()
    } catch (e) { setError(e instanceof Error ? e.message : 'Failed to add department') }
    finally { setBusy(false) }
  }

  // A support department is posted with no school_id at all.
  async function addSupportDept() {
    const name = supportName.trim()
    if (!name) return
    setBusy(true); setError(null)
    try {
      await api.post(`/api/v1/admin/tenants/${tenantId}/departments`, { name, kind: 'SUPPORT' })
      setSupportName(''); reload()
    } catch (e) { setError(e instanceof Error ? e.message : 'Failed to add department') }
    finally { setBusy(false) }
  }
  async function delDept(id: string) {
    if (!confirm('Delete this department?')) return
    try { await api.delete(`/api/v1/admin/tenants/${tenantId}/departments/${id}`); reload() }
    catch (e) { setError(e instanceof Error ? e.message : 'Failed to delete') }
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end', gap: 12, marginBottom: 20, flexWrap: 'wrap' }}>
        <div>
          <a href="/admin" style={{ color: 'var(--muted)', fontSize: 13, textDecoration: 'none' }}>← Admin</a>
          <h2 style={{ margin: '4px 0 0' }}>Schools &amp; Departments</h2>
          <p style={{ color: 'var(--muted)', margin: '4px 0 0', fontSize: 13 }}>
            Add schools/colleges and the departments under each. Courses inherit from these.
          </p>
        </div>
        {/* Typing a whole org chart one college at a time is the reason these pages sit empty.
            The template is filled in rather than a bare header row: the shape — a repeating
            school column, a blank department for a college alone, a blank school for a support
            unit — is the part that needs showing, and a header row alone cannot show it. */}
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <button onClick={() => downloadText('org_chart_template.csv', ORG_SAMPLE)} style={btnGhostSm}
            title="A CSV with the columns and a worked example of each case">Template</button>
          <input ref={fileRef} type="file" style={{ display: 'none' }} onChange={handleImport}
            accept=".csv,text/csv,.xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" />
          <button onClick={() => fileRef.current?.click()} disabled={importing} style={btnGhostSm}>
            {importing ? 'Importing…' : 'Import (CSV/Excel)'}
          </button>
        </div>
      </div>

      {error && <div style={errorBox}>{error}</div>}
      {importMsg && (
        <div style={{
          background: importMsg.startsWith('Import failed') ? '#fef2f2' : '#f0fdf4',
          color: importMsg.startsWith('Import failed') ? '#b91c1c' : '#166534',
          padding: '10px 14px', borderRadius: 8, marginBottom: 16, fontSize: 13,
        }}>{importMsg}</div>
      )}

      {/* A school has TWO names and both are needed: the full title for reports and letters, and
          the short form everyone actually says. The short form is not decoration — the backend
          accepts it as an alias when scoping a dean's dashboard, so historic rows that were filed
          under it keep resolving to this school. */}
      <div style={{ display: 'flex', gap: 8, marginBottom: 6, maxWidth: 760, flexWrap: 'wrap' }}>
        <input value={newSchool} onChange={e => setNewSchool(e.target.value)}
          placeholder="Full name — e.g. School of Mathematics and Computing"
          style={{ ...inputStyle, flex: 3, minWidth: 280 }}
          onKeyDown={e => { if (e.key === 'Enter') addSchool() }} />
        <input value={newAbbr} onChange={e => setNewAbbr(e.target.value.toUpperCase())}
          placeholder="Short form — e.g. SOMAC" maxLength={32}
          style={{ ...inputStyle, flex: 1, minWidth: 140, textTransform: 'uppercase' }}
          onKeyDown={e => { if (e.key === 'Enter') addSchool() }} />
        <button onClick={addSchool} disabled={busy || !newSchool.trim() || !newAbbr.trim()} style={btnPrimary}>+ Add school</button>
      </div>
      <p style={{ color: 'var(--muted)', fontSize: 12, margin: '0 0 24px' }}>
        Both are required. The short form is what appears in tables and reports, and it stays valid
        as a way of naming this school everywhere it has already been used.
      </p>

      {schoolsQ.status === 'loading' && <div style={{ color: 'var(--muted)' }}>Loading…</div>}
      {schoolsQ.status === 'ok' && schools.length === 0 && <div style={{ color: 'var(--muted)' }}>No schools yet — add one above.</div>}

      <div style={{ display: 'grid', gap: 16 }}>
        {schools.map(s => {
          const myDepts = academicDepts.filter(d => d.school_id === s.school_id)
          return (
            <div key={s.school_id} style={{ border: '1px solid #e2e8f0', borderRadius: 10, padding: 16 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                {edit[s.school_id] ? (
                  <div style={{ display: 'flex', gap: 8, flex: 1, flexWrap: 'wrap' }}>
                    <input value={edit[s.school_id].name}
                      onChange={e => setEdit(p => ({ ...p, [s.school_id]: { ...p[s.school_id], name: e.target.value } }))}
                      placeholder="Full name" style={{ ...inputStyle, flex: 3, minWidth: 240 }} />
                    <input value={edit[s.school_id].abbreviation}
                      onChange={e => setEdit(p => ({ ...p, [s.school_id]: { ...p[s.school_id], abbreviation: e.target.value.toUpperCase() } }))}
                      placeholder="Short form" maxLength={32} style={{ ...inputStyle, flex: 1, minWidth: 120 }} />
                    <button onClick={() => saveSchool(s.school_id)} disabled={busy} style={btnPrimary}>Save</button>
                    <button onClick={() => setEdit(p => { const n = { ...p }; delete n[s.school_id]; return n })} style={btnDanger}>Cancel</button>
                  </div>
                ) : (
                  // The whole header is the toggle — clicking a school is how you get at what is
                  // inside it, so the target is the school itself rather than a caret beside it.
                  <h3
                    onClick={() => toggle(s.school_id)}
                    role="button" tabIndex={0} aria-expanded={!!open[s.school_id]}
                    onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); toggle(s.school_id) } }}
                    style={{ margin: 0, cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 8, flex: 1, userSelect: 'none' }}
                  >
                    <span style={{
                      display: 'inline-block', transition: 'transform .15s', color: 'var(--muted)', fontSize: 12,
                      transform: open[s.school_id] ? 'rotate(90deg)' : 'none',
                    }}>▶</span>
                    {s.abbreviation && (
                      <span style={{
                        background: 'var(--brand)', color: '#fff', borderRadius: 6,
                        padding: '2px 8px', fontSize: 13, letterSpacing: '.03em',
                      }}>{s.abbreviation}</span>
                    )}
                    {s.name}
                    <span style={{ color: 'var(--muted)', fontWeight: 400, fontSize: 13 }}>· {myDepts.length} dept(s)</span>
                    {!s.abbreviation && (
                      <span style={{ color: '#b45309', fontWeight: 400, fontSize: 12 }}>
                        no short form set
                      </span>
                    )}
                  </h3>
                )}
                {!edit[s.school_id] && (
                  <span style={{ display: 'flex', gap: 8 }}>
                    <button onClick={() => setEdit(p => ({ ...p, [s.school_id]: { name: s.name, abbreviation: s.abbreviation } }))}
                      style={btnGhostSm}>Rename</button>
                    <button onClick={() => delSchool(s.school_id)} style={btnDanger}>Delete</button>
                  </span>
                )}
              </div>

              {/* The contents of a school appear only once it is opened. */}
              {open[s.school_id] && (
                <>
                  <div style={{ marginTop: 12, display: 'flex', flexWrap: 'wrap', gap: 8 }}>
                    {myDepts.map(d => (
                      <span key={d.department_id} style={chip}>
                        {d.name}{d.kind === 'SUPPORT' ? ' (support)' : ''}
                        <button onClick={() => delDept(d.department_id)} style={chipX} title="Remove">×</button>
                      </span>
                    ))}
                    {myDepts.length === 0 && <span style={{ color: 'var(--muted)', fontSize: 13 }}>No departments yet.</span>}
                  </div>

                  <div style={{ marginTop: 12, display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                    <input value={deptName[s.school_id] ?? ''} onChange={e => setDeptName(m => ({ ...m, [s.school_id]: e.target.value }))}
                      placeholder="New department name" style={{ ...inputStyle, maxWidth: 260 }}
                      onKeyDown={e => { if (e.key === 'Enter') addDept(s.school_id) }} />
                    <button onClick={() => addDept(s.school_id)} disabled={busy} style={btnPrimary}>+ Add department</button>
                  </div>
                </>
              )}
            </div>
          )
        })}
      </div>

      {/* ── Support departments: institution-wide, under no school ─────────── */}
      <div style={{ marginTop: 28, border: '1px solid #e2e8f0', borderRadius: 10, padding: 16, background: '#fcfdff' }}>
        <h3 style={{ margin: '0 0 2px' }}>
          Support departments
          <span style={{ color: 'var(--muted)', fontWeight: 400, fontSize: 13 }}> · {supportDepts.length}</span>
        </h3>
        <p style={{ color: 'var(--muted)', margin: '0 0 12px', fontSize: 13 }}>
          Finance, Admissions, Bursary, Library, ICT, Estates, Security and the like. These serve the
          whole institution and sit under <strong>no school</strong> — they are not part of any faculty.
          Their staff are managed under <a href="../employees" style={{ color: 'var(--brand)' }}>Employees</a>.
        </p>

        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginBottom: 12 }}>
          {supportDepts.map(d => (
            <span key={d.department_id} style={{ ...chip, background: '#ecfeff', color: '#155e75' }}>
              {d.name}
              <button onClick={() => delDept(d.department_id)} style={chipX} title="Remove">×</button>
            </span>
          ))}
          {supportDepts.length === 0 && (
            <span style={{ color: 'var(--muted)', fontSize: 13 }}>None yet — add one below.</span>
          )}
        </div>

        <div style={{ display: 'flex', gap: 8, maxWidth: 520 }}>
          <input value={supportName} onChange={e => setSupportName(e.target.value)}
            placeholder="e.g. Finance, Admissions, Library, ICT" style={inputStyle}
            onKeyDown={e => { if (e.key === 'Enter') addSupportDept() }} />
          <button onClick={addSupportDept} disabled={busy || !supportName.trim()} style={btnPrimary}>
            + Add support department
          </button>
        </div>
      </div>
    </div>
  )
}

const inputStyle: React.CSSProperties = { flex: 1, padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }
const btnPrimary: React.CSSProperties = { padding: '8px 16px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const btnGhostSm: React.CSSProperties = { padding: '6px 12px', background: '#fff', color: '#1e293b', border: '1px solid #cbd5e1', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 12 }
const btnDanger: React.CSSProperties = { padding: '6px 12px', background: '#fff', color: '#b91c1c', border: '1px solid #fecaca', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 12 }
const errorBox: React.CSSProperties = { background: '#fef2f2', color: '#b91c1c', padding: '8px 12px', borderRadius: 6, marginBottom: 12, fontSize: 13 }
const chip: React.CSSProperties = { display: 'inline-flex', alignItems: 'center', gap: 6, background: '#f1f5f9', borderRadius: 999, padding: '4px 10px', fontSize: 13 }
const chipX: React.CSSProperties = { border: 'none', background: 'transparent', cursor: 'pointer', color: '#64748b', fontSize: 16, lineHeight: 1, padding: 0 }
