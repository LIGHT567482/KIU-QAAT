package handlers

// The admin landing page's numbers.
//
//	GET /api/v1/admin/overview   (ADMIN)
//
// The admin home was a set of navigation tiles: it told you where the screens were, and nothing
// about whether the institution was actually working. These are the figures an administrator is
// the only person positioned to fix, grouped by the question each answers:
//
//	Is it set up?    accounts by role, courses, units, cohorts, rooms
//	Is it running?   sessions today, sessions this week, live right now
//	Is it broken?    units with no lecturer, cohorts with no coordinator, students with no cohort,
//	                 accounts that have never signed in, patrol handsets bound
//
// The third group is the point. Every one of those is a silent failure elsewhere in the system —
// an unstaffed unit shows a blank lecturer on the student's timetable and reaches the patrol
// manifest with nobody named against it; a student with no offering is invisible to their own
// coordinator's roster; an account that has never signed in is very often one whose password was
// never successfully handed over. None of them raise an error anywhere. They just quietly do
// nothing, which is why they belong on the first screen an admin sees.

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/clock"
	"github.com/qaat/api-gateway/internal/middleware"
	"github.com/qaat/api-gateway/internal/upanel"
)

// AdminOverview — GET /api/v1/admin/overview
func AdminOverview(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		ctx := r.Context()

		// one() runs a scalar count and returns 0 on any failure — a single missing table must
		// blank one tile, not the whole page.
		one := func(sql string, args ...interface{}) int {
			var n int
			all := append([]interface{}{tenantID}, args...)
			if pool.QueryRow(ctx, sql, all...).Scan(&n) != nil {
				return 0
			}
			return n
		}

		today := clock.Today()
		weekAgo := clock.Now().AddDate(0, 0, -7).Format("2006-01-02")

		// U-Panel rows have no tenant_id (single-institution import). Do not use one().
		raw := func(sql string, args ...interface{}) int {
			var n int
			if pool.QueryRow(ctx, sql, args...).Scan(&n) != nil {
				return 0
			}
			return n
		}
		upanel.RefreshIfEmpty(ctx, pool)

		// ── Roll call ────────────────────────────────────────────────────────
		byRole := map[string]int{}
		if rows, err := pool.Query(ctx,
			`SELECT role::text, COUNT(*) FROM users WHERE tenant_id = $1 GROUP BY 1`, tenantID); err == nil {
			for rows.Next() {
				var role string
				var n int
				if rows.Scan(&role, &n) == nil {
					byRole[role] = n
				}
			}
			rows.Close()
		}

		setup := map[string]int{
			"students":        one(`SELECT COUNT(*) FROM students_extended WHERE tenant_id=$1 AND enrollment_status='ACTIVE'`),
			"lecturers":       one(`SELECT COUNT(*) FROM lecturers WHERE tenant_id=$1`),
			"employees":       one(`SELECT COUNT(*) FROM employees WHERE tenant_id=$1`),
			"courses":         one(`SELECT COUNT(*) FROM courses WHERE tenant_id=$1`),
			"units":           one(`SELECT COUNT(*) FROM course_units WHERE tenant_id=$1`),
			"cohorts":         one(`SELECT COUNT(*) FROM course_offerings WHERE tenant_id=$1`),
			"schools":         one(`SELECT COUNT(*) FROM schools WHERE tenant_id=$1`),
			"departments":     one(`SELECT COUNT(*) FROM departments WHERE tenant_id=$1`),
			"rooms":           one(`SELECT COUNT(*) FROM rooms WHERE tenant_id=$1`),
			"timetable_slots": one(`SELECT COUNT(*) FROM timetable_slots WHERE tenant_id=$1`),
		}

		// ── Is it running? ───────────────────────────────────────────────────
		activity := map[string]int{
			"sessions_today":        one(`SELECT COUNT(*) FROM sessions WHERE tenant_id=$1 AND session_date = $2::date`, today),
			"sessions_week":         one(`SELECT COUNT(*) FROM sessions WHERE tenant_id=$1 AND session_date >= $2::date`, weekAgo),
			"sessions_live":         one(`SELECT COUNT(*) FROM sessions WHERE tenant_id=$1 AND session_status = 'OPEN'`),
			"checkins_today":        one(`SELECT COUNT(*) FROM attendance_logs al JOIN sessions s ON s.session_id = al.session_id WHERE s.tenant_id=$1 AND s.session_date = $2::date`, today),
			"lecturer_gates_today":  one(`SELECT COUNT(*) FROM lecturer_attendance_logs WHERE tenant_id=$1 AND session_date = $2::date`, today),
			"patrols_week":          one(`SELECT COUNT(*) FROM lecturer_patrol_logs WHERE tenant_id=$1 AND session_date >= $2::date`, weekAgo),
			"upanel_students_today": raw(`SELECT COUNT(*) FROM upanel_attendance WHERE kind = 'student' AND present AND occurred_at::date = $1::date`, today),
			"upanel_lectures_today": raw(`SELECT COUNT(*) FROM upanel_attendance WHERE kind = 'lecturer' AND occurred_at::date = $1::date`, today),
			"upanel_staff_today":    raw(`SELECT COUNT(*) FROM upanel_attendance WHERE kind = 'admin' AND occurred_at::date = $1::date`, today),
			"upanel_stored":         raw(`SELECT COUNT(*) FROM upanel_attendance`),
		}

		// Contabo often has U-Panel rows and an empty native registry.
		{
			c := upanel.LoadCensus(ctx, pool)
			if setup["students"] == 0 {
				setup["students"] = c.Students
			}
			if setup["lecturers"] == 0 {
				setup["lecturers"] = c.Lecturers
			}
			if setup["employees"] == 0 {
				setup["employees"] = c.Admins
			}
			if setup["courses"] == 0 {
				setup["courses"] = c.Courses
			}
			if setup["units"] == 0 {
				setup["units"] = c.Units
			}
			if activity["upanel_stored"] == 0 {
				activity["upanel_stored"] = c.Stored
			}
		}

		// ── What is quietly broken ───────────────────────────────────────────
		// Each of these is a real, silent failure somewhere else in the system.
		gaps := map[string]int{
			// No lecturer assigned: blank on the student's timetable, nameless on the patrol manifest.
			"units_unstaffed": one(`
				SELECT COUNT(*) FROM course_units cu
				WHERE cu.tenant_id = $1
				  AND NOT EXISTS (SELECT 1 FROM lecturer_assignments la
				                  WHERE la.unit_id = cu.unit_id)`),
			// No coordinator: nobody can open a session for that cohort at all.
			"cohorts_uncoordinated": one(`
				SELECT COUNT(*) FROM course_offerings o
				WHERE o.tenant_id = $1 AND COALESCE(o.coordinator_id,'') = ''`),
			// No offering: invisible to their own coordinator's roster and to cohort-scoped logs.
			"students_no_cohort": one(`
				SELECT COUNT(*) FROM students_extended
				WHERE tenant_id = $1 AND enrollment_status = 'ACTIVE' AND offering_id IS NULL`),
			// Never signed in — very often an account whose password never reached the person.
			"accounts_never_signed_in": one(`
				SELECT COUNT(*) FROM users WHERE tenant_id = $1 AND last_login_at IS NULL`),
			// Still on the seeded default, so anyone who knows the default can sign in as them.
			"accounts_default_password": one(`
				SELECT COUNT(*) FROM users WHERE tenant_id = $1 AND force_password_change = true`),
			// An org-scoped role with no unit set matches nothing — the account sees an empty screen.
			"org_roles_unscoped": one(`
				SELECT COUNT(*) FROM users
				WHERE tenant_id = $1
				  AND role IN ('HOD','QA_DEPT_REP','QA_OFFICER')     AND COALESCE(department,'') = ''
				   OR tenant_id = $1
				  AND role IN ('DEAN','QA_SCHOOL_HANDLER')           AND COALESCE(school,'') = ''`),
			// A department nobody heads. It runs, it teaches, and no one is answerable for it —
			// and it shows on the dean's Departments page as a red card they cannot fix themselves.
			"departments_no_hod": one(`
				SELECT COUNT(*) FROM departments d
				WHERE d.tenant_id = $1 AND COALESCE(d.kind,'ACADEMIC') <> 'SUPPORT'
				  AND NOT EXISTS (
				      SELECT 1 FROM users u
				      WHERE u.role = 'HOD'
				        AND btrim(lower(u.department)) = btrim(lower(d.name)))`),
			// A department on the org tree whose NAME matches no course. The org tree and the
			// academic data have drifted, so every query its HOD's dashboard runs returns nothing
			// — a working, blank dashboard with no clue as to why. Courses carry department as a
			// free-text name, which is exactly how the two get out of step.
			"departments_unlinked": one(`
				SELECT COUNT(*) FROM departments d
				WHERE d.tenant_id = $1 AND COALESCE(d.kind,'ACADEMIC') <> 'SUPPORT'
				  AND NOT EXISTS (
				      SELECT 1 FROM courses c
				      WHERE btrim(lower(COALESCE(c.department,''))) = btrim(lower(d.name)))`),
			// The mirror image: a course naming a department that does not exist on the org tree.
			// Its lecturers and students belong to no HOD and appear in no dean's college.
			"courses_orphan_department": one(`
				SELECT COUNT(*) FROM courses c
				WHERE c.tenant_id = $1 AND COALESCE(btrim(c.department),'') <> ''
				  AND NOT EXISTS (
				      SELECT 1 FROM departments d
				      WHERE btrim(lower(d.name)) = btrim(lower(c.department)))`),
			// Patrol handsets claimed, against patroller accounts that exist.
			"patrollers_unbound": one(`
				SELECT COUNT(*) FROM users u
				WHERE u.tenant_id = $1 AND u.role = 'QA_PATROLLER'
				  AND NOT EXISTS (SELECT 1 FROM patroller_device_bindings b WHERE b.user_id = u.user_id)`),
			// Sessions closed but never synced from a coordinator's phone.
			"sessions_unsynced": one(`
				SELECT COUNT(*) FROM sessions
				WHERE tenant_id = $1 AND session_status IN ('PENDING_SYNC')`),
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"accounts_by_role": byRole,
			"setup":            setup,
			"activity":         activity,
			"gaps":             gaps,
			"generated_at":     clock.Now().Format(time.RFC3339),
		})
	}
}
