import { useState } from 'react'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'
import { useAuth } from '../../contexts/AuthContext'

// Dashboards for the two org-scoped QA roles:
//
//   QA_DEPT_REP        — one department  (/qa-dept, /qa-dept/report)
//   QA_SCHOOL_HANDLER  — one school      (/qa-school, /qa-school/lecturers, /qa-school/reports)
//
// Both see the same data through different lenses, so the three views below are shared and the
// scope comes from the server (the caller's own user record), never from the page. The reports view
// is where a rep uploads the monitoring workbook they already fill in by hand: the recognised rows
// become teaching observations and the workbook itself is kept as the evidence behind them.

interface Scope {
  role: string; scope_kind: string; department: string; school: string
  full_name: string; staff_id: string; unscoped: boolean
}
interface ScopeResp {
  scope: Scope; label: string; my_submissions: number; message: string; can_submit: boolean
}

// ─── Shared header ───────────────────────────────────────────────────────────

// The scope banner is driven by /qa-rep/scope — the server's own answer to "who am I and what do I
// cover" — rather than inferred from whichever list happens to have loaded. That keeps the three
// views agreeing with each other and with the rules the upload is actually enforcing.
function useScope() {
  return useQuery<ScopeResp>(() => api.get('/api/v1/qa-rep/scope'), [])
}

function ScopeHeader({ title, subtitle, scope, message }: {
  title: string; subtitle: string; scope?: ScopeResp; message?: string
}) {
  const label = scope?.label
  const note = message || scope?.message
  return (
    <>
      <h2 style={{ margin: '0 0 4px' }}>{title}</h2>
      <p style={{ color: 'var(--muted)', margin: '0 0 16px', fontSize: 13 }}>
        {label ? <>Scope: <b>{label}</b>. </> : null}{subtitle}
      </p>
      {note && <div style={warn}>{note}</div>}
    </>
  )
}

// ─── Lecturers in scope ──────────────────────────────────────────────────────

interface LecturerRow {
  staff_id: string; full_name: string; department?: string; school?: string
  taught_count: number; patrolled_count: number; unit_count: number
}
interface LecturersResp { scope: { department: string; school: string }; lecturers: LecturerRow[]; message?: string }

export function QAOrgLecturers() {
  const { user } = useAuth()
  const bySchool = user?.role === 'QA_SCHOOL_HANDLER'
  const scope = useScope()
  const { status, data } = useQuery<LecturersResp>(() => api.get('/api/v1/qa-rep/lecturers'), [])
  const rows = data?.lecturers ?? []
  const scopeLabel = scope.data?.label || (bySchool ? 'your school' : 'your department')
  const [compose, setCompose] = useState<null | { audience: string; target?: string; who: string }>(null)

  return (
    <div>
      <ScopeHeader
        title={bySchool ? 'School — Lecturer Monitoring' : 'Department — Lecturer Monitoring'}
        subtitle="Taught / observed counts every record this term — the QA patroller's and your own uploads alike."
        scope={scope.data} message={data?.message}
      />

      <div style={{ display: 'flex', gap: 8, marginBottom: 16, flexWrap: 'wrap' }}>
        <button style={btn} onClick={() => setCompose({ audience: 'LECTURERS', who: `all lecturers in ${scopeLabel}` })}>✉ Notify all lecturers</button>
        <button style={btnGhost} onClick={() => setCompose({ audience: 'DQA', who: 'the DQA director(s)' })}>Message DQA</button>
        <button style={btnGhost} onClick={() => setCompose({ audience: 'ADMIN', who: 'the admin(s)' })}>Message Admin</button>
      </div>

      {status === 'loading' && <div style={muted}>Loading…</div>}
      {status === 'ok' && rows.length === 0 && !data?.message && (
        <div style={muted}>No lecturers found in this scope yet.</div>
      )}
      {rows.length > 0 && (
        <table style={table}>
          <thead>
            <tr style={htr}>
              <th style={th}>Lecturer</th><th style={th}>Staff ID</th>
              {bySchool && <th style={th}>Department</th>}
              <th style={th}>Units</th><th style={th}>Taught / Observed</th><th style={th}></th>
            </tr>
          </thead>
          <tbody>
            {rows.map(l => (
              <tr key={l.staff_id} style={tr}>
                <td style={td}><b>{l.full_name}</b></td>
                <td style={td}>{l.staff_id}</td>
                {bySchool && <td style={td}>{l.department || '—'}</td>}
                <td style={td}>{l.unit_count}</td>
                <td style={td}><Ratio taught={l.taught_count} total={l.patrolled_count} /></td>
                <td style={td}>
                  <button style={btnSm} onClick={() => setCompose({ audience: 'LECTURER', target: l.staff_id, who: l.full_name })}>Notify</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {compose && <ComposeDialog {...compose} onClose={() => setCompose(null)} />}
    </div>
  )
}

// ComposeDialog sends an in-app notification through the shared app-notifications endpoint. The
// backend re-derives the audience from the sender's own org unit, so "all lecturers" can only ever
// mean the lecturers of this rep's department or school.
function ComposeDialog({ audience, target, who, onClose }: {
  audience: string; target?: string; who: string; onClose: () => void
}) {
  const [subject, setSubject] = useState('')
  const [body, setBody] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [sent, setSent] = useState<number | null>(null)

  async function send() {
    setBusy(true); setErr(null)
    try {
      const res = await api.post<{ recipients: number }>('/api/v1/app-notifications',
        { audience, target_id: target, subject, body })
      setSent(res.recipients ?? 0)
    } catch (e) { setErr(e instanceof Error ? e.message : 'Failed to send') }
    finally { setBusy(false) }
  }

  return (
    <div style={overlay} onClick={onClose}>
      <div style={modal} onClick={e => e.stopPropagation()}>
        <h3 style={{ margin: '0 0 8px' }}>Notify {who}</h3>
        {sent !== null ? (
          <>
            <div style={sent > 0 ? ok : warn}>
              {sent > 0
                ? `Sent to ${sent} recipient(s).`
                : 'Nobody received this — no one in your scope has an account to receive it.'}
            </div>
            <button style={{ ...btn, marginTop: 10 }} onClick={onClose}>Close</button>
          </>
        ) : (
          <>
            {err && <div style={warn}>{err}</div>}
            <input placeholder="Subject" value={subject} onChange={e => setSubject(e.target.value)} style={{ ...inp, margin: '6px 0' }} />
            <textarea placeholder="Message…" value={body} onChange={e => setBody(e.target.value)}
              style={{ ...inp, height: 120, resize: 'vertical', margin: '6px 0' }} />
            <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
              <button style={btnGhost} onClick={onClose}>Cancel</button>
              <button style={btn} disabled={busy || !subject.trim()} onClick={send}>{busy ? 'Sending…' : 'Send'}</button>
            </div>
          </>
        )}
      </div>
    </div>
  )
}

// ─── Per-department roll-up (the school handler's landing page) ──────────────

interface DeptRow {
  department: string; school: string; lecturers: number
  patrolled: number; taught: number; last_report?: string
}
interface DeptResp { scope: Scope; label: string; message?: string; departments: DeptRow[] }

export function QAOrgDepartments() {
  const scope = useScope()
  const { status, data } = useQuery<DeptResp>(() => api.get('/api/v1/qa-rep/departments'), [])
  const rows = data?.departments ?? []

  return (
    <div>
      <ScopeHeader
        title="School Overview"
        subtitle="One row per department: how much teaching was observed, and when the department last filed a QA report."
        scope={scope.data} message={data?.message}
      />
      {status === 'loading' && <div style={muted}>Loading…</div>}
      {status === 'ok' && rows.length === 0 && !data?.message && (
        <div style={muted}>No departments carry courses in this scope yet.</div>
      )}
      {rows.length > 0 && (
        <table style={table}>
          <thead>
            <tr style={htr}>
              <th style={th}>Department</th><th style={th}>Lecturers</th>
              <th style={th}>Taught / Observed</th><th style={th}>Last QA report</th>
            </tr>
          </thead>
          <tbody>
            {rows.map(d => (
              <tr key={d.department} style={tr}>
                <td style={td}><b>{d.department}</b></td>
                <td style={td}>{d.lecturers}</td>
                <td style={td}><Ratio taught={d.taught} total={d.patrolled} /></td>
                <td style={td}>
                  {d.last_report
                    ? d.last_report
                    : <span style={{ color: '#b45309', fontWeight: 600 }}>never filed</span>}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

// ─── Reports: upload + the submissions on file ───────────────────────────────

interface Submission {
  submission_id: string; submitter_name: string; submitter_role: string
  scope_kind: string; department: string; school: string
  period_label: string; period_from?: string; period_to?: string; notes: string
  file_name: string; file_size: number
  total_rows: number; parsed_rows: number; skipped_rows: number
  parse_errors: string[]; created_at: string; mine: boolean
}
interface SubsResp { scope: Scope; label: string; message?: string; submissions: Submission[] }

interface UploadResult {
  submission_id: string; file_name: string; total_rows: number
  recorded: number; updated: number; skipped: number; errors: string[]; message: string
}

export function QAOrgReports() {
  const scope = useScope()
  const { status, data, refetch } = useQuery<SubsResp>(() => api.get('/api/v1/qa-rep/submissions'), [])
  const subs = data?.submissions ?? []
  // The server decides whether this account may file at all — an unscoped oversight role reads the
  // list without an upload box, and a rep whose department was never set is told why.
  const canSubmit = scope.data?.can_submit ?? false

  return (
    <div>
      <ScopeHeader
        title="QA Reports"
        subtitle="Upload the monitoring workbook for your unit. Its rows become teaching observations, and the file itself is kept as the record behind them."
        scope={scope.data} message={data?.message}
      />

      {canSubmit && <UploadCard onDone={() => { refetch(); scope.refetch() }} />}

      <h3 style={{ margin: '28px 0 8px' }}>Submissions on file</h3>
      {status === 'loading' && <div style={muted}>Loading…</div>}
      {status === 'ok' && subs.length === 0 && <div style={muted}>Nothing filed yet.</div>}
      {subs.map(s => <SubmissionCard key={s.submission_id} sub={s} onChange={refetch} />)}
    </div>
  )
}

// QASubmissionsPanel is the read side of the same list, for the oversight roles (QA officer, DQA,
// VC, admin) whose own pages embed it. They see every department's filing — the endpoint drops the
// org filter for an unscoped role — but get no upload box, since they file nothing themselves.
export function QASubmissionsPanel({ heading }: { heading?: string }) {
  const { status, data, refetch } = useQuery<SubsResp>(() => api.get('/api/v1/qa-rep/submissions'), [])
  const subs = data?.submissions ?? []
  const filed = subs.reduce((n, s) => n + s.parsed_rows, 0)

  return (
    <div>
      {heading && <h3 style={{ margin: '0 0 4px' }}>{heading}</h3>}
      <p style={{ ...muted, margin: '0 0 12px' }}>
        Monitoring workbooks filed by the QA department reps and school handlers. The rows they
        contain already count in the teaching figures above; the original file is kept as the record
        behind them.
        {subs.length > 0 && <> <b>{subs.length}</b> submission(s), <b>{filed}</b> observation(s).</>}
      </p>
      {status === 'loading' && <div style={muted}>Loading…</div>}
      {status === 'ok' && subs.length === 0 && <div style={muted}>No QA rep has filed a report yet.</div>}
      {subs.map(s => <SubmissionCard key={s.submission_id} sub={s} onChange={refetch} />)}
    </div>
  )
}

function UploadCard({ onDone }: { onDone: () => void }) {
  const [file, setFile] = useState<File | null>(null)
  const [meta, setMeta] = useState({ period_label: '', period_from: '', period_to: '', notes: '' })
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [result, setResult] = useState<UploadResult | null>(null)

  async function submit() {
    if (!file) return
    setBusy(true); setErr(null); setResult(null)
    try {
      const form = new FormData()
      form.append('report', file)
      Object.entries(meta).forEach(([k, v]) => form.append(k, v))
      const res = await api.upload<UploadResult>('/api/v1/qa-rep/submissions', form)
      setResult(res)
      setFile(null)
      setMeta({ period_label: '', period_from: '', period_to: '', notes: '' })
      onDone()
    } catch (e) { setErr(e instanceof Error ? e.message : 'Upload failed') }
    finally { setBusy(false) }
  }

  return (
    <div style={card}>
      <h3 style={{ margin: '0 0 4px' }}>File a report</h3>
      <p style={{ ...muted, margin: '0 0 14px' }}>
        Excel (.xlsx) or CSV. Start from the template — it comes pre-filled with your unit's
        timetabled sessions, so you only type YES or NO down the <code>taught</code> column.
      </p>

      <button style={btnGhost} onClick={() => api.download('/api/v1/qa-rep/template.xlsx', 'qa-monitoring-template.xlsx')}>
        ⭳ Download template
      </button>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: 12, marginTop: 16 }}>
        <Field label="Period (e.g. July 2026)">
          <input value={meta.period_label} onChange={e => setMeta({ ...meta, period_label: e.target.value })} style={inp} placeholder="optional" />
        </Field>
        <Field label="Covers from">
          <input type="date" value={meta.period_from} onChange={e => setMeta({ ...meta, period_from: e.target.value })} style={inp} />
        </Field>
        <Field label="Covers to">
          <input type="date" value={meta.period_to} onChange={e => setMeta({ ...meta, period_to: e.target.value })} style={inp} />
        </Field>
        <Field label="Workbook">
          <input type="file" accept=".xlsx,.csv,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
            onChange={e => setFile(e.target.files?.[0] ?? null)} style={{ ...inp, padding: 5 }} />
        </Field>
      </div>
      <Field label="Notes for the DQA (optional)">
        <textarea value={meta.notes} onChange={e => setMeta({ ...meta, notes: e.target.value })}
          style={{ ...inp, width: '100%', height: 64, resize: 'vertical', boxSizing: 'border-box' }} />
      </Field>

      {err && <div style={warn}>{err}</div>}
      <button style={{ ...btn, marginTop: 12, opacity: file && !busy ? 1 : .5 }} disabled={!file || busy} onClick={submit}>
        {busy ? 'Uploading…' : 'Submit report'}
      </button>

      {result && (
        <div style={{ ...ok, marginTop: 14 }}>
          <b>{result.message}</b>
          <div style={{ fontSize: 12, marginTop: 4 }}>
            {result.recorded} new · {result.updated} corrected · {result.skipped} skipped · file kept as {result.file_name}
          </div>
          <ErrorList errors={result.errors} />
        </div>
      )}
    </div>
  )
}

function SubmissionCard({ sub, onChange }: { sub: Submission; onChange: () => void }) {
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState(false)

  async function withdraw() {
    if (!confirm(`Withdraw "${sub.file_name}"?\nThe ${sub.parsed_rows} observation(s) it produced are removed with it.`)) return
    setBusy(true)
    try { await api.delete(`/api/v1/qa-rep/submissions/${sub.submission_id}`); onChange() }
    catch (e) { alert(e instanceof Error ? e.message : 'Could not withdraw it') }
    finally { setBusy(false) }
  }

  return (
    <div style={{ ...card, padding: 14 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap', alignItems: 'baseline' }}>
        <div>
          <b>{sub.department || sub.school || '—'}</b>
          {sub.period_label && <span style={{ color: 'var(--muted)' }}> · {sub.period_label}</span>}
          <div style={{ ...muted, marginTop: 2 }}>
            {sub.submitter_name || 'Unknown'} · {sub.created_at} · {sub.file_name} ({Math.max(1, Math.round(sub.file_size / 1024))} KB)
          </div>
        </div>
        <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
          <Pill n={sub.parsed_rows} label="recorded" tone="good" />
          {sub.skipped_rows > 0 && <Pill n={sub.skipped_rows} label="skipped" tone="warn" />}
          <button style={btnSm} onClick={() => api.download(`/api/v1/qa-rep/submissions/${sub.submission_id}/file`, sub.file_name)}>⭳ File</button>
          {(sub.parse_errors?.length ?? 0) > 0 && (
            <button style={btnSm} onClick={() => setOpen(o => !o)}>{open ? 'Hide' : 'Details'}</button>
          )}
          {sub.mine && <button style={btnSm} disabled={busy} onClick={withdraw}>Withdraw</button>}
        </div>
      </div>
      {sub.notes && <div style={{ ...muted, marginTop: 8, fontStyle: 'italic' }}>“{sub.notes}”</div>}
      {open && <ErrorList errors={sub.parse_errors} />}
    </div>
  )
}

// ─── Small shared pieces ─────────────────────────────────────────────────────

function ErrorList({ errors }: { errors?: string[] }) {
  if (!errors || errors.length === 0) return null
  return (
    <ul style={{ margin: '8px 0 0', paddingLeft: 18, fontSize: 12, color: '#b45309' }}>
      {errors.slice(0, 30).map((e, i) => <li key={i} style={{ marginBottom: 2 }}>{e}</li>)}
      {errors.length > 30 && <li>…and {errors.length - 30} more.</li>}
    </ul>
  )
}

function Ratio({ taught, total }: { taught: number; total: number }) {
  const pct = total ? Math.round((taught / total) * 100) : null
  return (
    <>
      {taught}/{total}
      {pct !== null && (
        <span style={{ marginLeft: 8, fontWeight: 700, color: pct >= 75 ? '#15803d' : '#b91c1c' }}>{pct}%</span>
      )}
    </>
  )
}

function Pill({ n, label, tone }: { n: number; label: string; tone: 'good' | 'warn' }) {
  const c = tone === 'good' ? { bg: '#f0fdf4', fg: '#15803d' } : { bg: '#fffbeb', fg: '#b45309' }
  return (
    <span style={{ background: c.bg, color: c.fg, borderRadius: 999, padding: '2px 9px', fontSize: 11, fontWeight: 700 }}>
      {n} {label}
    </span>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <label style={{ fontSize: 12, color: '#475569', display: 'block' }}><div style={{ margin: '8px 0 2px' }}>{label}</div>{children}</label>
}

const table: React.CSSProperties = { width: '100%', borderCollapse: 'collapse', fontSize: 14 }
const htr: React.CSSProperties = { textAlign: 'left', color: '#64748b', fontSize: 12 }
const th: React.CSSProperties = { padding: '6px 10px' }
const tr: React.CSSProperties = { borderTop: '1px solid #e2e8f0' }
const td: React.CSSProperties = { padding: '10px' }
const muted: React.CSSProperties = { color: '#64748b', fontSize: 13 }
const card: React.CSSProperties = { background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 10, padding: 20, marginBottom: 14 }
const inp: React.CSSProperties = { padding: '7px 9px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, width: '100%', boxSizing: 'border-box' }
const btn: React.CSSProperties = { padding: '9px 16px', background: '#1e293b', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const btnGhost: React.CSSProperties = { padding: '8px 14px', background: '#fff', color: '#1e293b', border: '1px solid #cbd5e1', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const btnSm: React.CSSProperties = { padding: '4px 10px', background: '#fff', color: '#1e293b', border: '1px solid #cbd5e1', borderRadius: 6, cursor: 'pointer', fontSize: 12 }
const warn: React.CSSProperties = { background: '#fffbeb', color: '#b45309', padding: '8px 12px', borderRadius: 6, margin: '8px 0', fontSize: 13 }
const ok: React.CSSProperties = { background: '#f0fdf4', color: '#15803d', padding: '10px 12px', borderRadius: 6, fontSize: 13 }
const overlay: React.CSSProperties = { position: 'fixed', inset: 0, background: 'rgba(0,0,0,.45)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 60, padding: 16 }
const modal: React.CSSProperties = { background: '#fff', color: '#0f172a', borderRadius: 12, padding: 20, width: 'min(480px, 94vw)' }
