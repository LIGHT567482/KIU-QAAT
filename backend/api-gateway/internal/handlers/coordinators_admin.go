package handlers

// Coordinators directory (next.txt batch): a dedicated admin view of every
// coordinator with their unique ID, registration number, contacts (phone +
// whatsapp), and the course/level/session they coordinate — with Excel import +
// export. Coordinators are `users` rows (role COORDINATOR); their course/level/
// session come from the offering they own (course_offerings → courses).

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type coordinatorRow struct {
	UserID             string `json:"user_id"`
	RegistrationNumber string `json:"registration_number"`
	CoordinatorCode    string `json:"coordinator_code"`
	Title              string `json:"title"`
	FullName           string `json:"full_name"`
	Gender             string `json:"gender"`
	Email              string `json:"email"`
	Phone              string `json:"phone"`
	Whatsapp           string `json:"whatsapp"`
	Course             string `json:"course"`
	Level              string `json:"level"`
	Session            string `json:"session"`
	StudyYear          int    `json:"study_year"` // the cohort year/semester they coordinate
	Semester           int    `json:"semester"`
	Intake             string `json:"intake"`
	IsActive           bool   `json:"is_active"`
}

func queryCoordinators(ctx context.Context, pool *pgxpool.Pool, tenantID string) ([]coordinatorRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT u.user_id::text, COALESCE(u.registration_number,''), COALESCE(u.coordinator_code,''),
		       COALESCE(u.title,''), u.full_name, COALESCE(u.gender,''),
		       u.email, COALESCE(u.phone,''), COALESCE(u.whatsapp,''),
		       COALESCE(c.name, ''), COALESCE(o.level,''), COALESCE(o.session_type,''),
		       COALESCE(o.study_year, 0), COALESCE(o.semester, 0), COALESCE(o.intake,''),
		       u.is_active
		FROM users u
		LEFT JOIN course_offerings o ON o.coordinator_id = u.user_id::text AND o.tenant_id = u.tenant_id
		LEFT JOIN courses c ON c.course_id = o.course_id AND c.tenant_id = u.tenant_id
		WHERE u.tenant_id = $1 AND u.role = 'COORDINATOR'
		ORDER BY u.full_name`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []coordinatorRow{}
	for rows.Next() {
		var c coordinatorRow
		rows.Scan(&c.UserID, &c.RegistrationNumber, &c.CoordinatorCode, &c.Title, &c.FullName, &c.Gender, &c.Email, //nolint:errcheck
			&c.Phone, &c.Whatsapp, &c.Course, &c.Level, &c.Session, &c.StudyYear, &c.Semester, &c.Intake, &c.IsActive)
		out = append(out, c)
	}
	return out, nil
}

// yearSemLabel renders "Y1 · S2" (or "—" when the coordinator has no cohort yet).
func yearSemLabel(c coordinatorRow) string {
	if c.StudyYear == 0 && c.Semester == 0 {
		return ""
	}
	return fmt.Sprintf("Y%d · S%d", c.StudyYear, c.Semester)
}

// GET /api/v1/admin/tenants/{tenant_id}/coordinators
func ListCoordinators(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		list, err := queryCoordinators(r.Context(), adminPool, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, list)
	}
}

var coordinatorExportCols = []string{
	"registration_number", "coordinator_code", "title", "full_name", "gender", "email",
	"phone", "whatsapp", "course", "level", "session", "year_sem",
}

// GET /api/v1/admin/tenants/{tenant_id}/coordinators/export.xlsx
func ExportCoordinatorsXLSX(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		list, err := queryCoordinators(r.Context(), adminPool, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		out := [][]string{coordinatorExportCols}
		for _, c := range list {
			out = append(out, []string{c.RegistrationNumber, c.CoordinatorCode, c.Title, c.FullName, c.Gender, c.Email,
				c.Phone, c.Whatsapp, c.Course, c.Level, c.Session, yearSemLabel(c)})
		}
		xlsx, err := buildXLSX(out)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="coordinators.xlsx"`)
		_, _ = w.Write(xlsx)
	}
}

// POST /api/v1/admin/tenants/{tenant_id}/coordinators/import — multipart "roster"
// (CSV or XLSX). Upserts coordinators by email: existing rows get contact updates;
// unknown emails create a COORDINATOR account with a random temp password (returned
// so the admin can hand it out). Columns: full_name, email[, phone, whatsapp,
// registration_number].
func ImportCoordinators(pool *pgxpool.Pool) http.HandlerFunc {
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
		data := new(bytes.Buffer)
		_, _ = data.ReadFrom(file)

		var rows [][]string
		if looksXLSX(data.Bytes()) {
			rows, err = parseXLSX(data.Bytes())
		} else {
			cr := csv.NewReader(bytes.NewReader(data.Bytes()))
			cr.TrimLeadingSpace = true
			cr.FieldsPerRecord = -1
			rows, err = cr.ReadAll()
		}
		if err != nil || len(rows) == 0 {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("PARSE_ERROR", "could not read the file"))
			return
		}

		col := map[string]int{}
		for i, h := range rows[0] {
			col[strings.ToLower(strings.TrimSpace(h))] = i
		}
		if _, ok := col["email"]; !ok {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("PARSE_ERROR", "missing required column: email"))
			return
		}
		if _, ok := col["full_name"]; !ok {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("PARSE_ERROR", "missing required column: full_name"))
			return
		}

		var domain string
		if err := pool.QueryRow(r.Context(),
			`SELECT domain FROM tenants WHERE tenant_id = $1`, tenantID).Scan(&domain); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("TENANT_NOT_FOUND", "tenant not found"))
			return
		}
		domain = strings.ToLower(strings.TrimSpace(domain))

		type created struct {
			Email        string `json:"email"`
			TempPassword string `json:"temp_password"`
		}
		res := struct {
			Created  int       `json:"created"`
			Updated  int       `json:"updated"`
			Skipped  int       `json:"skipped"`
			Errors   []string  `json:"errors"`
			NewLogin []created `json:"new_logins"`
		}{Errors: []string{}, NewLogin: []created{}}

		get := func(row []string, name string) string {
			if i, ok := col[name]; ok && i < len(row) {
				return strings.TrimSpace(row[i])
			}
			return ""
		}

		for ln := 1; ln < len(rows); ln++ {
			row := rows[ln]
			email := strings.ToLower(get(row, "email"))
			name := get(row, "full_name")
			if email == "" || name == "" {
				res.Skipped++
				continue
			}
			if !emailInDomain(email, domain) {
				res.Errors = append(res.Errors, fmt.Sprintf("row %d: %s not in @%s", ln+1, email, domain))
				res.Skipped++
				continue
			}
			phone := get(row, "phone")
			whatsapp := get(row, "whatsapp")
			reg := get(row, "registration_number")

			// Update if the coordinator already exists (by email).
			tag, uerr := pool.Exec(r.Context(), `
				UPDATE users SET full_name = $3,
				    phone = COALESCE(NULLIF($4,''), phone),
				    whatsapp = COALESCE(NULLIF($5,''), whatsapp),
				    registration_number = COALESCE(NULLIF($6,''), registration_number),
				    updated_at = now()
				WHERE tenant_id = $1 AND email = $2 AND role = 'COORDINATOR'`,
				tenantID, email, name, phone, whatsapp, reg)
			if uerr == nil && tag.RowsAffected() > 0 {
				res.Updated++
				continue
			}

			// Otherwise create a new coordinator with a temp password + code.
			temp := genCoordinatorCode() + "-" + genCoordinatorCode()[3:] // ~ random
			hash, herr := bcrypt.GenerateFromPassword([]byte(temp), 12)
			if herr != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("row %d: hash failed", ln+1))
				res.Skipped++
				continue
			}
			code := genCoordinatorCode()
			_, ierr := pool.Exec(r.Context(), `
				INSERT INTO users (tenant_id, email, password_hash, role, full_name, is_active,
				                   coordinator_code, phone, whatsapp, registration_number)
				VALUES ($1,$2,$3,'COORDINATOR',$4,true,$5, NULLIF($6,''), NULLIF($7,''), NULLIF($8,''))`,
				tenantID, email, string(hash), name, code, phone, whatsapp, reg)
			if ierr != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("row %d: %s", ln+1, ierr.Error()))
				res.Skipped++
				continue
			}
			res.Created++
			res.NewLogin = append(res.NewLogin, created{Email: email, TempPassword: temp})
		}
		writeJSON(w, http.StatusOK, res)
	}
}
