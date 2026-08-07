package handlers

// Bulk import for the organisation chart — schools/colleges and their departments.
//
// ONE FILE FOR BOTH, because that is how an institution already holds it. Nobody keeps a list of
// colleges and a separate list of departments; they keep a chart where each department sits under
// its college, and asking for two uploads in a fixed order invites the second to be loaded against
// half a first.
//
//	school,abbreviation,department,kind
//	School of Mathematics and Computing,SOMAC,Computer Science,ACADEMIC
//	School of Mathematics and Computing,SOMAC,Information Technology,ACADEMIC
//	School of Mathematics and Computing,SOMAC,,                          <- the college alone
//	,,Finance,SUPPORT                                                    <- a department under none
//
// The school column repeats on every row, exactly as a spreadsheet renders a merged cell, and the
// college is created once. A blank department creates only the school; a department with no school
// is a standalone SUPPORT unit (Finance, ICT, Library), which the schema already allows via a NULL
// school_id.
//
// Everything is an UPSERT keyed on the name. Re-uploading a corrected chart has to be safe — the
// realistic use is loading it, spotting three typos, and loading it again — so a second run
// updates the abbreviation and the kind rather than colliding or duplicating.

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// POST /api/v1/admin/tenants/{tenant_id}/org/import   (multipart field "roster")
func ImportOrgChart(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		if err := r.ParseMultipartForm(16 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "expected multipart/form-data"))
			return
		}
		file, _, err := r.FormFile("roster")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "field 'roster' not found"))
			return
		}
		defer file.Close()
		res, perr := processOrgCSV(r.Context(), adminPool, tenantID, file)
		if perr != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("CSV_PARSE_ERROR", perr.Error()))
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func processOrgCSV(ctx context.Context, pool *pgxpool.Pool, tenantID string, src io.Reader) (*importResult, error) {
	data, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("could not read file: %w", err)
	}
	var rows [][]string
	if looksXLSX(data) {
		if rows, err = parseXLSX(data); err != nil {
			return nil, err
		}
	} else {
		cr := csv.NewReader(bytes.NewReader(data))
		cr.TrimLeadingSpace = true
		cr.FieldsPerRecord = -1
		if rows, err = cr.ReadAll(); err != nil {
			return nil, fmt.Errorf("could not parse CSV: %w", err)
		}
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("file has no rows")
	}

	colIdx := make(map[string]int, len(rows[0]))
	for i, h := range rows[0] {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	if _, ok := colIdx["school"]; !ok {
		if _, ok2 := colIdx["department"]; !ok2 {
			return nil, fmt.Errorf("the file needs a 'school' column, a 'department' column, or both")
		}
	}

	res := &importResult{Errors: []string{}}
	// Schools resolved once and reused down the file: the column repeats on every row of a
	// college's block, and looking it up per row would be one query per department for nothing.
	schoolIDs := map[string]string{}

	for ln := 1; ln < len(rows); ln++ {
		row := rows[ln]
		get := func(c string) string {
			i, ok := colIdx[c]
			if !ok || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		schoolName := firstNonEmpty(get("school"), get("college"), get("school/college"), get("school_name"))
		abbrev := firstNonEmpty(get("abbreviation"), get("abbrev"), get("short_form"), get("short"))
		deptName := firstNonEmpty(get("department"), get("department_name"))
		kind := strings.ToUpper(firstNonEmpty(get("kind"), get("type")))
		if kind != "SUPPORT" {
			kind = "ACADEMIC"
		}
		if schoolName == "" && deptName == "" {
			continue // a blank line, not an error
		}

		schoolID := ""
		if schoolName != "" {
			key := strings.ToLower(schoolName)
			if id, seen := schoolIDs[key]; seen {
				schoolID = id
			} else {
				// LOOK, THEN WRITE — rather than insert-and-fall-back-to-update.
				//
				// The first version tried the INSERT and treated any error as "it already exists",
				// which threw the INSERT's error away and reported the fallback's instead. When
				// the INSERT was genuinely broken, every row failed with "no rows in result set"
				// — a message about the UPDATE finding nothing, which said nothing at all about
				// the real fault and sent me looking in the wrong place.
				//
				// Matched on the name OR the abbreviation, so a chart that writes "SOMAC" in one
				// block and the full title in another files both under one college instead of
				// creating two that no report can reconcile.
				var found string
				lookupErr := pool.QueryRow(ctx, `
					SELECT school_id::text FROM schools
					 WHERE tenant_id = $1::uuid
					   AND (btrim(lower(name)) = btrim(lower($2))
					        OR ($3 <> '' AND btrim(lower(COALESCE(abbreviation,''))) = btrim(lower($3))))
					 LIMIT 1`, tenantID, schoolName, abbrev).Scan(&found)
				switch {
				case lookupErr == nil:
					// Fill in an abbreviation the college was created without; never blank one out.
					if abbrev != "" {
						_, _ = pool.Exec(ctx,
							`UPDATE schools SET abbreviation = $2 WHERE school_id = $1::uuid AND COALESCE(abbreviation,'') = ''`,
							found, abbrev)
					}
					schoolID = found
					res.Updated++
				default:
					if e := pool.QueryRow(ctx, `
						INSERT INTO schools (tenant_id, name, abbreviation)
						VALUES ($1::uuid, $2, NULLIF($3,''))
						RETURNING school_id::text`, tenantID, schoolName, abbrev).Scan(&schoolID); e != nil {
						res.Skipped++
						res.Errors = append(res.Errors, fmt.Sprintf("line %d: college %q: %s", ln, schoolName, e.Error()))
						continue
					}
					res.Inserted++
				}
				schoolIDs[key] = schoolID
			}
		}

		if deptName == "" {
			continue // the college on its own — already handled above
		}
		// A department name is unique WITHIN its college, not across the institution: two colleges
		// may each run a "Computer Science", and matching on the name alone would merge them.
		// A department name is unique WITHIN its college, not across the institution: two colleges
		// may each run a "Computer Science", and matching on the name alone would merge them.
		var deptID string
		lookupErr := pool.QueryRow(ctx, `
			SELECT department_id::text FROM departments
			 WHERE tenant_id = $1::uuid AND btrim(lower(name)) = btrim(lower($3))
			   AND COALESCE(school_id::text,'') = COALESCE(NULLIF($2,''),'')
			 LIMIT 1`, tenantID, schoolID, deptName).Scan(&deptID)
		if lookupErr == nil {
			if _, e := pool.Exec(ctx,
				`UPDATE departments SET kind = $2 WHERE department_id = $1::uuid`, deptID, kind); e != nil {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf("line %d: department %q: %s", ln, deptName, e.Error()))
				continue
			}
			res.Updated++
			continue
		}
		if e := pool.QueryRow(ctx, `
			INSERT INTO departments (tenant_id, school_id, name, kind)
			VALUES ($1::uuid, NULLIF($2,'')::uuid, $3, $4)
			RETURNING department_id::text`, tenantID, schoolID, deptName, kind).Scan(&deptID); e != nil {
			res.Skipped++
			res.Errors = append(res.Errors, fmt.Sprintf("line %d: department %q: %s", ln, deptName, e.Error()))
			continue
		}
		res.Inserted++
	}
	return res, nil
}
