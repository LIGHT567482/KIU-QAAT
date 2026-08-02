import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

// Manage the org hierarchy: schools/colleges, and the departments under each. Courses inherit
// from these (the course form picks a school → department).
//
// SUPPORT departments — Finance, Admissions, Bursary, Library, ICT, Estates… — are
// institution-wide and belong to NO school. They get their own section below, and are stored with
// school_id NULL (migration 066) rather than parked under a fictional "Support Services" school.

interface School { school_id: string; name: string; dept_count: number }
interface Dept { department_id: string; school_id: string; name: string; kind: string }

export default function AdminSchools() {
  const { tenantId } = useParams<{ tenantId: string }>()
  const schoolsQ = useQuery<School[]>(() => api.get(`/api/v1/admin/tenants/${tenantId}/schools`), [tenantId])
  const deptsQ = useQuery<Dept[]>(() => api.get(`/api/v1/admin/tenants/${tenantId}/departments`), [tenantId])
  const schools = schoolsQ.data ?? []
  const depts = deptsQ.data ?? []

  const [newSchool, setNewSchool] = useState('')
  const [deptName, setDeptName] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  // Standalone support departments (no school).
  const [supportName, setSupportName] = useState('')

  // school_id === '' is the marker for a department that belongs to no school.
  const academicDepts = depts.filter(d => d.school_id !== '')
  const supportDepts = depts.filter(d => d.school_id === '')

  function reload() { schoolsQ.refetch(); deptsQ.refetch() }

  async function addSchool() {
    if (!newSchool.trim()) return
    setBusy(true); setError(null)
    try { await api.post(`/api/v1/admin/tenants/${tenantId}/schools`, { name: newSchool.trim() }); setNewSchool(''); reload() }
    catch (e) { setError(e instanceof Error ? e.message : 'Failed to add school') }
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
      <div style={{ marginBottom: 20 }}>
        <a href="/admin" style={{ color: 'var(--muted)', fontSize: 13, textDecoration: 'none' }}>← Admin</a>
        <h2 style={{ margin: '4px 0 0' }}>Schools &amp; Departments</h2>
        <p style={{ color: 'var(--muted)', margin: '4px 0 0', fontSize: 13 }}>
          Add schools/colleges and the departments under each. Courses inherit from these.
        </p>
      </div>

      {error && <div style={errorBox}>{error}</div>}

      <div style={{ display: 'flex', gap: 8, marginBottom: 24, maxWidth: 520 }}>
        <input value={newSchool} onChange={e => setNewSchool(e.target.value)} placeholder="New school / college name"
          style={inputStyle} onKeyDown={e => { if (e.key === 'Enter') addSchool() }} />
        <button onClick={addSchool} disabled={busy || !newSchool.trim()} style={btnPrimary}>+ Add school</button>
      </div>

      {schoolsQ.status === 'loading' && <div style={{ color: 'var(--muted)' }}>Loading…</div>}
      {schoolsQ.status === 'ok' && schools.length === 0 && <div style={{ color: 'var(--muted)' }}>No schools yet — add one above.</div>}

      <div style={{ display: 'grid', gap: 16 }}>
        {schools.map(s => {
          const myDepts = academicDepts.filter(d => d.school_id === s.school_id)
          return (
            <div key={s.school_id} style={{ border: '1px solid #e2e8f0', borderRadius: 10, padding: 16 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <h3 style={{ margin: 0 }}>{s.name} <span style={{ color: 'var(--muted)', fontWeight: 400, fontSize: 13 }}>· {myDepts.length} dept(s)</span></h3>
                <button onClick={() => delSchool(s.school_id)} style={btnDanger}>Delete</button>
              </div>

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
const btnDanger: React.CSSProperties = { padding: '6px 12px', background: '#fff', color: '#b91c1c', border: '1px solid #fecaca', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 12 }
const errorBox: React.CSSProperties = { background: '#fef2f2', color: '#b91c1c', padding: '8px 12px', borderRadius: 6, marginBottom: 12, fontSize: 13 }
const chip: React.CSSProperties = { display: 'inline-flex', alignItems: 'center', gap: 6, background: '#f1f5f9', borderRadius: 999, padding: '4px 10px', fontSize: 13 }
const chipX: React.CSSProperties = { border: 'none', background: 'transparent', cursor: 'pointer', color: '#64748b', fontSize: 16, lineHeight: 1, padding: 0 }
