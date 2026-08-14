package handlers

// BULK IMPORT AND EXPORT FOR THE ADMINISTRATION PAGE.
//
// Every other directory in this dashboard could already be moved in and out as a spreadsheet —
// rooms, lecturers, coordinators, students, employees, courses, units, assignments. The one that
// could not was the page that creates the accounts for everybody who OVERSEES those things: deans,
// heads of department, the QA offices, the TLC. An institution standing up QAAT for the first time
// has eighty of them to enter, and the only way in was a form, one person at a time.
//
// THE ACCOUNTS THIS CREATES CAN SIGN IN, which is what makes this different from importing a list
// of rooms and why the rules below are enforced rather than assumed:
//
//   - Only the roles the Administration page actually LISTS may be imported. A file naming a
//     LECTURER or a STUDENT is refused, and told where those belong. Creating an account here that
//     the page cannot then show would be the same "already exists, but it isn't there" trap that
//     CreateUser's error text exists to explain.
//   - The institution's email domain is enforced exactly as the form enforces it.
//   - The department/school scope requirement is enforced exactly as the form enforces it: an HOD
//     with no department sees an empty dashboard, and a bulk path that skipped the check would be
//     the way around a rule the form makes unavoidable.
//   - Passwords are NEVER read from the file. Imported accounts start on a seeded first-login word
//     with force_password_change set — see ImportedPasswordFor.
//   - Re-importing an existing user updates their details and NEVER touches their password. A
//     second import to fix a misspelt name must not sign eighty people out of their own accounts.

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// userImportCols is the column set, and doubles as the exported sheet's header — so a file that is
// exported, edited and imported back round-trips without the administrator having to reconcile two
// different column lists. Deliberately no password column; see the file comment.
var userImportCols = []string{"email", "title", "full_name", "role", "department", "school", "staff_id", "phone"}

// POST /api/v1/admin/tenants/{tenant_id}/users/import  (multipart field "roster")
func ImportUsers(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenantOf(r)

		file, ok := uploadFile(w, r)
		if !ok {
			return
		}
		defer file.Close()

		rows, idx, err := readTabular(file)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("CSV_PARSE_ERROR", err.Error()))
			return
		}
		for _, required := range []string{"email", "full_name", "role"} {
			if _, ok := idx[required]; !ok {
				writeJSON(w, http.StatusUnprocessableEntity, errBody("CSV_PARSE_ERROR",
					"missing required column: "+required))
				return
			}
		}

		// The institution's domain, resolved once. Same rule as the form: an address outside it
		// cannot sign in, so importing one would create an account nobody can use.
		var domain string
		if err := adminPool.QueryRow(r.Context(),
			`SELECT domain FROM tenants WHERE tenant_id = $1`, tenantID).Scan(&domain); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("TENANT_NOT_FOUND", "tenant not found"))
			return
		}
		domain = strings.ToLower(strings.TrimSpace(domain))

		// Hash each distinct seeded word ONCE. bcrypt at cost 12 is ~250ms by design, and a file of
		// eighty users would otherwise spend twenty seconds doing the same two hashes over and over
		// — long enough for the upload to look hung and be retried, which is how duplicates start.
		hashes := map[string]string{}
		hashFor := func(word string) (string, error) {
			if h, ok := hashes[word]; ok {
				return h, nil
			}
			h, err := bcrypt.GenerateFromPassword([]byte(word), 12)
			if err != nil {
				return "", err
			}
			hashes[word] = string(h)
			return string(h), nil
		}

		res := &importResult{Errors: []string{}}
		// Which seeded words this file actually used, so the response can tell the administrator
		// what to tell people rather than making them look it up.
		usedWords := map[string]bool{}

		for ln := 1; ln < len(rows); ln++ {
			row := rows[ln]
			email := strings.ToLower(cell(row, idx, "email"))
			fullName := cell(row, idx, "full_name")
			role := strings.ToUpper(strings.TrimSpace(cell(row, idx, "role")))
			if email == "" && fullName == "" && role == "" {
				continue // a blank line in the middle of a sheet is not an error
			}
			if email == "" || fullName == "" || role == "" {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf("line %d: email, full_name and role are required", ln))
				continue
			}
			if !MANAGED_ROLES[role] {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf(
					"line %d: %q is not a role this page manages. Lecturers, coordinators and students "+
						"have their own pages — import them there", ln, role))
				continue
			}
			if !emailInDomain(email, domain) {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf(
					"line %d: %s must use the institution domain @%s", ln, email, domain))
				continue
			}

			dept := cell(row, idx, "department")
			school := cell(row, idx, "school")
			// The same scope rule the form enforces, for the same reason: these roles' dashboards
			// ARE the department or school on the account, so one without it is an account that
			// opens onto nothing.
			switch role {
			case "QA_OFFICER", "HOD", "QA_DEPT_REP":
				if dept == "" {
					res.Skipped++
					res.Errors = append(res.Errors, fmt.Sprintf(
						"line %d: a department is required for %s — it is the scope of everything they see",
						ln, humanRole(role)))
					continue
				}
			case "DEAN", "QA_SCHOOL_HANDLER":
				if school == "" {
					res.Skipped++
					res.Errors = append(res.Errors, fmt.Sprintf(
						"line %d: a college/school is required for %s — it is the scope of everything they see",
						ln, humanRole(role)))
					continue
				}
			}

			word := ImportedPasswordFor(role)
			hash, herr := hashFor(word)
			if herr != nil {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf("line %d: password hashing failed", ln))
				continue
			}

			var inserted bool
			// The password columns appear ONLY in the INSERT half. On conflict the details are
			// refreshed and password_hash and force_password_change are left exactly as they are —
			// so re-importing a corrected sheet cannot reset a password somebody has already
			// chosen, or re-flag an account that has already been through first login.
			err := adminPool.QueryRow(r.Context(), `
				INSERT INTO users (tenant_id, email, password_hash, role, full_name, is_active,
				                   title, department, school, staff_id, phone, force_password_change)
				VALUES ($1, $2, $3, $4, $5, true,
				        NULLIF($6,''), NULLIF($7,''), NULLIF($8,''), NULLIF($9,''), NULLIF($10,''), true)
				ON CONFLICT (tenant_id, email) DO UPDATE SET
				    role       = EXCLUDED.role,
				    full_name  = EXCLUDED.full_name,
				    title      = COALESCE(NULLIF(EXCLUDED.title,''), users.title),
				    department = COALESCE(NULLIF(EXCLUDED.department,''), users.department),
				    school     = COALESCE(NULLIF(EXCLUDED.school,''), users.school),
				    staff_id   = COALESCE(NULLIF(EXCLUDED.staff_id,''), users.staff_id),
				    phone      = COALESCE(NULLIF(EXCLUDED.phone,''), users.phone)
				RETURNING (xmax = 0)`,
				tenantID, email, hash, role, fullName,
				cell(row, idx, "title"), dept, school, cell(row, idx, "staff_id"), cell(row, idx, "phone"),
			).Scan(&inserted)
			if err != nil {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf("line %d: %s", ln, err.Error()))
				continue
			}
			if inserted {
				res.Inserted++
				usedWords[word] = true
			} else {
				res.Updated++
			}
		}

		// What to tell the people whose accounts were just created. Only stated when accounts were
		// actually created — an import that only updated existing rows changed nobody's password,
		// and saying otherwise would send an administrator to hand out a word that does not work.
		out := map[string]interface{}{
			"inserted": res.Inserted, "updated": res.Updated,
			"skipped": res.Skipped, "errors": res.Errors,
		}
		if len(usedWords) > 0 {
			words := make([]string, 0, len(usedWords))
			for word := range usedWords {
				words = append(words, word)
			}
			out["first_login_passwords"] = words
			out["password_notice"] = "New accounts start on the first-login password \"" +
				strings.Join(words, "\" or \"") + "\" and must change it before they can use the system. " +
				"That word is public, so hand these accounts out now rather than later."
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// GET /api/v1/admin/tenants/{tenant_id}/users/export.xlsx
//
// Exports the same columns the importer reads, so the sheet round-trips: export, correct a batch of
// departments in Excel, import back. Password fields are absent in both directions.
//
// Scoped to MANAGED_ROLES — the roles the Administration page lists. Exporting the coordinators,
// lecturers and students who also live in this table would produce a file that cannot be imported
// back through this endpoint, and would quietly hand out a staff directory from a page that does
// not claim to show one.
func ExportUsersXLSX(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := tenantOf(r)

		managed := make([]string, 0, len(MANAGED_ROLES))
		for role := range MANAGED_ROLES {
			managed = append(managed, role)
		}

		rows, err := adminPool.Query(r.Context(), `
			SELECT email, COALESCE(title,''), full_name, role::text,
			       COALESCE(department,''), COALESCE(school,''), COALESCE(staff_id,''), COALESCE(phone,'')
			FROM users
			WHERE tenant_id = $1 AND role::text = ANY($2)
			ORDER BY role, full_name`, tenantID, managed)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		out := [][]string{userImportCols}
		for rows.Next() {
			var email, title, name, role, dept, school, staffID, phone string
			rows.Scan(&email, &title, &name, &role, &dept, &school, &staffID, &phone) //nolint:errcheck
			out = append(out, []string{email, title, name, role, dept, school, staffID, phone})
		}
		xlsx, err := buildXLSX(out)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="users.xlsx"`)
		_, _ = w.Write(xlsx)
	}
}
