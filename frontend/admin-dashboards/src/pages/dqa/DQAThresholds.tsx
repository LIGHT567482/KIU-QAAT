/**
 * Attendance policy, as displayed to the DQA director.
 *
 * This was an editable form — the DQA's landing page, in fact, which is why they
 * now land on DQAHome instead. The threshold and the auto-close are fixed
 * institution-wide in the backend (internal/policy), so the form is gone: a field
 * that accepts a number and then silently discards it is worse than no field,
 * because the person walks away believing they changed something.
 *
 * The editor is preserved in git history and the tenants columns are untouched, so
 * restoring per-tenant policy means putting the form back and un-commenting
 * PutThresholds — not reconstructing either.
 */

interface PolicyFact {
  label: string
  value: string
  why: string
}

const FACTS: PolicyFact[] = [
  {
    label: 'Attendance threshold',
    value: '75%',
    why: 'A student below this is ineligible to sit exams. Applies to every course and every cohort.',
  },
  {
    label: 'Session auto-close',
    value: '15 minutes after the timetabled end',
    why: 'A session nobody closed shuts itself 15 minutes after the lecture was due to finish, so attendance cannot be added to it afterwards.',
  },
]

export default function DQAThresholds() {
  return (
    <div style={{ maxWidth: 620 }}>
      <h2 style={{ marginBottom: 4 }}>Attendance Policy</h2>
      <p style={{ color: 'var(--muted)', marginBottom: 24 }}>
        These are institution-wide and fixed. They are shown here so the numbers every
        report is judged against are visible in one place.
      </p>

      <div style={{ display: 'grid', gap: 18 }}>
        {FACTS.map(f => (
          <div key={f.label} style={{
            padding: 16, borderRadius: 12,
            background: 'var(--surface,#fff)', border: '1px solid var(--border,#e2e8f0)',
          }}>
            <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--muted)', marginBottom: 6 }}>
              {f.label}
            </div>
            <div style={{ fontSize: 22, fontWeight: 800, color: 'var(--text,#0f172a)', marginBottom: 6 }}>
              {f.value}
            </div>
            <div style={{ fontSize: 13, color: 'var(--muted)', lineHeight: 1.5 }}>{f.why}</div>
          </div>
        ))}
      </div>

      <p style={{ color: 'var(--muted)', fontSize: 12, marginTop: 20, lineHeight: 1.6 }}>
        The daily session window — the hours and weekdays during which a coordinator may
        open a session at all — is still configurable, under{' '}
        <strong>Administration → Settings</strong>.
      </p>
    </div>
  )
}
