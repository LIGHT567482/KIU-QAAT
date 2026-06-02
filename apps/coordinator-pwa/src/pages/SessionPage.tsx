// Full Session Management UI — drives the XState session machine through all
// states: IDLE → PENDING_LECTURER → ACTIVE → CLOSED/AUTO_CLOSED

import { useState, useEffect, useCallback, useRef } from 'react'
import { useMachine } from '@xstate/react'
import { sessionMachine } from '../session/state-machine'
import { useManifest } from '../hooks/useManifest'
import { startBLEScan, stopBLEScan, isBLEBeaconDetected, isBLESupported, getBLERollingAverage } from '../ble/scanner'
import { computeHardwareFingerprint } from '../crypto/fingerprint'
import { validateStudentQR, type ActiveSession, type DeviceContext, type ValidationResult } from '../qr/validator'
import { db, type SessionRecord } from '../db/vault'
import { enqueueSession } from '../sync/outbox'
import { startLANValidationHost } from '../sync/lan-host'
import { activateLANServer, deactivateLANServer } from '../sync/lan-server'
import QRScanner from '../components/QRScanner'

export default function SessionPage() {
  const [state, send] = useMachine(sessionMachine)
  const { getDecryptedManifest, fetchManifest, status: manifestStatus } = useManifest()
  const [manifest, setManifest] = useState<Record<string, unknown> | null>(null)
  const [selectedUnit, setSelectedUnit] = useState<string | null>(null)
  const [lastScan, setLastScan] = useState<{ status: string; reason?: string } | null>(null)
  const [attendanceCount, setAttendanceCount] = useState(0)
  const [bleStatus, setBleStatus] = useState<'idle' | 'scanning' | 'detected' | 'lost'>('idle')
  const bleInterval = useRef<ReturnType<typeof setInterval> | null>(null)
  const autoTimers = useRef<ReturnType<typeof setTimeout>[]>([])

  // Load manifest on mount.
  useEffect(() => {
    fetchManifest().then(() => getDecryptedManifest().then(setManifest))
  }, [fetchManifest, getDecryptedManifest])

  // BLE scanning tick — check beacon presence every 2 seconds.
  useEffect(() => {
    const beaconUUID = state.context.beacon_uuid
    if (!beaconUUID || state.value === 'IDLE') return

    if (!isBLESupported()) {
      setBleStatus('detected')  // GPS-only fallback for unsupported browsers (plan.md risk register)
      send({ type: 'BLE_DETECTED' })
      return
    }

    startBLEScan(beaconUUID).catch(console.error)
    setBleStatus('scanning')

    bleInterval.current = setInterval(() => {
      const threshold = state.context.policy.rssi_threshold_dbm
      if (isBLEBeaconDetected(beaconUUID, threshold)) {
        setBleStatus('detected')
        if (state.value === 'PENDING_LECTURER') {
          send({ type: 'BLE_DETECTED' })
        }
      } else {
        setBleStatus('lost')
      }
    }, 2000)

    return () => {
      clearInterval(bleInterval.current!)
      stopBLEScan()
    }
  }, [state.context.beacon_uuid, state.value, send, state.context.policy])

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
      beacon_uuid:  ctx.beacon_uuid ?? '',
      gate_open_time: ctx.gate_open_time ?? undefined,
      created_at:   new Date().toISOString(),
    }).catch(console.error)
  }, [state.value, state.context])

  // When session closes → seal + enqueue.
  useEffect(() => {
    if ((state.value === 'CLOSED' || state.value === 'AUTO_CLOSED') && state.context.session_id) {
      enqueueSession(state.context.session_id).catch(console.error)
    }
  }, [state.value, state.context.session_id])

  const handleUnitSelect = useCallback((unit: { unit_id: string; unit_name: string; venue_id: string; beacon_uuid: string }) => {
    setSelectedUnit(unit.unit_id)
    send({ type: 'SELECT_UNIT', ...unit })
  }, [send])

  // Shared validation used by both the Coordinator camera path and the LAN
  // student-submit path. Returns the result so the LAN relay can report it back
  // to the student's page.
  // device is supplied by the LAN path (measured on the student's phone). On the
  // Coordinator camera path it is omitted, and we fall back to this device's own
  // fingerprint + measured RSSI (the Coordinator is the one physically present).
  const validateRaw = useCallback(async (raw: string, device?: DeviceContext): Promise<ValidationResult> => {
    const ctx = state.context
    if (state.value !== 'ACTIVE' || !ctx.session_id) {
      return { status: 'REJECTED', reason: 'SESSION_NOT_ACTIVE' }
    }

    const activeSession: ActiveSession = {
      sessionId:            ctx.session_id,
      tenantId:             (manifest as { tenant_id?: string })?.tenant_id as string ?? '',
      beaconUUID:           ctx.beacon_uuid ?? '',
      academicYear:         new Date().getFullYear() + '/' + (new Date().getFullYear() + 1),
      institutionPublicKey: (manifest as { institution_public_key?: string })?.institution_public_key as string ?? '',
      studentHashKey:       (manifest as { student_hash_key?: string })?.student_hash_key ?? '',
      rosterHashes:         getRosterHashes(manifest, ctx.unit_id ?? ''),
      rosterSerials:        getRosterSerials(manifest, ctx.unit_id ?? ''),
      policy:               { rssiThresholdDBM: ctx.policy.rssi_threshold_dbm },
    }

    const resolvedDevice: DeviceContext = device ?? {
      fingerprintHash: await computeHardwareFingerprint(),
      rssi:            getBLERollingAverage(ctx.beacon_uuid ?? '', 10_000),
    }

    const result = await validateStudentQR(raw, activeSession, resolvedDevice)
    setLastScan(result)

    if (result.status === 'PRESENT') {
      send({ type: 'STUDENT_CHECKED_IN' })
      setAttendanceCount(c => c + 1)
    }
    return result
  }, [state, manifest, send])

  const handleQRScan = useCallback((raw: string) => { void validateRaw(raw) }, [validateRaw])

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
    return <SessionClosed status={state.value} count={attendanceCount} />
  }

  return (
    <div style={{ maxWidth: 480, margin: '0 auto', padding: 16, fontFamily: 'system-ui' }}>
      <h2 style={{ marginBottom: 4 }}>Session</h2>
      <StatusBadge state={state.value as string} bleStatus={bleStatus} />

      {state.value === 'IDLE' && (
        <UnitSelector
          manifest={manifest}
          manifestStatus={manifestStatus}
          onSelect={handleUnitSelect}
          selectedUnit={selectedUnit}
        />
      )}

      {state.value === 'PENDING_LECTURER' && (
        <PendingLecturer
          unitName={state.context.unit_name ?? ''}
          bleStatus={bleStatus}
          onLecturerScan={(lecturerId) => send({ type: 'LECTURER_GATE_OPEN', lecturer_id: lecturerId })}
        />
      )}

      {state.value === 'ACTIVE' && (
        <ActiveSession
          unitName={state.context.unit_name ?? ''}
          count={attendanceCount}
          checkinWindowEnd={state.context.checkin_window_end}
          lastScan={lastScan}
          onScan={handleQRScan}
          onEnd={() => send({ type: 'END_SESSION' })}
        />
      )}
    </div>
  )
}

// ─── Sub-components ───────────────────────────────────────────────────────────

function StatusBadge({ state, bleStatus }: { state: string; bleStatus: string }) {
  const colors: Record<string, string> = {
    IDLE: '#94a3b8', PENDING_LECTURER: '#f59e0b',
    ACTIVE: '#22c55e', CLOSED: '#64748b', AUTO_CLOSED: '#ef4444',
  }
  return (
    <div style={{ display: 'flex', gap: 8, marginBottom: 16, alignItems: 'center' }}>
      <span style={{ background: colors[state] ?? '#94a3b8', color: '#fff', padding: '3px 10px', borderRadius: 999, fontSize: 12, fontWeight: 600 }}>
        {state.replace('_', ' ')}
      </span>
      <span style={{ fontSize: 12, color: bleStatus === 'detected' ? '#22c55e' : '#f59e0b' }}>
        BLE {bleStatus}
      </span>
    </div>
  )
}

function UnitSelector({ manifest, manifestStatus, onSelect, selectedUnit }: {
  manifest: Record<string, unknown> | null
  manifestStatus: string
  onSelect: (u: { unit_id: string; unit_name: string; venue_id: string; beacon_uuid: string }) => void
  selectedUnit: string | null
}) {
  const sessions = (manifest?.sessions as Array<{ unit_id: string; unit_name: string; venue_id: string; beacon_uuid: string }>) ?? []

  if (manifestStatus === 'fetching') return <p style={{ color: '#64748b' }}>Fetching today's manifest…</p>
  if (manifestStatus === 'error' && sessions.length === 0) return <p style={{ color: '#b91c1c' }}>No manifest. Ensure internet connection for first fetch.</p>

  return (
    <div>
      <p style={{ color: '#64748b', marginBottom: 12 }}>Select a course unit to start:</p>
      {sessions.map(s => (
        <button
          key={s.unit_id}
          onClick={() => onSelect(s)}
          disabled={selectedUnit === s.unit_id}
          style={{ display: 'block', width: '100%', textAlign: 'left', padding: '12px 14px', marginBottom: 8, borderRadius: 8, border: '1px solid #e2e8f0', background: selectedUnit === s.unit_id ? '#eff6ff' : '#fff', cursor: 'pointer' }}
        >
          <strong>{s.unit_name}</strong>
          <span style={{ float: 'right', color: '#94a3b8', fontSize: 13 }}>{s.unit_id}</span>
        </button>
      ))}
      {sessions.length === 0 && <p style={{ color: '#64748b' }}>No sessions scheduled for today.</p>}
    </div>
  )
}

function PendingLecturer({ unitName, bleStatus, onLecturerScan }: {
  unitName: string; bleStatus: string
  onLecturerScan: (id: string) => void
}) {
  return (
    <div>
      <p><strong>{unitName}</strong></p>
      <p style={{ color: '#64748b' }}>Waiting for lecturer to scan Gate-Open QR…</p>
      {bleStatus !== 'detected' && (
        <div style={{ background: '#fffbeb', border: '1px solid #fbbf24', borderRadius: 8, padding: 12, marginBottom: 12 }}>
          BLE beacon not yet detected. Ensure you are in the venue.
        </div>
      )}
      <QRScanner active={bleStatus === 'detected'} onScan={onLecturerScan} />
    </div>
  )
}

function ActiveSession({ unitName, count, checkinWindowEnd, lastScan, onScan, onEnd }: {
  unitName: string; count: number; checkinWindowEnd: string | null
  lastScan: { status: string; reason?: string } | null
  onScan: (raw: string) => void; onEnd: () => void
}) {
  const [timeLeft, setTimeLeft] = useState('')
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

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 12 }}>
        <div>
          <strong>{unitName}</strong>
          <div style={{ fontSize: 28, fontWeight: 700, color: '#1e293b' }}>{count}</div>
          <div style={{ fontSize: 12, color: '#64748b' }}>students present</div>
        </div>
        <div style={{ textAlign: 'right' }}>
          <div style={{ fontSize: 13, color: '#64748b' }}>Window closes in</div>
          <div style={{ fontSize: 20, fontWeight: 600, color: timeLeft === 'CLOSED' ? '#ef4444' : '#1e293b' }}>{timeLeft}</div>
        </div>
      </div>

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

      <QRScanner active onScan={onScan} />

      <button
        onClick={onEnd}
        style={{ marginTop: 16, width: '100%', padding: 14, background: '#ef4444', color: '#fff', border: 'none', borderRadius: 8, fontWeight: 700, fontSize: 16, cursor: 'pointer' }}
      >
        End Session
      </button>
    </div>
  )
}

function SessionClosed({ status, count }: { status: string; count: number }) {
  return (
    <div style={{ maxWidth: 480, margin: '80px auto', padding: 24, textAlign: 'center', fontFamily: 'system-ui' }}>
      <div style={{ fontSize: 48 }}>✓</div>
      <h2>{status === 'AUTO_CLOSED' ? 'Session auto-closed' : 'Session closed'}</h2>
      <p style={{ color: '#64748b' }}><strong>{count}</strong> students recorded.</p>
      <p style={{ color: '#64748b' }}>Session sealed and queued for sync.</p>
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
