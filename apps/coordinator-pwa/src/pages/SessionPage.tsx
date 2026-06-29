// Full Session Management UI — drives the XState session machine through all
// states: IDLE → PENDING_LECTURER → ACTIVE → CLOSED/AUTO_CLOSED

import { useState, useEffect, useCallback, useRef, useMemo } from 'react'
import { useMachine } from '@xstate/react'
import { sessionMachine } from '../session/state-machine'
import { useManifest } from '../hooks/useManifest'
import { computeHardwareFingerprint } from '../crypto/fingerprint'
import { validateStudentQR, type ActiveSession, type DeviceContext, type ValidationResult } from '../qr/validator'
import { db, type SessionRecord, type OutboxEntry } from '../db/vault'
import { enqueueSession, processOutboxQueue } from '../sync/outbox'
import { hmacHex } from '../crypto/vault-crypto'
import { startLANValidationHost } from '../sync/lan-host'
import { activateLANServer, deactivateLANServer } from '../sync/lan-server'
import QRCodeLib from 'qrcode'
import { BrandHeader } from '../components/BrandHeader'
import { useAuthStore } from '../store/auth'

const API = import.meta.env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')

interface ServerSession {
  session_id: string
  tenant_id: string
  checkin_code: string
  seconds_remaining: number
  checkin_window_end: string
}

interface RosterStudent {
  student_id:   string
  full_name:    string
  status:       'PRESENT' | 'ABSENT'
  checkin_time: string
  entry_method: string
}

export default function SessionPage({ onGoDashboard }: { onGoDashboard?: () => void }) {
  const [state, send] = useMachine(sessionMachine)
  const { getDecryptedManifest, fetchManifest } = useManifest()
  const [manifest, setManifest] = useState<Record<string, unknown> | null>(null)
  const [selectedUnit, setSelectedUnit] = useState<string | null>(null)
  const [lastScan, setLastScan] = useState<{ status: string; reason?: string } | null>(null)
  const [attendanceCount, setAttendanceCount] = useState(0)
  const [serverSession, setServerSession] = useState<ServerSession | null>(null)
  const [openError, setOpenError] = useState<string | null>(null)
  // When opening is blocked because the coordinator already has a live session,
  // the server returns its id — keep it so we can offer a one-tap "close it".
  const [blockedSessionId, setBlockedSessionId] = useState<string | null>(null)
  const autoTimers = useRef<ReturnType<typeof setTimeout>[]>([])

  // Load manifest on mount (for crypto material + roster). The unit LIST shown
  // comes from the coordinator's cohort overview, not the date-gated manifest, so
  // they always see their course units for their semester.
  useEffect(() => {
    fetchManifest().then(() => getDecryptedManifest().then(setManifest))
  }, [fetchManifest, getDecryptedManifest])

  const [cohortUnits, setCohortUnits] = useState<{ unit_id: string; name: string; year: number; semester: number }[]>([])
  const [cohort, setCohort] = useState<{ course_name: string; study_year: number; semester: number; level: string } | null>(null)
  useEffect(() => {
    const token = useAuthStore.getState().token
    if (!token) return
    fetch(`${API}/api/v1/coordinator/overview`, { headers: { Authorization: `Bearer ${token}` } })
      .then(r => r.ok ? r.json() : null)
      .then((d: { offering: typeof cohort; units: typeof cohortUnits } | null) => {
        if (d) { setCohortUnits(d.units ?? []); setCohort(d.offering) }
      })
      .catch(() => {})
  }, [])

  // Auto-timers when session goes ACTIVE.
  useEffect(() => {
    if (state.value !== 'ACTIVE') return
    autoTimers.current.forEach(clearTimeout)
    autoTimers.current = []

    const now = Date.now()
    const windowEnd  = state.context.checkin_window_end  ? new Date(state.context.checkin_window_end).getTime()  : now + 120 * 60_000
    const autoKill   = state.context.auto_close_time     ? new Date(state.context.auto_close_time).getTime()     : now + 180 * 60_000

    autoTimers.current.push(
      setTimeout(() => send({ type: 'CHECKIN_WINDOW_EXPIRED' }), windowEnd - now),
      setTimeout(() => send({ type: 'AUTO_KILL_EXPIRED' }),      autoKill  - now),
    )

    return () => autoTimers.current.forEach(clearTimeout)
  }, [state.value, send, state.context.checkin_window_end, state.context.auto_close_time])

  // Persist session to IndexedDB when state changes.
  useEffect(() => {
    const ctx = state.context
    if (!ctx.session_id || state.value === 'IDLE') return
    db.sessions.put({
      session_id:   ctx.session_id,
      status:       state.value as SessionRecord['status'],
      date:         new Date().toISOString().split('T')[0],
      unit_id:      ctx.unit_id ?? '',
      unit_name:    ctx.unit_name ?? '',
      venue_id:     ctx.venue_id ?? '',
      gate_open_time: ctx.gate_open_time ?? undefined,
      created_at:   new Date().toISOString(),
    }).catch(console.error)
  }, [state.value, state.context])

  // When session closes → pull any server-side (captive-portal Path B) check-ins
  // into IndexedDB, then seal + enqueue. This makes the coordinator's local store
  // the source of truth for ALL attendance records before the outbox sync fires.
  useEffect(() => {
    if (!((state.value === 'CLOSED' || state.value === 'AUTO_CLOSED') && state.context.session_id)) return
    const sessionId = state.context.session_id
    const hashKey = (manifest as { student_hash_key?: string })?.student_hash_key ?? ''
    const token = useAuthStore.getState().token

    const mergeAndEnqueue = async () => {
      if (serverSession?.session_id && hashKey && token) {
        try {
          const res = await fetch(`${API}/api/v1/sessions/${serverSession.session_id}/roster`, {
            headers: { Authorization: `Bearer ${token}` },
          })
          if (res.ok) {
            const { roster } = await res.json() as { roster: RosterStudent[] }
            for (const s of roster.filter(r => r.status === 'PRESENT')) {
              const hash = await hmacHex(hashKey, s.student_id)
              const exists = await db.attendance_records
                .where('[session_id+student_id_hash]').equals([sessionId, hash]).first()
              if (!exists) {
                const seq = (await db.attendance_records.where('session_id').equals(sessionId).count()) + 1
                await db.attendance_records.add({
                  log_id:                  crypto.randomUUID(),
                  session_id:              sessionId,
                  student_id_hash:         hash,
                  device_fingerprint_hash: 'SERVER_CAPTIVE_PORTAL',
                  sequence_number:         seq,
                  checkin_timestamp:       s.checkin_time || new Date().toISOString(),
                  entry_method:            'QR_SCAN',
                })
              }
            }
          }
        } catch { /* offline — skip merge; Path A records still sync */ }
      }
      await enqueueSession(sessionId)
      // Sync the sealed package right away rather than waiting for background sync.
      await processOutboxQueue().catch(() => {})
    }

    mergeAndEnqueue().catch(console.error)
  }, [state.value, state.context.session_id, manifest, serverSession])

  const handleUnitSelect = useCallback((unit: { unit_id: string; unit_name: string; venue_id: string }) => {
    setSelectedUnit(unit.unit_id)
    send({ type: 'SELECT_UNIT', ...unit })
  }, [send])

  // Shared validation used by both the Coordinator camera path and the LAN
  // student-submit path. Returns the result so the LAN relay can report it back
  // to the student's page.
  // device is supplied by the LAN path (measured on the student's phone). On the
  // Coordinator camera path it is omitted, and we fall back to this device's own
  // fingerprint (the Coordinator is the one physically present).
  const validateRaw = useCallback(async (raw: string, device?: DeviceContext): Promise<ValidationResult> => {
    const ctx = state.context
    if (state.value !== 'ACTIVE' || !ctx.session_id) {
      return { status: 'REJECTED', reason: 'SESSION_NOT_ACTIVE' }
    }

    const activeSession: ActiveSession = {
      sessionId:            ctx.session_id,
      tenantId:             (manifest as { tenant_id?: string })?.tenant_id as string ?? '',
      academicYear:         new Date().getFullYear() + '/' + (new Date().getFullYear() + 1),
      institutionPublicKey: (manifest as { institution_public_key?: string })?.institution_public_key as string ?? '',
      studentHashKey:       (manifest as { student_hash_key?: string })?.student_hash_key ?? '',
      rosterHashes:         getRosterHashes(manifest, ctx.unit_id ?? ''),
      rosterSerials:        getRosterSerials(manifest, ctx.unit_id ?? ''),
    }

    const resolvedDevice: DeviceContext = device ?? {
      fingerprintHash: await computeHardwareFingerprint(),
    }

    const result = await validateStudentQR(raw, activeSession, resolvedDevice)
    setLastScan(result)

    if (result.status === 'PRESENT') {
      send({ type: 'STUDENT_CHECKED_IN' })
      setAttendanceCount(c => c + 1)
    }
    return result
  }, [state, manifest, send])

  // While ACTIVE, expose the validator to the LAN student-submit relay and tell
  // the Service Worker which session the scan page belongs to.
  useEffect(() => {
    if (state.value !== 'ACTIVE' || !state.context.session_id) return
    const stop = startLANValidationHost((raw, _sessionId, device) => validateRaw(raw, device))
    activateLANServer(state.context.session_id).catch(() => {})
    return () => {
      stop()
      deactivateLANServer().catch(() => {})
    }
  }, [state.value, state.context.session_id, validateRaw])

  // ── Render ────────────────────────────────────────────────────────────────

  if (state.value === 'CLOSED' || state.value === 'AUTO_CLOSED') {
    return <SessionClosed status={state.value} count={attendanceCount} onGoDashboard={onGoDashboard} />
  }

  return (
    <div style={{ maxWidth: 480, margin: '0 auto', padding: 16, fontFamily: 'system-ui' }}>
      <BrandHeader />
      <h2 style={{ marginBottom: 4 }}>Session</h2>
      <StatusBadge state={state.value as string} />

      {state.value === 'IDLE' && (
        <UnitSelector
          units={cohortUnits}
          cohort={cohort}
          onSelect={handleUnitSelect}
          selectedUnit={selectedUnit}
        />
      )}

      {state.value === 'PENDING_LECTURER' && (
        <PendingLecturer
          unitId={state.context.unit_id ?? ''}
          unitName={state.context.unit_name ?? ''}
          openError={openError}
          blockedSessionId={blockedSessionId}
          onCloseBlocking={async () => {
            if (!blockedSessionId) return
            const authToken = useAuthStore.getState().token
            await fetch(`${API}/api/v1/sessions/${blockedSessionId}/close`, {
              method: 'POST', headers: { 'Authorization': `Bearer ${authToken}` },
            }).catch(() => {})
            setBlockedSessionId(null)
            setOpenError(null)
          }}
          onManualOpen={async (lecturerId: string) => {
            setOpenError(null)
            setBlockedSessionId(null)
            try {
              const token = useAuthStore.getState().token
              const res = await fetch(`${API}/api/v1/sessions/open`, {
                method: 'POST',
                headers: { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' },
                body: JSON.stringify({
                  unit_id: state.context.unit_id,
                  venue_id: state.context.venue_id,
                  lecturer_id: lecturerId,
                }),
              })
              if (!res.ok) {
                const err = await res.json().catch(() => ({})) as { message?: string; error?: string; session_id?: string }
                setOpenError(err.message ?? `Server error ${res.status}`)
                if (err.error === 'SESSION_ALREADY_OPEN' && err.session_id) setBlockedSessionId(err.session_id)
                return
              }
              const session = await res.json() as ServerSession
              setServerSession(session)
              send({ type: 'LECTURER_GATE_OPEN', lecturer_id: lecturerId || 'MANUAL-OPEN' })
            } catch (e) {
              setOpenError(e instanceof Error ? e.message : 'Network error')
            }
          }}
        />
      )}

      {state.value === 'ACTIVE' && (
        <ActiveSession
          unitName={state.context.unit_name ?? ''}
          count={attendanceCount}
          checkinWindowEnd={serverSession?.checkin_window_end ?? state.context.checkin_window_end}
          lastScan={lastScan}
          onEnd={async () => {
            // Tell the backend to close the session so gate_close_time and
            // contact_hours are recorded in lecturer_attendance_logs.
            if (serverSession?.session_id) {
              const authToken = useAuthStore.getState().token
              await fetch(`${API}/api/v1/sessions/${serverSession.session_id}/close`, {
                method: 'POST',
                headers: { 'Authorization': `Bearer ${authToken}` },
              }).catch(() => {}) // best-effort; local state machine still transitions
            }
            send({ type: 'END_SESSION' })
          }}
          serverSession={serverSession}
          token={useAuthStore.getState().token}
        />
      )}
    </div>
  )
}

// ─── Sub-components ───────────────────────────────────────────────────────────

function StatusBadge({ state }: { state: string }) {
  const colors: Record<string, string> = {
    IDLE: '#94a3b8', PENDING_LECTURER: '#f59e0b',
    ACTIVE: '#22c55e', CLOSED: '#64748b', AUTO_CLOSED: '#ef4444',
  }
  return (
    <div style={{ display: 'flex', gap: 8, marginBottom: 16, alignItems: 'center' }}>
      <span style={{ background: colors[state] ?? '#94a3b8', color: '#fff', padding: '3px 10px', borderRadius: 999, fontSize: 12, fontWeight: 600 }}>
        {state.replace('_', ' ')}
      </span>
    </div>
  )
}

// Shows the coordinator's OWN cohort units (their course, their semester) — always
// available, no daily-window gate. The coordinator picks any unit to start its
// session. Grouped by semester for clarity.
function UnitSelector({ units, cohort, onSelect, selectedUnit }: {
  units: { unit_id: string; name: string; year: number; semester: number }[]
  cohort: { course_name: string; study_year: number; semester: number; level: string } | null
  onSelect: (u: { unit_id: string; unit_name: string; venue_id: string }) => void
  selectedUnit: string | null
}) {
  if (units.length === 0) {
    return (
      <div style={{ background: 'var(--surface-2, #f8fafc)', border: '1px solid var(--border, #e2e8f0)', borderRadius: 12, padding: '24px 20px', textAlign: 'center' }}>
        <div style={{ fontSize: 34 }}>📚</div>
        <h3 style={{ margin: '8px 0 6px', color: 'var(--text, #0f172a)' }}>No units for your cohort yet</h3>
        <p style={{ color: 'var(--muted, #64748b)', fontSize: 14, margin: 0 }}>
          Ask your admin to add units for your level, year and semester.
        </p>
      </div>
    )
  }
  // Group by semester (within the cohort the units are normally one semester).
  const bySem = new Map<number, typeof units>()
  for (const u of units) { const k = u.semester || 0; (bySem.get(k) ?? bySem.set(k, []).get(k)!).push(u) }
  const sems = [...bySem.keys()].sort()

  return (
    <div>
      {cohort && (
        <div style={{ fontSize: 13, color: 'var(--muted,#64748b)', marginBottom: 10 }}>
          {[cohort.course_name, `Year ${cohort.study_year}`, `Sem ${cohort.semester}`, cohort.level].filter(Boolean).join(' · ')}
        </div>
      )}
      <p style={{ color: 'var(--muted,#64748b)', marginBottom: 12 }}>Select a course unit to start its session:</p>
      {sems.map(sem => (
        <div key={sem} style={{ marginBottom: 12 }}>
          {sems.length > 1 && <div style={{ fontSize: 12, fontWeight: 700, color: 'var(--muted,#94a3b8)', marginBottom: 6 }}>Semester {sem}</div>}
          {(bySem.get(sem) ?? []).map(u => (
            <button
              key={u.unit_id}
              onClick={() => onSelect({ unit_id: u.unit_id, unit_name: u.name, venue_id: '' })}
              disabled={selectedUnit === u.unit_id}
              style={{ display: 'block', width: '100%', textAlign: 'left', padding: '12px 14px', marginBottom: 8, borderRadius: 8, border: '1px solid var(--border,#e2e8f0)', background: selectedUnit === u.unit_id ? 'var(--bg,#eff6ff)' : 'var(--surface,#fff)', color: 'inherit', cursor: 'pointer' }}
            >
              <strong>{u.name}</strong>
              <span style={{ float: 'right', color: 'var(--muted,#94a3b8)', fontSize: 13 }}>{u.unit_id}</span>
            </button>
          ))}
        </div>
      ))}
    </div>
  )
}

function PendingLecturer({ unitId, unitName, openError, blockedSessionId, onCloseBlocking, onManualOpen }: {
  unitId: string; unitName: string; openError: string | null
  blockedSessionId: string | null
  onCloseBlocking: () => Promise<void>
  onManualOpen: (lecturerId: string) => Promise<void>
}) {
  const [closingBlocking, setClosingBlocking] = useState(false)
  const [opening,  setOpening]  = useState(false)
  const [lecturers, setLecturers] = useState<{ lecturer_id: string; full_name: string; department: string }[]>([])
  const [selectedLecturer, setSelectedLecturer] = useState('')
  const [loadingLecturers, setLoadingLecturers] = useState(false)
  const token = useAuthStore(s => s.token)

  // Per-unit lecture schedule (#2): set once by the coordinator, then locked.
  const [sched, setSched] = useState<{ session_start: string; session_duration_minutes: number; schedule_locked: boolean } | null>(null)
  const [schedStart, setSchedStart] = useState('')
  const [schedDur, setSchedDur] = useState(60)
  const [schedErr, setSchedErr] = useState<string | null>(null)

  // Fetch lecturers assigned to this unit on mount
  useEffect(() => {
    if (!unitId) return
    setLoadingLecturers(true)
    fetch(`${API}/api/v1/coordinator/units/${unitId}/lecturers`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(r => r.ok ? r.json() : [])
      .then((data: { lecturer_id: string; full_name: string; department: string }[]) => setLecturers(data))
      .catch(() => setLecturers([]))
      .finally(() => setLoadingLecturers(false))
  }, [unitId, token])

  // Fetch the unit schedule
  useEffect(() => {
    if (!unitId) return
    fetch(`${API}/api/v1/coordinator/units/${unitId}/schedule`, { headers: { Authorization: `Bearer ${token}` } })
      .then(r => r.ok ? r.json() : null)
      .then((s: { session_start: string; session_duration_minutes: number; schedule_locked: boolean } | null) => {
        if (!s) return
        setSched(s)
        if (s.session_start) setSchedStart(s.session_start)
        if (s.session_duration_minutes) setSchedDur(s.session_duration_minutes)
      })
      .catch(() => {})
  }, [unitId, token])

  // Save the schedule (if not locked) then open the session.
  async function openWith(lecturerId: string) {
    setSchedErr(null)
    if (!sched?.schedule_locked) {
      if (!schedStart || schedDur < 5) { setSchedErr('Set the lecture start time and length first.'); return }
      const res = await fetch(`${API}/api/v1/coordinator/units/${unitId}/schedule`, {
        method: 'PUT', headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({ session_start: schedStart, session_duration_minutes: schedDur }),
      })
      if (!res.ok && res.status !== 409) {
        const e = await res.json().catch(() => ({})) as { message?: string }
        setSchedErr(e.message ?? 'Could not save schedule'); return
      }
    }
    await onManualOpen(lecturerId)
  }

  return (
    <div>
      <p><strong>{unitName}</strong></p>

      <div style={{ marginBottom: 16 }}>
        <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: '#475569', marginBottom: 6 }}>
          Select Lecturer
        </label>
        {loadingLecturers ? (
          <p style={{ color: 'var(--muted)', fontSize: 13 }}>Loading lecturers…</p>
        ) : (
          <select
            value={selectedLecturer}
            onChange={e => setSelectedLecturer(e.target.value)}
            style={{ width: '100%', padding: '10px 12px', borderRadius: 8, border: '1px solid #e2e8f0', fontSize: 14, background: '#fff', boxSizing: 'border-box' }}
          >
            <option value="">Select lecturer for this session…</option>
            {lecturers.map(l => (
              <option key={l.lecturer_id} value={l.lecturer_id}>
                {l.full_name}{l.department ? ` — ${l.department}` : ''}
              </option>
            ))}
          </select>
        )}
        {!loadingLecturers && lecturers.length === 0 && (
          <p style={{ color: 'var(--muted)', fontSize: 12, marginTop: 4 }}>
            No lecturers assigned to this unit. Use "Open without lecturer" below.
          </p>
        )}
      </div>

      {/* Lecture schedule — set once, then locked */}
      <div style={{ marginBottom: 16, background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 8, padding: 12 }}>
        <div style={{ fontSize: 13, fontWeight: 600, color: '#475569', marginBottom: 8 }}>Lecture schedule</div>
        {sched?.schedule_locked ? (
          <div style={{ fontSize: 13, color: '#0f172a' }}>
            Starts <strong>{sched.session_start}</strong> · {sched.session_duration_minutes} min
            <span style={{ color: 'var(--muted)' }}> · locked (only the admin can change)</span>
          </div>
        ) : (
          <>
            <div style={{ display: 'flex', gap: 8 }}>
              <label style={{ flex: 1 }}>
                <div style={{ fontSize: 12, color: 'var(--muted)', marginBottom: 4 }}>Start time</div>
                <input type="time" value={schedStart} onChange={e => setSchedStart(e.target.value)}
                  style={{ width: '100%', padding: '9px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }} />
              </label>
              <label style={{ flex: 1 }}>
                <div style={{ fontSize: 12, color: 'var(--muted)', marginBottom: 4 }}>Length (min)</div>
                <input type="number" min={5} max={600} value={schedDur} onChange={e => setSchedDur(Number(e.target.value))}
                  style={{ width: '100%', padding: '9px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 14, boxSizing: 'border-box' }} />
              </label>
            </div>
            <div style={{ fontSize: 11, color: 'var(--muted)', marginTop: 6 }}>Set once — it locks after the first session. Only the admin can change it later.</div>
          </>
        )}
        {schedErr && <div style={{ color: '#b91c1c', fontSize: 12, marginTop: 6 }}>{schedErr}</div>}
      </div>

      {openError && (
        <div style={{ background: '#fef2f2', border: '1px solid #fca5a5', borderRadius: 8, padding: 10, marginBottom: 12, color: '#b91c1c', fontSize: 13 }}>
          {openError}
          {blockedSessionId && (
            <button
              disabled={closingBlocking}
              onClick={async () => { setClosingBlocking(true); await onCloseBlocking(); setClosingBlocking(false) }}
              style={{ display: 'block', marginTop: 8, padding: '8px 14px', background: '#dc2626', color: '#fff', border: 'none', borderRadius: 8, fontWeight: 700, fontSize: 13, cursor: closingBlocking ? 'not-allowed' : 'pointer', opacity: closingBlocking ? 0.6 : 1 }}>
              {closingBlocking ? 'Closing…' : 'Close that session now'}
            </button>
          )}
        </div>
      )}

      <button
        disabled={opening || !selectedLecturer}
        onClick={async () => { setOpening(true); await openWith(selectedLecturer); setOpening(false) }}
        style={{ width: '100%', padding: 14, background: '#0f172a', color: '#fff', border: 'none', borderRadius: 8, fontWeight: 700, fontSize: 15, cursor: (!selectedLecturer || opening) ? 'not-allowed' : 'pointer', opacity: (!selectedLecturer || opening) ? 0.5 : 1 }}
      >
        {opening ? 'Opening session…' : 'Open Session'}
      </button>

      <button
        disabled={opening}
        onClick={async () => { setOpening(true); await openWith(''); setOpening(false) }}
        style={{ marginTop: 8, width: '100%', padding: 10, background: 'transparent', color: 'var(--muted)', border: '1px solid #e2e8f0', borderRadius: 8, fontWeight: 500, fontSize: 13, cursor: opening ? 'not-allowed' : 'pointer' }}
      >
        Open without lecturer assignment
      </button>
    </div>
  )
}

function ActiveSession({ unitName, count, checkinWindowEnd, lastScan, onEnd, serverSession, token }: {
  unitName: string; count: number; checkinWindowEnd: string | null
  lastScan: { status: string; reason?: string } | null
  onEnd: () => void
  serverSession: ServerSession | null
  token: string | null
}) {
  const [timeLeft,    setTimeLeft]    = useState('')
  const [roomCode,    setRoomCode]    = useState(serverSession?.checkin_code ?? '')
  const [lecturerQRErr, setLecturerQRErr] = useState<string | null>(null)
  const [showLecturerQR, setShowLecturerQR] = useState(false)
  const [codeSecsLeft,setCodeSecsLeft]= useState(serverSession?.seconds_remaining ?? 15)
  const [liveCount,   setLiveCount]   = useState(count)
  const [roster,      setRoster]      = useState<RosterStudent[]>([])
  const [rosterSearch,setRosterSearch]= useState('')
  const [showRoster,  setShowRoster]  = useState(false)
  const [outbox,      setOutbox]      = useState<OutboxEntry[]>([])
  const [syncing,     setSyncing]     = useState(false)

  // Window countdown
  useEffect(() => {
    if (!checkinWindowEnd) return
    const id = setInterval(() => {
      const ms = new Date(checkinWindowEnd).getTime() - Date.now()
      if (ms <= 0) { setTimeLeft('CLOSED'); clearInterval(id); return }
      const m = Math.floor(ms / 60_000)
      const s = Math.floor((ms % 60_000) / 1000)
      setTimeLeft(`${m}m ${s}s`)
    }, 1000)
    return () => clearInterval(id)
  }, [checkinWindowEnd])

  // Room code + live count polling every 5s
  useEffect(() => {
    if (!serverSession?.session_id) return
    const poll = async () => {
      try {
        const res = await fetch(`${API}/api/v1/sessions/${serverSession.session_id}/checkin-code`, {
          headers: { Authorization: `Bearer ${token}` },
        })
        if (res.ok) {
          const d = await res.json() as { code: string; seconds_remaining: number; checkin_count: number }
          setRoomCode(d.code)
          setCodeSecsLeft(d.seconds_remaining)
          if (typeof d.checkin_count === 'number') setLiveCount(d.checkin_count)
        }
      } catch { /* ignore */ }
    }
    poll()
    const id = setInterval(poll, 5000)
    return () => clearInterval(id)
  }, [serverSession?.session_id, token])

  // Roster polling every 10s when panel is open
  useEffect(() => {
    if (!showRoster || !serverSession?.session_id) return
    const fetchRoster = async () => {
      try {
        const res = await fetch(`${API}/api/v1/sessions/${serverSession.session_id}/roster`, {
          headers: { Authorization: `Bearer ${token}` },
        })
        if (res.ok) {
          const d = await res.json() as { roster: RosterStudent[] }
          setRoster(d.roster ?? [])
        }
      } catch { /* ignore */ }
    }
    fetchRoster()
    const id = setInterval(fetchRoster, 10_000)
    return () => clearInterval(id)
  }, [showRoster, serverSession?.session_id, token])

  // Sync queue status polling every 5s
  useEffect(() => {
    const poll = async () => {
      const entries = await db.outbox_queue.toArray()
      setOutbox(entries)
    }
    poll()
    const id = setInterval(poll, 5000)
    return () => clearInterval(id)
  }, [])

  const filteredRoster = useMemo(() => {
    if (!rosterSearch) return roster
    const q = rosterSearch.toLowerCase()
    return roster.filter(s => s.full_name.toLowerCase().includes(q) || s.student_id.toLowerCase().includes(q))
  }, [roster, rosterSearch])

  const presentCount = roster.filter(s => s.status === 'PRESENT').length

  const checkinURL = serverSession
    ? `${API}/checkin?session=${serverSession.session_id}&tenant=${serverSession.tenant_id}`
    : null

  const pendingSync = outbox.filter(e => e.status === 'PENDING' || e.status === 'UPLOADING').length
  const failedSync  = outbox.filter(e => e.status === 'FAILED').length

  return (
    <div>
      {/* Room code — the big projector display */}
      {serverSession && (
        <div style={{ background: '#0f172a', borderRadius: 12, padding: '20px 24px', marginBottom: 16, textAlign: 'center' }}>
          <div style={{ color: 'var(--muted)', fontSize: 12, marginBottom: 4, letterSpacing: 1 }}>ROOM CODE</div>
          <div style={{ color: '#fff', fontSize: 'clamp(34px, 11vw, 52px)', fontWeight: 800, letterSpacing: 'clamp(6px, 3vw, 12px)', fontFamily: 'monospace' }}>{roomCode}</div>
          <div style={{ color: 'var(--muted)', fontSize: 11, marginTop: 4 }}>changes in {codeSecsLeft}s</div>
        </div>
      )}

      {/* Rotation reminder — a phone hotspot only holds ~10 students at once, so they
          must disconnect after checking in to free a slot for the next student. */}
      {serverSession && (
        <div style={{ background: '#fffbeb', border: '1px solid #f59e0b', borderRadius: 8, padding: '10px 14px', marginBottom: 14, fontSize: 12.5, color: '#92400e', lineHeight: 1.45 }}>
          📴 <strong>Tell students to turn Wi-Fi OFF the moment they see ✓.</strong> Only ~10 phones
          fit on the hotspot at once, so each must disconnect to let the next student check in. Let
          them through in small batches.
        </div>
      )}

      {/* Check-in link */}
      {checkinURL && (
        <div style={{ background: '#f0fdf4', border: '1px solid #86efac', borderRadius: 8, padding: '10px 14px', marginBottom: 14 }}>
          <div style={{ fontSize: 11, color: '#166534', marginBottom: 4, fontWeight: 600 }}>Student check-in link:</div>
          <a href={checkinURL} target="_blank" rel="noreferrer" style={{ fontSize: 12, color: '#15803d', wordBreak: 'break-all' }}>{checkinURL}</a>
        </div>
      )}

      {/* Count + window */}
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 12 }}>
        <div>
          <strong>{unitName}</strong>
          <div style={{ fontSize: 28, fontWeight: 700, color: '#1e293b' }}>{liveCount}</div>
          <div style={{ fontSize: 12, color: 'var(--muted)' }}>students present</div>
        </div>
        <div style={{ textAlign: 'right' }}>
          <div style={{ fontSize: 13, color: 'var(--muted)' }}>Window closes in</div>
          <div style={{ fontSize: 20, fontWeight: 600, color: timeLeft === 'CLOSED' ? '#ef4444' : '#1e293b' }}>{timeLeft}</div>
        </div>
      </div>

      {/* Sync queue status */}
      {(pendingSync > 0 || failedSync > 0) && (
        <div style={{
          display: 'flex', justifyContent: 'space-between', alignItems: 'center',
          background: failedSync > 0 ? '#fef2f2' : '#fffbeb',
          border: `1px solid ${failedSync > 0 ? '#fca5a5' : '#fbbf24'}`,
          borderRadius: 8, padding: '8px 12px', marginBottom: 12, fontSize: 13,
        }}>
          <span style={{ color: failedSync > 0 ? '#b91c1c' : '#92400e' }}>
            {failedSync > 0 ? `⚠ ${failedSync} sync failed` : `↑ ${pendingSync} session(s) pending sync`}
          </span>
          <button
            disabled={syncing}
            onClick={async () => { setSyncing(true); await processOutboxQueue(); setSyncing(false) }}
            style={{ padding: '3px 10px', fontSize: 11, border: '1px solid currentColor', borderRadius: 4, background: 'transparent', cursor: 'pointer', fontWeight: 600 }}
          >
            {syncing ? 'Syncing…' : 'Sync Now'}
          </button>
        </div>
      )}

      {lastScan && (
        <div style={{
          padding: '10px 14px', borderRadius: 8, marginBottom: 12,
          background: lastScan.status === 'PRESENT' ? '#f0fdf4' : '#fef2f2',
          border: `1px solid ${lastScan.status === 'PRESENT' ? '#86efac' : '#fca5a5'}`,
          color: lastScan.status === 'PRESENT' ? '#166534' : '#b91c1c',
          fontWeight: 600,
        }}>
          {lastScan.status === 'PRESENT' ? '✓ Checked in' : `✗ ${lastScan.reason?.replace(/_/g, ' ')}`}
        </div>
      )}

      {/* Students self-scan their own QR with their phone's camera and check in
          on their own device — the coordinator no longer scans student QR codes. */}

      {/* Student roster toggle */}
      {serverSession && (
        <div style={{ marginTop: 14 }}>
          <button
            onClick={() => setShowRoster(v => !v)}
            style={{ width: '100%', padding: '10px', border: '1px solid #e2e8f0', borderRadius: 8, background: '#f8fafc', cursor: 'pointer', fontWeight: 600, fontSize: 14 }}
          >
            {showRoster ? 'Hide' : 'Show'} Student List ({roster.length > 0 ? `${presentCount} present / ${roster.length} enrolled` : 'loading…'})
          </button>

          {showRoster && (
            <div style={{ marginTop: 8, border: '1px solid #e2e8f0', borderRadius: 8, overflow: 'hidden' }}>
              <div style={{ padding: '8px 10px', background: '#f8fafc', borderBottom: '1px solid #e2e8f0' }}>
                <input
                  value={rosterSearch}
                  onChange={e => setRosterSearch(e.target.value)}
                  placeholder="Search students…"
                  style={{ width: '100%', padding: '6px 10px', borderRadius: 6, border: '1px solid #e2e8f0', fontSize: 13, boxSizing: 'border-box' }}
                />
              </div>
              <div style={{ maxHeight: 280, overflowY: 'auto' }}>
                {filteredRoster.length === 0 && (
                  <div style={{ padding: 16, color: 'var(--muted)', textAlign: 'center', fontSize: 13 }}>
                    {roster.length === 0 ? 'Loading roster…' : 'No students match.'}
                  </div>
                )}
                {filteredRoster.map(s => {
                  const present = (s.status as string) === 'PRESENT'
                  return (
                    <div key={s.student_id} style={{
                      display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                      padding: '8px 12px', borderBottom: '1px solid #f1f5f9',
                    }}>
                      <div>
                        <div style={{ fontSize: 13, fontWeight: 600 }}>{s.full_name}</div>
                        <div style={{ fontSize: 11, color: 'var(--muted)' }}>{s.student_id}</div>
                      </div>
                      <span style={{
                        padding: '2px 8px', borderRadius: 999, fontSize: 11, fontWeight: 600,
                        background: present ? '#f0fdf4' : '#f8fafc',
                        color: present ? '#166534' : 'var(--muted)',
                      }}>
                        {present ? '✓ PRESENT' : 'NOT YET'}
                      </span>
                    </div>
                  )
                })}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Lecturer Gate QR — coordinator shows this so the lecturer can scan to prove presence */}
      {serverSession && (
        <div style={{ marginTop: 14 }}>
          <button
            onClick={() => { setLecturerQRErr(null); if (!token) { setLecturerQRErr('Not authenticated'); return } setShowLecturerQR(true) }}
            style={{ width: '100%', padding: 12, background: 'var(--brand)', color: 'var(--brand-contrast, #fff)', border: 'none', borderRadius: 8, fontWeight: 700, fontSize: 14, cursor: 'pointer' }}
          >
            Show Lecturer QR (live)
          </button>
          {lecturerQRErr && (
            <div style={{ marginTop: 6, fontSize: 12, color: '#b91c1c', textAlign: 'center' }}>{lecturerQRErr}</div>
          )}
        </div>
      )}

      {/* Lecturer QR modal — rotates every 10s */}
      {showLecturerQR && token && serverSession && (
        <LecturerQRModal sessionId={serverSession.session_id} token={token} onClose={() => setShowLecturerQR(false)} />
      )}

      <button
        onClick={onEnd}
        style={{ marginTop: 14, width: '100%', padding: 14, background: '#ef4444', color: '#fff', border: 'none', borderRadius: 8, fontWeight: 700, fontSize: 16, cursor: 'pointer' }}
      >
        End Session
      </button>
    </div>
  )
}

function SessionClosed({ status, count, onGoDashboard }: { status: string; count: number; onGoDashboard?: () => void }) {
  return (
    <div style={{ maxWidth: 480, margin: '80px auto', padding: 24, textAlign: 'center', fontFamily: 'system-ui' }}>
      <div style={{ fontSize: 48 }}>✓</div>
      <h2>{status === 'AUTO_CLOSED' ? 'Session auto-closed' : 'Session closed'}</h2>
      <p style={{ color: 'var(--muted)' }}><strong>{count}</strong> students recorded.</p>
      <p style={{ color: '#16a34a' }}>Session sealed and synced.</p>
      {onGoDashboard && (
        <button onClick={onGoDashboard}
          style={{ marginTop: 20, padding: '12px 28px', background: 'var(--brand)', color: 'var(--brand-contrast, #fff)', border: 'none', borderRadius: 10, fontWeight: 700, fontSize: 15, cursor: 'pointer' }}>
          Go to my dashboard →
        </button>
      )}
    </div>
  )
}

// ── Lecturer QR Modal (live, rotates every 10s) ───────────────────────────────
// Re-fetches a fresh signed lecturer-gate token every 10 seconds so a photo of the
// QR expires almost immediately. The lecturer scans the CURRENT code, enters their
// staff ID on the captive portal, and is fingerprint-checked server-side.
function LecturerQRModal({ sessionId, token, onClose }: { sessionId: string; token: string; onClose: () => void }) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const [code, setCode] = useState<string>('······')
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    let active = true
    // The QR is STABLE (it carries only the session_id, like the student QR), so we
    // fetch + render it once. Anti-replay comes from the live digit code below.
    async function loadQR() {
      try {
        const res = await fetch(`${API}/api/v1/sessions/${sessionId}/lecturer-gate-qr`, {
          headers: { Authorization: `Bearer ${token}` },
        })
        if (!res.ok) { setErr('Could not load QR'); return }
        const data = await res.json() as { checkin_url: string }
        if (!active || !canvasRef.current) return
        await QRCodeLib.toCanvas(canvasRef.current, data.checkin_url, {
          width: 320, margin: 2, errorCorrectionLevel: 'H',
          color: { dark: '#000000', light: '#ffffff' },
        })
        setErr(null)
      } catch { setErr('Network error') }
    }
    // The lecturer types this live digit code (the same rotating room code students
    // use), so they must be physically near this screen to read it.
    async function refreshCode() {
      try {
        const res = await fetch(`${API}/api/v1/sessions/${sessionId}/checkin-code`, {
          headers: { Authorization: `Bearer ${token}` },
        })
        if (!res.ok) return
        const d = await res.json() as { code: string }
        if (active && d.code) setCode(d.code)
      } catch { /* keep last */ }
    }
    loadQR(); refreshCode()
    const codePoll = setInterval(refreshCode, 2000)
    return () => { active = false; clearInterval(codePoll) }
  }, [sessionId, token])

  return (
    <div
      onClick={onClose}
      style={{
        position: 'fixed', inset: 0, background: 'rgba(0,0,0,.85)',
        display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
        zIndex: 9999, padding: 24,
      }}
    >
      <div onClick={e => e.stopPropagation()} style={{ background: '#fff', borderRadius: 16, padding: 24, textAlign: 'center', maxWidth: 380, width: '100%' }}>
        <div style={{ fontSize: 13, fontWeight: 700, color: '#0f172a', marginBottom: 12, letterSpacing: .04 }}>
          LECTURER ATTENDANCE QR · LIVE
        </div>
        <canvas ref={canvasRef} style={{ width: '100%', height: 'auto', borderRadius: 8 }} />
        <div style={{ marginTop: 14, padding: '10px 12px', background: '#f1f5f9', borderRadius: 10 }}>
          <div style={{ fontSize: 11, fontWeight: 700, color: 'var(--muted)', letterSpacing: .08 }}>LIVE DIGIT CODE</div>
          <div style={{ fontSize: 30, fontWeight: 800, letterSpacing: 8, color: '#0f172a', fontVariantNumeric: 'tabular-nums' }}>{code}</div>
        </div>
        <p style={{ fontSize: 12, color: 'var(--muted)', marginTop: 8 }}>
          The lecturer scans this QR to <b>begin</b> and again to <b>end</b> the lecture, each time entering their staff ID and the live digit code above (it changes every 10s and is device-verified).
        </p>
        {err && <div style={{ fontSize: 12, color: '#b91c1c', marginTop: 6 }}>{err}</div>}
        <button
          onClick={onClose}
          style={{ marginTop: 16, padding: '10px 24px', background: '#0f172a', color: '#fff', border: 'none', borderRadius: 8, fontWeight: 600, fontSize: 14, cursor: 'pointer' }}
        >
          Close
        </button>
      </div>
    </div>
  )
}

function getRosterHashes(manifest: Record<string, unknown> | null, unitId: string): string[] {
  if (!manifest?.roster) return []
  const roster = (manifest.roster as Record<string, Array<{ student_id_hash: string }>>)[unitId] ?? []
  return roster.map((r) => r.student_id_hash)
}

// Maps each student hash to its current QR serial so Step 4b (SERIAL_REVOKED)
// can reject superseded cards. Without this the map was empty and every scan
// failed the serial check.
function getRosterSerials(manifest: Record<string, unknown> | null, unitId: string): Map<string, string> {
  const map = new Map<string, string>()
  if (!manifest?.roster) return map
  const roster = (manifest.roster as Record<string, Array<{ student_id_hash: string; qr_serial_number: string }>>)[unitId] ?? []
  for (const r of roster) map.set(r.student_id_hash, r.qr_serial_number)
  return map
}
