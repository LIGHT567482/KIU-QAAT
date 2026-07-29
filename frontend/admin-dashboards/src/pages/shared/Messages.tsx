import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import { useAuth } from '../../contexts/AuthContext'

interface Msg {
  message_id: string
  sender_name: string
  sender_role: string
  audience: 'ALL_QA' | 'DEPARTMENT' | 'SCHOOL' | 'DQA'
  audience_value: string
  subject: string
  body: string
  has_attachment: boolean
  attachment_name: string
  created_at: string
  read: boolean
}

function audienceLabel(m: Msg): string {
  switch (m.audience) {
    case 'ALL_QA':     return 'All QA officers'
    case 'DEPARTMENT': return `Dept · ${m.audience_value}`
    case 'SCHOOL':     return `School · ${m.audience_value}`
    case 'DQA':        return 'DQA Director'
  }
}

function fileToB64(file: File): Promise<{ name: string; mime: string; b64: string }> {
  return new Promise((resolve, reject) => {
    const rd = new FileReader()
    rd.onload = () => resolve({ name: file.name, mime: file.type || 'application/octet-stream', b64: String(rd.result).split(',', 2)[1] ?? '' })
    rd.onerror = reject
    rd.readAsDataURL(file)
  })
}

export default function Messages() {
  const { user } = useAuth()
  const isDQA = user?.role === 'DQA_DIRECTOR'
  const [tab, setTab] = useState<'inbox' | 'sent' | 'compose'>('inbox')

  return (
    <div>
      <h2 style={{ margin: '0 0 4px' }}>Messages</h2>
      <p style={{ color: 'var(--muted)', margin: '0 0 18px', fontSize: 13 }}>
        {isDQA
          ? 'Share reports and notices with QA officers — everyone, or by department or college/school — and read their replies.'
          : 'Notices and reports from the Director of Quality Assurance, and your replies to the DQA.'}
      </p>

      <div style={{ display: 'flex', gap: 8, marginBottom: 18 }}>
        <TabBtn on={tab === 'inbox'}   onClick={() => setTab('inbox')}>Inbox</TabBtn>
        <TabBtn on={tab === 'sent'}    onClick={() => setTab('sent')}>Sent</TabBtn>
        <TabBtn on={tab === 'compose'} onClick={() => setTab('compose')}>✎ Compose</TabBtn>
      </div>

      {tab === 'compose'
        ? <Compose isDQA={isDQA} onSent={() => setTab('sent')} />
        : <MessageList box={tab} />}
    </div>
  )
}

function MessageList({ box }: { box: 'inbox' | 'sent' }) {
  const [msgs, setMsgs] = useState<Msg[] | null>(null)
  const [err, setErr] = useState<string | null>(null)
  const [open, setOpen] = useState<string | null>(null)

  function load() {
    setMsgs(null); setErr(null)
    api.get<Msg[]>(`/api/v1/messages?box=${box}`).then(setMsgs).catch(e => setErr(e.message))
  }
  useEffect(load, [box])

  async function expand(m: Msg) {
    setOpen(o => (o === m.message_id ? null : m.message_id))
    if (!m.read && box === 'inbox') {
      await api.post(`/api/v1/messages/${m.message_id}/read`).catch(() => {})
      setMsgs(list => list?.map(x => x.message_id === m.message_id ? { ...x, read: true } : x) ?? null)
    }
  }

  if (err) return <div style={errBox}>{err}</div>
  if (!msgs) return <p style={{ color: 'var(--muted)' }}>Loading…</p>
  if (msgs.length === 0) return <p style={{ color: 'var(--muted)' }}>No messages {box === 'sent' ? 'sent yet' : 'yet'}.</p>

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      {msgs.map(m => {
        const isOpen = open === m.message_id
        const unread = box === 'inbox' && !m.read
        return (
          <div key={m.message_id} style={{ border: '1px solid var(--border)', borderRadius: 10, background: 'var(--surface)', overflow: 'hidden' }}>
            <button onClick={() => expand(m)} style={{
              width: '100%', textAlign: 'left', cursor: 'pointer', border: 'none', background: unread ? 'rgba(26,122,63,.06)' : 'transparent',
              padding: '12px 14px', display: 'flex', justifyContent: 'space-between', gap: 12, alignItems: 'center', color: 'var(--text)',
            }}>
              <span style={{ minWidth: 0 }}>
                <span style={{ fontWeight: unread ? 700 : 600 }}>
                  {unread && <span style={{ display: 'inline-block', width: 8, height: 8, borderRadius: 8, background: 'var(--brand)', marginRight: 8 }} />}
                  {m.subject}
                </span>
                <span style={{ display: 'block', fontSize: 12, color: 'var(--muted)', marginTop: 2 }}>
                  {box === 'sent' ? `To: ${audienceLabel(m)}` : `From: ${m.sender_name} (${m.sender_role.replace(/_/g, ' ')})`}
                  {m.has_attachment && ' · 📎'}
                </span>
              </span>
              <span style={{ fontSize: 11, color: 'var(--muted)', whiteSpace: 'nowrap' }}>{new Date(m.created_at).toLocaleString()}</span>
            </button>
            {isOpen && (
              <div style={{ padding: '0 14px 14px', borderTop: '1px solid var(--border)' }}>
                <p style={{ whiteSpace: 'pre-wrap', margin: '12px 0', fontSize: 14, lineHeight: 1.5 }}>{m.body || <em style={{ color: 'var(--muted)' }}>(no message body)</em>}</p>
                {m.has_attachment && (
                  <button onClick={() => api.download(`/api/v1/messages/${m.message_id}/attachment`, m.attachment_name || 'attachment').catch(() => {})}
                    style={{ ...smallBtn }}>📎 Download {m.attachment_name || 'attachment'}</button>
                )}
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}

function Compose({ isDQA, onSent }: { isDQA: boolean; onSent: () => void }) {
  const [audience, setAudience] = useState<'ALL_QA' | 'DEPARTMENT' | 'SCHOOL'>('ALL_QA')
  const [audienceValue, setAudienceValue] = useState('')
  const [subject, setSubject] = useState('')
  const [body, setBody] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [saving, setSaving] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [ok, setOk] = useState(false)
  const [audOpts, setAudOpts] = useState<{ departments: string[]; schools: string[] }>({ departments: [], schools: [] })

  useEffect(() => {
    if (isDQA) api.get<{ departments: string[]; schools: string[] }>('/api/v1/messages/audiences').then(setAudOpts).catch(() => {})
  }, [isDQA])

  async function send() {
    setErr(null)
    if (!subject.trim()) { setErr('Subject is required.'); return }
    if (isDQA && (audience === 'DEPARTMENT' || audience === 'SCHOOL') && !audienceValue.trim()) {
      setErr(`Pick a ${audience === 'DEPARTMENT' ? 'department' : 'college/school'}.`); return
    }
    setSaving(true)
    try {
      const att = file ? await fileToB64(file) : null
      await api.post('/api/v1/messages', {
        audience: isDQA ? audience : 'DQA',
        audience_value: isDQA ? audienceValue : '',
        subject: subject.trim(),
        body,
        attachment_name: att?.name ?? '',
        attachment_mime: att?.mime ?? '',
        attachment_b64: att?.b64 ?? '',
      })
      setOk(true); setSubject(''); setBody(''); setFile(null); setAudienceValue('')
      setTimeout(onSent, 700)
    } catch (e) { setErr(e instanceof Error ? e.message : 'Failed to send') }
    finally { setSaving(false) }
  }

  const suggestions = audience === 'SCHOOL' ? audOpts.schools : audOpts.departments

  return (
    <div style={{ maxWidth: 620, border: '1px solid var(--border)', borderRadius: 12, padding: 20, background: 'var(--surface)' }}>
      {err && <div style={errBox}>{err}</div>}
      {ok && <div style={{ background: '#ecfdf5', color: '#065f46', padding: '8px 12px', borderRadius: 6, marginBottom: 12, fontSize: 13 }}>✓ Sent.</div>}

      {isDQA ? (
        <>
          <label style={lbl}>Send to</label>
          <div style={{ display: 'flex', gap: 8, marginBottom: 12, flexWrap: 'wrap' }}>
            {(['ALL_QA', 'DEPARTMENT', 'SCHOOL'] as const).map(a => (
              <button key={a} type="button" onClick={() => { setAudience(a); setAudienceValue('') }}
                style={{ padding: '6px 14px', borderRadius: 999, fontSize: 13, fontWeight: 600, cursor: 'pointer',
                  border: audience === a ? '1px solid var(--brand)' : '1px solid var(--border)',
                  background: audience === a ? 'var(--brand)' : 'transparent', color: audience === a ? '#fff' : 'var(--text)' }}>
                {a === 'ALL_QA' ? 'All QA officers' : a === 'DEPARTMENT' ? 'By department' : 'By college/school'}
              </button>
            ))}
          </div>
          {(audience === 'DEPARTMENT' || audience === 'SCHOOL') && (
            <>
              <input list="aud-opts" value={audienceValue} onChange={e => setAudienceValue(e.target.value)}
                placeholder={audience === 'DEPARTMENT' ? 'Department name' : 'College / school name'} style={inp} />
              <datalist id="aud-opts">{suggestions.map(s => <option key={s} value={s} />)}</datalist>
            </>
          )}
        </>
      ) : (
        <div style={{ fontSize: 13, color: 'var(--muted)', marginBottom: 12 }}>To: <strong>Director of Quality Assurance</strong></div>
      )}

      <label style={lbl}>Subject</label>
      <input value={subject} onChange={e => setSubject(e.target.value)} placeholder="Subject" style={inp} />
      <label style={lbl}>Message</label>
      <textarea value={body} onChange={e => setBody(e.target.value)} placeholder="Write your message…" rows={6}
        style={{ ...inp, resize: 'vertical', fontFamily: 'inherit' }} />
      <label style={lbl}>Attachment (optional)</label>
      <input type="file" onChange={e => setFile(e.target.files?.[0] ?? null)} style={{ marginBottom: 16, fontSize: 13 }} />
      {file && <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: -10, marginBottom: 12 }}>📎 {file.name} ({Math.ceil(file.size / 1024)} KB)</div>}

      <button onClick={send} disabled={saving} style={{ ...smallBtn, background: 'var(--brand)', color: '#fff', border: 'none', padding: '10px 18px', fontSize: 14 }}>
        {saving ? 'Sending…' : 'Send'}
      </button>
    </div>
  )
}

function TabBtn({ on, onClick, children }: { on: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button onClick={onClick} style={{
      padding: '8px 16px', borderRadius: 8, fontSize: 14, fontWeight: 600, cursor: 'pointer',
      border: on ? '1px solid var(--brand)' : '1px solid var(--border)',
      background: on ? 'var(--brand)' : 'transparent', color: on ? '#fff' : 'var(--text)',
    }}>{children}</button>
  )
}

const lbl: React.CSSProperties = { display: 'block', fontSize: 13, fontWeight: 600, color: 'var(--muted)', margin: '0 0 5px' }
const inp: React.CSSProperties = { width: '100%', padding: '9px 11px', borderRadius: 6, border: '1px solid var(--border)', fontSize: 14, boxSizing: 'border-box', marginBottom: 14, background: 'var(--surface)', color: 'var(--text)' }
const smallBtn: React.CSSProperties = { padding: '7px 12px', borderRadius: 6, border: '1px solid var(--border)', background: 'var(--surface)', color: 'var(--text)', cursor: 'pointer', fontSize: 13, fontWeight: 600 }
const errBox: React.CSSProperties = { background: '#fef2f2', color: '#b91c1c', padding: '8px 12px', borderRadius: 6, marginBottom: 12, fontSize: 13 }
