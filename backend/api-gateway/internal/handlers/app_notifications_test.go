package handlers

// THE BUG CLASS THIS FILE EXISTS FOR.
//
// resolveRecipients decides WHO RECEIVES A MESSAGE. It builds SQL by appending optional fragments
// and numbering the placeholders from the running length of the args slice — so the numbering is
// only correct if every fragment that contributed an argument also contributed its reference. One
// missing `+ unitClause` in the lecturer→STUDENT branch left $3 bound and unreferenced, and the
// only symptom would have been Postgres replying "could not determine data type of parameter $3":
// a message naming no audience, no role, and no feature.
//
// That is untestable against a live database without a fixture of courses, offerings, rosters and
// assignments — and a test that heavy would not have been written, which is how the hole survived.
// The invariant, though, is pure: every bound parameter must appear in the query. This file tests
// that rule directly, and resolveRecipients now enforces it before any query is sent.

import (
	"os"
	"strings"
	"testing"
)

func TestDanglingParams_catchesTheGapThatShipped(t *testing.T) {
	// The exact shape of the lecturer→STUDENT branch when a unit filter was supplied: $3 was the
	// unit id, bound because unitID was non-empty, and referenced nowhere because the branch did
	// not append unitClause.
	sql := `SELECT u.user_id FROM lecturer_assignments la
	        WHERE la.tenant_id = $1 AND la.lecturer_id = $2::uuid
	          AND s.student_id = $4`
	got := danglingParams(sql, 4)
	if len(got) != 1 || got[0] != 3 {
		t.Fatalf("danglingParams = %v, want [3] — the unreferenced unit-id parameter", got)
	}
}

func TestDanglingParams_acceptsWellFormedQueries(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		argc int
	}{
		{"no parameters at all", `SELECT 1`, 0},
		{"single", `SELECT x WHERE a = $1`, 1},
		{"in order", `SELECT x WHERE a = $1 AND b = $2 AND c = $3`, 3},
		{"out of order is fine", `SELECT x WHERE c = $3 AND a = $1 AND b = $2`, 3},
		{"repeated references are fine", `SELECT x WHERE a = $1 AND b <> $1 AND c = $2`, 2},
		// The real fixed branch: unit clause present, so every parameter is referenced.
		{"lecturer STUDENT with unit filter", `SELECT u.user_id
			WHERE la.tenant_id = $1 AND la.lecturer_id = $2::uuid
			  AND s.student_id = $4 AND cu.unit_id = $3`, 4},
		// Casts and array operators must not confuse the scanner.
		{"casts and ANY()", `SELECT x WHERE t = $1 AND s = ANY($2) AND u = $3::uuid`, 3},
	}
	for _, c := range cases {
		if got := danglingParams(c.sql, c.argc); got != nil {
			t.Errorf("%s: danglingParams = %v, want none", c.name, got)
		}
	}
}

func TestDanglingParams_reportsEveryGap(t *testing.T) {
	// Two holes, both named, in order — so an error message points at all of them rather than
	// making somebody fix one and run into the next.
	got := danglingParams(`SELECT x WHERE a = $1 AND d = $4`, 4)
	if len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("danglingParams = %v, want [2 3]", got)
	}
}

func TestDanglingParams_ignoresPlaceholdersAboveTheArgCount(t *testing.T) {
	// A reference to $5 when only 3 arguments are bound is a different bug (Postgres rejects it
	// outright and says so clearly), and it must not make this check report $5 as "missing" — a
	// nonsensical claim that would send the reader hunting for an argument that should not exist.
	if got := danglingParams(`SELECT x WHERE a = $1 AND b = $2 AND c = $3 AND d = $5`, 3); got != nil {
		t.Errorf("danglingParams = %v, want none (an over-reference is not a dangling arg)", got)
	}
}

func TestDanglingParams_doubleDigitPlaceholders(t *testing.T) {
	// $1 must not satisfy $10, and $10 must not be read as $1 followed by a stray 0. The org-scoped
	// audiences already bind enough parameters to reach two digits.
	var b strings.Builder
	b.WriteString(`SELECT x WHERE `)
	for n := 1; n <= 12; n++ {
		if n > 1 {
			b.WriteString(" AND ")
		}
		b.WriteString("c = $")
		b.WriteString(itoa(n))
	}
	if got := danglingParams(b.String(), 12); got != nil {
		t.Errorf("danglingParams = %v, want none", got)
	}
	// Now drop the reference to $1 only. $10, $11 and $12 all contain a '1' and must not cover it.
	sql := strings.Replace(b.String(), "c = $1 AND ", "", 1)
	got := danglingParams(sql, 12)
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("danglingParams = %v, want [1] — $10/$11/$12 must not satisfy $1", got)
	}
}

// ─── The audience vocabulary ────────────────────────────────────────────────
//
// Which audience strings each sending role answers to is a security decision — it is the list of
// people a message can reach — and it is spread across a 250-line switch. These lock in the
// spellings the shipping clients actually send, so a rename cannot silently turn a working
// "send to my students" button into a no-op.

func TestAudienceSpellings_areStillHandledByTheResolver(t *testing.T) {
	// resolveRecipients needs a database, so the switch cannot be executed here. What CAN be
	// checked without one is that every audience string a shipping client sends still has a `case`
	// in the resolver — which is the failure that actually bites. A renamed or deleted case does
	// not error: it falls through to `default: return nil, nil`, the send reports "0 recipients",
	// and the feature looks broken with nothing in the logs. Exactly that happened once already,
	// which is why PATROLLERS/PATROLLER are still accepted alongside MONITORS/MONITOR.
	//
	// Client sources: NotificationUi.kt and CoordinatorApp.kt (Android); OrgLecturers.tsx,
	// QAMonitorBriefing.tsx and QAOrgDashboard.tsx (dashboards).
	src, err := os.ReadFile("app_notifications.go")
	if err != nil {
		t.Fatalf("could not read the resolver source: %v", err)
	}
	resolver := string(src)

	clientSends := []string{
		// lecturer
		"STUDENTS", "STUDENT", "COORDINATOR", "COORDINATORS",
		// coordinator
		"LECTURERS", "LECTURER",
		// HOD / dean / QA reps
		"DQA", "ADMIN", "HODS", "HOD",
		// QA officer + DQA director, both spellings of the renamed role
		"MONITORS", "MONITOR", "PATROLLERS", "PATROLLER",
	}
	for _, a := range clientSends {
		if a != strings.ToUpper(a) {
			t.Errorf("audience %q must be upper-case — the resolver compares exact strings", a)
		}
		// The audience appears as a quoted case label, e.g. `case "STUDENTS":` or
		// `case "MONITOR", "PATROLLER":`.
		if !strings.Contains(resolver, `"`+a+`"`) {
			t.Errorf("audience %q is sent by a shipping client but no longer appears in "+
				"resolveRecipients — it will fall through to default and silently reach nobody", a)
		}
	}
}
