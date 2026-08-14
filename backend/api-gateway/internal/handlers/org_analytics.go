package handlers

// THE NUMBERS BEHIND THE OVERVIEW, SHAPED FOR CHARTS.
//
//	GET /api/v1/org/analytics?weeks=12
//
// The overview header has always been six scalars — students, units, taught rate, attendance, at
// risk. A scalar answers "what is it now" and refuses the two questions an overview is actually
// opened with: is it getting better or worse, and where is the problem. Both need shape, so this
// returns three differently-shaped sets from one pass:
//
//	trend         — week by week, for a line. Direction over time.
//	by_department — one row per department (or per unit for a head of one), for a bar. Where.
//	outcomes      — taught / not taught / no record, for a pie. Part-to-whole of the term.
//
// SCOPED EXACTLY LIKE EVERY OTHER OVERSIGHT READ. A dean gets their college, a head of department
// gets their department, the directorate gets the institution — resolved from the account by
// lecturerLogScope, never from the request. That is what lets one endpoint serve all three
// dashboards without a role branch in the SQL.

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

type analyticsWeek struct {
	WeekStart     string  `json:"week_start"`
	Sessions      int     `json:"sessions"`
	Taught        int     `json:"taught"`
	AttendancePct float64 `json:"attendance_pct"`
	TaughtPct     float64 `json:"taught_pct"`
}

type analyticsGroup struct {
	Name          string  `json:"name"`
	Sessions      int     `json:"sessions"`
	AttendancePct float64 `json:"attendance_pct"`
}

// GET /api/v1/org/analytics
func OrgAnalytics(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		weeks := 12
		if v := strings.TrimSpace(r.URL.Query().Get("weeks")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 52 {
				weeks = n
			}
		}

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

		// The session spine, shared by all three result sets so they cannot disagree about which
		// lectures are in the window. Attendance is a subquery per session rather than a join: a
		// session has many attendance rows AND may have several lecturer rows, and joining both at
		// once multiplies them — the taught count would rise with class size.
		const spine = `
			WITH win AS (
			    SELECT (CURRENT_DATE - ($2::int * 7 - 1)) AS from_date
			),
			sess AS (
			    SELECT s.session_id, s.session_date, s.unit_id,
			           COALESCE(NULLIF(c.department,''), 'Unassigned') AS department,
			           COALESCE(NULLIF(c.school,''), 'Unassigned')     AS school,
			           COALESCE(cu.name, s.unit_id)                    AS unit_name,
			           EXISTS (SELECT 1 FROM lecturer_attendance_logs lal
			                    WHERE lal.session_id = s.session_id) AS has_record,
			           EXISTS (SELECT 1 FROM lecturer_attendance_logs lal
			                    WHERE lal.session_id = s.session_id
			                      AND lal.lecturer_scanned_at IS NOT NULL) AS taught,
			           (SELECT COUNT(*) FROM attendance_logs al WHERE al.session_id = s.session_id) AS checkins,
			           (SELECT COUNT(*) FROM students_extended se
			             WHERE se.enrollment_status = 'ACTIVE'
			               AND se.course_id = cu.course_id
			               AND se.offering_id IS NOT DISTINCT FROM s.offering_id) AS roll
			    FROM sessions s
			    JOIN course_units cu ON cu.unit_id = s.unit_id
			    LEFT JOIN courses c  ON c.course_id = cu.course_id
			    WHERE s.tenant_id = $1 AND s.session_date >= (SELECT from_date FROM win)`

		args := []interface{}{tenantID, weeks}
		scopeSQL := lecturerLogScope(r, pool, tenantID, &args)
		spineScoped := spine + scopeSQL + `)`

		// ── Trend: one point per ISO week ───────────────────────────────────
		trend := []analyticsWeek{}
		if rows, qErr := conn.Query(r.Context(), spineScoped+`
			SELECT date_trunc('week', session_date)::date AS wk,
			       COUNT(*), COUNT(*) FILTER (WHERE taught),
			       COALESCE(SUM(checkins),0), COALESCE(SUM(roll),0)
			FROM sess GROUP BY wk ORDER BY wk`, args...); qErr == nil {
			for rows.Next() {
				var wk time.Time
				var sessions, taught, checkins, roll int
				if rows.Scan(&wk, &sessions, &taught, &checkins, &roll) != nil {
					continue
				}
				a := analyticsWeek{WeekStart: wk.Format("2006-01-02"), Sessions: sessions, Taught: taught}
				a.AttendancePct = pct1(checkins, roll)
				a.TaughtPct = pct1(taught, sessions)
				trend = append(trend, a)
			}
			rows.Close()
		}

		// ── By department, or by unit for a head of a single department ─────
		// A HOD grouping by department gets one bar, which is a stat tile with extra steps (see
		// the form heuristic). Their comparison is between the UNITS they are responsible for, so
		// that is what they get — same chart, the axis that is actually informative for them.
		groupCol := "department"
		if middleware.GetRole(r.Context()) == middleware.RoleHOD ||
			middleware.GetRole(r.Context()) == middleware.RoleQADeptRep {
			groupCol = "unit_name"
		}
		groups := []analyticsGroup{}
		if rows, qErr := conn.Query(r.Context(), spineScoped+`
			SELECT `+groupCol+`, COUNT(*), COALESCE(SUM(checkins),0), COALESCE(SUM(roll),0)
			FROM sess GROUP BY 1 ORDER BY COUNT(*) DESC LIMIT 12`, args...); qErr == nil {
			for rows.Next() {
				var g analyticsGroup
				var checkins, roll int
				if rows.Scan(&g.Name, &g.Sessions, &checkins, &roll) != nil {
					continue
				}
				g.AttendancePct = pct1(checkins, roll)
				groups = append(groups, g)
			}
			rows.Close()
		}

		// ── Outcomes: the part-to-whole for the pie ─────────────────────────
		// Three genuinely different states, and the third is the reason this is worth charting:
		// a session with NO lecturer record at all is neither taught nor not-taught, it is
		// unevidenced, and folding it into either would be a claim the data does not support.
		var taught, notTaught, noRecord int
		_ = conn.QueryRow(r.Context(), spineScoped+`
			SELECT COUNT(*) FILTER (WHERE taught),
			       COUNT(*) FILTER (WHERE has_record AND NOT taught),
			       COUNT(*) FILTER (WHERE NOT has_record)
			FROM sess`, args...).Scan(&taught, &notTaught, &noRecord)

		// ── Employee time accuracy, by support department ────────────────────
		//
		// The terminal records four things about a working day and they are not degrees of the
		// same failure: arriving late, leaving early, missing the day entirely, and keeping the
		// hours as scheduled. Summed into one "attendance %" they become indistinguishable — a
		// department where everybody turns up twenty minutes late scores the same as one where a
		// fifth of the staff never came, and the two call for completely different conversations.
		// So each day is classified into exactly ONE bucket and the buckets are shown side by side.
		//
		// Precedence matters and is deliberate: absent first (there is no arrival time to judge),
		// then late (arriving late is the more serious of the two clock faults and outranks also
		// leaving early), then early, then on time. Without a fixed order a day that was both late
		// and early would be counted twice and the stack would exceed the number of days worked.
		//
		// `late` and `early` are the terminal's own free-text minute counts, so anything that is
		// neither empty nor a zero counts as a fault — the device writes '0', '' and sometimes
		// '00:00' for a clean day, and reading those as lateness would make every department look
		// delinquent.
		type empRow struct {
			Name    string  `json:"name"`
			Days    int     `json:"days"`
			OnTime  int     `json:"on_time"`
			Late    int     `json:"late"`
			Early   int     `json:"early"`
			Absent  int     `json:"absent"`
			OnTPct  float64 `json:"on_time_pct"`
			Workers int     `json:"employees"`
		}
		employees := []empRow{}
		var empDays, empOnTime int
		if rows, qErr := conn.Query(r.Context(), `
			WITH d AS (
			    SELECT COALESCE(NULLIF(btrim(department),''), 'Unassigned') AS dept,
			           ac_no,
			           CASE
			             WHEN absent THEN 'absent'
			             WHEN COALESCE(NULLIF(btrim(late), ''), '0')  NOT IN ('0','00:00','0:00') THEN 'late'
			             WHEN COALESCE(NULLIF(btrim(early), ''), '0') NOT IN ('0','00:00','0:00') THEN 'early'
			             ELSE 'on_time'
			           END AS bucket
			      FROM employee_attendance_days
			     WHERE tenant_id = $1
			       AND work_date >= CURRENT_DATE - ($2::int * 7 - 1)
			)
			SELECT dept, COUNT(*), COUNT(DISTINCT ac_no),
			       COUNT(*) FILTER (WHERE bucket = 'on_time'),
			       COUNT(*) FILTER (WHERE bucket = 'late'),
			       COUNT(*) FILTER (WHERE bucket = 'early'),
			       COUNT(*) FILTER (WHERE bucket = 'absent')
			  FROM d
			 GROUP BY dept
			 ORDER BY COUNT(*) DESC
			 LIMIT 12`, tenantID, weeks); qErr == nil {
			for rows.Next() {
				var e empRow
				if rows.Scan(&e.Name, &e.Days, &e.Workers, &e.OnTime, &e.Late, &e.Early, &e.Absent) == nil {
					e.OnTPct = pct1(e.OnTime, e.Days)
					empDays += e.Days
					empOnTime += e.OnTime
					employees = append(employees, e)
				}
			}
			rows.Close()
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"weeks":            weeks,
			"group_by":         groupCol,
			"trend":            trend,
			"by_group":         groups,
			"outcomes":         map[string]int{"taught": taught, "not_taught": notTaught, "no_record": noRecord},
			"session_total":    taught + notTaught + noRecord,
			"employee_time":    employees,
			"employee_days":    empDays,
			"employee_on_time": empOnTime,
		})
	}
}

// pct1 is a percentage to one decimal, and returns 0 when there is nothing to divide by.
// The caller is expected to distinguish "0%" from "no data" by the counts, not by this.
func pct1(part, whole int) float64 {
	if whole <= 0 {
		return 0
	}
	return float64(int(float64(part)/float64(whole)*1000+0.5)) / 10
}
