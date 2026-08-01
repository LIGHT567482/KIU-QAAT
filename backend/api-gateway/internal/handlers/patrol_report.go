package handlers

// Cross-dimension lecturer-teaching report (Phase 5). Aggregates patrol observations
// (lecturer_patrol_logs) with optional filters, for QA/DQA/VC/Admin oversight.
//
//   GET /api/v1/reports/lecturer-teaching?from=&to=&school=&department=&lecturer=&unit=&status=
//        status = taught | not_taught | all (default all)

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

func LecturerTeachingReport(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
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

		q := r.URL.Query()
		where := []string{"p.tenant_id = $1"}
		args := []interface{}{tenantID}
		add := func(clause, val string) {
			if val != "" {
				args = append(args, val)
				where = append(where, strings.Replace(clause, "$?", "$"+strconv.Itoa(len(args)), 1))
			}
		}

		// Org-scoped callers see only their own unit. This report takes its dimensions from the
		// query string, so without this an HOD, dean or QA rep could simply ask for another
		// department by name — their whole dashboard is scoped, and this must match it. The
		// oversight roles (QA officer, DQA, VC, DVC, admin) stay unscoped by design.
		scope, err := resolveQAScope(r.Context(), conn, middleware.GetUserID(r.Context()), middleware.GetRole(r.Context()))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "could not resolve your org unit"))
			return
		}
		if clause, val, hasVal := scope.scopeSQL("c.department", "c.school", len(args)+1); clause != "" {
			if hasVal {
				args = append(args, val)
			}
			where = append(where, strings.TrimPrefix(clause, " AND "))
		}

		add("p.session_date >= $?::date", q.Get("from"))
		add("p.session_date <= $?::date", q.Get("to"))
		add("btrim(lower(c.school)) = btrim(lower($?))", q.Get("school"))
		add("btrim(lower(c.department)) = btrim(lower($?))", q.Get("department"))
		add("btrim(lower(p.lecturer_id)) = btrim(lower($?))", q.Get("lecturer"))
		add("p.unit_id = $?", q.Get("unit"))
		switch q.Get("status") {
		case "taught":
			where = append(where, "p.taught = true")
		case "not_taught":
			where = append(where, "p.taught = false")
		}

		sql := `
			SELECT p.lecturer_id, COALESCE(MAX(p.lecturer_name),''),
			       COALESCE(MAX(c.department),''), COALESCE(MAX(c.school),''),
			       COUNT(*) FILTER (WHERE p.taught) AS taught,
			       COUNT(*) AS patrolled
			FROM lecturer_patrol_logs p
			LEFT JOIN course_units cu ON cu.unit_id = p.unit_id AND cu.tenant_id = p.tenant_id
			LEFT JOIN courses c ON c.course_id = cu.course_id AND c.tenant_id = cu.tenant_id
			WHERE ` + strings.Join(where, " AND ") + `
			GROUP BY p.lecturer_id
			ORDER BY MAX(p.lecturer_name)`

		rows, err := conn.Query(r.Context(), sql, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type row struct {
			LecturerID string `json:"lecturer_id"`
			Name       string `json:"full_name"`
			Department string `json:"department"`
			School     string `json:"school"`
			Taught     int    `json:"taught"`
			Patrolled  int    `json:"patrolled"`
		}
		out := []row{}
		var totTaught, totPatrolled int
		for rows.Next() {
			var rr row
			if rows.Scan(&rr.LecturerID, &rr.Name, &rr.Department, &rr.School, &rr.Taught, &rr.Patrolled) == nil {
				out = append(out, rr)
				totTaught += rr.Taught
				totPatrolled += rr.Patrolled
			}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"rows":            out,
			"total_taught":    totTaught,
			"total_patrolled": totPatrolled,
		})
	}
}
