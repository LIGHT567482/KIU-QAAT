import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'
import { Kpi, KpiRow, RateBar } from '../../components/Kpi'

/**
 * The dean's view of their heads of department — the management layer they are accountable
 * *through*, which the dashboard previously skipped entirely.
 *
 * Before this, a dean saw one flat list of every lecturer in the college and had no way to tell
 * which department each belonged to, who ran it, or how that department was doing. The only action
 * available was "notify all lecturers", which routes straight past the person actually responsible
 * for them.
 *
 * Each row is one department: who heads it, whether they have ever signed in, and the four figures
 * a dean is judged on — classes taught against the timetable, whether the lecturer actually turned
 * up to the ones that ran, student attendance, and how many students are about to lose eligibility.
 *
 * The two red states are coordination failures, not slow departments:
 *   • **No HOD** — nobody is answerable for it.
 *   • **Not linked** — the department exists on the org tree but no course carries its name, so
 *     every query its HOD's dashboard runs comes back empty. They would see a working, blank
 *     dashboard and have no way to know why.
 */

interface Hod {
  user_id: string; full_name: string; email: string
  title: string; phone: string; last_login_at: string
}
interface Dept {
  department_id: string; name: string; school: string; kind: string
  hod: Hod | null
  courses: number; units: number; units_unstaffed: number
  lecturers: number; students: number
  sessions_held: number; sessions_planned: number
  taught_rate: number; lecturer_show_rate: number
  avg_attendance: number; at_risk: number
  patrolled: number; patrol_taught: number
  no_hod: boolean; unlinked: boolean; hod_never_signed_in: boolean
}
interface Resp { scope?: string; window_days?: number; departments?: Dept[]; unset?: boolean; message?: string }

export default function OrgDepartments({ canNotify = true }: { canNotify?: boolean }) {
  const nav = useNavigate()
  const { status, data } = useQuery<Resp>(() => api.get('/api/v1/org/departments'), [])
  const [notify, setNotify] = useState<null | { audience: string; target?: string; who: string }>(null)

  const rows = data?.departments ?? []

  if (status === 'loading') return <p style={{ color: 'var(--muted)' }}>Loading…</p>
  if (data?.unset) {
    return (
      <div style={warnBox}>
        <strong>Your account has no college or school set.</strong>
        <p style={{ margin: '6px 0 0', fontSize: 13 }}>{data.message}</p>
      </div>
    )
  }

  const noHod = rows.filter(d => d.no_hod).length
  const unlinked = rows.filter(d => d.unlinked).length
  const neverIn = rows.filter(d => d.hod_never_signed_in).length
  const atRisk = rows.reduce((s, d) => s + d.at_risk, 0)

  return (
    <div>
      <h2 style={{ margin: '0 0 4px' }}>Departments &amp; heads of department</h2>
      <p style={{ color: 'var(--muted)', margin: '0 0 18px', fontSize: 13 }}>
        Every department in <b>{data?.scope || 'your college'}</b>, who runs it, and how it is doing.
        Teaching figures cover the last {data?.window_days ?? 90} days.
      </p>

      <KpiRow>
        <Kpi label="Departments" value={rows.length} />
        <Kpi label="Without a head" value={noHod} tone={noHod > 0 ? 'bad' : 'good'}
          sub={noHod > 0 ? 'nobody is answerable' : 'all headed'} />
        <Kpi label="Heads never signed in" value={neverIn} tone={neverIn > 0 ? 'warn' : 'good'}
          sub={neverIn > 0 ? 'account issued, never used' : 'all active'} />
        <Kpi label="Not linked to courses" value={unlinked} tone={unlinked > 0 ? 'bad' : 'good'}
          sub={unlinked > 0 ? 'their dashboards show nothing' : 'all linked'} />
        <Kpi label="Students at risk" value={atRisk} tone={atRisk > 0 ? 'bad' : 'good'} sub="across the college" />
      </KpiRow>

      {canNotify && rows.some(d => d.hod) && (
        <div style={{ marginBottom: 16 }}>
          <button style={btn} onClick={() => setNotify({ audience: 'HODS', who: `every head of department in ${data?.scope || 'your college'}` })}>
            ✉ Notify all heads of department
          </button>
        </div>
      )}

      {rows.length === 0 && (
        <p style={{ color: 'var(--muted)' }}>
          No departments are recorded under this college yet. An administrator adds them under
          Schools &amp; Departments.
        </p>
      )}

      <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        {rows.map(d => (
          <DeptCard
            key={d.department_id} d={d} canNotify={canNotify}
            onNotify={() => d.hod && setNotify({ audience: 'HOD', target: d.hod.user_id, who: d.hod.full_name || d.name })}
            onOpen={() => nav(`/dean/lecturers?department=${encodeURIComponent(d.name)}`)}
          />
        ))}
      </div>

      {notify && <NotifyDialog {...notify} onClose={() => setNotify(null)} />}
    </div>
  )
}

function DeptCard({ d, canNotify, onNotify, onOpen }: {
  d: Dept; canNotify: boolean; onNotify: () => void; onOpen: () => void
}) {
  const broken = d.no_hod || d.unlinked
  return (
    <div style={{
      border: `1px solid ${broken ? 'rgba(185,28,28,.35)' : 'var(--border)'}`,
      background: broken ? 'rgba(185,28,28,.04)' : 'var(--surface)',
      borderRadius: 12, padding: 16,
    }}>
      <div style={{ display: 'flex', gap: 12, alignItems: 'flex-start', flexWrap: 'wrap' }}>
        <div style={{ flex: 1, minWidth: 220 }}>
          <div style={{ fontWeight: 700, fontSize: 15 }}>{d.name}</div>
          {d.hod ? (
            <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 3 }}>
              Head: <strong style={{ color: 'var(--text)' }}>
                {[d.hod.title, d.hod.full_name].filter(Boolean).join(' ')}
              </strong>
              {d.hod.email && <> · {d.hod.email}</>}
              {d.hod.phone && <> · {d.hod.phone}</>}
              <div style={{ fontSize: 11, marginTop: 2 }}>
                {d.hod.last_login_at
                  ? `Last signed in ${d.hod.last_login_at}`
                  : <span style={{ color: '#b45309', fontWeight: 600 }}>Has never signed in</span>}
              </div>
            </div>
          ) : (
            <div style={{ fontSize: 13, color: '#b91c1c', fontWeight: 600, marginTop: 3 }}>
              No head of department — nobody is answerable for this department.
            </div>
          )}
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          {/* Straight from a department to the lecturers inside it — the drill-down a dean was
              missing, since the flat college-wide list could not be narrowed at all. */}
          {!d.unlinked && <button style={btnGhost} onClick={onOpen}>Lecturers →</button>}
          {canNotify && d.hod && <button style={btnGhost} onClick={onNotify}>✉ Notify</button>}
        </div>
      </div>

      {d.unlinked ? (
        <div style={{ marginTop: 10, fontSize: 12, color: '#b91c1c' }}>
          <strong>Not linked to any course.</strong> No course carries this department's name, so
          every screen its head opens will be empty. An administrator should set the department on
          its courses (Courses &amp; Sessions) so the two match exactly.
        </div>
      ) : (
        <>
          <div style={{
            display: 'grid', gap: 10, marginTop: 12,
            gridTemplateColumns: 'repeat(auto-fit, minmax(120px, 1fr))',
          }}>
            <Stat label="Lecturers" value={d.lecturers} />
            <Stat label="Students" value={d.students} />
            <Stat label="Courses" value={`${d.courses} · ${d.units} units`} />
            <Stat
              label="Units unstaffed" value={d.units_unstaffed}
              danger={d.units_unstaffed > 0}
            />
            <Stat label="Patrolled" value={d.patrolled > 0 ? `${d.patrol_taught}/${d.patrolled} teaching` : '—'} />
          </div>

          <div style={{
            display: 'grid', gap: 14, marginTop: 14,
            gridTemplateColumns: 'repeat(auto-fit, minmax(170px, 1fr))',
          }}>
            <Metric label="Classes taught" hint={`${d.sessions_held} of ~${d.sessions_planned} timetabled`}>
              <RateBar pct={d.taught_rate} threshold={90} />
            </Metric>
            {/* The ghost-lecture measure: a session ran, but did the lecturer actually gate in? */}
            <Metric label="Lecturer turned up" hint="of the sessions that ran">
              <RateBar pct={d.lecturer_show_rate} threshold={90} />
            </Metric>
            <Metric label="Student attendance" hint={`${d.at_risk} below the bar`}>
              <RateBar pct={d.avg_attendance} threshold={75} />
            </Metric>
          </div>
        </>
      )}
    </div>
  )
}

function Stat({ label, value, danger = false }: { label: string; value: React.ReactNode; danger?: boolean }) {
  return (
    <div>
      <div style={{ fontSize: 11, color: 'var(--muted)' }}>{label}</div>
      <div style={{ fontWeight: 700, fontSize: 15, color: danger ? '#b91c1c' : 'var(--text)' }}>{value}</div>
    </div>
  )
}

function Metric({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <div>
      <div style={{ fontSize: 11, color: 'var(--muted)' }}>{label}</div>
      {children}
      {hint && <div style={{ fontSize: 10, color: 'var(--muted)', marginTop: 2 }}>{hint}</div>}
    </div>
  )
}

/** Sends through the shared app-notifications endpoint; the backend re-derives the audience from
 *  the sender's own school, so "all heads" can only ever mean the heads of THIS college. */
function NotifyDialog({ audience, target, who, onClose }: {
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
            <p style={{ fontSize: 14 }}>
              {sent === 0
                ? 'Nobody matched — that head may have no account, or no department under your college.'
                : `Sent to ${sent} recipient${sent === 1 ? '' : 's'}.`}
            </p>
            <button style={btn} onClick={onClose}>Close</button>
          </>
        ) : (
          <>
            {err && <div style={{ background: '#fef2f2', color: '#b91c1c', padding: 10, borderRadius: 8, fontSize: 13, marginBottom: 10 }}>{err}</div>}
            <input value={subject} onChange={e => setSubject(e.target.value)} placeholder="Subject" style={{ ...inp, width: '100%', marginBottom: 8 }} />
            <textarea value={body} onChange={e => setBody(e.target.value)} placeholder="Message" rows={5} style={{ ...inp, width: '100%', marginBottom: 10, resize: 'vertical' }} />
            <div style={{ display: 'flex', gap: 8 }}>
              <button style={btn} disabled={busy || !subject.trim()} onClick={send}>{busy ? 'Sending…' : 'Send'}</button>
              <button style={btnGhost} onClick={onClose}>Cancel</button>
            </div>
          </>
        )}
      </div>
    </div>
  )
}

const btn: React.CSSProperties = {
  padding: '8px 16px', borderRadius: 8, border: 'none',
  background: 'var(--brand)', color: '#fff', cursor: 'pointer', fontSize: 13, fontWeight: 600,
}
const btnGhost: React.CSSProperties = {
  padding: '8px 14px', borderRadius: 8, border: '1px solid var(--border)',
  background: 'var(--surface)', color: 'var(--text)', cursor: 'pointer', fontSize: 13,
}
const inp: React.CSSProperties = {
  padding: '8px 10px', borderRadius: 8, border: '1px solid var(--border)',
  fontSize: 13, background: 'var(--surface)', color: 'var(--text)', boxSizing: 'border-box',
}
const overlay: React.CSSProperties = {
  position: 'fixed', inset: 0, background: 'rgba(0,0,0,.45)',
  display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 50, padding: 20,
}
const modal: React.CSSProperties = {
  background: 'var(--surface)', color: 'var(--text)', borderRadius: 12,
  padding: 20, width: '100%', maxWidth: 460,
}
const warnBox: React.CSSProperties = {
  background: 'rgba(180,83,9,.08)', border: '1px solid rgba(180,83,9,.3)',
  borderRadius: 10, padding: '14px 16px', color: '#92400e',
}
