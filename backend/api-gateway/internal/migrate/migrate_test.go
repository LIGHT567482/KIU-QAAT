package migrate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// ─── Filename parsing / ordering ─────────────────────────────────────────────

func TestParseFileName(t *testing.T) {
	ok := map[string][2]string{
		"001_init_schema.sql":              {"001", "init_schema"},
		"062_qa_rep_reports_and_rooms.sql": {"062", "qa_rep_reports_and_rooms"},
		"0100_four_digit.sql":              {"0100", "four_digit"},
		"046_fix.attendance.summary.sql":   {"046", "fix.attendance.summary"},
	}
	for in, want := range ok {
		v, n, got := ParseFileName(in)
		if !got || v != want[0] || n != want[1] {
			t.Errorf("ParseFileName(%q) = (%q,%q,%v), want (%q,%q,true)", in, v, n, got, want[0], want[1])
		}
	}
	// Anything not NNN_name.sql is ignored rather than mis-ordered into the run.
	for _, in := range []string{
		"README.md", "init.sql", "1_too_short.sql", "abc_name.sql",
		"001_init_schema.sql.bak", "001-init.sql", ".hidden.sql",
	} {
		if _, _, got := ParseFileName(in); got {
			t.Errorf("ParseFileName(%q) was accepted; want ignored", in)
		}
	}
}

func TestSortMigrationsOrdersNumerically(t *testing.T) {
	ms := []Migration{{Version: "010"}, {Version: "002"}, {Version: "0100"}, {Version: "062"}, {Version: "001"}}
	SortMigrations(ms)
	var got []string
	for _, m := range ms {
		got = append(got, m.Version)
	}
	want := []string{"001", "002", "010", "062", "0100"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestPendingPreservesOrderAndSkipsApplied(t *testing.T) {
	all := []Migration{{Version: "001"}, {Version: "002"}, {Version: "003"}, {Version: "004"}}
	// A deliberately ragged ledger — the real database had exactly this shape: some later
	// migrations applied by hand while an earlier one was missed.
	applied := map[string]bool{"001": true, "003": true}
	got := Pending(all, applied)
	if len(got) != 2 || got[0].Version != "002" || got[1].Version != "004" {
		t.Fatalf("Pending = %v, want [002 004]", versions(got))
	}
	// Nothing pending once everything is recorded — and the result is empty, never nil-panicking.
	if p := Pending(all, map[string]bool{"001": true, "002": true, "003": true, "004": true}); len(p) != 0 {
		t.Errorf("Pending on a fully applied ledger = %v, want none", versions(p))
	}
	if p := Pending(all, map[string]bool{}); len(p) != 4 {
		t.Errorf("Pending on an empty ledger = %d, want 4", len(p))
	}
}

func versions(ms []Migration) []string {
	out := []string{}
	for _, m := range ms {
		out = append(out, m.Version)
	}
	return out
}

// TestUpTo covers the selection behind a baseline. Getting this wrong either leaves migrations
// unrecorded (they would then be replayed against a database that already has them) or records
// ones that never ran (they would be skipped forever, silently missing).
func TestUpTo(t *testing.T) {
	all := []Migration{{Version: "001"}, {Version: "002"}, {Version: "010"}, {Version: "061"}, {Version: "062"}}

	got, ok := UpTo(all, "061")
	if !ok {
		t.Fatal("version 061 was not recognised")
	}
	if v := versions(got); strings.Join(v, ",") != "001,002,010,061" {
		t.Errorf("UpTo(061) = %v, want everything through 061 and nothing after", v)
	}

	// The boundary is inclusive — baselining "at 061" must record 061 itself.
	if last := got[len(got)-1].Version; last != "061" {
		t.Errorf("UpTo(061) stopped at %s; the named version must be included", last)
	}

	if got, _ := UpTo(all, "001"); len(got) != 1 {
		t.Errorf("UpTo(001) = %v, want just 001", versions(got))
	}
	if got, _ := UpTo(all, "062"); len(got) != len(all) {
		t.Errorf("UpTo(the last version) = %v, want all of them", versions(got))
	}

	// An unknown version must be refused, not silently rounded to a neighbour — baselining the
	// wrong set is exactly how a migration gets skipped forever.
	for _, bad := range []string{"999", "05", "61", "", "abc"} {
		if _, ok := UpTo(all, bad); ok {
			t.Errorf("UpTo(%q) was accepted; want refused", bad)
		}
	}
}

func TestLessOrEqual(t *testing.T) {
	if !lessOrEqual("001", "061") || !lessOrEqual("061", "061") || lessOrEqual("062", "061") {
		t.Error("same-width version comparison is wrong")
	}
	// A wider version number is always later, so 0100 must sort after 062.
	if !lessOrEqual("062", "0100") || lessOrEqual("0100", "062") {
		t.Error("versions of different widths compare wrongly")
	}
}

// ─── Statement splitting ─────────────────────────────────────────────────────

func TestSplitStatementsBasics(t *testing.T) {
	got := SplitStatements("CREATE TABLE a (x int);\nCREATE TABLE b (y int);\n")
	if len(got) != 2 {
		t.Fatalf("got %d statements: %#v", len(got), got)
	}
	if !strings.HasPrefix(got[0], "CREATE TABLE a") || !strings.HasPrefix(got[1], "CREATE TABLE b") {
		t.Errorf("split wrong: %#v", got)
	}
	// Trailing semicolons and blank space must not produce empty statements — an empty statement
	// sent to Postgres is an error, so this would break every migration ending in a newline.
	for _, in := range []string{"", "   \n\t ", ";;;", "SELECT 1;;\n;", "-- just a comment\n"} {
		for _, s := range SplitStatements(in) {
			if strings.TrimSpace(s) == "" {
				t.Errorf("SplitStatements(%q) produced an empty statement", in)
			}
		}
	}
}

func TestSplitStatementsIgnoresSemicolonsInsideLiterals(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want int
	}{
		{"single-quoted string", `INSERT INTO t VALUES ('a;b');`, 1},
		{"escaped quote in string", `INSERT INTO t VALUES ('it''s; fine');`, 1},
		{"quoted identifier", `CREATE TABLE "weird;name" (x int);`, 1},
		{"line comment", "SELECT 1; -- a comment with ; in it\nSELECT 2;", 2},
		{"comment before statement", "-- lead; comment\nSELECT 1;", 1},
		{"block comment", "SELECT 1 /* block ; comment */ ;", 1},
		{"nested block comment", "SELECT 1 /* outer /* inner ; */ still ; */ ;", 1},
		{"dollar quoted", "DO $$ BEGIN PERFORM 1; PERFORM 2; END $$;", 1},
		{"tagged dollar quote", "DO $tag$ BEGIN PERFORM 1; END $tag$;", 1},
		{"two dollar blocks", "DO $$ BEGIN PERFORM 1; END $$;\nDO $$ BEGIN PERFORM 2; END $$;", 2},
		{"dollar quote containing the other tag", "DO $a$ SELECT '$b$'; $a$;", 1},
	}
	for _, c := range cases {
		if got := SplitStatements(c.sql); len(got) != c.want {
			t.Errorf("%s: got %d statements, want %d\n%#v", c.name, len(got), c.want, got)
		}
	}
}

// TestSplitStatementsDollarPlaceholder guards the one ambiguity in dollar-quote detection: "$1" is
// a query placeholder, not the opening of a dollar-quoted body. Treating it as a quote would
// swallow the rest of the file.
func TestSplitStatementsDollarPlaceholder(t *testing.T) {
	sql := "UPDATE t SET a = $1 WHERE b = $2;\nSELECT 1;"
	if got := SplitStatements(sql); len(got) != 2 {
		t.Errorf("a $1 placeholder was mistaken for a dollar quote: got %d statements: %#v", len(got), got)
	}
}

func TestSplitStatementsPreservesContent(t *testing.T) {
	sql := `CREATE POLICY "tenant_isolation" ON schools
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);`
	got := SplitStatements(sql)
	if len(got) != 1 {
		t.Fatalf("got %d statements", len(got))
	}
	for _, frag := range []string{`"tenant_isolation"`, `current_setting('app.current_tenant', true)`, "::uuid"} {
		if !strings.Contains(got[0], frag) {
			t.Errorf("statement lost %q:\n%s", frag, got[0])
		}
	}
	if strings.Contains(got[0], ";") {
		t.Errorf("terminating semicolon should be stripped:\n%s", got[0])
	}
}

// ─── Error classification ────────────────────────────────────────────────────

// TestIsDuplicateObject pins the exact rule adopt mode relies on. Too wide and a genuine failure
// gets silently skipped, leaving the database short of the change; too narrow and adopting a
// hand-migrated database fails on the first CREATE POLICY.
func TestIsDuplicateObject(t *testing.T) {
	dup := map[string]string{
		"42P07": "duplicate_table",
		"42710": "duplicate_object (this is what CREATE POLICY raises)",
		"42701": "duplicate_column",
		"42723": "duplicate_function",
		"42P06": "duplicate_schema",
		"42712": "duplicate_alias",
	}
	for code, what := range dup {
		if !IsDuplicateObject(&pgconn.PgError{Code: code}) {
			t.Errorf("SQLSTATE %s (%s) should be treated as already-present", code, what)
		}
		// It must survive wrapping — Apply gets the error back through pgx.
		if !IsDuplicateObject(fmt.Errorf("exec failed: %w", &pgconn.PgError{Code: code})) {
			t.Errorf("SQLSTATE %s not detected through a wrapped error", code)
		}
	}
	notDup := map[string]string{
		"42P01": "undefined_table — a real missing dependency",
		"42703": "undefined_column",
		"23505": "unique_violation — real data conflict",
		"23503": "foreign_key_violation",
		"42601": "syntax_error",
		"42501": "insufficient_privilege",
		"57014": "query_canceled",
	}
	for code, what := range notDup {
		if IsDuplicateObject(&pgconn.PgError{Code: code}) {
			t.Errorf("SQLSTATE %s (%s) must NOT be skipped as already-present", code, what)
		}
	}
	// A non-Postgres error (network, context cancellation) is never a duplicate.
	if IsDuplicateObject(errors.New("connection reset by peer")) {
		t.Error("a plain error was classified as duplicate-object")
	}
	if IsDuplicateObject(nil) {
		t.Error("nil was classified as duplicate-object")
	}
}

// TestAdoptSkippable pins the second, narrower adopt rule. `DROP … IF EXISTS` can still fail on a
// database where the migration already ran, because a later migration changed what the name refers
// to — 007 turns a materialized view into a table, 036 turns an index into a constraint. Those
// failures mean "already done"; the same error codes anywhere else mean something is genuinely
// wrong and must stop the run.
func TestAdoptSkippable(t *testing.T) {
	wrongType := &pgconn.PgError{Code: "42809"} // wrong_object_type
	dependent := &pgconn.PgError{Code: "2BP01"} // dependent_objects_still_exist

	skippable := []struct {
		name string
		stmt string
		err  error
	}{
		{"007 materialized view became a table", "DROP MATERIALIZED VIEW IF EXISTS student_attendance_summary", wrongType},
		{"036 index became a constraint", "DROP INDEX IF EXISTS ux_offerings_cohort", dependent},
		{"leading comment block", "-- 036 — make the cohort uniqueness DEFERRABLE\n\nDROP INDEX IF EXISTS ux_offerings_cohort", dependent},
		{"block comment", "/* note */ DROP INDEX IF EXISTS ux_offerings_cohort", dependent},
		{"lower case", "drop index if exists ux_offerings_cohort", dependent},
		{"duplicate object still wins anywhere", "CREATE POLICY tenant_isolation ON schools FOR ALL USING (true)", &pgconn.PgError{Code: "42710"}},
	}
	for _, c := range skippable {
		if !AdoptSkippable(c.stmt, c.err) {
			t.Errorf("%s: should be skippable while adopting", c.name)
		}
	}

	notSkippable := []struct {
		name string
		stmt string
		err  error
	}{
		// The whole point of tying these codes to DROP: a CREATE or ALTER hitting wrong_object_type
		// means the schema is genuinely not what the migration expects.
		{"CREATE hitting wrong_object_type", "CREATE INDEX ix_a ON t (c)", wrongType},
		{"ALTER hitting wrong_object_type", "ALTER TABLE t ADD COLUMN c int", wrongType},
		{"DROP CASCADE dependency on a real drop", "DROP TABLE users", dependent},
		{"DROP without IF EXISTS", "DROP INDEX ux_offerings_cohort", dependent},
		{"a real missing table", "DROP INDEX IF EXISTS ix", &pgconn.PgError{Code: "42P01"}},
		{"a syntax error", "DROP INDEX IF EXISTS ix", &pgconn.PgError{Code: "42601"}},
		{"insufficient privilege", "DROP INDEX IF EXISTS ix", &pgconn.PgError{Code: "42501"}},
		{"a non-postgres error", "DROP INDEX IF EXISTS ix", errors.New("connection reset")},
	}
	for _, c := range notSkippable {
		if AdoptSkippable(c.stmt, c.err) {
			t.Errorf("%s: must NOT be skipped — it would hide a real failure", c.name)
		}
	}
}

func TestStripLeadingComments(t *testing.T) {
	cases := map[string]string{
		"SELECT 1":                     "SELECT 1",
		"-- one\nSELECT 1":             "SELECT 1",
		"-- one\n-- two\n\n  SELECT 1": "SELECT 1",
		"/* block */ SELECT 1":         "SELECT 1",
		"/* a */\n-- b\nSELECT 1":      "SELECT 1",
		"-- only a comment":            "",
		"/* unterminated":              "",
	}
	for in, want := range cases {
		if got := stripLeadingComments(in); got != want {
			t.Errorf("stripLeadingComments(%q) = %q, want %q", in, got, want)
		}
	}
}

// ─── Loading from disk ───────────────────────────────────────────────────────

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("002_second.sql", "SELECT 2;")
	write("001_first.sql", "SELECT 1;")
	write("README.md", "not a migration")
	write("notes.sql", "SELECT 'unnumbered, ignored';")

	ms, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(ms) != 2 {
		t.Fatalf("loaded %d migrations, want 2: %v", len(ms), versions(ms))
	}
	if ms[0].Version != "001" || ms[1].Version != "002" {
		t.Errorf("not ordered: %v", versions(ms))
	}
	if ms[0].Name != "first" || ms[0].FileName != "001_first.sql" {
		t.Errorf("bad metadata: %+v", ms[0])
	}
	if len(ms[0].Checksum) != 64 {
		t.Errorf("checksum is not a sha256 hex digest: %q", ms[0].Checksum)
	}
	if ms[0].Checksum == ms[1].Checksum {
		t.Error("different files produced the same checksum")
	}

	// Two files claiming the same version is ambiguous ordering — refuse rather than pick one.
	write("001_duplicate.sql", "SELECT 3;")
	if _, err := Load(dir); err == nil {
		t.Error("duplicate version was accepted; want an error")
	}

	if _, err := Load(filepath.Join(dir, "does-not-exist")); err == nil {
		t.Error("a missing directory was accepted; want an error")
	}
	if _, err := Load(t.TempDir()); err == nil {
		t.Error("an empty directory was accepted; want an error")
	}
}

// TestLoadRealMigrations runs the loader over the repository's own db/migrations, so a badly named
// or duplicate-numbered file added later fails here rather than at deploy time.
func TestLoadRealMigrations(t *testing.T) {
	dir := repoMigrationsDir(t)
	ms, err := Load(dir)
	if err != nil {
		t.Fatalf("the repository's own migrations do not load: %v", err)
	}
	if len(ms) < 60 {
		t.Fatalf("only %d migrations loaded from %s — expected the full set", len(ms), dir)
	}
	if ms[0].Version != "001" {
		t.Errorf("first migration is %s, want 001", ms[0].FileName)
	}
	// Every file must split into at least one statement, and no statement may be blank.
	for _, m := range ms {
		stmts := SplitStatements(m.SQL)
		if len(stmts) == 0 {
			t.Errorf("%s split into zero statements", m.FileName)
		}
		for i, s := range stmts {
			if strings.TrimSpace(s) == "" {
				t.Errorf("%s statement %d is empty", m.FileName, i+1)
			}
		}
	}
}

// TestRealMigrationsDollarBlocksSurvive checks the files that actually use DO $$ … $$ blocks: if
// the splitter broke one apart, the fragments would be invalid SQL and the migration would fail
// half-applied. 009 (the RLS/role bootstrap) is the important one.
func TestRealMigrationsDollarBlocksSurvive(t *testing.T) {
	ms, err := Load(repoMigrationsDir(t))
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, m := range ms {
		if !strings.Contains(m.SQL, "$$") {
			continue
		}
		checked++
		for _, s := range SplitStatements(m.SQL) {
			// A DO block must keep matched $$ delimiters — an odd count means it was cut in half.
			if n := strings.Count(s, "$$"); n%2 != 0 {
				t.Errorf("%s: a dollar-quoted block was split apart:\n%s", m.FileName, truncate(s, 300))
			}
		}
	}
	if checked == 0 {
		t.Skip("no migration uses $$ blocks any more")
	}
	t.Logf("verified dollar-quoted blocks in %d migration(s)", checked)
}

// repoMigrationsDir walks up from the package directory to the repository root.
func repoMigrationsDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, "db", "migrations")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	t.Skip("db/migrations not found relative to the test working directory")
	return ""
}
