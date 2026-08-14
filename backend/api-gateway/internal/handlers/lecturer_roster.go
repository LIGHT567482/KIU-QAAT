package handlers

// Lecturer roster & analytics (LECTURER role). Everyone who studies his unit(s) and their
// attendance across every session he has ever had — ACROSS cohorts. Every row carries the
// sortable dimensions (cohort, course, unit, level, year, semester, intake) so the app can
// sort/filter by any of them client-side; the session endpoints cover "attended / did NOT
// attend a specific session".
//
//   GET /api/v1/lecturer/roster?scope=enrolled|attended&unit_id=
//   GET /api/v1/lecturer/sessions?unit_id=
//   GET /api/v1/lecturer/sessions/{session_id}/students?status=present|absent|all

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

func cohortLabel(year, sem int, level, intake string) string {
	parts := []string{}
	if year > 0 {
		parts = append(parts, fmt.Sprintf("Yr%d", year))
	}
	if sem > 0 {
		parts = append(parts, fmt.Sprintf("Sem%d", sem))
	}
	if level != "" {
		parts = append(parts, level)
	}
	label := strings.Join(parts, " ")
	if intake != "" {
		if label != "" {
			label += " · "
		}
		label += intake
	}
	return label
}

// LecturerRoster — students who study this lecturer's unit(s). scope=enrolled (all who
// should study), scope=attended (only those who have attended ≥1 of his sessions).
func LecturerRoster(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		lecturerID, ok := resolveLecturerID(adminPool, r, tenantID, userID)
		if !ok {
			writeJSON(w, http.StatusForbidden, errBody("NOT_A_LECTURER", "no lecturer profile for this account"))
			return
		}
		scope := r.URL.Query().Get("scope")
		unitFilter := strings.TrimSpace(r.URL.Query().Get("unit_id"))

		args := []interface{}{tenantID, lecturerID}
		unitWhere := ""
		if unitFilter != "" {
			args = append(args, unitFilter)
			unitWhere = fmt.Sprintf(" AND cu.unit_id = $%d", len(args))
		}

		// ── THE SIEVE ────────────────────────────────────────────────────────────────────────
		//
		// This joined course_offerings on the COURSE, so it returned every cohort of every course
		// containing a unit the lecturer is assigned to. A lecturer who teaches one unit to the Day
		// cohort was shown the Evening, Weekend and e-learning students of that course as well —
		// hundreds of people they have never taught, mixed in with their own, and no filter could
		// separate them because the extra rows were indistinguishable from the real ones.
		//
		// The cohorts a lecturer actually teaches are the ones with a timetable slot naming them,
		// or (for the very common imported timetable that names nobody) a slot for a unit they hold
		// the assignment to. Same rule as their timetable and the monitor's round, so all three
		// agree on whose lecture is whose.
		//
		// THE FALLBACK, and why it is safe: a unit with no timetable slot ANYWHERE has no cohorts to
		// narrow to, and showing that lecturer nobody would be worse than showing them everybody —
		// an unscheduled unit still has students who need marking. So the course-wide join survives
		// for exactly that case and no other.
		nicheWhere := `
			AND ( EXISTS (
			        SELECT 1 FROM timetable_slots ts
			         WHERE ts.offering_id = o.offering_id
			           AND ts.unit_id = cu.unit_id
			           AND ( ts.lecturer_id = $2::uuid
			              OR ( ts.lecturer_id IS NULL
			                   AND EXISTS (SELECT 1 FROM lecturer_assignments la2
			                                WHERE la2.tenant_id = ts.tenant_id
			                                  AND la2.unit_id = ts.unit_id
			                                  AND la2.lecturer_id = $2::uuid) ) ) )
			   OR NOT EXISTS (
			        SELECT 1 FROM timetable_slots ts2
			         WHERE ts2.tenant_id = cu.tenant_id AND ts2.unit_id = cu.unit_id) )`

		// The dimensions a lecturer actually sorts their students by. Applied in SQL rather than
		// shipped to the client, because a cohort filter that only hides rows locally would still
		// have counted them into the percentages below.
		for _, f := range []struct{ param, col string }{
			{"offering_id", "o.offering_id::text"},
			{"session_type", "o.session_type"},
			{"intake", "o.intake"},
			{"level", "o.level"},
			{"study_year", "o.study_year::text"},
			{"semester", "o.semester::text"},
			{"course_id", "o.course_id"},
		} {
			if v := strings.TrimSpace(r.URL.Query().Get(f.param)); v != "" {
				args = append(args, v)
				nicheWhere += fmt.Sprintf(" AND btrim(lower(COALESCE(%s,''))) = btrim(lower($%d))", f.col, len(args))
			}
		}
		attendedWhere := ""
		if scope == "attended" {
			attendedWhere = ` AND EXISTS (
				SELECT 1 FROM attendance_logs al JOIN sessions ses ON ses.session_id = al.session_id
				WHERE al.tenant_id = $1 AND al.student_id = s.student_id AND ses.unit_id = cu.unit_id)`
		}

		rows, err := adminPool.Query(r.Context(), `
			SELECT DISTINCT s.student_id, s.full_name,
			       cu.unit_id, cu.name, COALESCE(cu.level,''), COALESCE(cu.year,0), COALESCE(cu.semester,0),
			       o.course_id, COALESCE(c.name,''), o.offering_id::text,
			       COALESCE(o.study_year,0), COALESCE(o.semester,0), COALESCE(o.level,''), COALESCE(o.intake,''),
			       -- Attended, and out of HOW MANY. A count with no denominator cannot be read:
			       -- 6 sessions is excellent out of 7 and a failure out of 20, and the number that
			       -- decides exam eligibility is the ratio, not the count.
			       (SELECT count(*) FROM attendance_logs al JOIN sessions ses ON ses.session_id = al.session_id
			         WHERE al.tenant_id = $1 AND al.student_id = s.student_id AND ses.unit_id = cu.unit_id
			           AND ses.offering_id = o.offering_id) AS attended_count,
			       (SELECT count(*) FROM sessions ses
			         WHERE ses.tenant_id = $1 AND ses.unit_id = cu.unit_id
			           AND ses.offering_id = o.offering_id) AS held_count
			FROM lecturer_assignments la
			JOIN course_units cu     ON cu.unit_id   = la.unit_id 
			JOIN course_offerings o  ON o.course_id  = cu.course_id
			LEFT JOIN courses c      ON c.course_id  = o.course_id 
			JOIN students_extended s ON s.offering_id = o.offering_id AND s.enrollment_status = 'ACTIVE'
			WHERE la.tenant_id = $1 AND la.lecturer_id = $2::uuid`+unitWhere+nicheWhere+attendedWhere+`
			ORDER BY cu.name, s.full_name`, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type row struct {
			StudentID     string `json:"student_id"`
			FullName      string `json:"full_name"`
			UnitID        string `json:"unit_id"`
			UnitName      string `json:"unit_name"`
			UnitLevel     string `json:"unit_level"`
			UnitYear      int    `json:"unit_year"`
			UnitSemester  int    `json:"unit_semester"`
			CourseID      string `json:"course_id"`
			CourseName    string `json:"course_name"`
			OfferingID    string `json:"offering_id"`
			Cohort        string `json:"cohort"`
			CohortYear    int    `json:"cohort_year"`
			CohortSem     int    `json:"cohort_semester"`
			CohortLevel   string `json:"cohort_level"`
			Intake        string `json:"intake"`
			AttendedCount int    `json:"attended_count"`
			HeldCount     int    `json:"held_count"`
			// The number the exam board reads, computed once here rather than in each of the four
			// screens that show it — three of which would round it differently.
			Pct float64 `json:"pct"`
		}
		out := []row{}
		// Per-student overall across every unit THIS lecturer teaches them. A student at 90% in one
		// unit and 40% in another is not "at 65%" to anyone who matters, but the lecturer still
		// needs the single figure they will be asked for.
		type tally struct{ attended, held int }
		perStudent := map[string]*tally{}
		var totAttended, totHeld int

		for rows.Next() {
			var x row
			if rows.Scan(&x.StudentID, &x.FullName, &x.UnitID, &x.UnitName, &x.UnitLevel, &x.UnitYear, &x.UnitSemester,
				&x.CourseID, &x.CourseName, &x.OfferingID, &x.CohortYear, &x.CohortSem, &x.CohortLevel, &x.Intake,
				&x.AttendedCount, &x.HeldCount) != nil {
				continue
			}
			x.Cohort = cohortLabel(x.CohortYear, x.CohortSem, x.CohortLevel, x.Intake)
			if x.HeldCount > 0 {
				x.Pct = round1(float64(x.AttendedCount) / float64(x.HeldCount) * 100)
			}
			t := perStudent[x.StudentID]
			if t == nil {
				t = &tally{}
				perStudent[x.StudentID] = t
			}
			t.attended += x.AttendedCount
			t.held += x.HeldCount
			totAttended += x.AttendedCount
			totHeld += x.HeldCount
			out = append(out, x)
		}

		type overallRow struct {
			StudentID string  `json:"student_id"`
			Attended  int     `json:"attended"`
			Held      int     `json:"held"`
			Pct       float64 `json:"pct"`
		}
		overall := make([]overallRow, 0, len(perStudent))
		for id, t := range perStudent {
			o := overallRow{StudentID: id, Attended: t.attended, Held: t.held}
			if t.held > 0 {
				o.Pct = round1(float64(t.attended) / float64(t.held) * 100)
			}
			overall = append(overall, o)
		}

		// A bare array was the old shape and the app still reads it, so the rows stay the body and
		// the rollups ride in headers — changing the body shape would blank the roster on every
		// handset that has not been updated.
		classPct := 0.0
		if totHeld > 0 {
			classPct = round1(float64(totAttended) / float64(totHeld) * 100)
		}
		w.Header().Set("X-Attendance-Overall-Pct", ftoa1(classPct))
		w.Header().Set("X-Attendance-Attended", itoa(totAttended))
		w.Header().Set("X-Attendance-Held", itoa(totHeld))
		if r.URL.Query().Get("with_overall") == "1" {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"students": out, "overall": overall,
				"class_pct": classPct, "attended": totAttended, "held": totHeld,
			})
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// LecturerSessions — every session for this lecturer's units (across cohorts), with the
// cohort, date, day-of-week, present count and enrolled count.
func LecturerSessions(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		lecturerID, ok := resolveLecturerID(adminPool, r, tenantID, userID)
		if !ok {
			writeJSON(w, http.StatusForbidden, errBody("NOT_A_LECTURER", "no lecturer profile for this account"))
			return
		}
		unitFilter := strings.TrimSpace(r.URL.Query().Get("unit_id"))
		args := []interface{}{tenantID, lecturerID}
		unitWhere := ""
		if unitFilter != "" {
			args = append(args, unitFilter)
			unitWhere = fmt.Sprintf(" AND ses.unit_id = $%d", len(args))
		}

		rows, err := adminPool.Query(r.Context(), `
			SELECT ses.session_id::text, ses.unit_id, COALESCE(cu.name,''), ses.session_date::text,
			       COALESCE(EXTRACT(ISODOW FROM ses.session_date)::int, 0),
			       COALESCE(o.study_year,0), COALESCE(o.semester,0), COALESCE(o.level,''), COALESCE(o.intake,''),
			       ses.session_status::text,
			       (SELECT count(*) FROM attendance_logs al WHERE al.session_id = ses.session_id) AS present,
			       (SELECT count(*) FROM students_extended s WHERE s.offering_id = ses.offering_id AND s.enrollment_status='ACTIVE') AS enrolled,
			       -- The QA monitor's side of the same lecture, so the lecturer sees the second
			       -- record about them rather than only hearing of it when it is quoted back.
			       COALESCE(mon.patroller_name,''), COALESCE(mon.is_compensation,false),
			       COALESCE(mon.compensation_for::text,'')
			FROM sessions ses
			JOIN lecturer_assignments la ON la.unit_id = ses.unit_id AND la.lecturer_id = $2::uuid
			LEFT JOIN course_units cu     ON cu.unit_id = ses.unit_id
			LEFT JOIN course_offerings o  ON o.offering_id = ses.offering_id
			LEFT JOIN LATERAL (
			    SELECT pl.patroller_name, pl.is_compensation, pl.compensation_for
			      FROM lecturer_patrol_logs pl
			     WHERE pl.unit_id = ses.unit_id
			       AND pl.session_date = ses.session_date
			     ORDER BY pl.is_compensation DESC, pl.taken_at DESC
			     LIMIT 1
			) mon ON true
			WHERE ses.tenant_id = $1`+unitWhere+`
			ORDER BY ses.session_date DESC, ses.gate_open_time DESC`, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type sess struct {
			SessionID string `json:"session_id"`
			UnitID    string `json:"unit_id"`
			UnitName  string `json:"unit_name"`
			Date      string `json:"session_date"`
			DayOfWeek int    `json:"day_of_week"`
			Cohort    string `json:"cohort"`
			// The same cohort as YEAR:SEMESTER ("2:1"), which is how the institution says it
			// aloud and how it is written on every other attendance record.
			ClassGroup      string `json:"class_group"`
			Status          string `json:"status"`
			Present         int    `json:"present_count"`
			Enrolled        int    `json:"enrolled_count"`
			QAMonitor       string `json:"qa_monitor"`
			IsCompensation  bool   `json:"is_compensation"`
			CompensationFor string `json:"compensation_for"`
		}
		out := []sess{}
		for rows.Next() {
			var x sess
			var yr, sm int
			var lvl, intake string
			if rows.Scan(&x.SessionID, &x.UnitID, &x.UnitName, &x.Date, &x.DayOfWeek,
				&yr, &sm, &lvl, &intake, &x.Status, &x.Present, &x.Enrolled,
				&x.QAMonitor, &x.IsCompensation, &x.CompensationFor) != nil {
				continue
			}
			x.Cohort = cohortLabel(yr, sm, lvl, intake)
			x.ClassGroup = classGroup(yr, sm)
			out = append(out, x)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// LecturerSessionStudents — for ONE session: who was present, who was absent, or all
// enrolled with a present flag. status = present | absent | all (default all).
func LecturerSessionStudents(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		lecturerID, ok := resolveLecturerID(adminPool, r, tenantID, userID)
		if !ok {
			writeJSON(w, http.StatusForbidden, errBody("NOT_A_LECTURER", "no lecturer profile for this account"))
			return
		}
		sessionID := extractPathID(r.URL.Path, "/api/v1/lecturer/sessions/", "/students")
		if sessionID == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "missing session id"))
			return
		}
		status := r.URL.Query().Get("status")
		if status == "" {
			status = "all"
		}

		// Authorise: the session must be for a unit this lecturer is assigned to.
		var offeringID, unitID string
		err := adminPool.QueryRow(r.Context(), `
			SELECT ses.offering_id::text, ses.unit_id
			FROM sessions ses
			JOIN lecturer_assignments la ON la.unit_id = ses.unit_id AND la.lecturer_id = $2::uuid
			WHERE ses.tenant_id = $1 AND ses.session_id = $3::uuid LIMIT 1`,
			tenantID, lecturerID, sessionID).Scan(&offeringID, &unitID)
		if err != nil {
			writeJSON(w, http.StatusForbidden, errBody("NOT_ASSIGNED", "not your session"))
			return
		}

		// Enrolled students of the session's cohort, LEFT JOINed to their attendance row for
		// THIS session — so present = matched, absent = NULL.
		rows, err := adminPool.Query(r.Context(), `
			SELECT s.student_id, s.full_name, (al.log_id IS NOT NULL) AS present,
			       COALESCE(to_char(al.checkin_timestamp, 'YYYY-MM-DD"T"HH24:MI:SSZ'), '')
			FROM students_extended s
			LEFT JOIN attendance_logs al ON al.session_id = $2::uuid AND al.student_id = s.student_id AND al.tenant_id = $1
			WHERE s.tenant_id = $1 AND s.offering_id = $3::uuid AND s.enrollment_status = 'ACTIVE'
			ORDER BY s.full_name`, tenantID, sessionID, offeringID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type stu struct {
			StudentID string `json:"student_id"`
			FullName  string `json:"full_name"`
			Present   bool   `json:"present"`
			CheckinAt string `json:"checkin_at"`
		}
		out := []stu{}
		for rows.Next() {
			var x stu
			if rows.Scan(&x.StudentID, &x.FullName, &x.Present, &x.CheckinAt) != nil {
				continue
			}
			if status == "present" && !x.Present {
				continue
			}
			if status == "absent" && x.Present {
				continue
			}
			out = append(out, x)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"unit_id": unitID, "students": out})
	}
}

// ftoa1 renders a percentage to one decimal place. Kept next to the roster because the headers it
// feeds are read by clients that cannot parse a locale-formatted number.
func ftoa1(f float64) string {
	return strconv.FormatFloat(f, 'f', 1, 64)
}
