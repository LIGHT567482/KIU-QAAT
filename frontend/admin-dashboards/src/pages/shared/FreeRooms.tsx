import { useState } from 'react'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

/**
 * WHICH ROOMS ARE FREE, RIGHT NOW.
 *
 * A lecture is timetabled into a room; on the day that room is unusable — double-booked, being
 * repainted, taken by an exam, projector dead — and a class is standing in the corridor. What the
 * coordinator had was the room registry: every room the institution owns, with nothing to say which
 * of them has a lecture in it at this moment. So the choice was made by walking the corridor and
 * looking through doors, and the room they found was often one another college was about to fill.
 *
 * It searches the WHOLE institution, deliberately. The free room is usually somebody else's — the
 * college next door, a different block — and a list scoped to the coordinator's own department
 * would hide exactly the rooms that are available. Grouped by building so "what is free near me"
 * is one glance, and each row names the college it belongs to, because borrowing another faculty's
 * room is a thing you should know you are doing.
 *
 * READ-ONLY, deliberately. Sessions are opened from the coordinator's phone (the in-room hub), not
 * from this console, so a "use this room" button here would be an affordance that leads nowhere.
 * The picking happens where the coordinator is standing; this is the same answer for everyone who
 * has to see, judge or plan around it.
 */

interface Room {
  venue_id: string; name: string; building: string; floor: number
  capacity: number; room_type: string; school: string; department: string
  free: boolean
  occupied_by: string; occupied_until: string; occupied_kind: string; occupied_note: string
}
interface Resp { at: string; minutes: number; total: number; free: number; rooms: Room[] }

export default function FreeRooms() {
  // Blank = "now", which is the question almost every time. The box exists for the coordinator who
  // is looking ahead to the hour their next class starts rather than finding out at the door.
  const [at, setAt] = useState('')
  const [minutes, setMinutes] = useState(60)
  const [onlyFree, setOnlyFree] = useState(true)
  const [search, setSearch] = useState('')

  const qs = `?minutes=${minutes}${at ? `&at=${encodeURIComponent(at)}` : ''}`
  const { data, status, refetch } = useQuery<Resp>(
    () => api.get(`/api/v1/rooms/free${qs}`), [qs])

  const rooms = data?.rooms ?? []
  const sq = search.trim().toLowerCase()
  const visible = rooms.filter(r =>
    (!onlyFree || r.free) &&
    (!sq || [r.name, r.venue_id, r.building, r.school, r.department].some(v => (v || '').toLowerCase().includes(sq))))

  // Grouped by block, because a coordinator with a waiting class cares about distance first.
  const byBuilding = visible.reduce<Record<string, Room[]>>((acc, r) => {
    const k = r.building || 'Unlisted block'
    ;(acc[k] ??= []).push(r)
    return acc
  }, {})

  return (
    <div style={{ color: 'var(--text)' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 12, flexWrap: 'wrap', marginBottom: 12 }}>
        <p style={{ color: 'var(--muted)', margin: 0, fontSize: 13, maxWidth: 640 }}>
          Every room in the institution — all colleges, all departments, all blocks — checked
          against the timetable <em>and</em> against sessions running right now.
          {data && <> <strong style={{ color: 'var(--text)' }}>{data.free}</strong> free of {data.total} at {data.at}.</>}
        </p>
        <button onClick={() => refetch()} style={btn}>Refresh</button>
      </div>

      <div style={{ display: 'flex', gap: 10, alignItems: 'flex-end', flexWrap: 'wrap', marginBottom: 14 }}>
        <label style={lbl}>
          <div style={{ marginBottom: 3 }}>At</div>
          <input type="time" value={at} onChange={e => setAt(e.target.value)} style={inp} />
        </label>
        <label style={lbl}>
          <div style={{ marginBottom: 3 }}>For</div>
          <select value={minutes} onChange={e => setMinutes(Number(e.target.value))} style={inp}>
            {[30, 60, 90, 120, 180].map(m => <option key={m} value={m}>{m} min</option>)}
          </select>
        </label>
        {at && <button onClick={() => setAt('')} style={btn}>Now</button>}
        <label style={{ ...lbl, display: 'flex', alignItems: 'center', gap: 6 }}>
          <input type="checkbox" checked={onlyFree} onChange={e => setOnlyFree(e.target.checked)} />
          <span>Free only</span>
        </label>
        <input value={search} onChange={e => setSearch(e.target.value)}
          placeholder="Search room, block or college…" style={{ ...inp, flex: 1, minWidth: 200 }} />
      </div>

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Checking every room…</p>}
      {status === 'ok' && visible.length === 0 && (
        <div style={{ background: '#fffbeb', border: '1px solid #fde68a', color: '#92400e', borderRadius: 10, padding: 16, fontSize: 13 }}>
          {onlyFree
            ? 'Nothing is free for that window anywhere in the institution. Untick “Free only” to see what is holding each room, and until when.'
            : 'No room matches that search.'}
        </div>
      )}

      {Object.entries(byBuilding).map(([building, list]) => (
        <div key={building} style={{ marginBottom: 18 }}>
          <h4 style={{ margin: '0 0 8px', fontSize: 13, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: .4 }}>
            {building} <span style={{ fontWeight: 400 }}>· {list.filter(r => r.free).length} free of {list.length}</span>
          </h4>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))', gap: 10 }}>
            {list.map(r => (
              <div key={r.venue_id} style={{
                border: '1px solid var(--border,#e2e8f0)', borderRadius: 10, padding: 12,
                background: r.free ? 'var(--surface,#f8fafc)' : 'transparent',
                opacity: r.free ? 1 : .72,
              }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8 }}>
                  <strong style={{ fontSize: 14 }}>{r.name}</strong>
                  <span style={{
                    fontSize: 10.5, fontWeight: 700, padding: '1px 7px', borderRadius: 999, whiteSpace: 'nowrap',
                    background: r.free ? '#dcfce7' : '#fee2e2', color: r.free ? '#166534' : '#b91c1c',
                  }}>{r.free ? 'FREE' : 'IN USE'}</span>
                </div>
                <div style={{ fontSize: 11, color: 'var(--muted)', fontFamily: 'monospace' }}>{r.venue_id}</div>
                <div style={{ fontSize: 11.5, color: 'var(--muted)', marginTop: 4 }}>
                  {r.capacity > 0 && <>seats {r.capacity} · </>}{r.school || 'no college'}
                </div>
                {!r.free && (
                  <div style={{ fontSize: 11.5, marginTop: 6, color: '#b91c1c' }}>
                    {r.occupied_by}
                    {r.occupied_until && <> until {r.occupied_until}</>}
                    {/* A room held by a live session that is ITSELF a provision is the case the
                        timetable cannot explain, and the one a reader is most likely to doubt. */}
                    {r.occupied_note && <> — {r.occupied_note}</>}
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}

const lbl: React.CSSProperties = { fontSize: 12, color: 'var(--muted)' }
const inp: React.CSSProperties = {
  padding: '8px 10px', fontSize: 14, borderRadius: 6, boxSizing: 'border-box',
  border: '1px solid var(--border,#e2e8f0)', background: 'var(--surface,#fff)', color: 'var(--text)',
}
const btn: React.CSSProperties = {
  padding: '8px 12px', background: 'var(--surface,#fff)', color: 'var(--text,#334155)',
  border: '1px solid var(--border,#e2e8f0)', borderRadius: 6, cursor: 'pointer',
  fontWeight: 600, fontSize: 13, whiteSpace: 'nowrap',
}
