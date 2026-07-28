import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { registerSW } from 'virtual:pwa-register'
import App from './App'
import brand from './brand.json'
import { applyPalette } from './theme'

// Instant institution branding from the bundled brand.json; backend fetch confirms it later.
applyPalette(brand)

// Auto-update service worker — prompts user on new version available.
registerSW({ onNeedRefresh() {}, onOfflineReady() {} })

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
