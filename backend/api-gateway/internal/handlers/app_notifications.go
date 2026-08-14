package handlers

// Cross-role in-app notifications (workstream D). A LECTURER notifies the students of his
// unit(s) or the coordinator(s) of those units; a COORDINATOR notifies his cohort's students
// or the lecturer(s) of his course units. Recipients are materialised at send time so a
// reader's inbox (student / coordinator / lecturer) is a simple, fast lookup.
//
//   POST /api/v1/app-notifications                       (LECTURER, COORDINATOR)
//   GET  /api/v1/app-notifications                       (any signed-in app user)
//   GET  /api/v1/app-notifications/unread-count          (any signed-in app user)
//   POST /api/v1/app-notifications/{id}/read             (any signed-in app user)
//   DELETE /api/v1/app-notifications/{id}                (dismiss from your own inbox)

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// resolveRecipients returns the distinct recipient user_ids for a send, based on the sender's
// role + the requested audience + an optional unit scope.
func resolveRecipients(pool *pgxpool.Pool, r *http.Request, tenantID, senderID, senderRole, audience, unitID, targetID string) ([]string, error) {
	var sql string
	args := []interface{}{tenantID}

	switch senderRole {
	case middleware.RoleLecturer:
		lecturerID, ok := resolveLecturerID(pool, r, tenantID, senderID)
		if !ok {
			return nil, nil
		}
		args = append(args, lecturerID) // $2
		unitClause := ""
		if unitID != "" {
			args = append(args, unitID)
			unitClause = " AND cu.unit_id = $3"
		}
		// EVERY branch below carries lecturerNiche (see lecturer_recipients.go). Without it these
		// joined offerings on the COURSE, so a lecturer reached the students and coordinators of
		// cohorts they do not teach — the Evening and Weekend runs of a course they only take the
		// Day class for. A message is not harmless when it goes to the wrong people: it tells three
		// coordinators a room they do not run has no projector, and the one who should act assumes
		// somebody else will.
		switch audience {
		case "STUDENTS":
			sql = `SELECT DISTINCT u.user_id::text
				FROM lecturer_assignments la
				JOIN course_units cu     ON cu.unit_id = la.unit_id
				JOIN course_offerings o  ON o.course_id = cu.course_id
				JOIN students_extended s ON s.offering_id = o.offering_id AND s.enrollment_status='ACTIVE'
				JOIN users u ON lower(u.email) = lower(s.email)
				WHERE la.tenant_id = $1 AND la.lecturer_id = $2::uuid` + unitClause + lecturerNiche

		case "STUDENT":
			// ONE student, addressed by registration number. The lecturer must actually teach them:
			// the roster this is picked from is already scoped, but the target arrives from a
			// client and a client is the least trusted thing here.
			//
			// unitClause is carried here for the same reason its siblings carry it, and its absence
			// was a live tripwire rather than a missing feature: when unit_id was supplied, $3 was
			// BOUND and referenced nowhere, and Postgres cannot infer a type for a parameter that
			// never appears — the send would have failed outright with "could not determine data
			// type of parameter $3". No shipping client sends unit_id, so nobody had hit it; the
			// first person to wire up the per-unit filter would have.
			args = append(args, targetID)
			sql = `SELECT DISTINCT u.user_id::text
				FROM lecturer_assignments la
				JOIN course_units cu     ON cu.unit_id = la.unit_id
				JOIN course_offerings o  ON o.course_id = cu.course_id
				JOIN students_extended s ON s.offering_id = o.offering_id AND s.enrollment_status='ACTIVE'
				JOIN users u ON lower(u.email) = lower(s.email)
				WHERE la.tenant_id = $1 AND la.lecturer_id = $2::uuid
				  AND btrim(lower(s.student_id)) = btrim(lower($` + itoa(len(args)) + `))` + unitClause + lecturerNiche

		case "COORDINATOR", "COORDINATORS":
			// ONE coordinator when a target is given, all of the lecturer's coordinators when it is
			// not. A lecturer teaching one unit to four cohorts has four coordinators, and the
			// message they have in mind is almost always for exactly one of them — but "tell all my
			// coordinators" is a real thing to want too, and it is what this endpoint has always
			// done, so a handset that has not been updated keeps working rather than being told to
			// pick from a list its build has no screen for.
			target := ""
			if targetID != "" {
				args = append(args, targetID)
				target = ` AND o.coordinator_id = $` + itoa(len(args))
			}
			sql = `SELECT DISTINCT o.coordinator_id
				FROM lecturer_assignments la
				JOIN course_units cu    ON cu.unit_id = la.unit_id
				JOIN course_offerings o ON o.course_id = cu.course_id
				WHERE la.tenant_id = $1 AND la.lecturer_id = $2::uuid
				  AND COALESCE(o.coordinator_id,'') <> ''` + unitClause + target + lecturerNiche
		default:
			return nil, nil
		}

	case middleware.RoleCoordinator:
		args = append(args, senderID) // $2 = coordinator user_id
		unitClause := ""
		if unitID != "" {
			args = append(args, unitID)
			unitClause = " AND cu.unit_id = $3"
		}
		switch audience {
		case "STUDENTS":
			sql = `SELECT DISTINCT u.user_id::text
				FROM course_offerings o
				JOIN students_extended s ON s.offering_id = o.offering_id AND s.enrollment_status='ACTIVE'
				JOIN users u ON lower(u.email) = lower(s.email)
				WHERE o.tenant_id = $1 AND o.coordinator_id = $2`
		case "LECTURERS":
			sql = `SELECT DISTINCT l.user_id::text
				FROM course_offerings o
				JOIN course_units cu ON cu.course_id = o.course_id
				JOIN lecturer_assignments la ON la.unit_id = cu.unit_id
				JOIN lecturers l ON l.lecturer_id = la.lecturer_id
				WHERE o.tenant_id = $1 AND o.coordinator_id = $2 AND l.user_id IS NOT NULL` + unitClause
		case "STUDENT":
			// One specific student, but ONLY if they belong to this coordinator's cohort.
			args = append(args, targetID)
			sql = fmt.Sprintf(`SELECT u.user_id::text
				FROM course_offerings o
				JOIN students_extended s ON s.offering_id = o.offering_id AND s.enrollment_status='ACTIVE'
				JOIN users u ON lower(u.email) = lower(s.email)
				WHERE o.tenant_id = $1 AND o.coordinator_id = $2 AND s.student_id = $%d`, len(args))
		case "LECTURER":
			// One specific lecturer, but ONLY if they teach a unit of this coordinator's course.
			args = append(args, targetID)
			sql = fmt.Sprintf(`SELECT DISTINCT l.user_id::text
				FROM course_offerings o
				JOIN course_units cu ON cu.course_id = o.course_id
				JOIN lecturer_assignments la ON la.unit_id = cu.unit_id
				JOIN lecturers l ON l.lecturer_id = la.lecturer_id
				WHERE o.tenant_id = $1 AND o.coordinator_id = $2 AND l.user_id IS NOT NULL AND l.lecturer_id::text = $%d`, len(args))
		default:
			return nil, nil
		}
	case middleware.RoleHOD, middleware.RoleDean, middleware.RoleQADeptRep, middleware.RoleQASchool:
		// Scope = the sender's own org unit, from their user account. Which of the two columns
		// applies is decided by the role, not by anything in the request: a head of department and
		// a QA department rep are both bounded by users.department, a dean and a QA school handler
		// by users.school.
		var dept, school string
		_ = pool.QueryRow(r.Context(),
			`SELECT COALESCE(department,''), COALESCE(school,'') FROM users WHERE user_id = $1::uuid AND tenant_id = $2`,
			senderID, tenantID).Scan(&dept, &school)
		scopeCol := "c.department"
		scopeVal := dept
		if senderRole == middleware.RoleDean || senderRole == middleware.RoleQASchool {
			scopeCol = "c.school"
			scopeVal = school
		}
		// An org unit that was never set must reach nobody — otherwise a blank scope would match
		// every lecturer whose course also has a blank department.
		if strings.TrimSpace(scopeVal) == "" {
			return nil, nil
		}
		// A school answers to two names — its full title and its short form — and rows written
		// before the institution filled in the other one hold whichever was in use then. Address
		// the alias set so a broadcast does not silently reach nobody after a rename.
		scopeVals := normaliseAliases([]string{scopeVal})
		if senderRole == middleware.RoleDean || senderRole == middleware.RoleQASchool {
			scopeVals = normaliseAliases(schoolAliases(r.Context(), pool, tenantID, scopeVal))
		}
		lecturersInScope := `
			FROM lecturers l
			JOIN lecturer_assignments la ON la.lecturer_id = l.lecturer_id
			JOIN course_units cu ON cu.unit_id = la.unit_id
			JOIN courses c ON c.course_id = cu.course_id
			WHERE l.tenant_id = $1 AND l.user_id IS NOT NULL AND btrim(lower(` + scopeCol + `)) = ANY($2)`
		switch audience {
		case "LECTURERS": // bulk — every lecturer in scope
			args = append(args, scopeVals)
			sql = `SELECT DISTINCT l.user_id::text ` + lecturersInScope
		case "LECTURER": // one specific lecturer (by staff id), if in scope
			args = append(args, scopeVals, targetID)
			sql = `SELECT DISTINCT l.user_id::text ` + lecturersInScope + ` AND l.staff_id = $3`
		case "DQA": // message the DQA directors
			sql = `SELECT user_id::text FROM users WHERE tenant_id = $1 AND role = 'DQA_DIRECTOR'`
		case "ADMIN": // message the admins
			sql = `SELECT user_id::text FROM users WHERE tenant_id = $1 AND role = 'ADMIN'`

		// ── The management layer, which had no channel at all ────────────────
		// A dean is accountable for a college THROUGH its heads of department, and a head of
		// department answers upward to their dean — yet neither could send the other so much as a
		// notice. The dean could only address every lecturer in the school at once, going straight
		// past the person actually responsible for them.
		case "HODS": // DEAN / QA_SCHOOL_HANDLER → every HOD of a department in their school
			args = append(args, scopeVals)
			sql = `SELECT u.user_id::text
				FROM users u
				JOIN departments d ON btrim(lower(d.name)) = btrim(lower(u.department))
				JOIN schools s ON s.school_id = d.school_id
				WHERE u.tenant_id = $1 AND u.role = 'HOD'
				  AND (btrim(lower(s.name)) = ANY($2) OR btrim(lower(COALESCE(s.abbreviation,''))) = ANY($2))`
		case "HOD": // one specific head of department, if they run a department of this school
			args = append(args, scopeVals, targetID)
			sql = `SELECT u.user_id::text
				FROM users u
				JOIN departments d ON btrim(lower(d.name)) = btrim(lower(u.department))
				JOIN schools s ON s.school_id = d.school_id
				WHERE u.tenant_id = $1 AND u.role = 'HOD'
				  AND (btrim(lower(s.name)) = ANY($2) OR btrim(lower(COALESCE(s.abbreviation,''))) = ANY($2))
				  AND u.user_id::text = $3`
		case "DEAN": // HOD / QA_DEPT_REP → the dean of the school their department sits in
			args = append(args, scopeVal)
			// The dean's account may hold EITHER form of the school's name, so compare against
			// both. Matching only `schools.name` would miss a dean whose account still says
			// "SOMAC" after the college was given its full title — and a head of department would
			// see their message reach nobody, with no error to explain it.
			sql = `SELECT u.user_id::text
				FROM users u
				WHERE u.tenant_id = $1 AND u.role = 'DEAN'
				  AND COALESCE(u.school,'') <> ''
				  AND btrim(lower(COALESCE(u.school,''))) IN (
				      SELECT unnest(ARRAY[btrim(lower(COALESCE(s.name,''))),
				                          btrim(lower(COALESCE(s.abbreviation,'')))])
				      FROM departments d
				      LEFT JOIN schools s ON s.school_id = d.school_id
				      WHERE d.tenant_id = $1 AND btrim(lower(d.name)) = btrim(lower($2)))`
		default:
			return nil, nil
		}

	// ── Quality Assurance → its own field staff ──────────────────────────────
	//
	// The QA officer and the DQA director could not send a notification to ANYONE. Every other
	// role with people under it had a channel; the two roles that run the patrol round had none,
	// so the only way to tell a patroller anything — a round reassigned, a room changed, a phone
	// to bring in — was to find their number.
	//
	// Patrollers are institution-wide, not scoped to a department or a school, which is why this
	// is its own branch rather than another audience on the org-role case above: those queries all
	// hang off the sender's department/school, and a patroller has neither. QA is the one office
	// whose remit is the whole institution, so the scope is the tenant.
	case middleware.RoleQAOfficer, middleware.RoleDQADirector:
		switch audience {
		// MONITORS/MONITOR are what the dashboards send now that the role is called QA Monitor.
		// The old spellings are still accepted because a handset or a browser tab that has not
		// been reloaded since the rename would otherwise fail to send a briefing, and the failure
		// would look like the messaging feature being broken rather than a stale client.
		case "MONITORS", "PATROLLERS": // every monitor in the institution
			// is_active, because a suspended or departed patroller must not receive a round
			// briefing — and because a message that reports "sent to 9" when 3 of them cannot
			// sign in is worse than no number at all.
			sql = `SELECT user_id::text FROM users
				WHERE tenant_id = $1 AND role = 'QA_PATROLLER' AND COALESCE(is_active, true)`
		case "MONITOR", "PATROLLER": // one monitor, addressed by their staff id
			args = append(args, targetID)
			sql = `SELECT user_id::text FROM users
				WHERE tenant_id = $1 AND role = 'QA_PATROLLER' AND COALESCE(is_active, true)
				  AND btrim(lower(COALESCE(staff_id,''))) = btrim(lower($2))`
		// The rest of QA's reach, for the same reason: these roles had no way to write to anyone.
		case "LECTURERS":
			sql = `SELECT l.user_id::text FROM lecturers l
				WHERE l.tenant_id = $1 AND l.user_id IS NOT NULL`
		case "LECTURER":
			args = append(args, targetID)
			sql = `SELECT l.user_id::text FROM lecturers l
				WHERE l.tenant_id = $1 AND l.user_id IS NOT NULL
				  AND btrim(lower(l.staff_id)) = btrim(lower($2))`
		case "COORDINATORS":
			sql = `SELECT user_id::text FROM users
				WHERE tenant_id = $1 AND role = 'COORDINATOR' AND COALESCE(is_active, true)`
		default:
			return nil, nil
		}

	default:
		return nil, nil
	}

	// EVERY BOUND PARAMETER MUST BE REFERENCED, checked before the query is ever sent.
	//
	// The SQL above is assembled from fragments whose placeholder numbers are computed from the
	// length of `args` as it grows, and the fragments are appended CONDITIONALLY. That is a shape
	// where one missing `+ unitClause` silently leaves a hole in the numbering — which is exactly
	// what happened in the lecturer→STUDENT branch, and Postgres answers such a query with "could
	// not determine data type of parameter $3": a message that names no audience, no role and no
	// feature, and sends whoever reads it to the wrong place entirely.
	//
	// Catching it here costs one pass over a short string and turns the whole class of mistake
	// into a named, greppable failure. It is deliberately an ERROR and not a repair: a query with
	// a dangling parameter is one whose author meant something we cannot infer, and quietly
	// dropping the filter could send a message to a wider audience than intended.
	if gaps := danglingParams(sql, len(args)); len(gaps) > 0 {
		return nil, fmt.Errorf(
			"notification audience %q for role %s built a query binding %d parameters but never "+
				"referencing %v — this is a bug in resolveRecipients, not a bad request",
			audience, senderRole, len(args), gaps)
	}

	rows, err := pool.Query(r.Context(), sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil && id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// danglingParams reports which of the argc bound parameters ($1…$argc) never appear in sql.
//
// Pure and dependency-free so the rule can be tested exhaustively without a database — see
// app_notifications_test.go. Scanning for "$<digits>" is sufficient here because these queries are
// built from our own fragments: there are no dollar-quoted string literals and no user text is
// concatenated in, only placeholders.
func danglingParams(sql string, argc int) []int {
	seen := make([]bool, argc+1)
	for i := 0; i < len(sql); i++ {
		if sql[i] != '$' {
			continue
		}
		n, j := 0, i+1
		for j < len(sql) && sql[j] >= '0' && sql[j] <= '9' {
			n = n*10 + int(sql[j]-'0')
			j++
		}
		if j > i+1 && n >= 1 && n <= argc {
			seen[n] = true
		}
		i = j - 1
	}
	var missing []int
	for n := 1; n <= argc; n++ {
		if !seen[n] {
			missing = append(missing, n)
		}
	}
	return missing
}

// CoordinatorLecturers — GET /api/v1/coordinator/lecturers → the lecturers who teach this
// coordinator's course units (id + name), so the composer can target one specifically.
func CoordinatorLecturers(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		coordID := middleware.GetUserID(r.Context())
		rows, err := pool.Query(r.Context(), `
			SELECT DISTINCT l.lecturer_id::text, l.full_name, COALESCE(l.staff_id,'')
			FROM course_offerings o
			JOIN course_units cu ON cu.course_id = o.course_id
			JOIN lecturer_assignments la ON la.unit_id = cu.unit_id
			JOIN lecturers l ON l.lecturer_id = la.lecturer_id
			WHERE o.tenant_id = $1 AND o.coordinator_id = $2 AND l.user_id IS NOT NULL
			ORDER BY l.full_name`, tenantID, coordID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()
		type lec struct {
			LecturerID string `json:"lecturer_id"`
			FullName   string `json:"full_name"`
			StaffID    string `json:"staff_id"`
		}
		out := []lec{}
		for rows.Next() {
			var l lec
			if rows.Scan(&l.LecturerID, &l.FullName, &l.StaffID) == nil {
				out = append(out, l)
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// SendAppNotification — POST /api/v1/app-notifications
func SendAppNotification(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		senderID := middleware.GetUserID(r.Context())
		// The route already restricts this to the sending roles (see router.go); the `valid` map
		// below is what decides who may address whom. This early check used to hardcode
		// lecturer-or-coordinator and had silently fallen behind the map — an HOD passed the route
		// and was then refused here with a message naming two roles they are neither of.
		role := middleware.GetRole(r.Context())

		var req struct {
			Audience string `json:"audience"`
			UnitID   string `json:"unit_id"`
			TargetID string `json:"target_id"` // specific student_id or lecturer_id (for STUDENT/LECTURER)
			Subject  string `json:"subject"`
			Body     string `json:"body"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		req.Audience = strings.ToUpper(strings.TrimSpace(req.Audience))
		req.UnitID = strings.TrimSpace(req.UnitID)
		req.TargetID = strings.TrimSpace(req.TargetID)
		req.Subject = strings.TrimSpace(req.Subject)
		if req.Subject == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "subject is required"))
			return
		}
		// Validate audience against the sender's role.
		// Who each role may address. The school-level roles reach DOWN to their heads of
		// department and the department-level roles reach UP to their dean, so the management
		// chain is a channel in both directions instead of a gap everyone routed around by
		// notifying every lecturer at once.
		valid := map[string]map[string]bool{
			// A lecturer addresses their whole class, one student, all of their coordinators, or —
			// the case this was missing — ONE coordinator, because one unit is routinely taught to
			// four cohorts with four different coordinators.
			middleware.RoleLecturer:    {"STUDENTS": true, "STUDENT": true, "COORDINATOR": true, "COORDINATORS": true},
			middleware.RoleCoordinator: {"STUDENTS": true, "LECTURERS": true, "STUDENT": true, "LECTURER": true},
			middleware.RoleHOD:         {"LECTURERS": true, "LECTURER": true, "DEAN": true, "DQA": true, "ADMIN": true},
			middleware.RoleDean:        {"LECTURERS": true, "LECTURER": true, "HODS": true, "HOD": true, "DQA": true, "ADMIN": true},
			middleware.RoleQADeptRep:   {"LECTURERS": true, "LECTURER": true, "DEAN": true, "DQA": true, "ADMIN": true},
			middleware.RoleQASchool:    {"LECTURERS": true, "LECTURER": true, "HODS": true, "HOD": true, "DQA": true, "ADMIN": true},
			// Quality Assurance reaches its own field staff. PATROLLERS is the round briefing —
			// institution-wide, because that is the scope patrollers work at; PATROLLER is one
			// person, by staff id.
			middleware.RoleQAOfficer:   {"MONITORS": true, "MONITOR": true, "PATROLLERS": true, "PATROLLER": true, "LECTURERS": true, "LECTURER": true, "COORDINATORS": true},
			middleware.RoleDQADirector: {"MONITORS": true, "MONITOR": true, "PATROLLERS": true, "PATROLLER": true, "LECTURERS": true, "LECTURER": true, "COORDINATORS": true},
		}
		if !valid[role][req.Audience] {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "invalid audience for your role"))
			return
		}
		if (req.Audience == "STUDENT" || req.Audience == "LECTURER" || req.Audience == "HOD" ||
			req.Audience == "PATROLLER" || req.Audience == "MONITOR") && req.TargetID == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "pick a recipient"))
			return
		}

		recipients, err := resolveRecipients(pool, r, tenantID, senderID, role, req.Audience, req.UnitID, req.TargetID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		// Don't notify yourself.
		filtered := recipients[:0]
		for _, id := range recipients {
			if id != senderID {
				filtered = append(filtered, id)
			}
		}
		recipients = filtered
		// NOBODY TO SEND TO IS NOT A SEND.
		//
		// This answered "SENT, recipients: 0" — a success the app shows as a success, for a message
		// that went nowhere. It was always wrong (a QA officer briefing a team that has no members
		// believes they briefed it), and targeting made it dangerous: a lecturer picking a
		// particular coordinator whose id no longer resolves — stale list, cohort reassigned, the
		// target simply not theirs to write to — gets a tick and never learns the message was
		// discarded. The one thing they cannot afford is to think it arrived.
		if len(recipients) == 0 {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("NO_RECIPIENTS",
				"nobody could be found to send this to — the person or group you chose is not one "+
					"you can write to, or has no active accounts. Nothing was sent."))
			return
		}

		// Store the sender's TITLE alongside their name ("Dr Jane Smith"), so every reader —
		// inbox, dashboard, export — shows the courtesy title without having to re-join later.
		var senderName, senderTitle string
		_ = pool.QueryRow(r.Context(),
			`SELECT COALESCE(full_name,''), COALESCE(title,'') FROM users WHERE user_id = $1 AND tenant_id = $2`,
			senderID, tenantID).Scan(&senderName, &senderTitle)
		if senderName == "" {
			senderName = role
		}
		senderName = withTitle(senderTitle, senderName)

		tx, err := pool.Begin(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "tx"))
			return
		}
		defer tx.Rollback(r.Context()) //nolint:errcheck

		var nid string
		err = tx.QueryRow(r.Context(), `
			INSERT INTO app_notifications (tenant_id, sender_id, sender_name, sender_role, audience, unit_id, subject, body)
			VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8) RETURNING notification_id::text`,
			tenantID, senderID, senderName, role, req.Audience, req.UnitID, req.Subject, req.Body).Scan(&nid)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		for _, uid := range recipients {
			_, err = tx.Exec(r.Context(), `
				INSERT INTO notification_recipients (notification_id, tenant_id, recipient_user_id)
				VALUES ($1,$2,$3::uuid) ON CONFLICT DO NOTHING`, nid, tenantID, uid)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
				return
			}
		}
		if err := tx.Commit(r.Context()); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "commit"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"status": "SENT", "notification_id": nid, "recipients": len(recipients)})
	}
}

// ListAppNotifications — GET /api/v1/app-notifications (the caller's inbox).
func ListAppNotifications(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		rows, err := pool.Query(r.Context(), `
			SELECT n.notification_id::text, n.sender_name, COALESCE(u.title,''), n.sender_role,
			       COALESCE(n.unit_id,''),
			       n.subject, n.body, n.created_at, (nr.read_at IS NOT NULL),
			       -- What the reader can DO about it (migration 090). A "not taught" finding
			       -- travels with the reply to it, so the lecturer answers the accusation from the
			       -- message rather than having to know which other screen the answer lives on.
			       COALESCE(n.action,''), COALESCE(n.action_ref,'')
			FROM notification_recipients nr
			JOIN app_notifications n ON n.notification_id = nr.notification_id
			-- Rows written before titles were stored still get one, via the sender's account.
			LEFT JOIN users u ON u.user_id = n.sender_id
			WHERE nr.tenant_id = $1 AND nr.recipient_user_id = $2::uuid
			  AND nr.dismissed_at IS NULL
			ORDER BY n.created_at DESC LIMIT 300`, tenantID, userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()
		type notif struct {
			NotificationID string `json:"notification_id"`
			SenderName     string `json:"sender_name"`
			SenderRole     string `json:"sender_role"`
			UnitID         string `json:"unit_id"`
			Subject        string `json:"subject"`
			Body           string `json:"body"`
			CreatedAt      string `json:"created_at"`
			Read           bool   `json:"read"`
			Action         string `json:"action"`     // "" | APPEAL_NOT_TAUGHT
			ActionRef      string `json:"action_ref"` // unit|YYYY-MM-DD|HH:MM
		}
		out := []notif{}
		for rows.Next() {
			var n notif
			var created time.Time
			var title string
			if rows.Scan(&n.NotificationID, &n.SenderName, &title, &n.SenderRole, &n.UnitID,
				&n.Subject, &n.Body, &created, &n.Read, &n.Action, &n.ActionRef) != nil {
				continue
			}
			n.SenderName = withTitle(title, n.SenderName)
			n.CreatedAt = created.Format(time.RFC3339)
			out = append(out, n)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// UnreadAppNotificationCount — GET /api/v1/app-notifications/unread-count
func UnreadAppNotificationCount(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		var n int
		// Dismissed alerts are gone from the inbox, so they must be gone from the badge too —
		// otherwise the ✕ leaves a permanent "3 unread" the reader can never clear, because the
		// rows it counts are no longer listed anywhere they could be opened.
		_ = pool.QueryRow(r.Context(),
			`SELECT count(*) FROM notification_recipients
			  WHERE tenant_id=$1 AND recipient_user_id=$2::uuid AND read_at IS NULL AND dismissed_at IS NULL`,
			tenantID, userID).Scan(&n)
		writeJSON(w, http.StatusOK, map[string]int{"unread": n})
	}
}

// MarkAppNotificationRead — POST /api/v1/app-notifications/{id}/read
func MarkAppNotificationRead(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		id := extractPathID(r.URL.Path, "/api/v1/app-notifications/", "/read")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "missing id"))
			return
		}
		_, err := pool.Exec(r.Context(),
			`UPDATE notification_recipients SET read_at = now()
			 WHERE notification_id = $1::uuid AND recipient_user_id = $2::uuid AND tenant_id = $3 AND read_at IS NULL`,
			id, userID, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "READ"})
	}
}

// withTitle prefixes a courtesy title to a name once — "Dr" + "Jane Smith" -> "Dr Jane Smith".
// Blank titles pass through, and a name that already starts with the title is left alone so
// nobody ends up as "Dr Dr Jane Smith".
func withTitle(title, name string) string {
	t := strings.TrimSpace(title)
	n := strings.TrimSpace(name)
	if t == "" || n == "" {
		return n
	}
	if strings.HasPrefix(strings.ToLower(n), strings.ToLower(t)+" ") {
		return n
	}
	return t + " " + n
}

// DismissAppNotification — DELETE /api/v1/app-notifications/{id}
//
// Removes the notification from THIS recipient's inbox only. The notification itself and every
// other recipient's copy are untouched: an alert sent to a whole cohort is one row fanned out to
// many, and one student clearing their copy must not delete everyone else's.
//
// IDEMPOTENT. Dismissing something already dismissed is a success, not a 404. The clients treat a
// failed dismiss as "the server refused" and restore the card, so a second ✕ (a double-tap, a retry
// after a flaky connection, two devices signed into the same account) used to make the alert pop
// back — the very thing dismissal is supposed to prevent. Only a genuinely foreign id 404s.
func DismissAppNotification(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		id := strings.TrimSpace(chi.URLParam(r, "id"))
		if id == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "missing id"))
			return
		}
		// COALESCE keeps the FIRST dismissal's timestamp: re-dismissing must not rewrite history,
		// and the row still matches so RowsAffected reports the real "is this yours?" answer.
		ct, err := pool.Exec(r.Context(), `
			UPDATE notification_recipients
			   SET dismissed_at = COALESCE(dismissed_at, now())
			 WHERE notification_id = $1::uuid AND tenant_id = $2 AND recipient_user_id = $3::uuid`,
			id, tenantID, userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		if ct.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "no such notification in your inbox"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "DISMISSED"})
	}
}
