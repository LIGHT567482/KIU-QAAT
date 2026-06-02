// Typed API client — injects Bearer token from session storage automatically.

const BASE = import.meta.env.VITE_API_URL ?? 'http://localhost:8443'

function getToken(): string {
  try {
    const raw = sessionStorage.getItem('qaat_admin_token')
    if (!raw) return ''
    return (JSON.parse(raw) as { token: string }).token ?? ''
  } catch {
    return ''
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${getToken()}`,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw Object.assign(new Error((err as { message?: string }).message ?? `HTTP ${res.status}`), { status: res.status, code: (err as { error?: string }).error })
  }
  return res.json() as Promise<T>
}

export const api = {
  get:  <T>(path: string)                    => request<T>('GET',  path),
  put:  <T>(path: string, body: unknown)     => request<T>('PUT',  path, body),
  post: <T>(path: string, body?: unknown)    => request<T>('POST', path, body),
}

// ─── Response types ───────────────────────────────────────────────────────────

export interface ThresholdConfig {
  attendance_threshold:    number
  checkin_window_minutes:  number
  auto_kill_minutes:       number
  rssi_threshold_dbm:      number
}

export interface EligibilityUnit {
  unit_id:              string
  unit_name:            string
  sessions_held:        number
  sessions_attended:    number
  attendance_percentage: number
  threshold:            number
  status:               'ELIGIBLE' | 'EXAM_INELIGIBLE'
  deficit_sessions?:    number
}

export interface EligibilityRecord {
  student_id:    string
  academic_year: string
  semester:      number
  units:         EligibilityUnit[]
}

export interface LiveSession {
  session_id:       string
  coordinator_id:   string
  unit_id:          string
  unit_name:        string
  venue_id:         string
  session_status:   string
  student_count:    number
  gate_open_time:   string
  audit_flags:      string[]
}

export interface VCOverview {
  total_sessions_today:  number
  total_students_today:  number
  ghost_lecture_count:   number
  avg_attendance_pct:    number
  eligibility_summary: {
    eligible:   number
    ineligible: number
    pending:    number
  }
}
