/**
 * A room that was substituted for the timetabled one.
 *
 * The reason this is loud rather than a quiet footnote: a QA monitor's round is built from the
 * timetable, so when a lecture moves the monitor walks to the OLD room, finds it empty, and files
 * "not taught" against a lecturer who was teaching thirty metres away. This tag is the fact that
 * reconciles the two records — without it, a reader comparing them has an unexplained
 * contradiction and no way to tell which side was wrong.
 */
export default function ProvisionTag({ note }: { note?: string }) {
  return (
    <span
      title={note ? `Room provision — ${note}` : 'Taught in a room other than the timetabled one'}
      style={{
        marginLeft: 6, padding: '1px 6px', borderRadius: 4, fontSize: 10.5, fontWeight: 700,
        letterSpacing: 0.3, whiteSpace: 'nowrap', verticalAlign: 'middle',
        background: 'var(--info-bg,#e0f2fe)', color: 'var(--info-fg,#075985)',
        border: '1px solid var(--info-border,#7dd3fc)',
      }}
    >
      PROVISION
    </span>
  )
}
