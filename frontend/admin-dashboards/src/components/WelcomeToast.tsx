import { useEffect, useState } from 'react'

/** Reads a one-shot welcome message from sessionStorage (set at login) and shows a brief toast.
 *  Mount once near the app root. No toast library needed. */
export default function WelcomeToast() {
  const [msg, setMsg] = useState<string | null>(null)
  useEffect(() => {
    const name = sessionStorage.getItem('qaat_welcome')
    if (!name) return
    sessionStorage.removeItem('qaat_welcome')
    setMsg(`Welcome, ${name} \u{1F44B}`)
    const t = setTimeout(() => setMsg(null), 4000)
    return () => clearTimeout(t)
  }, [])
  if (!msg) return null
  return (
    <div style={{
      position: 'fixed', bottom: 18, right: 18, zIndex: 9999,
      background: 'var(--brand, #1a7a3f)', color: '#fff', padding: '10px 20px', borderRadius: 10,
      boxShadow: '0 8px 24px rgba(0,0,0,.18)', fontWeight: 600, fontSize: 14,
    }}>{msg}</div>
  )
}
