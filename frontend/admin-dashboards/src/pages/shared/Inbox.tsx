import RecordTabs, { type RecordTab } from '../../components/RecordTabs'
import { useAuth, type Role } from '../../contexts/AuthContext'
import Alerts from './Alerts'
import Messages from './Messages'
import QAMonitorBriefing from '../qa/QAMonitorBriefing'

/**
 * EVERYTHING ADDRESSED TO YOU, IN ONE PLACE.
 *
 * Correspondence was scattered across up to three sidebar entries per role, and the split was not
 * one a reader could predict — it followed which backend a message happened to travel over:
 *
 *   • Alerts    → /api/v1/app-notifications, the cross-role notice board the phone app also polls
 *   • Messages  → /api/v1/messages, the QA channel between desks, with attachments
 *
 * That distinction is real and the two systems stay separate underneath — different audiences,
 * different retention, one of them reaching handsets in the field. But it is an implementation
 * fact, and it was being used as navigation. A director had "Alerts", "Messages" and "Message QA
 * Monitors" as three peers in one sidebar and no way to know which held the thing they were
 * looking for, so the answer was to open all three. The labels drifted to match: HOD and DEAN
 * both called their entry "Alerts" while pointing it at /…/messages.
 *
 * So the split moves inside: ONE feature per dashboard, tabs within it. Nothing about how a
 * message is sent, stored or authorised changes — this is the shelf they are read from.
 *
 * The tab set is derived from the role rather than fixed, because the two APIs do not admit the
 * same roles and a tab that is certain to 403 is worse than an absent one. The lists below mirror
 * the router exactly:
 *
 *   Alerts    — inboxRoles in router.go: everyone with a dashboard except TLC.
 *   Messages  — DQA_DIRECTOR, QA_OFFICER, QA_DEPT_REP, QA_SCHOOL_HANDLER.
 *   Monitors  — the two roles that RUN the round and can brief it.
 *
 * TLC is deliberately absent from all three: it is not in inboxRoles, so the Teaching & Learning
 * Centre has no inbox to show and gets no entry rather than an empty page.
 */

// Mirrors `inboxRoles` in backend/api-gateway/internal/router/router.go, intersected with the
// roles that actually have a web sidebar. STUDENT, COORDINATOR, LECTURER and QA_PATROLLER are in
// that list too but read the same inbox on the phone; DVC is in it on the server and is not part
// of this app's Role union at all.
const ALERT_ROLES: Role[] = [
  'VC', 'DQA_DIRECTOR', 'QA_OFFICER', 'HOD', 'DEAN',
  'QA_DEPT_REP', 'QA_SCHOOL_HANDLER', 'ADMIN',
]

// Mirrors the RequireRole set on /api/v1/messages.
const MESSAGE_ROLES: Role[] = ['DQA_DIRECTOR', 'QA_OFFICER', 'QA_DEPT_REP', 'QA_SCHOOL_HANDLER']

// Who may brief the monitors walking the round.
const MONITOR_BRIEF_ROLES: Role[] = ['DQA_DIRECTOR', 'QA_OFFICER']

export default function Inbox() {
  const { user } = useAuth()
  const role = user?.role

  const tabs: RecordTab[] = []
  if (role && ALERT_ROLES.includes(role)) {
    tabs.push({
      id: 'alerts',
      label: 'Alerts',
      hint: 'Notices sent to you across roles — the same inbox the phone app shows.',
      render: () => <Alerts />,
    })
  }
  if (role && MESSAGE_ROLES.includes(role)) {
    tabs.push({
      id: 'messages',
      label: 'Messages',
      hint: 'The QA channel, with attachments. Between the directorate and QA officers.',
      render: () => <Messages />,
    })
  }
  if (role && MONITOR_BRIEF_ROLES.includes(role)) {
    tabs.push({
      id: 'monitors',
      label: 'Message QA Monitors',
      hint: 'Write to the monitors on the round. Arrives on the handset they are carrying.',
      render: () => <QAMonitorBriefing />,
    })
  }

  // Reachable only if a role were routed here without belonging to any of the three sets — a
  // routing mistake rather than a state a user can produce. Say so plainly instead of rendering
  // an empty tab strip that looks like a page still loading.
  if (tabs.length === 0) {
    return (
      <div>
        <h2 style={{ margin: '0 0 10px' }}>Messages &amp; Alerts</h2>
        <p style={{ color: 'var(--muted)' }}>This role has no inbox.</p>
      </div>
    )
  }

  // A plain string prop, not JSX text — "&amp;" here would reach the DOM verbatim.
  return <RecordTabs title="Messages & Alerts" tabs={tabs} />
}
