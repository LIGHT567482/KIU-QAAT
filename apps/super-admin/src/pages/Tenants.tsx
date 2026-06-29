import { useEffect, useRef, useState } from 'react'
import { api, type Tenant, type TenantUser } from '../lib/api'
import { PLATFORM_TENANT_ID } from '../auth'

const EMPTY_FORM = {
  name: '', domain: '', institution_id: '', attendance_threshold: 75,
  logo_url: '',
  brand_color: '#1a73e8', sidebar_color: '#1e293b', background_color: '#f1f5f9', footer_color: '#0f172a',
  motto: '', slogan: '', address: '',
}

// Five curated brand colours for an attractive default palette.
const PRESET_COLORS = ['#1a73e8', '#16a34a', '#7c3aed', '#f59e0b', '#e11d48']
const MAX_LOGO_BYTES = 700 * 1024

export default function Tenants() {
  const [tenants, setTenants] = useState<Tenant[]>([])
  const [status, setStatus] = useState<'loading' | 'ok' | 'error'>('loading')
  const [creating, setCreating] = useState(false)
  const [editing, setEditing] = useState<Tenant | null>(null)
  const [form, setForm] = useState({ ...EMPTY_FORM })
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [usersTenant, setUsersTenant] = useState<Tenant | null>(null)

  function load() {
    setStatus('loading')
    api.get<Tenant[]>('/api/v1/admin/tenants')
      .then(t => { setTenants(t); setStatus('ok') })
      .catch(() => setStatus('error'))
  }
  useEffect(load, [])

  async function handleCreate() {
    setSaving(true); setError(null)
    try {
      await api.post('/api/v1/admin/tenants', form)
      setCreating(false); setForm({ ...EMPTY_FORM }); load()
    } catch (e) { setError(e instanceof Error ? e.message : 'Failed') }
    finally { setSaving(false) }
  }

  async function handleSaveBranding() {
    if (!editing) return
    setSaving(true); setError(null)
    try {
      await api.patch(`/api/v1/admin/tenants/${editing.tenant_id}/branding`, {
        institution_id: form.institution_id,
        logo_url: form.logo_url, brand_color: form.brand_color,
        sidebar_color: form.sidebar_color, background_color: form.background_color, footer_color: form.footer_color,
        motto: form.motto, slogan: form.slogan, address: form.address,
      })
      setEditing(null); load()
    } catch (e) { setError(e instanceof Error ? e.message : 'Failed') }
    finally { setSaving(false) }
  }

  async function toggleStatus(t: Tenant) {
    await api.patch(`/api/v1/admin/tenants/${t.tenant_id}/status`, { is_active: !t.is_active })
    load()
  }

  async function deleteTenant(t: Tenant) {
    if (!confirm(`Permanently delete "${t.name}" and ALL its data (users, courses, students, sessions, attendance)?\n\nThis cannot be undone. Type-check: ${t.domain}`)) return
    try { await api.delete(`/api/v1/admin/tenants/${t.tenant_id}`); load() }
    catch (e) { setError(e instanceof Error ? e.message : 'Delete failed') }
  }

  function startEdit(t: Tenant) {
    setCreating(false)
    setEditing(t)
    setForm({
      ...EMPTY_FORM,
      name: t.name, domain: t.domain, institution_id: t.institution_id || '', attendance_threshold: t.attendance_threshold,
      logo_url: t.logo_url, brand_color: t.brand_color || '#1a73e8',
      sidebar_color: t.sidebar_color || '#1e293b',
      background_color: t.background_color || '#f1f5f9',
      footer_color: t.footer_color || '#0f172a',
      motto: t.motto, slogan: t.slogan, address: t.address,
    })
  }

  return (
    <div style={{ padding: 24, fontFamily: 'system-ui', color: 'var(--text)' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 20 }}>
        <h2 style={{ margin: 0 }}>Tenants</h2>
        <button onClick={() => { setCreating(c => !c); setEditing(null); setForm({ ...EMPTY_FORM }); setError(null) }} style={btnPrimary}>
          {creating ? 'Cancel' : '+ Register Institution'}
        </button>
      </div>

      {(creating || editing) && (
        <div style={card}>
          <h3 style={{ margin: '0 0 16px' }}>
            {editing ? `Edit branding — ${editing.name}` : 'Register Institution'}
          </h3>
          {error && <div style={errorBox}>{error}</div>}
          {!editing && (
            <div style={grid}>
              <Field label="Institution name" value={form.name} onChange={v => setForm(f => ({ ...f, name: v }))} />
              <Field label="Domain (all user emails use this)" value={form.domain} placeholder="university.edu" onChange={v => setForm(f => ({ ...f, domain: v }))} />
              <Field label="Institution ID (admin sign-in code)" value={form.institution_id} placeholder="e.g. KIU-2024" onChange={v => setForm(f => ({ ...f, institution_id: v }))} />
              <Field label="Default attendance threshold (%)" type="number" value={String(form.attendance_threshold)} onChange={v => setForm(f => ({ ...f, attendance_threshold: Number(v) }))} />
            </div>
          )}
          {editing && (
            <div style={{ ...grid, marginBottom: 4 }}>
              <Field label="Institution ID (admin sign-in code)" value={form.institution_id} placeholder="e.g. KIU-2024" onChange={v => setForm(f => ({ ...f, institution_id: v }))} />
            </div>
          )}

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 20, marginTop: 16 }}>
            <LogoPicker value={form.logo_url} onChange={v => setForm(f => ({ ...f, logo_url: v }))} onError={setError} />
            <PalettePreview form={form} />
          </div>

          <div style={{ marginTop: 16, display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
            <ColorPicker label="Accent (buttons & links)" value={form.brand_color} onChange={v => setForm(f => ({ ...f, brand_color: v }))} />
            <ColorPicker label="Sidebar / header" value={form.sidebar_color} onChange={v => setForm(f => ({ ...f, sidebar_color: v }))} />
            <ColorPicker label="Background" value={form.background_color} onChange={v => setForm(f => ({ ...f, background_color: v }))} />
            <ColorPicker label="Footer" value={form.footer_color} onChange={v => setForm(f => ({ ...f, footer_color: v }))} />
          </div>

          <div style={{ ...grid, marginTop: 16 }}>
            <Field label="Motto" value={form.motto} onChange={v => setForm(f => ({ ...f, motto: v }))} />
            <Field label="Slogan" value={form.slogan} onChange={v => setForm(f => ({ ...f, slogan: v }))} />
            <Field label="Address" value={form.address} onChange={v => setForm(f => ({ ...f, address: v }))} />
          </div>
          <button onClick={editing ? handleSaveBranding : handleCreate} disabled={saving} style={{ ...btnPrimary, marginTop: 16 }}>
            {saving ? 'Saving…' : editing ? 'Save Branding' : 'Register Institution'}
          </button>
        </div>
      )}

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}
      {status === 'error' && <p style={{ color: '#ef4444' }}>Failed to load tenants.</p>}

      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
        <thead>
          <tr style={{ background: 'var(--surface-2)' }}>
            {['', 'Institution', 'Domain', 'Institution ID', 'Threshold', 'Status', ''].map((h, i) => (
              <th key={i} style={th}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {tenants.map(t => (
            <tr key={t.tenant_id} style={{ borderBottom: '1px solid var(--border)' }}>
              <td style={td}>
                <div style={{
                  height: 28, width: 28, borderRadius: 6, background: t.brand_color || 'var(--brand)',
                  color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', fontWeight: 700, fontSize: 13, overflow: 'hidden',
                }}>
                  {t.logo_url
                    ? <img src={t.logo_url} alt="" style={{ height: 28, width: 28, objectFit: 'contain' }} />
                    : t.name.slice(0, 1)}
                </div>
              </td>
              <td style={{ ...td, fontWeight: 600 }}>{t.name}</td>
              <td style={{ ...td, color: 'var(--muted)' }}>{t.domain}</td>
              <td style={td}>{t.institution_id
                ? <code style={{ fontSize: 12 }}>{t.institution_id}</code>
                : <span style={{ color: '#b45309', fontSize: 12 }}>⚠ none — set via Branding</span>}</td>
              <td style={td}>{t.attendance_threshold}%</td>
              <td style={td}>
                <span style={{ background: t.is_active ? '#dcfce7' : '#fee2e2', color: t.is_active ? '#166534' : '#b91c1c', padding: '2px 10px', borderRadius: 999, fontSize: 12, fontWeight: 600 }}>
                  {t.is_active ? 'Active' : 'Suspended'}
                </span>
              </td>
              <td style={td}>
                <div style={{ display: 'flex', gap: 4 }}>
                  <button onClick={() => { setUsersTenant(t); setEditing(null); setCreating(false) }} style={{ ...btnSmall, fontWeight: 700 }}>Users</button>
                  <button onClick={() => startEdit(t)} style={btnSmall}>Branding</button>
                  {t.tenant_id !== PLATFORM_TENANT_ID && <>
                    <button onClick={() => toggleStatus(t)} style={{ ...btnSmall, color: t.is_active ? '#b91c1c' : '#166534' }}>
                      {t.is_active ? 'Suspend' : 'Activate'}
                    </button>
                    <button onClick={() => deleteTenant(t)} style={{ ...btnSmall, color: '#b91c1c', borderColor: '#fecaca', background: '#fef2f2' }}>Delete</button>
                  </>}
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {usersTenant && <UsersPanel tenant={usersTenant} onClose={() => setUsersTenant(null)} />}
    </div>
  )
}

// ─── Users panel (list + create the tenant's admin / staff) ───────────────────
function UsersPanel({ tenant, onClose }: { tenant: Tenant; onClose: () => void }) {
  const [users, setUsers] = useState<TenantUser[]>([])
  const [status, setStatus] = useState<'loading' | 'ok' | 'error'>('loading')
  // The super-admin only ever creates the tenant ADMIN; the admin adds everyone else.
  const [form, setForm] = useState({ local: '', full_name: '', password: '' })
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [ok, setOk] = useState<string | null>(null)

  function load() {
    setStatus('loading')
    api.get<TenantUser[]>(`/api/v1/admin/tenants/${tenant.tenant_id}/users`)
      .then(u => { setUsers(u); setStatus('ok') })
      .catch(() => setStatus('error'))
  }
  useEffect(load, [tenant.tenant_id])

  async function create() {
    setSaving(true); setError(null); setOk(null)
    try {
      if (form.password.length < 8) throw new Error('Password must be at least 8 characters.')
      if (!form.local.trim()) throw new Error('Email username is required.')
      const email = `${form.local.trim().toLowerCase()}@${tenant.domain}`
      await api.post(`/api/v1/admin/tenants/${tenant.tenant_id}/users`, {
        email, full_name: form.full_name, password: form.password, role: 'ADMIN',
      })
      setOk(`Admin ${email} created.`)
      setForm({ local: '', full_name: '', password: '' })
      load()
    } catch (e) { setError(e instanceof Error ? e.message : 'Failed') }
    finally { setSaving(false) }
  }

  async function removeAdmin(u: TenantUser) {
    if (!confirm(`Delete admin ${u.email}? This cannot be undone.`)) return
    try { await api.delete(`/api/v1/admin/users/${u.user_id}`); load() }
    catch (e) { setError(e instanceof Error ? e.message : 'Delete failed') }
  }

  const admins = users.filter(u => u.role === 'ADMIN')

  return (
    <div onClick={onClose} style={{
      position: 'fixed', inset: 0, background: 'rgba(0,0,0,.45)', display: 'flex',
      alignItems: 'flex-start', justifyContent: 'center', padding: '40px 16px', zIndex: 50, overflowY: 'auto',
    }}>
      <div onClick={e => e.stopPropagation()} style={{
        background: 'var(--surface)', color: 'var(--text)', borderRadius: 12, padding: 24,
        width: 560, maxWidth: '100%', border: '1px solid var(--border)', boxShadow: 'var(--shadow)',
      }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 4 }}>
          <h3 style={{ margin: 0 }}>Tenant Admin — {tenant.name}</h3>
          <button onClick={onClose} style={btnSmall}>Close</button>
        </div>
        <p style={{ color: 'var(--muted)', fontSize: 13, marginTop: 4, marginBottom: 6 }}>
          You create only the institution ADMIN here; the admin then adds coordinators, lecturers, students and other staff.
        </p>
        {!tenant.institution_id && (
          <div style={{ ...errorBox, background: '#fef9c3', color: '#854d0e' }}>
            Set an Institution ID for this tenant (via Branding) — the admin needs it to sign in.
          </div>
        )}

        {/* Create-admin form */}
        <div style={{ ...card, marginBottom: 18 }}>
          <div style={grid}>
            <Field label="Full name" value={form.full_name} onChange={v => setForm(f => ({ ...f, full_name: v }))} />
            <label style={{ display: 'block' }}>
              <div style={labelStyle}>Admin email</div>
              <div style={{ display: 'flex', alignItems: 'center' }}>
                <input value={form.local} placeholder="admin" onChange={e => setForm(f => ({ ...f, local: e.target.value }))}
                  style={{ flex: 1, padding: '8px 10px', borderRadius: '6px 0 0 6px', border: '1px solid var(--border)', borderRight: 0, fontSize: 14, boxSizing: 'border-box' }} />
                <span style={{ padding: '8px 10px', borderRadius: '0 6px 6px 0', border: '1px solid var(--border)', background: 'var(--surface-2)', color: 'var(--muted)', fontSize: 13, whiteSpace: 'nowrap' }}>@{tenant.domain}</span>
              </div>
            </label>
            <Field label="Password (min 8)" type="password" value={form.password} onChange={v => setForm(f => ({ ...f, password: v }))} />
          </div>
          {error && <div style={errorBox}>{error}</div>}
          {ok && <div style={{ ...errorBox, background: '#dcfce7', color: '#166534' }}>{ok}</div>}
          <button onClick={create} disabled={saving || !form.local || !form.full_name || !form.password} style={{ ...btnPrimary, marginTop: 12 }}>
            {saving ? 'Creating…' : 'Create Tenant Admin'}
          </button>
        </div>

        {/* Existing users */}
        {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading users…</p>}
        {status === 'error' && <p style={{ color: '#ef4444' }}>Couldn’t load users.</p>}
        {status === 'ok' && (
          admins.length === 0
            ? <p style={{ color: 'var(--muted)', fontSize: 13 }}>No admin yet for this institution.</p>
            : <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
                <thead><tr style={{ background: 'var(--surface-2)' }}>
                  {['Admin', 'Email', 'Status', ''].map(h => <th key={h} style={th}>{h}</th>)}
                </tr></thead>
                <tbody>
                  {admins.map(u => (
                    <tr key={u.user_id} style={{ borderBottom: '1px solid var(--border)' }}>
                      <td style={td}>{u.full_name}</td>
                      <td style={{ ...td, color: 'var(--muted)' }}>{u.email}</td>
                      <td style={td}>{u.is_active ? 'Active' : 'Disabled'}</td>
                      <td style={td}><button onClick={() => removeAdmin(u)} style={{ ...btnSmall, color: '#b91c1c', borderColor: '#fecaca' }}>Delete</button></td>
                    </tr>
                  ))}
                </tbody>
              </table>
        )}
      </div>
    </div>
  )
}

// ─── Logo upload (file → data URL) ────────────────────────────────────────────
function LogoPicker({ value, onChange, onError, label = 'Logo', maxBytes = MAX_LOGO_BYTES, hint }: {
  value: string; onChange: (v: string) => void; onError: (e: string | null) => void; label?: string; maxBytes?: number; hint?: string
}) {
  const inputRef = useRef<HTMLInputElement>(null)
  const maxKB = Math.round(maxBytes / 1024)

  function onFile(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    onError(null)
    // SVG can carry scripts — only accept raster images (server enforces this too).
    if (!/^image\/(png|jpeg|jpg|webp|gif)$/.test(file.type)) {
      onError('Please choose a PNG, JPEG, WebP or GIF image (SVG is not allowed).'); return
    }
    if (file.size > maxBytes) { onError(`${label} too large — please choose an image under ~${maxKB} KB.`); return }
    const reader = new FileReader()
    reader.onload = () => onChange(String(reader.result))
    reader.onerror = () => onError('Could not read that file.')
    reader.readAsDataURL(file)
  }

  return (
    <div>
      <div style={labelStyle}>{label}</div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <div style={{
          height: 56, width: 56, borderRadius: 10, border: '1px dashed var(--border)',
          background: value ? `center/cover no-repeat url("${value}")` : 'var(--surface-2)',
          display: 'flex', alignItems: 'center', justifyContent: 'center', overflow: 'hidden',
        }}>
          {!value && <span style={{ fontSize: 11, color: 'var(--muted)' }}>none</span>}
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <input ref={inputRef} type="file" accept="image/png,image/jpeg,image/webp,image/gif" onChange={onFile} style={{ display: 'none' }} />
          <button type="button" onClick={() => inputRef.current?.click()} style={btnSmall}>Choose file…</button>
          {value && <button type="button" onClick={() => onChange('')} style={{ ...btnSmall, color: '#b91c1c' }}>Remove</button>}
        </div>
      </div>
      <div style={{ fontSize: 11, color: 'var(--muted)', marginTop: 6 }}>{hint ?? `PNG/JPG/WebP/GIF from your device · max ~${maxKB} KB`}</div>
    </div>
  )
}

// ─── Brand colour (presets + picker) ──────────────────────────────────────────
function ColorPicker({ label, value, onChange }: { label: string; value: string; onChange: (v: string) => void }) {
  return (
    <div>
      <div style={labelStyle}>{label}</div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
        {PRESET_COLORS.map(c => (
          <button key={c} type="button" onClick={() => onChange(c)} title={c}
            style={{
              height: 26, width: 26, borderRadius: '50%', background: c, cursor: 'pointer',
              border: value.toLowerCase() === c.toLowerCase() ? '3px solid var(--text)' : '2px solid var(--border)',
            }} />
        ))}
        <input type="color" value={value} onChange={e => onChange(e.target.value)}
          style={{ height: 28, width: 38, padding: 0, border: '1px solid var(--border)', borderRadius: 6, background: 'none', cursor: 'pointer' }} />
        <span style={{ fontSize: 12, color: 'var(--muted)', fontFamily: 'monospace' }}>{value}</span>
      </div>
    </div>
  )
}

// Live mini-mockup so the super-admin sees how the four region colours combine.
function PalettePreview({ form }: { form: typeof EMPTY_FORM }) {
  return (
    <div>
      <div style={labelStyle}>Live preview</div>
      <div style={{ border: '1px solid var(--border)', borderRadius: 10, overflow: 'hidden', height: 92, display: 'flex' }}>
        <div style={{ width: 60, background: form.sidebar_color, display: 'flex', flexDirection: 'column', gap: 5, padding: 8 }}>
          {[0, 1, 2].map(i => <div key={i} style={{ height: 6, borderRadius: 3, background: 'rgba(255,255,255,.6)' }} />)}
        </div>
        <div style={{ flex: 1, backgroundColor: form.background_color, display: 'flex', flexDirection: 'column' }}>
          <div style={{ flex: 1, padding: 8 }}>
            <span style={{ display: 'inline-block', background: form.brand_color, color: '#fff', borderRadius: 5, fontSize: 10, padding: '4px 8px', fontWeight: 700 }}>Action</span>
          </div>
          <div style={{ background: form.footer_color, height: 16 }} />
        </div>
      </div>
    </div>
  )
}

function Field({ label, value, onChange, type = 'text', placeholder }: {
  label: string; value: string; onChange: (v: string) => void; type?: string; placeholder?: string
}) {
  return (
    <label style={{ display: 'block' }}>
      <div style={labelStyle}>{label}</div>
      <input type={type} value={value} placeholder={placeholder} onChange={e => onChange(e.target.value)}
        style={{ width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid var(--border)', fontSize: 14, boxSizing: 'border-box' }} />
    </label>
  )
}

const card: React.CSSProperties = { background: 'var(--surface)', border: '1px solid var(--border)', borderRadius: 10, padding: 20, marginBottom: 24 }
const grid: React.CSSProperties = { display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }
const th: React.CSSProperties = { padding: '8px 12px', textAlign: 'left', borderBottom: '1px solid var(--border)', whiteSpace: 'nowrap', color: 'var(--muted)' }
const td: React.CSSProperties = { padding: '10px 12px' }
const btnPrimary: React.CSSProperties = { padding: '8px 16px', background: 'var(--brand)', color: 'var(--brand-contrast)', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const btnSmall: React.CSSProperties = { padding: '4px 10px', background: 'var(--surface-2)', border: '1px solid var(--border)', borderRadius: 4, cursor: 'pointer', fontSize: 12, color: 'var(--text)' }
const errorBox: React.CSSProperties = { background: '#fee2e2', color: '#b91c1c', padding: '8px 12px', borderRadius: 6, marginBottom: 12, fontSize: 13 }
const labelStyle: React.CSSProperties = { fontSize: 13, fontWeight: 600, marginBottom: 4, color: 'var(--text)' }
