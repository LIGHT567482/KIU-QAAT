package handlers

// HOD + Dean dashboards (Phase 2). Each is scoped to the org unit carried on the user's account:
//   HOD  → users.department  (sees lecturers teaching in that department)
//   Dean → users.school      (sees lecturers teaching in any department of that school/college)
//
// A lecturer's department/school is derived from the courses whose units they teach
// (lecturer_assignments → course_units → courses). "Progress" = how many patrolled sessions the
// lecturer was found TEACHING vs total patrolled, this term.
//
//   GET /api/v1/hod/lecturers
//   GET /api/v1/dean/lecturers

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

type lecturerProgress struct {
	StaffID        string `json:"staff_id"`
	FullName       string `json:"full_name"`
	Department     string `json:"department,omitempty"`
	School         string `json:"school,omitempty"`
	TaughtCount    int    `json:"taught_count"`
	PatrolledCount int    `json:"patrolled_count"`
	UnitCount      int    `json:"unit_count"`
}

// hodDeanLecturers runs the shared lecturer-progress query filtered by department (HOD) or school
// (Dean). `bySchool` picks which of the caller's org fields scopes the result.
func hodDeanLecturers(pool *pgxpool.Pool, bySchool bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		defer conn.Release()
		if err := middleware.SetTenantConn(r.Context(), conn, tenantID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}

		var department, school string
		_ = conn.QueryRow(r.Context(),
			`SELECT COALESCE(department,''), COALESCE(school,'') FROM users WHERE user_id = $1::uuid`,
			userID).Scan(&department, &school)

		scopeVal := department
		scopeCol := "c.department"
		if bySchool {
			scopeVal = school
			scopeCol = "c.school"
		}
		if scopeVal == "" {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"scope":     map[string]string{"department": department, "school": school},
				"lecturers": []lecturerProgress{},
				"message":   "No department/school is set on your account — ask an admin to set it.",
			})
			return
		}

		rows, err := conn.Query(r.Context(), `
			SELECT l.staff_id, l.full_name,
			       COALESCE(MAX(c.department),''), COALESCE(MAX(c.school),''),
			       COUNT(DISTINCT la.unit_id) AS unit_count,
			       COUNT(DISTINCT p.patrol_id) FILTER (WHERE p.taught) AS taught_count,
			       COUNT(DISTINCT p.patrol_id) AS patrolled_count
			FROM lecturers l
			JOIN lecturer_assignments la ON la.lecturer_id = l.lecturer_id AND la.tenant_id = l.tenant_id
			JOIN course_units cu ON cu.unit_id = la.unit_id AND cu.tenant_id = la.tenant_id
			JOIN courses c ON c.course_id = cu.course_id AND c.tenant_id = cu.tenant_id
			LEFT JOIN lecturer_patrol_logs p
			       ON p.lecturer_id = l.staff_id AND p.tenant_id = l.tenant_id
			WHERE l.tenant_id = $1 AND btrim(lower(`+scopeCol+`)) = btrim(lower($2))
			GROUP BY l.staff_id, l.full_name
			ORDER BY l.full_name`, tenantID, scopeVal)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		out := []lecturerProgress{}
		for rows.Next() {
			var lp lecturerProgress
			if rows.Scan(&lp.StaffID, &lp.FullName, &lp.Department, &lp.School,
				&lp.UnitCount, &lp.TaughtCount, &lp.PatrolledCount) == nil {
				out = append(out, lp)
			}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"scope":     map[string]string{"department": department, "school": school},
			"lecturers": out,
		})
	}
}

// HODLecturers — lecturers in the HOD's department + their teaching progress.
func HODLecturers(pool *pgxpool.Pool) http.HandlerFunc { return hodDeanLecturers(pool, false) }

// DeanLecturers — lecturers across the Dean's school (each row carries its department).
func DeanLecturers(pool *pgxpool.Pool) http.HandlerFunc { return hodDeanLecturers(pool, true) }

// QARepLecturers — the same org-scoped lecturer progress for the two QA rep roles, which have the
// identical shape of oversight (one department, or one school) and so share the query. The scope
// comes from the caller's role, never from the request.
func QARepLecturers(pool *pgxpool.Pool) http.HandlerFunc {
	byDept, bySchool := hodDeanLecturers(pool, false), hodDeanLecturers(pool, true)
	return func(w http.ResponseWriter, r *http.Request) {
		if middleware.GetRole(r.Context()) == middleware.RoleQASchool {
			bySchool(w, r)
			return
		}
		byDept(w, r)
	}
}
