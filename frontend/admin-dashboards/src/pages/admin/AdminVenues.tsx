import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

interface Venue {
  venue_id: string; name: string; building: string; floor: number; capacity: number
}

export default function AdminVenues() {
  const { tenantId } = useParams<{ tenantId: string }>()
  const { status, data: venues, refetch } = useQuery<Venue[]>(
    () => api.get(`/api/v1/admin/tenants/${tenantId}/venues`),
    [tenantId],
  )

  const [creating, setCreating] = useState(false)
  const [form, setForm] = useState({ venue_id: '', name: '', building: '', floor: 0, capacity: 0 })
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleCreate() {
    setSaving(true); setError(null)
    try {
      await api.post(`/api/v1/admin/tenants/${tenantId}/venues`, form)
      setCreating(false)
      setForm({ venue_id: '', name: '', building: '', floor: 0, capacity: 0 })
      refetch()
    } catch (e) { setError(e instanceof Error ? e.message : 'Failed') }
    finally { setSaving(false) }
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 20 }}>
        <div>
          <a href="/admin/tenants" style={{ color: 'var(--muted)', fontSize: 13, textDecoration: 'none' }}>← Tenants</a>
          <h2 style={{ margin: '4px 0 0' }}>Venues</h2>
          <p style={{ color: 'var(--muted)', margin: '4px 0 0', fontSize: 13 }}>Tenant: {tenantId}</p>
        </div>
        <button onClick={() => setCreating(c => !c)} style={btnPrimary}>
          {creating ? 'Cancel' : '+ New Venue'}
        </button>
      </div>

      {creating && (
        <div style={{ background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 10, padding: 20, marginBottom: 24 }}>
          <h3 style={{ margin: '0 0 16px' }}>Add Venue</h3>
          {error && <div style={errorBox}>{error}</div>}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <Input label="Venue ID (unique key)" value={form.venue_id} onChange={v => setForm(f => ({ ...f, venue_id: v }))} placeholder="e.g. LH-101" />
            <Input label="Venue Name" value={form.name} onChange={v => setForm(f => ({ ...f, name: v }))} placeholder="e.g. Lecture Hall 101" />
            <Input label="Building" value={form.building} onChange={v => setForm(f => ({ ...f, building: v }))} placeholder="e.g. Main Block" />
            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Floor</div>
              <input type="number" value={form.floor} min={0}
                onChange={e => setForm(f => ({ ...f, floor: Number(e.target.value) }))}
                style={{ width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }} />
            </label>
            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Capacity (seats)</div>
              <input type="number" value={form.capacity} min={0}
                onChange={e => setForm(f => ({ ...f, capacity: Number(e.target.value) }))}
                style={{ width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }} />
            </label>
          </div>
          <button onClick={handleCreate} disabled={saving || !form.venue_id || !form.name} style={{ ...btnPrimary, marginTop: 16 }}>
            {saving ? 'Adding…' : 'Add Venue'}
          </button>
        </div>
      )}

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}

      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
        <thead>
          <tr style={{ background: '#f8fafc' }}>
            {['Venue ID', 'Name', 'Building', 'Floor', 'Capacity'].map(h => (
              <th key={h} style={{ padding: '8px 12px', textAlign: 'left', borderBottom: '1px solid #e2e8f0' }}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {(venues ?? []).map(v => (
            <tr key={v.venue_id} style={{ borderBottom: '1px solid #f1f5f9' }}>
              <td style={{ padding: '10px 12px', fontFamily: 'monospace', fontSize: 12 }}>{v.venue_id}</td>
              <td style={{ padding: '10px 12px', fontWeight: 600 }}>{v.name}</td>
              <td style={{ padding: '10px 12px', color: 'var(--muted)' }}>{v.building || '—'}</td>
              <td style={{ padding: '10px 12px', textAlign: 'center' }}>{v.floor}</td>
              <td style={{ padding: '10px 12px', textAlign: 'center' }}>{v.capacity || '—'}</td>
            </tr>
          ))}
          {status === 'ok' && (venues ?? []).length === 0 && (
            <tr>
              <td colSpan={5} style={{ padding: 32, textAlign: 'center', color: 'var(--muted)' }}>
                No venues yet. Click "+ New Venue" to add one.
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
}

function Input({ label, value, onChange, placeholder }: {
  label: string; value: string; onChange: (v: string) => void; placeholder?: string
}) {
  return (
    <label style={{ display: 'block' }}>
      <div style={labelStyle}>{label}</div>
      <input value={value} placeholder={placeholder} onChange={e => onChange(e.target.value)}
        style={{ width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }} />
    </label>
  )
}

const btnPrimary: React.CSSProperties = { padding: '8px 16px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const labelStyle: React.CSSProperties = { fontSize: 13, fontWeight: 600, marginBottom: 4, color: '#475569' }
const errorBox:   React.CSSProperties = { background: '#fef2f2', color: '#b91c1c', padding: '8px 12px', borderRadius: 6, marginBottom: 12, fontSize: 13 }
