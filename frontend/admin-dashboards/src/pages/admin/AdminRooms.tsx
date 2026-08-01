import { useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

// Rooms & room codes. The room code (LR-101, LAB2) is the key the timetable, the patroller app and
// every past session point at, so it is chosen once and never edited — everything else about a room
// is. Retiring a room deactivates it rather than deleting it, which is why a room still carrying
// timetable slots refuses to delete.

interface Room {
  room_code: string; name: string; building: string; floor: number; capacity: number
  room_type: string; is_active: boolean
  school_id: string; school_name: string
  department_id: string; department_name: string
  slot_count: number
}
interface School { school_id: string; name: string }
interface Department { department_id: string; school_id: string; name: string }
interface ImportResult { inserted: number; updated: number; skipped: number; errors: string[] }

const ROOM_TYPES = ['LECTURE_HALL', 'LAB', 'SEMINAR', 'WORKSHOP', 'OFFICE', 'OTHER']

const blankForm = {
  room_code: '', name: '', building: '', floor: '', capacity: '',
  room_type: 'LECTURE_HALL', school_id: '', department_id: '',
}

export default function AdminRooms() {
  const { tenantId } = useParams<{ tenantId: string }>()
  const rooms = useQuery<Room[]>(() => api.get(`/api/v1/admin/tenants/${tenantId}/rooms`), [tenantId])
  const schools = useQuery<School[]>(() => api.get(`/api/v1/admin/tenants/${tenantId}/schools`), [tenantId])
  const depts = useQuery<Department[]>(() => api.get(`/api/v1/admin/tenants/${tenantId}/departments`), [tenantId])

  const [creating, setCreating] = useState(false)
  const [form, setForm] = useState(blankForm)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [editing, setEditing] = useState<Room | null>(null)
  const [filter, setFilter] = useState({ q: '', school_id: '', show: 'active' as 'active' | 'all' })

  // A department only makes sense under its school, so the picker narrows with the choice.
  const deptOptions = (depts.data ?? []).filter(d => !form.school_id || d.school_id === form.school_id)

  async function create() {
    setSaving(true); setError(null)
    try {
      await api.post(`/api/v1/admin/tenants/${tenantId}/rooms`, {
        room_code: form.room_code,
        name: form.name,
        building: form.building,
        floor: form.floor === '' ? 0 : Number(form.floor),
        capacity: form.capacity === '' ? 0 : Number(form.capacity),
        room_type: form.room_type,
        school_id: form.school_id,
        department_id: form.department_id,
      })
      setCreating(false); setForm(blankForm); rooms.refetch()
    } catch (e) { setError(e instanceof Error ? e.message : 'Failed') }
    finally { setSaving(false) }
  }

  async function setActive(r: Room, is_active: boolean) {
    await api.patch(`/api/v1/admin/tenants/${tenantId}/rooms/${encodeURIComponent(r.room_code)}`, { is_active })
    rooms.refetch()
  }

  async function remove(r: Room) {
    if (!confirm(`Delete room ${r.room_code}?`)) return
    try { await api.delete(`/api/v1/admin/tenants/${tenantId}/rooms/${encodeURIComponent(r.room_code)}`); rooms.refetch() }
    catch (e) { alert(e instanceof Error ? e.message : 'Delete failed') }
  }

  const list = (rooms.data ?? []).filter(r => {
    if (filter.show === 'active' && !r.is_active) return false
    if (filter.school_id && r.school_id !== filter.school_id) return false
    const q = filter.q.trim().toLowerCase()
    if (!q) return true
    return [r.room_code, r.name, r.building, r.school_name, r.department_name]
      .some(v => (v ?? '').toLowerCase().includes(q))
  })
  const inactiveCount = (rooms.data ?? []).filter(r => !r.is_active).length

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 20, gap: 12, flexWrap: 'wrap' }}>
        <div>
          <a href="/admin/tenants" style={{ color: 'var(--muted)', fontSize: 13, textDecoration: 'none' }}>← Home</a>
          <h2 style={{ margin: '4px 0 0' }}>Rooms &amp; Room Codes</h2>
          <p style={{ color: 'var(--muted)', margin: '4px 0 0', fontSize: 13 }}>
            The room code is what the timetable and the patroller app point at — pick it once, then keep it.
          </p>
        </div>
        <div style={{ display: 'flex', gap: 8, alignItems: 'flex-start', flexWrap: 'wrap' }}>
          <button onClick={() => api.download(`/api/v1/admin/tenants/${tenantId}/rooms/export.xlsx`, 'rooms.xlsx')} style={btnGhost}>⭳ Export</button>
          <ImportButton tenantId={tenantId!} onDone={rooms.refetch} />
          <button onClick={() => setCreating(c => !c)} style={btnPrimary}>{creating ? 'Cancel' : '+ New Room'}</button>
        </div>
      </div>

      {creating && (
        <div style={panel}>
          <h3 style={{ margin: '0 0 4px' }}>Add Room</h3>
          <p style={{ ...mutedText, margin: '0 0 16px' }}>
            Room codes are unique across the whole platform, so prefix yours if it is a common one (e.g. <code>KIU-LR-101</code>).
          </p>
          {error && <div style={errorBox}>{error}</div>}
          <div style={grid}>
            <Text label="Room code" value={form.room_code} onChange={v => setForm(f => ({ ...f, room_code: v }))} placeholder="e.g. LR-101" mono />
            <Text label="Room name" value={form.name} onChange={v => setForm(f => ({ ...f, name: v }))} placeholder="e.g. Lecture Room 101" />
            <Text label="Building" value={form.building} onChange={v => setForm(f => ({ ...f, building: v }))} placeholder="e.g. Main Block" />
            <Text label="Floor" value={form.floor} onChange={v => setForm(f => ({ ...f, floor: v }))} type="number" />
            <Text label="Capacity (seats)" value={form.capacity} onChange={v => setForm(f => ({ ...f, capacity: v }))} type="number" />
            <Select label="Type" value={form.room_type} onChange={v => setForm(f => ({ ...f, room_type: v }))}
              options={ROOM_TYPES.map(t => ({ value: t, label: t.replace(/_/g, ' ') }))} />
            <Select label="College / School" value={form.school_id}
              onChange={v => setForm(f => ({ ...f, school_id: v, department_id: '' }))}
              options={[{ value: '', label: '— none —' }, ...(schools.data ?? []).map(s => ({ value: s.school_id, label: s.name }))]} />
            <Select label="Department" value={form.department_id} onChange={v => setForm(f => ({ ...f, department_id: v }))}
              options={[{ value: '', label: '— none —' }, ...deptOptions.map(d => ({ value: d.department_id, label: d.name }))]} />
          </div>
          <button onClick={create} disabled={saving || !form.room_code || !form.name} style={{ ...btnPrimary, marginTop: 16, opacity: (!form.room_code || !form.name) ? .5 : 1 }}>
            {saving ? 'Adding…' : 'Add Room'}
          </button>
        </div>
      )}

      <div style={{ display: 'flex', gap: 8, marginBottom: 14, flexWrap: 'wrap', alignItems: 'center' }}>
        <input value={filter.q} onChange={e => setFilter(f => ({ ...f, q: e.target.value }))}
          placeholder="Search code, name, building…" style={{ ...inputStyle, width: 260 }} />
        <select value={filter.school_id} onChange={e => setFilter(f => ({ ...f, school_id: e.target.value }))} style={inputStyle}>
          <option value="">All schools</option>
          {(schools.data ?? []).map(s => <option key={s.school_id} value={s.school_id}>{s.name}</option>)}
        </select>
        <label style={{ fontSize: 13, color: 'var(--muted)', display: 'flex', gap: 6, alignItems: 'center' }}>
          <input type="checkbox" checked={filter.show === 'all'}
            onChange={e => setFilter(f => ({ ...f, show: e.target.checked ? 'all' : 'active' }))} />
          Show retired ({inactiveCount})
        </label>
      </div>

      {rooms.status === 'loading' && <p style={mutedText}>Loading…</p>}

      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
        <thead>
          <tr style={{ background: '#f8fafc' }}>
            {['Room code', 'Name', 'Building', 'Floor', 'Seats', 'Type', 'School / Dept', 'In use', ''].map(h => (
              <th key={h} style={{ padding: '8px 12px', textAlign: 'left', borderBottom: '1px solid #e2e8f0' }}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {list.map(r => (
            <tr key={r.room_code} style={{ borderBottom: '1px solid #f1f5f9', opacity: r.is_active ? 1 : .55 }}>
              <td style={{ padding: '10px 12px', fontFamily: 'ui-monospace, monospace', fontSize: 12, fontWeight: 700 }}>{r.room_code}</td>
              <td style={{ padding: '10px 12px', fontWeight: 600 }}>
                {r.name}{!r.is_active && <span style={{ marginLeft: 6, fontSize: 11, color: '#b45309' }}>(retired)</span>}
              </td>
              <td style={{ padding: '10px 12px', color: 'var(--muted)' }}>{r.building || '—'}</td>
              <td style={{ padding: '10px 12px', textAlign: 'center' }}>{r.floor || '—'}</td>
              <td style={{ padding: '10px 12px', textAlign: 'center' }}>{r.capacity || '—'}</td>
              <td style={{ padding: '10px 12px', fontSize: 12, color: 'var(--muted)' }}>{(r.room_type || '').replace(/_/g, ' ')}</td>
              <td style={{ padding: '10px 12px', fontSize: 12, color: 'var(--muted)' }}>
                {r.school_name || '—'}{r.department_name ? ` · ${r.department_name}` : ''}
              </td>
              <td style={{ padding: '10px 12px', textAlign: 'center', fontSize: 12 }}>
                {r.slot_count > 0 ? `${r.slot_count} slot${r.slot_count === 1 ? '' : 's'}` : '—'}
              </td>
              <td style={{ padding: '10px 12px', whiteSpace: 'nowrap' }}>
                <button style={btnSm} onClick={() => setEditing(r)}>Edit</button>{' '}
                <button style={btnSm} onClick={() => setActive(r, !r.is_active)}>{r.is_active ? 'Retire' : 'Restore'}</button>{' '}
                {r.slot_count === 0 && <button style={btnSm} onClick={() => remove(r)}>Delete</button>}
              </td>
            </tr>
          ))}
          {rooms.status === 'ok' && list.length === 0 && (
            <tr><td colSpan={9} style={{ padding: 32, textAlign: 'center', color: 'var(--muted)' }}>
              {(rooms.data ?? []).length === 0
                ? 'No rooms yet. Add one, or import your estates list from Excel.'
                : 'No room matches those filters.'}
            </td></tr>
          )}
        </tbody>
      </table>

      {editing && (
        <EditRoomModal
          tenantId={tenantId!} room={editing}
          schools={schools.data ?? []} departments={depts.data ?? []}
          onClose={() => setEditing(null)}
          onSaved={() => { setEditing(null); rooms.refetch() }}
        />
      )}
    </div>
  )
}

function EditRoomModal({ tenantId, room, schools, departments, onClose, onSaved }: {
  tenantId: string; room: Room; schools: School[]; departments: Department[]
  onClose: () => void; onSaved: () => void
}) {
  const [f, setF] = useState({
    name: room.name, building: room.building,
    floor: String(room.floor || ''), capacity: String(room.capacity || ''),
    room_type: room.room_type || 'LECTURE_HALL',
    school_id: room.school_id, department_id: room.department_id,
  })
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const deptOptions = departments.filter(d => !f.school_id || d.school_id === f.school_id)

  async function save() {
    setBusy(true); setErr(null)
    try {
      await api.patch(`/api/v1/admin/tenants/${tenantId}/rooms/${encodeURIComponent(room.room_code)}`, {
        name: f.name, building: f.building,
        floor: f.floor === '' ? 0 : Number(f.floor),
        capacity: f.capacity === '' ? 0 : Number(f.capacity),
        room_type: f.room_type, school_id: f.school_id, department_id: f.department_id,
      })
      onSaved()
    } catch (e) { setErr(e instanceof Error ? e.message : 'Save failed') }
    finally { setBusy(false) }
  }

  return (
    <div style={overlay} onClick={onClose}>
      <div style={modal} onClick={e => e.stopPropagation()}>
        <h3 style={{ margin: '0 0 4px' }}>Edit {room.room_code}</h3>
        <p style={{ ...mutedText, margin: '0 0 14px' }}>
          The room code cannot change — {room.slot_count} timetable slot(s) and past sessions point at it.
        </p>
        {err && <div style={errorBox}>{err}</div>}
        <div style={grid}>
          <Text label="Room name" value={f.name} onChange={v => setF(s => ({ ...s, name: v }))} />
          <Text label="Building" value={f.building} onChange={v => setF(s => ({ ...s, building: v }))} />
          <Text label="Floor" value={f.floor} onChange={v => setF(s => ({ ...s, floor: v }))} type="number" />
          <Text label="Capacity (seats)" value={f.capacity} onChange={v => setF(s => ({ ...s, capacity: v }))} type="number" />
          <Select label="Type" value={f.room_type} onChange={v => setF(s => ({ ...s, room_type: v }))}
            options={ROOM_TYPES.map(t => ({ value: t, label: t.replace(/_/g, ' ') }))} />
          <Select label="College / School" value={f.school_id}
            onChange={v => setF(s => ({ ...s, school_id: v, department_id: '' }))}
            options={[{ value: '', label: '— none —' }, ...schools.map(s => ({ value: s.school_id, label: s.name }))]} />
          <Select label="Department" value={f.department_id} onChange={v => setF(s => ({ ...s, department_id: v }))}
            options={[{ value: '', label: '— none —' }, ...deptOptions.map(d => ({ value: d.department_id, label: d.name }))]} />
        </div>
        <div style={{ display: 'flex', gap: 8, marginTop: 16, justifyContent: 'flex-end' }}>
          <button style={btnGhost} onClick={onClose}>Cancel</button>
          <button style={btnPrimary} disabled={busy || !f.name} onClick={save}>{busy ? 'Saving…' : 'Save'}</button>
        </div>
      </div>
    </div>
  )
}

function ImportButton({ tenantId, onDone }: { tenantId: string; onDone: () => void }) {
  const ref = useRef<HTMLInputElement>(null)
  const [res, setRes] = useState<ImportResult | null>(null)
  const [busy, setBusy] = useState(false)

  async function pick(file: File) {
    setBusy(true); setRes(null)
    try {
      const form = new FormData()
      form.append('roster', file)
      setRes(await api.upload<ImportResult>(`/api/v1/admin/tenants/${tenantId}/rooms/import`, form))
      onDone()
    } catch (e) { alert(e instanceof Error ? e.message : 'Import failed') }
    finally { setBusy(false); if (ref.current) ref.current.value = '' }
  }

  return (
    <>
      <input ref={ref} type="file" accept=".xlsx,.csv" style={{ display: 'none' }}
        onChange={e => { const f = e.target.files?.[0]; if (f) pick(f) }} />
      <button style={btnGhost} disabled={busy} onClick={() => ref.current?.click()}>
        {busy ? 'Importing…' : '⭱ Import'}
      </button>
      {res && (
        <div style={{
          position: 'fixed', right: 20, bottom: 20, zIndex: 60, maxWidth: 420,
          background: '#fff', border: '1px solid #e2e8f0', borderRadius: 10, padding: 14, boxShadow: '0 8px 24px rgba(0,0,0,.12)',
        }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12 }}>
            <b>{res.inserted} added · {res.updated} updated{res.skipped ? ` · ${res.skipped} skipped` : ''}</b>
            <button onClick={() => setRes(null)} style={{ border: 'none', background: 'none', cursor: 'pointer', fontSize: 18, lineHeight: 1 }}>×</button>
          </div>
          {res.errors?.length > 0 && (
            <ul style={{ margin: '8px 0 0', paddingLeft: 18, fontSize: 12, color: '#b45309', maxHeight: 200, overflowY: 'auto' }}>
              {res.errors.slice(0, 25).map((e, i) => <li key={i}>{e}</li>)}
              {res.errors.length > 25 && <li>…and {res.errors.length - 25} more.</li>}
            </ul>
          )}
        </div>
      )}
    </>
  )
}

function Text({ label, value, onChange, placeholder, type, mono }: {
  label: string; value: string; onChange: (v: string) => void
  placeholder?: string; type?: string; mono?: boolean
}) {
  return (
    <label style={{ display: 'block' }}>
      <div style={labelStyle}>{label}</div>
      <input value={value} placeholder={placeholder} type={type ?? 'text'}
        onChange={e => onChange(e.target.value)}
        style={{ ...inputStyle, width: '100%', fontFamily: mono ? 'ui-monospace, monospace' : undefined }} />
    </label>
  )
}

function Select({ label, value, onChange, options }: {
  label: string; value: string; onChange: (v: string) => void
  options: { value: string; label: string }[]
}) {
  return (
    <label style={{ display: 'block' }}>
      <div style={labelStyle}>{label}</div>
      <select value={value} onChange={e => onChange(e.target.value)} style={{ ...inputStyle, width: '100%' }}>
        {options.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
      </select>
    </label>
  )
}

const grid: React.CSSProperties = { display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: 12 }
const panel: React.CSSProperties = { background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 10, padding: 20, marginBottom: 24 }
const inputStyle: React.CSSProperties = { padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }
const labelStyle: React.CSSProperties = { fontSize: 13, fontWeight: 600, marginBottom: 4, color: '#475569' }
const mutedText: React.CSSProperties = { color: 'var(--muted)', fontSize: 13 }
const btnPrimary: React.CSSProperties = { padding: '8px 16px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const btnGhost: React.CSSProperties = { padding: '8px 14px', background: '#fff', color: '#1e293b', border: '1px solid #cbd5e1', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const btnSm: React.CSSProperties = { padding: '3px 9px', background: '#fff', color: '#1e293b', border: '1px solid #cbd5e1', borderRadius: 6, cursor: 'pointer', fontSize: 12 }
const errorBox: React.CSSProperties = { background: '#fef2f2', color: '#b91c1c', padding: '8px 12px', borderRadius: 6, marginBottom: 12, fontSize: 13 }
const overlay: React.CSSProperties = { position: 'fixed', inset: 0, background: 'rgba(0,0,0,.45)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 60, padding: 16 }
const modal: React.CSSProperties = { background: '#fff', borderRadius: 12, padding: 22, width: 'min(640px, 96vw)', maxHeight: '90vh', overflowY: 'auto' }
