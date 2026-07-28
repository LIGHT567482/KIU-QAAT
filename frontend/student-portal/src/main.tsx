import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'
import brand from './brand.json'
import { applyPalette } from './theme'

// Instant institution branding from the bundled brand.json; the public branding fetch confirms it.
applyPalette(brand)

createRoot(document.getElementById('root')!).render(<StrictMode><App /></StrictMode>)
