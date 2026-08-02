import type { ReactNode } from 'react'

/**
 * The stat tiles every dashboard header is built from.
 *
 * One component rather than each page rolling its own, because these numbers are compared ACROSS
 * dashboards — a dean and the DQA look at the same attendance rate and must not have to work out
 * whether two differently-styled tiles mean the same thing.
 *
 * `tone` carries meaning, not decoration: `bad` is reserved for a number somebody has to act on.
 */

export type KpiTone = 'neutral' | 'good' | 'warn' | 'bad'

const TONE: Record<KpiTone, { fg: string; bg: string; border: string }> = {
  neutral: { fg: 'var(--text)',  bg: 'var(--surface)',        border: 'var(--border)' },
  good:    { fg: '#166534',      bg: 'rgba(22,101,52,.07)',   border: 'rgba(22,101,52,.25)' },
  warn:    { fg: '#b45309',      bg: 'rgba(180,83,9,.07)',    border: 'rgba(180,83,9,.25)' },
  bad:     { fg: '#b91c1c',      bg: 'rgba(185,28,28,.07)',   border: 'rgba(185,28,28,.28)' },
}

export function Kpi({
  label, value, sub, tone = 'neutral', onClick,
}: {
  label: string
  value: ReactNode
  sub?: ReactNode
  tone?: KpiTone
  /** Given when the tile is a way IN to the thing it counts — a gap you can go and close. */
  onClick?: () => void
}) {
  const t = TONE[tone]
  return (
    <div
      onClick={onClick}
      role={onClick ? 'button' : undefined}
      tabIndex={onClick ? 0 : undefined}
      onKeyDown={onClick ? e => { if (e.key === 'Enter' || e.key === ' ') onClick() } : undefined}
      style={{
        background: t.bg, border: `1px solid ${t.border}`, borderRadius: 12,
        padding: '14px 16px', minWidth: 0, cursor: onClick ? 'pointer' : 'default',
      }}
    >
      <div style={{ fontSize: 12, color: 'var(--muted)', marginBottom: 6, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
        {label}
      </div>
      <div style={{ fontSize: 26, fontWeight: 700, lineHeight: 1.1, color: t.fg }}>{value}</div>
      {sub && <div style={{ fontSize: 11, color: 'var(--muted)', marginTop: 4 }}>{sub}</div>}
    </div>
  )
}

/** Responsive tile row. Wraps rather than scrolls, so nothing is hidden off the right edge. */
export function KpiRow({ children }: { children: ReactNode }) {
  return (
    <div style={{
      display: 'grid', gap: 12, marginBottom: 20,
      gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))',
    }}>
      {children}
    </div>
  )
}

/** Section heading with an optional right-hand action, used to break long dashboards up. */
export function Section({ title, hint, right, children }: {
  title: string; hint?: string; right?: ReactNode; children: ReactNode
}) {
  return (
    <section style={{ marginBottom: 26 }}>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 12, marginBottom: 10, flexWrap: 'wrap' }}>
        <h3 style={{ margin: 0, fontSize: 16 }}>{title}</h3>
        {hint && <span style={{ fontSize: 12, color: 'var(--muted)' }}>{hint}</span>}
        <div style={{ marginLeft: 'auto' }}>{right}</div>
      </div>
      {children}
    </section>
  )
}

/** A percentage with a bar, coloured against the threshold it is judged by. */
export function RateBar({ pct, threshold = 75 }: { pct: number; threshold?: number }) {
  const ok = pct >= threshold
  return (
    <div>
      <div style={{ fontSize: 13, fontWeight: 700, color: ok ? '#166534' : '#b91c1c' }}>{pct.toFixed(1)}%</div>
      <div style={{ height: 6, borderRadius: 6, background: 'var(--border)', overflow: 'hidden', marginTop: 3 }}>
        <div style={{
          width: `${Math.max(0, Math.min(100, pct))}%`, height: '100%',
          background: ok ? '#16a34a' : '#dc2626',
        }} />
      </div>
    </div>
  )
}
