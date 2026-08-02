package migrate

// Database-backed tests. Like the rest of the repo's integration tests these SKIP unless a
// connection is supplied, so `go test ./...` stays runnable with no services up.
//
//	MIGRATE_TEST_DB_URL='postgres://qaat:pw@localhost:5434/postgres?sslmode=disable' go test ./internal/migrate/ -v
//
// The URL must point at a database the test may CREATE and DROP databases from (it connects to the
// given database only to create scratch ones); nothing existing is touched.

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func adminURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("MIGRATE_TEST_DB_URL")
	if url == "" {
		t.Skip("set MIGRATE_TEST_DB_URL to run the database-backed migration tests")
	}
	return url
}

// scratchDB creates a throwaway database and returns a pool onto it.
func scratchDB(t *testing.T, name string) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, adminURL(t))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer admin.Close()

	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name); err != nil {
		t.Fatalf("drop %s: %v", name, err)
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	t.Cleanup(func() {
		a, err := pgxpool.New(context.Background(), adminURL(t))
		if err != nil {
			return
		}
		defer a.Close()
		_, _ = a.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name)
	})

	pool, err := pgxpool.New(ctx, swapDBName(adminURL(t), name))
	if err != nil {
		t.Fatalf("connect to %s: %v", name, err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// swapDBName replaces the database path component of a Postgres URL.
func swapDBName(url, name string) string {
	q := ""
	if i := strings.Index(url, "?"); i >= 0 {
		q = url[i:]
		url = url[:i]
	}
	if i := strings.LastIndex(url, "/"); i >= 0 {
		url = url[:i]
	}
	return url + "/" + name + q
}

// TestUpOnEmptyDatabase is the everyday path: a fresh database takes every migration, in order,
// and a second run is a no-op. The no-op half is what makes the runner safe to invoke on deploy.
func TestUpOnEmptyDatabase(t *testing.T) {
	pool := scratchDB(t, "qaat_migtest_empty")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	dir := repoMigrationsDir(t)

	all, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	results, err := Up(ctx, pool, dir, Options{})
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	if len(results) != len(all) {
		t.Fatalf("applied %d of %d migrations", len(results), len(all))
	}
	for _, r := range results {
		if r.Skipped != 0 {
			t.Errorf("%s skipped %d statement(s) on a fresh database — nothing should already exist",
				r.Migration.FileName, r.Skipped)
		}
	}

	// Re-running must do nothing at all. If this regresses, a redeploy would start reapplying
	// migrations over live data.
	again, err := Up(ctx, pool, dir, Options{})
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	if len(again) != 0 {
		t.Errorf("second run applied %d migration(s); want 0", len(again))
	}

	// The features that were missing in production must now be present.
	assertPresent(t, ctx, pool, "schools", "departments", "qa_rep_submissions",
		"lecturer_patrol_logs", "qa_messages", "employees", "lecturer_daily_codes")
}

// TestAdoptRaggedDatabase reproduces the production fault this tool exists for: a database that was
// migrated by hand, so some migrations are applied and some are not, with no record of which.
// Adopt mode must bring it fully up to date without a rebuild — and must not leave anything out.
func TestAdoptRaggedDatabase(t *testing.T) {
	dir := repoMigrationsDir(t)
	all, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Build the ragged database the way the real one got that way: apply an early run of
	// migrations directly, with no ledger, deliberately skipping one in the middle.
	ragged := scratchDB(t, "qaat_migtest_ragged")
	const skipMid = "050" // the migration production actually missed
	applied := 0
	for _, m := range all {
		if m.Version > "056" || m.Version == skipMid {
			continue
		}
		for _, stmt := range SplitStatements(m.SQL) {
			if _, err := ragged.Exec(ctx, stmt); err != nil {
				t.Fatalf("seeding %s: %v", m.FileName, err)
			}
		}
		applied++
	}
	t.Logf("seeded %d migrations by hand (no ledger), skipping %s", applied, skipMid)

	// Plain `up` must REFUSE this database rather than guess — re-running 001 would fail.
	if _, err := Up(ctx, ragged, dir, Options{}); err == nil {
		t.Fatal("plain 'up' succeeded on a hand-migrated database; it must fail rather than guess")
	}

	// Adopt mode brings it under management.
	results, err := Up(ctx, ragged, dir, Options{Adopt: true})
	if err != nil {
		t.Fatalf("adopt failed: %v", err)
	}
	if len(results) != len(all) {
		t.Fatalf("adopt recorded %d of %d migrations", len(results), len(all))
	}

	// The migration that was skipped by hand must have been genuinely APPLIED, not adopted —
	// this is the case that silently loses a feature if statement-level skipping is too eager.
	for _, r := range results {
		if r.Migration.Version == skipMid && r.Skipped != 0 {
			t.Errorf("%s was skipped as already-present, but it never ran here", r.Migration.FileName)
		}
	}
	assertPresent(t, ctx, ragged, "student_device_bindings", // 050 — the one that was missed
		"schools", "departments", "qa_rep_submissions", "lecturer_patrol_logs")

	// And a second adopt run is a no-op, like any other.
	if again, err := Up(ctx, ragged, dir, Options{Adopt: true}); err != nil || len(again) != 0 {
		t.Errorf("second adopt run applied %d migration(s) (err=%v); want 0", len(again), err)
	}

	// The decisive check: an adopted database must end up with every object a from-scratch one
	// has. Extra objects are acceptable (old indexes can survive); missing ones are not.
	fresh := scratchDB(t, "qaat_migtest_fresh")
	if _, err := Up(ctx, fresh, dir, Options{}); err != nil {
		t.Fatalf("from-scratch run failed: %v", err)
	}
	freshObjs := schemaObjects(t, ctx, fresh)
	adoptedObjs := schemaObjects(t, ctx, ragged)
	var missing []string
	for o := range freshObjs {
		if !adoptedObjs[o] {
			missing = append(missing, o)
		}
	}
	if len(missing) > 0 {
		t.Errorf("the adopted database is missing %d object(s) a fresh one has:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
	t.Logf("adopted schema covers all %d objects of a from-scratch build", len(freshObjs))
}

// schemaObjects fingerprints a database as a set of column, index, policy and enum-value names.
func schemaObjects(t *testing.T, ctx context.Context, pool *pgxpool.Pool) map[string]bool {
	t.Helper()
	const q = `
		SELECT 'col:'||table_name||'.'||column_name FROM information_schema.columns
		 WHERE table_schema='public' AND table_name <> 'schema_migrations'
		UNION ALL SELECT 'idx:'||indexname FROM pg_indexes
		 WHERE schemaname='public' AND tablename <> 'schema_migrations'
		UNION ALL SELECT 'pol:'||tablename||'.'||policyname FROM pg_policies WHERE schemaname='public'
		UNION ALL SELECT 'enum:'||t.typname||'.'||e.enumlabel
		 FROM pg_enum e JOIN pg_type t ON t.oid = e.enumtypid`
	rows, err := pool.Query(ctx, q)
	if err != nil {
		t.Fatalf("fingerprint: %v", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var s string
		if rows.Scan(&s) == nil {
			out[s] = true
		}
	}
	return out
}

func assertPresent(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tables ...string) {
	t.Helper()
	for _, tbl := range tables {
		var n int
		if err := pool.QueryRow(ctx,
			`SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name=$1`,
			tbl).Scan(&n); err != nil {
			t.Fatalf("checking %s: %v", tbl, err)
		}
		if n == 0 {
			t.Errorf("table %s is missing after migrating — %s", tbl, whyItMatters(tbl))
		}
	}
}

func whyItMatters(table string) string {
	switch table {
	case "schools", "departments":
		return "the Schools & Departments admin page cannot load without it"
	case "qa_rep_submissions":
		return "QA reps cannot file reports without it"
	case "lecturer_patrol_logs":
		return "the patroller app has nowhere to sync to"
	case "student_device_bindings":
		return "this is migration 050, the one production silently missed"
	}
	return fmt.Sprintf("%s is created by a migration that did not run", table)
}
