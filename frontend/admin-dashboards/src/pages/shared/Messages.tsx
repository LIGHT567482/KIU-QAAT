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
  const vals = m.audience_value.split('||').filter(Boolean).join(', ')
  switch (m.audience) {
    case 'ALL_QA':     return 'All QA officers'
    case 'DEPARTMENT': return `Dept · ${vals}`
    case 'SCHOOL':     return `School · ${vals}`
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

  // Remove it from this inbox at once so the ✕ feels instant, then confirm with the server.
  async function dismiss(m: Msg) {
    setMsgs(list => list?.filter(x => x.message_id !== m.message_id) ?? null)
    if (open === m.message_id) setOpen(null)
    try {
      await api.delete(`/api/v1/messages/${m.message_id}`)
    } catch {
      load()   // put it back if the server refused
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
          <div key={m.message_id} style={{ position: 'relative', border: '1px solid var(--border)', borderRadius: 10, background: 'var(--surface)', overflow: 'hidden' }}>
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
            {/* Dismiss — removes it from THIS inbox; other recipients keep theirs. */}
            <button onClick={e => { e.stopPropagation(); dismiss(m) }} title="Dismiss this message"
              aria-label="Dismiss this message" style={{
                position: 'absolute', top: 6, right: 6, width: 24, height: 24, lineHeight: '22px',
                border: 'none', background: 'transparent', color: 'var(--muted)', cursor: 'pointer',
                fontSize: 16, borderRadius: 6, padding: 0,
              }}>✕</button>
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
  const [audienceValue, setAudienceValue] = useState('')   // single college/school
  const [selectedDepts, setSelectedDepts] = useState<string[]>([]) // multiple departments
  const [deptInput, setDeptInput] = useState('')
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

  function addDept(v: string) {
    const d = v.trim()
    if (d && !selectedDepts.includes(d)) setSelectedDepts(s => [...s, d])
    setDeptInput('')
  }

  async function send() {
    setErr(null)
    if (!subject.trim()) { setErr('Subject is required.'); return }
    if (isDQA && audience === 'DEPARTMENT' && selectedDepts.length === 0) { setErr('Pick at least one department.'); return }
    if (isDQA && audience === 'SCHOOL' && !audienceValue.trim()) { setErr('Pick a college/school.'); return }
    setSaving(true)
    try {
      const att = file ? await fileToB64(file) : null
      const audValue = !isDQA ? ''
        : audience === 'DEPARTMENT' ? selectedDepts.join('||')
        : audience === 'SCHOOL' ? audienceValue.trim()
        : ''
      await api.post('/api/v1/messages', {
        audience: isDQA ? audience : 'DQA',
        audience_value: audValue,
        subject: subject.trim(),
        body,
        attachment_name: att?.name ?? '',
        attachment_mime: att?.mime ?? '',
        attachment_b64: att?.b64 ?? '',
      })
      setOk(true); setSubject(''); setBody(''); setFile(null); setAudienceValue(''); setSelectedDepts([])
      setTimeout(onSent, 700)
    } catch (e) { setErr(e instanceof Error ? e.message : 'Failed to send') }
    finally { setSaving(false) }
  }

  // Chip options = the known departments plus any custom ones the DQA has already picked.
  const deptChips = [...new Set([...audOpts.departments, ...selectedDepts])]

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
          {audience === 'DEPARTMENT' && (
            <div style={{ marginBottom: 14 }}>
              {deptChips.length > 0 && (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginBottom: 8 }}>
                  {deptChips.map(d => {
                    const on = selectedDepts.includes(d)
                    return (
                      <button key={d} type="button" onClick={() => setSelectedDepts(s => on ? s.filter(x => x !== d) : [...s, d])}
                        style={{ padding: '5px 12px', borderRadius: 999, fontSize: 12, fontWeight: 600, cursor: 'pointer',
                          border: on ? '1px solid var(--brand)' : '1px solid var(--border)',
                          background: on ? 'var(--brand)' : 'transparent', color: on ? '#fff' : 'var(--text)' }}>
                        {on ? '✓ ' : ''}{d}
                      </button>
                    )
                  })}
                </div>
              )}
              <div style={{ display: 'flex', gap: 6 }}>
                <input value={deptInput} onChange={e => setDeptInput(e.target.value)}
                  onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); addDept(deptInput) } }}
                  placeholder="Add a department…" style={{ ...inp, marginBottom: 0, flex: 1 }} />
                <button type="button" onClick={() => addDept(deptInput)} style={smallBtn}>Add</button>
              </div>
              <div style={{ fontSize: 11, color: 'var(--muted)', marginTop: 6 }}>
                {selectedDepts.length > 0 ? `Sending to ${selectedDepts.length} department${selectedDepts.length > 1 ? 's' : ''}: ${selectedDepts.join(', ')}` : 'Tap a department to select — you can pick several.'}
              </div>
            </div>
          )}
          {audience === 'SCHOOL' && (
            <>
              <input list="aud-opts" value={audienceValue} onChange={e => setAudienceValue(e.target.value)}
                placeholder="College / school name" style={inp} />
              <datalist id="aud-opts">{audOpts.schools.map(s => <option key={s} value={s} />)}</datalist>
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
