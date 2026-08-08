/**
 * A lecture taught to make good an earlier one.
 *
 * Compensations are a normal part of a semester — a public holiday, an illness, a room clash — and
 * until the QA monitor could record one, every record the institution kept read them as an anomaly:
 * a lecture in a room the timetable says is empty, a missed lecture that stays missed forever, a
 * lecturer whose hours do not add up. Marking it is what turns three separate-looking mistakes back
 * into one explained event.
 *
 * It is deliberately loud. A reader scanning a column of dates needs to see at a glance that THIS
 * row is not where the timetable says it should be, because that is the exact thing they would
 * otherwise flag as wrong.
 */
export default function CompensationTag({ forDate }: { forDate?: string }) {
  return (
    <span
      title={forDate ? `Compensation for the lecture of ${forDate}` : 'Recorded by the QA monitor as a compensation lecture'}
      style={{
        marginLeft: 6, padding: '1px 6px', borderRadius: 4, fontSize: 10.5, fontWeight: 700,
        letterSpacing: 0.3, whiteSpace: 'nowrap', verticalAlign: 'middle',
        background: 'var(--warn-bg,#fef3c7)', color: 'var(--warn-fg,#92400e)',
        border: '1px solid var(--warn-border,#fcd34d)',
      }}
    >
      COMP{forDate ? ` · ${forDate}` : ''}
    </span>
  )
}
