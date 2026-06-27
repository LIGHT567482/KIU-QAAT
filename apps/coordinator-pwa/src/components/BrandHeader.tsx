import { useEffect, useState } from 'react'
import { useAuthStore } from '../store/auth'
import { useTheme, ThemeToggle, applyPalette } from '../theme'

const API = import.meta.env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')

interface Branding {
  name: string; logo_url: string; motto: string
  brand_color: string; sidebar_color: string; background_color: string; footer_color: string
}

// Institution branding (logo + motto) shown top-left on the coordinator PWA, with
// a light/dark toggle on the right. The tenant brand colour is pushed into the theme.
export function BrandHeader() {
  const token = useAuthStore(s => s.token)
  const { theme, toggle } = useTheme()
  const [b, setB] = useState<Branding | null>(null)

  useEffect(() => {
    if (!token) return
    fetch(`${API}/api/v1/branding`, { headers: { Authorization: `Bearer ${token}` } })
      .then(r => (r.ok ? r.json() : null))
      .then((br: Branding | null) => { setB(br); if (br) applyPalette(br) })
      .catch(() => setB(null))
  }, [token])

  const name = b?.name || 'QAAT'

  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        {b?.logo_url
          ? <img src={b.logo_url} alt={name} style={{ height: 64, width: 64, objectFit: 'contain', borderRadius: 6 }} />
          : <div style={{
              height: 64, width: 64, borderRadius: 6, background: 'var(--brand)',
              color: 'var(--brand-contrast)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontWeight: 700,
            }}>{name.slice(0, 1)}</div>}
        <div>
          <div style={{ fontWeight: 700, fontSize: 15, lineHeight: 1.1, color: 'var(--text)' }}>{name}</div>
          {b?.motto && <div style={{ fontSize: 11, color: 'var(--muted)' }}>{b.motto}</div>}
        </div>
      </div>
      <ThemeToggle theme={theme} toggle={toggle} />
    </div>
  )
}
