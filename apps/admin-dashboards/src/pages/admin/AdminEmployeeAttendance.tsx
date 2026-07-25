import { useCallback, useEffect, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../../lib/api'

// Employee attendance from the check-in tablet. Admin uploads the tablet's export
// (staff_id, title, full_name, datetime, in/out, comment) and reads the report:
// one row per employee with an auto-generated Comment (date, times, days worked).
const PUNCH_COLS = ['staff_id', 'title', 'full_name', 'datetime', 'in/out', 'comment']

function downloadText(name: string, content: string, type = 'text/csv') {
  const url = URL.createObjectURL(new Blob([content], { type })); const a = document.createElement('a')
  a.href = url; a.download = name; a.click(); URL.revokeObjectURL(url)
}
function csvCell(s: string) { return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s }

interface Row {
  staff_id: string; title: string; full_name: string; contact: string; comment: string
}
interface Report { from: string; to: string; rows: Row[] }

export default function AdminEmployeeAttendance() {
  const { tenantId } = useParams<{ tenantId: string }>()
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [rep, setRep] = useState<Report | null>(null)
  const [loading, setLoading] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const fileRef = useRef<HTMLInputElement>(null)
  const [importing, setImporting] = useState(false)
  const [importMsg, setImportMsg] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true); setErr(null)
    try {
      const qs = new URLSearchParams()
      if (from) qs.set('from', from)
      if (to) qs.set('to', to)
      const res = await api.get<Report>(`/api/v1/admin/tenants/${tenantId}/employee-attendance?${qs.toString()}`)
      setRep(res)
    } catch (e) { setErr(e instanceof Error ? e.message : 'Failed to load report') }
    finally { setLoading(false) }
  }, [tenantId, from, to])

  useEffect(() => { load() }, [load])

  async function handleImport(ev: React.ChangeEvent<HTMLInputElement>) {
    const file = ev.target.files?.[0]
    if (!file) return
    setImporting(true); setImportMsg(null)
    try {
      const fd = new FormData(); fd.append('punches', file)
      const res = await api.upload<{ inserted: number; updated: number; skipped: number; errors: string[] }>(
        `/api/v1/admin/tenants/${tenantId}/employee-attendance/import`, fd)
      setImportMsg(`Imported: ${res.inserted} punches, ${res.updated} duplicates skipped, ${res.skipped} unreadable${res.errors?.length ? ` · ${res.errors.slice(0, 3).join('; ')}` : ''}`)
      load()
    } catch (e) { setImportMsg(e instanceof Error ? `Import failed: ${e.message}` : 'Import failed') }
    finally { setImporting(false); if (fileRef.current) fileRef.current.value = '' }
  }

  function exportReport() {
    if (!rep) return
    const head = ['staff_id', 'name', 'contact', 'comment']
    const lines = [head.join(',')]
    for (const r of rep.rows) {
      lines.push([r.staff_id, `${r.title ? r.title + ' ' : ''}${r.full_name}`, r.contact, r.comment].map(csvCell).join(','))
    }
    downloadText(`employee-attendance_${rep.from}_to_${rep.to}.csv`, lines.join('\n'))
  }

  return (
    <div>
      <h2 style={{ margin: 0 }}>Employee Attendance</h2>
      <p style={{ color: 'var(--muted)', margin: '4px 0 14px', fontSize: 13 }}>
        Upload the check-in tablet's export. Each row's <strong>Comment</strong> is generated automatically — the latest check-in/out date &amp; time and the total days the employee worked (a day with both a check-in and check-out).
      </p>

      <div style={{ display: 'flex', gap: 8, alignItems: 'flex-end', flexWrap: 'wrap', marginBottom: 14 }}>
        <button onClick={() => fileRef.current?.click()} disabled={importing} style={btnPrimary}>{importing ? 'Importing…' : 'Import tablet punches'}</button>
        <input ref={fileRef} type="file" accept=".csv,.xlsx" onChange={handleImport} style={{ display: 'none' }} />
        <button onClick={() => downloadText('tablet_punches_template.csv', PUNCH_COLS.join(',') + '\n')} style={btnSmall}>Template</button>
        <span style={{ flex: 1 }} />
        <Field label="From"><input type="date" value={from} onChange={e => setFrom(e.target.value)} style={inp} /></Field>
        <Field label="To"><input type="date" value={to} onChange={e => setTo(e.target.value)} style={inp} /></Field>
        <button onClick={load} style={btnSmall}>Apply</button>
        <button onClick={exportReport} disabled={!rep?.rows.length} style={btnSmall}>Export report</button>
      </div>
      {importMsg && <div style={{ background: importMsg.startsWith('Import failed') ? '#fef2f2' : '#f0fdf4', color: importMsg.startsWith('Import failed') ? '#b91c1c' : '#166534', padding: '10px 14px', borderRadius: 8, marginBottom: 14, fontSize: 13 }}>{importMsg}</div>}
      {err && <div style={{ background: '#fef2f2', color: '#b91c1c', padding: '10px 14px', borderRadius: 8, marginBottom: 14, fontSize: 13 }}>{err}</div>}

      {rep && <div style={{ color: 'var(--muted)', fontSize: 12, marginBottom: 8 }}>Period {rep.from} → {rep.to} · {rep.rows.length} employee(s)</div>}
      {loading ? <p style={{ color: 'var(--muted)' }}>Loading…</p> : (
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
            <thead><tr style={{ background: 'var(--surface-2)' }}>
              <th style={th}>Staff ID</th><th style={th}>Name</th><th style={th}>Contact</th><th style={{ ...th, minWidth: 360 }}>Comment (auto)</th>
            </tr></thead>
            <tbody>
              {(rep?.rows ?? []).map(r => (
                <tr key={r.staff_id} style={{ borderTop: '1px solid var(--border)' }}>
                  <td style={{ ...td, fontFamily: 'monospace' }}>{r.staff_id}</td>
                  <td style={td}>{r.title ? `${r.title} ` : ''}{r.full_name}</td>
                  <td style={{ ...td, color: 'var(--muted)' }}>{r.contact || '—'}</td>
                  <td style={{ ...td, color: 'var(--muted)' }}>{r.comment}</td>
                </tr>
              ))}
              {rep && rep.rows.length === 0 && <tr><td colSpan={4} style={{ ...td, textAlign: 'center', color: 'var(--muted)', padding: 28 }}>No employees or punches in this period. Register employees, then import the tablet's export.</td></tr>}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <label style={{ display: 'block' }}><div style={{ fontSize: 11, fontWeight: 700, color: 'var(--muted)', marginBottom: 3 }}>{label}</div>{children}</label>
}

const th: React.CSSProperties = { padding: '8px 12px', textAlign: 'left', color: 'var(--muted)', fontSize: 12, whiteSpace: 'nowrap' }
const td: React.CSSProperties = { padding: '9px 12px', verticalAlign: 'top' }
const inp: React.CSSProperties = { padding: '7px 10px', borderRadius: 6, border: '1px solid var(--border)', fontSize: 13, background: 'var(--surface)', color: 'var(--text)' }
const btnPrimary: React.CSSProperties = { padding: '8px 16px', background: 'var(--brand)', color: 'var(--brand-contrast)', border: 'none', borderRadius: 6, cursor: 'pointer', fontWeight: 600, fontSize: 13 }
const btnSmall: React.CSSProperties = { padding: '7px 12px', background: 'var(--surface-2)', border: '1px solid var(--border)', borderRadius: 6, cursor: 'pointer', fontSize: 12, color: 'var(--text)' }
