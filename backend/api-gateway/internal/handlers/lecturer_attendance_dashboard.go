package handlers

// Lecturer-attendance views for the oversight dashboards (QA Officer, VC, DQA
// Director) — same data as the admin's lecturer-attendance pages, but scoped to
// the CALLER's tenant (from the JWT) over the RLS pool, so no {tenant_id} path.

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
	"github.com/qaat/api-gateway/internal/upanel"
)

// lecturerLogScope renders the caller's own college/department as a SQL condition on the joined
// `courses c`, appending its bind value to args. Institution-wide roles get "" and see everything.
//
// A BOUNDED ROLE WITH NO ORG UNIT SET MATCHES NOTHING, which is the whole reason this is a named
// helper rather than an inline call. resolveOrgScope returns ok=false in that case, and the
// tempting reading — "no scope, so no filter" — would hand a QA school handler whose account is
// half-configured the entire institution's teaching record. The safe reading of "your scope is
// unset" is an empty page, and it is the same choice made in qaFiltersScoped and resolveRecipients.
func lecturerLogScope(r *http.Request, pool *pgxpool.Pool, tenantID string, args *[]interface{}) string {
	s, ok := resolveOrgScope(r, pool, tenantID, middleware.GetUserID(r.Context()), middleware.GetRole(r.Context()))
	if s.Unbounded {
		return ""
	}
	if !ok {
		*args = append(*args, []string{"\x00-unset-\x00"})
		return " AND btrim(lower(" + s.Col + ")) = ANY($" + itoa(len(*args)) + ")"
	}
	return s.whereScope(args)
}

// GET /api/v1/dashboard/lecturer-attendance — detailed session logs.
func LecturerAttendanceLogsForCaller(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		defer conn.Release()
		middleware.SetTenantConn(r.Context(), conn, tenantID) //nolint:errcheck

		// ORG SCOPE. This page is read by the institution-wide offices (DQA, QA officer, VC) AND
		// by roles bounded to one college or department (QA school handler, QA dept rep, dean,
		// HOD). The bounded ones must see their own unit and no further — the same rule the
		// student-attendance endpoint has always applied, and the reason this endpoint could not
		// simply be opened to them: it had no scoping at all and returned the whole tenant.
		//
		// Resolved from the ACCOUNT, never from the request, so nobody can widen their own view by
		// adding a query parameter.
		args := []interface{}{tenantID}
		scopeSQL := lecturerLogScope(r, pool, tenantID, &args)

		// Department comes from the unit this log is FOR, not from the lecturer: they have none of
		// their own, and each row is already about one unit, which names its department exactly.
		rows, err := conn.Query(r.Context(), `
			SELECT lal.log_id::text, lal.lecturer_id,
			       COALESCE(l.full_name, lal.lecturer_id), COALESCE(c.department,''),
			       lal.unit_id, COALESCE(cu.name, lal.unit_id), lal.session_date,
			       lal.gate_open_time, lal.gate_close_time, COALESCE(lal.contact_hours,0),
			       COALESCE(s.session_status::text,'UNKNOWN'),`+lecturerLogColumns+`
			FROM lecturer_attendance_logs lal
			LEFT JOIN lecturers l ON l.lecturer_id::text = lal.lecturer_id
			LEFT JOIN course_units cu ON cu.unit_id = lal.unit_id
			LEFT JOIN courses c ON c.course_id = cu.course_id
			LEFT JOIN sessions s ON s.session_id = lal.session_id`+lecturerLogMonitorJoin+`
			WHERE lal.tenant_id = $1`+scopeSQL+`
			ORDER BY lal.session_date DESC, lal.gate_open_time DESC`, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type logRow struct {
			LogID         string  `json:"log_id"`
			LecturerID    string  `json:"lecturer_id"`
			LecturerName  string  `json:"lecturer_name"`
			Department    string  `json:"department"`
			UnitID        string  `json:"unit_id"`
			UnitName      string  `json:"unit_name"`
			SessionDate   string  `json:"session_date"`
			GateOpenTime  string  `json:"gate_open_time"`
			GateCloseTime string  `json:"gate_close_time"`
			ContactHours  float64 `json:"contact_hours"`
			SessionStatus string  `json:"session_status"`
			lecturerLogExtras
		}
		out := []logRow{}
		for rows.Next() {
			var l logRow
			var sd time.Time
			var open, close *time.Time
			rows.Scan(append([]interface{}{ //nolint:errcheck
				&l.LogID, &l.LecturerID, &l.LecturerName, &l.Department, &l.UnitID, &l.UnitName,
				&sd, &open, &close, &l.ContactHours, &l.SessionStatus},
				l.lecturerLogExtras.scanTargets()...)...)
			l.SessionDate = sd.Format("2006-01-02")
			if open != nil {
				l.GateOpenTime = open.Format(time.RFC3339)
			}
			if close != nil {
				l.GateCloseTime = close.Format(time.RFC3339)
			}
			out = append(out, l)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// GET /api/v1/dashboard/lecturer-attendance/summary — per-lecturer aggregates.
func LecturerAttendanceSummaryForCaller(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		defer conn.Release()
		middleware.SetTenantConn(r.Context(), conn, tenantID) //nolint:errcheck

		// Same org scope as the detail rows — they are two views of one dataset, and a summary
		// counting lectures the detail list will not show is worse than either on its own.
		args := []interface{}{tenantID}
		scopeSQL := lecturerLogScope(r, pool, tenantID, &args)

		rows, err := conn.Query(r.Context(), `
			SELECT lal.lecturer_id, COALESCE(l.full_name, lal.lecturer_id),
			       COALESCE(l.staff_id,''),
			       COALESCE(STRING_AGG(DISTINCT c.department, ', ') FILTER (WHERE COALESCE(c.department,'') <> ''), ''),
			       COALESCE(l.email,''), COUNT(*), COALESCE(SUM(lal.contact_hours),0),
			       COALESCE(AVG(lal.contact_hours),0), MAX(lal.session_date)
			FROM lecturer_attendance_logs lal
			LEFT JOIN lecturers l ON l.lecturer_id::text = lal.lecturer_id
			LEFT JOIN course_units cu ON cu.unit_id = lal.unit_id
			LEFT JOIN courses c ON c.course_id = cu.course_id
			WHERE lal.tenant_id = $1`+scopeSQL+`
			GROUP BY lal.lecturer_id, l.full_name, l.email, l.staff_id
			ORDER BY COUNT(*) DESC`, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		out := []lecturerSummaryRow{}
		for rows.Next() {
			var sr lecturerSummaryRow
			var last *time.Time
			rows.Scan(&sr.LecturerID, &sr.LecturerName, &sr.StaffID, &sr.Department, &sr.Email, //nolint:errcheck
				&sr.TotalSessions, &sr.TotalContactHours, &sr.AvgContactHours, &last)
			if last != nil {
				sr.LastSessionDate = last.Format("2006-01-02")
			}
			out = append(out, sr)
		}
		out = appendUPanelLecturerSummary(r.Context(), pool, out)
		writeJSON(w, http.StatusOK, out)
	}
}

type lecturerSummaryRow struct {
	LecturerID        string  `json:"lecturer_id"`
	LecturerName      string  `json:"lecturer_name"`
	StaffID           string  `json:"staff_id,omitempty"`
	Department        string  `json:"department"`
	Email             string  `json:"email"`
	TotalSessions     int     `json:"total_sessions"`
	TotalContactHours float64 `json:"total_contact_hours"`
	AvgContactHours   float64 `json:"avg_contact_hours"`
	LastSessionDate   string  `json:"last_session_date"`
}

func appendUPanelLecturerSummary(ctx context.Context, pool *pgxpool.Pool, native []lecturerSummaryRow) []lecturerSummaryRow {
	upanel.RefreshIfEmpty(ctx, pool)
	rolls, err := upanel.LecturerRollups(ctx, pool)
	if err != nil || len(rolls) == 0 {
		return native
	}
	seen := map[string]bool{}
	out := make([]lecturerSummaryRow, 0, len(native)+len(rolls))
	for _, sr := range native {
		out = append(out, sr)
		seen[strings.ToLower(sr.LecturerID)] = true
		seen[strings.ToLower(sr.LecturerName)] = true
		if s := strings.ToLower(strings.TrimSpace(sr.StaffID)); s != "" {
			seen[s] = true
		}
	}
	for _, u := range rolls {
		if seen[strings.ToLower(u.LecturerID)] || seen[strings.ToLower(u.Name)] {
			continue
		}
		avg := 0.0
		if u.Sessions > 0 {
			avg = u.Hours / float64(u.Sessions)
		}
		out = append(out, lecturerSummaryRow{
			LecturerID:        u.LecturerID,
			LecturerName:      u.Name,
			StaffID:           u.LecturerID,
			Department:        u.UnitName,
			TotalSessions:     u.Sessions,
			TotalContactHours: u.Hours,
			AvgContactHours:   avg,
			LastSessionDate:   u.LastDate,
		})
	}
	return out
}
