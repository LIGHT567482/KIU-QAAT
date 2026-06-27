// Typed API client — injects Bearer token from session storage automatically.

const BASE = import.meta.env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')

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
  // 204 No Content (e.g. DELETE) or any empty body → don't try to parse JSON,
  // which would throw and make a successful call look like a failure.
  if (res.status === 204) return undefined as unknown as T
  const text = await res.text()
  return (text ? JSON.parse(text) : undefined) as T
}

async function upload<T>(path: string, form: FormData): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${getToken()}` }, // no Content-Type → browser sets multipart boundary
    body: form,
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw Object.assign(new Error((err as { message?: string }).message ?? `HTTP ${res.status}`), { status: res.status, code: (err as { error?: string }).error })
  }
  return res.json() as Promise<T>
}

// Authenticated file download (e.g. XLSX export) — fetches a blob with the bearer
// token and triggers a browser download. A plain <a href> can't send the header.
async function download(path: string, filename: string): Promise<void> {
  const res = await fetch(`${BASE}${path}`, { headers: { Authorization: `Bearer ${getToken()}` } })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  const blob = await res.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a'); a.href = url; a.download = filename; a.click(); URL.revokeObjectURL(url)
}

export const api = {
  get:    <T>(path: string)                => request<T>('GET',    path),
  put:    <T>(path: string, body: unknown) => request<T>('PUT',    path, body),
  post:   <T>(path: string, body?: unknown)=> request<T>('POST',   path, body),
  patch:  <T>(path: string, body?: unknown)=> request<T>('PATCH',  path, body),
  delete: <T>(path: string)                => request<T>('DELETE', path),
  upload: <T>(path: string, form: FormData)=> upload<T>(path, form),
  download,
}

// ─── Response types ───────────────────────────────────────────────────────────

export interface ThresholdConfig {
  attendance_threshold:    number
  checkin_window_minutes:  number
  auto_kill_minutes:       number
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
  total_sessions_scheduled: number
  total_sessions_actual:    number
  total_students_today:     number
  ghost_lecture_count:      number
  avg_attendance_pct:       number
  ier:                      string
  eligibility_summary: {
    eligible:   number
    ineligible: number
    pending:    number
  }
  ghost_sessions: GhostSession[]
  filters: { date_from: string; date_to: string; course_id: string; unit_id: string; department: string }
}

export interface GhostSession {
  session_id:    string
  unit_id:       string
  unit_name:     string
  session_date:  string
  student_count: number
}

export interface WorkloadRecord {
  coordinator_id:          string
  coordinator_name:        string
  scheduled_sessions:      number
  actual_sessions:         number
  total_contact_hours_actual: number
  distinct_units:          number
  attendance_rate_pct:     number
}

export interface CourseHealthUnit {
  unit_id:               string
  unit_name:             string
  course_id:             string
  course_name:           string
  department:            string
  enrolled_count:        number
  avg_attendance_pct:    number
  below_threshold_count: number
  threshold:             number
  risk_level:            'HEALTHY' | 'AT_RISK' | 'CRITICAL'
}

export interface TrendPoint {
  week_start:             string
  sessions_held:          number
  unique_students:        number
  total_checkins:         number
  avg_students_per_session: number
}

export interface PunctualityRecord {
  coordinator_id:   string
  coordinator_name: string
  total_sessions:   number
  gate_opened_count: number
  no_gate_open_count: number
  avg_wait_minutes: number
  late_open_count:  number
}

export interface IneligibleRecord {
  student_id:         string
  student_name:       string
  email:              string
  course_id:          string
  course_name:        string
  unit_id:            string
  unit_name:          string
  sessions_held:      number
  sessions_attended:  number
  attendance_percentage: number
  threshold:          number
  deficit_sessions:   number
}

// Every student's per-unit eligibility (eligible + ineligible), with the
// dimensions the DQA dashboard filters on.
export interface AllEligibilityRecord extends IneligibleRecord {
  status:          'ELIGIBLE' | 'INELIGIBLE'
  current_year:    number
  semester:        number
  intake_session:  string
  academic_year:   string
}

export interface CoordinatorHealth {
  coordinator_id:       string
  coordinator_name:     string
  total_sessions:       number
  synced_sessions:      number
  failed_sessions:      number
  pending_sessions:     number
  sync_success_rate:    number
  manual_overrides:     number
  total_checkins:       number
  manual_override_ratio: number
}
