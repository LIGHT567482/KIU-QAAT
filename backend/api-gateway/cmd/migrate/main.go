// Command migrate applies db/migrations to a database, once each, in order.
//
//	go run ./cmd/migrate status                 # what is applied, what is pending
//	go run ./cmd/migrate up                     # apply everything pending
//	go run ./cmd/migrate up --adopt             # first run against a hand-migrated database
//	go run ./cmd/migrate baseline 061           # record 001..061 as applied WITHOUT running them
//	go run ./cmd/migrate up --dry-run           # show what would run, change nothing
//
// The connection comes from -db, then $ADMIN_DB_URL, then $DB_URL. It must be the OWNER/superuser
// connection (ADMIN_DB_URL), not the RLS-confined qaat_app role, since migrations create tables,
// policies and roles.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/migrate"
)

func main() {
	var (
		dbURL  = flag.String("db", "", "Postgres URL (default: $ADMIN_DB_URL, then $DB_URL)")
		dir    = flag.String("dir", "", "migrations directory (default: $MIGRATIONS_DIR, then ./db/migrations found by walking up)")
		at     = flag.String("at", "", "baseline only: the version the database is already at, e.g. 061")
		adopt  = flag.Bool("adopt", false, "skip statements whose object already exists — for the FIRST run against a database migrated by hand")
		dryRun = flag.Bool("dry-run", false, "report what would run; change nothing")
	)
	// Go's flag package stops parsing at the first positional argument, so `up --adopt` would
	// leave --adopt unparsed and silently ignored — which on a hand-migrated database is the
	// difference between adopting it and failing. Parse once to pick up any flags before the
	// subcommand, then parse again over whatever followed it, so either order works.
	flag.Parse()
	cmd := "status"
	if flag.NArg() > 0 {
		cmd = flag.Arg(0)
		if err := flag.CommandLine.Parse(flag.Args()[1:]); err != nil {
			fail("%v", err)
		}
	}
	switch cmd {
	case "status", "up", "baseline":
	default:
		fail("unknown command %q — expected 'status', 'up' or 'baseline'", cmd)
	}

	url := firstNonEmpty(*dbURL, os.Getenv("ADMIN_DB_URL"), os.Getenv("DB_URL"))
	if url == "" {
		fail("no database URL — pass -db, or set ADMIN_DB_URL / DB_URL")
	}
	mdir, err := resolveDir(*dir)
	if err != nil {
		fail("%v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		fail("could not connect: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		fail("could not reach the database: %v", err)
	}

	all, err := migrate.Load(mdir)
	if err != nil {
		fail("%v", err)
	}
	applied, err := migrate.Applied(ctx, pool)
	if err != nil {
		fail("%v", err)
	}
	pending := migrate.Pending(all, applied)

	fmt.Printf("migrations dir : %s\n", mdir)
	fmt.Printf("on disk        : %d\n", len(all))
	fmt.Printf("already applied: %d\n", len(applied))
	fmt.Printf("pending        : %d\n", len(pending))

	if cmd == "baseline" {
		// The version may be given as `baseline 061` or `-at 061`.
		version := *at
		if version == "" && flag.NArg() > 0 {
			version = flag.Arg(0)
		}
		if version == "" {
			fail("baseline needs the version the database is already at, e.g. 'baseline 061'")
		}
		if len(applied) > 0 {
			fail("this database already has a ledger (%d migration(s) recorded) — baseline is only for a database that has never been under management", len(applied))
		}
		fmt.Println()
		fmt.Println("BASELINE records migrations as applied WITHOUT running them.")
		fmt.Println("Only do this when you have verified the schema really is at that version;")
		fmt.Println("anything wrongly recorded here will never be applied.")
		fmt.Println()
		recorded, err := migrate.Baseline(ctx, pool, all, version)
		if err != nil {
			fail("%v", err)
		}
		fmt.Printf("recorded %d migration(s) as already applied, up to and including %s\n", len(recorded), version)
		remaining := migrate.Pending(all, mapVersions(recorded))
		fmt.Printf("still pending: %d\n", len(remaining))
		for _, m := range remaining {
			fmt.Printf("  PENDING  %s\n", m.FileName)
		}
		fmt.Println("\nRun 'migrate up' to apply them.")
		return
	}

	if cmd == "status" {
		for _, m := range pending {
			fmt.Printf("  PENDING  %s\n", m.FileName)
		}
		if len(pending) == 0 {
			fmt.Println("\nDatabase is up to date.")
		} else {
			fmt.Printf("\nRun 'migrate up' to apply them.\n")
			if len(applied) == 0 {
				fmt.Println("The ledger is empty. If this database was previously migrated by hand,")
				fmt.Println("use 'migrate up --adopt' so already-present objects are stepped over.")
			}
		}
		return
	}

	if len(pending) == 0 {
		fmt.Println("\nNothing to do.")
		return
	}
	fmt.Println()

	results, err := migrate.Up(ctx, pool, mdir, migrate.Options{
		Adopt:  *adopt,
		DryRun: *dryRun,
		Log:    func(s string) { fmt.Println(s) },
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nFAILED after %d migration(s): %v\n", len(results), err)
		fmt.Fprintln(os.Stderr, "\nNothing from the failing migration was applied — it ran in a transaction.")
		fmt.Fprintln(os.Stderr, "Fix the cause and run again; the migrations that already succeeded are recorded and will be skipped.")
		os.Exit(1)
	}

	var adopted, skipped int
	for _, r := range results {
		if r.Adopted {
			adopted++
		}
		skipped += r.Skipped
	}
	if *dryRun {
		fmt.Printf("\nDry run: %d migration(s) would be applied. Nothing was changed.\n", len(results))
		return
	}
	fmt.Printf("\nDone: %d migration(s) applied", len(results))
	if adopted > 0 {
		fmt.Printf(" (%d adopted, %d statement(s) already present)", adopted, skipped)
	}
	fmt.Println(".")
}

// resolveDir finds db/migrations: the flag, then $MIGRATIONS_DIR, then by walking up from the
// working directory so the command works from the repo root or from backend/api-gateway.
func resolveDir(flagDir string) (string, error) {
	if d := firstNonEmpty(flagDir, os.Getenv("MIGRATIONS_DIR")); d != "" {
		if st, err := os.Stat(d); err != nil || !st.IsDir() {
			return "", fmt.Errorf("migrations directory %s is not readable", d)
		}
		return d, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 6; i++ {
		c := filepath.Join(wd, "db", "migrations")
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c, nil
		}
		wd = filepath.Dir(wd)
	}
	return "", fmt.Errorf("could not find db/migrations — pass -dir or set MIGRATIONS_DIR")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "migrate: "+format+"\n", args...)
	os.Exit(1)
}

// mapVersions turns a migration slice into the applied-set shape Pending expects.
func mapVersions(ms []migrate.Migration) map[string]bool {
	out := map[string]bool{}
	for _, m := range ms {
		out[m.Version] = true
	}
	return out
}
