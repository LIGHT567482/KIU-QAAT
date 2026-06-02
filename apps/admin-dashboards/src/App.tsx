import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider } from './contexts/AuthContext'
import { RoleLayout } from './layouts/RoleLayout'
import Login from './pages/Login'
import Unauthorized from './pages/Unauthorized'

import VCOverview from './pages/vc/VCOverview'
import DQAThresholds from './pages/dqa/DQAThresholds'
import DQAEligibility from './pages/dqa/DQAEligibility'
import QALiveSessions from './pages/qa/QALiveSessions'
import QADeviceReset from './pages/qa/QADeviceReset'
import AdminTenants from './pages/admin/AdminTenants'
import AdminUsers from './pages/admin/AdminUsers'

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/login"        element={<Login />} />
          <Route path="/unauthorized" element={<Unauthorized />} />

          {/* ── VC ─────────────────────────────────────────────────────── */}
          <Route element={<RoleLayout allowedRoles={['VC']} />}>
            <Route path="/vc" element={<VCOverview />} />
          </Route>

          {/* ── DQA Director ───────────────────────────────────────────── */}
          <Route element={<RoleLayout allowedRoles={['DQA_DIRECTOR']} />}>
            <Route path="/dqa/thresholds"  element={<DQAThresholds />} />
            <Route path="/dqa/eligibility" element={<DQAEligibility />} />
          </Route>

          {/* ── QA Officer ─────────────────────────────────────────────── */}
          <Route element={<RoleLayout allowedRoles={['QA_OFFICER']} />}>
            <Route path="/qa/live"         element={<QALiveSessions />} />
            <Route path="/qa/device-reset" element={<QADeviceReset />} />
          </Route>

          {/* ── Platform Admin ─────────────────────────────────────────── */}
          <Route element={<RoleLayout allowedRoles={['ADMIN']} />}>
            <Route path="/admin/tenants"                      element={<AdminTenants />} />
            <Route path="/admin/tenants/:tenantId/users"      element={<AdminUsers />} />
          </Route>

          <Route path="*" element={<Navigate to="/login" replace />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  )
}
