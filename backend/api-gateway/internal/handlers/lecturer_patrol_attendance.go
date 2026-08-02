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

// patrolVisit is one room visit by a patroller.
type patrolVisit struct {
	PatrolID    string `json:"patrol_id"`
	LecturerID  string `json:"lecturer_id"`
	UnitID      string `json:"unit_id"`
	UnitName    string `json:"unit_name"`
	Room        string `json:"room"`
	SessionDate string `json:"session_date"`
	Scheduled   string `json:"scheduled_time"`
	Taught      bool   `json:"taught"`
	Patroller   string `json:"patroller_name"`
	PatrollerID string `json:"patroller_staff_id"`
	TakenAt     string `json:"taken_at"`
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

	// Visits, newest first — the detail rows behind each lecturer.
	vRows, err := conn.Query(r.Context(), `
		SELECT p.patrol_id::text, COALESCE(p.lecturer_id,''), p.unit_id, COALESCE(p.unit_name,''),
		       COALESCE(p.room,''), p.session_date::text, COALESCE(p.scheduled_time,''),
		       p.taught, COALESCE(p.patroller_name,''), COALESCE(p.patroller_staff_id,''),
		       p.taken_at
		FROM lecturer_patrol_logs p
		WHERE p.tenant_id = $1`+where+`
		ORDER BY p.session_date DESC, p.scheduled_time DESC`, args...)
	if err != nil {
		return nil, nil, err
	}
	visits := []patrolVisit{}
	for vRows.Next() {
		var v patrolVisit
		var takenAt time.Time
		if vRows.Scan(&v.PatrolID, &v.LecturerID, &v.UnitID, &v.UnitName, &v.Room,
			&v.SessionDate, &v.Scheduled, &v.Taught, &v.Patroller, &v.PatrollerID, &takenAt) == nil {
			v.TakenAt = takenAt.Format(time.RFC3339)
			visits = append(visits, v)
		}
	}
	vRows.Close()

	// Per-lecturer rollup. lecturer_patrol_logs keys lecturers by STAFF ID (that is what the
	// patroller's cached timetable carries), so the join back to `lecturers` is on staff_id.
	sRows, err := conn.Query(r.Context(), `
		SELECT COALESCE(p.lecturer_id,''),
		       COALESCE(MAX(l.full_name), MAX(p.lecturer_name), ''),
		       COALESCE(MAX(l.department),''), COALESCE(MAX(c.school),''),
		       COUNT(*)                              AS patrolled,
		       COUNT(*) FILTER (WHERE p.taught)      AS taught,
		       MAX(p.session_date)::text             AS last_patrol
		FROM lecturer_patrol_logs p
		LEFT JOIN lecturers l   ON l.staff_id = p.lecturer_id AND l.tenant_id = p.tenant_id
		LEFT JOIN course_units cu ON cu.unit_id = p.unit_id   AND cu.tenant_id = p.tenant_id
		LEFT JOIN courses c     ON c.course_id = cu.course_id AND c.tenant_id = cu.tenant_id
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

// patrolAttendanceTable flattens the patrol summary for download.
func patrolAttendanceTable(r *http.Request, pool *pgxpool.Pool) (reportTable, error) {
	summary, _, err := queryPatrolAttendance(r, pool)
	if err != nil {
		return reportTable{}, err
	}
	t := reportTable{
		Title:    "Lecturer Attendance — QA patrol record",
		Subtitle: itoa(len(summary)) + " lecturer(s) patrolled",
		Headers:  []string{"Lecturer", "Staff ID", "Department", "School", "Patrolled", "Teaching", "Missed", "Rate %", "Last patrol"},
		Weights:  []float64{3, 1.8, 2.4, 2.4, 1.2, 1.1, 1, 1, 1.5},
	}
	for _, s := range summary {
		t.Rows = append(t.Rows, []string{
			s.LecturerName, s.LecturerID, s.Department, s.School,
			itoa(s.Patrolled), itoa(s.Taught), itoa(s.Missed),
			formatFloat1(s.Rate), s.LastPatrol,
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
