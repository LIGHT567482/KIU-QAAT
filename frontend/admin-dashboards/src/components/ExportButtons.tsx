import { useState } from 'react'
import { api } from '../lib/api'

// Every report offers the same three downloads — Excel for analysis, CSV for feeding another
// system, PDF for circulating or filing — built from the report's CURRENT filters, so what
// downloads is exactly what is on screen.
//
// `base` is the endpoint without its extension, e.g.
// "/api/v1/dashboard/lecturer-attendance/export"; this appends ".xlsx" / ".pdf".
export default function ExportButtons({ base, filename, query = '', disabled }: {
  base: string
  /** File stem, e.g. "lecturer-attendance" — the extension is added per format. */
  filename: string
  /** Active filters as a query string, with or without the leading "?". */
  query?: string
  disabled?: boolean
}) {
  const [busy, setBusy] = useState<Fmt | null>(null)

  async function grab(fmt: Fmt) {
    setBusy(fmt)
    const qs = query ? (query.startsWith('?') ? query : `?${query}`) : ''
    try {
      await api.download(`${base}.${fmt}${qs}`, `${filename}.${fmt}`)
    } catch (e) {
      alert(e instanceof Error ? `Export failed: ${e.message}` : 'Export failed')
    } finally {
      setBusy(null)
    }
  }

  return (
    <div style={{ display: 'flex', gap: 8 }}>
      {FORMATS.map(f => (
        <button key={f.id} onClick={() => grab(f.id)} disabled={disabled || busy !== null}
          style={btn} title={f.title}>
          {busy === f.id ? 'Preparing…' : f.label}
        </button>
      ))}
    </div>
  )
}

type Fmt = 'xlsx' | 'csv' | 'pdf'

// Labelled "Export …" rather than just the format name: these pages also carry IMPORT
// controls, and two bare buttons marked "CSV" and "Excel" next to an "Import" one are easy
// to mistake for each other. The direction is spelled out on every button.
const FORMATS: { id: Fmt; label: string; title: string }[] = [
  { id: 'xlsx', label: 'Export Excel', title: 'Download this report as an Excel workbook (.xlsx)' },
  { id: 'csv',  label: 'Export CSV',   title: 'Download this report as a CSV file' },
  { id: 'pdf',  label: 'Export PDF',   title: 'Download this report as a PDF' },
]

const btn: React.CSSProperties = {
  padding: '8px 12px', background: 'var(--surface,#fff)', color: 'var(--text,#334155)',
  border: '1px solid var(--border,#e2e8f0)', borderRadius: 6, cursor: 'pointer',
  fontWeight: 600, fontSize: 13, whiteSpace: 'nowrap',
}
