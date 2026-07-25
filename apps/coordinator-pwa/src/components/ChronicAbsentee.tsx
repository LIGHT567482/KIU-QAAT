import { useEffect, useState } from 'react'
import { useAuthStore } from '../store/auth'

// Chronic Absentee (coordinator's app.md Feature 3) — surfaces at-risk students in the
// coordinator's cohort from the attendance summary. Below the institutional threshold →
// WARNING; below half of it → CRITICAL.
const API = import.meta.env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')

interface Row {
  student_id: string; full_name: string; unit_id: string; unit_name: string
  sessions_held: number; sessions_attended: number; attendance_percentage: number; threshold: number
}

export default function ChronicAbsentee() {
  const token = useAuthStore(s => s.token)
  const [rows, setRows] = useState<Row[]>([])
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    if (!token) return
    fetch(`${API}/api/v1/coordinator/attendance`, { headers: { Authorization: `Bearer ${token}` } })
      .then(r => (r.ok ? r.json() : []))
      .then((d: Row[]) => setRows(d ?? []))
      .catch(() => {})
      .finally(() => setLoaded(true))
  }, [token])

  const flagged = rows.filter(r => r.sessions_held > 0 && r.attendance_percentage < r.threshold)

  return (
    <div style={{ background: 'var(--surface, #f8fafc)', border: '1px solid var(--border, #e2e8f0)', borderRadius: 12, padding: 16, marginTop: 16 }}>
      <div style={{ fontWeight: 800, marginBottom: 8 }}>Chronic absentees</div>
      {!loaded ? (
        <div style={{ fontSize: 13, color: 'var(--muted)' }}>Loading…</div>
      ) : flagged.length === 0 ? (
        <div style={{ fontSize: 13, color: 'var(--muted)' }}>No students below the attendance threshold. 🎉</div>
      ) : (
        <div style={{ display: 'grid', gap: 6 }}>
          {flagged.map((r, i) => {
            const critical = r.attendance_percentage < r.threshold / 2
            const c = critical ? { bg: '#fee2e2', fg: '#991b1b', label: 'CRITICAL' } : { bg: '#fef9c3', fg: '#854d0e', label: 'WARNING' }
            return (
              <div key={`${r.student_id}-${r.unit_id}-${i}`} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', fontSize: 13, padding: '6px 0', borderBottom: '1px solid var(--border)' }}>
                <div style={{ minWidth: 0 }}>
                  <div style={{ fontWeight: 600 }}>{r.full_name}</div>
                  <div style={{ color: 'var(--muted)', fontSize: 11 }}>
                    {r.student_id} · {r.unit_name || r.unit_id} · {r.sessions_attended}/{r.sessions_held} · {Math.round(r.attendance_percentage)}% (need {r.threshold}%)
                  </div>
                </div>
                <span style={{ background: c.bg, color: c.fg, borderRadius: 100, padding: '2px 10px', fontSize: 11, fontWeight: 700, whiteSpace: 'nowrap' }}>{c.label}</span>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
