import { useState, useRef, useEffect } from 'react'
import { useParams } from 'react-router-dom'
import QRCode from 'qrcode'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

interface Coordinator {
  user_id: string
  registration_number: string
  coordinator_code: string
  title: string
  full_name: string
  gender: string
  email: string
  phone: string
  whatsapp: string
  course: string
  level: string
  session: string
  study_year: number
  semester: number
  intake: string
  is_active: boolean
}

const GENDERS = ['', 'Male', 'Female', 'Other']

export default function AdminCoordinators() {
  const { tenantId } = useParams<{ tenantId: string }>()
  const { status, data, refetch } = useQuery<Coordinator[]>(
    () => api.get(`/api/v1/admin/tenants/${tenantId}/coordinators`), [tenantId])
  const titlesQ = useQuery<{ titles: string[] }>(() => api.get('/api/v1/admin/settings/titles'), [tenantId])
  const titles = (titlesQ.status === 'ok' ? titlesQ.data?.titles : null) ?? []

  const [search, setSearch] = useState('')
  const [importing, setImporting] = useState(false)
  const [msg, setMsg] = useState<string | null>(null)
  const fileRef = useRef<HTMLInputElement>(null)

  const [editId, setEditId] = useState<string | null>(null)
  const [editForm, setEditForm] = useState<Partial<Coordinator>>({})

  const [qr, setQr] = useState<{ name: string; url: string; coordCode: string } | null>(null)
  const [qrImg, setQrImg] = useState('')
  async function showQR(c: Coordinator) {
    try {
      const res = await api.get<{ full_name: string; url: string; coordinator_code: string }>(
        `/api/v1/admin/tenants/${tenantId}/coordinators/${c.user_id}/qr`)
      setQr({ name: res.full_name, url: res.url, coordCode: res.coordinator_code })
    } catch (e) { alert(e instanceof Error ? e.message : 'Could not load QR') }
  }
  useEffect(() => {
    if (!qr) { setQrImg(''); return }
    QRCode.toDataURL(qr.url, { width: 320, margin: 2, errorCorrectionLevel: 'H' }).then(setQrImg).catch(() => setQrImg(''))
  }, [qr])

  const all = status === 'ok' ? (data ?? []) : []
  const q = search.toLowerCase()
  const list = all.filter(c => !q ||
    c.full_name.toLowerCase().includes(q) || c.email.toLowerCase().includes(q) ||
    c.coordinator_code.toLowerCase().includes(q) || c.course.toLowerCase().includes(q))

  async function handleImport(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    setImporting(true); setMsg(null)
    try {
      const fd = new FormData(); fd.append('roster', file)
      const r = await api.upload<{ created: number; updated: number; skipped: number; new_logins: { email: string; temp_password: string }[] }>(
        `/api/v1/admin/tenants/${tenantId}/coordinators/import`, fd)
      const creds = r.new_logins?.length ? ` · new logins: ${r.new_logins.map(n => `${n.email}=${n.temp_password}`).join(', ')}` : ''
      setMsg(`Imported: ${r.created} new, ${r.updated} updated, ${r.skipped} skipped${creds}`)
      refetch()
    } catch (e) { setMsg(e instanceof Error ? `Import failed: ${e.message}` : 'Import failed') }
    finally { setImporting(false); if (fileRef.current) fileRef.current.value = '' }
  }

  function startEdit(c: Coordinator) {
    setEditId(c.user_id)
    setEditForm({ title: c.title, full_name: c.full_name, gender: c.gender, phone: c.phone, whatsapp: c.whatsapp, registration_number: c.registration_number })
  }
  async function saveEdit() {
    if (!editId) return
    try {
      await api.patch(`/api/v1/admin/users/${editId}`, {
        full_name: editForm.full_name, title: editForm.title, gender: editForm.gender,
        phone: editForm.phone, whatsapp: editForm.whatsapp, registration_number: editForm.registration_number,
      })
      setEditId(null); refetch()
    } catch (e) { alert(e instanceof Error ? e.message : 'Failed') }
  }
  async function del(c: Coordinator) {
    if (!confirm(`Permanently delete coordinator ${c.full_name} (${c.email})?\nThis cannot be undone.`)) return
    try { await api.delete(`/api/v1/admin/users/${c.user_id}`); refetch() }
    catch (e) { alert(e instanceof Error ? e.message : 'Delete failed (a coordinator with sessions cannot be deleted).') }
  }
  async function manageTitles() {
    const v = window.prompt('Titles offered (comma-separated, e.g. Prof., Dr., Eng., Mr., Mrs., Ms.):', titles.join(', '))
    if (v === null) return
    const arr = v.split(',').map(s => s.trim()).filter(Boolean)
    try { await api.put('/api/v1/admin/settings/titles', { titles: arr }); titlesQ.refetch() }
    catch (e) { alert(e instanceof Error ? e.message : 'Failed') }
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 16 }}>
        <div>
          <a href="/admin/tenants" style={{ color: 'var(--muted)', fontSize: 13, textDecoration: 'none' }}>← Home</a>
          <h2 style={{ margin: '4px 0 0' }}>Coordinators</h2>
          <p style={{ color: 'var(--muted)', margin: '4px 0 0', fontSize: 13 }}>Directory of coordinators with their contacts and the session they coordinate.</p>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button onClick={manageTitles} style={btnGhost} title="Define the title list (Prof., Dr., Eng.…)">Manage titles</button>
          <button onClick={() => api.download(`/api/v1/admin/tenants/${tenantId}/coordinators/export.xlsx`, 'coordinators.xlsx').catch(e => alert(e instanceof Error ? e.message : 'Export failed'))} style={btnGhost}>Export Excel</button>
          <input ref={fileRef} type="file" accept=".csv,.xlsx,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" onChange={handleImport} style={{ display: 'none' }} />
          <button onClick={() => fileRef.current?.click()} disabled={importing} style={btnGhost}>{importing ? 'Importing…' : 'Import (CSV/Excel)'}</button>
        </div>
      </div>

      {msg && <div style={{ background: msg.startsWith('Import failed') ? '#fef2f2' : '#f0fdf4', color: msg.startsWith('Import failed') ? '#b91c1c' : '#166534', padding: '10px 14px', borderRadius: 8, marginBottom: 14, fontSize: 13 }}>{msg}</div>}

      <input value={search} onChange={e => setSearch(e.target.value)} placeholder="Search name, email, ID, course…"
        style={{ padding: '8px 12px', borderRadius: 8, border: '1px solid #e2e8f0', fontSize: 14, width: 320, marginBottom: 14 }} />

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}

      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
        <thead>
          <tr style={{ background: '#f8fafc' }}>
            {['Reg No.', 'Unique ID', 'Title', 'Name', 'Gender', 'Email', 'Phone', 'WhatsApp', 'Course', 'Level', 'Session', 'Year/Sem', 'Intake', ''].map(h => (
              <th key={h} style={{ padding: '8px 10px', textAlign: 'left', borderBottom: '1px solid #e2e8f0', whiteSpace: 'nowrap' }}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {list.map(c => editId === c.user_id ? (
            <tr key={c.user_id} style={{ background: '#fefce8' }}>
              <td colSpan={14} style={{ padding: 12 }}>
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 10 }}>
                  <label><div style={lbl}>Title</div>
                    <select value={editForm.title ?? ''} onChange={e => setEditForm(f => ({ ...f, title: e.target.value }))} style={inp}>
                      <option value="">—</option>
                      {titles.map(t => <option key={t} value={t}>{t}</option>)}
                    </select></label>
                  <Edit label="Full name" value={editForm.full_name ?? ''} onChange={v => setEditForm(f => ({ ...f, full_name: v }))} />
                  <label><div style={lbl}>Gender</div>
                    <select value={editForm.gender ?? ''} onChange={e => setEditForm(f => ({ ...f, gender: e.target.value }))} style={inp}>
                      {GENDERS.map(g => <option key={g} value={g}>{g || '—'}</option>)}
                    </select></label>
                  <Edit label="Phone" value={editForm.phone ?? ''} onChange={v => setEditForm(f => ({ ...f, phone: v }))} />
                  <Edit label="WhatsApp" value={editForm.whatsapp ?? ''} onChange={v => setEditForm(f => ({ ...f, whatsapp: v }))} />
                  <Edit label="Registration No." value={editForm.registration_number ?? ''} onChange={v => setEditForm(f => ({ ...f, registration_number: v }))} />
                </div>
                <div style={{ marginTop: 10, display: 'flex', gap: 8 }}>
                  <button onClick={saveEdit} style={{ ...btnGhost, background: '#92400e', color: '#fff', borderColor: '#92400e' }}>Save</button>
                  <button onClick={() => setEditId(null)} style={btnGhost}>Cancel</button>
                </div>
              </td>
            </tr>
          ) : (
            <tr key={c.user_id} style={{ borderBottom: '1px solid #f1f5f9', opacity: c.is_active ? 1 : 0.5 }}>
              <td style={{ padding: '8px 10px', fontFamily: 'monospace', fontSize: 12 }}>{c.registration_number || '—'}</td>
              <td style={{ padding: '8px 10px', fontFamily: 'monospace', fontSize: 12, color: '#0369a1' }}>{c.coordinator_code || '—'}</td>
              <td style={{ padding: '8px 10px' }}>{c.title || '—'}</td>
              <td style={{ padding: '8px 10px', fontWeight: 600 }}>{c.full_name}</td>
              <td style={{ padding: '8px 10px' }}>{c.gender || '—'}</td>
              <td style={{ padding: '8px 10px', color: 'var(--muted)' }}>{c.email}</td>
              <td style={{ padding: '8px 10px', color: 'var(--muted)' }}>{c.phone || '—'}</td>
              <td style={{ padding: '8px 10px', color: 'var(--muted)' }}>{c.whatsapp || '—'}</td>
              <td style={{ padding: '8px 10px' }}>{c.course || <span style={{ color: '#f59e0b' }}>unassigned</span>}</td>
              <td style={{ padding: '8px 10px' }}>{c.level || '—'}</td>
              <td style={{ padding: '8px 10px' }}>{c.session ? <span style={{ background: '#f0fdf4', color: '#166534', padding: '2px 8px', borderRadius: 999, fontSize: 11, fontWeight: 600 }}>{c.session}</span> : '—'}</td>
              <td style={{ padding: '8px 10px', whiteSpace: 'nowrap' }}>{c.study_year || c.semester ? `Y${c.study_year} · S${c.semester}` : '—'}</td>
              <td style={{ padding: '8px 10px', color: 'var(--muted)' }}>{c.intake || '—'}</td>
              <td style={{ padding: '8px 10px', whiteSpace: 'nowrap' }}>
                <button onClick={() => startEdit(c)} style={{ ...btnTiny, marginRight: 4 }}>Edit</button>
                <button onClick={() => showQR(c)} style={{ ...btnTiny, marginRight: 4, background: '#fef9c3', borderColor: '#fde68a', color: '#854d0e' }} title="Show this coordinator's QR — scan to open their cohort dashboard">QR</button>
                <button onClick={() => del(c)} style={{ ...btnTiny, color: '#b91c1c', borderColor: '#fecaca', background: '#fef2f2' }}>Delete</button>
              </td>
            </tr>
          ))}
          {status === 'ok' && list.length === 0 && (
            <tr><td colSpan={12} style={{ padding: 28, textAlign: 'center', color: 'var(--muted)' }}>
              {all.length === 0 ? 'No coordinators yet. Add them under Users, then assign each to a session under Courses.' : 'No coordinators match the search.'}
            </td></tr>
          )}
        </tbody>
      </table>

      {qr && (
        <div onClick={() => setQr(null)} style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.6)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 9999, padding: 20 }}>
          <div onClick={e => e.stopPropagation()} style={{ background: '#fff', borderRadius: 14, padding: 24, maxWidth: 380, width: '100%', textAlign: 'center' }}>
            <h3 style={{ margin: '0 0 2px' }}>{qr.name}</h3>
            {qr.coordCode && <div style={{ fontSize: 12, color: 'var(--muted)', fontFamily: 'monospace', marginBottom: 12 }}>{qr.coordCode}</div>}
            {qrImg
              ? <img src={qrImg} alt="Coordinator QR" style={{ width: 280, height: 280 }} />
              : <p style={{ color: 'var(--muted)' }}>Generating QR…</p>}
            <p style={{ fontSize: 12, color: 'var(--muted)', margin: '12px 0' }}>The coordinator scans this with their phone to open their cohort dashboard — scoped to just their cohort. No password needed.</p>
            <div style={{ display: 'flex', gap: 8, justifyContent: 'center' }}>
              {qrImg && <a href={qrImg} download={`coordinator-qr-${qr.coordCode || qr.name}.png`} style={{ ...btnGhost, textDecoration: 'none', fontWeight: 600, background: '#1e293b', color: '#fff' }}>Download</a>}
              <button onClick={() => { navigator.clipboard?.writeText(qr.url) }} style={btnGhost}>Copy link</button>
              <button onClick={() => setQr(null)} style={btnGhost}>Close</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function Edit({ label, value, onChange }: { label: string; value: string; onChange: (v: string) => void }) {
  return <label><div style={lbl}>{label}</div><input value={value} onChange={e => onChange(e.target.value)} style={inp} /></label>
}

const btnGhost: React.CSSProperties = { padding: '8px 12px', background: '#fff', color: '#334155', border: '1px solid #e2e8f0', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const btnTiny: React.CSSProperties = { padding: '3px 9px', background: '#fff', color: '#334155', border: '1px solid #e2e8f0', borderRadius: 4, cursor: 'pointer', fontSize: 12 }
const lbl: React.CSSProperties = { fontSize: 12, fontWeight: 600, marginBottom: 3, color: '#475569' }
const inp: React.CSSProperties = { width: '100%', padding: '7px 9px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 13, boxSizing: 'border-box', background: '#fff' }
