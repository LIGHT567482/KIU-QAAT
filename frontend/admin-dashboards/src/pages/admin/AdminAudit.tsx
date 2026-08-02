import { useState } from 'react'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

/**
 * The administrative audit trail.
 *
 * `admin_audit_log` has existed since the very first migration and, until now, NOTHING WROTE TO IT
 * and nothing read it. The table was there, the row-level security policy was there, and every
 * sensitive action — releasing a patroller's handset binding, clearing their PIN, resetting a
 * student's device, deleting an account — happened with no record at all. A trail nobody writes to
 * is worse than none, because from the outside it looks like there is one.
 *
 * Each entry answers the four questions an investigation asks: who, what, to whom, and when. The
 * payload holds the detail that cannot be recovered afterwards — the deleted account's email and
 * role, the reason an officer gave for a device reset — precisely because the row it describes is
 * gone by the time anyone looks.
 */

interface Entry {
  audit_id: string
  actor_id: string; actor_name: string; actor_role: string
  action: string
  target_type: string; target_id: string
  payload: string
  ip_address: string
  occurred_at: string
}
interface Resp { entries: Entry[]; actions: string[] }

// Actions the system records, in plain words. Anything not listed falls back to its raw code, so a
// newly-audited action shows up here immediately rather than waiting on this map.
const LABEL: Record<string, string> = {
  USER_DELETED: 'Deleted an account',
  STUDENT_DEVICE_RESET: 'Reset a student’s device binding',
  PATROL_DEVICE_RELEASED: 'Released a patroller’s handset',
  PATROL_PIN_RESET: 'Cleared a patroller’s PIN',
}

// Which actions are worth a second glance in a list of hundreds.
const SENSITIVE = new Set(['USER_DELETED', 'PATROL_DEVICE_RELEASED', 'PATROL_PIN_RESET'])

export default function AdminAudit() {
  const [filters, setFilters] = useState({ action: '', actor: '', from: '', to: '' })
  const [applied, setApplied] = useState(filters)

  const qs = new URLSearchParams(
    Object.entries(applied).filter(([, v]) => v.trim() !== '') as [string, string][],
  ).toString()
  const { status, data } = useQuery<Resp>(
    () => api.get(`/api/v1/admin/audit${qs ? `?${qs}` : ''}`),
    [qs],
  )

  const entries = data?.entries ?? []

  return (
    <div>
      <h2 style={{ margin: '0 0 4px' }}>Audit trail</h2>
      <p style={{ color: 'var(--muted)', margin: '0 0 18px', fontSize: 13 }}>
        Every sensitive administrative action, newest first. Records are written by the system and
        cannot be edited or removed from here.
      </p>

      <div style={{ display: 'flex', gap: 8, marginBottom: 16, flexWrap: 'wrap' }}>
        <select
          value={filters.action} onChange={e => setFilters(f => ({ ...f, action: e.target.value }))}
          style={inp}
        >
          <option value="">All actions</option>
          {(data?.actions ?? []).map(a => (
            <option key={a} value={a}>{LABEL[a] ?? a.replace(/_/g, ' ').toLowerCase()}</option>
          ))}
        </select>
        <input
          value={filters.actor} onChange={e => setFilters(f => ({ ...f, actor: e.target.value }))}
          placeholder="Who did it — name or id" style={{ ...inp, minWidth: 200 }}
        />
        <label style={dateLabel}>
          From <input type="date" value={filters.from}
            onChange={e => setFilters(f => ({ ...f, from: e.target.value }))} style={inp} />
        </label>
        <label style={dateLabel}>
          To <input type="date" value={filters.to}
            onChange={e => setFilters(f => ({ ...f, to: e.target.value }))} style={inp} />
        </label>
        <button onClick={() => setApplied(filters)} style={btnPrimary}>Apply</button>
        {(applied.action || applied.actor || applied.from || applied.to) && (
          <button
            onClick={() => { const blank = { action: '', actor: '', from: '', to: '' }; setFilters(blank); setApplied(blank) }}
            style={btn}
          >Clear</button>
        )}
      </div>

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}
      {status === 'ok' && entries.length === 0 && (
        <p style={{ color: 'var(--muted)' }}>
          No audited actions {qs ? 'match those filters' : 'have been recorded yet'}.
        </p>
      )}

      {entries.length > 0 && (
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13, minWidth: 760 }}>
            <thead>
              <tr>{['When', 'Who', 'Action', 'Target', 'Detail'].map(h => <th key={h} style={th}>{h}</th>)}</tr>
            </thead>
            <tbody>
              {entries.map(e => (
                <tr key={e.audit_id} style={SENSITIVE.has(e.action) ? { background: 'rgba(185,28,28,.04)' } : undefined}>
                  <td style={{ ...td, whiteSpace: 'nowrap' }}>
                    {new Date(e.occurred_at).toLocaleString()}
                  </td>
                  <td style={td}>
                    {e.actor_name || <span style={{ fontFamily: 'monospace', fontSize: 11 }}>{e.actor_id}</span>}
                    <div style={{ fontSize: 11, color: 'var(--muted)' }}>
                      {e.actor_role.replace(/_/g, ' ')}{e.ip_address ? ` · ${e.ip_address}` : ''}
                    </div>
                  </td>
                  <td style={{ ...td, fontWeight: 600 }}>
                    {LABEL[e.action] ?? e.action.replace(/_/g, ' ').toLowerCase()}
                  </td>
                  <td style={td}>
                    {e.target_id
                      ? <><span style={{ fontSize: 11, color: 'var(--muted)' }}>{e.target_type}</span>
                          <div style={{ fontFamily: 'monospace', fontSize: 11 }}>{e.target_id}</div></>
                      : '—'}
                  </td>
                  <td style={{ ...td, maxWidth: 320 }}><Payload raw={e.payload} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

/** The JSONB payload as readable key/value lines, falling back to the raw text if it isn't an object. */
function Payload({ raw }: { raw: string }) {
  let parsed: unknown
  try { parsed = JSON.parse(raw || '{}') } catch { return <span style={{ fontSize: 12 }}>{raw}</span> }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return <span style={{ fontSize: 12 }}>{String(parsed ?? '')}</span>
  }
  const rows = Object.entries(parsed as Record<string, unknown>)
  if (rows.length === 0) return <span style={{ color: 'var(--muted)' }}>—</span>
  return (
    <div style={{ fontSize: 12 }}>
      {rows.map(([k, v]) => (
        <div key={k}>
          <span style={{ color: 'var(--muted)' }}>{k.replace(/_/g, ' ')}: </span>
          {String(v)}
        </div>
      ))}
    </div>
  )
}

const th: React.CSSProperties = {
  textAlign: 'left', padding: '8px 10px', borderBottom: '2px solid var(--border)',
  fontSize: 11, textTransform: 'uppercase', letterSpacing: '.04em', color: 'var(--muted)',
}
const td: React.CSSProperties = { padding: '8px 10px', borderBottom: '1px solid var(--border)', verticalAlign: 'top' }
const inp: React.CSSProperties = {
  padding: '8px 10px', borderRadius: 8, border: '1px solid var(--border)',
  fontSize: 13, background: 'var(--surface)', color: 'var(--text)',
}
const dateLabel: React.CSSProperties = { fontSize: 12, color: 'var(--muted)', display: 'flex', alignItems: 'center', gap: 6 }
const btn: React.CSSProperties = { ...inp, cursor: 'pointer' }
const btnPrimary: React.CSSProperties = {
  padding: '8px 16px', borderRadius: 8, border: 'none',
  background: 'var(--brand)', color: '#fff', cursor: 'pointer', fontSize: 13, fontWeight: 600,
}
