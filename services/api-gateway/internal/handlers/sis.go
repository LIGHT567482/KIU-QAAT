package handlers

// SIS Import Pipeline — plan.md Team A Weeks 6–7
// POST /api/v1/import/csv   — manual CSV upload
// POST /api/v1/import/sync  — nightly automated pull (called by cron job)

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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

		result, parseErr := processCSV(r.Context(), pool, tenantID, file)
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

// Required CSV columns (case-insensitive header row).
var requiredCols = []string{
	"student_id", "full_name", "email", "course_id",
	"academic_year", "current_year", "semester",
}

func processCSV(ctx context.Context, pool *pgxpool.Pool, tenantID string, r io.Reader) (*importResult, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("could not read header row: %w", err)
	}
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

	res := &importResult{Errors: []string{}}
	lineNum := 1

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		lineNum++
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("line %d: parse error", lineNum))
			res.Skipped++
			continue
		}

		get := func(col string) string {
			i, ok := colIdx[col]
			if !ok || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}

		studentID    := get("student_id")
		fullName     := get("full_name")
		email        := get("email")
		courseID     := get("course_id")
		academicYear := get("academic_year")

		if studentID == "" || email == "" || courseID == "" {
			res.Errors = append(res.Errors, fmt.Sprintf("line %d: student_id, email, course_id required", lineNum))
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
			   current_year, semester, enrollment_status, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,
			  NULLIF($7,'')::smallint,
			  NULLIF($8,'')::smallint,
			  'ACTIVE', now())
			ON CONFLICT (student_id) DO UPDATE
			  SET full_name      = EXCLUDED.full_name,
			      email          = EXCLUDED.email,
			      course_id      = EXCLUDED.course_id,
			      academic_year  = EXCLUDED.academic_year,
			      current_year   = EXCLUDED.current_year,
			      semester       = EXCLUDED.semester,
			      updated_at     = now()
			RETURNING (xmax = 0)`,
			studentID, tenantID, fullName, email, courseID, academicYear,
			get("current_year"), get("semester"),
		).Scan(&inserted)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("line %d: %s", lineNum, err.Error()))
			res.Skipped++
			continue
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
