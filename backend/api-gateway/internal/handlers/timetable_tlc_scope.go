package handlers

// Who may design which department's timetable.
//
// The Teaching & Learning Centre owns the timetable. Migration 075 made that one institution-wide
// account, and one desk building every department's schedule is a bottleneck standing exactly where
// a term starts. Migration 083 puts a TLC in each department instead — the way an HOD sits in one —
// and this is the rule that makes the boundary real rather than advisory.
//
// THE SCOPE COMES FROM THE ACCOUNT, NEVER FROM THE REQUEST. A TLC does not say which department
// they are editing; their own user row says it. That is the same rule every org-scoped role in this
// system follows, and it is the whole difference between a scoped role and an unscoped one with a
// filter in the UI — a filter is a suggestion, and the request that omits it gets everything.
//
// A TLC with NO department is institution-wide. That is what every TLC account created before 083
// is, and demoting them silently on the morning of a deploy would lock the timetable's owner out of
// the timetable. It is also the right answer for an institution small enough to want one.

import (
	"context"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// tlcScopeError describes a refusal in the words the person reading it needs: which department they
// hold, and which one they just reached for.
type tlcScopeError struct {
	Theirs string
	Unit   string
	UnitIn string
}

func (e tlcScopeError) Error() string {
	if e.UnitIn == "" {
		return "the unit " + e.Unit + " is not filed under any department, so it cannot be matched to yours (" + e.Theirs + "). Ask an administrator to file its course."
	}
	return "you design the timetable for " + e.Theirs + "; " + e.Unit + " belongs to " + e.UnitIn + "."
}

// checkTimetableScope reports whether the caller may timetable unitID.
//
// Everyone except a departmental TLC passes: ADMIN and QA_OFFICER are institution-wide by role, and
// a TLC without a department is institution-wide by configuration. It returns the refusal rather
// than writing a response so the caller keeps its own error shape.
func checkTimetableScope(ctx context.Context, q *pgxpool.Conn, tenantID, userID, role, unitID string) error {
	if role != middleware.RoleTLC {
		return nil
	}

	var department string
	_ = q.QueryRow(ctx,
		`SELECT COALESCE(department,'') FROM users WHERE user_id = $1::uuid AND tenant_id = $2`,
		userID, tenantID).Scan(&department)
	if strings.TrimSpace(department) == "" {
		return nil // institution-wide TLC
	}

	var unitDept string
	_ = q.QueryRow(ctx, `
		SELECT COALESCE(c.department,'')
		  FROM course_units cu
		  JOIN courses c ON c.course_id = cu.course_id AND c.tenant_id = cu.tenant_id
		 WHERE cu.unit_id = $1 AND cu.tenant_id = $2`, unitID, tenantID).Scan(&unitDept)

	// Compared case- and whitespace-insensitively: these are two names typed by an administrator
	// on two different screens, and "Computer Science " must match "computer science".
	if strings.EqualFold(strings.TrimSpace(unitDept), strings.TrimSpace(department)) {
		return nil
	}
	return tlcScopeError{Theirs: department, Unit: unitID, UnitIn: unitDept}
}

// writeTimetableScopeRefusal answers a scope failure. 403, not 404: the unit exists and the caller
// is simply not its timetabler, and saying so is what lets them ask the right person.
func writeTimetableScopeRefusal(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusForbidden, errBody("OUT_OF_DEPARTMENT", err.Error()))
}

// tlcDepartment returns the department a TLC is confined to, or "" for an institution-wide one.
// Used by the read paths to label the page with whose timetable it is.
//
// It takes the CONNECTION the caller already holds, not the pool. Taking a fresh one from the RLS
// pool gets a connection with no app.current_tenant set, so the row-level policy on `users` hides
// every row — including the caller's own — and the answer comes back empty. Read as "no department",
// that silently promotes a departmental TLC to institution-wide on screen while the write path
// (which does hold a scoped connection) keeps refusing them: the page says one thing, the save says
// another, and neither is obviously wrong.
func tlcDepartment(ctx context.Context, conn *pgxpool.Conn, tenantID, userID, role string) string {
	if role != middleware.RoleTLC {
		return ""
	}
	var department string
	_ = conn.QueryRow(ctx,
		`SELECT COALESCE(department,'') FROM users WHERE user_id = $1::uuid AND tenant_id = $2`,
		userID, tenantID).Scan(&department)
	return strings.TrimSpace(department)
}
