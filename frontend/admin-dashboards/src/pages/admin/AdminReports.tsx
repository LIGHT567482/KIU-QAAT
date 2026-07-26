import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { useAuth } from '../../contexts/AuthContext'
import { api } from '../../lib/api'

// Reports hub — one place for admins to reach every attendance report:
// students, lecturers, and general employees (tablet check-in/out), plus the
// downloadable zip archives created when a semester's data is cleared.
export default function AdminReports() {
  const { user } = useAuth()
  const t = `/admin/tenants/${user?.tenantId ?? ''}`

  const cards = [
    { to: `${t}/student-attendance`, title: 'Student Attendance', desc: 'Per-student attendance and exam eligibility across units, with filters and export.' },
    { to: `${t}/lecturer-attendance`, title: 'Lecturer Attendance', desc: 'Lecturer contact hours and session start/end confirmations.' },
    { to: `${t}/employee-attendance`, title: 'Employee Attendance', desc: 'General staff check-in/out from the tablet, with auto-generated comments and days-worked totals.' },
  ]

  return (
    <div>
      <h2 style={{ margin: 0 }}>Reports</h2>
      <p style={{ color: 'var(--muted)', margin: '4px 0 20px', fontSize: 13 }}>Generate and export attendance reports for every group in your institution.</p>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))', gap: 16 }}>
        {cards.map(c => (
          <Link key={c.to} to={c.to} style={{ textDecoration: 'none', color: 'inherit' }}>
            <div style={{ background: 'var(--surface)', border: '1px solid var(--border)', borderRadius: 12, padding: 20, height: '100%', boxShadow: 'var(--shadow)' }}>
              <div style={{ fontWeight: 700, fontSize: 16, marginBottom: 8 }}>{c.title}</div>
              <div style={{ color: 'var(--muted)', fontSize: 13, lineHeight: 1.5 }}>{c.desc}</div>
              <div style={{ marginTop: 14, color: 'var(--brand)', fontWeight: 600, fontSize: 13 }}>Open report →</div>
            </div>
          </Link>
        ))}
      </div>

      <SemesterArchives tenantId={user?.tenantId ?? ''} />
    </div>
  )
}

interface Archive {
  archive_id: string; label: string; intakes: string[]; academic_year: string; semester: number
  filename: string; size_bytes: number; attendance_rows: number; session_rows: number
  lecturer_rows: number; created_by: string; created_at: string
}

function fmtBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

// Semester archives — the zip snapshots created before each intake's data is cleared.
function SemesterArchives({ tenantId }: { tenantId: string }) {
  const [rows, setRows] = useState<Archive[]>([])
  const [loaded, setLoaded] = useState(false)
  const [busy, setBusy] = useState<string | null>(null)

  function load() {
    if (!tenantId) return
    api.get<Archive[]>(`/api/v1/admin/tenants/${tenantId}/semester-archives`)
      .then(d => setRows(d ?? []))
      .catch(() => setRows([]))
      .finally(() => setLoaded(true))
  }
  useEffect(load, [tenantId])

  async function remove(a: Archive) {
    if (!confirm(`Delete the archive "${a.label}"? This removes the stored zip permanently.`)) return
    setBusy(a.archive_id)
    try {
      await api.delete(`/api/v1/admin/tenants/${tenantId}/semester-archives/${a.archive_id}`)
      setRows(rs => rs.filter(x => x.archive_id !== a.archive_id))
    } catch (e) { alert(e instanceof Error ? e.message : 'Delete failed') }
    finally { setBusy(null) }
  }

  return (
    <div style={{ marginTop: 32 }}>
      <h3 style={{ margin: '0 0 4px', fontSize: 16 }}>Semester archives</h3>
      <p style={{ color: 'var(--muted)', margin: '0 0 14px', fontSize: 13 }}>
        Every end-of-semester data clear zips what it deletes and stores it here. Download to keep a permanent record of a cleared intake's attendance.
      </p>
      {!loaded ? (
        <div style={{ fontSize: 13, color: 'var(--muted)' }}>Loading…</div>
      ) : rows.length === 0 ? (
        <div style={{ fontSize: 13, color: 'var(--muted)', background: 'var(--surface)', border: '1px solid var(--border)', borderRadius: 10, padding: 16 }}>
          No archives yet. When you clear a semester's data for an intake, its zip appears here.
        </div>
      ) : (
        <div style={{ overflowX: 'auto', border: '1px solid var(--border)', borderRadius: 10 }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
            <thead>
              <tr style={{ background: 'var(--surface)', textAlign: 'left' }}>
                {['Archive', 'Intake(s)', 'Rows', 'Size', 'Created', ''].map(h => (
                  <th key={h} style={{ padding: '10px 12px', color: 'var(--muted)', fontWeight: 600, whiteSpace: 'nowrap' }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map(a => (
                <tr key={a.archive_id} style={{ borderTop: '1px solid var(--border)' }}>
                  <td style={{ padding: '10px 12px' }}>
                    <div style={{ fontWeight: 600 }}>{a.label}</div>
                    {a.created_by && <div style={{ color: 'var(--muted)', fontSize: 11 }}>by {a.created_by}</div>}
                  </td>
                  <td style={{ padding: '10px 12px' }}>{a.intakes.join(', ')}{a.academic_year ? ` · ${a.academic_year}` : ''}</td>
                  <td style={{ padding: '10px 12px', whiteSpace: 'nowrap' }}>{a.attendance_rows} att · {a.session_rows} sess · {a.lecturer_rows} lect</td>
                  <td style={{ padding: '10px 12px', whiteSpace: 'nowrap' }}>{fmtBytes(a.size_bytes)}</td>
                  <td style={{ padding: '10px 12px', whiteSpace: 'nowrap' }}>{a.created_at.slice(0, 16).replace('T', ' ')}</td>
                  <td style={{ padding: '10px 12px', whiteSpace: 'nowrap' }}>
                    <div style={{ display: 'flex', gap: 6 }}>
                      <button onClick={() => api.download(`/api/v1/admin/tenants/${tenantId}/semester-archives/${a.archive_id}/download`, a.filename).catch(e => alert(e instanceof Error ? e.message : 'Download failed'))}
                        style={{ padding: '4px 10px', border: '1px solid var(--brand)', color: 'var(--brand)', background: 'transparent', borderRadius: 6, cursor: 'pointer', fontSize: 12, fontWeight: 600 }}>Download zip</button>
                      <button onClick={() => remove(a)} disabled={busy === a.archive_id}
                        style={{ padding: '4px 10px', border: '1px solid #fecaca', color: '#b91c1c', background: '#fef2f2', borderRadius: 6, cursor: 'pointer', fontSize: 12 }}>{busy === a.archive_id ? '…' : 'Delete'}</button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
