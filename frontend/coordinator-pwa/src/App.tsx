import { useState, useEffect } from 'react'
import { useAuthStore } from './store/auth'
import { useTheme, applyPalette } from './theme'
import Login from './pages/Login'
import SessionPage from './pages/SessionPage'
import Dashboard from './pages/Dashboard'
import WelcomeToast from './components/WelcomeToast'

const API = import.meta.env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')

// True when the browser reports no network connectivity.
function useOnlineStatus() {
  const [online, setOnline] = useState(navigator.onLine)
  useEffect(() => {
    const go = () => setOnline(true)
    const goOff = () => setOnline(false)
    globalThis.addEventListener('online', go)
    globalThis.addEventListener('offline', goOff)
    return () => { globalThis.removeEventListener('online', go); globalThis.removeEventListener('offline', goOff) }
  }, [])
  return online
}

export default function App() {
  const { isAuthenticated, isExpired } = useAuthStore()
  const token = useAuthStore(s => s.token)
  const online = useOnlineStatus()
  useTheme()

  // Apply the tenant's brand palette if online, or from cached manifest if offline.
  useEffect(() => {
    if (!token) return
    fetch(`${API}/api/v1/branding`, { headers: { Authorization: `Bearer ${token}` } })
      .then(r => (r.ok ? r.json() : null))
      .then(b => { if (b) applyPalette(b) })
      .catch(() => {})
  }, [token])

  const [view, setView] = useState<'dashboard' | 'session'>('dashboard')

  // ── Offline-first auth logic ──────────────────────────────────────────────
  // If we have a token (even expired), show the dashboard — the user can still
  // view cached data and take attendance. Only force login when there is NO
  // token at all.
  const showLogin = !isAuthenticated

  const body = showLogin
    ? <Login />
    : view === 'session'
      ? <SessionPage onGoDashboard={() => setView('dashboard')} />
      : <Dashboard onTakeAttendance={() => setView('session')} />

  return (
    <div style={{ minHeight: '100vh', display: 'flex', flexDirection: 'column', background: 'var(--app-bg)' }}>
      <WelcomeToast />
      {/* Offline banner */}
      {!online && isAuthenticated && (
        <div style={{ background: '#fef3c7', color: '#92400e', padding: '8px 16px', fontSize: 13, textAlign: 'center', borderBottom: '1px solid #fde68a' }}>
          Offline — attendance data will sync when connectivity is restored.
        </div>
      )}
      {!online && showLogin && (
        <div style={{ background: '#fef3c7', color: '#92400e', padding: '8px 16px', fontSize: 13, textAlign: 'center', borderBottom: '1px solid #fde68a' }}>
          You are offline. Sign in was required earlier — connect to the internet to log in.
        </div>
      )}
      {isAuthenticated && isExpired() && online && (
        <div style={{ background: '#fef3c7', color: '#92400e', padding: '8px 16px', fontSize: 13, textAlign: 'center', borderBottom: '1px solid #fde68a' }}>
          Session expired — sign in again to refresh data. <button onClick={() => useAuthStore.getState().logout()} style={{ background: 'none', border: 'none', color: '#2563eb', cursor: 'pointer', textDecoration: 'underline', padding: 0, fontSize: 13 }}>Sign in</button>
        </div>
      )}
      <div style={{ flex: 1 }}>{body}</div>
      <footer style={{ background: 'var(--footer)', color: 'var(--footer-text)', padding: '10px 16px', fontSize: 11, textAlign: 'center' }}>
        Powered by LIGHT TECHNOLOGIES
      </footer>
    </div>
  )
}
