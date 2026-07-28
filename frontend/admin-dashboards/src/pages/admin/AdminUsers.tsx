import { useState, useEffect } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../../lib/api'
import PasswordInput from '../../components/PasswordInput'
import { useQuery } from '../../lib/useApi'

// The Users page manages the oversight roles only — coordinators have their own
// directory (Coordinators) and students their own page.
const ROLES = ['ADMIN', 'VC', 'DVC', 'QA_OFFICER', 'DQA_DIRECTOR'] as const
const MANAGED_ROLES = new Set<string>(ROLES)

interface User {
  user_id: string; email: string; role: string
  full_name: string; is_active: boolean; last_login_at: string | null; created_at: string
  coordinator_code?: string
}

export default function AdminUsers() {
  // Gate the whole page behind the tenant's Users passcode before any data loads.
  const [unlocked, setUnlocked] = useState(false)
  if (!unlocked) return <PasscodeGate onUnlock={() => setUnlocked(true)} />
  return <UsersInner />
}

function UsersInner() {
  const { tenantId } = useParams<{ tenantId: string }>()
  const { status, data, refetch } = useQuery<User[]>(
    () => api.get(`/api/v1/admin/tenants/${tenantId}/users`),
    [tenantId],
  )
  // Every user email must use the institution's domain; fetch it for the suffix.
  const brand = useQuery<{ domain: string }>(() => api.get('/api/v1/branding'))
  const domain = brand.status === 'ok' ? (brand.data?.domain ?? '') : ''

  const titlesQ = useQuery<{ titles: string[] }>(() => api.get('/api/v1/admin/settings/titles'))
  const titles = (titlesQ.status === 'ok' ? titlesQ.data?.titles : null) ?? []
  const GENDERS = ['', 'Male', 'Female', 'Other']

  const [creating, setCreating] = useState(false)
  const [form, setForm] = useState({ local: '', password: '', role: '', full_name: '', phone: '', whatsapp: '', registration_number: '', title: '', gender: '' })
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [issuedCode, setIssuedCode] = useState<string | null>(null)

  async function handleCreate() {
    setSaving(true); setError(null)
    try {
      if (!form.local.trim()) throw new Error('Email username is required.')
      const email = `${form.local.trim().toLowerCase()}@${domain}`
      const res = await api.post(`/api/v1/admin/tenants/${tenantId}/users`, {
        email, password: form.password, role: form.role, full_name: form.full_name,
        phone: form.phone, whatsapp: form.whatsapp, registration_number: form.registration_number,
        title: form.title, gender: form.gender,
      }) as { coordinator_code?: string }
      setCreating(false)
      setForm({ local: '', password: '', role: '', full_name: '', phone: '', whatsapp: '', registration_number: '', title: '', gender: '' })
      setIssuedCode(res?.coordinator_code ?? null)
      refetch()
    } catch (e) { setError(e instanceof Error ? e.message : 'Failed') }
    finally { setSaving(false) }
  }

  async function toggleStatus(u: User) {
    await api.patch(`/api/v1/admin/users/${u.user_id}/status`, { is_active: !u.is_active })
    refetch()
  }

  async function deleteUser(u: User) {
    if (!confirm(`Permanently delete ${u.full_name} (${u.email})?\nThis cannot be undone.`)) return
    try { await api.delete(`/api/v1/admin/users/${u.user_id}`); refetch() }
    catch (e) { alert(e instanceof Error ? e.message : 'Delete failed') }
  }

  // Only the oversight roles belong on this page (coordinators/students excluded).
  const users = (status === 'ok' ? (data ?? []) : []).filter(u => MANAGED_ROLES.has(u.role))

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 20 }}>
        <div>
          <a href="/admin/tenants" style={{ color: 'var(--muted)', fontSize: 13, textDecoration: 'none' }}>← Home</a>
          <h2 style={{ margin: '4px 0 0' }}>Administration</h2>
          <p style={{ color: 'var(--muted)', margin: '4px 0 0', fontSize: 13 }}>Staff accounts &amp; the institution's academic period.</p>
        </div>
        <button onClick={() => setCreating(c => !c)} style={btn}>
          {creating ? 'Cancel' : '+ Add User'}
        </button>
      </div>

      <AcademicPeriodCard tenantId={tenantId!} />


      {creating && (
        <div style={{ background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 10, padding: 20, marginBottom: 24 }}>
          <h3 style={{ margin: '0 0 16px' }}>Create User</h3>
          {error && <div style={errorBox}>{error}</div>}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <Field label="Full name"    value={form.full_name} onChange={v => setForm(f => ({ ...f, full_name: v }))} />
            <label>
              <div style={labelStyle}>Email</div>
              <div style={{ display: 'flex', alignItems: 'center' }}>
                <input value={form.local} placeholder="username" onChange={e => setForm(f => ({ ...f, local: e.target.value }))}
                  style={{ flex: 1, padding: '8px 10px', borderRadius: '6px 0 0 6px', border: '1px solid #e2e8f0', borderRight: 0, fontSize: 14, boxSizing: 'border-box' }} />
                <span style={{ padding: '8px 10px', borderRadius: '0 6px 6px 0', border: '1px solid #e2e8f0', background: '#f1f5f9', color: 'var(--muted)', fontSize: 13, whiteSpace: 'nowrap' }}>@{domain || '…'}</span>
              </div>
            </label>
            <Field label="Password"     value={form.password}  onChange={v => setForm(f => ({ ...f, password: v }))}  type="password" />
            <label>
              <div style={labelStyle}>Role</div>
              <select value={form.role} onChange={e => setForm(f => ({ ...f, role: e.target.value }))} style={{ ...selectStyle, color: form.role ? '#1e293b' : 'var(--muted)' }}>
                <option value="">— Select role —</option>
                {ROLES.map(r => <option key={r} value={r} style={{ color: '#1e293b' }}>{r.replace(/_/g, ' ')}</option>)}
              </select>
            </label>
            <label>
              <div style={labelStyle}>Title (optional)</div>
              <select value={form.title} onChange={e => setForm(f => ({ ...f, title: e.target.value }))} style={selectStyle}>
                <option value="">— none —</option>
                {titles.map(t => <option key={t} value={t}>{t}</option>)}
              </select>
            </label>
            <label>
              <div style={labelStyle}>Gender (optional)</div>
              <select value={form.gender} onChange={e => setForm(f => ({ ...f, gender: e.target.value }))} style={selectStyle}>
                {GENDERS.map(g => <option key={g} value={g}>{g || '—'}</option>)}
              </select>
            </label>
            <Field label="Phone (optional)"        value={form.phone}    onChange={v => setForm(f => ({ ...f, phone: v }))} />
            <Field label="WhatsApp (optional)"      value={form.whatsapp} onChange={v => setForm(f => ({ ...f, whatsapp: v }))} />
            <Field label="Registration No. (optional)" value={form.registration_number} onChange={v => setForm(f => ({ ...f, registration_number: v }))} />
          </div>
          <button onClick={handleCreate} disabled={saving || !form.role || !form.full_name || !form.local || !form.password} style={{ ...btn, marginTop: 16, opacity: (!form.role || !form.full_name || !form.local || !form.password) ? 0.5 : 1 }}>
            {saving ? 'Creating…' : 'Create User'}
          </button>
        </div>
      )}

      {issuedCode && (
        <div style={{ background: '#ecfdf5', border: '1px solid #6ee7b7', borderRadius: 10, padding: '12px 16px', marginBottom: 20, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <span style={{ color: '#065f46', fontSize: 14 }}>
            Coordinator created. Their coordinator ID is{' '}
            <strong style={{ fontFamily: 'ui-monospace, monospace', letterSpacing: 0.5 }}>{issuedCode}</strong>{' '}— give it to them.
          </span>
          <button onClick={() => setIssuedCode(null)} style={{ border: 'none', background: 'transparent', color: '#065f46', cursor: 'pointer', fontSize: 18 }}>×</button>
        </div>
      )}

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}

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
              <td style={{ padding: '10px 12px', fontWeight: 600 }}>
                {u.full_name}
                {u.coordinator_code && (
                  <div style={{ fontFamily: 'ui-monospace, monospace', fontSize: 11, fontWeight: 600, color: '#0369a1' }}>{u.coordinator_code}</div>
                )}
              </td>
              <td style={{ padding: '10px 12px', color: 'var(--muted)' }}>{u.email}</td>
              <td style={{ padding: '10px 12px' }}>
                <span style={{ background: roleColor(u.role).bg, color: roleColor(u.role).text, padding: '2px 8px', borderRadius: 999, fontSize: 11, fontWeight: 600 }}>
                  {u.role.replace(/_/g, ' ')}
                </span>
              </td>
              <td style={{ padding: '10px 12px', color: 'var(--muted)', fontSize: 12 }}>
                {u.last_login_at ? new Date(u.last_login_at).toLocaleDateString() : 'Never'}
              </td>
              <td style={{ padding: '10px 12px' }}>
                <span style={{ color: u.is_active ? '#166534' : '#b91c1c', fontSize: 12, fontWeight: 600 }}>
                  {u.is_active ? 'Active' : 'Inactive'}
                </span>
              </td>
              <td style={{ padding: '10px 12px' }}>
                <div style={{ display: 'flex', gap: 6 }}>
                  <button onClick={() => toggleStatus(u)} style={{ padding: '3px 8px', border: '1px solid #e2e8f0', borderRadius: 4, cursor: 'pointer', fontSize: 11, background: '#fff' }}>
                    {u.is_active ? 'Deactivate' : 'Activate'}
                  </button>
                  <button onClick={() => deleteUser(u)} style={{ padding: '3px 8px', border: '1px solid #fecaca', borderRadius: 4, cursor: 'pointer', fontSize: 11, background: '#fef2f2', color: '#b91c1c' }}>
                    Delete
                  </button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

// IntakeChips — a reusable multi-select for the tenant's configured intakes.
// A semester can end for one intake (August) while another (May) keeps studying, so
// both the advance and the clear are scoped by picking intakes here.
function IntakeChips({ all, selected, onToggle }: { all: string[]; selected: string[]; onToggle: (i: string) => void }) {
  if (all.length === 0) return <div style={{ fontSize: 12, color: '#b45309' }}>No intakes configured — set them on the Students page first.</div>
  return (
    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
      {all.map(i => {
        const on = selected.includes(i)
        return (
          <button key={i} type="button" onClick={() => onToggle(i)}
            style={{ padding: '5px 12px', borderRadius: 999, fontSize: 12, fontWeight: 600, cursor: 'pointer',
              border: on ? '1px solid #1e293b' : '1px solid #cbd5e1', background: on ? '#1e293b' : '#fff', color: on ? '#fff' : '#334155' }}>
            {on ? '✓ ' : ''}{i}
          </button>
        )
      })}
    </div>
  )
}

// Academic period — view + advance + end-of-semester clear. Both are heavy/irreversible
// and gated behind the admin re-entering their password. Because a semester rarely ends
// for the whole institution at once, both are scoped by intake: the admin advances /
// clears the August intake while the May intake continues. Every clear first stores a
// downloadable zip archive of the data under Reports before deleting anything.
function AcademicPeriodCard({ tenantId }: { tenantId: string }) {
  const { status, data, refetch } = useQuery<{ active_academic_year: string; active_semester: number }>(() => api.get('/api/v1/branding'))
  const info = status === 'ok' ? data : undefined

  // Tenant intakes drive both scoped actions.
  const [intakes, setIntakes] = useState<string[]>([])
  useEffect(() => {
    api.get<{ intakes: string[] }>('/api/v1/admin/settings/intakes').then(r => setIntakes(r.intakes ?? [])).catch(() => {})
  }, [])

  // Set the institution's active academic YEAR — the one value all intakes share.
  // There is deliberately no single "active semester": different intakes/cohorts sit in
  // different semesters at the same time (that lives on each cohort + student).
  const [setOpenP, setSetOpenP] = useState(false)
  const [ayInput, setAyInput] = useState('')
  const [setBusyP, setSetBusyP] = useState(false)
  const [setErrP, setSetErrP] = useState<string | null>(null)
  useEffect(() => {
    if (info?.active_academic_year) setAyInput(info.active_academic_year)
  }, [info?.active_academic_year])
  async function savePeriod() {
    if (!/^\d{4}\/\d{4}$/.test(ayInput.trim())) { setSetErrP('Enter the academic year as YYYY/YYYY, e.g. 2025/2026.'); return }
    setSetBusyP(true); setSetErrP(null)
    try {
      await api.patch(`/api/v1/admin/tenants/${tenantId}/academic-period`, { active_academic_year: ayInput.trim() })
      setSetOpenP(false); refetch()
    } catch (e) { setSetErrP(e instanceof Error ? e.message : 'Failed') }
    finally { setSetBusyP(false) }
  }

  const [open, setOpen] = useState(false)
  const [pw, setPw] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [done, setDone] = useState<string | null>(null)
  const [advScope, setAdvScope] = useState<'ALL' | 'INTAKE'>('ALL')
  const [advIntakes, setAdvIntakes] = useState<string[]>([])
  const toggleAdv = (i: string) => setAdvIntakes(p => p.includes(i) ? p.filter(x => x !== i) : [...p, i])

  // Separate, also-destructive action: clear a semester's attendance data (by intake).
  const [clrOpen, setClrOpen] = useState(false)
  const [clrPw, setClrPw] = useState('')
  const [clrBusy, setClrBusy] = useState(false)
  const [clrErr, setClrErr] = useState<string | null>(null)
  const [clrDone, setClrDone] = useState<string | null>(null)
  const [clrIntakes, setClrIntakes] = useState<string[]>([])
  const [clrAY, setClrAY] = useState('')
  const toggleClr = (i: string) => setClrIntakes(p => p.includes(i) ? p.filter(x => x !== i) : [...p, i])

  async function clearData() {
    if (clrIntakes.length === 0) { setClrErr('Pick at least one intake to clear.'); return }
    setClrBusy(true); setClrErr(null)
    try {
      const res = await api.post<{ status: string; attendance_logs_deleted: number; sessions_deleted: number; lecturer_logs_deleted: number; archive_filename?: string }>(
        `/api/v1/admin/tenants/${tenantId}/clear-semester-data`, { password: clrPw, intakes: clrIntakes, academic_year: clrAY })
      if (res.status === 'NO_MATCHING_STUDENTS') {
        setClrDone(`No students found for intake(s) ${clrIntakes.join(', ')}${clrAY ? ` · ${clrAY}` : ''} — nothing was cleared.`)
      } else {
        setClrDone(`Archived → ${res.archive_filename ?? 'zip'} (under Reports → Semester archives), then cleared ${res.attendance_logs_deleted} attendance record(s), ${res.sessions_deleted} emptied session(s), ${res.lecturer_logs_deleted} lecturer log(s) for intake(s) ${clrIntakes.join(', ')}. Students, lecturers, courses, cohorts, timetable and other intakes were kept.`)
      }
      setClrOpen(false); setClrPw(''); setClrIntakes([]); setClrAY('')
    } catch (e) { setClrErr(e instanceof Error ? e.message : 'Failed') }
    finally { setClrBusy(false) }
  }

  const next = info?.active_semester === 2
    ? `${incYear(info?.active_academic_year ?? '')} · Semester 1`
    : `${info?.active_academic_year ?? ''} · Semester 2`

  async function advance() {
    if (advScope === 'INTAKE' && advIntakes.length === 0) { setErr('Pick at least one intake to advance.'); return }
    setBusy(true); setErr(null)
    try {
      const res = await api.post<{ status: string; scope?: string; active_academic_year?: string; active_semester?: number; students_advanced: number; students_graduated: number; cohorts_advanced: number; cohorts_completed: number }>(
        `/api/v1/admin/tenants/${tenantId}/academic-period/advance`, { password: pw, intakes: advScope === 'INTAKE' ? advIntakes : [] })
      if (res.scope === 'INTAKE') {
        setDone(`Advanced intake(s) ${advIntakes.join(', ')}: ${res.students_advanced} student(s) advanced, ${res.students_graduated} graduated; ${res.cohorts_advanced} cohort(s) advanced, ${res.cohorts_completed} completed. Other intakes and the institution's academic year were left unchanged.`)
      } else {
        setDone(`Now ${res.active_academic_year} · Semester ${res.active_semester}. ${res.students_advanced} student(s) advanced, ${res.students_graduated} graduated; ${res.cohorts_advanced} cohort(s) advanced, ${res.cohorts_completed} completed.`)
      }
      setOpen(false); setPw(''); setAdvIntakes([]); setAdvScope('ALL'); refetch()
    } catch (e) { setErr(e instanceof Error ? e.message : 'Failed') }
    finally { setBusy(false) }
  }

  const radio: React.CSSProperties = { display: 'flex', gap: 6, alignItems: 'center', fontSize: 13, color: '#334155', cursor: 'pointer' }

  return (
    <div style={{ background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 10, padding: 16, marginBottom: 24 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 8 }}>
        <div>
          <strong style={{ fontSize: 14 }}>Active Academic Year</strong>
          <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 4 }}>
            {info?.active_academic_year
              ? <>{info.active_academic_year}</>
              : <span style={{ color: '#b45309' }}>⚠ Not set</span>}
          </div>
          <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 4, opacity: .85 }}>
            Semesters run <strong>per intake/cohort</strong> — within one academic year you'll have cohorts in Sem&nbsp;1 and Sem&nbsp;2 at the same time. Each cohort's year &amp; semester is set on the cohort and moved with “Advance”.
          </div>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button onClick={() => { setSetOpenP(o => !o); setSetErrP(null) }} style={{ ...btn, background: '#0f766e' }}>{setOpenP ? 'Cancel' : (info?.active_academic_year ? 'Change year' : 'Set year')}</button>
          <button onClick={() => { setOpen(o => !o); setErr(null); setDone(null) }} style={btn}>{open ? 'Cancel' : 'Advance to next semester'}</button>
        </div>
      </div>
      {setOpenP && (
        <div style={{ marginTop: 12, borderTop: '1px solid #e2e8f0', paddingTop: 12 }}>
          <p style={{ fontSize: 13, color: '#475569', margin: '0 0 10px' }}>
            Set the institution's <strong>active academic year</strong> (all intakes share it). It's rolled onto every active student. This does <strong>not</strong> set a semester and does <strong>not</strong> promote anyone — semesters live per cohort; use “Advance” to move an intake forward.
          </p>
          {setErrP && <div style={{ background: '#fef2f2', color: '#b91c1c', padding: '8px 12px', borderRadius: 6, marginBottom: 10, fontSize: 13 }}>{setErrP}</div>}
          <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
            <input value={ayInput} onChange={e => setAyInput(e.target.value)} placeholder="Academic year (YYYY/YYYY)" autoFocus
              style={{ maxWidth: 200, padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14 }} />
            <button onClick={savePeriod} disabled={setBusyP} style={{ ...btn, background: '#0f766e', opacity: setBusyP ? 0.6 : 1 }}>{setBusyP ? 'Saving…' : 'Save academic year'}</button>
          </div>
        </div>
      )}
      {done && <div style={{ background: '#ecfdf5', color: '#065f46', padding: '8px 12px', borderRadius: 6, marginTop: 12, fontSize: 13 }}>{done}</div>}
      {open && (
        <div style={{ marginTop: 12, borderTop: '1px solid #e2e8f0', paddingTop: 12 }}>
          <div style={{ display: 'flex', gap: 18, marginBottom: 10, flexWrap: 'wrap' }}>
            <label style={radio}><input type="radio" checked={advScope === 'ALL'} onChange={() => setAdvScope('ALL')} />Whole institution</label>
            <label style={radio}><input type="radio" checked={advScope === 'INTAKE'} onChange={() => setAdvScope('INTAKE')} />Specific intake(s)</label>
          </div>
          {advScope === 'ALL' ? (
            <p style={{ fontSize: 13, color: '#475569', margin: '0 0 10px' }}>
              This promotes <strong>every student and cohort</strong> one semester → <strong>{next}</strong>
              {' '}(Sem 2 → next year) and moves the institution's active period. Final-year students graduate.
            </p>
          ) : (
            <>
              <p style={{ fontSize: 13, color: '#475569', margin: '0 0 8px' }}>
                Advances <strong>only the selected intakes'</strong> students <em>and their cohorts</em> one step, driven by each student's semester (yr1/sem2 → yr2/sem1; final year graduates). Other intakes and the institution's academic year are left unchanged — use this when one intake finishes while others continue.
              </p>
              <div style={{ marginBottom: 10 }}><IntakeChips all={intakes} selected={advIntakes} onToggle={toggleAdv} /></div>
            </>
          )}
          {err && <div style={{ background: '#fef2f2', color: '#b91c1c', padding: '8px 12px', borderRadius: 6, marginBottom: 10, fontSize: 13 }}>{err}</div>}
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <PasswordInput value={pw} onChange={e => setPw(e.target.value)} placeholder="Your admin password" autoFocus
              wrapperStyle={{ flex: 1, maxWidth: 260 }}
              style={{ padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14 }} />
            <button onClick={advance} disabled={busy || !pw} style={{ ...btn, background: '#b45309', opacity: !pw ? 0.5 : 1 }}>{busy ? 'Advancing…' : 'Confirm advance'}</button>
          </div>
        </div>
      )}

      {/* End-of-semester data clear — intake-scoped, archived first. */}
      <div style={{ marginTop: 14, borderTop: '1px solid #e2e8f0', paddingTop: 14 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 8 }}>
          <div>
            <strong style={{ fontSize: 14 }}>Clear a semester's attendance</strong>
            <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 4 }}>Zips the data to Reports first, then wipes attendance + emptied sessions for the chosen intake(s). Keeps students, lecturers, courses, cohorts, timetable and other intakes.</div>
          </div>
          <button onClick={() => { setClrOpen(o => !o); setClrErr(null); setClrDone(null) }} style={{ ...btn, background: '#b91c1c' }}>{clrOpen ? 'Cancel' : 'Clear data'}</button>
        </div>
        {clrDone && <div style={{ background: '#ecfdf5', color: '#065f46', padding: '8px 12px', borderRadius: 6, marginTop: 12, fontSize: 13 }}>{clrDone}</div>}
        {clrOpen && (
          <div style={{ marginTop: 12 }}>
            <p style={{ fontSize: 13, color: '#475569', margin: '0 0 8px' }}>
              Pick the intake(s) whose semester has ended. Their data is <strong>compressed to a zip and stored under Reports</strong>, then <strong>permanently deleted</strong>. You cannot clear the whole institution in one action.
            </p>
            <div style={{ marginBottom: 10 }}><IntakeChips all={intakes} selected={clrIntakes} onToggle={toggleClr} /></div>
            <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 10 }}>
              <input value={clrAY} onChange={e => setClrAY(e.target.value)} placeholder="Academic year (optional, e.g. 2024/2025)"
                style={{ flex: 1, maxWidth: 320, padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 13 }} />
            </div>
            {clrErr && <div style={{ background: '#fef2f2', color: '#b91c1c', padding: '8px 12px', borderRadius: 6, marginBottom: 10, fontSize: 13 }}>{clrErr}</div>}
            <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
              <PasswordInput value={clrPw} onChange={e => setClrPw(e.target.value)} placeholder="Your admin password"
                wrapperStyle={{ flex: 1, maxWidth: 260 }}
                style={{ padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14 }} />
              <button onClick={clearData} disabled={clrBusy || !clrPw || clrIntakes.length === 0} style={{ ...btn, background: '#b91c1c', opacity: (!clrPw || clrIntakes.length === 0) ? 0.5 : 1 }}>{clrBusy ? 'Archiving & clearing…' : 'Archive & clear'}</button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function incYear(s: string): string {
  const m = s.match(/^(\d{4})\/(\d{4})$/)
  return m ? `${Number(m[1]) + 1}/${Number(m[2]) + 1}` : s
}

function roleColor(role: string) {
  const map: Record<string, { bg: string; text: string }> = {
    VC:          { bg: '#fef3c7', text: '#92400e' },
    DVC:         { bg: '#ffedd5', text: '#9a3412' },
    DQA_DIRECTOR:{ bg: '#e0e7ff', text: '#3730a3' },
    QA_OFFICER:  { bg: '#f0fdf4', text: '#166534' },
    COORDINATOR: { bg: '#f0f9ff', text: '#0369a1' },
    ADMIN:       { bg: '#fce7f3', text: '#9d174d' },
  }
  return map[role] ?? { bg: '#f1f5f9', text: '#475569' }
}

// PasscodeGate blocks the Users page until the tenant's fixed passcode is entered.
// If no passcode is set yet, the admin is prompted to set one first.
function PasscodeGate({ onUnlock }: { onUnlock: () => void }) {
  const [isSet, setIsSet] = useState<boolean | null>(null)
  const [code, setCode] = useState('')
  const [confirm, setConfirm] = useState('')
  const [err, setErr] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    api.get<{ is_set: boolean }>('/api/v1/admin/settings/users-passcode')
      .then(r => setIsSet(!!r.is_set))
      .catch(() => setIsSet(false))
  }, [])

  async function verify() {
    setBusy(true); setErr(null)
    try {
      const r = await api.post<{ ok: boolean }>('/api/v1/admin/settings/users-passcode/verify', { passcode: code })
      if (r.ok) onUnlock()
      else setErr('Incorrect passcode.')
    } catch (e) { setErr(e instanceof Error ? e.message : 'Verification failed') }
    finally { setBusy(false) }
  }

  async function setPasscode() {
    setBusy(true); setErr(null)
    try {
      if (code.length < 4) throw new Error('Passcode must be at least 4 characters.')
      if (code !== confirm) throw new Error('Passcodes do not match.')
      await api.put('/api/v1/admin/settings/users-passcode', { passcode: code })
      onUnlock()
    } catch (e) { setErr(e instanceof Error ? e.message : 'Could not set passcode') }
    finally { setBusy(false) }
  }

  if (isSet === null) return <p style={{ color: 'var(--muted)' }}>Loading…</p>

  return (
    <div style={{ maxWidth: 380, margin: '60px auto', background: '#fff', border: '1px solid #e2e8f0', borderRadius: 14, padding: 28, textAlign: 'center' }}>
      <div style={{ fontSize: 40 }}>🔒</div>
      <h2 style={{ margin: '8px 0 4px' }}>{isSet ? 'Enter Users passcode' : 'Set a Users passcode'}</h2>
      <p style={{ color: 'var(--muted)', fontSize: 13, marginBottom: 18 }}>
        {isSet
          ? 'This page is protected. Enter the passcode set by the administrator.'
          : 'No passcode is set yet. Create one to protect the Users page from here on.'}
      </p>
      {err && <div style={errorBox}>{err}</div>}
      <PasswordInput value={code} placeholder="Passcode" autoFocus
        onChange={e => setCode(e.target.value)}
        onKeyDown={e => { if (e.key === 'Enter' && isSet) verify() }}
        wrapperStyle={{ marginBottom: 10 }}
        style={{ padding: '11px 12px', border: '1px solid #cbd5e1', borderRadius: 8, fontSize: 15 }} />
      {!isSet && (
        <PasswordInput value={confirm} placeholder="Confirm passcode"
          onChange={e => setConfirm(e.target.value)}
          wrapperStyle={{ marginBottom: 10 }}
          style={{ padding: '11px 12px', border: '1px solid #cbd5e1', borderRadius: 8, fontSize: 15 }} />
      )}
      <button onClick={isSet ? verify : setPasscode} disabled={busy} style={{ ...btn, width: '100%', padding: 12 }}>
        {busy ? 'Please wait…' : isSet ? 'Unlock' : 'Set passcode & continue'}
      </button>
    </div>
  )
}

function Field({ label, value, onChange, type = 'text' }: { label: string; value: string; onChange: (v: string) => void; type?: string }) {
  const [show, setShow] = useState(false)
  const isPw = type === 'password'
  return (
    <label>
      <div style={labelStyle}>{label}</div>
      <div style={{ position: 'relative' }}>
        <input type={isPw && show ? 'text' : type} value={value} onChange={e => onChange(e.target.value)}
          style={{ width: '100%', padding: '8px 10px', paddingRight: isPw ? 58 : 10, borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }} />
        {isPw && <button type="button" tabIndex={-1} onClick={() => setShow(s => !s)}
          style={{ position: 'absolute', right: 8, top: '50%', transform: 'translateY(-50%)', border: 'none', background: 'none', cursor: 'pointer', fontSize: 12, fontWeight: 600, color: '#64748b', padding: 4 }}>{show ? 'Hide' : 'Show'}</button>}
      </div>
    </label>
  )
}

const btn:         React.CSSProperties = { padding: '8px 16px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const labelStyle:  React.CSSProperties = { fontSize: 13, fontWeight: 600, marginBottom: 4, color: '#475569' }
const selectStyle: React.CSSProperties = { width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14 }
const errorBox:    React.CSSProperties = { background: '#fef2f2', color: '#b91c1c', padding: '8px 12px', borderRadius: 6, marginBottom: 12, fontSize: 13 }
