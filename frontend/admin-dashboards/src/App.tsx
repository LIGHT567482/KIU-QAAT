import { useEffect, useState } from 'react'
import { BrowserRouter, Routes, Route, Navigate, useNavigate } from 'react-router-dom'
import { AuthProvider, useAuth } from './contexts/AuthContext'
import { RoleLayout } from './layouts/RoleLayout'
import Login from './pages/Login'
import Unauthorized from './pages/Unauthorized'
import WelcomeToast from './components/WelcomeToast'

import VCOverview from './pages/vc/VCOverview'
import VCLecturerWorkload from './pages/vc/VCLecturerWorkload'
import DQAThresholds from './pages/dqa/DQAThresholds'
import DQAEligibility from './pages/dqa/DQAEligibility'
import DQACourseHealth from './pages/dqa/DQACourseHealth'
import DQATrends from './pages/dqa/DQATrends'
import DQAPunctuality from './pages/dqa/DQAPunctuality'
import QALiveSessions from './pages/qa/QALiveSessions'
import QADeviceReset from './pages/qa/QADeviceReset'
import QAManualCorrection from './pages/qa/QAManualCorrection'
import QACoordinatorHealth from './pages/qa/QACoordinatorHealth'
import AdminHome from './pages/admin/AdminTenants'
import AdminSettings from './pages/admin/AdminSettings'
import AdminUsers from './pages/admin/AdminUsers'
import AdminCourses from './pages/admin/AdminCourses'
import AdminCourseUnits from './pages/admin/AdminCourseUnits'
import AdminStudents from './pages/admin/AdminStudents'
import AdminVenues from './pages/admin/AdminVenues'
import AdminSchools from './pages/admin/AdminSchools'
import OrgLecturers from './pages/OrgLecturers'
import AdminLecturers from './pages/admin/AdminLecturers'
import LecturerDashboard from './pages/lecturer/LecturerDashboard'
import AdminLecturerAssignments from './pages/admin/AdminLecturerAssignments'
import AdminLecturerAttendance from './pages/admin/AdminLecturerAttendance'
import AdminCoordinators from './pages/admin/AdminCoordinators'
import AdminEmployees from './pages/admin/AdminEmployees'
import AdminEmployeeAttendance from './pages/admin/AdminEmployeeAttendance'
import AdminReports from './pages/admin/AdminReports'
import DashLecturerAttendance from './pages/shared/DashLecturerAttendance'
import QAStudentAttendance from './pages/qa/QAStudentAttendance'
import QAReports from './pages/qa/QAReports'
import Timetable from './pages/shared/Timetable'
import Messages from './pages/shared/Messages'
import LecturerPortal from './pages/LecturerPortal'

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <WelcomeToast />
        <CoordinatorQRBoot />
        <LecturerQRBoot />
        <Routes>
          <Route path="/login"        element={<Login />} />
          <Route path="/unauthorized" element={<Unauthorized />} />
          {/* Public, passwordless, read-only lecturer attendance portal. */}
          <Route path="/lecturer-portal" element={<LecturerPortal />} />

          {/* ── VC ─────────────────────────────────────────────────────── */}
          <Route element={<RoleLayout allowedRoles={['VC']} />}>
            <Route path="/vc"                     element={<VCOverview />} />
            <Route path="/vc/lecturer-workload"   element={<VCLecturerWorkload />} />
            <Route path="/vc/lecturer-attendance" element={<DashLecturerAttendance />} />
            <Route path="/vc/student-attendance"  element={<QAStudentAttendance />} />
          </Route>

          {/* ── DQA Director ───────────────────────────────────────────── */}
          <Route element={<RoleLayout allowedRoles={['DQA_DIRECTOR']} />}>
            <Route path="/dqa/thresholds"   element={<DQAThresholds />} />
            <Route path="/dqa/eligibility"  element={<DQAEligibility />} />
            <Route path="/dqa/course-health" element={<DQACourseHealth />} />
            <Route path="/dqa/trends"       element={<DQATrends />} />
            <Route path="/dqa/punctuality"  element={<DQAPunctuality />} />
            <Route path="/dqa/lecturer-attendance" element={<DashLecturerAttendance />} />
            <Route path="/dqa/student-attendance"  element={<QAStudentAttendance />} />
            <Route path="/dqa/messages"            element={<Messages />} />
          </Route>

          {/* ── QA Officer ─────────────────────────────────────────────── */}
          <Route element={<RoleLayout allowedRoles={['QA_OFFICER']} />}>
            <Route path="/qa/live"              element={<QALiveSessions />} />
            <Route path="/qa/reports"           element={<QAReports />} />
            <Route path="/qa/device-reset"      element={<QADeviceReset />} />
            <Route path="/qa/correction"        element={<QAManualCorrection />} />
            <Route path="/qa/coordinator-health" element={<QACoordinatorHealth />} />
            <Route path="/qa/student-attendance"  element={<QAStudentAttendance />} />
            <Route path="/qa/lecturer-attendance" element={<DashLecturerAttendance />} />
            <Route path="/qa/timetable"           element={<Timetable />} />
            <Route path="/qa/messages"            element={<Messages />} />
          </Route>

          {/* ── Tenant Admin (own institution only) ────────────────────── */}
          <Route element={<RoleLayout allowedRoles={['ADMIN']} />}>
            <Route path="/admin"                                      element={<AdminHome />} />
            <Route path="/admin/settings"                             element={<AdminSettings />} />
            <Route path="/admin/tenants/:tenantId/users"              element={<AdminUsers />} />
            <Route path="/admin/tenants/:tenantId/schools"            element={<AdminSchools />} />
            <Route path="/admin/tenants/:tenantId/courses"            element={<AdminCourses />} />
            <Route path="/admin/tenants/:tenantId/students"           element={<AdminStudents />} />
            <Route path="/admin/timetable"                            element={<Timetable />} />
            <Route path="/admin/tenants/:tenantId/venues"             element={<AdminVenues />} />
            <Route path="/admin/courses/:courseId/units"              element={<AdminCourseUnits />} />
            <Route path="/admin/tenants/:tenantId/coordinators"          element={<AdminCoordinators />} />
            <Route path="/admin/tenants/:tenantId/lecturers"              element={<AdminLecturers />} />
            <Route path="/admin/tenants/:tenantId/lecturer-assignments"  element={<AdminLecturerAssignments />} />
            <Route path="/admin/tenants/:tenantId/lecturer-attendance"   element={<AdminLecturerAttendance />} />
            <Route path="/admin/tenants/:tenantId/employees"             element={<AdminEmployees />} />
            <Route path="/admin/tenants/:tenantId/employee-attendance"   element={<AdminEmployeeAttendance />} />
            <Route path="/admin/tenants/:tenantId/student-attendance"    element={<QAStudentAttendance />} />
            <Route path="/admin/reports"                                 element={<AdminReports />} />
          </Route>

          {/* ── Lecturer (own assigned units) ──────────────────────────── */}
          <Route element={<RoleLayout allowedRoles={['LECTURER']} />}>
            <Route path="/lecturer" element={<LecturerDashboard />} />
          </Route>

          {/* ── HOD (own department) / Dean (own school) ───────────────── */}
          <Route element={<RoleLayout allowedRoles={['HOD']} />}>
            <Route path="/hod" element={<OrgLecturers level="hod" />} />
          </Route>
          <Route element={<RoleLayout allowedRoles={['DEAN']} />}>
            <Route path="/dean" element={<OrgLecturers level="dean" />} />
          </Route>

          <Route path="*" element={<Navigate to="/login" replace />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  )
}

// When a lecturer scans their QR it opens this app at /?lqr=<token>. We exchange the
// token for a LECTURER session (passwordless) and drop them on their dashboard.
const API = (import.meta as unknown as { env: { VITE_API_URL?: string } }).env.VITE_API_URL
  ?? (typeof location !== 'undefined' ? `${location.protocol}//${location.hostname}:8443` : 'http://localhost:8443')

// When a coordinator scans their QR it opens this app at /?cqr=<token>. We exchange the
// token for a COORDINATOR session (passwordless) and drop them on their dashboard.
function CoordinatorQRBoot() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const [err, setErr] = useState<string | null>(null)
  useEffect(() => {
    const cqr = new URLSearchParams(location.search).get('cqr')
    if (!cqr) return
    ;(async () => {
      try {
        const res = await fetch(`${API}/api/v1/coordinator/qr-login`, {
          method: 'POST', headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ qr: cqr }),
        })
        const d = await res.json()
        if (!res.ok || !d.access_token) throw new Error(d.message || 'QR sign-in failed')
        const p = JSON.parse(atob(d.access_token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')))
        login(d.access_token, { userId: p.sub, tenantId: p.tenant_id, role: p.role, expiresAt: p.exp })
        window.history.replaceState({}, '', location.pathname)
        navigate('/coordinator', { replace: true })
      } catch (e) { setErr(e instanceof Error ? e.message : 'QR sign-in failed') }
    })()
  }, [login, navigate])
  if (err) return <div style={{ padding: 16, background: '#fef2f2', color: '#b91c1c', fontSize: 14 }}>Coordinator QR sign-in: {err}</div>
  return null
}

function LecturerQRBoot() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const [err, setErr] = useState<string | null>(null)
  useEffect(() => {
    const lqr = new URLSearchParams(location.search).get('lqr')
    if (!lqr) return
    ;(async () => {
      try {
        const res = await fetch(`${API}/api/v1/lecturer/qr-login`, {
          method: 'POST', headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ qr: lqr }),
        })
        const d = await res.json()
        if (!res.ok || !d.access_token) throw new Error(d.message || 'QR sign-in failed')
        const p = JSON.parse(atob(d.access_token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')))
        login(d.access_token, { userId: p.sub, tenantId: p.tenant_id, role: p.role, expiresAt: p.exp })
        // Drop the token from the URL, then land on the lecturer dashboard.
        window.history.replaceState({}, '', location.pathname)
        navigate('/lecturer', { replace: true })
      } catch (e) { setErr(e instanceof Error ? e.message : 'QR sign-in failed') }
    })()
  }, [login, navigate])
  if (err) return <div style={{ padding: 16, background: '#fef2f2', color: '#b91c1c', fontSize: 14 }}>Lecturer QR sign-in: {err}</div>
  return null
}
