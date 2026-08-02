package handlers

// The dean's view of the heads of department beneath them.
//
//	GET /api/v1/org/departments   (DEAN · QA_SCHOOL_HANDLER · QA_OFFICER · DQA · VC · DVC · ADMIN)
//
// A dean is accountable for a college through its HEADS OF DEPARTMENT, but until now could only see
// a flat list of every lecturer in the school — the management layer between them was invisible.
// This returns one row per department in their school: who runs it, and how that department is
// doing on the things a dean is judged on.
//
// DRIVEN BY THE `departments` TABLE, NOT BY `courses.department`. This matters. The org tree
// (`schools` → `departments`) is id-linked and authoritative; `courses.department` /
// `courses.school` are free-text NAMES matched by string. Deriving the list from courses would
// silently omit a department that exists but has no course attached yet — exactly the department a
// dean most needs to ask about. So the departments table is the spine and everything else is a LEFT
// JOIN onto it, which also lets the two genuine coordination failures be reported rather than
// hidden:
//
//   • a department with NO HOD ACCOUNT — nobody is answerable for it;
//   • a department whose name matches no course — the org tree and the academic data have drifted
//     apart, and every scoped query for that HOD returns nothing at all.

import (
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// OrgDepartments — GET /api/v1/org/departments
func OrgDepartments(pool *pgxpool.Pool) http.HandlerFunc {
	type hod struct {
		UserID      string `json:"user_id"`
		FullName    string `json:"full_name"`
		Email       string `json:"email"`
		Title       string `json:"title"`
		Phone       string `json:"phone"`
		LastLoginAt string `json:"last_login_at"`
	}
	type deptRow struct {
		DepartmentID string `json:"department_id"`
		Name         string `json:"name"`
		School       string `json:"school"`
		Kind         string `json:"kind"`
		HOD          *hod   `json:"hod"`

		Courses        int `json:"courses"`
		Units          int `json:"units"`
		UnitsUnstaffed int `json:"units_unstaffed"`
		Lecturers      int `json:"lecturers"`
		Students       int `json:"students"`

		SessionsHeld    int     `json:"sessions_held"`
		SessionsPlanned int     `json:"sessions_planned"`
		TaughtRate      float64 `json:"taught_rate"`
		// Of the sessions that ran, how many the timetabled lecturer actually gated in for. This is
		// the ghost-lecture measure: a room opened around a lecturer who never came.
		LecturerShowRate float64 `json:"lecturer_show_rate"`

		AvgAttendance float64 `json:"avg_attendance"`
		AtRisk        int     `json:"at_risk"`

		Patrolled int `json:"patrolled"`
		PatrolOK  int `json:"patrol_taught"`

		// Coordination faults, reported rather than silently absorbed.
		NoHOD      bool `json:"no_hod"`
		Unlinked   bool `json:"unlinked"` // the name matches no course → every scoped query is empty
		HODNeverIn bool `json:"hod_never_signed_in"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		role := middleware.GetRole(r.Context())

		s, ok := resolveOrgScope(r, pool, tenantID, userID, role)
		// Only the school-level roles are meaningful here; a HOD runs ONE department and has no
		// heads beneath them.
		if !ok {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"unset": true, "departments": []any{},
				"message": "No college or school is set on your account. Ask an administrator to set it.",
			})
			return
		}

		// ── The spine: departments of this school (all of them, for the unscoped roles) ──
		args := []interface{}{tenantID}
		q := `SELECT d.department_id::text, d.name, COALESCE(s.name,''), COALESCE(d.kind,'ACADEMIC')
		      FROM departments d
		      LEFT JOIN schools s ON s.school_id = d.school_id AND s.tenant_id = d.tenant_id
		      WHERE d.tenant_id = $1`
		if !s.Unbounded {
			// A dean is bounded by their school; support departments (school_id NULL) belong to no
			// faculty and are correctly excluded from a dean's view.
			args = append(args, s.Val)
			q += ` AND btrim(lower(COALESCE(s.name,''))) = btrim(lower($2))`
		}
		q += ` ORDER BY d.name`

		rows, err := pool.Query(r.Context(), q, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		out := []deptRow{}
		for rows.Next() {
			var d deptRow
			if rows.Scan(&d.DepartmentID, &d.Name, &d.School, &d.Kind) == nil {
				out = append(out, d)
			}
		}
		rows.Close()
		if len(out) == 0 {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"scope": orgScopeLabel(s, role), "departments": out,
			})
			return
		}

		// Index by lower-cased name — the only key `courses.department` can be joined on.
		idx := map[string]int{}
		for i := range out {
			idx[strings.ToLower(strings.TrimSpace(out[i].Name))] = i
		}
		at := func(name string) *deptRow {
			if i, ok := idx[strings.ToLower(strings.TrimSpace(name))]; ok {
				return &out[i]
			}
			return nil
		}

		since := time.Now().AddDate(0, 0, -90).Format("2006-01-02")

		// ── Who runs each one ────────────────────────────────────────────────
		hRows, _ := pool.Query(r.Context(), `
			SELECT COALESCE(department,''), user_id::text, COALESCE(full_name,''), COALESCE(email,''),
			       COALESCE(title,''), COALESCE(phone,''),
			       COALESCE(to_char(last_login_at,'YYYY-MM-DD'),'')
			FROM users
			WHERE tenant_id = $1 AND role = 'HOD' AND COALESCE(department,'') <> ''
			ORDER BY last_login_at DESC NULLS LAST`, tenantID)
		if hRows != nil {
			for hRows.Next() {
				var dept string
				var h hod
				if hRows.Scan(&dept, &h.UserID, &h.FullName, &h.Email, &h.Title, &h.Phone, &h.LastLoginAt) != nil {
					continue
				}
				// First (most recently active) wins if an institution has somehow created two.
				if d := at(dept); d != nil && d.HOD == nil {
					hh := h
					d.HOD = &hh
				}
			}
			hRows.Close()
		}

		// ── Teaching: courses, units, unstaffed units, lecturers ─────────────
		cRows, _ := pool.Query(r.Context(), `
			SELECT COALESCE(c.department,''),
			       COUNT(DISTINCT c.course_id),
			       COUNT(DISTINCT cu.unit_id),
			       COUNT(DISTINCT cu.unit_id) FILTER (
			           WHERE NOT EXISTS (SELECT 1 FROM lecturer_assignments la
			                             WHERE la.unit_id = cu.unit_id AND la.tenant_id = cu.tenant_id)),
			       COUNT(DISTINCT la.lecturer_id)
			FROM courses c
			LEFT JOIN course_units cu ON cu.course_id = c.course_id AND cu.tenant_id = c.tenant_id
			LEFT JOIN lecturer_assignments la ON la.unit_id = cu.unit_id AND la.tenant_id = cu.tenant_id
			WHERE c.tenant_id = $1
			GROUP BY 1`, tenantID)
		if cRows != nil {
			for cRows.Next() {
				var dept string
				var courses, units, unstaffed, lecturers int
				if cRows.Scan(&dept, &courses, &units, &unstaffed, &lecturers) != nil {
					continue
				}
				if d := at(dept); d != nil {
					d.Courses, d.Units, d.UnitsUnstaffed, d.Lecturers = courses, units, unstaffed, lecturers
				}
			}
			cRows.Close()
		}

		// ── Students + attendance + at-risk ──────────────────────────────────
		sRows, _ := pool.Query(r.Context(), `
			SELECT COALESCE(c.department,''),
			       COUNT(DISTINCT se.student_id),
			       ROUND(COALESCE(AVG(sas.attendance_percentage),0),1),
			       COUNT(DISTINCT sas.student_id) FILTER (WHERE sas.attendance_percentage < t.attendance_threshold)
			FROM students_extended se
			JOIN courses c ON c.course_id = se.course_id AND c.tenant_id = se.tenant_id
			JOIN tenants t ON t.tenant_id = se.tenant_id
			LEFT JOIN student_attendance_summary sas
			       ON sas.student_id = se.student_id AND sas.tenant_id = se.tenant_id
			WHERE se.tenant_id = $1 AND se.enrollment_status = 'ACTIVE'
			GROUP BY 1`, tenantID)
		if sRows != nil {
			for sRows.Next() {
				var dept string
				var students, atRisk int
				var avg float64
				if sRows.Scan(&dept, &students, &avg, &atRisk) != nil {
					continue
				}
				if d := at(dept); d != nil {
					d.Students, d.AvgAttendance, d.AtRisk = students, avg, atRisk
				}
			}
			sRows.Close()
		}

		// ── Sessions held, and whether the lecturer actually turned up ───────
		//
		// The LEFT JOIN onto lecturer_attendance_logs is the ghost-lecture test: a session with no
		// gate record for its unit and date means a room was opened around a lecturer who never
		// came. `sessions` alone cannot tell a dean that.
		hsRows, _ := pool.Query(r.Context(), `
			SELECT COALESCE(c.department,''),
			       COUNT(*),
			       COUNT(*) FILTER (WHERE lal.log_id IS NOT NULL)
			FROM sessions ss
			JOIN course_units cu ON cu.unit_id = ss.unit_id AND cu.tenant_id = ss.tenant_id
			JOIN courses c ON c.course_id = cu.course_id AND c.tenant_id = cu.tenant_id
			LEFT JOIN LATERAL (
			    SELECT log_id FROM lecturer_attendance_logs l
			    WHERE l.tenant_id = ss.tenant_id AND l.unit_id = ss.unit_id
			      AND l.session_date = ss.session_date
			    LIMIT 1
			) lal ON true
			WHERE ss.tenant_id = $1 AND ss.session_date >= $2::date
			GROUP BY 1`, tenantID, since)
		if hsRows != nil {
			for hsRows.Next() {
				var dept string
				var held, withLecturer int
				if hsRows.Scan(&dept, &held, &withLecturer) != nil {
					continue
				}
				if d := at(dept); d != nil {
					d.SessionsHeld = held
					if held > 0 {
						d.LecturerShowRate = round1(float64(withLecturer) / float64(held) * 100)
					}
				}
			}
			hsRows.Close()
		}

		// The denominator: each weekly timetable slot recurs ~13 times in the 90-day window.
		tRows, _ := pool.Query(r.Context(), `
			SELECT COALESCE(c.department,''), COUNT(*)
			FROM timetable_slots ts
			JOIN course_units cu ON cu.unit_id = ts.unit_id AND cu.tenant_id = ts.tenant_id
			JOIN courses c ON c.course_id = cu.course_id AND c.tenant_id = cu.tenant_id
			WHERE ts.tenant_id = $1
			GROUP BY 1`, tenantID)
		if tRows != nil {
			for tRows.Next() {
				var dept string
				var slots int
				if tRows.Scan(&dept, &slots) != nil {
					continue
				}
				if d := at(dept); d != nil {
					d.SessionsPlanned = slots * 13
					if d.SessionsPlanned > 0 {
						d.TaughtRate = round1(float64(d.SessionsHeld) / float64(d.SessionsPlanned) * 100)
						if d.TaughtRate > 100 {
							d.TaughtRate = 100
						}
					}
				}
			}
			tRows.Close()
		}

		// ── The independent record: QA patrol observations ───────────────────
		pRows, _ := pool.Query(r.Context(), `
			SELECT COALESCE(c.department,''), COUNT(*), COUNT(*) FILTER (WHERE p.taught)
			FROM lecturer_patrol_logs p
			JOIN course_units cu ON cu.unit_id = p.unit_id AND cu.tenant_id = p.tenant_id
			JOIN courses c ON c.course_id = cu.course_id AND c.tenant_id = cu.tenant_id
			WHERE p.tenant_id = $1 AND p.session_date >= $2::date
			GROUP BY 1`, tenantID, since)
		if pRows != nil {
			for pRows.Next() {
				var dept string
				var patrolled, taught int
				if pRows.Scan(&dept, &patrolled, &taught) != nil {
					continue
				}
				if d := at(dept); d != nil {
					d.Patrolled, d.PatrolOK = patrolled, taught
				}
			}
			pRows.Close()
		}

		// ── Flag the coordination faults ─────────────────────────────────────
		for i := range out {
			out[i].NoHOD = out[i].HOD == nil
			out[i].HODNeverIn = out[i].HOD != nil && out[i].HOD.LastLoginAt == ""
			// No course carries this department's name, so nothing the HOD's dashboard queries can
			// ever match. The department exists on the org tree and nowhere else.
			out[i].Unlinked = out[i].Courses == 0
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"scope":       orgScopeLabel(s, role),
			"window_days": 90,
			"departments": out,
		})
	}
}
