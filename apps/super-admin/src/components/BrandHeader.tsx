import { useEffect, useState } from 'react'
import { api, type Branding } from '../lib/api'
import { applyPalette } from '../theme'

// Top bar: logo + motto, coloured with the tenant's sidebar colour. For the
// super-admin console this is the QAAT platform's own branding. The full palette
// is pushed into the theme so every region reflects the institution's colours.
export function BrandHeader({ right }: { right?: React.ReactNode }) {
  const [b, setB] = useState<Branding | null>(null)

  useEffect(() => {
    api.get<Branding>('/api/v1/branding').then(br => { setB(br); applyPalette(br) }).catch(() => setB(null))
  }, [])

  const name = b?.name || 'QAAT Platform'
  const motto = b?.motto || 'Attendance integrity, verified.'

  return (
    <header style={{
      display: 'flex', alignItems: 'center', justifyContent: 'space-between',
      padding: '12px 24px', background: 'var(--sidebar)', color: 'var(--sidebar-text)',
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        {b?.logo_url
          ? <img src={b.logo_url} alt={name} style={{ height: 72, width: 72, objectFit: 'contain', borderRadius: 6 }} />
          : <div style={{
              height: 72, width: 72, borderRadius: 8, background: 'var(--brand)',
              color: 'var(--brand-contrast)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontWeight: 700,
            }}>{name.slice(0, 1)}</div>}
        <div>
          <div style={{ fontWeight: 700, fontSize: 15, lineHeight: 1.1 }}>{name}</div>
          <div style={{ fontSize: 12, opacity: .8 }}>{motto}</div>
        </div>
      </div>
      {right}
    </header>
  )
}
