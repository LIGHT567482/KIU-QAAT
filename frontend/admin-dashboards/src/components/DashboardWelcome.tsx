import { useAuth, type Role } from '../contexts/AuthContext'

/**
 * The greeting every dashboard carries: title, name, role and today's date.
 *
 * There was already a WelcomeToast, but it is a four-second pill that fires once
 * at sign-in and is gone before most people have finished reading the page — and
 * only the ADMIN home had anything persistent. So on every other dashboard there
 * was nothing at all telling you whose account you were looking at, which matters
 * on a console where one machine is shared between a dean, an HOD and the QA
 * office and the pages look broadly alike.
 *
 * Rendered by RoleLayout above the outlet, so a new dashboard gets it by existing
 * rather than by remembering to add it.
 */

// What to call each role in the greeting. Deliberately the human title rather than
// the enum: "Head of Department", not "HOD".
const ROLE_LABEL: Record<Role, string> = {
  VC: 'Vice-Chancellor',
  DQA_DIRECTOR: 'Director of Quality Assurance',
  QA_OFFICER: 'Quality Assurance Officer',
  COORDINATOR: 'Course Coordinator',
  ADMIN: 'Administrator',
  LECTURER: 'Lecturer',
  HOD: 'Head of Department',
  DEAN: 'Dean',
  QA_SCHOOL_HANDLER: 'QA School Handler',
  QA_DEPT_REP: 'QA Department Representative',
  TLC: 'Teaching & Learning Centre',
  // No web dashboard — present only so the type covers every role sign-in can return.
  QA_PATROLLER: 'QA Monitor',
  STUDENT: 'Student',
}

function partOfDay(h: number): string {
  if (h < 12) return 'Good morning'
  if (h < 17) return 'Good afternoon'
  return 'Good evening'
}

export default function DashboardWelcome() {
  const { user } = useAuth()
  if (!user) return null

  // A token minted before the login response carried these will have neither.
  // Greet by role in that case rather than rendering "Welcome, undefined".
  const name = [user.title, user.fullName].filter(Boolean).join(' ').trim()
  const now = new Date()
  const greeting = partOfDay(now.getHours())
  const dateStr = now.toLocaleDateString(undefined, {
    weekday: 'long', day: 'numeric', month: 'long', year: 'numeric',
  })

  return (
    <div
      style={{
        display: 'flex', alignItems: 'baseline', gap: 10, flexWrap: 'wrap',
        padding: '12px 16px', marginBottom: 16, borderRadius: 10,
        background: 'var(--surface-2, #f8fafc)', border: '1px solid var(--border, #e2e8f0)',
      }}
    >
      <span style={{ fontSize: 16, fontWeight: 700, color: 'var(--text, #0f172a)' }}>
        {greeting}{name ? `, ${name}` : ''} 👋
      </span>
      <span style={{ fontSize: 13, color: 'var(--muted, #64748b)' }}>
        {ROLE_LABEL[user.role] ?? user.role} · {dateStr}
      </span>
    </div>
  )
}
