package handlers

// One school, two names — and both have to match.
//
// A college has a formal title ("School of Mathematics and Computing") and the short form everyone
// actually uses ("SOMAC"). Migration 072 stores both. This file is the single place that knows they
// are the same thing.
//
// WHY IT MATTERS MORE THAN A LABEL. `users.school` and `courses.school` are free TEXT, and every
// org-scoped query is a string comparison between them: a dean sees their college because
// `courses.school` equals the value on their account. Real institutions have been entering the
// abbreviation as the name (this deployment's only school is literally called "SOMAC"), so historic
// rows hold the short form. The instant an admin fills in the full title, a matcher that knew only
// `schools.name` would compare the long form against rows holding the short one and every scoped
// dashboard would empty out — which reads as "no data" rather than "mismatch", and is therefore the
// hardest kind of failure to diagnose.
//
// So a school reference resolves to the SET of strings that mean it, and matching accepts any.

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// schoolAliases returns every string that identifies the same school as `ref` — its full name and
// its abbreviation — matched case- and whitespace-insensitively against either.
//
// `ref` itself is always included, even when no school record matches it: a department or account
// may name a college that was never added to the org tree, and that value must still scope to
// exactly what it says rather than silently widening to nothing.
func schoolAliases(ctx context.Context, pool *pgxpool.Pool, tenantID, ref string) []string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}
	out := []string{ref}
	seen := map[string]bool{strings.ToLower(ref): true}

	rows, err := pool.Query(ctx, `
		SELECT name, COALESCE(abbreviation,'')
		FROM schools
		WHERE tenant_id = $1
		  AND (btrim(lower(name)) = btrim(lower($2))
		    OR btrim(lower(COALESCE(abbreviation,''))) = btrim(lower($2)))`,
		tenantID, ref)
	if err != nil {
		// The scope still works on the literal value; it just does not gain the alias.
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var name, abbr string
		if rows.Scan(&name, &abbr) != nil {
			continue
		}
		for _, v := range []string{name, abbr} {
			v = strings.TrimSpace(v)
			if v == "" || seen[strings.ToLower(v)] {
				continue
			}
			seen[strings.ToLower(v)] = true
			out = append(out, v)
		}
	}
	return out
}

// normaliseAliases lower-cases and trims a set for comparison against
// `btrim(lower(column)) = ANY(...)`, which is how the alias list is used in SQL.
func normaliseAliases(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v = strings.ToLower(strings.TrimSpace(v)); v != "" {
			out = append(out, v)
		}
	}
	return out
}
