import { useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

// email is OPTIONAL — correspondence only; the staff ID is the lecturer's identity.
//
// `school` IS here and `department` is not, and the difference is the org model. Every lecturer
// sits under one college, stored on the person, so a new lecturer with no units yet still belongs
// somewhere. Their department is not stored at all: they can teach across several at once, so it
// comes from the units they are assigned to. Offering a department column would invite an entry
// that is then ignored.
//
// The school cell accepts the short form or the full title — "SOMAC" and "School of Mathematics
// and Computing" resolve to the same college.
const LECT_COLS = ['staff_id', 'full_name', 'email', 'phone', 'title', 'gender', 'school']
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
  staff_id: string
  /**
   * DERIVED, never stored on the lecturer. A lecturer has no department or school of their own —
   * they can teach across several colleges, so the units they are assigned to are what carry the
   * org unit, and each department and school reaches them through those. Read-only here.
   */
  departments: string[]
  schools: string[]
  // The lecturer's HOME college — stored, one per lecturer, unlike the derived
  // `schools` above which lists everywhere they happen to teach.
  school_id: string
  school_name: string
  /** The short form — "SOMAC". What the table column shows, because a column of full college
   *  titles pushes every other column off the screen. */
  school_abbrev: string
}

interface DeleteResult {
  deleted: number
  assignments_removed: number
  timetable_slots_kept: number
  logins_deactivated: number
}

const GENDERS = ['', 'Male', 'Female', 'Other']

export default function AdminLecturers() {
  const { tenantId } = useParams<{ tenantId: string }>()
  const { status, data, refetch } = useQuery<Lecturer[]>(
    () => api.get(`/api/v1/admin/tenants/${tenantId}/lecturers`)
  )
  const [creating, setCreating] = useState(false)
  const [form, setForm] = useState({ full_name: '', email: '', phone: '', staff_id: '', title: '', gender: '', school_id: '' })
  // The colleges a lecturer can be filed under. Every lecturer must sit under one,
  // including a new one who has not been given any unit yet — which is precisely
  // what the derived value could not express.
  const orgSchools = useQuery<{ school_id: string; name: string }[]>(
    () => api.get(`/api/v1/admin/tenants/${tenantId}/schools`), [tenantId])
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [editId, setEditId] = useState<string | null>(null)
  const [editForm, setEditForm] = useState({ full_name: '', phone: '', staff_id: '', title: '', gender: '' })

  // Biometric enrolment: issue a one-time link the lecturer opens on their phone.
  const [enroll, setEnroll] = useState<{ name: string; url: string } | null>(null)
  async function makeEnroll(l: Lecturer) {
    try {
      const res = await api.post<{ enroll_url: string }>(`/api/v1/admin/tenants/${tenantId}/lecturers/${l.lecturer_id}/enroll-link`, {})
      setEnroll({ name: l.full_name, url: res.enroll_url })
    } catch (e) { alert(e instanceof Error ? e.message : 'Could not create enrolment link') }
  }

  // Lecturers sign in with their staff ID (passwordless) — no QR is issued.

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
      setForm({ full_name: '', email: '', phone: '', staff_id: '', title: '', gender: '', school_id: '' })
      refetch()
    } catch (e) { setError(e instanceof Error ? e.message : 'Failed') }
    finally { setSaving(false) }
  }

  function startEdit(l: Lecturer) {
    setEditId(l.lecturer_id)
    setEditForm({ full_name: l.full_name, phone: l.phone || '', staff_id: l.staff_id || '', title: l.title || '', gender: l.gender || '' })
  }
  async function handleEditSave() {
    if (!editId) return
    try { await api.patch(`/api/v1/admin/lecturers/${editId}`, editForm); setEditId(null); refetch() }
    catch (e) { alert(e instanceof Error ? e.message : 'Failed') }
  }

  // ── Bulk import / export + department filter ───────────────────────────────
  const fileRef = useRef<HTMLInputElement>(null)
  const [importing, setImporting] = useState(false)
  // ONE notice bar for import, export and delete, carrying its own kind.
  //
  // It used to be an import-only string, and whether it rendered red was decided by
  // `startsWith('Import failed')`. Delete reused it, so a failed DELETE announced itself as
  // "Import failed: HTTP 404" — naming the wrong feature and hiding which one actually broke.
  // A message that has to be pattern-matched to find out if it is an error is a message that
  // will eventually lie.
  const [notice, setNotice] = useState<{ text: string; kind: 'ok' | 'error' } | null>(null)
  // Filtered by COLLEGE, not by department. A department was the wrong axis for this screen: it is
  // derived from assignments, so the filter silently hid every lecturer who had not been given a
  // unit yet — exactly the people an administrator opens this page to find. A college is stored on
  // the lecturer, so everyone appears under exactly one.
  const [school, setSchool] = useState('')
  const schoolOptions = Array.from(new Set(
    (data ?? []).map(l => l.school_abbrev || l.school_name).filter(Boolean))).sort()

  async function handleImport(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    setImporting(true); setNotice(null)
    try {
      const fd = new FormData(); fd.append('roster', file)
      const res = await api.upload<{ inserted: number; updated: number; skipped: number; errors: string[] }>(
        `/api/v1/admin/tenants/${tenantId}/lecturers/import`, fd)
      setNotice({ kind: 'ok', text: `Imported: ${res.inserted} new, ${res.updated} updated, ${res.skipped} skipped${res.errors?.length ? ` · ${res.errors.length} error(s): ${res.errors.slice(0, 3).join('; ')}` : ''}` })
      refetch()
    } catch (e) { setNotice({ kind: 'error', text: e instanceof Error ? `Import failed: ${e.message}` : 'Import failed' }) }
    finally { setImporting(false); if (fileRef.current) fileRef.current.value = '' }
  }

  // ── Removing lecturers ─────────────────────────────────────────────────────
  const [selected, setSelected] = useState<string[]>([])
  const [deleting, setDeleting] = useState(false)

  /**
   * Delete, with the consequences shown BEFORE the confirm rather than after it.
   *
   * An administrator pressing this cannot see what hangs off a lecturer, so the prompt says it:
   * their unit assignments, fingerprint and passkey go, their timetabled lectures survive with
   * nobody attached, and their sign-in is disabled. It also says what does NOT go — the teaching
   * record — because that is the thing people assume a delete erases, and being wrong about it in
   * either direction is bad: someone deletes expecting the history gone, or refuses to tidy the
   * list for fear of losing it.
   */
  async function handleDelete(ids: string[], label?: string) {
    if (ids.length === 0) return
    const who = label ? `"${label}"` : `${ids.length} lecturers`
    if (!window.confirm(
      `Delete ${who}?\n\n` +
      `This removes their unit assignments, enrolled fingerprint and passkey, and disables their sign-in. ` +
      `Their timetabled lectures stay on the timetable with no lecturer attached.\n\n` +
      `Their teaching record is NOT deleted — attendance logs, patrol ticks and presence claims are kept.\n\n` +
      `This cannot be undone.`)) return
    setDeleting(true); setNotice(null)
    try {
      const res = ids.length === 1
        ? await api.delete<DeleteResult>(`/api/v1/admin/tenants/${tenantId}/lecturers/${ids[0]}`)
        : await api.post<DeleteResult>(`/api/v1/admin/tenants/${tenantId}/lecturers/bulk-delete`, { lecturer_ids: ids })
      setNotice({ kind: 'ok', text:
        `Deleted ${res.deleted} lecturer(s). ` +
        `${res.assignments_removed} assignment(s) removed, ` +
        `${res.timetable_slots_kept} timetabled lecture(s) kept without a lecturer, ` +
        `${res.logins_deactivated} sign-in(s) disabled.` })
      setSelected([])
      refetch()
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e)
      // A 404 here does not mean "no such lecturer" — it means this SERVER has no delete route,
      // i.e. it is running a build from before the feature. Saying "HTTP 404" sends someone
      // hunting for a lecturer that is sitting right there in the table.
      setNotice({ kind: 'error', text: /\b404\b/.test(msg)
        ? 'This server does not have the delete feature yet — it is running an older build. Redeploy the API gateway from the current source, then try again.'
        : `Delete failed: ${msg}` })
    } finally { setDeleting(false) }
  }

  const [search, setSearch] = useState('')
  const q = search.trim().toLowerCase()
  const lecturers = (status === 'ok' ? (data ?? []) : []).filter(l =>
    (!school || (l.school_abbrev || l.school_name) === school) &&
    (!q || [l.full_name, l.staff_id, l.phone, l.title, l.school_abbrev, l.school_name, ...(l.departments ?? [])]
      .some(v => (v || '').toLowerCase().includes(q))))

  // Every VISIBLE row, not every row in the institution: the header checkbox must mean what the
  // user can see, or filtering to one college and pressing it would silently select all 389.
  const allShownSelected = lecturers.length > 0 && lecturers.every(l => selected.includes(l.lecturer_id))

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <div>
          <h2 style={{ margin: '0' }}>Lecturers</h2>
        </div>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <button onClick={editPrefix} style={btnSmall} title="Format for auto-generated staff IDs">
            Auto-ID prefix: {prefix || '(set)'}
          </button>
          <a href={`/admin/tenants/${tenantId}/lecturer-assignments`} style={{ ...btnSmall, textDecoration: 'none', display: 'inline-block' }}>
            View Assignments
          </a>
          <button onClick={() => downloadText('lecturers_template.csv', LECT_COLS.join(',') + '\n')} style={btnSmall} title="Download a blank CSV with the lecturer columns">Template</button>
          <button onClick={() => api.download(`/api/v1/admin/tenants/${tenantId}/lecturers/export.xlsx`, 'lecturers.xlsx').catch(e => alert(e instanceof Error ? e.message : 'Export failed'))} style={btnSmall} title="Exports every lecturer">Export Excel</button>
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
            <Input label="Email (optional)" value={form.email} onChange={v => setForm(f => ({ ...f, email: v }))} placeholder="lecturer@university.edu — leave blank to skip" />
            <Input label="Phone" value={form.phone} onChange={v => setForm(f => ({ ...f, phone: v }))} placeholder="+256 700 000000" />
            <Input label="Staff ID (optional — auto-generated if left blank)" value={form.staff_id} onChange={v => setForm(f => ({ ...f, staff_id: v }))} placeholder="leave blank to auto-generate e.g. KIU/STAFF/00001" />
            <label style={{ display: 'block' }}>
              <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--muted)', marginBottom: 4 }}>College / School</div>
              <select value={form.school_id} onChange={e => setForm(f => ({ ...f, school_id: e.target.value }))} style={selectStyle}>
                <option value="">— select a college —</option>
                {(orgSchools.data ?? []).map(sc => <option key={sc.school_id} value={sc.school_id}>{sc.name}</option>)}
              </select>
            </label>
          </div>
          {/* No department or school here on purpose. A lecturer can teach in several colleges at
              once, so one field on the person could only ever be wrong for the rest. The units
              they are assigned to carry the department and the school, and that is how each HOD
              and dean reaches them — assign the lecturer a unit under Assignments. */}
          <p style={{ color: 'var(--muted)', fontSize: 12, margin: '14px 0 0', maxWidth: 620 }}>
            No department or school is set here. A lecturer belongs to the <strong>units they
            teach</strong> — assign them one under <a href={`/admin/tenants/${tenantId}/lecturer-assignments`} style={{ color: 'var(--brand)' }}>Assignments</a>{' '}
            and they appear to that unit's HOD and dean automatically, in every school they teach in.
          </p>
          <button onClick={handleCreate} disabled={saving || !form.full_name} style={{ ...btnPrimary, marginTop: 16, opacity: !form.full_name ? 0.5 : 1 }}>
            {saving ? 'Saving…' : 'Register Lecturer'}
          </button>
        </div>
      )}

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}

      {notice && (
        <div style={{
          background: notice.kind === 'error' ? '#fef2f2' : '#f0fdf4',
          color: notice.kind === 'error' ? '#b91c1c' : '#166534',
          padding: '10px 14px', borderRadius: 8, marginBottom: 16, fontSize: 13,
        }}>{notice.text}</div>
      )}

      <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 12, flexWrap: 'wrap' }}>
        <input value={search} onChange={e => setSearch(e.target.value)} placeholder="Search by name, staff ID, school/college or phone…"
          style={{ flex: 1, minWidth: 260, maxWidth: 420, padding: '8px 12px', borderRadius: 8, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }} />
        <select value={school} onChange={e => setSchool(e.target.value)} style={{ ...selectStyle, width: 'auto', minWidth: 200 }}>
          <option value="">All schools / colleges</option>
          {schoolOptions.map(d => <option key={d} value={d}>{d}</option>)}
        </select>
      </div>

      {selected.length > 0 && (
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '10px 14px', marginBottom: 12,
                      background: '#fef2f2', border: '1px solid #fecaca', borderRadius: 8 }}>
          <strong style={{ color: '#b91c1c' }}>{selected.length} selected</strong>
          <button onClick={() => handleDelete(selected)} disabled={deleting}
            style={{ ...btnSmall, background: '#b91c1c', color: '#fff', borderColor: '#b91c1c' }}>
            {deleting ? 'Deleting…' : `Delete ${selected.length} lecturer${selected.length > 1 ? 's' : ''}`}
          </button>
          <button onClick={() => setSelected([])} style={btnSmall}>Clear selection</button>
        </div>
      )}

      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
        <thead>
          <tr style={{ background: '#f8fafc' }}>
            <th style={{ padding: '8px 12px', width: 34, borderBottom: '1px solid #e2e8f0' }}>
              <input
                type="checkbox"
                aria-label="Select every lecturer shown"
                checked={allShownSelected}
                onChange={e => setSelected(e.target.checked ? lecturers.map(l => l.lecturer_id) : [])}
              />
            </th>
            {/* ONE college column, not the two this had. "College" and "Teaches in" sat side by
                side showing a stored college and a derived department list, which read as two
                answers to the same question — and the derived one was blank for every lecturer
                without an assignment. The short form is shown because the full titles are long
                enough that a column of them pushes the rest of the table off screen. */}
            {['Title', 'Name', 'Gender', 'Staff ID', 'Phone', 'School / College', ''].map(h => (
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
                </div>
                <div style={{ marginTop: 10, display: 'flex', gap: 8 }}>
                  <button onClick={handleEditSave} style={{ ...btnPrimary, background: '#92400e' }}>Save</button>
                  <button onClick={() => setEditId(null)} style={btnSmall}>Cancel</button>
                </div>
              </td>
            </tr>
          ) : (
            <tr key={l.lecturer_id} style={{ borderBottom: '1px solid #f1f5f9', background: selected.includes(l.lecturer_id) ? '#eff6ff' : undefined }}>
              <td style={{ padding: '10px 12px' }}>
                <input
                  type="checkbox"
                  aria-label={`Select ${l.full_name}`}
                  checked={selected.includes(l.lecturer_id)}
                  onChange={e => setSelected(s => e.target.checked ? [...s, l.lecturer_id] : s.filter(x => x !== l.lecturer_id))}
                />
              </td>
              <td style={{ padding: '10px 12px' }}>{l.title || '—'}</td>
              <td style={{ padding: '10px 12px', fontWeight: 600 }}>{l.full_name}</td>
              <td style={{ padding: '10px 12px', color: 'var(--muted)' }}>{l.gender || '—'}</td>
              <td style={{ padding: '10px 12px', fontFamily: 'monospace', fontSize: 12 }}>{l.staff_id || '—'}</td>
              <td style={{ padding: '10px 12px', color: 'var(--muted)' }}>{l.phone || '—'}</td>
              {/* The short form, with the full title on hover — and said plainly when there is
                  none, because a lecturer under no college is invisible to every dean and HOD. */}
              <td style={{ padding: '10px 12px', fontWeight: 600 }} title={l.school_name || undefined}>
                {l.school_abbrev || l.school_name
                  ? (l.school_abbrev || l.school_name)
                  : <span style={{ color: '#b45309', fontSize: 12, fontWeight: 400 }}>no college set</span>}
              </td>
              <td style={{ padding: '10px 12px', whiteSpace: 'nowrap' }}>
                <button onClick={() => startEdit(l)} style={btnSmall}>Edit</button>
                <button onClick={() => makeEnroll(l)} style={{ ...btnSmall, marginLeft: 6, background: '#eef2ff', borderColor: '#c7d2fe', color: '#3730a3' }}>Enroll FP</button>
                <button onClick={() => handleDelete([l.lecturer_id], l.full_name)} style={{ ...btnSmall, marginLeft: 6, background: '#fef2f2', borderColor: '#fecaca', color: '#b91c1c' }}>Delete</button>
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
