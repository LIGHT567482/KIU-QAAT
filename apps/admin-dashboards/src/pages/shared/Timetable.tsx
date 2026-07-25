import { useMemo, useRef, useState } from 'react'
import { api } from '../../lib/api'
import { useQuery } from '../../lib/useApi'

// Weekly-grid timetable styled after the institution PDFs: a green-themed table,
// Time rows (left) × Mon–Fri columns, each cell showing unit code + name +
// lecturer + room. One timetable per cohort (offering). Supports bulk import.

const KIU_GREEN = '#1a7a3f'
const DAYS = ['', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday']
// A weekend cohort runs Sat–Sun; every other session type runs Mon–Fri.
const daysFor = (sessionType: string): number[] =>
  (sessionType || '').toLowerCase().includes('weekend') ? [6, 7] : [1, 2, 3, 4, 5]
// How many one-hour rows a session covers (08:00–11:00 = 180 min → 3 rows).
const spanOf = (mins: number) => Math.max(1, Math.ceil((mins || 60) / 60))
const TT_COLS = ['course_id', 'level', 'study_year', 'semester', 'session_type', 'unit_id', 'unit_name', 'day', 'start_time', 'duration_minutes', 'room', 'staff_id']

interface Offering {
  offering_id: string; course_id: string; course_name: string; session_type: string
  study_year: number; semester: number; level: string; intake: string; coordinator_name: string
}
interface Slot {
  slot_id: string; offering_id: string; unit_id: string; unit_name: string
  day_of_week: number; start_time: string; duration_minutes: number; room: string
  lecturer_id: string; lecturer_name: string
}
interface OverviewRow { offering_id: string; unit_id: string; unit_name: string }

function downloadText(name: string, content: string) {
  const url = URL.createObjectURL(new Blob([content], { type: 'text/csv' }))
  const a = document.createElement('a'); a.href = url; a.download = name; a.click(); URL.revokeObjectURL(url)
}
const hourOf = (hhmm: string) => parseInt((hhmm || '').split(':')[0] || '0', 10)
const endTime = (start: string, mins: number) => {
  const [h, m] = start.split(':').map(Number)
  const t = h * 60 + m + (mins || 60)
  return `${String(Math.floor(t / 60) % 24).padStart(2, '0')}:${String(t % 60).padStart(2, '0')}`
}

export default function Timetable() {
  const { data, status, refetch } = useQuery<{ offerings: Offering[]; slots: Slot[] }>(
    () => api.get('/api/v1/dashboard/timetable/slots'))
  // Legacy overview gives the full unit list per offering (incl. unscheduled) for the add-slot picker.
  const { data: overview } = useQuery<OverviewRow[]>(() => api.get('/api/v1/dashboard/timetable'))

  const offerings = useMemo(() => data?.offerings ?? [], [data])
  const slots = useMemo(() => data?.slots ?? [], [data])
  const [sel, setSel] = useState('')
  const [search, setSearch] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)
  const [importing, setImporting] = useState(false)
  const [msg, setMsg] = useState<string | null>(null)

  const cohortLabel = (o: Offering) =>
    [o.course_name, o.session_type, `Yr${o.study_year}`, `Sem${o.semester}`, o.level, o.intake].filter(Boolean).join(' · ')

  const filteredOfferings = offerings.filter(o => {
    const q = search.trim().toLowerCase()
    return !q || cohortLabel(o).toLowerCase().includes(q) || o.course_id.toLowerCase().includes(q)
  })
  const current = offerings.find(o => o.offering_id === sel)
  const curSlots = slots.filter(s => s.offering_id === sel)
  const curUnits = (overview ?? []).filter(r => r.offering_id === sel && r.unit_id)

  async function handleImport(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    setImporting(true); setMsg(null)
    try {
      const fd = new FormData(); fd.append('roster', file)
      const tenantId = location.pathname.split('/')[3] // /admin/tenants/:id/...
      const res = await api.upload<{ inserted: number; skipped: number; errors: string[] }>(
        `/api/v1/admin/tenants/${tenantId}/timetable/import`, fd)
      setMsg(`Imported: ${res.inserted} slots${res.skipped ? `, ${res.skipped} skipped` : ''}${res.errors?.length ? ` · ${res.errors.slice(0, 3).join('; ')}` : ''}`)
      refetch()
    } catch (e) { setMsg(e instanceof Error ? `Import failed: ${e.message}` : 'Import failed') }
    finally { setImporting(false); if (fileRef.current) fileRef.current.value = '' }
  }

  // Time rows: evening cohorts run 17:00–21:00, day cohorts 08:00–19:00. Widen to
  // cover any out-of-range imported slot too.
  const rows = useMemo(() => {
    const eve = (current?.session_type || '').toLowerCase().startsWith('eve')
    let lo = eve ? 17 : 8
    let hi = eve ? 21 : 19
    for (const s of curSlots) {
      const h = hourOf(s.start_time)
      if (h < lo) lo = h
      // Widen to cover the WHOLE duration so a multi-hour block has enough rows to span.
      if (h + spanOf(s.duration_minutes) > hi) hi = h + spanOf(s.duration_minutes)
    }
    return Array.from({ length: Math.max(1, hi - lo) }, (_, i) => lo + i)
  }, [current, curSlots])

  return (
    <div>
      <div style={{ marginBottom: 16 }}>
        <a href="/admin/tenants" style={{ color: 'var(--muted)', fontSize: 13, textDecoration: 'none' }}>← Tenants</a>
        <h2 style={{ margin: '4px 0 2px' }}>Timetable</h2>
        <p style={{ color: 'var(--muted)', margin: 0, fontSize: 13 }}>Weekly lecture schedule per cohort. Import the whole institution's timetable, or edit a cohort below.</p>
      </div>

      <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap', marginBottom: 14 }}>
        <input value={search} onChange={e => setSearch(e.target.value)} placeholder="Search a cohort (course, level, year)…"
          style={{ flex: 1, minWidth: 240, maxWidth: 380, padding: '8px 12px', borderRadius: 8, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }} />
        <select value={sel} onChange={e => setSel(e.target.value)} style={sel_}>
          <option value="">— Select a cohort —</option>
          {filteredOfferings.map(o => <option key={o.offering_id} value={o.offering_id}>{cohortLabel(o)}</option>)}
        </select>
        <button onClick={() => downloadText('timetable_template.csv', TT_COLS.join(',') + '\n')} style={btnGhost} title="Download a blank CSV with the timetable columns">Template</button>
        <input ref={fileRef} type="file" accept=".csv,text/csv,.xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" onChange={handleImport} style={{ display: 'none' }} />
        <button onClick={() => fileRef.current?.click()} disabled={importing} style={btnGhost}>{importing ? 'Importing…' : 'Import timetable'}</button>
      </div>

      {msg && <div style={{ background: msg.startsWith('Import failed') ? '#fef2f2' : '#f0fdf4', color: msg.startsWith('Import failed') ? '#b91c1c' : '#166534', padding: '10px 14px', borderRadius: 8, marginBottom: 14, fontSize: 13 }}>{msg}</div>}

      {status === 'loading' && <p style={{ color: 'var(--muted)' }}>Loading…</p>}

      {!current ? (
        <div style={{ padding: '28px 0', color: 'var(--muted)', textAlign: 'center' }}>
          {offerings.length === 0 ? 'No cohorts yet. Import a timetable or add offerings first.' : 'Select a cohort above to view its weekly timetable.'}
        </div>
      ) : (
        <CohortTimetable offering={current} slots={curSlots} units={curUnits} rows={rows} onChanged={refetch} cohortLabel={cohortLabel(current)} />
      )}
    </div>
  )
}

function CohortTimetable({ offering, slots, units, rows, onChanged, cohortLabel }: {
  offering: Offering; slots: Slot[]; units: OverviewRow[]; rows: number[]; onChanged: () => void; cohortLabel: string
}) {
  const [adding, setAdding] = useState(false)
  // Weekend cohorts show Sat–Sun columns; everyone else Mon–Fri.
  const days = daysFor(offering.session_type)
  // Pre-compute where each slot STARTS and which lower cells it COVERS, so a multi-hour
  // block renders as a single cell spanning several rows (rowSpan) instead of one cell.
  const covered = new Set<string>()
  const startAt = new Map<string, Slot[]>()
  for (const s of slots) {
    const sh = hourOf(s.start_time)
    const k = `${s.day_of_week}-${sh}`
    startAt.set(k, [...(startAt.get(k) ?? []), s])
    for (let i = 1; i < spanOf(s.duration_minutes); i++) covered.add(`${s.day_of_week}-${sh + i}`)
  }

  async function del(slotId: string) {
    if (!confirm('Remove this lecture from the timetable?')) return
    try { await api.delete(`/api/v1/dashboard/timetable/slots/${slotId}`); onChanged() }
    catch (e) { alert(e instanceof Error ? e.message : 'Failed') }
  }

  return (
    <div>
      {/* KIU-style header band — white sheet so the timetable sits as its own card
          on top of whatever background colour the super-admin set for the tenant. */}
      <div style={{ background: '#fff', color: '#0f172a', border: `2px solid ${KIU_GREEN}`, borderRadius: 10, overflow: 'hidden' }}>
        <div style={{ textAlign: 'center', padding: '10px 12px', color: KIU_GREEN }}>
          <div style={{ fontWeight: 800, fontSize: 15 }}>Lectures Timetable</div>
          <div style={{ fontSize: 13 }}>{cohortLabel}</div>
          <div style={{ fontSize: 11, color: 'var(--muted)' }}>Session: {offering.session_type}{offering.coordinator_name ? ` · Coordinator: ${offering.coordinator_name}` : ''}</div>
        </div>
        <table style={{ width: '100%', borderCollapse: 'collapse', tableLayout: 'fixed' }}>
          <thead>
            <tr style={{ background: KIU_GREEN, color: '#fff' }}>
              <th style={{ ...th, width: 76 }}>Time</th>
              {days.map(d => <th key={d} style={th}>{DAYS[d]}</th>)}
            </tr>
          </thead>
          <tbody>
            {rows.map(hour => (
              <tr key={hour}>
                <td style={timeCell}>{String(hour).padStart(2, '0')}:00<br />{String(hour + 1).padStart(2, '0')}:00</td>
                {days.map(d => {
                  const key = `${d}-${hour}`
                  if (covered.has(key)) return null // a block above spans into this cell
                  const here = startAt.get(key) ?? []
                  const span = here.length ? Math.max(...here.map(s => spanOf(s.duration_minutes))) : 1
                  return (
                    <td key={d} style={cell} rowSpan={span}>
                      {here.map(s => (
                        <div key={s.slot_id} style={card} title="Click × to remove">
                          <button onClick={() => del(s.slot_id)} style={delBtn}>×</button>
                          <div style={{ fontWeight: 800, color: KIU_GREEN, fontSize: 12 }}>{s.unit_id}</div>
                          <div style={{ fontSize: 12, fontWeight: 600 }}>{s.unit_name}</div>
                          {s.lecturer_name && <div style={{ fontSize: 11, color: '#475569' }}>Lecturer: {s.lecturer_name}</div>}
                          <div style={{ fontSize: 11, color: 'var(--muted)' }}>{s.start_time}–{endTime(s.start_time, s.duration_minutes)}{s.room ? ` · Room: ${s.room}` : ''}</div>
                        </div>
                      ))}
                    </td>
                  )
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div style={{ marginTop: 12 }}>
        {adding
          ? <AddSlot offering={offering} units={units} onDone={() => { setAdding(false); onChanged() }} onCancel={() => setAdding(false)} />
          : <button onClick={() => setAdding(true)} style={btnGhost} disabled={units.length === 0}>+ Add a lecture slot</button>}
        {units.length === 0 && <span style={{ marginLeft: 10, color: 'var(--muted)', fontSize: 12 }}>This cohort has no units yet — add units (or import a timetable) first.</span>}
      </div>
    </div>
  )
}

function AddSlot({ offering, units, onDone, onCancel }: {
  offering: Offering; units: OverviewRow[]; onDone: () => void; onCancel: () => void
}) {
  const ttDays = daysFor(offering.session_type)
  const [unit, setUnit] = useState('')
  const [day, setDay] = useState(ttDays[0])
  const [start, setStart] = useState(offering.session_type.toLowerCase().startsWith('eve') ? '17:00' : '08:00')
  const [dur, setDur] = useState(60)
  const [room, setRoom] = useState('')
  const [saving, setSaving] = useState(false)
  const uniqUnits = Array.from(new Map(units.map(u => [u.unit_id, u])).values())

  async function save() {
    if (!unit) return
    setSaving(true)
    try {
      await api.put('/api/v1/dashboard/timetable/slots', {
        offering_id: offering.offering_id, unit_id: unit, day_of_week: day,
        start_time: start, duration_minutes: dur, room,
      })
      onDone()
    } catch (e) { alert(e instanceof Error ? e.message : 'Failed') }
    finally { setSaving(false) }
  }

  return (
    <div style={{ background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 10, padding: 14, display: 'flex', gap: 10, alignItems: 'flex-end', flexWrap: 'wrap' }}>
      <Field label="Unit"><select value={unit} onChange={e => setUnit(e.target.value)} style={sel_}><option value="">— select —</option>{uniqUnits.map(u => <option key={u.unit_id} value={u.unit_id}>{u.unit_id} — {u.unit_name}</option>)}</select></Field>
      <Field label="Day"><select value={day} onChange={e => setDay(Number(e.target.value))} style={sel_}>{ttDays.map(d => <option key={d} value={d}>{DAYS[d]}</option>)}</select></Field>
      <Field label="Start"><input type="time" value={start} onChange={e => setStart(e.target.value)} style={inp} /></Field>
      <Field label="Minutes"><input type="number" min={15} max={300} value={dur} onChange={e => setDur(Number(e.target.value))} style={{ ...inp, width: 80 }} /></Field>
      <Field label="Room"><input value={room} onChange={e => setRoom(e.target.value)} placeholder="e.g. C01 O.B." style={{ ...inp, width: 120 }} /></Field>
      <button onClick={save} disabled={!unit || saving} style={{ ...btnGhost, background: KIU_GREEN, color: '#fff', borderColor: KIU_GREEN }}>{saving ? 'Saving…' : 'Add'}</button>
      <button onClick={onCancel} style={btnGhost}>Cancel</button>
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <label style={{ display: 'block' }}><div style={{ fontSize: 11, fontWeight: 700, color: 'var(--muted)', marginBottom: 3 }}>{label}</div>{children}</label>
}

const th: React.CSSProperties = { padding: '8px 10px', textAlign: 'center', fontSize: 13, borderRight: '1px solid rgba(255,255,255,.25)' }
const timeCell: React.CSSProperties = { padding: '6px 8px', textAlign: 'center', fontSize: 11, fontWeight: 700, color: KIU_GREEN, background: '#f0fdf4', borderBottom: '1px solid #e2e8f0', borderRight: `1px solid ${KIU_GREEN}`, whiteSpace: 'nowrap', verticalAlign: 'top' }
const cell: React.CSSProperties = { padding: 5, borderBottom: '1px solid #e2e8f0', borderRight: '1px solid #e2e8f0', verticalAlign: 'top', height: 64 }
const card: React.CSSProperties = { position: 'relative', background: '#fff', border: `1px solid ${KIU_GREEN}`, borderLeft: `4px solid ${KIU_GREEN}`, borderRadius: 6, padding: '5px 7px', marginBottom: 5 }
const delBtn: React.CSSProperties = { position: 'absolute', top: 2, right: 4, border: 'none', background: 'none', color: 'var(--muted)', cursor: 'pointer', fontSize: 15, lineHeight: 1 }
const sel_: React.CSSProperties = { padding: '8px 10px', borderRadius: 8, border: '1px solid #e2e8f0', fontSize: 14, background: '#fff', minWidth: 220 }
const inp: React.CSSProperties = { padding: '8px 10px', borderRadius: 8, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }
const btnGhost: React.CSSProperties = { padding: '8px 14px', background: '#fff', border: '1px solid #e2e8f0', borderRadius: 8, cursor: 'pointer', fontSize: 13, fontWeight: 600, color: '#1e293b' }
