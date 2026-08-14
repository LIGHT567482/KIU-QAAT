package handlers

// THE DIRECTORATE'S TWO NEW REPORTS.
//
//	GET /api/v1/dashboard/dqa/patrol-coverage/export.{xlsx,csv,pdf}
//	GET /api/v1/dashboard/dqa/unit-attendance            (+ /export.{xlsx,csv,pdf})
//
// The coverage export is the page built in patrol_coverage.go, taken away as a file. The unit
// report is new, and exists because the two attendance records this system keeps have never been
// readable side by side.
//
// A director asking "is this unit in trouble?" needs both halves at once — whether the lectures
// happened and whether the cohort turned up — and until now that meant opening the lecturer
// attendance page, noting figures per unit, opening the student attendance page, and lining them up
// by hand. The interesting cases are precisely the ones where the two DISAGREE, and a reader
// reconciling two screens by eye is exactly who misses them: a unit taught faithfully to an empty
// room, and a unit whose students are marked present for lectures with no lecturer record at all.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// ─── QA monitor coverage, as a file ──────────────────────────────────────────
//
// Named "patrol coverage" internally — the handler, the route and the JSON keys still say patrol,
// because they are contracts. Only what a reader sees was renamed.

type patrolCoverageExport struct {
	Days        int     `json:"days"`
	Expected    int     `json:"expected"`
	Patrolled   int     `json:"patrolled"`
	CoveragePct float64 `json:"coverage_pct"`
	Taught      int     `json:"taught"`
	NotTaught   int     `json:"not_taught"`
	BySchool    []struct {
		Name        string  `json:"name"`
		Expected    int     `json:"expected"`
		Patrolled   int     `json:"patrolled"`
		NotTaught   int     `json:"not_taught"`
		CoveragePct float64 `json:"coverage_pct"`
	} `json:"by_school"`
	Gaps []struct {
		UnitID   string `json:"unit_id"`
		UnitName string `json:"unit_name"`
		Room     string `json:"room"`
		School   string `json:"school"`
		Day      int    `json:"day_of_week"`
		Time     string `json:"scheduled_time"`
		Missed   int    `json:"missed"`
	} `json:"gaps"`
}

var weekdayNames = [...]string{"", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

func weekdayName(d int) string {
	if d >= 1 && d < len(weekdayNames) {
		return weekdayNames[d]
	}
	return "—"
}

// GET /api/v1/dashboard/dqa/patrol-coverage/export.{xlsx,csv,pdf}
//
// ONE FLAT TABLE, not the three the screen shows. A spreadsheet with three differently-shaped
// blocks stacked in it cannot be sorted or filtered, which is the only reason to download one — so
// the by-college rollup and the missed-slot list are emitted as rows of a single table with a
// leading Section column, and the headline figures ride in the subtitle where a reader will
// actually see them.
func PatrolCoverageExport(pool *pgxpool.Pool, format string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, ok := captureJSON(PatrolCoverage(pool), w, r)
		if !ok {
			return
		}
		var c patrolCoverageExport
		if err := json.Unmarshal(body, &c); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}

		t := reportTable{
			Title: "QA Monitor Coverage",
			Subtitle: fmt.Sprintf(
				"Last %d days · %d of %d timetabled slots visited by a QA monitor (%.0f%%) · %d found not taught · %d never visited",
				c.Days, c.Patrolled, c.Expected, c.CoveragePct, c.NotTaught, c.Expected-c.Patrolled),
			Headers: []string{"Section", "College", "Unit", "Room", "When", "Timetabled", "Monitored", "Coverage %", "Not taught", "Times missed"},
			Weights: []float64{1.4, 2.6, 3, 1.6, 1.6, 1.2, 1.2, 1.2, 1.2, 1.3},
		}
		for _, s := range c.BySchool {
			t.Rows = append(t.Rows, []string{
				"By college", s.Name, "", "", "",
				itoa(s.Expected), itoa(s.Patrolled), fmt.Sprintf("%.0f", s.CoveragePct), itoa(s.NotTaught), "",
			})
		}
		for _, g := range c.Gaps {
			t.Rows = append(t.Rows, []string{
				"Never reached", g.School, g.UnitName + " (" + g.UnitID + ")", g.Room,
				weekdayName(g.Day) + " " + g.Time, "", "", "", "", itoa(g.Missed),
			})
		}
		writeReport(w, r, pool, format, "patrol-coverage", t)
	}
}

// ─── Unit attendance: both records, one row ──────────────────────────────────

type unitAttendanceRow struct {
	UnitID     string  `json:"unit_id"`
	UnitName   string  `json:"unit_name"`
	Course     string  `json:"course"`
	Department string  `json:"department"`
	School     string  `json:"school"`
	Lecturer   string  `json:"lecturer_name"`
	Sessions   int     `json:"sessions_held"`
	Taught     int     `json:"lectures_recorded"`
	Hours      float64 `json:"contact_hours"`
	Enrolled   int     `json:"students_enrolled"`
	Checkins   int     `json:"student_checkins"`
	Manual     int     `json:"manual_checkins"`
	StudentPct float64 `json:"student_attendance_pct"`
	// Sessions where students were marked present but NO lecturer record exists. Surfaced rather
	// than derived on the client because it is the finding the report is FOR.
	Unmatched int `json:"sessions_without_lecturer_record"`
}

// GET /api/v1/dashboard/dqa/unit-attendance?days=90
func UnitAttendanceReport(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := queryUnitAttendance(r, pool)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, rows)
	}
}

func queryUnitAttendance(r *http.Request, pool *pgxpool.Pool) ([]unitAttendanceRow, error) {
	tenantID := middleware.GetTenantID(r.Context())

	days := 90
	if v := strings.TrimSpace(r.URL.Query().Get("days")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 400 {
			days = n
		}
	}

	conn, err := pool.Acquire(r.Context())
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	if err := middleware.SetTenantConn(r.Context(), conn, tenantID); err != nil {
		return nil, err
	}

	args := []interface{}{tenantID, days}
	// Bounded roles read this too, and are held to their own college or department exactly as they
	// are everywhere else. Appended from the account, never from the query string.
	scopeSQL := lecturerLogScope(r, pool, tenantID, &args)

	// ONE PASS OVER SESSIONS, with both records hung off it.
	//
	// Sessions are the spine rather than either attendance table, and that choice is the report:
	// counting from lecturer_attendance_logs would make a lecture nobody recorded invisible, and
	// counting from attendance_logs would lose a lecture taught to an empty room. A session exists
	// either way, so both failures stay countable.
	//
	// The student counts are subqueries rather than joins because a session has many attendance
	// rows AND many lecturer rows; joined together they multiply, and the contact hours would be
	// silently inflated by the size of the class.
	const q = `
		SELECT s.unit_id,
		       COALESCE(cu.name, s.unit_id),
		       COALESCE(c.name,''), COALESCE(c.department,''), COALESCE(c.school,''),
		       COALESCE(MAX(l.full_name), '') AS lecturer,
		       COUNT(DISTINCT s.session_id) AS sessions,
		       COUNT(DISTINCT lal.session_id) FILTER (WHERE lal.lecturer_scanned_at IS NOT NULL) AS taught,
		       COALESCE(SUM(DISTINCT lal.contact_hours), 0) AS hours,
		       (SELECT COUNT(*) FROM students_extended se
		         WHERE se.enrollment_status = 'ACTIVE'
		           AND se.course_id = cu.course_id) AS enrolled,
		       (SELECT COUNT(*) FROM attendance_logs al
		         WHERE al.session_id IN (SELECT session_id FROM sessions s2
		                                  WHERE s2.tenant_id = s.tenant_id AND s2.unit_id = s.unit_id
		                                    AND s2.session_date >= CURRENT_DATE - ($2::int - 1))) AS checkins,
		       -- Manual entries are COUNTED, never excluded — a student marked present by a
		       -- coordinator on paper attended the lecture. They are reported separately only so a
		       -- unit whose whole register was typed in afterwards is visible as such.
		       --
		       -- MANUAL_OVERRIDE is the ONLY manual value here. attendance_logs.entry_method is the
		       -- enum entry_method_enum (QR_SCAN, MANUAL_OVERRIDE, AUTHENTICATED); the 'MANUAL' this
		       -- once also tested for belongs to lecturer_patrol_logs.entry_method, which is plain
		       -- text and a different vocabulary. Naming a label the enum does not have is not an
		       -- empty result but a hard 22P02 at parse time — the whole report 500s.
		       (SELECT COUNT(*) FROM attendance_logs al
		         WHERE al.entry_method = 'MANUAL_OVERRIDE'
		           AND al.session_id IN (SELECT session_id FROM sessions s2
		                                  WHERE s2.tenant_id = s.tenant_id AND s2.unit_id = s.unit_id
		                                    AND s2.session_date >= CURRENT_DATE - ($2::int - 1))) AS manual,
		       COUNT(DISTINCT s.session_id) FILTER (WHERE lal.session_id IS NULL) AS unmatched
		FROM sessions s
		JOIN course_units cu ON cu.unit_id = s.unit_id
		LEFT JOIN courses c  ON c.course_id = cu.course_id
		LEFT JOIN lecturer_attendance_logs lal ON lal.session_id = s.session_id
		LEFT JOIN lecturers l ON l.lecturer_id::text = s.lecturer_id
		WHERE s.tenant_id = $1
		  AND s.session_date >= CURRENT_DATE - ($2::int - 1)`

	sql := q + scopeSQL + `
		GROUP BY s.unit_id, cu.name, cu.course_id, c.name, c.department, c.school, s.tenant_id
		ORDER BY c.school, c.department, cu.name`

	rows, err := conn.Query(r.Context(), sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []unitAttendanceRow{}
	for rows.Next() {
		var u unitAttendanceRow
		if rows.Scan(&u.UnitID, &u.UnitName, &u.Course, &u.Department, &u.School, &u.Lecturer,
			&u.Sessions, &u.Taught, &u.Hours, &u.Enrolled, &u.Checkins, &u.Manual, &u.Unmatched) != nil {
			continue
		}
		// Attendance as a share of the seats that could have been filled: enrolled students times
		// sessions held. Zero on either side means there is nothing to divide by and nothing to
		// report, which is different from 0% and must not be shown as it.
		if denom := u.Enrolled * u.Sessions; denom > 0 {
			u.StudentPct = float64(int(float64(u.Checkins)/float64(denom)*1000+0.5)) / 10
		}
		out = append(out, u)
	}
	return out, nil
}

// GET /api/v1/dashboard/dqa/unit-attendance/export.{xlsx,csv,pdf}
func UnitAttendanceExport(pool *pgxpool.Pool, format string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := queryUnitAttendance(r, pool)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		t := reportTable{
			Title:    "Unit Attendance — teaching and turnout",
			Subtitle: itoa(len(list)) + " unit(s)",
			Headers: []string{"Unit", "Name", "Course", "Department", "College", "Lecturer",
				"Sessions", "Lectures recorded", "No lecturer record", "Contact hrs",
				"Enrolled", "Check-ins", "of which manual", "Attendance %"},
			Weights: []float64{1.4, 2.6, 2.4, 2, 2, 2.2, 1, 1.4, 1.4, 1.1, 1, 1.1, 1.3, 1.2},
		}
		for _, u := range list {
			t.Rows = append(t.Rows, []string{
				u.UnitID, u.UnitName, u.Course, u.Department, u.School, u.Lecturer,
				itoa(u.Sessions), itoa(u.Taught), itoa(u.Unmatched), formatFloat1(u.Hours),
				itoa(u.Enrolled), itoa(u.Checkins), itoa(u.Manual), formatFloat1(u.StudentPct),
			})
		}
		writeReport(w, r, pool, format, "unit-attendance", t)
	}
}
