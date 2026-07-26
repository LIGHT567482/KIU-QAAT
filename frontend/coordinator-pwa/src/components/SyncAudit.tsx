import { useEffect, useState } from 'react'
import { db, type OutboxEntry } from '../db/vault'
import { processOutboxQueue } from '../sync/outbox'

// Sync Audit (coordinator's app.md Feature 4) — visibility into every session upload:
// status, chunk progress, retries, last error. Reads the local IndexedDB outbox; no
// backend needed. A SYNC_OVERDUE banner flags anything stuck > 48h.
const OVERDUE_MS = 48 * 3600 * 1000

const badge: Record<OutboxEntry['status'], { bg: string; fg: string }> = {
  SYNCED:    { bg: '#dcfce7', fg: '#166534' },
  PENDING:   { bg: '#fef9c3', fg: '#854d0e' },
  UPLOADING: { bg: '#dbeafe', fg: '#1e40af' },
  FAILED:    { bg: '#fee2e2', fg: '#991b1b' },
}

export default function SyncAudit() {
  const [rows, setRows] = useState<OutboxEntry[]>([])
  const [busy, setBusy] = useState(false)

  async function load() { setRows(await db.outbox_queue.orderBy('created_at').reverse().toArray()) }
  useEffect(() => { load(); const t = setInterval(load, 4000); return () => clearInterval(t) }, [])

  const overdue = rows.filter(r => r.status !== 'SYNCED' && Date.now() - Date.parse(r.created_at) > OVERDUE_MS)
  const failed = rows.filter(r => r.status === 'FAILED')

  async function retry() { setBusy(true); try { await processOutboxQueue() } finally { setBusy(false); load() } }

  return (
    <div style={{ background: 'var(--surface, #f8fafc)', border: '1px solid var(--border, #e2e8f0)', borderRadius: 12, padding: 16, marginTop: 16 }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
        <div style={{ fontWeight: 800 }}>Sync audit</div>
        <button onClick={retry} disabled={busy || failed.length === 0}
          style={{ padding: '6px 12px', borderRadius: 8, border: '1px solid var(--border)', background: failed.length ? 'var(--brand)' : 'var(--bg)', color: failed.length ? '#fff' : 'var(--muted)', cursor: failed.length ? 'pointer' : 'default' }}>
          {busy ? 'Retrying…' : `Retry failed (${failed.length})`}
        </button>
      </div>

      {overdue.length > 0 && (
        <div style={{ background: '#fef2f2', border: '1px solid #fecaca', color: '#991b1b', borderRadius: 8, padding: '8px 12px', marginBottom: 10, fontSize: 13 }}>
          ⚠️ {overdue.length} session(s) unsynced for over 48h — connect to the internet to upload.
        </div>
      )}

      {rows.length === 0 ? (
        <div style={{ fontSize: 13, color: 'var(--muted)' }}>No sessions queued for sync yet.</div>
      ) : (
        <div style={{ display: 'grid', gap: 6 }}>
          {rows.map(r => {
            const b = badge[r.status]
            return (
              <div key={r.id ?? r.session_id} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', fontSize: 13, padding: '6px 0', borderBottom: '1px solid var(--border)' }}>
                <div style={{ minWidth: 0 }}>
                  <div style={{ fontWeight: 600 }}>{r.session_id.slice(0, 8)}…</div>
                  <div style={{ color: 'var(--muted)', fontSize: 11 }}>
                    {new Date(r.created_at).toLocaleString()} · chunks {r.chunks_acked.length}/{r.total_chunks} · tries {r.attempt_count}
                    {r.last_error ? ` · ${r.last_error.slice(0, 40)}` : ''}
                  </div>
                </div>
                <span style={{ background: b.bg, color: b.fg, borderRadius: 100, padding: '2px 10px', fontSize: 11, fontWeight: 700, whiteSpace: 'nowrap' }}>{r.status}</span>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
