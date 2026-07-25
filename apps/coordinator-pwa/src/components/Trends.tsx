import { useEffect, useRef, useState } from 'react'
import { useAuthStore } from '../store/auth'

// Attendance-trend view — mirrors the phone app's Trends screen: a weekly attendance-rate
// line (red points below the threshold) + summary stats, so the coordinator sees the same
// analytics on the laptop PWA as on the phone.
const API = import.meta.env.VITE_API_URL ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')

interface Week { label: string; rate_pct: number; below_threshold: boolean }
interface TrendsData {
  threshold: number; weeks: Week[]; current_week_rate: number; semester_average: number
  best_week: Week | null; worst_week: Week | null; weeks_below: number; trend: string
}

function cssVar(name: string, fallback: string): string {
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return v || fallback
}

export default function Trends() {
  const token = useAuthStore(s => s.token)
  const [data, setData] = useState<TrendsData | null>(null)
  const [loaded, setLoaded] = useState(false)
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    if (!token) return
    fetch(`${API}/api/v1/coordinator/trends`, { headers: { Authorization: `Bearer ${token}` } })
      .then(r => (r.ok ? r.json() : null)).then(setData).catch(() => {}).finally(() => setLoaded(true))
  }, [token])

  useEffect(() => {
    const cv = canvasRef.current
    if (!cv || !data || data.weeks.length === 0) return
    const dpr = window.devicePixelRatio || 1
    const W = cv.clientWidth, H = 220
    cv.width = W * dpr; cv.height = H * dpr
    const ctx = cv.getContext('2d')!; ctx.scale(dpr, dpr); ctx.clearRect(0, 0, W, H)
    const brand = cssVar('--brand', '#2563eb'), muted = cssVar('--border', '#e2e8f0'), red = '#dc2626'
    const pad = 8
    const x = (i: number) => data.weeks.length === 1 ? W / 2 : pad + (W - 2 * pad) * i / (data.weeks.length - 1)
    const y = (r: number) => H - pad - (r / 100) * (H - 2 * pad)
    // threshold (dashed)
    ctx.strokeStyle = muted; ctx.lineWidth = 1.5; ctx.setLineDash([10, 6])
    ctx.beginPath(); ctx.moveTo(0, y(data.threshold)); ctx.lineTo(W, y(data.threshold)); ctx.stroke(); ctx.setLineDash([])
    // line
    ctx.strokeStyle = brand; ctx.lineWidth = 3; ctx.beginPath()
    data.weeks.forEach((wk, i) => { const px = x(i), py = y(wk.rate_pct); i === 0 ? ctx.moveTo(px, py) : ctx.lineTo(px, py) })
    ctx.stroke()
    // points
    data.weeks.forEach((wk, i) => { ctx.fillStyle = wk.below_threshold ? red : brand; ctx.beginPath(); ctx.arc(x(i), y(wk.rate_pct), 5, 0, Math.PI * 2); ctx.fill() })
  }, [data])

  if (!loaded) return <p style={{ color: 'var(--muted)', textAlign: 'center', padding: 24 }}>Loading…</p>
  if (!data || data.weeks.length === 0) return <p style={{ color: 'var(--muted)', textAlign: 'center', padding: 24 }}>No completed sessions yet — trends appear once you've held and synced some sessions.</p>

  const arrow = data.trend === 'RISING' ? 'Rising ↑' : data.trend === 'FALLING' ? 'Falling ↓' : 'Stable →'
  const stat = (label: string, value: string) => (
    <div style={{ background: 'var(--surface,#f8fafc)', border: '1px solid var(--border,#e2e8f0)', borderRadius: 10, padding: '10px 14px' }}>
      <div style={{ fontSize: 18, fontWeight: 800 }}>{value}</div>
      <div style={{ fontSize: 12, color: 'var(--muted)' }}>{label}</div>
    </div>
  )

  return (
    <div>
      <div style={{ fontSize: 12, color: 'var(--muted)', marginBottom: 8 }}>Weekly attendance rate across your cohort. The dashed line is the {data.threshold}% threshold; red points fell below it.</div>
      <div style={{ background: 'var(--surface,#fff)', border: '1px solid var(--border,#e2e8f0)', borderRadius: 12, padding: 12, marginBottom: 14 }}>
        <canvas ref={canvasRef} style={{ width: '100%', height: 220 }} />
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))', gap: 10 }}>
        {stat('Current week', `${Math.round(data.current_week_rate)}%`)}
        {stat('Semester average', `${Math.round(data.semester_average)}%`)}
        {data.best_week && stat('Best week', `${Math.round(data.best_week.rate_pct)}%`)}
        {data.worst_week && stat('Worst week', `${Math.round(data.worst_week.rate_pct)}%`)}
        {stat('Weeks below threshold', `${data.weeks_below}`)}
        {stat('Trend', arrow)}
      </div>
    </div>
  )
}
