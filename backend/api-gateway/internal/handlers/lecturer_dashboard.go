package handlers

// Lecturer dashboard — a logged-in lecturer (role LECTURER) sees the units they're
// assigned to and, per unit, the student attendance MATRIX (students × session
// dates, ✓ = present) along with the coordinator who ran each session. Read-only.
//
//   POST /api/v1/admin/tenants/{tenant_id}/lecturers/{lecturer_id}/create-login (ADMIN)
//   GET  /api/v1/lecturer/overview              (LECTURER)
//   GET  /api/v1/lecturer/attendance?unit_id=   (LECTURER)

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// randPassword generates a short strong password (shared: used by admin user/lecturer
// creation flows elsewhere). Lecturers themselves no longer log in with a password.
func randPassword() string {
	b := make([]byte, 9)
	_, _ = rand.Read(b)
	return "Lec" + base64.RawURLEncoding.EncodeToString(b) // ~12 chars, mixed
}

// resolveLecturerID maps the logged-in LECTURER user to their lecturers.lecturer_id
// (by the user_id link, with an email fallback for accounts made via Users).
func resolveLecturerID(adminPool *pgxpool.Pool, r *http.Request, tenantID, userID string) (string, bool) {
	var id string
	err := adminPool.QueryRow(r.Context(), `
		SELECT l.lecturer_id::text FROM lecturers l
		WHERE l.tenant_id = $1
		  AND ( l.user_id::text = $2
		        OR (l.user_id IS NULL AND l.email = (SELECT u.email FROM users u WHERE u.user_id::text = $2)) )
		LIMIT 1`, tenantID, userID).Scan(&id)
	return id, err == nil && id != ""
}

// GET /api/v1/lecturer/overview
func LecturerOverview(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		lecturerID, ok := resolveLecturerID(adminPool, r, tenantID, userID)
		if !ok {
			writeJSON(w, http.StatusOK, map[string]interface{}{"lecturer": nil, "units": []any{}})
			return
		}
		var name string
		_ = adminPool.QueryRow(r.Context(), `SELECT full_name FROM lecturers WHERE lecturer_id=$1::uuid`, lecturerID).Scan(&name)

		// session_count / last_session let the dashboard label each collapsed unit
		// without having to open it first.
		rows, err := adminPool.Query(r.Context(), `
			SELECT cu.unit_id, cu.name, COALESCE(cu.year,1), COALESCE(cu.semester,1),
			       COALESCE(c.name,''),
			       COUNT(DISTINCT s.session_id),
			       COALESCE(MAX(s.session_date)::text,'')
			FROM lecturer_assignments la
			JOIN course_units cu ON cu.unit_id = la.unit_id AND cu.tenant_id = la.tenant_id
			LEFT JOIN courses c ON c.course_id = cu.course_id AND c.tenant_id = cu.tenant_id
			LEFT JOIN sessions s ON s.unit_id = cu.unit_id AND s.tenant_id = cu.tenant_id
			WHERE la.tenant_id = $1 AND la.lecturer_id = $2::uuid
			GROUP BY cu.unit_id, cu.name, cu.year, cu.semester, c.name
			ORDER BY COALESCE(c.name,''), COALESCE(cu.year,1), COALESCE(cu.semester,1), cu.name`, tenantID, lecturerID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()
		type unit struct {
			UnitID       string `json:"unit_id"`
			Name         string `json:"name"`
			Year         int    `json:"year"`
			Semester     int    `json:"semester"`
			CourseName   string `json:"course_name"`
			SessionCount int    `json:"session_count"`
			LastSession  string `json:"last_session_date"`
		}
		units := []unit{}
		for rows.Next() {
			var u unit
			rows.Scan(&u.UnitID, &u.Name, &u.Year, &u.Semester, &u.CourseName, &u.SessionCount, &u.LastSession) //nolint:errcheck
			units = append(units, u)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"lecturer": map[string]string{"lecturer_id": lecturerID, "full_name": name},
			"units":    units,
		})
	}
}

// GET /api/v1/lecturer/attendance?unit_id=
//
// The session logs for ONE of the lecturer's units, split per COHORT. A unit is
// shared across the study sessions a course runs (Day, Evening, Weekend…), and each
// of those is a separate `course_offerings` row with its own coordinator, its own
// timetable and its own students. Returning one flat roster mixed them together —
// a Weekend student showed up in the Day cohort's logs because they share a course.
// Each cohort here therefore carries only its own sessions and only its own
// students, plus anyone who actually checked in to one of those sessions (a genuine
// attendee is never dropped, whichever cohort they came from).
func LecturerAttendance(adminPool *pgxpool.Pool) http.HandlerFunc {
	type sess struct {
		SessionID   string `json:"session_id"`
		Date        string `json:"session_date"`
		Coordinator string `json:"coordinator_name"`
		Status      string `json:"session_status"`
	}
	type student struct {
		StudentID string `json:"student_id"`
		FullName  string `json:"full_name"`
		Present   []bool `json:"present"`
		// True when the student is not enrolled in this cohort but did check in to one
		// of its sessions (shared unit / late transfer) — the UI marks them as a guest.
		Guest bool `json:"guest"`
	}
	type cohort struct {
		OfferingID  string    `json:"offering_id"`
		Label       string    `json:"label"`
		SessionType string    `json:"session_type"`
		Level       string    `json:"level"`
		Intake      string    `json:"intake"`
		Coordinator string    `json:"coordinator_name"`
		Sessions    []sess    `json:"sessions"`
		Students    []student `json:"students"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		unitID := strings.TrimSpace(r.URL.Query().Get("unit_id"))
		if unitID == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "unit_id is required"))
			return
		}
		lecturerID, ok := resolveLecturerID(adminPool, r, tenantID, userID)
		if !ok {
			writeJSON(w, http.StatusForbidden, errBody("NOT_A_LECTURER", "no lecturer profile for this account"))
			return
		}
		// Authorisation: the unit must be one the lecturer is assigned to.
		var assigned bool
		_ = adminPool.QueryRow(r.Context(),
			`SELECT EXISTS(SELECT 1 FROM lecturer_assignments WHERE tenant_id=$1 AND lecturer_id=$2::uuid AND unit_id=$3)`,
			tenantID, lecturerID, unitID).Scan(&assigned)
		if !assigned {
			writeJSON(w, http.StatusForbidden, errBody("NOT_ASSIGNED", "you are not assigned to this unit"))
			return
		}

		// The unit's own (course, year, semester) — the cohort year/semester a student
		// must be sitting in to be on this unit's roster.
		var courseID string
		var unitYear, unitSem int
		_ = adminPool.QueryRow(r.Context(),
			`SELECT COALESCE(course_id,''), COALESCE(year,0), COALESCE(semester,0)
			 FROM course_units WHERE unit_id=$1 AND tenant_id=$2`, unitID, tenantID).
			Scan(&courseID, &unitYear, &unitSem)

		// ── Cohorts: every study session this unit's course runs ────────────────
		cohorts := []*cohort{}
		byOffering := map[string]*cohort{}
		cRows, err := adminPool.Query(r.Context(), `
			SELECT o.offering_id::text, o.session_type, COALESCE(c.level,''), COALESCE(u.full_name,'')
			FROM course_offerings o
			JOIN courses c ON c.course_id = o.course_id AND c.tenant_id = o.tenant_id
			LEFT JOIN users u ON u.user_id::text = o.coordinator_id AND u.tenant_id = o.tenant_id
			WHERE o.tenant_id = $1 AND o.course_id = $2
			ORDER BY o.session_type`, tenantID, courseID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		for cRows.Next() {
			c := &cohort{Sessions: []sess{}, Students: []student{}}
			cRows.Scan(&c.OfferingID, &c.SessionType, &c.Level, &c.Coordinator) //nolint:errcheck
			cohorts = append(cohorts, c)
			byOffering[c.OfferingID] = c
		}
		cRows.Close()

		// Sessions predating the cohort model carry no offering_id; keep them visible
		// in their own bucket rather than silently dropping them from the logs.
		unassigned := &cohort{OfferingID: "", SessionType: "Unassigned", Sessions: []sess{}, Students: []student{}}

		// ── Sessions held for this unit, filed under their cohort ───────────────
		sIndex := map[string]struct {
			c   *cohort
			row int
		}{}
		sRows, err := adminPool.Query(r.Context(), `
			SELECT s.session_id::text, s.session_date::text, COALESCE(s.offering_id::text,''),
			       COALESCE(u.full_name, ''), s.session_status::text
			FROM sessions s
			LEFT JOIN users u ON u.user_id::text = s.coordinator_id AND u.tenant_id = s.tenant_id
			WHERE s.tenant_id = $1 AND s.unit_id = $2
			ORDER BY s.session_date, s.gate_open_time`, tenantID, unitID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		for sRows.Next() {
			var sx sess
			var offID string
			sRows.Scan(&sx.SessionID, &sx.Date, &offID, &sx.Coordinator, &sx.Status) //nolint:errcheck
			c, found := byOffering[offID]
			if !found {
				c = unassigned
			}
			sIndex[sx.SessionID] = struct {
				c   *cohort
				row int
			}{c, len(c.Sessions)}
			c.Sessions = append(c.Sessions, sx)
		}
		sRows.Close()
		if len(unassigned.Sessions) > 0 {
			cohorts = append(cohorts, unassigned)
			byOffering[""] = unassigned
		}

		// ── Roster: each cohort gets ONLY its own enrolled students ─────────────
		// Bound to the unit's year + semester too, so a Year-1 roster never appears
		// under a Year-3 unit of the same programme.
		stIndex := map[*cohort]map[string]int{}
		addStudent := func(c *cohort, id, name string, guest bool) int {
			if id == "" {
				return -1
			}
			idx, ok := stIndex[c]
			if !ok {
				idx = map[string]int{}
				stIndex[c] = idx
			}
			if i, seen := idx[id]; seen {
				return i
			}
			i := len(c.Students)
			idx[id] = i
			c.Students = append(c.Students, student{
				StudentID: id, FullName: name, Guest: guest,
				Present: make([]bool, len(c.Sessions)),
			})
			return i
		}
		offeringIDs := make([]string, 0, len(cohorts))
		for _, c := range cohorts {
			if c.OfferingID != "" {
				offeringIDs = append(offeringIDs, c.OfferingID)
			}
		}
		if len(offeringIDs) > 0 {
			rRows, err := adminPool.Query(r.Context(), `
				SELECT se.offering_id::text, se.student_id, se.full_name
				FROM students_extended se
				WHERE se.tenant_id = $1 AND se.enrollment_status = 'ACTIVE'
				  AND se.offering_id::text = ANY($2)
				  AND ($3 = 0 OR COALESCE(se.current_year,0) = $3)
				  AND ($4 = 0 OR COALESCE(se.semester,0)     = $4)
				ORDER BY se.full_name`, tenantID, offeringIDs, unitYear, unitSem)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
				return
			}
			for rRows.Next() {
				var offID, id, name string
				rRows.Scan(&offID, &id, &name) //nolint:errcheck
				if c, found := byOffering[offID]; found {
					addStudent(c, id, name, false)
				}
			}
			rRows.Close()
		}

		// ── Fill the matrix from the ledger ─────────────────────────────────────
		// A check-in always counts: if the attendee is not on that cohort's roster
		// they are appended as a guest rather than dropped.
		sessionIDs := make([]string, 0, len(sIndex))
		for id := range sIndex {
			sessionIDs = append(sessionIDs, id)
		}
		if len(sessionIDs) > 0 {
			aRows, err := adminPool.Query(r.Context(), `
				SELECT al.session_id::text, al.student_id, COALESCE(se.full_name, al.student_id)
				FROM attendance_logs al
				LEFT JOIN students_extended se ON se.student_id = al.student_id AND se.tenant_id = $1
				WHERE al.session_id = ANY($2)`, tenantID, sessionIDs)
			if err == nil {
				type hit struct{ sid, stid, name string }
				hits := []hit{}
				for aRows.Next() {
					var h hit
					aRows.Scan(&h.sid, &h.stid, &h.name) //nolint:errcheck
					hits = append(hits, h)
				}
				aRows.Close()
				for _, h := range hits {
					loc, ok := sIndex[h.sid]
					if !ok {
						continue
					}
					ti := addStudent(loc.c, h.stid, h.name, true)
					if ti < 0 {
						continue
					}
					// Rosters can grow after Present[] was sized — normalise before writing.
					if len(loc.c.Students[ti].Present) != len(loc.c.Sessions) {
						grown := make([]bool, len(loc.c.Sessions))
						copy(grown, loc.c.Students[ti].Present)
						loc.c.Students[ti].Present = grown
					}
					loc.c.Students[ti].Present[loc.row] = true
				}
			}
		}

		// Final normalise + human label, so the UI can render straight through.
		for _, c := range cohorts {
			for i := range c.Students {
				if len(c.Students[i].Present) != len(c.Sessions) {
					grown := make([]bool, len(c.Sessions))
					copy(grown, c.Students[i].Present)
					c.Students[i].Present = grown
				}
			}
			parts := []string{}
			for _, p := range []string{c.SessionType, c.Level} {
				if p != "" {
					parts = append(parts, p)
				}
			}
			if unitYear > 0 {
				parts = append(parts, fmt.Sprintf("Y%d", unitYear))
			}
			if unitSem > 0 {
				parts = append(parts, fmt.Sprintf("S%d", unitSem))
			}
			c.Label = strings.Join(parts, " · ")
			if c.Coordinator != "" {
				c.Label += " — " + c.Coordinator
			}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"unit_id": unitID,
			"cohorts": cohorts,
		})
	}
}
