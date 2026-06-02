import { useAuthStore } from '../store/auth'

// Phase 2 will fill in the session management UI.
// This is the post-login shell: daily manifest status + session launcher.

export default function Dashboard() {
  const { role, userId, logout } = useAuthStore(s => ({
    role: s.role, userId: s.userId, logout: s.logout,
  }))

  return (
    <div style={{ maxWidth: 480, margin: '0 auto', padding: 24, fontFamily: 'system-ui' }}>
      <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h2 style={{ margin: 0 }}>QAAT</h2>
        <button onClick={logout} style={{ background: 'none', border: 'none', color: '#1a73e8', cursor: 'pointer' }}>
          Sign out
        </button>
      </header>

      <div style={{ background: '#f0f7ff', borderRadius: 8, padding: 16, marginBottom: 16 }}>
        <p style={{ margin: 0, fontSize: 14, color: '#444' }}>
          Signed in as <strong>{role}</strong> · {userId?.slice(0, 8)}…
        </p>
      </div>

      <div style={{ background: '#fff3cd', borderRadius: 8, padding: 16 }}>
        <p style={{ margin: 0, fontWeight: 600 }}>Phase 2 — Coming next</p>
        <ul style={{ margin: '8px 0 0', paddingLeft: 20, color: '#555' }}>
          <li>Daily Manifest fetch</li>
          <li>BLE beacon scan</li>
          <li>Session state machine UI</li>
          <li>QR scanner</li>
        </ul>
      </div>
    </div>
  )
}
