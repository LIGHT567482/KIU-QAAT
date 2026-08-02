package handlers

// SIS Import Pipeline — plan.md Team A Weeks 6–7
// POST /api/v1/import/csv   — manual CSV upload
// POST /api/v1/import/sync  — nightly automated pull (called by cron job)

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/qaat/api-gateway/internal/middleware"
)

// POST /api/v1/import/csv — multipart/form-data with file "roster"
// Roles: ADMIN, DQA_DIRECTOR
func ImportCSV(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "expected multipart/form-data"))
			return
		}

		file, _, err := r.FormFile("roster")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "field 'roster' not found"))
			return
		}
		defer file.Close()

		// Optional "import into this cohort" context: any value here is applied to
		// rows that don't carry it, so a file of just names+reg-numbers lands in the
		// chosen course/offering/year/semester.
		def := importDefaults{
			CourseID:   strings.TrimSpace(r.FormValue("course_id")),
			OfferingID: strings.TrimSpace(r.FormValue("offering_id")),
			Level:      strings.TrimSpace(r.FormValue("level")),
			StudyYear:  strings.TrimSpace(r.FormValue("study_year")),
			Semester:   strings.TrimSpace(r.FormValue("semester")),
			Intake:     strings.TrimSpace(r.FormValue("intake")),
			AcademicYr: strings.TrimSpace(r.FormValue("academic_year")),
		}

		result, parseErr := processCSV(r.Context(), pool, tenantID, file, def, r.Header.Get("Authorization"))
		if parseErr != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("CSV_PARSE_ERROR", parseErr.Error()))
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

type importResult struct {
	Inserted int      `json:"inserted"`
	Updated  int      `json:"updated"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors"`
}

// importDefaults is the optional "import into this cohort" context. Any non-empty
// field is applied to rows that omit it, so a minimal file (name + reg no + email)
// can be imported straight into a chosen course/offering/year/semester.
type importDefaults struct {
	CourseID   string
	OfferingID string
	Level      string
	StudyYear  string
	Semester   string
	Intake     string
	AcademicYr string
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// Required CSV columns (case-insensitive header row). course_id may instead come
// from the import-target context; email is auto-synthesised from the reg-no — so
// only the truly per-row fields are required.
var requiredCols = []string{
	"student_id", "full_name",
}

func processCSV(ctx context.Context, pool *pgxpool.Pool, tenantID string, r io.Reader, def importDefaults, authHeader string) (*importResult, error) {
	// Read the whole upload so we can accept EITHER CSV or Excel (.xlsx) — the
	// admin chooses the format (next.txt #6).
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("could not read file: %w", err)
	}
	var rows [][]string
	if looksXLSX(data) {
		rows, err = parseXLSX(data)
		if err != nil {
			return nil, err
		}
	} else {
		cr := csv.NewReader(bytes.NewReader(data))
		cr.TrimLeadingSpace = true
		cr.FieldsPerRecord = -1 // tolerate ragged rows
		rows, err = cr.ReadAll()
		if err != nil {
			return nil, fmt.Errorf("could not parse CSV: %w", err)
		}
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("file has no rows")
	}

	headers := rows[0]
	colIdx := make(map[string]int, len(headers))
	for i, h := range headers {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, req := range requiredCols {
		if _, ok := colIdx[req]; !ok {
			return nil, fmt.Errorf("missing required column: %s", req)
		}
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("db connection failed")
	}
	defer conn.Release()
	if err := middleware.SetTenantConn(ctx, conn, tenantID); err != nil {
		return nil, fmt.Errorf("set tenant: %w", err)
	}

	// Students get a hidden in-domain login (reg-no → synthetic email) so the QR /
	// check-in path keeps working without the admin entering emails. They never
	// password-login (the QR is their key), so one random hash for the batch is fine.
	var domain string
	_ = conn.QueryRow(ctx, `SELECT domain FROM tenants WHERE tenant_id = $1`, tenantID).Scan(&domain)
	domain = strings.ToLower(strings.TrimSpace(domain))
	// Default sign-in password for the unified app; students change it later. (Was a random
	// throwaway when students were QR/passwordless — see migration 052 for existing rows.)
	sharedHash, _ := bcrypt.GenerateFromPassword([]byte(DefaultStudentPassword), 10)

	res := &importResult{Errors: []string{}}

	for lineNum := 1; lineNum < len(rows); lineNum++ {
		row := rows[lineNum]

		get := func(col string) string {
			i, ok := colIdx[col]
			if !ok || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}

		studentID := get("student_id")
		fullName := get("full_name")
		// Row value first, else the import-target context.
		courseID := orDefault(get("course_id"), def.CourseID)
		academicYear := orDefault(get("academic_year"), def.AcademicYr)
		currentYear := orDefault(get("current_year"), def.StudyYear)
		semester := orDefault(get("semester"), def.Semester)
		level := orDefault(get("level"), def.Level)
		intake := orDefault(get("intake_session"), def.Intake)
		offeringID := orDefault(get("offering_id"), def.OfferingID)
		// Email is synthesised from the reg-no when not supplied (hidden identity).
		// realEmail tracks whether the row carried a genuine address — only those
		// students get their QR emailed (optional, QR-dispatch only).
		realEmail := get("email")
		email := orDefault(realEmail, synthEmail(studentID, domain))

		if studentID == "" || courseID == "" {
			res.Errors = append(res.Errors, fmt.Sprintf("line %d: student_id and a course (row or import target) are required", lineNum))
			res.Skipped++
			continue
		}

		// RETURNING (xmax = 0) distinguishes a fresh INSERT (xmax = 0 → true)
		// from an ON CONFLICT UPDATE of an existing row (xmax != 0 → false),
		// so the inserted/updated counts are accurate.
		var inserted bool
		err = conn.QueryRow(ctx, `
			INSERT INTO students_extended
			  (student_id, tenant_id, full_name, email, course_id, academic_year,
			   current_year, semester, level, intake_session, offering_id,
			   enrollment_status, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,
			  NULLIF($7,'')::smallint,
			  NULLIF($8,'')::smallint,
			  NULLIF($9,''),
			  NULLIF($10,''),
			  NULLIF($11,'')::uuid,
			  'ACTIVE', now())
			ON CONFLICT (student_id) DO UPDATE
			  SET full_name      = EXCLUDED.full_name,
			      email          = EXCLUDED.email,
			      course_id      = EXCLUDED.course_id,
			      academic_year  = EXCLUDED.academic_year,
			      current_year   = EXCLUDED.current_year,
			      semester       = EXCLUDED.semester,
			      level          = COALESCE(EXCLUDED.level, students_extended.level),
			      intake_session = COALESCE(EXCLUDED.intake_session, students_extended.intake_session),
			      offering_id    = COALESCE(EXCLUDED.offering_id, students_extended.offering_id),
			      updated_at     = now()
			RETURNING (xmax = 0)`,
			studentID, tenantID, fullName, email, courseID, academicYear,
			currentYear, semester, level, intake, offeringID,
		).Scan(&inserted)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("line %d: %s", lineNum, err.Error()))
			res.Skipped++
			continue
		}

		// Hidden login so the QR / check-in identity path (users.email → student) works
		// for imported students too. They never password-login.
		if domain != "" {
			_, _ = conn.Exec(ctx, `
				INSERT INTO users (tenant_id, email, password_hash, role, full_name, force_password_change)
				VALUES ($1, $2, $3, 'STUDENT', $4, true)
				ON CONFLICT (tenant_id, email) DO NOTHING`,
				tenantID, email, string(sharedHash), fullName)
		}

		if inserted {
			res.Inserted++
		} else {
			res.Updated++
		}
	}

	return res, nil
}

// POST /api/v1/import/trigger — trigger a nightly SIS pull (stubbed; connects
// to configured SIS REST API). In production this is called by a Kubernetes
// CronJob; here it can also be triggered manually by ADMIN.
func ImportTrigger(_ *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusAccepted, map[string]interface{}{
			"status":      "QUEUED",
			"job_id":      fmt.Sprintf("sis-pull-%d", time.Now().Unix()),
			"description": "Automated SIS pull queued — see nightly cron schedule",
		})
	}
}
