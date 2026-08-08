import { useEffect, useState, type FormEvent } from 'react'
import { api } from '../../lib/api'

/**
 * Quality Assurance writing to its own field staff.
 *
 * The QA officer and the DQA director run the patrol round, and until now they could not send a
 * patroller a single line. Every other role with people under it had a channel — a lecturer reaches
 * their students, a coordinator their lecturers, a dean their heads of department — while the two
 * roles responsible for the round had none. Reassigning a round, moving a start time, calling a
 * handset in: all of it was a phone call, off the record.
 *
 * It goes through `/api/v1/app-notifications`, which is the same inbox the patroller's phone app
 * already polls and shows a badge for — so this arrives where they are actually looking, rather
 * than in a web console they never open. Nothing new had to be built on the phone.
 *
 * Not `/api/v1/messages`: that is the DQA ⇄ QA-officer channel with attachments, between desks.
 * This is a notice to someone walking a corridor.
 */

interface Patroller {
  user_id: string
  staff_id: string
  full_name: string
  email: string
}

type Sent = { count: number; to: string } | null

export default function QAPatrollerBriefing() {
  const [patrollers, setPatrollers] = useState<Patroller[] | null>(null)
  const [toAll, setToAll]   = useState(true)
  const [target, setTarget] = useState('')
  const [subject, setSubject] = useState('')
  const [body, setBody]     = useState('')
  const [busy, setBusy]     = useState(false)
  const [sent, setSent]     = useState<Sent>(null)
  const [error, setError]   = useState<string | null>(null)

  useEffect(() => {
    api.get<Patroller[]>('/api/v1/dashboard/qa/patrollers')
      .then(setPatrollers)
      .catch(e => { setPatrollers([]); setError(e instanceof Error ? e.message : 'Could not load monitors') })
  }, [])

  async function send(e: FormEvent) {
    e.preventDefault()
    setBusy(true); setError(null); setSent(null)
    try {
      const res = await api.post<{ recipients: number }>('/api/v1/app-notifications', {
        audience: toAll ? 'PATROLLERS' : 'PATROLLER',
        target_id: toAll ? '' : target,
        subject: subject.trim(),
        body: body.trim(),
      })
      const who = toAll
        ? 'every QA monitor'
        : patrollers?.find(p => p.staff_id === target)?.full_name || target
      setSent({ count: res.recipients ?? 0, to: who })
      setSubject(''); setBody('')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not send')
    } finally {
      setBusy(false)
    }
  }

  const canSend = subject.trim() !== '' && (toAll || target !== '') && !busy

  return (
    <div style={{ maxWidth: 680 }}>
      <h2 style={{ marginBottom: 4 }}>Message the QA monitors</h2>
      <p style={{ color: 'var(--muted)', marginBottom: 20 }}>
        Goes straight to the alerts tab of their phone app, with a badge — the screen they already
        check between rooms. It stays in their inbox until they clear it.
      </p>

      {sent && (
        <div style={successBox}>
          Sent to {sent.to}
          {sent.count > 0 && <> — <strong>{sent.count}</strong> {sent.count === 1 ? 'monitor' : 'monitors'}</>}.
          {sent.count === 0 && ' No active monitor matched, so nobody received it.'}
        </div>
      )}
      {error && <div style={errorBox}>{error}</div>}

      <form onSubmit={send} style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        <fieldset style={{ border: '1px solid var(--border)', borderRadius: 8, padding: 14 }}>
          <legend style={{ padding: '0 6px', fontSize: 13, color: 'var(--muted)' }}>Who</legend>
          <label style={row}>
            <input type="radio" checked={toAll} onChange={() => setToAll(true)} />
            <span>Every QA monitor {patrollers && <span style={{ color: 'var(--muted)' }}>({patrollers.length})</span>}</span>
          </label>
          <label style={row}>
            <input type="radio" checked={!toAll} onChange={() => setToAll(false)} />
            <span>One QA monitor</span>
          </label>
          {!toAll && (
            <select value={target} onChange={e => setTarget(e.target.value)} style={{ ...inp, marginTop: 8 }}>
              <option value="">Choose…</option>
              {(patrollers ?? []).map(p => (
                <option key={p.user_id} value={p.staff_id}>
                  {p.full_name || p.email} {p.staff_id && `(${p.staff_id})`}
                </option>
              ))}
            </select>
          )}
          {patrollers?.length === 0 && (
            <p style={{ color: 'var(--muted)', fontSize: 13, marginTop: 8, marginBottom: 0 }}>
              No active QA monitor accounts exist yet. An administrator creates them under Users.
            </p>
          )}
        </fieldset>

        <label>
          <div style={lbl}>Subject</div>
          <input
            value={subject} onChange={e => setSubject(e.target.value)} required maxLength={120}
            placeholder="e.g. Round change — start at the Science block today"
            style={inp}
          />
        </label>

        <label>
          <div style={lbl}>Message</div>
          <textarea
            value={body} onChange={e => setBody(e.target.value)} rows={6}
            placeholder="Anything they need before they set off. Rooms, times, a lecturer to look out for."
            style={{ ...inp, resize: 'vertical', fontFamily: 'inherit' }}
          />
        </label>

        <button type="submit" disabled={!canSend} style={{ ...btn, opacity: canSend ? 1 : 0.5 }}>
          {busy ? 'Sending…' : toAll ? 'Send to every QA monitor' : 'Send'}
        </button>
      </form>

      <p style={{ color: 'var(--muted)', fontSize: 13, marginTop: 20 }}>
        Inactive accounts are skipped, so the count above is the number of patrollers who can
        actually sign in and read it.
      </p>
    </div>
  )
}

const row: React.CSSProperties = { display: 'flex', alignItems: 'center', gap: 8, padding: '4px 0' }
const lbl: React.CSSProperties = { fontSize: 13, color: 'var(--muted)', marginBottom: 4 }
const inp: React.CSSProperties = {
  padding: '10px 12px', fontSize: 15, borderRadius: 6, width: '100%', boxSizing: 'border-box',
  border: '1px solid var(--border)', background: 'var(--surface)', color: 'var(--text)',
}
const btn: React.CSSProperties = {
  padding: 12, fontSize: 15, fontWeight: 600, borderRadius: 6, border: 'none', cursor: 'pointer',
  background: 'var(--brand)', color: 'var(--brand-contrast)',
}
const successBox: React.CSSProperties = { background: '#dcfce7', color: '#166534', padding: '10px 14px', borderRadius: 6, marginBottom: 16 }
const errorBox: React.CSSProperties = { background: '#fee2e2', color: '#b91c1c', padding: '10px 14px', borderRadius: 6, marginBottom: 16 }
