import { useRef, useState, useEffect } from 'react'
import { useParams } from 'react-router-dom'
import QRCode from 'qrcode'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

// email is OPTIONAL — used only to email the lecturer their permanent career QR.
const LECT_COLS = ['staff_id', 'full_name', 'email', 'phone', 'department', 'title', 'gender']
function downloadText(name: string, content: string) {
  const url = URL.createObjectURL(new Blob([content], { type: 'text/csv' }))
  const a = document.createElement('a'); a.href = url; a.download = name; a.click(); URL.revokeObjectURL(url)
}

interface Lecturer {
  lecturer_id: string
  title: string
  full_name: string
  gender: string
  email: string
  phone: string
  department: string
  staff_id: string
}

const GENDERS = ['', 'Male', 'Female', 'Other']

export default function AdminLecturers() {
  const { tenantId } = useParams<{ tenantId: string }>()
  const { status, data, refetch } = useQuery<Lecturer[]>(
    () => api.get(`/api/v1/admin/tenants/${tenantId}/lecturers`)
  )
  const [creating, setCreating] = useState(false)
  const [form, setForm] = useState({ full_name: '', email: '', phone: '', department: '', staff_id: '', title: '', gender: '' })
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [editId, setEditId] = useState<string | null>(null)
  const [editForm, setEditForm] = useState({ full_name: '', phone: '', department: '', staff_id: '', title: '', gender: '' })

  // Biometric enrolment: issue a one-time link the lecturer opens on their phone.
  const [enroll, setEnroll] = useState<{ name: string; url: string } | null>(null)
  async function makeEnroll(l: Lecturer) {
    try {
      const res = await api.post<{ enroll_url: string }>(`/api/v1/admin/tenants/${tenantId}/lecturers/${l.lecturer_id}/enroll-link`, {})
      setEnroll({ name: l.full_name, url: res.enroll_url })
    } catch (e) { alert(e instanceof Error ? e.message : 'Could not create enrolment link') }
  }

  // The lecturer's personal QR — scanning it signs them into their dashboard.
  const [qr, setQr] = useState<{ name: string; url: string; staff_id: string } | null>(null)
  const [qrImg, setQrImg] = useState('')
  async function showQR(l: Lecturer) {
    try {
      const res = await api.get<{ full_name: string; url: string; staff_id: string }>(`/api/v1/admin/tenants/${tenantId}/lecturers/${l.lecturer_id}/qr`)
      setQr({ name: res.full_name, url: res.url, staff_id: res.staff_id })
    } catch (e) { alert(e instanceof Error ? e.message : 'Could not load QR') }
  }
  useEffect(() => {
    if (!qr) { setQrImg(''); return }
    QRCode.toDataURL(qr.url, { width: 320, margin: 2, errorCorrectionLevel: 'H' }).then(setQrImg).catch(() => setQrImg(''))
  }, [qr])

  const titlesQ = useQuery<{ titles: string[] }>(() => api.get('/api/v1/admin/settings/titles'))
  const titles = (titlesQ.status === 'ok' ? titlesQ.data?.titles : null) ?? []

  // Tenant short name used to auto-generate staff IDs (<PREFIX>/STAFF/00001).
  const prefixQ = useQuery<{ staff_id_prefix: string }>(() => api.get('/api/v1/admin/settings/staff-id-prefix'))
  const prefix = prefixQ.status === 'ok' ? (prefixQ.data?.staff_id_prefix ?? '') : ''
  async function editPrefix() {
    const v = window.prompt('Short institution code for auto-generated staff IDs (e.g. KIU → KIU/STAFF/00001):', prefix)
    if (v === null) return
    try { await api.put('/api/v1/admin/settings/staff-id-prefix', { staff_id_prefix: v }); prefixQ.refetch() }
    catch (e) { alert(e instanceof Error ? e.message : 'Failed') }
  }

  async function handleCreate() {
    setSaving(true); setError(null)
    try {
      await api.post(`/api/v1/admin/tenants/${tenantId}/lecturers`, form)
      setCreating(false)
      setForm({ full_name: '', email: '', phone: '', department: '', staff_id: '', title: '', gender: '' })
      refetch()
    } catch (e) { setError(e instanceof Error ? e.message : 'Failed') }
    finally { setSaving(false) }
  }

  function startEdit(l: Lecturer) {
    setEditId(l.lecturer_id)
    setEditForm({ full_name: l.full_name, phone: l.phone || '', department: l.department || '', staff_id: l.staff_id || '', title: l.title || '', gender: l.gender || '' })
  }
  async function handleEditSave() {
    if (!editId) return
    try { await api.patch(`/api/v1/admin/lecturers/${editId}`, editForm); setEditId(null); refetch() }
    catch (e) { alert(e instanceof Error ? e.message : 'Failed') }
  }

  // ── Bulk import / export + department filter ───────────────────────────────
  const fileRef = useRef<HTMLInputElement>(null)
  const [importing, setImporting] = useState(false)
  const [importMsg, setImportMsg] = useState<string | null>(null)
  const [dept, setDept] = useState('')
  const departments = Array.from(new Set((data ?? []).map(l => l.department).filter(Boolean))).sort()

  async function handleImport(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    setImporting(true); setImportMsg(null)
    try {
      const fd = new FormData(); fd.append('roster', file)
      const res = await api.upload<{ inserted: number; updated: number; skipped: number; errors: string[] }>(
        `/api/v1/admin/tenants/${tenantId}/lecturers/import`, fd)
      setImportMsg(`Imported: ${res.inserted} new, ${res.updated} updated, ${res.skipped} skipped${res.errors?.length ? ` · ${res.errors.length} error(s): ${res.errors.slice(0, 3).join('; ')}` : ''}`)
      refetch()
    } catch (e) { setImportMsg(e instanceof Error ? `Import failed: ${e.message}` : 'Import failed') }
    finally { setImporting(false); if (fileRef.current) fileRef.current.value = '' }
  }

  async function createLogin(l: Lecturer) {
    if (!confirm(`Create a dashboard login for ${l.full_name}? They'll sign in with their staff ID (${l.staff_id || 'set one first'}).`)) return
    try {
      const res = await api.post<{ staff_id: string; password?: string }>(`/api/v1/admin/tenants/${tenantId}/lecturers/${l.lecturer_id}/create-login`, {})
      alert(`Login created.\n\nStaff ID: ${res.staff_id}${res.password ? `\nPassword: ${res.password}\n\nGive these to the lecturer now — the password won't be shown again.` : ''}`)
      refetch()
    } catch (e) { alert(e instanceof Error ? e.message : 'Could not create login') }
  }

  const [search, setSearch] = useState('')
  const q = search.trim().toLowerCase()
  const lecturers = (status === 'ok' ? (data ?? []) : []).filter(l =>
    (!dept || l.department === dept) &&
    (!q || [l.full_name, l.staff_id, l.department, l.phone, l.title].some(v => (v || '').toLowerCase().includes(q))))

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <div>
          <a href="/admin/tenants" style={{ fontSize: 13, color: 'var(--muted)', textDecoration: 'none' }}>← Tenants</a>
          <h2 style={{ margin: '4px 0 0' }}>Lecturers</h2>
          <div style={{ fontSize: 13, color: 'var(--muted)' }}>Tenant: {tenantId}</div>
        </div>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <button onClick={editPrefix} style={btnSmall} title="Format for auto-generated staff IDs">
            Auto-ID prefix: {prefix || '(set)'}
          </button>
          <a href={`/admin/tenants/${tenantId}/lecturer-assignments`} style={{ ...btnSmall, textDecoration: 'none', display: 'inline-block' }}>
            View Assignments
          </a>
          <button onClick={() => downloadText('lecturers_template.csv', LECT_COLS.join(',') + '\n')} style={btnSmall} title="Download a blank CSV with the lecturer columns">Template</button>
          <button onClick={() => api.download(`/api/v1/admin/tenants/${tenantId}/lecturers/export.xlsx${dept ? `?department=${encodeURIComponent(dept)}` : ''}`, 'lecturers.xlsx').catch(e => alert(e instanceof Error ? e.message : 'Export failed'))} style={btnSmall} title="Exports the filtered lecturers">Export Excel</button>
          <input ref={fileRef} type="file" accept=".csv,text/csv,.xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" onChange={handleImport} style={{ display: 'none' }} />
          <button onClick={() => fileRef.current?.click()} disabled={importing} style={btnSmall}>{importing ? 'Importing…' : 'Import (CSV/Excel)'}</button>
          <button onClick={() => setCreating(c => !c)} style={btnPrimary}>
            {creating ? 'Cancel' : '+ New Lecturer'}
          </button>
        </div>
      </div>

      {creating && (
        <div style={{ background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 10, padding: 20, marginBottom: 24 }}>
          <h3 style={{ margin: '0 0 16px' }}>Register Lecturer</h3>
          {error && <div style={errorBox}>{error}</div>}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <label style={{ display: 'block' }}>
              <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 4, color: '#475569' }}>Title</div>
              <select value={form.title} onChange={e => setForm(f => ({ ...f, title: e.target.value }))} style={selectStyle}>
                <option value="">— none —</option>
                {titles.map(t => <option key={t} value={t}>{t}</option>)}
              </select>
            </label>
            <label style={{ display: 'block' }}>
              <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 4, color: '#475569' }}>Gender</div>
              <select value={form.gender} onChange={e => setForm(f => ({ ...f, gender: e.target.value }))} style={selectStyle}>
                {GENDERS.map(g => <option key={g} value={g}>{g || '—'}</option>)}
              </select>
            </label>
            <Input label="Full name *" value={form.full_name} onChange={v => setForm(f => ({ ...f, full_name: v }))} />
            <Input label="Email (optional — emails them their QR)" value={form.email} onChange={v => setForm(f => ({ ...f, email: v }))} placeholder="lecturer@university.edu — leave blank to skip" />
            <Input label="Phone" value={form.phone} onChange={v => setForm(f => ({ ...f, phone: v }))} placeholder="+256 700 000000" />
            <Input label="Department" value={form.department} onChange={v => setForm(f => ({ ...f, department: v }))} placeholder="Computer Science" />
            <Input label="Staff ID (optional — auto-generated if left blank)" value={form.staff_id} onChange={v => setForm(f => ({ ...f, staff_id: v }))} placeholder="leave blank to auto-generate e.g. KIU/STAFF/00001" />
          </div>
          <button onClick={handleCreate} disabled={saving || !form.full_name} style={{ ...btnPrimary, marginTop: 16, opacity: !form.full_name ? 0.5 : 1 }}>
            {saving ? 'Saving…' : 'Register Lecturer'}
          </button>
        </div>
      )}

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}

      {importMsg && (
        <div style={{ background: importMsg.startsWith('Import failed') ? '#fef2f2' : '#f0fdf4', color: importMsg.startsWith('Import failed') ? '#b91c1c' : '#166534', padding: '10px 14px', borderRadius: 8, marginBottom: 16, fontSize: 13 }}>{importMsg}</div>
      )}

      <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 12, flexWrap: 'wrap' }}>
        <input value={search} onChange={e => setSearch(e.target.value)} placeholder="Search by name, staff ID, department or phone…"
          style={{ flex: 1, minWidth: 260, maxWidth: 420, padding: '8px 12px', borderRadius: 8, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }} />
        <select value={dept} onChange={e => setDept(e.target.value)} style={{ ...selectStyle, width: 'auto', minWidth: 180 }}>
          <option value="">All departments</option>
          {departments.map(d => <option key={d} value={d}>{d}</option>)}
        </select>
      </div>

      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
        <thead>
          <tr style={{ background: '#f8fafc' }}>
            {['Title', 'Name', 'Gender', 'Staff ID', 'Phone', 'Department', ''].map(h => (
              <th key={h} style={{ padding: '8px 12px', textAlign: 'left', borderBottom: '1px solid #e2e8f0' }}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {(lecturers ?? []).map(l => editId === l.lecturer_id ? (
            <tr key={l.lecturer_id} style={{ background: '#fefce8' }}>
              <td colSpan={8} style={{ padding: 12 }}>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 10 }}>
                  <label><div style={{ fontSize: 13, fontWeight: 600, marginBottom: 4, color: '#475569' }}>Title</div>
                    <select value={editForm.title} onChange={e => setEditForm(f => ({ ...f, title: e.target.value }))} style={selectStyle}>
                      <option value="">— none —</option>{titles.map(t => <option key={t} value={t}>{t}</option>)}
                    </select></label>
                  <Input label="Full name" value={editForm.full_name} onChange={v => setEditForm(f => ({ ...f, full_name: v }))} />
                  <label><div style={{ fontSize: 13, fontWeight: 600, marginBottom: 4, color: '#475569' }}>Gender</div>
                    <select value={editForm.gender} onChange={e => setEditForm(f => ({ ...f, gender: e.target.value }))} style={selectStyle}>
                      {GENDERS.map(g => <option key={g} value={g}>{g || '—'}</option>)}
                    </select></label>
                  <Input label="Staff ID" value={editForm.staff_id} onChange={v => setEditForm(f => ({ ...f, staff_id: v }))} />
                  <Input label="Phone" value={editForm.phone} onChange={v => setEditForm(f => ({ ...f, phone: v }))} />
                  <Input label="Department" value={editForm.department} onChange={v => setEditForm(f => ({ ...f, department: v }))} />
                </div>
                <div style={{ marginTop: 10, display: 'flex', gap: 8 }}>
                  <button onClick={handleEditSave} style={{ ...btnPrimary, background: '#92400e' }}>Save</button>
                  <button onClick={() => setEditId(null)} style={btnSmall}>Cancel</button>
                </div>
              </td>
            </tr>
          ) : (
            <tr key={l.lecturer_id} style={{ borderBottom: '1px solid #f1f5f9' }}>
              <td style={{ padding: '10px 12px' }}>{l.title || '—'}</td>
              <td style={{ padding: '10px 12px', fontWeight: 600 }}>{l.full_name}</td>
              <td style={{ padding: '10px 12px', color: 'var(--muted)' }}>{l.gender || '—'}</td>
              <td style={{ padding: '10px 12px', fontFamily: 'monospace', fontSize: 12 }}>{l.staff_id || '—'}</td>
              <td style={{ padding: '10px 12px', color: 'var(--muted)' }}>{l.phone || '—'}</td>
              <td style={{ padding: '10px 12px', color: 'var(--muted)' }}>{l.department || '—'}</td>
              <td style={{ padding: '10px 12px', whiteSpace: 'nowrap' }}>
                <button onClick={() => startEdit(l)} style={btnSmall}>Edit</button>
                <button onClick={() => makeEnroll(l)} style={{ ...btnSmall, marginLeft: 6, background: '#eef2ff', borderColor: '#c7d2fe', color: '#3730a3' }}>Enroll FP</button>
                <button onClick={() => createLogin(l)} style={{ ...btnSmall, marginLeft: 6, background: '#ecfdf5', borderColor: '#a7f3d0', color: '#065f46' }} title="Create a dashboard login for this lecturer">Login</button>
                <button onClick={() => showQR(l)} style={{ ...btnSmall, marginLeft: 6, background: '#fef9c3', borderColor: '#fde68a', color: '#854d0e' }} title="Show this lecturer's QR — scan to open their dashboard">QR</button>
              </td>
            </tr>
          ))}
          {(lecturers ?? []).length === 0 && status === 'ok' && (
            <tr>
              <td colSpan={8} style={{ padding: 24, textAlign: 'center', color: 'var(--muted)' }}>
                No lecturers registered yet. Click "+ New Lecturer" to add one.
              </td>
            </tr>
          )}
        </tbody>
      </table>

      {qr && (
        <div onClick={() => setQr(null)} style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.6)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 9999, padding: 20 }}>
          <div onClick={e => e.stopPropagation()} style={{ background: '#fff', borderRadius: 14, padding: 24, maxWidth: 380, width: '100%', textAlign: 'center' }}>
            <h3 style={{ margin: '0 0 2px' }}>{qr.name}</h3>
            {qr.staff_id && <div style={{ fontSize: 12, color: 'var(--muted)', fontFamily: 'monospace', marginBottom: 12 }}>{qr.staff_id}</div>}
            {qrImg
              ? <img src={qrImg} alt="Lecturer QR" style={{ width: 280, height: 280 }} />
              : <p style={{ color: 'var(--muted)' }}>Generating QR…</p>}
            <p style={{ fontSize: 12, color: 'var(--muted)', margin: '12px 0' }}>The lecturer scans this with their phone to open their attendance dashboard — every unit they teach, separated and filterable. No password.</p>
            <div style={{ display: 'flex', gap: 8, justifyContent: 'center' }}>
              {qrImg && <a href={qrImg} download={`lecturer-qr-${qr.staff_id || qr.name}.png`} style={{ ...btnPrimary, textDecoration: 'none' }}>Download</a>}
              <button onClick={() => { navigator.clipboard?.writeText(qr.url) }} style={btnSmall}>Copy link</button>
              <button onClick={() => setQr(null)} style={btnSmall}>Close</button>
            </div>
          </div>
        </div>
      )}

      {enroll && (
        <div onClick={() => setEnroll(null)} style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.6)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 9999, padding: 20 }}>
          <div onClick={e => e.stopPropagation()} style={{ background: '#fff', borderRadius: 14, padding: 24, maxWidth: 440, width: '100%' }}>
            <h3 style={{ margin: '0 0 4px' }}>Enroll fingerprint — {enroll.name}</h3>
            <p style={{ color: 'var(--muted)', fontSize: 13, marginBottom: 14 }}>
              The lecturer opens this link <strong>on their own phone</strong> and registers their fingerprint
              (or Face unlock). It works once and expires in 24 hours. Send it to them, or have them open it now.
            </p>
            <div style={{ display: 'flex', gap: 8 }}>
              <input readOnly value={enroll.url} onFocus={e => e.currentTarget.select()}
                style={{ flex: 1, padding: '9px 10px', border: '1px solid #cbd5e1', borderRadius: 8, fontSize: 12, fontFamily: 'monospace' }} />
              <button onClick={() => { navigator.clipboard?.writeText(enroll.url); }} style={btnPrimary}>Copy</button>
            </div>
            <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 16 }}>
              <a href={enroll.url} target="_blank" rel="noreferrer" style={{ ...btnSmall, textDecoration: 'none' }}>Open link</a>
              <button onClick={() => setEnroll(null)} style={btnSmall}>Close</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function Input({ label, value, onChange, placeholder }: {
  label: string; value: string; onChange: (v: string) => void; placeholder?: string
}) {
  return (
    <label style={{ display: 'block' }}>
      <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 4, color: '#475569' }}>{label}</div>
      <input value={value} placeholder={placeholder} onChange={e => onChange(e.target.value)}
        style={{ width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }} />
    </label>
  )
}

const selectStyle: React.CSSProperties = { width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, background: '#fff', boxSizing: 'border-box' }
const btnPrimary: React.CSSProperties = { padding: '8px 16px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const btnSmall:   React.CSSProperties = { padding: '6px 12px', background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 4, cursor: 'pointer', fontSize: 12, color: '#1e293b' }
const errorBox:   React.CSSProperties = { background: '#fef2f2', color: '#b91c1c', padding: '8px 12px', borderRadius: 6, marginBottom: 12, fontSize: 13 }
