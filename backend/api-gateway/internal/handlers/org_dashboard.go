package handlers

// The org-scoped dashboards: HOD, Dean, and the two QA rep roles.
//
//	GET /api/v1/org/overview   — the KPI header for whatever unit the caller is bounded to
//	GET /api/v1/org/at-risk    — students below the attendance threshold, worst first
//
// SCOPE IS NOT A PARAMETER. Every one of these roles is bounded by exactly one org unit, carried on
// their own account: HOD and QA_DEPT_REP by `users.department`, DEAN and QA_SCHOOL_HANDLER by
// `users.school`. The caller cannot name a different one — there is no query parameter for it — so
// a dean cannot read another college by asking nicely. The institution-wide roles (DQA, QA officer,
// VC/DVC, ADMIN) get the unscoped view of the same data, which is what makes /org/at-risk one page
// rather than four.
//
// An account with a BLANK org unit matches nothing rather than everything. That is the whole reason
// the admin form refuses to create one without it: an empty scope that matched every department
// would quietly hand a head of one department the whole institution.

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/clock"
	"github.com/qaat/api-gateway/internal/middleware"
)

// orgScope resolves the caller's own org unit and how it filters `courses`.
//
// Returns ok=false when the role is bounded but the account carries no unit — the caller then gets
// an empty result and a message telling them to have an admin set it, which is far better than the
// silent nothing they used to get.
type scope struct {
	Department string
	School     string
	// Column on `courses` this caller filters by, and the value. Empty col = institution-wide.
	Col, Val  string
	Unbounded bool
	// Every string that means this scope. For a school that is its full name AND its short form
	// (SOMAC), because historic rows hold whichever the institution was using at the time — see
	// schoolAliases. For a department it is just the one name.
	Aliases []string
}

func resolveOrgScope(r *http.Request, pool *pgxpool.Pool, tenantID, userID, role string) (scope, bool) {
	var s scope
	// THE SCOPE IS READ OVER A TENANT-SCOPED CONNECTION, and it has to be.
	//
	// This used to query the pool directly. A connection taken straight from the RLS pool carries
	// no app.current_tenant, so the row-level policy on `users` hides EVERY row — including the
	// caller's own — and the department came back empty with the error discarded. What that meant
	// depended on the role, and both readings were wrong: a head of department was told "no
	// department is set on your account" and shown an empty page, while any role whose empty case
	// falls through to unbounded would have been handed the whole institution.
	//
	// It is the same trap SetTenantConn exists to close, and the reason every other handler in
	// this package acquires a connection before it reads anything tenant-scoped.
	if conn, err := pool.Acquire(r.Context()); err == nil {
		defer conn.Release()
		if middleware.SetTenantConn(r.Context(), conn, tenantID) == nil {
			_ = conn.QueryRow(r.Context(),
				`SELECT COALESCE(department,''), COALESCE(school,'') FROM users WHERE user_id = $1::uuid AND tenant_id = $2`,
				userID, tenantID).Scan(&s.Department, &s.School)
		}
	}

	switch role {
	case middleware.RoleHOD, middleware.RoleQADeptRep:
		s.Col, s.Val = "c.department", s.Department
		s.Aliases = []string{s.Department}
	case middleware.RoleTLC:
		// A TLC designs one department's timetable (migration 083) and, with it, staffs that
		// department's lectures — so they are bounded exactly like a head of department.
		//
		// The fall-through matters: a TLC with NO department drops to the unbounded default
		// below, which is what every TLC account created before 083 is and what a small
		// institution wants. Putting them in this case unconditionally would have scoped them to
		// the empty string, and whereScope renders that as "match nothing" — the timetable's
		// owner would have opened their page to a blank week with no error to explain it.
		if strings.TrimSpace(s.Department) != "" {
			s.Col, s.Val = "c.department", s.Department
			s.Aliases = []string{s.Department}
			break
		}
		s.Unbounded = true
		return s, true
	case middleware.RoleDean, middleware.RoleQASchool:
		s.Col, s.Val = "c.school", s.School
		// A dean whose account says "SOMAC" must still match courses filed under the full title,
		// and vice versa.
		s.Aliases = schoolAliases(r.Context(), pool, tenantID, s.School)
		// A QA school handler is routinely given more than one school, and the single
		// users.school column had nowhere to put the second — so everything outside
		// their first school was simply invisible to them, with no error to explain it.
		// user_schools (migration 075) carries the rest; every name is widened through
		// the same alias lookup so abbreviations still match.
		for _, extra := range userSchools(r.Context(), pool, tenantID, userID) {
			s.Aliases = append(s.Aliases, schoolAliases(r.Context(), pool, tenantID, extra)...)
			if strings.TrimSpace(s.Val) == "" {
				s.Val = extra // a handler with no legacy users.school is still scoped
			}
		}
	default:
		// DQA / QA officer / VC / DVC / ADMIN see the institution.
		s.Unbounded = true
		return s, true
	}
	return s, strings.TrimSpace(s.Val) != ""
}

// whereScope renders the scope as SQL, appending its bind value.
//
// Matches against the whole alias set, not one string: for a dean that is the college's full name
// and its abbreviation, so a rename does not orphan every row written before it. Compared case- and
// whitespace-insensitively, because these are names typed by an admin on one screen and matched
// against names typed on another — the org picker makes both come from the same list now, but
// historic rows predate it.
func (s scope) whereScope(args *[]interface{}) string {
	if s.Unbounded {
		return ""
	}
	aliases := normaliseAliases(s.Aliases)
	if len(aliases) == 0 {
		aliases = normaliseAliases([]string{s.Val})
	}
	*args = append(*args, aliases)
	return " AND btrim(lower(" + s.Col + ")) = ANY($" + strconv.Itoa(len(*args)) + ")"
}

// OrgOverview — GET /api/v1/org/overview
//
// The header every org dashboard opens with. Six numbers, each chosen because somebody has to act
// on it: how many lecturers and students fall under this unit, what proportion of timetabled
// classes were actually taught, how many students are below the attendance bar, and where the
// coverage gaps are (units with no lecturer assigned — invisible everywhere else).
func OrgOverview(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		role := middleware.GetRole(r.Context())

		s, ok := resolveOrgScope(r, pool, tenantID, userID, role)
		if !ok {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"scope":   map[string]string{"department": s.Department, "school": s.School},
				"unset":   true,
				"message": "No department or college is set on your account, so there is nothing to show. Ask an administrator to set it — it is the scope of everything you see.",
			})
			return
		}

		// Window: this term's worth of sessions. 90 days is a semester either side of a break.
		since := clock.Now().AddDate(0, 0, -90).Format("2006-01-02")

		type out struct {
			Lecturers       int     `json:"lecturers"`
			Students        int     `json:"students"`
			Courses         int     `json:"courses"`
			Units           int     `json:"units"`
			UnitsUnstaffed  int     `json:"units_unstaffed"`
			SessionsHeld    int     `json:"sessions_held"`
			SessionsPlanned int     `json:"sessions_planned"`
			TaughtRate      float64 `json:"taught_rate"`
			AvgAttendance   float64 `json:"avg_attendance"`
			AtRisk          int     `json:"at_risk"`
			Threshold       float64 `json:"threshold"`
		}
		var o out

		// Courses + units in scope, and the units with nobody assigned to teach them. An unstaffed
		// unit is invisible on every other screen: it shows a blank lecturer on the student's
		// timetable and reaches the patrol manifest with nobody named against it.
		args := []interface{}{tenantID}
		_ = pool.QueryRow(r.Context(), `
			SELECT COUNT(DISTINCT c.course_id),
			       COUNT(DISTINCT cu.unit_id),
			       COUNT(DISTINCT cu.unit_id) FILTER (
			           WHERE NOT EXISTS (SELECT 1 FROM lecturer_assignments la
			                             WHERE la.unit_id = cu.unit_id))
			FROM courses c
			LEFT JOIN course_units cu ON cu.course_id = c.course_id
			WHERE c.tenant_id = $1`+s.whereScope(&args), args...,
		).Scan(&o.Courses, &o.Units, &o.UnitsUnstaffed)

		// Distinct lecturers teaching anything in scope.
		args = []interface{}{tenantID}
		_ = pool.QueryRow(r.Context(), `
			SELECT COUNT(DISTINCT l.lecturer_id)
			FROM lecturers l
			JOIN lecturer_assignments la ON la.lecturer_id = l.lecturer_id
			JOIN course_units cu ON cu.unit_id = la.unit_id
			JOIN courses c ON c.course_id = cu.course_id
			WHERE l.tenant_id = $1`+s.whereScope(&args), args...).Scan(&o.Lecturers)

		// Active students on courses in scope.
		args = []interface{}{tenantID}
		_ = pool.QueryRow(r.Context(), `
			SELECT COUNT(*)
			FROM students_extended se
			JOIN courses c ON c.course_id = se.course_id
			WHERE se.tenant_id = $1 AND se.enrollment_status = 'ACTIVE'`+s.whereScope(&args), args...).Scan(&o.Students)

		// Sessions actually held in the window vs the timetable's expectation for the same period.
		args = []interface{}{tenantID, since}
		_ = pool.QueryRow(r.Context(), `
			SELECT COUNT(*)
			FROM sessions ss
			JOIN course_units cu ON cu.unit_id = ss.unit_id
			JOIN courses c ON c.course_id = cu.course_id
			WHERE ss.tenant_id = $1 AND ss.session_date >= $2::date`+s.whereScope(&args), args...).Scan(&o.SessionsHeld)

		// The denominator: each weekly timetable slot recurs once a week over the window.
		args = []interface{}{tenantID}
		var weeklySlots int
		_ = pool.QueryRow(r.Context(), `
			SELECT COUNT(*)
			FROM timetable_slots ts
			JOIN course_units cu ON cu.unit_id = ts.unit_id
			JOIN courses c ON c.course_id = cu.course_id
			WHERE ts.tenant_id = $1`+s.whereScope(&args), args...).Scan(&weeklySlots)
		o.SessionsPlanned = weeklySlots * 13 // ~13 teaching weeks in the 90-day window
		if o.SessionsPlanned > 0 {
			o.TaughtRate = round1(float64(o.SessionsHeld) / float64(o.SessionsPlanned) * 100)
			if o.TaughtRate > 100 {
				o.TaughtRate = 100
			}
		}

		// Attendance + the at-risk count, against the institution's own threshold.
		args = []interface{}{tenantID}
		_ = pool.QueryRow(r.Context(), `
			SELECT ROUND(COALESCE(AVG(sas.attendance_percentage),0), 1),
			       COUNT(DISTINCT sas.student_id) FILTER (WHERE sas.attendance_percentage < 75 /* fixed: internal/policy.AttendanceThresholdPercent */),
			       MAX(75 /* fixed: internal/policy.AttendanceThresholdPercent */)
			FROM student_attendance_summary sas
			JOIN students_extended se ON se.student_id = sas.student_id
			JOIN courses c ON c.course_id = se.course_id
			CROSS JOIN tenants t
			WHERE sas.tenant_id = $1`+s.whereScope(&args), args...).Scan(&o.AvgAttendance, &o.AtRisk, &o.Threshold)

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"scope": map[string]string{
				"department": s.Department, "school": s.School,
				"label": orgScopeLabel(s, role),
			},
			"window_days": 90,
			"kpis":        o,
		})
	}
}

func orgScopeLabel(s scope, role string) string {
	switch role {
	case middleware.RoleHOD, middleware.RoleQADeptRep:
		return s.Department
	case middleware.RoleDean, middleware.RoleQASchool:
		return s.School
	}
	return "Institution-wide"
}

// OrgAtRisk — GET /api/v1/org/at-risk[?limit=&course=]
//
// The students who will lose exam eligibility if nothing changes, worst first, with the number of
// sessions each must attend to recover. This existed only inside the DQA's CSV export, which meant
// the people who could actually DO something about it — the head of department, the dean, the QA
// rep who visits the class — could not see it at all.
//
// `deficit` is the actionable number: how many more sessions this student has to attend to reach
// the threshold, given how many have been held. A deficit that already exceeds the sessions
// remaining is beyond recovery, which is exactly what a HOD needs to know in week 9 rather than
// week 14.
func OrgAtRisk(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		role := middleware.GetRole(r.Context())

		s, ok := resolveOrgScope(r, pool, tenantID, userID, role)
		if !ok {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"unset": true, "students": []any{},
				"message": "No department or college is set on your account. Ask an administrator to set it.",
			})
			return
		}

		limit := 200
		if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 2000 {
			limit = n
		}

		args := []interface{}{tenantID}
		q := `
			SELECT sas.student_id, se.full_name, COALESCE(se.email,''),
			       COALESCE(c.name, se.course_id), COALESCE(c.department,''), COALESCE(c.school,''),
			       sas.unit_id, COALESCE(cu.name, sas.unit_id),
			       sas.sessions_held, sas.sessions_attended, sas.attendance_percentage,
			       75 /* fixed: internal/policy.AttendanceThresholdPercent */,
			       GREATEST(0, CEIL(75 /* fixed: internal/policy.AttendanceThresholdPercent */::numeric / 100.0 * sas.sessions_held) - sas.sessions_attended)::int
			FROM student_attendance_summary sas
			JOIN students_extended se ON se.student_id = sas.student_id
			JOIN courses c ON c.course_id = se.course_id
			LEFT JOIN course_units cu ON cu.unit_id = sas.unit_id
			CROSS JOIN tenants t
			WHERE sas.tenant_id = $1
			  AND se.enrollment_status = 'ACTIVE'
			  AND sas.attendance_percentage < 75 /* fixed: internal/policy.AttendanceThresholdPercent */` + s.whereScope(&args)

		if course := strings.TrimSpace(r.URL.Query().Get("course")); course != "" {
			args = append(args, course)
			q += " AND se.course_id = $" + strconv.Itoa(len(args))
		}
		args = append(args, limit)
		q += " ORDER BY sas.attendance_percentage ASC, sas.student_id LIMIT $" + strconv.Itoa(len(args))

		rows, err := pool.Query(r.Context(), q, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type risk struct {
			StudentID  string  `json:"student_id"`
			FullName   string  `json:"full_name"`
			Email      string  `json:"email"`
			Course     string  `json:"course_name"`
			Department string  `json:"department"`
			School     string  `json:"school"`
			UnitID     string  `json:"unit_id"`
			UnitName   string  `json:"unit_name"`
			Held       int     `json:"sessions_held"`
			Attended   int     `json:"sessions_attended"`
			Pct        float64 `json:"attendance_percentage"`
			Threshold  float64 `json:"threshold"`
			Deficit    int     `json:"deficit_sessions"`
		}
		out := []risk{}
		for rows.Next() {
			var x risk
			if rows.Scan(&x.StudentID, &x.FullName, &x.Email, &x.Course, &x.Department, &x.School,
				&x.UnitID, &x.UnitName, &x.Held, &x.Attended, &x.Pct, &x.Threshold, &x.Deficit) == nil {
				out = append(out, x)
			}
		}

		// Distinct students, not rows: one student failing four units is one person to talk to.
		seen := map[string]bool{}
		for _, x := range out {
			seen[x.StudentID] = true
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"scope":             orgScopeLabel(s, role),
			"students":          out,
			"distinct_students": len(seen),
		})
	}
}
