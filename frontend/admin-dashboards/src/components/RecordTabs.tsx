import { useState } from 'react'

// A lecture is now witnessed twice — once by the coordinator who ran the session, once by the QA
// patroller who walked in on it — so "Lecturer Attendance" is one feature with two pages. This is
// the shell they share: a heading plus a tab strip, with only the selected page mounted (so the
// hidden one costs no fetch).

export interface RecordTab {
  id: string
  label: string
  /** One line under the tab strip explaining what this record is. */
  hint?: string
  render: () => React.ReactNode
}

export default function RecordTabs({ title, tabs }: { title: string; tabs: RecordTab[] }) {
  const [active, setActive] = useState(tabs[0]?.id ?? '')
  const current = tabs.find(t => t.id === active) ?? tabs[0]

  return (
    <div style={{ color: 'var(--text)' }}>
      <h2 style={{ margin: '0 0 10px' }}>{title}</h2>

      <div role="tablist" style={{
        display: 'flex', gap: 4, borderBottom: '1px solid var(--border,#e2e8f0)', marginBottom: 16,
        flexWrap: 'wrap',
      }}>
        {tabs.map(t => {
          const on = t.id === current?.id
          return (
            <button key={t.id} role="tab" aria-selected={on} onClick={() => setActive(t.id)} style={{
              padding: '9px 16px', border: 'none', background: 'transparent', cursor: 'pointer',
              font: 'inherit', fontSize: 14, fontWeight: on ? 700 : 500,
              color: on ? 'var(--brand,#2563eb)' : 'var(--muted,#64748b)',
              borderBottom: on ? '2px solid var(--brand,#2563eb)' : '2px solid transparent',
              marginBottom: -1,
            }}>
              {t.label}
            </button>
          )
        })}
      </div>

      {current?.render()}
    </div>
  )
}
