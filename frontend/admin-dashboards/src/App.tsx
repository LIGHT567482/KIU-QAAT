import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider } from './contexts/AuthContext'
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
import QADeviceReset from './pages/qa/QADeviceReset'
import QAManualCorrection from './pages/qa/QAManualCorrection'
import QACoordinatorHealth from './pages/qa/QACoordinatorHealth'
import AdminHome from './pages/admin/AdminTenants'
import AdminSettings from './pages/admin/AdminSettings'
import AdminUsers from './pages/admin/AdminUsers'
import AdminCourses from './pages/admin/AdminCourses'
import AdminCourseUnits from './pages/admin/AdminCourseUnits'
import AdminStudents from './pages/admin/AdminStudents'
import AdminRooms from './pages/admin/AdminRooms'
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
import { QAOrgLecturers, QAOrgDepartments, QAOrgReports } from './pages/qa/QAOrgDashboard'
import Timetable from './pages/shared/Timetable'
import Messages from './pages/shared/Messages'
import LecturerPortal from './pages/LecturerPortal'
import OrgOverview from './pages/shared/OrgOverview'
import AtRisk from './pages/shared/AtRisk'
import AdminAudit from './pages/admin/AdminAudit'
import Alerts from './pages/shared/Alerts'
import OrgDepartments from './pages/shared/OrgDepartments'

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <WelcomeToast />
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
            <Route path="/dqa/qa-reports"          element={<QAReports />} />
            <Route path="/dqa/messages"            element={<Messages />} />
          </Route>

          {/* ── QA Officer ─────────────────────────────────────────────── */}
          <Route element={<RoleLayout allowedRoles={['QA_OFFICER']} />}>
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
            <Route path="/admin/tenants/:tenantId/rooms"              element={<AdminRooms />} />
            {/* /venues is the old path for the same page — kept so existing links resolve. */}
            <Route path="/admin/tenants/:tenantId/venues"             element={<AdminRooms />} />
            <Route path="/admin/courses/:courseId/units"              element={<AdminCourseUnits />} />
            <Route path="/admin/tenants/:tenantId/coordinators"          element={<AdminCoordinators />} />
            <Route path="/admin/tenants/:tenantId/lecturers"              element={<AdminLecturers />} />
            <Route path="/admin/tenants/:tenantId/lecturer-assignments"  element={<AdminLecturerAssignments />} />
            <Route path="/admin/tenants/:tenantId/lecturer-attendance"   element={<AdminLecturerAttendance />} />
            <Route path="/admin/tenants/:tenantId/employees"             element={<AdminEmployees />} />
            <Route path="/admin/tenants/:tenantId/employee-attendance"   element={<AdminEmployeeAttendance />} />
            <Route path="/admin/tenants/:tenantId/student-attendance"    element={<QAStudentAttendance />} />
            <Route path="/admin/reports"                                 element={<AdminReports />} />
            <Route path="/admin/at-risk"                                 element={<AtRisk />} />
            <Route path="/admin/audit"                                   element={<AdminAudit />} />
          </Route>

          {/* ── Lecturer (own assigned units) ──────────────────────────── */}
          <Route element={<RoleLayout allowedRoles={['LECTURER']} />}>
            <Route path="/lecturer" element={<LecturerDashboard />} />
          </Route>

          {/* ── HOD (own department) / Dean (own school) ─────────────────
              Both landed on a bare lecturer list with no sense of whether their unit was
              working. They now open on the KPI overview, with the lecturer list, the
              at-risk watchlist, the timetable and their inbox alongside it. Every page is
              scoped server-side by the unit on their own account. */}
          <Route element={<RoleLayout allowedRoles={['HOD']} />}>
            <Route path="/hod"           element={<OrgOverview level="hod" />} />
            <Route path="/hod/lecturers" element={<OrgLecturers level="hod" />} />
            <Route path="/hod/at-risk"   element={<AtRisk />} />
            <Route path="/hod/attendance" element={<QAStudentAttendance />} />
            <Route path="/hod/timetable" element={<Timetable readOnly />} />
            <Route path="/hod/messages"  element={<Alerts />} />
          </Route>
          <Route element={<RoleLayout allowedRoles={['DEAN']} />}>
            <Route path="/dean"            element={<OrgOverview level="dean" />} />
            {/* The management layer a dean is accountable THROUGH — skipped entirely before. */}
            <Route path="/dean/departments" element={<OrgDepartments />} />
            <Route path="/dean/lecturers"  element={<OrgLecturers level="dean" />} />
            <Route path="/dean/at-risk"   element={<AtRisk />} />
            <Route path="/dean/attendance" element={<QAStudentAttendance />} />
            <Route path="/dean/timetable" element={<Timetable readOnly />} />
            <Route path="/dean/messages"  element={<Alerts />} />
          </Route>

          {/* ── QA reps: department rep / school handler ────────────────── */}
          <Route element={<RoleLayout allowedRoles={['QA_DEPT_REP']} />}>
            <Route path="/qa-dept"          element={<OrgOverview level="qa-dept" />} />
            <Route path="/qa-dept/lecturers" element={<QAOrgLecturers />} />
            <Route path="/qa-dept/at-risk"  element={<AtRisk />} />
            <Route path="/qa-dept/report"   element={<QAOrgReports />} />
            <Route path="/qa-dept/messages" element={<Messages />} />
          </Route>
          <Route element={<RoleLayout allowedRoles={['QA_SCHOOL_HANDLER']} />}>
            <Route path="/qa-school"           element={<OrgOverview level="qa-school" />} />
            <Route path="/qa-school/departments" element={<OrgDepartments />} />
            <Route path="/qa-school/qa-departments" element={<QAOrgDepartments />} />
            <Route path="/qa-school/lecturers" element={<QAOrgLecturers />} />
            <Route path="/qa-school/at-risk"   element={<AtRisk />} />
            <Route path="/qa-school/reports"   element={<QAOrgReports />} />
            <Route path="/qa-school/messages"  element={<Messages />} />
          </Route>

          <Route path="*" element={<Navigate to="/login" replace />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  )
}
