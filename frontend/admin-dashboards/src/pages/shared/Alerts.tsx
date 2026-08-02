import { useState } from 'react'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

/**
 * The web inbox for cross-role in-app notifications.
 *
 * These alerts already existed and already reached HOD, dean and the QA rep roles — the gateway
 * lists them, and those roles can SEND them from the lecturer pages — but the only inbox ever built
 * was in the phone app. So a head of department could notify every lecturer in their department and
 * then had nowhere to read the reply. This is that missing half.
 *
 * Note the two different systems, deliberately kept apart:
 *   • `/api/v1/messages` — the DQA ⇄ QA-officer channel, with attachments. See Messages.tsx.
 *   • `/api/v1/app-notifications` — cross-role notices between lecturers, coordinators, students
 *     and the org tier. This page.
 *
 * Dismissing is per-recipient: one alert sent to forty lecturers is one row fanned out to forty,
 * and clearing your copy must not clear theirs.
 */

interface Notif {
  notification_id: string
  sender_name: string; sender_role: string
  unit_id: string; subject: string; body: string
  created_at: string; read: boolean
}

export default function Alerts() {
  const { status, data, refetch } = useQuery<Notif[]>(() => api.get('/api/v1/app-notifications'), [])
  const [open, setOpen] = useState<string | null>(null)
  const [hidden, setHidden] = useState<string[]>([])
  const [err, setErr] = useState<string | null>(null)

  const items = (data ?? []).filter(n => !hidden.includes(n.notification_id))

  async function expand(n: Notif) {
    setOpen(o => (o === n.notification_id ? null : n.notification_id))
    if (!n.read) {
      await api.post(`/api/v1/app-notifications/${n.notification_id}/read`).catch(() => {})
      refetch()
    }
  }

  // Hide immediately so the ✕ feels instant, then confirm. A refusal has to be visible — a card
  // that silently reappears reads as "delete is broken" rather than "the server said no".
  async function dismiss(n: Notif) {
    setHidden(h => [...h, n.notification_id])
    setErr(null)
    try {
      await api.delete(`/api/v1/app-notifications/${n.notification_id}`)
      refetch()
    } catch (e) {
      setHidden(h => h.filter(id => id !== n.notification_id))
      setErr(e instanceof Error ? e.message : "Couldn't clear that alert — it is still in your inbox.")
    }
  }

  return (
    <div>
      <h2 style={{ margin: '0 0 4px' }}>Alerts</h2>
      <p style={{ color: 'var(--muted)', margin: '0 0 18px', fontSize: 13 }}>
        Notices sent to you by lecturers, coordinators and the QA office. Clearing one removes it
        from your inbox only.
      </p>

      {err && <div style={errBox}>{err}</div>}
      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}
      {status === 'ok' && items.length === 0 && <p style={{ color: 'var(--muted)' }}>No alerts.</p>}

      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        {items.map(n => {
          const isOpen = open === n.notification_id
          return (
            <div key={n.notification_id} style={{
              position: 'relative', border: '1px solid var(--border)', borderRadius: 10,
              background: 'var(--surface)', overflow: 'hidden',
            }}>
              <button onClick={() => expand(n)} style={{
                width: '100%', textAlign: 'left', cursor: 'pointer', border: 'none',
                background: n.read ? 'transparent' : 'rgba(26,122,63,.06)',
                padding: '12px 40px 12px 14px', color: 'var(--text)',
              }}>
                <span style={{ fontWeight: n.read ? 600 : 700 }}>
                  {!n.read && <span style={dot} />}
                  {n.subject}
                </span>
                <span style={{ display: 'block', fontSize: 12, color: 'var(--muted)', marginTop: 2 }}>
                  From {n.sender_name} ({n.sender_role.replace(/_/g, ' ').toLowerCase()})
                  {n.unit_id ? ` · ${n.unit_id}` : ''} · {new Date(n.created_at).toLocaleString()}
                </span>
              </button>
              <button
                onClick={e => { e.stopPropagation(); dismiss(n) }}
                title="Clear from my inbox" aria-label="Clear from my inbox"
                style={{
                  position: 'absolute', top: 6, right: 6, width: 26, height: 26,
                  border: 'none', background: 'transparent', color: 'var(--muted)',
                  cursor: 'pointer', fontSize: 15, borderRadius: 6, padding: 0,
                }}
              >✕</button>
              {isOpen && (
                <div style={{ padding: '0 14px 14px', borderTop: '1px solid var(--border)' }}>
                  <p style={{ whiteSpace: 'pre-wrap', margin: '12px 0 0', fontSize: 14, lineHeight: 1.5 }}>
                    {n.body || <em style={{ color: 'var(--muted)' }}>(no message body)</em>}
                  </p>
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}

const dot: React.CSSProperties = {
  display: 'inline-block', width: 8, height: 8, borderRadius: 8,
  background: 'var(--brand)', marginRight: 8,
}
const errBox: React.CSSProperties = {
  background: '#fef2f2', color: '#b91c1c', padding: '10px 14px',
  borderRadius: 8, marginBottom: 14, fontSize: 13,
}
