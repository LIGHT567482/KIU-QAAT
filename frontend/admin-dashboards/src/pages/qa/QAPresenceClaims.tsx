import { useCallback, useEffect, useState } from 'react'
import { api } from '../../lib/api'

/**
 * "The patroller never reached my room."
 *
 * A QA patrol tick is one person's account of a moment, timestamped and filed against a named
 * lecturer. Until now it was the only account on file, so a lecturer marked NOT TAUGHT had nothing
 * to answer with but their word, offered days later against a record made at the time.
 *
 * This page is the other record. A lecturer presses one button in the app while standing in the
 * room; the phone captures where it is, when, and which timetabled slot that lands in, and files it
 * offline. This is where it arrives.
 *
 * THE TWO RECORDS SIT ON THE SAME ROW, and that is the whole design. The question a reviewer is
 * answering is never "what did the lecturer say" — it is whether the two accounts agree. Reading
 * them from two screens and lining them up by eye is how a reviewer gets it wrong.
 *
 * Neither record wins automatically and this page does not pretend otherwise. A phone reports the
 * coordinates it is handed. What a claim has going for it is that it is contemporaneous and
 * specific, and that is what the verdict column says — nothing stronger.
 */

interface Claim {
  claim_id:            string
  lecturer_staff_id:   string
  lecturer_name:       string
  latitude:            number | null
  longitude:           number | null
  accuracy_metres:     number | null
  location_status:     'OK' | 'NO_FIX' | 'PERMISSION_DENIED' | 'DISABLED'
  captured_at:         string
  received_at:         string
  unit_id:             string
  unit_name:           string
  room:                string
  scheduled_time:      string
  session_date:        string
  match_kind:          'IN_SLOT' | 'NEAR_SLOT' | 'NEAREST' | 'NONE'
  minutes_from_start:  number | null
  note:                string
  patrol_taught:       boolean | null
  patrol_room:         string
  patrol_taken_at:     string
}

const MATCH_LABEL: Record<Claim['match_kind'], string> = {
  IN_SLOT:   'During the lecture',
  NEAR_SLOT: 'Just outside the slot',
  NEAREST:   'Nothing was running',
  NONE:      'Nothing timetabled',
}

const LOCATION_LABEL: Record<Claim['location_status'], string> = {
  OK:                'Located',
  NO_FIX:            'No fix',
  PERMISSION_DENIED: 'Permission refused',
  DISABLED:          'Location off',
}

/**
 * What the pair of records adds up to. Deliberately descriptive rather than a ruling — the page
 * hands a reviewer the shape of the disagreement, and the reviewer decides.
 */
function verdict(c: Claim): { text: string; tone: 'conflict' | 'agree' | 'neutral' } {
  if (c.patrol_taught === null) return { text: 'No patrol record for this lecture', tone: 'neutral' }
  if (c.patrol_taught === false && c.match_kind === 'IN_SLOT')
    return { text: 'Disagrees — patrol says not taught', tone: 'conflict' }
  if (c.patrol_taught === false) return { text: 'Patrol says not taught', tone: 'conflict' }
  return { text: 'Agrees — patrol says taught', tone: 'agree' }
}

function when(iso: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString(undefined, {
    weekday: 'short', day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit',
  })
}

/** Signed minutes read as English: the number a reviewer actually wants when the patrol log says
 *  14:00 and the claim says 14:07. */
function offset(m: number | null): string {
  if (m === null || m === undefined) return ''
  if (m === 0) return 'on the hour'
  return m > 0 ? `${m} min in` : `${-m} min early`
}

export default function QAPresenceClaims() {
  const [claims, setClaims]   = useState<Claim[] | null>(null)
  const [error, setError]     = useState<string | null>(null)
  const [days, setDays]       = useState(30)
  const [staff, setStaff]     = useState('')
  const [onlyConflicts, setOnlyConflicts] = useState(false)

  const load = useCallback(async () => {
    setError(null)
    try {
      const qs = new URLSearchParams({ days: String(days) })
      if (staff.trim()) qs.set('staff_id', staff.trim())
      setClaims(await api.get<Claim[]>(`/api/v1/dashboard/qa/presence-claims?${qs}`))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not load the records')
      setClaims([])
    }
  }, [days, staff])

  useEffect(() => { load() }, [load])

  const shown = (claims ?? []).filter(c => !onlyConflicts || verdict(c).tone === 'conflict')

  return (
    <div style={{ maxWidth: 1100 }}>
      <h2 style={{ marginBottom: 4 }}>Lecturer presence records</h2>
      <p style={{ color: 'var(--muted)', marginBottom: 20, maxWidth: 760 }}>
        A lecturer's own record of being in the room, captured on their phone at the time — location,
        clock, and the lecture it falls in on the timetable. Filed offline and synced later, so the
        time shown is when they pressed the button, not when it reached us. The patrol tick for the
        same lecture sits beside each one.
      </p>

      <div style={{ display: 'flex', gap: 12, alignItems: 'flex-end', flexWrap: 'wrap', marginBottom: 16 }}>
        <label>
          <div style={lbl}>Staff ID</div>
          <input value={staff} onChange={e => setStaff(e.target.value)} placeholder="all lecturers" style={inp} />
        </label>
        <label>
          <div style={lbl}>Period</div>
          <select value={days} onChange={e => setDays(Number(e.target.value))} style={inp}>
            <option value={7}>Last 7 days</option>
            <option value={30}>Last 30 days</option>
            <option value={90}>Last 90 days</option>
            <option value={365}>Last year</option>
          </select>
        </label>
        <label style={{ display: 'flex', alignItems: 'center', gap: 6, paddingBottom: 8 }}>
          <input type="checkbox" checked={onlyConflicts} onChange={e => setOnlyConflicts(e.target.checked)} />
          <span>Only where the two records disagree</span>
        </label>
        <button onClick={load} style={btn}>Refresh</button>
      </div>

      {error && <div style={errorBox}>{error}</div>}

      {claims === null && <p style={{ color: 'var(--muted)' }}>Loading…</p>}

      {claims !== null && shown.length === 0 && (
        <p style={{ color: 'var(--muted)' }}>
          {onlyConflicts
            ? 'No disagreements in this period — every record here matches its patrol tick.'
            : 'No lecturer has filed a presence record in this period.'}
        </p>
      )}

      {shown.length > 0 && (
        <div style={{ overflowX: 'auto' }}>
          <table style={{ borderCollapse: 'collapse', width: '100%', fontSize: 14 }}>
            <thead>
              <tr>
                {['Lecturer', 'Lecture', 'Lecturer says', 'Location', 'Patrol says', 'Reading'].map(h => (
                  <th key={h} style={th}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {shown.map(c => {
                const v = verdict(c)
                return (
                  <tr key={c.claim_id} style={{ borderTop: '1px solid var(--border)' }}>
                    <td style={td}>
                      <div style={{ fontWeight: 600 }}>{c.lecturer_name || '—'}</div>
                      <div style={sub}>{c.lecturer_staff_id}</div>
                    </td>
                    <td style={td}>
                      <div>{c.unit_name || <em style={{ color: 'var(--muted)' }}>nothing timetabled</em>}</div>
                      <div style={sub}>
                        {[c.session_date, c.scheduled_time, c.room].filter(Boolean).join(' · ')}
                      </div>
                    </td>
                    <td style={td}>
                      <div>{when(c.captured_at)}</div>
                      <div style={sub}>
                        {MATCH_LABEL[c.match_kind]}
                        {offset(c.minutes_from_start) ? ` · ${offset(c.minutes_from_start)}` : ''}
                      </div>
                      {c.note && <div style={{ ...sub, fontStyle: 'italic' }}>“{c.note}”</div>}
                    </td>
                    <td style={td}>
                      {c.location_status === 'OK' && c.latitude !== null && c.longitude !== null ? (
                        <>
                          {/* An external map link, not an embedded map. A map tile provider would be a
                              third party watching which lecturers are under investigation, and this
                              console is deliberately self-contained. */}
                          <a
                            href={`https://www.openstreetmap.org/?mlat=${c.latitude}&mlon=${c.longitude}#map=19/${c.latitude}/${c.longitude}`}
                            target="_blank" rel="noreferrer"
                            style={{ color: 'var(--brand)' }}
                          >
                            {c.latitude.toFixed(5)}, {c.longitude.toFixed(5)}
                          </a>
                          <div style={sub}>
                            ±{Math.round(c.accuracy_metres ?? 0)} m
                          </div>
                        </>
                      ) : (
                        <span style={{ color: 'var(--muted)' }}>{LOCATION_LABEL[c.location_status]}</span>
                      )}
                    </td>
                    <td style={td}>
                      {c.patrol_taught === null
                        ? <span style={{ color: 'var(--muted)' }}>no tick</span>
                        : (
                          <>
                            <div>{c.patrol_taught ? 'Taught' : 'Not taught'}</div>
                            <div style={sub}>
                              {[c.patrol_room, c.patrol_taken_at ? when(c.patrol_taken_at) : ''].filter(Boolean).join(' · ')}
                            </div>
                          </>
                        )}
                    </td>
                    <td style={{ ...td, whiteSpace: 'nowrap' }}>
                      <span style={{
                        ...pill,
                        background: v.tone === 'conflict' ? '#fef2f2' : v.tone === 'agree' ? '#f0fdf4' : 'var(--surface)',
                        color:      v.tone === 'conflict' ? '#b91c1c' : v.tone === 'agree' ? '#166534' : 'var(--muted)',
                      }}>{v.text}</span>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}

      <p style={{ color: 'var(--muted)', fontSize: 13, marginTop: 20, maxWidth: 760 }}>
        A phone reports the coordinates it is given, so this is evidence rather than proof, and it
        does not overturn a patrol tick on its own. What it establishes is that the lecturer made the
        statement <em>at the time</em>, from a fixed point, against the lecture the timetable says
        they were down for. Records cannot be edited or deleted once filed — by anyone.
      </p>
    </div>
  )
}

const th: React.CSSProperties  = { textAlign: 'left', padding: '8px 10px', color: 'var(--muted)', fontWeight: 600, fontSize: 13, whiteSpace: 'nowrap' }
const td: React.CSSProperties  = { padding: '10px', verticalAlign: 'top' }
const sub: React.CSSProperties = { color: 'var(--muted)', fontSize: 12 }
const lbl: React.CSSProperties = { fontSize: 13, color: 'var(--muted)', marginBottom: 4 }
const inp: React.CSSProperties = { padding: '8px 10px', fontSize: 14, borderRadius: 6, border: '1px solid var(--border)', background: 'var(--surface)', color: 'var(--text)' }
const btn: React.CSSProperties = { padding: '9px 16px', fontSize: 14, fontWeight: 600, borderRadius: 6, background: 'var(--brand)', color: 'var(--brand-contrast)', border: 'none', cursor: 'pointer' }
const pill: React.CSSProperties = { padding: '3px 9px', borderRadius: 999, fontSize: 12, fontWeight: 600 }
const errorBox: React.CSSProperties = { background: '#fee2e2', color: '#b91c1c', padding: '10px 14px', borderRadius: 6, marginBottom: 16 }
