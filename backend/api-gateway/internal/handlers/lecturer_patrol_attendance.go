package handlers

// Lecturer attendance, PATROL source — the second, independent record of whether a lecturer
// actually taught.
//
// A lecture is now witnessed twice:
//
//	1. the COORDINATOR opens the session and the lecturer starts/ends it   → lecturer_attendance_logs
//	2. a QA PATROLLER walks the room and ticks taught / not taught         → lecturer_patrol_logs
//
// They are deliberately NOT merged. The coordinator's record is a contact-hours ledger the lecturer
// participated in creating; the patroller's is an outside spot-check that can contradict it. Merging
// would destroy exactly the disagreement QA exists to find, so each keeps its own page and this file
// serves the patrol side:
//
//	GET /api/v1/dashboard/lecturer-attendance/patrol           — per-lecturer summary + every visit
//	GET /api/v1/dashboard/lecturer-attendance/patrol/export.*  — the same, downloadable

import (
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// patrolVisit is one room visit by a QA monitor.
type patrolVisit struct {
	PatrolID string `json:"patrol_id"`
	// lecturer_patrol_logs keys lecturers by STAFF ID — that is what the monitor's cached
	// timetable carries — so LecturerID is a staff id, and the name is resolved separately.
	LecturerID   string `json:"lecturer_id"`
	LecturerName string `json:"lecturer_name"`
	UnitID       string `json:"unit_id"`
	UnitName     string `json:"unit_name"`
	Room         string `json:"room"`
	SessionDate  string `json:"session_date"`
	Scheduled    string `json:"scheduled_time"`
	Taught       bool   `json:"taught"`
	// patroller_name / patroller_staff_id are also emitted as qa_monitor / qa_monitor_staff_id.
	// The old names stay so the handsets already in the field keep parsing this; the new ones are
	// what every screen and download now reads, because "monitor" is what the institution calls
	// the person doing this work.
	Patroller        string `json:"patroller_name"`
	PatrollerID      string `json:"patroller_staff_id"`
	QAMonitor        string `json:"qa_monitor"`
	QAMonitorStaffID string `json:"qa_monitor_staff_id"`
	TakenAt          string `json:"taken_at"`
	ClassGroup       string `json:"class_group"`
	StudentsAttended int    `json:"students_attended"`
	IsCompensation   bool   `json:"is_compensation"`
	CompensationFor  string `json:"compensation_for"`
	// Where the observation came from: PATROL (a tick against a timetabled slot), MANUAL (a
	// lecture with no slot, described by the monitor from scratch), QA_REP_UPLOAD (a workbook).
	// Kept on the row because they are not the same kind of evidence and a reader has to be able
	// to tell — a headcount taken by eye and a check-in register both say "42".
	EntryMethod string `json:"entry_method"`
	School      string `json:"school"`
}

// patrolLecturer aggregates one lecturer's visits.
type patrolLecturer struct {
	LecturerID   string  `json:"lecturer_id"`
	LecturerName string  `json:"lecturer_name"`
	Department   string  `json:"department"`
	School       string  `json:"school"`
	Patrolled    int     `json:"patrolled"`
	Taught       int     `json:"taught"`
	Missed       int     `json:"missed"`
	Rate         float64 `json:"rate"`
	LastPatrol   string  `json:"last_patrol_date"`
}

// queryPatrolAttendance returns the per-lecturer summary and the flat visit list, both scoped to
// the caller's tenant and optionally to a date range.
func queryPatrolAttendance(r *http.Request, pool *pgxpool.Pool) ([]patrolLecturer, []patrolVisit, error) {
	tenantID := middleware.GetTenantID(r.Context())
	conn, err := pool.Acquire(r.Context())
	if err != nil {
		return nil, nil, err
	}
	defer conn.Release()
	if err := middleware.SetTenantConn(r.Context(), conn, tenantID); err != nil {
		return nil, nil, err
	}

	args := []interface{}{tenantID}
	where := ""
	if v := strings.TrimSpace(r.URL.Query().Get("from")); v != "" {
		args = append(args, v)
		where += " AND p.session_date >= $2::date"
	}
	if v := strings.TrimSpace(r.URL.Query().Get("to")); v != "" {
		args = append(args, v)
		where += " AND p.session_date <= $" + itoa(len(args)) + "::date"
	}
	// ORG SCOPE, for the college- and department-bounded QA roles now reading this page. Resolved
	// from the account and appended AFTER the caller's own date filters, so it cannot be widened
	// by anything in the request. Institution-wide roles add nothing.
	//
	// The scope names `c.department` / `c.school`, so the queries below join `courses c` through
	// the patrolled unit — the patrol log itself carries a free-text school and no department at
	// all, and scoping on that would quietly miss every row written before the column existed.
	where += lecturerLogScope(r, pool, tenantID, &args)

	// Visits, newest first — the detail rows behind each lecturer.
	vRows, err := conn.Query(r.Context(), `
		SELECT p.patrol_id::text, COALESCE(p.lecturer_id,''),
		       COALESCE(l.full_name, NULLIF(p.lecturer_name,''), p.lecturer_id, ''),
		       p.unit_id, COALESCE(p.unit_name,''),
		       COALESCE(p.room,''), p.session_date::text, COALESCE(p.scheduled_time,''),
		       p.taught, COALESCE(p.patroller_name,''), COALESCE(p.patroller_staff_id,''),
		       p.taken_at,
		       p.is_compensation, COALESCE(p.compensation_for::text,''),
		       -- Which cohort was in the room, as YEAR:SEMESTER. The monitor's record carries the
		       -- offering, so the same unit visited for two different intakes on one day is two
		       -- distinguishable rows rather than an apparent duplicate. A MANUAL entry has no
		       -- offering, so it carries the cohort the monitor typed instead.
		       COALESCE(o.study_year,0), COALESCE(o.semester,0), COALESCE(p.class_group,''),
		       -- How many students. For a timetabled lecture that is the roll the coordinator's
		       -- session took; for a manual one there IS no session, so it is the monitor's own
		       -- headcount. Which of the two this is comes from entry_method, and the two are
		       -- never added together — a register and an estimate are different claims.
		       COALESCE(p.students_counted,
		                (SELECT COUNT(*) FROM attendance_logs al
		                  JOIN sessions ss ON ss.session_id = al.session_id
		                 WHERE ss.unit_id = p.unit_id
		                   AND ss.session_date = p.session_date), 0),
		       COALESCE(p.entry_method,'PATROL'), COALESCE(p.school,'')
		FROM lecturer_patrol_logs p
		LEFT JOIN course_offerings o ON o.offering_id = p.offering_id
		LEFT JOIN lecturers l ON l.staff_id = p.lecturer_id
		-- Joined for the org scope appended below (c.department / c.school). The rollup query
		-- further down already had these; the visit rows did not, so a bounded caller would have
		-- got a summary of their own college beside a visit list of the whole institution.
		LEFT JOIN course_units cu ON cu.unit_id = p.unit_id
		LEFT JOIN courses c ON c.course_id = cu.course_id
		WHERE p.tenant_id = $1`+where+`
		ORDER BY p.session_date DESC, p.scheduled_time DESC`, args...)
	if err != nil {
		return nil, nil, err
	}
	visits := []patrolVisit{}
	for vRows.Next() {
		var v patrolVisit
		var takenAt time.Time
		var yr, sm int
		var typedClassGroup string
		if vRows.Scan(&v.PatrolID, &v.LecturerID, &v.LecturerName, &v.UnitID, &v.UnitName, &v.Room,
			&v.SessionDate, &v.Scheduled, &v.Taught, &v.Patroller, &v.PatrollerID, &takenAt,
			&v.IsCompensation, &v.CompensationFor, &yr, &sm, &typedClassGroup,
			&v.StudentsAttended, &v.EntryMethod, &v.School) == nil {
			v.TakenAt = takenAt.Format(time.RFC3339)
			// The offering is the better answer where there is one; what the monitor typed is the
			// only answer where there is not.
			if v.ClassGroup = classGroup(yr, sm); v.ClassGroup == "" {
				v.ClassGroup = typedClassGroup
			}
			v.QAMonitor, v.QAMonitorStaffID = v.Patroller, v.PatrollerID
			visits = append(visits, v)
		}
	}
	vRows.Close()

	// Per-lecturer rollup. lecturer_patrol_logs keys lecturers by STAFF ID (that is what the
	// patroller's cached timetable carries), so the join back to `lecturers` is on staff_id.
	sRows, err := conn.Query(r.Context(), `
		SELECT COALESCE(p.lecturer_id,''),
		       COALESCE(MAX(l.full_name), MAX(p.lecturer_name), ''),
		       COALESCE(MAX(c.department),''), COALESCE(MAX(c.school),''),
		       COUNT(*)                              AS patrolled,
		       COUNT(*) FILTER (WHERE p.taught)      AS taught,
		       MAX(p.session_date)::text             AS last_patrol
		FROM lecturer_patrol_logs p
		LEFT JOIN lecturers l   ON l.staff_id = p.lecturer_id
		LEFT JOIN course_units cu ON cu.unit_id = p.unit_id  
		LEFT JOIN courses c     ON c.course_id = cu.course_id
		WHERE p.tenant_id = $1`+where+`
		GROUP BY p.lecturer_id
		ORDER BY 2`, args...)
	if err != nil {
		return nil, nil, err
	}
	summary := []patrolLecturer{}
	for sRows.Next() {
		var s patrolLecturer
		if sRows.Scan(&s.LecturerID, &s.LecturerName, &s.Department, &s.School,
			&s.Patrolled, &s.Taught, &s.LastPatrol) == nil {
			s.Missed = s.Patrolled - s.Taught
			if s.Patrolled > 0 {
				s.Rate = float64(int(float64(s.Taught)/float64(s.Patrolled)*1000+0.5)) / 10
			}
			summary = append(summary, s)
		}
	}
	sRows.Close()

	return summary, visits, nil
}

// GET /api/v1/dashboard/lecturer-attendance/patrol
func LecturerPatrolAttendance(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		summary, visits, err := queryPatrolAttendance(r, pool)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"summary": summary,
			"visits":  visits,
		})
	}
}

// patrolAttendanceTable flattens the monitor's record for download.
//
// It downloads the VISITS, not the per-lecturer rollup it used to. The rollup answers "how is this
// lecturer doing", which the screen already shows; the download is asked for when someone needs the
// underlying observations — and an observation with no observer on it is not evidence of anything.
// So every row now names the QA monitor who took it, alongside the cohort that was in the room, the
// roll the coordinator recorded, and whether the monitor logged it as a compensation.
func patrolAttendanceTable(r *http.Request, pool *pgxpool.Pool) (reportTable, error) {
	_, visits, err := queryPatrolAttendance(r, pool)
	if err != nil {
		return reportTable{}, err
	}
	t := reportTable{
		Title:    "Lecturer Attendance — QA monitor record",
		Subtitle: itoa(len(visits)) + " monitor visit(s)",
		Headers: []string{"Date", "Time", "Lecturer", "Staff ID", "Unit", "Class/Group", "School",
			"Room", "Taught", "Compensation", "For", "Students", "Source",
			"QA Monitor", "Monitor ID", "Taken at"},
		Weights: []float64{1.4, 1, 2.6, 1.4, 2.4, 1.2, 2, 1.3, 1, 1.3, 1.2, 1, 1.4, 2.4, 1.4, 1.7},
	}
	for _, v := range visits {
		taught := "No"
		if v.Taught {
			taught = "Yes"
		}
		comp := ""
		if v.IsCompensation {
			comp = "Yes"
		}
		// Named for the reader, not for the database. "Timetabled" and "Manual" say what the
		// difference actually is — this row was a tick against a scheduled lecture, or a lecture
		// the monitor found and wrote down — where PATROL and MANUAL say nothing to anyone who
		// has not read the schema.
		source := map[string]string{
			"PATROL": "Timetabled", "MANUAL": "Manual entry", "QA_REP_UPLOAD": "QA rep workbook",
		}[v.EntryMethod]
		if source == "" {
			source = v.EntryMethod
		}
		t.Rows = append(t.Rows, []string{
			v.SessionDate, v.Scheduled, v.LecturerName, v.LecturerID, v.UnitName, v.ClassGroup,
			v.School, v.Room, taught, comp, v.CompensationFor, itoaOrBlank(v.StudentsAttended),
			source, v.QAMonitor, v.QAMonitorStaffID, v.TakenAt,
		})
	}
	return t, nil
}

// GET /api/v1/dashboard/lecturer-attendance/patrol/export.{xlsx,csv,pdf}
func LecturerPatrolExport(pool *pgxpool.Pool, format string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t, err := patrolAttendanceTable(r, pool)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeReport(w, r, pool, format, "lecturer-attendance-patrol", t)
	}
}
