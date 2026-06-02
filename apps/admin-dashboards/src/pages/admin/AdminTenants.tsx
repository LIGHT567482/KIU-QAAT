import { useState } from 'react'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

interface Tenant {
  tenant_id: string; name: string; domain: string
  attendance_threshold: number; is_active: boolean; created_at: string
}

export default function AdminTenants() {
  const { status, data, refetch } = useQuery<Tenant[]>(() => api.get('/api/v1/admin/tenants'))
  const [creating, setCreating] = useState(false)
  const [form, setForm] = useState({ name: '', domain: '', attendance_threshold: 75, brand_color: '#1a73e8' })
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleCreate() {
    setSaving(true); setError(null)
    try {
      await api.post('/api/v1/admin/tenants', form)
      setCreating(false)
      setForm({ name: '', domain: '', attendance_threshold: 75, brand_color: '#1a73e8' })
      refetch()
    } catch (e) { setError(e instanceof Error ? e.message : 'Failed') }
    finally { setSaving(false) }
  }

  async function toggleStatus(t: Tenant) {
    await api.post(`/api/v1/admin/tenants/${t.tenant_id}/status`, { is_active: !t.is_active })
    refetch()
  }

  const tenants = status === 'ok' ? data : []

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 20 }}>
        <h2 style={{ margin: 0 }}>Tenants</h2>
        <button onClick={() => setCreating(c => !c)} style={btnPrimary}>
          {creating ? 'Cancel' : '+ New Tenant'}
        </button>
      </div>

      {creating && (
        <div style={{ background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 10, padding: 20, marginBottom: 24 }}>
          <h3 style={{ margin: '0 0 16px' }}>Create Tenant</h3>
          {error && <div style={errorBox}>{error}</div>}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <Input label="Institution name" value={form.name} onChange={v => setForm(f => ({ ...f, name: v }))} />
            <Input label="Domain" value={form.domain} onChange={v => setForm(f => ({ ...f, domain: v }))} placeholder="university.edu" />
            <Input label="Attendance threshold (%)" type="number" value={String(form.attendance_threshold)} onChange={v => setForm(f => ({ ...f, attendance_threshold: Number(v) }))} />
            <Input label="Brand colour (hex)" value={form.brand_color} onChange={v => setForm(f => ({ ...f, brand_color: v }))} />
          </div>
          <button onClick={handleCreate} disabled={saving} style={{ ...btnPrimary, marginTop: 16 }}>
            {saving ? 'Creating…' : 'Create Tenant'}
          </button>
        </div>
      )}

      {status === 'loading' && <p style={{ color: '#94a3b8' }}>Loading…</p>}

      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
        <thead>
          <tr style={{ background: '#f8fafc' }}>
            {['Institution', 'Domain', 'Threshold', 'Status', ''].map(h => (
              <th key={h} style={{ padding: '8px 12px', textAlign: 'left', borderBottom: '1px solid #e2e8f0' }}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {(tenants ?? []).map(t => (
            <tr key={t.tenant_id} style={{ borderBottom: '1px solid #f1f5f9' }}>
              <td style={{ padding: '10px 12px', fontWeight: 600 }}>{t.name}</td>
              <td style={{ padding: '10px 12px', color: '#64748b' }}>{t.domain}</td>
              <td style={{ padding: '10px 12px' }}>{t.attendance_threshold}%</td>
              <td style={{ padding: '10px 12px' }}>
                <span style={{ background: t.is_active ? '#f0fdf4' : '#fef2f2', color: t.is_active ? '#166534' : '#b91c1c', padding: '2px 10px', borderRadius: 999, fontSize: 12, fontWeight: 600 }}>
                  {t.is_active ? 'Active' : 'Suspended'}
                </span>
              </td>
              <td style={{ padding: '10px 12px' }}>
                <button onClick={() => toggleStatus(t)} style={{ ...btnSmall, background: t.is_active ? '#fef2f2' : '#f0fdf4', color: t.is_active ? '#b91c1c' : '#166534' }}>
                  {t.is_active ? 'Suspend' : 'Activate'}
                </button>
                <a href={`/admin/tenants/${t.tenant_id}/users`} style={{ ...btnSmall, marginLeft: 6, textDecoration: 'none', display: 'inline-block' }}>Users</a>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function Input({ label, value, onChange, type = 'text', placeholder }: {
  label: string; value: string; onChange: (v: string) => void; type?: string; placeholder?: string
}) {
  return (
    <label style={{ display: 'block' }}>
      <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 4, color: '#475569' }}>{label}</div>
      <input type={type} value={value} placeholder={placeholder} onChange={e => onChange(e.target.value)}
        style={{ width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }} />
    </label>
  )
}

const btnPrimary: React.CSSProperties = { padding: '8px 16px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const btnSmall:   React.CSSProperties = { padding: '4px 10px', background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 4, cursor: 'pointer', fontSize: 12 }
const errorBox:   React.CSSProperties = { background: '#fef2f2', color: '#b91c1c', padding: '8px 12px', borderRadius: 6, marginBottom: 12, fontSize: 13 }
