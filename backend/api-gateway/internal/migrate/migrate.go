// Package migrate applies the SQL files in db/migrations to a database, once each, in order,
// recording what it has done in a `schema_migrations` ledger.
//
// WHY THIS EXISTS. Until now migrations only ran automatically on an *empty* Postgres volume (the
// docker-entrypoint-initdb.d hook), and production was updated by `scripts/migrate-prod.sh`, which
// carried a hand-written list of three filenames. Every migration added after that list was written
// had to be remembered and applied by hand — and several were not, so long-shipped features simply
// did not exist in the running database while looking perfectly present in the code.
//
// THE ADOPT PROBLEM. A database in that state is *ragged*: some later migrations are applied, some
// earlier ones are not, and no record says which. Re-running an already-applied migration is mostly
// harmless here (the files use IF NOT EXISTS almost everywhere) except for `CREATE POLICY`, which
// has no IF NOT EXISTS form and hard-fails. Adopt mode therefore runs each migration
// statement-by-statement and skips only the individual statements that fail with a Postgres
// duplicate-object error class — so every statement that is genuinely new still lands, and nothing
// is guessed at. It is driven by SQLSTATE, not by pattern-matching error text.
package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Migration is one SQL file on disk.
type Migration struct {
	Version  string // "062" — the numeric prefix, the ledger key
	Name     string // "qa_rep_reports_and_rooms"
	FileName string // "062_qa_rep_reports_and_rooms.sql"
	SQL      string
	Checksum string // sha256 of SQL, so a file edited after being applied is detectable
}

// fileNamePattern matches "NNN_some_name.sql". The numeric prefix orders the run and keys the
// ledger; anything not matching is ignored rather than silently mis-ordered.
var fileNamePattern = regexp.MustCompile(`^(\d{3,})_(.+)\.sql$`)

// ParseFileName splits a migration filename into its version and name.
func ParseFileName(fileName string) (version, name string, ok bool) {
	m := fileNamePattern.FindStringSubmatch(fileName)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// Load reads every migration file in dir, ordered by version.
func Load(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("could not read the migrations directory %s: %w", dir, err)
	}
	var out []Migration
	seen := map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		version, name, ok := ParseFileName(e.Name())
		if !ok {
			continue
		}
		if prev, dup := seen[version]; dup {
			return nil, fmt.Errorf("two migrations share version %s: %s and %s", version, prev, e.Name())
		}
		seen[version] = e.Name()

		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("could not read %s: %w", e.Name(), err)
		}
		sum := sha256.Sum256(body)
		out = append(out, Migration{
			Version:  version,
			Name:     name,
			FileName: e.Name(),
			SQL:      string(body),
			Checksum: hex.EncodeToString(sum[:]),
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no migration files found in %s", dir)
	}
	SortMigrations(out)
	return out, nil
}

// SortMigrations orders by numeric version. Versions are zero-padded to the same width in this
// repo, but comparing by length first keeps ordering correct if a 4-digit version is ever added.
func SortMigrations(ms []Migration) {
	sort.Slice(ms, func(i, j int) bool {
		a, b := ms[i].Version, ms[j].Version
		if len(a) != len(b) {
			return len(a) < len(b)
		}
		return a < b
	})
}

// UpTo returns every migration with a version at or below `version`, in run order. It is the
// selection behind a baseline. An unknown version returns ok=false rather than silently baselining
// a different set than the operator meant.
func UpTo(all []Migration, version string) ([]Migration, bool) {
	known := false
	for _, m := range all {
		if m.Version == version {
			known = true
			break
		}
	}
	if !known {
		return nil, false
	}
	out := []Migration{}
	for _, m := range all {
		if lessOrEqual(m.Version, version) {
			out = append(out, m)
		}
	}
	return out, true
}

// lessOrEqual compares zero-padded numeric versions the same way SortMigrations orders them.
func lessOrEqual(a, b string) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a <= b
}

// Baseline records migrations 001..version as applied WITHOUT running them, for a database that is
// already known to be at that version.
//
// This exists because adopting an old database statement-by-statement has a hard limit: it cannot
// tell "this object already exists" from "a later migration deliberately removed it". Replaying
// migration 005 against a database past 063 tries to recreate `tenant_rsa_keys`, which 063 dropped
// on purpose — no error code distinguishes those two cases. When the operator can *verify* the
// schema is already at a given version, asserting it is both safer and honest.
//
// Rows are marked adopted=true with statements=0, so the ledger never claims to have run them.
func Baseline(ctx context.Context, conn Conn, all []Migration, version string) ([]Migration, error) {
	selected, ok := UpTo(all, version)
	if !ok {
		return nil, fmt.Errorf("no migration has version %q — pass the version of a real file, e.g. 061", version)
	}
	if err := EnsureLedger(ctx, conn); err != nil {
		return nil, err
	}
	for _, m := range selected {
		if _, err := conn.Exec(ctx, `
			INSERT INTO schema_migrations (version, name, checksum, adopted, statements, skipped)
			VALUES ($1,$2,$3,true,0,0)
			ON CONFLICT (version) DO NOTHING`, m.Version, m.Name, m.Checksum); err != nil {
			return nil, fmt.Errorf("recording %s: %w", m.FileName, err)
		}
	}
	return selected, nil
}

// Pending returns the migrations not present in applied, in run order.
func Pending(all []Migration, applied map[string]bool) []Migration {
	out := []Migration{}
	for _, m := range all {
		if !applied[m.Version] {
			out = append(out, m)
		}
	}
	return out
}

// ─── Statement splitting ─────────────────────────────────────────────────────

// SplitStatements breaks a migration into individual SQL statements on top-level semicolons.
//
// Adopt mode needs statement granularity: a migration where the first statement already exists but
// the fifth is new must still apply the fifth. Splitting has to respect the things a semicolon can
// hide inside — string literals, quoted identifiers, line and block comments, and dollar-quoted
// bodies ($$ … $$ / $tag$ … $tag$), which migrations 009 and 062 use for DO blocks.
func SplitStatements(sql string) []string {
	var out []string
	var cur strings.Builder
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			out = append(out, s)
		}
		cur.Reset()
	}

	runes := []rune(sql)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch {
		// -- line comment: copy to end of line.
		case c == '-' && i+1 < len(runes) && runes[i+1] == '-':
			for i < len(runes) && runes[i] != '\n' {
				cur.WriteRune(runes[i])
				i++
			}
			if i < len(runes) {
				cur.WriteRune(runes[i]) // the newline
			}

		// /* block comment */ — nests in Postgres.
		case c == '/' && i+1 < len(runes) && runes[i+1] == '*':
			depth := 0
			for i < len(runes) {
				if runes[i] == '/' && i+1 < len(runes) && runes[i+1] == '*' {
					depth++
					cur.WriteRune(runes[i])
					i++
					cur.WriteRune(runes[i])
				} else if runes[i] == '*' && i+1 < len(runes) && runes[i+1] == '/' {
					depth--
					cur.WriteRune(runes[i])
					i++
					cur.WriteRune(runes[i])
					if depth == 0 {
						break
					}
				} else {
					cur.WriteRune(runes[i])
				}
				i++
			}

		// 'string literal' — '' is an escaped quote, not a terminator.
		case c == '\'':
			cur.WriteRune(c)
			i++
			for i < len(runes) {
				cur.WriteRune(runes[i])
				if runes[i] == '\'' {
					if i+1 < len(runes) && runes[i+1] == '\'' {
						i++
						cur.WriteRune(runes[i])
					} else {
						break
					}
				}
				i++
			}

		// "quoted identifier" — "" is an escaped quote.
		case c == '"':
			cur.WriteRune(c)
			i++
			for i < len(runes) {
				cur.WriteRune(runes[i])
				if runes[i] == '"' {
					if i+1 < len(runes) && runes[i+1] == '"' {
						i++
						cur.WriteRune(runes[i])
					} else {
						break
					}
				}
				i++
			}

		// $tag$ dollar-quoted body $tag$ — everything inside is opaque.
		case c == '$':
			if tag, ok := dollarTagAt(runes, i); ok {
				cur.WriteString(tag)
				i += len([]rune(tag))
				for i < len(runes) {
					if runes[i] == '$' {
						if t2, ok2 := dollarTagAt(runes, i); ok2 && t2 == tag {
							cur.WriteString(tag)
							i += len([]rune(tag)) - 1
							break
						}
					}
					cur.WriteRune(runes[i])
					i++
				}
			} else {
				cur.WriteRune(c)
			}

		case c == ';':
			flush()

		default:
			cur.WriteRune(c)
		}
	}
	flush()
	return out
}

// dollarTagAt reports the dollar-quote tag starting at position i ("$$" or "$name$"). A tag body
// must be a valid identifier, so "$1" (a placeholder) and a bare "$" are correctly rejected.
func dollarTagAt(runes []rune, i int) (string, bool) {
	if i >= len(runes) || runes[i] != '$' {
		return "", false
	}
	j := i + 1
	for j < len(runes) && (runes[j] == '_' ||
		(runes[j] >= 'a' && runes[j] <= 'z') ||
		(runes[j] >= 'A' && runes[j] <= 'Z') ||
		(j > i+1 && runes[j] >= '0' && runes[j] <= '9')) {
		j++
	}
	if j < len(runes) && runes[j] == '$' {
		return string(runes[i : j+1]), true
	}
	return "", false
}

// ─── Error classification ────────────────────────────────────────────────────

// duplicateObjectCodes are the Postgres SQLSTATEs meaning "this already exists". Adopt mode skips
// a statement only for these — every other failure is a real error and stops the run.
//
//	42P07 duplicate_table (covers tables, indexes, views, sequences)
//	42710 duplicate_object (covers policies, types, constraints, roles)
//	42701 duplicate_column
//	42723 duplicate_function
//	42P06 duplicate_schema
//	42P16 invalid_table_definition (e.g. "multiple primary keys")
const (
	sqlStateDuplicateTable    = "42P07"
	sqlStateDuplicateObject   = "42710"
	sqlStateDuplicateColumn   = "42701"
	sqlStateDuplicateFunction = "42723"
	sqlStateDuplicateSchema   = "42P06"
	sqlStateDuplicateAlias    = "42712"
)

// States that a `DROP … IF EXISTS` raises when the object *does* exist but not in the form this
// statement expects — the signature of a migration that has already been applied here.
//
//	42809 wrong_object_type            — e.g. the name is now a table, not a materialized view (007)
//	2BP01 dependent_objects_still_exist — e.g. the index is now owned by a constraint (036)
const (
	sqlStateWrongObjectType = "42809"
	sqlStateDependentObject = "2BP01"
)

// IsDuplicateObject reports whether err is Postgres telling us the object is already there.
func IsDuplicateObject(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case sqlStateDuplicateTable, sqlStateDuplicateObject, sqlStateDuplicateColumn,
		sqlStateDuplicateFunction, sqlStateDuplicateSchema, sqlStateDuplicateAlias:
		return true
	}
	return false
}

// dropIfExistsPattern matches `DROP <kind> IF EXISTS …`, allowing leading comments/whitespace.
var dropIfExistsPattern = regexp.MustCompile(`(?is)^\s*DROP\s+[A-Z ]+\s+IF\s+EXISTS\b`)

// AdoptSkippable reports whether a failed statement means "this migration already happened here",
// and may therefore be stepped over while adopting a hand-migrated database.
//
// Two cases, both narrow and both driven by SQLSTATE:
//
//  1. duplicate-object — the thing this statement creates is already there.
//  2. wrong_object_type / dependent_objects_still_exist on a `DROP … IF EXISTS` — the name exists
//     but not in the form the statement expects, because the migration already ran here. Migration
//     007 turns the `student_attendance_summary` materialized view into a plain table, and 036
//     replaces the `ux_offerings_cohort` index with a deferrable constraint; re-running either
//     `DROP … IF EXISTS` on an up-to-date database raises exactly these.
//
// The second case is deliberately tied to the statement text: these codes on anything other than a
// `DROP … IF EXISTS` are real errors and must still stop the run.
func AdoptSkippable(stmt string, err error) bool {
	if IsDuplicateObject(err) {
		return true
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case sqlStateWrongObjectType, sqlStateDependentObject:
		return dropIfExistsPattern.MatchString(stripLeadingComments(stmt))
	}
	return false
}

// stripLeadingComments removes the comment block a migration statement usually carries so the
// statement's real first keyword can be matched.
func stripLeadingComments(stmt string) string {
	for {
		s := strings.TrimSpace(stmt)
		if strings.HasPrefix(s, "--") {
			if i := strings.IndexByte(s, '\n'); i >= 0 {
				stmt = s[i+1:]
				continue
			}
			return ""
		}
		if strings.HasPrefix(s, "/*") {
			if i := strings.Index(s, "*/"); i >= 0 {
				stmt = s[i+2:]
				continue
			}
			return ""
		}
		return s
	}
}

// ─── Ledger ──────────────────────────────────────────────────────────────────

// LedgerDDL creates the bookkeeping table. Idempotent, and the first thing every run does.
const LedgerDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version     TEXT        PRIMARY KEY,
    name        TEXT        NOT NULL,
    checksum    TEXT        NOT NULL,
    -- true when the migration was ADOPTED rather than run: its objects were already present in a
    -- database that predates this ledger, so some statements were skipped as duplicates.
    adopted     BOOLEAN     NOT NULL DEFAULT false,
    statements  INTEGER     NOT NULL DEFAULT 0,
    skipped     INTEGER     NOT NULL DEFAULT 0,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);`

// sqlStateUndefinedTable (42P01) — the relation does not exist.
const sqlStateUndefinedTable = "42P01"

// EnsureLedger creates the bookkeeping table. Only the commands that WRITE call it, so `status`
// stays genuinely read-only — reporting on a database must never be the thing that modifies it.
func EnsureLedger(ctx context.Context, conn Conn) error {
	if _, err := conn.Exec(ctx, LedgerDDL); err != nil {
		return fmt.Errorf("could not create the schema_migrations ledger: %w", err)
	}
	return nil
}

// Applied reads the ledger. A missing ledger table is not an error — it means a database that has
// never been migrated by this tool, which is exactly the case baseline and adopt mode are for. It
// does NOT create the table: see EnsureLedger.
func Applied(ctx context.Context, conn Conn) (map[string]bool, error) {
	rows, err := conn.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == sqlStateUndefinedTable {
			return map[string]bool{}, nil // never migrated by this tool
		}
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var v string
		if rows.Scan(&v) == nil {
			out[v] = true
		}
	}
	return out, rows.Err()
}

// Conn is the slice of pgx used here — satisfied by *pgxpool.Pool, *pgxpool.Conn and *pgx.Conn.
type Conn interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Result is what became of one migration.
type Result struct {
	Migration  Migration
	Statements int
	Skipped    int  // statements skipped as already-present (adopt mode only)
	Adopted    bool // true when at least one statement was skipped
}

// Options controls a run.
type Options struct {
	// Adopt allows individual statements that fail with a duplicate-object error to be skipped,
	// so a database that was migrated by hand before this ledger existed can be brought under
	// management without being rebuilt. Leave false for a database this tool already owns: then
	// any error at all stops the run.
	Adopt bool
	// DryRun applies nothing and writes nothing; it only reports what would run.
	DryRun bool
	// Log receives one line per migration. Optional.
	Log func(string)
}

func (o Options) logf(format string, args ...any) {
	if o.Log != nil {
		o.Log(fmt.Sprintf(format, args...))
	}
}

// Apply runs one migration in a single transaction and records it in the ledger. Either the whole
// migration lands and is recorded, or nothing of it does — there is no half-applied state.
func Apply(ctx context.Context, conn Conn, m Migration, opts Options) (Result, error) {
	stmts := SplitStatements(m.SQL)
	res := Result{Migration: m, Statements: len(stmts)}
	if opts.DryRun {
		return res, nil
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return res, fmt.Errorf("%s: could not begin: %w", m.FileName, err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	for i, stmt := range stmts {
		// Each statement gets a savepoint so that, in adopt mode, one already-present object can
		// be stepped over without discarding the statements that came before it.
		if _, err := tx.Exec(ctx, "SAVEPOINT stmt"); err != nil {
			return res, fmt.Errorf("%s: savepoint failed: %w", m.FileName, err)
		}
		_, execErr := tx.Exec(ctx, stmt)
		if execErr == nil {
			if _, err := tx.Exec(ctx, "RELEASE SAVEPOINT stmt"); err != nil {
				return res, fmt.Errorf("%s: release savepoint failed: %w", m.FileName, err)
			}
			continue
		}
		if !opts.Adopt || !AdoptSkippable(stmt, execErr) {
			return res, fmt.Errorf("%s: statement %d/%d failed: %w\n--- statement ---\n%s",
				m.FileName, i+1, len(stmts), execErr, truncate(stmt, 500))
		}
		if _, err := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT stmt"); err != nil {
			return res, fmt.Errorf("%s: rollback to savepoint failed: %w", m.FileName, err)
		}
		res.Skipped++
	}
	res.Adopted = res.Skipped > 0

	if _, err := tx.Exec(ctx, `
		INSERT INTO schema_migrations (version, name, checksum, adopted, statements, skipped)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (version) DO UPDATE
		   SET checksum = EXCLUDED.checksum, adopted = EXCLUDED.adopted,
		       statements = EXCLUDED.statements, skipped = EXCLUDED.skipped, applied_at = now()`,
		m.Version, m.Name, m.Checksum, res.Adopted, res.Statements, res.Skipped); err != nil {
		return res, fmt.Errorf("%s: could not record in the ledger: %w", m.FileName, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return res, fmt.Errorf("%s: commit failed: %w", m.FileName, err)
	}
	return res, nil
}

// Up applies every pending migration in order, stopping at the first failure.
func Up(ctx context.Context, conn Conn, dir string, opts Options) ([]Result, error) {
	all, err := Load(dir)
	if err != nil {
		return nil, err
	}
	if err := EnsureLedger(ctx, conn); err != nil {
		return nil, err
	}
	applied, err := Applied(ctx, conn)
	if err != nil {
		return nil, err
	}
	pending := Pending(all, applied)
	if len(pending) == 0 {
		opts.logf("database is up to date — %d migration(s) already applied", len(applied))
		return nil, nil
	}
	opts.logf("%d migration(s) pending of %d on disk", len(pending), len(all))

	var results []Result
	for _, m := range pending {
		res, err := Apply(ctx, conn, m, opts)
		if err != nil {
			return results, err
		}
		results = append(results, res)
		switch {
		case opts.DryRun:
			opts.logf("  would apply %-45s (%d statements)", m.FileName, res.Statements)
		case res.Adopted:
			opts.logf("  adopted %-48s (%d statements, %d already present)", m.FileName, res.Statements, res.Skipped)
		default:
			opts.logf("  applied %-48s (%d statements)", m.FileName, res.Statements)
		}
	}
	return results, nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + " …"
}
