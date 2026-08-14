import { Link } from 'react-router-dom'

/**
 * EVERY REPORT THE DIRECTORATE CAN PRODUCE, ON ONE PAGE.
 *
 * The reports themselves were never missing — student attendance, lecturer attendance, the monitor
 * record, lecturer teaching, exam eligibility and now patrol coverage all existed, and all of them
 * export. What was missing was any single place that said so. They were scattered across the
 * sidebar under names describing the SCREEN rather than the QUESTION, so producing a report meant
 * already knowing which page happened to hold it.
 *
 * So each entry below leads with the question it answers, and says plainly what formats come out.
 * Nothing here is a new capability except where marked — this is a map, and the value is entirely
 * in not having to hold the map in your head.
 */

interface Entry {
  title: string
  question: string
  href: string
  formats: string
  note?: string
}

const REPORTS: Entry[] = [
  {
    title: 'Unit Attendance',
    question: 'Was this unit taught, and did the cohort turn up?',
    href: '/dqa/unit-attendance',
    formats: 'Excel · CSV · PDF',
    note: 'Both records side by side, and flags sessions with student check-ins but no lecturer record.',
  },
  {
    title: 'Student Attendance',
    question: 'Who is attending, and who is short of the exam threshold?',
    href: '/dqa/student-attendance',
    formats: 'Excel · CSV · PDF',
    note: 'Filterable by course, unit, study session, year and semester.',
  },
  {
    title: 'Lecturer Attendance',
    question: 'Which lectures were delivered, and for how many contact hours?',
    href: '/dqa/lecturer-attendance',
    formats: 'Excel · CSV · PDF',
    note: 'The coordinator’s record. Includes manually recorded sessions.',
  },
  {
    title: 'QA Monitor Coverage',
    question: 'How much of the timetable did the QA round actually reach?',
    href: '/dqa/monitor-coverage',
    formats: 'Excel · CSV · PDF',
    note: 'The denominator behind every monitor figure — and which slots are never visited.',
  },
  {
    title: 'Lecturer Teaching',
    question: 'What does the monitor record say was taught, across any dimension?',
    href: '/dqa/qa-reports',
    formats: 'Excel · CSV · PDF',
    note: 'The independent second record, which may disagree with the coordinator’s.',
  },
  {
    title: 'Exam Eligibility',
    question: 'Who may sit, and how many sessions must the rest still attend?',
    href: '/dqa/eligibility',
    formats: 'CSV',
  },
  {
    title: 'At-risk Students',
    question: 'Who is below the bar right now, and by how much?',
    href: '/dqa/at-risk',
    formats: 'On screen',
    note: 'Sorted so recoverable cases separate from hopeless ones.',
  },
  {
    title: 'Presence Disputes',
    question: 'Where does a lecturer’s own record contradict the monitor’s?',
    href: '/dqa/presence-claims',
    formats: 'On screen',
  },
]

export default function DQAReportsHub() {
  return (
    <div>
      <h2 style={{ margin: '0 0 4px' }}>Reports</h2>
      <p style={{ color: 'var(--muted)', margin: '0 0 20px', fontSize: 13 }}>
        Everything the directorate can generate. Each report exports exactly what its filters are
        showing, so the file matches the screen it came from.
      </p>

      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))',
        gap: 14,
      }}>
        {REPORTS.map(rep => (
          <Link
            key={rep.href}
            to={rep.href}
            style={{
              display: 'block', textDecoration: 'none', color: 'inherit',
              background: '#fff', border: '1px solid #e2e8f0', borderRadius: 10,
              padding: 16,
            }}
          >
            <div style={{ fontWeight: 700, fontSize: 15 }}>{rep.title}</div>
            <div style={{ color: 'var(--muted)', fontSize: 13, margin: '4px 0 8px' }}>
              {rep.question}
            </div>
            {rep.note && (
              <div style={{ fontSize: 11.5, color: 'var(--muted)', marginBottom: 8 }}>{rep.note}</div>
            )}
            <div style={{
              display: 'inline-block', fontSize: 11, fontWeight: 600,
              background: '#f1f5f9', color: '#475569',
              padding: '2px 8px', borderRadius: 999,
            }}>
              {rep.formats}
            </div>
          </Link>
        ))}
      </div>
    </div>
  )
}
