package handlers

// The four columns every lecturer-attendance log carries, in one place.
//
// A session log is read on four different screens — the admin's log page, the oversight
// dashboards, the lecturer's own dashboard, and the downloads built from them — and each one
// used to build its own SELECT. Adding a column meant finding all four and adding it four times,
// which is how they drift: the same lecture, four descriptions, and no way to tell which screen is
// wrong. The SQL below is shared so that cannot happen.
//
// WHAT THEY ARE, AND WHY EACH ONE IS DERIVED RATHER THAN STORED:
//
//	students_attended  How many students were actually marked present in that session. Counted
//	                   from attendance_logs, the student check-in ledger itself, so it cannot fall
//	                   out of step with the roster the way a cached total would. "The lecturer was
//	                   present" and "nobody came" are very different facts about a lecture, and the
//	                   log used to record only the first.
//
//	class_group        Which cohort sat in the room, as YEAR:SEMESTER — "2:1" for year 2 semester 1.
//	                   Read from the session's offering, because one unit is commonly taught to
//	                   several cohorts in the same week; without this, four rows for the same unit
//	                   on the same day are indistinguishable, and the obvious reading — a duplicate
//	                   — is the wrong one.
//
//	qa_monitor         The QA monitor who independently witnessed this lecture, if one did. Matched
//	                   on lecturer + unit + date rather than session_id, because the monitor's round
//	                   is deliberately not tied to the coordinator's session: the monitor walks into
//	                   rooms, not into records. Naming them puts a person behind the second opinion,
//	                   which is what makes it answerable.
//
//	is_compensation    Whether that monitor recorded the lecture as making good an earlier one. See
//	                   migration 083: it is stored on the monitor's record because the monitor is
//	                   the only witness in a position to establish it, and derived everywhere else
//	                   so there is one source of truth rather than two that can disagree.
//
// The monitor join is LEFT JOIN LATERAL … LIMIT 1: a lecture can be visited more than once in a
// day, and the log needs one row per lecture, not one per visit. It takes the visit that recorded a
// compensation first, then the most recent — so a compensation is never hidden by a later plain
// tick, which would silently undo the one fact the monitor went out of their way to record.
const lecturerLogColumns = `
	    (SELECT COUNT(*) FROM attendance_logs al WHERE al.session_id = lal.session_id)  AS students_attended,
	    COALESCE(
	        (SELECT o.study_year || ':' || o.semester
	           FROM sessions ss
	           JOIN course_offerings o ON o.offering_id = ss.offering_id
	          WHERE ss.session_id = lal.session_id),
	        -- No offering on the session (an older log, or a unit taught outside one): fall back to
	        -- the unit's own place in the curriculum, which is the same answer whenever a unit is
	        -- taught to a single cohort.
	        NULLIF(COALESCE(cu.year::text,'') || ':' || COALESCE(cu.semester::text,''), ':'),
	        ''
	    )                                                                              AS class_group,
	    COALESCE(mon.monitor_name, '')                                                 AS qa_monitor,
	    COALESCE(mon.monitor_staff_id, '')                                             AS qa_monitor_staff_id,
	    COALESCE(mon.is_compensation, false)                                           AS is_compensation,
	    COALESCE(mon.compensation_for::text, '')                                       AS compensation_for,
	    -- WHERE it was taught, and whether that was the timetabled room. A provision is a room
	    -- the coordinator substituted at the door (migration 085); without it on the log, a
	    -- monitor's "not taught" against the timetabled room and the lecturer's account of
	    -- teaching are two records that cannot be reconciled by anyone reading them later.
	    COALESCE((SELECT COALESCE(NULLIF(v.name,''), ss.venue_id)
	                FROM sessions ss
	                LEFT JOIN venues v ON v.venue_id = ss.venue_id
	               WHERE ss.session_id = lal.session_id), '')                             AS room,
	    COALESCE((SELECT ss.room_is_provision FROM sessions ss
	               WHERE ss.session_id = lal.session_id), false)                          AS room_is_provision,
	    COALESCE((SELECT ss.provision_note FROM sessions ss
	               WHERE ss.session_id = lal.session_id), '')                             AS provision_note,
	    -- The lecture was not on the timetable at all (migration 086). Distinct from a provision:
	    -- that is the right lecture in the wrong room, this is a lecture the schedule never had.
	    -- Both matter to a reader comparing this record with the monitor's.
	    COALESCE((SELECT ss.unscheduled FROM sessions ss
	               WHERE ss.session_id = lal.session_id), false)                          AS unscheduled,
	    -- HOW it was delivered (migration 087). An ONLINE row is a distance / e-learning class: the
	    -- lecturer started it themselves from wherever they were and the students checked in with
	    -- the rotating code, with no room and no proximity gate. That is a weaker proof than
	    -- standing in a hall, so it travels with the row rather than being left for a reader to
	    -- assume — a blank room on an online lecture is not a missing room, and the absence of a QA
	    -- monitor is not an unvisited one.
	    COALESCE((SELECT ss.delivery_mode FROM sessions ss
	               WHERE ss.session_id = lal.session_id), 'IN_PERSON')                    AS delivery_mode`

// lecturerLogMonitorJoin resolves the monitor record for the log aliased `lal`. Kept next to the
// columns above because neither is valid without the other.
const lecturerLogMonitorJoin = `
	LEFT JOIN LATERAL (
	    SELECT COALESCE(pl.patroller_name, '')     AS monitor_name,
	           COALESCE(pl.patroller_staff_id, '') AS monitor_staff_id,
	           pl.is_compensation,
	           pl.compensation_for
	      FROM lecturer_patrol_logs pl
	     WHERE pl.tenant_id    = lal.tenant_id
	       AND pl.unit_id      = lal.unit_id
	       AND pl.session_date = lal.session_date
	       AND pl.lecturer_id  IN (lal.lecturer_id, COALESCE(l.staff_id, ''))
	     ORDER BY pl.is_compensation DESC, pl.taken_at DESC
	     LIMIT 1
	) mon ON true`

// classGroup renders a cohort's place in the curriculum the way the institution says it: year 2
// semester 1 is "2:1". A zero in either half means the cohort's position is not recorded, and the
// honest answer is an empty cell rather than "0:0", which reads like a real class.
func classGroup(year, semester int) string {
	if year <= 0 || semester <= 0 {
		return ""
	}
	return itoa(year) + ":" + itoa(semester)
}

// lecturerLogExtras is the tail of every log row. Embedding it keeps the JSON field names — and so
// the column headings on four screens and three download formats — identical by construction.
type lecturerLogExtras struct {
	StudentsAttended int    `json:"students_attended"`
	ClassGroup       string `json:"class_group"`
	QAMonitor        string `json:"qa_monitor"`
	QAMonitorStaffID string `json:"qa_monitor_staff_id"`
	IsCompensation   bool   `json:"is_compensation"`
	CompensationFor  string `json:"compensation_for"`
	Room             string `json:"room"`
	RoomIsProvision  bool   `json:"room_is_provision"`
	ProvisionNote    string `json:"provision_note"`
	Unscheduled      bool   `json:"unscheduled"`
	DeliveryMode     string `json:"delivery_mode"`
}

// scanTargets returns the destinations for the shared columns, in the order lecturerLogColumns
// declares them, so a caller appends one spread rather than six arguments in a remembered order.
func (e *lecturerLogExtras) scanTargets() []interface{} {
	return []interface{}{
		&e.StudentsAttended, &e.ClassGroup, &e.QAMonitor,
		&e.QAMonitorStaffID, &e.IsCompensation, &e.CompensationFor,
		&e.Room, &e.RoomIsProvision, &e.ProvisionNote, &e.Unscheduled, &e.DeliveryMode,
	}
}
