import Dexie, { type Table } from 'dexie'

// ─── Record types ─────────────────────────────────────────────────────────────

export interface DailyManifest {
  manifest_id: string        // "<date>-<coordinator_id>"
  date: string               // ISO date "2026-05-31"
  coordinator_id: string
  encrypted_blob: string     // AES-256-GCM encrypted JSON (base64)
  fetched_at: string         // ISO timestamp
  expires_at: string
}

export interface SessionRecord {
  session_id: string
  status: 'PENDING_LECTURER' | 'ACTIVE' | 'CLOSED' | 'AUTO_CLOSED'
  date: string
  unit_id: string
  unit_name: string
  venue_id: string
  beacon_uuid: string
  gate_open_time?: string
  gate_close_time?: string
  checkin_window_start?: string
  checkin_window_end?: string
  coordinator_end_time?: string
  auto_close_time?: string
  lecturer_id?: string
  created_at: string
}

export interface AttendanceRecord {
  log_id: string
  session_id: string
  student_id_hash: string    // per-tenant keyed HMAC of registration number — no PII stored
  device_fingerprint_hash: string
  beacon_rssi: number
  sequence_number: number
  checkin_timestamp: string  // ISO
  entry_method: 'QR_SCAN' | 'MANUAL_OVERRIDE'
}

export interface HardwareBinding {
  student_id_hash: string
  fingerprint_hash: string
  academic_year: string
  first_bound_at: string
}

export interface OutboxEntry {
  id?: number               // auto-increment
  session_id: string
  status: 'PENDING' | 'UPLOADING' | 'SYNCED' | 'FAILED'
  upload_id?: string        // assigned by Sync Receiver on init
  chunks_acked: number[]
  total_chunks: number
  attempt_count: number
  last_error?: string
  created_at: string
  synced_at?: string
}

export interface PolicyCache {
  tenant_id: string
  attendance_threshold: number
  checkin_window_minutes: number
  auto_kill_minutes: number
  rssi_threshold_dbm: number
  fetched_at: string
}

// ─── Database ─────────────────────────────────────────────────────────────────

class QAATVault extends Dexie {
  daily_manifest!: Table<DailyManifest>
  sessions!: Table<SessionRecord>
  attendance_records!: Table<AttendanceRecord>
  hardware_vault!: Table<HardwareBinding>
  outbox_queue!: Table<OutboxEntry>
  policy_cache!: Table<PolicyCache>

  constructor() {
    super('QAATVault')
    this.version(1).stores({
      daily_manifest:     'manifest_id, date, coordinator_id',
      sessions:           'session_id, status, date, unit_id',
      attendance_records: 'log_id, session_id, student_id_hash, [session_id+student_id_hash]',
      hardware_vault:     'student_id_hash, academic_year',
      outbox_queue:       '++id, session_id, status, created_at',
      policy_cache:       'tenant_id',
    })
  }
}

export const db = new QAATVault()
