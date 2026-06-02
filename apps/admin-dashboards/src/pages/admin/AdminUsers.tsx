import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

const ROLES = ['COORDINATOR', 'QA_OFFICER', 'DQA_DIRECTOR', 'VC'] as const

interface User {
  user_id: string; email: string; role: string
  full_name: string; is_active: boolean; last_login_at: string | null; created_at: string
}

export default function AdminUsers() {
  const { tenantId } = useParams<{ tenantId: string }>()
  const { status, data, refetch } = useQuery<User[]>(
    () => api.get(`/api/v1/admin/tenants/${tenantId}/users`),
    [tenantId],
  )

  const [creating, setCreating] = useState(false)
  const [form, setForm] = useState({ email: '', password: '', role: 'COORDINATOR', full_name: '' })
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleCreate() {
    setSaving(true); setError(null)
    try {
      await api.post(`/api/v1/admin/tenants/${tenantId}/users`, form)
      setCreating(false)
      setForm({ email: '', password: '', role: 'COORDINATOR', full_name: '' })
      refetch()
    } catch (e) { setError(e instanceof Error ? e.message : 'Failed') }
    finally { setSaving(false) }
  }

  async function toggleStatus(u: User) {
    await api.post(`/api/v1/admin/users/${u.user_id}/status`, { is_active: !u.is_active })
    refetch()
  }

  const users = status === 'ok' ? data : []

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 20 }}>
        <div>
          <a href="/admin/tenants" style={{ color: '#64748b', fontSize: 13, textDecoration: 'none' }}>← Tenants</a>
          <h2 style={{ margin: '4px 0 0' }}>Users</h2>
        </div>
        <button onClick={() => setCreating(c => !c)} style={btn}>
          {creating ? 'Cancel' : '+ Add User'}
        </button>
      </div>

      {creating && (
        <div style={{ background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 10, padding: 20, marginBottom: 24 }}>
          <h3 style={{ margin: '0 0 16px' }}>Create User</h3>
          {error && <div style={errorBox}>{error}</div>}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <Field label="Full name"    value={form.full_name} onChange={v => setForm(f => ({ ...f, full_name: v }))} />
            <Field label="Email"        value={form.email}     onChange={v => setForm(f => ({ ...f, email: v }))}     type="email" />
            <Field label="Password"     value={form.password}  onChange={v => setForm(f => ({ ...f, password: v }))}  type="password" />
            <label>
              <div style={labelStyle}>Role</div>
              <select value={form.role} onChange={e => setForm(f => ({ ...f, role: e.target.value }))} style={selectStyle}>
                {ROLES.map(r => <option key={r} value={r}>{r.replace(/_/g, ' ')}</option>)}
              </select>
            </label>
          </div>
          <button onClick={handleCreate} disabled={saving} style={{ ...btn, marginTop: 16 }}>
            {saving ? 'Creating…' : 'Create User'}
          </button>
        </div>
      )}

      {status === 'loading' && <p style={{ color: '#94a3b8' }}>Loading…</p>}

      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
        <thead>
          <tr style={{ background: '#f8fafc' }}>
            {['Name', 'Email', 'Role', 'Last Login', 'Status', ''].map(h => (
              <th key={h} style={{ padding: '8px 12px', textAlign: 'left', borderBottom: '1px solid #e2e8f0' }}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {(users ?? []).map(u => (
            <tr key={u.user_id} style={{ borderBottom: '1px solid #f1f5f9', opacity: u.is_active ? 1 : 0.5 }}>
              <td style={{ padding: '10px 12px', fontWeight: 600 }}>{u.full_name}</td>
              <td style={{ padding: '10px 12px', color: '#64748b' }}>{u.email}</td>
              <td style={{ padding: '10px 12px' }}>
                <span style={{ background: roleColor(u.role).bg, color: roleColor(u.role).text, padding: '2px 8px', borderRadius: 999, fontSize: 11, fontWeight: 600 }}>
                  {u.role.replace(/_/g, ' ')}
                </span>
              </td>
              <td style={{ padding: '10px 12px', color: '#94a3b8', fontSize: 12 }}>
                {u.last_login_at ? new Date(u.last_login_at).toLocaleDateString() : 'Never'}
              </td>
              <td style={{ padding: '10px 12px' }}>
                <span style={{ color: u.is_active ? '#166534' : '#b91c1c', fontSize: 12, fontWeight: 600 }}>
                  {u.is_active ? 'Active' : 'Inactive'}
                </span>
              </td>
              <td style={{ padding: '10px 12px' }}>
                <button onClick={() => toggleStatus(u)} style={{ padding: '3px 8px', border: '1px solid #e2e8f0', borderRadius: 4, cursor: 'pointer', fontSize: 11, background: '#fff' }}>
                  {u.is_active ? 'Deactivate' : 'Activate'}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function roleColor(role: string) {
  const map: Record<string, { bg: string; text: string }> = {
    VC:          { bg: '#fef3c7', text: '#92400e' },
    DQA_DIRECTOR:{ bg: '#e0e7ff', text: '#3730a3' },
    QA_OFFICER:  { bg: '#f0fdf4', text: '#166534' },
    COORDINATOR: { bg: '#f0f9ff', text: '#0369a1' },
    ADMIN:       { bg: '#fce7f3', text: '#9d174d' },
  }
  return map[role] ?? { bg: '#f1f5f9', text: '#475569' }
}

function Field({ label, value, onChange, type = 'text' }: { label: string; value: string; onChange: (v: string) => void; type?: string }) {
  return (
    <label>
      <div style={labelStyle}>{label}</div>
      <input type={type} value={value} onChange={e => onChange(e.target.value)}
        style={{ width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }} />
    </label>
  )
}

const btn:         React.CSSProperties = { padding: '8px 16px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const labelStyle:  React.CSSProperties = { fontSize: 13, fontWeight: 600, marginBottom: 4, color: '#475569' }
const selectStyle: React.CSSProperties = { width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14 }
const errorBox:    React.CSSProperties = { background: '#fef2f2', color: '#b91c1c', padding: '8px 12px', borderRadius: 6, marginBottom: 12, fontSize: 13 }
