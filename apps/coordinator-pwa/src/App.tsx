import { useState, useEffect } from 'react'
import { useAuthStore } from './store/auth'
import { useTheme, applyPalette } from './theme'
import Login from './pages/Login'
import SessionPage from './pages/SessionPage'
import Dashboard from './pages/Dashboard'

const API = import.meta.env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')

export default function App() {
  const { isAuthenticated, isExpired } = useAuthStore()
  const token = useAuthStore(s => s.token)
  useTheme() // initialise + inject theme CSS for the whole app

  // Apply the tenant's brand palette across EVERY page of the PWA (not just the
  // screens that mount BrandHeader) as soon as the coordinator is authenticated.
  useEffect(() => {
    if (!token) return
    fetch(`${API}/api/v1/branding`, { headers: { Authorization: `Bearer ${token}` } })
      .then(r => (r.ok ? r.json() : null))
      .then(b => { if (b) applyPalette(b) })
      .catch(() => {})
  }, [token])

  // Coordinators land on their dashboard; "Take attendance" opens the session
  // screen, and closing a session offers "Go to my dashboard" back here.
  const [view, setView] = useState<'dashboard' | 'session'>('dashboard')

  const body = (!isAuthenticated || isExpired())
    ? <Login />
    : view === 'session'
      ? <SessionPage onGoDashboard={() => setView('dashboard')} />
      : <Dashboard onTakeAttendance={() => setView('session')} />

  return (
    <div style={{ minHeight: '100vh', display: 'flex', flexDirection: 'column', background: 'var(--app-bg)' }}>
      <div style={{ flex: 1 }}>{body}</div>
      <footer style={{ background: 'var(--footer)', color: 'var(--footer-text)', padding: '10px 16px', fontSize: 11, textAlign: 'center' }}>
        Powered by LIGHT TECHNOLOGIES
      </footer>
    </div>
  )
}
