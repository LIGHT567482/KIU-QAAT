import { useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../../lib/api'
import { OrgPicker, useOrg } from '../../components/OrgPicker'
import { useQuery } from '../../lib/useApi'

// General (non-teaching) staff whose attendance is tracked by the check-in tablet.
// Separate registry from lecturers, with its own bulk import/export template.
const EMP_COLS = ['staff_id', 'title', 'full_name', 'phone', 'email']

function downloadText(name: string, content: string) {
  const url = URL.createObjectURL(new Blob([content], { type: 'text/csv' }))
  const a = document.createElement('a'); a.href = url; a.download = name; a.click(); URL.revokeObjectURL(url)
}

interface Employee {
  employee_pk: string
  staff_id: string
  title: string
  full_name: string
  department: string
  job_title: string
  email: string
  phone: string
  is_active: boolean
}

const BLANK = { staff_id: '', title: '', full_name: '', department: '', job_title: '', email: '', phone: '' }

export default function AdminEmployees() {
  const { tenantId } = useParams<{ tenantId: string }>()
  const { status, data, refetch } = useQuery<Employee[]>(() => api.get(`/api/v1/admin/tenants/${tenantId}/employees`))
  const [creating, setCreating] = useState(false)
  const org = useOrg(tenantId ?? '')
  const [form, setForm] = useState({ ...BLANK })
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [editId, setEditId] = useState<string | null>(null)
  const [editForm, setEditForm] = useState({ ...BLANK })

  const fileRef = useRef<HTMLInputElement>(null)
  const [importing, setImporting] = useState(false)
  const [importMsg, setImportMsg] = useState<string | null>(null)
  const [search, setSearch] = useState('')

  const q = search.trim().toLowerCase()
  const list = (data ?? []).filter(e => !q ||
    [e.staff_id, e.full_name, e.department, e.job_title].some(v => (v || '').toLowerCase().includes(q)))

  async function handleCreate() {
    setSaving(true); setError(null)
    try {
      await api.post(`/api/v1/admin/tenants/${tenantId}/employees`, form)
      setCreating(false); setForm({ ...BLANK }); refetch()
    } catch (e) { setError(e instanceof Error ? e.message : 'Failed') }
    finally { setSaving(false) }
  }

  function startEdit(e: Employee) {
    setEditId(e.employee_pk)
    setEditForm({ staff_id: e.staff_id, title: e.title || '', full_name: e.full_name, department: e.department || '', job_title: e.job_title || '', email: e.email || '', phone: e.phone || '' })
  }
  async function saveEdit() {
    if (!editId) return
    try { await api.patch(`/api/v1/admin/employees/${editId}`, editForm); setEditId(null); refetch() }
    catch (e) { alert(e instanceof Error ? e.message : 'Failed') }
  }
  async function del(e: Employee) {
    if (!confirm(`Delete employee "${e.full_name}" (${e.staff_id})? Their tablet punches stay in the log.`)) return
    try { await api.delete(`/api/v1/admin/employees/${e.employee_pk}`); refetch() }
    catch (err) { alert(err instanceof Error ? err.message : 'Failed') }
  }

  async function handleImport(ev: React.ChangeEvent<HTMLInputElement>) {
    const file = ev.target.files?.[0]
    if (!file) return
    setImporting(true); setImportMsg(null)
    try {
      const fd = new FormData(); fd.append('roster', file)
      const res = await api.upload<{ inserted: number; updated: number; skipped: number; errors: string[] }>(
        `/api/v1/admin/tenants/${tenantId}/employees/import`, fd)
      setImportMsg(`Imported: ${res.inserted} new, ${res.updated} updated, ${res.skipped} skipped${res.errors?.length ? ` · ${res.errors.length} error(s): ${res.errors.slice(0, 3).join('; ')}` : ''}`)
      refetch()
    } catch (e) { setImportMsg(e instanceof Error ? `Import failed: ${e.message}` : 'Import failed') }
    finally { setImporting(false); if (fileRef.current) fileRef.current.value = '' }
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
        <div>
          <h2 style={{ margin: 0 }}>Employees</h2>
          <p style={{ color: 'var(--muted)', margin: '4px 0 0', fontSize: 13 }}>
            General staff tracked by the physical check-in tablet. Register them here (separate from lecturers), then import the tablet's punches under <strong>Employee Attendance</strong>.
          </p>
        </div>
        <button onClick={() => { setCreating(c => !c); setForm({ ...BLANK }); setError(null) }} style={btnPrimary}>
          {creating ? 'Cancel' : '+ Add employee'}
        </button>
      </div>

      <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap', margin: '10px 0 14px' }}>
        <button onClick={() => fileRef.current?.click()} disabled={importing} style={btnSmall}>{importing ? 'Importing…' : 'Import CSV/Excel'}</button>
        <input ref={fileRef} type="file" accept=".csv,.xlsx" onChange={handleImport} style={{ display: 'none' }} />
        <button onClick={() => downloadText('employees_template.csv', EMP_COLS.join(',') + '\n')} style={btnSmall} title="Blank template with the employee columns">Template</button>
        <button onClick={() => api.download(`/api/v1/admin/tenants/${tenantId}/employees/export.xlsx`, 'employees.xlsx').catch(e => alert(e instanceof Error ? e.message : 'Export failed'))} style={btnSmall}>Export Excel</button>
        <input value={search} onChange={e => setSearch(e.target.value)} placeholder="Search by name, ID, department…" style={{ ...inp, flex: 1, minWidth: 200 }} />
      </div>
      {importMsg && <div style={{ background: importMsg.startsWith('Import failed') ? '#fef2f2' : '#f0fdf4', color: importMsg.startsWith('Import failed') ? '#b91c1c' : '#166534', padding: '10px 14px', borderRadius: 8, marginBottom: 14, fontSize: 13 }}>{importMsg}</div>}

      {creating && (
        <div style={card}>
          {error && <div style={errBox}>{error}</div>}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 12 }}>
            <Input label="Staff / Badge ID *" value={form.staff_id} onChange={v => setForm(f => ({ ...f, staff_id: v }))} />
            <Input label="Title" value={form.title} onChange={v => setForm(f => ({ ...f, title: v }))} placeholder="Mr / Ms / Dr" />
            <Input label="Full name *" value={form.full_name} onChange={v => setForm(f => ({ ...f, full_name: v }))} />
            <Input label="Phone (contact)" value={form.phone} onChange={v => setForm(f => ({ ...f, phone: v }))} />
            <Input label="Email (contact)" value={form.email} onChange={v => setForm(f => ({ ...f, email: v }))} />
            <Input label="Job title" value={form.job_title} onChange={v => setForm(f => ({ ...f, job_title: v }))} placeholder="Bursar / Systems Officer" />
            {/* Department was in the payload but had no field, so every employee was saved with a
                blank one and the no-show report could not say whose office to chase. Non-teaching
                staff sit in SUPPORT departments — Finance, ICT, Library — which belong to no
                faculty, which is exactly the case the picker handles by clearing the school. */}
            <OrgPicker
              schools={org.schools} departments={org.departments}
              department={form.department} school=""
              onChange={next => setForm(f => ({ ...f, department: next.department }))}
              showSchool={false}
            />
          </div>
          <button onClick={handleCreate} disabled={saving || !form.staff_id.trim() || !form.full_name.trim()} style={{ ...btnPrimary, marginTop: 12 }}>
            {saving ? 'Saving…' : 'Save employee'}
          </button>
        </div>
      )}

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}
      {status === 'ok' && (
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
            <thead><tr style={{ background: 'var(--surface-2)' }}>
              <th style={th}>Staff ID</th><th style={th}>Name</th><th style={th}>Contact</th><th style={th}></th>
            </tr></thead>
            <tbody>
              {list.map(e => editId === e.employee_pk ? (
                <tr key={e.employee_pk}>
                  <td style={td}><input value={editForm.staff_id} onChange={ev => setEditForm(f => ({ ...f, staff_id: ev.target.value }))} style={inp} /></td>
                  <td style={td}><input value={editForm.full_name} onChange={ev => setEditForm(f => ({ ...f, full_name: ev.target.value }))} style={inp} /></td>
                  <td style={td}><input value={editForm.phone} onChange={ev => setEditForm(f => ({ ...f, phone: ev.target.value }))} style={inp} placeholder="phone / email" /></td>
                  <td style={{ ...td, whiteSpace: 'nowrap' }}>
                    <button onClick={saveEdit} style={btnSmall}>Save</button>{' '}
                    <button onClick={() => setEditId(null)} style={btnSmall}>Cancel</button>
                  </td>
                </tr>
              ) : (
                <tr key={e.employee_pk} style={{ borderTop: '1px solid var(--border)' }}>
                  <td style={{ ...td, fontFamily: 'monospace' }}>{e.staff_id}</td>
                  <td style={td}>{e.title ? `${e.title} ` : ''}{e.full_name}</td>
                  <td style={{ ...td, color: 'var(--muted)' }}>{e.phone || e.email || '—'}</td>
                  <td style={{ ...td, whiteSpace: 'nowrap' }}>
                    <button onClick={() => startEdit(e)} style={btnSmall}>Edit</button>{' '}
                    <button onClick={() => del(e)} style={{ ...btnSmall, color: '#b91c1c', borderColor: '#fecaca' }}>Delete</button>
                  </td>
                </tr>
              ))}
              {list.length === 0 && <tr><td colSpan={4} style={{ ...td, textAlign: 'center', color: 'var(--muted)', padding: 28 }}>No employees yet — add one or import a list.</td></tr>}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function Input({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (v: string) => void; placeholder?: string }) {
  return (
    <label style={{ display: 'block' }}>
      <div style={{ fontSize: 12, fontWeight: 600, marginBottom: 4, color: 'var(--muted)' }}>{label}</div>
      <input value={value} placeholder={placeholder} onChange={e => onChange(e.target.value)} style={inp} />
    </label>
  )
}

const card: React.CSSProperties = { background: 'var(--surface)', border: '1px solid var(--border)', borderRadius: 10, padding: 18, marginBottom: 18 }
const th: React.CSSProperties = { padding: '8px 12px', textAlign: 'left', color: 'var(--muted)', fontSize: 12, whiteSpace: 'nowrap' }
const td: React.CSSProperties = { padding: '9px 12px' }
const inp: React.CSSProperties = { width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid var(--border)', fontSize: 14, boxSizing: 'border-box', background: 'var(--surface)', color: 'var(--text)' }
const btnPrimary: React.CSSProperties = { padding: '8px 16px', background: 'var(--brand)', color: 'var(--brand-contrast)', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const btnSmall: React.CSSProperties = { padding: '5px 10px', background: 'var(--surface-2)', border: '1px solid var(--border)', borderRadius: 6, cursor: 'pointer', fontSize: 12, color: 'var(--text)' }
const errBox: React.CSSProperties = { background: '#fee2e2', color: '#b91c1c', padding: '8px 12px', borderRadius: 6, marginBottom: 12, fontSize: 13 }
