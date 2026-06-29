import { useEffect, useState, useCallback } from 'react'

// Self-contained light/dark theme for the app. Injects CSS variables once, persists
// the choice, and defaults to the OS preference. Components reference the variables
// in their inline styles (e.g. background: 'var(--surface)') so a single toggle
// re-themes the whole app. The per-tenant brand colour is injected at runtime via
// setBrandColor() so institution branding is reflected across every screen.
export type Theme = 'light' | 'dark'

const KEY = 'qaat_theme'

const CSS = `
:root, :root[data-theme="light"] {
  --bg: #f1f5f9;
  --surface: #ffffff;
  --surface-2: #f8fafc;
  --text: #0f172a;
  --muted: #334155;
  --border: #e2e8f0;
  --brand: #1a73e8;
  --brand-contrast: #ffffff;
  --shadow: 0 1px 4px rgba(0,0,0,.06);
  /* Brand palette regions — overridden per-tenant by applyPalette(). Default to
     the theme so an unbranded tenant still looks right in light & dark. */
  --app-bg: #f1f5f9;
  --sidebar: #1e293b;
  --sidebar-text: #e2e8f0;
  --footer: #0f172a;
  --footer-text: #e2e8f0;
  color-scheme: light;
}
:root[data-theme="dark"] {
  --bg: #0b1220;
  --surface: #111b30;
  --surface-2: #0f1729;
  --text: #e2e8f0;
  --muted: #94a3b8;
  --border: #25324d;
  --brand-contrast: #ffffff;
  --shadow: 0 1px 6px rgba(0,0,0,.5);
  --app-bg: #0b1220;
  --sidebar: #0f1729;
  --sidebar-text: #e2e8f0;
  --footer: #060b16;
  --footer-text: #cbd5e1;
  color-scheme: dark;
}
* { box-sizing: border-box; }
html, body, #root { margin: 0; padding: 0; height: 100%; }
/* Full-bleed shell — fills the visible viewport (100dvh handles mobile browser
   chrome) so the footer always sits at the very bottom, on phones too. */
.app-shell { min-height: 100vh; min-height: 100dvh; }
html, body { background: var(--app-bg); color: var(--text); transition: background .2s, color .2s; }
input, select, textarea {
  background: var(--surface); color: var(--text); border-color: var(--border);
}
::placeholder { color: var(--muted); }
`

let injected = false
function injectCSS() {
  if (injected || typeof document === 'undefined') return
  const s = document.createElement('style')
  s.id = 'qaat-theme'
  s.textContent = CSS
  document.head.appendChild(s)
  injected = true
}

export function getInitialTheme(): Theme {
  try {
    const saved = localStorage.getItem(KEY)
    if (saved === 'light' || saved === 'dark') return saved
  } catch { /* ignore */ }
  return typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

// The per-region brand palette a tenant can configure. Each is an optional hex;
// unset regions fall back to the theme defaults in CSS.
export interface Palette {
  brand_color?: string
  sidebar_color?: string
  background_color?: string
  footer_color?: string
}

// readableOn returns a legible text colour (near-black or white) for a given
// background hex, using perceived luminance — so a green sidebar gets white text
// and a white background gets dark text automatically.
function readableOn(hex?: string): string {
  if (!hex) return '#0f172a'
  const c = hex.replace('#', '')
  const full = c.length === 3 ? c.split('').map(x => x + x).join('') : c
  if (full.length !== 6) return '#0f172a'
  const r = parseInt(full.slice(0, 2), 16), g = parseInt(full.slice(2, 4), 16), b = parseInt(full.slice(4, 6), 16)
  const lum = (0.299 * r + 0.587 * g + 0.114 * b) / 255
  return lum > 0.6 ? '#0f172a' : '#ffffff'
}

const HEX = /^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/

// applyPalette pushes a tenant's colour palette into CSS variables so each region
// (accent, sidebar, background, footer) is coloured independently across the whole
// system. Values are validated as hex to prevent CSS injection.
export function applyPalette(p?: Palette | null) {
  if (!p || typeof document === 'undefined') return
  const root = document.documentElement.style
  const set = (name: string, val?: string) => { if (val && HEX.test(val)) root.setProperty(name, val) }
  set('--brand', p.brand_color)
  if (p.sidebar_color && HEX.test(p.sidebar_color)) {
    root.setProperty('--sidebar', p.sidebar_color)
    root.setProperty('--sidebar-text', readableOn(p.sidebar_color))
  }
  set('--app-bg', p.background_color)
  if (p.footer_color && HEX.test(p.footer_color)) {
    root.setProperty('--footer', p.footer_color)
    root.setProperty('--footer-text', readableOn(p.footer_color))
  }
}

// Back-compat: set just the accent colour.
export function setBrandColor(color?: string | null) {
  applyPalette({ brand_color: color ?? undefined })
}

// Module-level store so every useTheme() caller (e.g. the sidebar toggle and a
// login screen) shares one source of truth and stays in sync.
let current: Theme = getInitialTheme()
const listeners = new Set<() => void>()

function apply(t: Theme) {
  current = t
  if (typeof document !== 'undefined') document.documentElement.dataset.theme = t
  try { localStorage.setItem(KEY, t) } catch { /* ignore */ }
  listeners.forEach(l => l())
}

// useTheme returns the current theme + a toggle. Safe to call in any component.
export function useTheme() {
  const [theme, setTheme] = useState<Theme>(current)
  useEffect(() => {
    injectCSS()
    apply(current) // ensure data-theme is set on first mount
    const l = () => setTheme(current)
    listeners.add(l)
    return () => { listeners.delete(l) }
  }, [])
  const toggle = useCallback(() => apply(current === 'dark' ? 'light' : 'dark'), [])
  return { theme, toggle }
}

export function ThemeToggle({ theme, toggle }: { theme: Theme; toggle: () => void }) {
  return (
    <button onClick={toggle} title="Toggle light / dark"
      aria-label="Toggle light or dark mode"
      style={{
        height: 34, width: 34, borderRadius: 8, cursor: 'pointer',
        background: 'var(--surface-2)', color: 'var(--text)', border: '1px solid var(--border)',
        fontSize: 16, lineHeight: 1, display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
      }}>
      {theme === 'dark' ? '☀' : '☾'}
    </button>
  )
}
