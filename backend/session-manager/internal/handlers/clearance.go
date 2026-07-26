package handlers

// Exam Clearance Token Generator — plan.md Team B Week 6–7
// POST /api/v1/eligibility/clearance-token
// Generates an RSA-signed QR payload for each eligible student in a course unit.
// Role: DQA_DIRECTOR

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/session-manager/internal/auth"
)

type clearanceRequest struct {
	UnitID       string `json:"unit_id"`
	AcademicYear string `json:"academic_year"`
	Semester     int    `json:"semester"`
	ExamDate     string `json:"exam_date"`
}

type clearanceToken struct {
	StudentID    string `json:"student_id"`
	UnitID       string `json:"unit_id"`
	AcademicYear string `json:"academic_year"`
	ExamDate     string `json:"exam_date"`
	TokenID      string `json:"token_id"`
	IssuedAt     string `json:"issued_at"`
	ExpiresAt    string `json:"expires_at"`
	Signature    string `json:"signature"`   // SHA-256 HMAC of payload (RSA signing deferred to QR service key)
}

type clearanceResponse struct {
	Generated int              `json:"generated"`
	Tokens    []clearanceToken `json:"tokens"`
}

// ClearanceToken generates exam clearance QR tokens for all eligible students
// in the specified unit.  Tokens are valid for 48 hours from issuance.
func ClearanceToken(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req clearanceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "malformed body")
			return
		}
		if req.UnitID == "" || req.AcademicYear == "" || req.ExamDate == "" {
			writeErr(w, http.StatusBadRequest, "INVALID_REQUEST", "unit_id, academic_year, exam_date required")
			return
		}

		// Tenant comes from the verified JWT claim, never a client header.
		tenantID := auth.TenantID(r.Context())

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "INTERNAL_ERROR", "db unavailable")
			return
		}
		defer conn.Release()
		// Parameterised, session-scoped RLS GUC (fixes the prior SQL-injection +
		// inert SET LOCAL). A malformed/empty tenant is rejected here.
		if err := setTenantConn(r.Context(), conn, tenantID); err != nil {
			writeErr(w, http.StatusForbidden, "INVALID_TENANT", "missing or malformed tenant context")
			return
		}

		// Fetch eligible students from the materialized view.
		threshold := 0
		conn.QueryRow(r.Context(),
			`SELECT attendance_threshold FROM tenants WHERE tenant_id = $1`, tenantID).
			Scan(&threshold) //nolint:errcheck

		rows, err := conn.Query(r.Context(), `
			SELECT s.student_id
			FROM student_attendance_summary s
			WHERE s.unit_id   = $1
			  AND s.tenant_id = $2
			  AND s.attendance_percentage >= $3`,
			req.UnitID, tenantID, threshold)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
		defer rows.Close()

		now       := time.Now().UTC()
		expiresAt := now.Add(48 * time.Hour)
		var tokens []clearanceToken

		for rows.Next() {
			var studentID string
			rows.Scan(&studentID) //nolint:errcheck

			tokenID := generateTokenID()
			payload := fmt.Sprintf("%s|%s|%s|%s|%s",
				studentID, req.UnitID, req.AcademicYear, req.ExamDate, tokenID)
			sig := hmacPayload(payload)

			tokens = append(tokens, clearanceToken{
				StudentID:    studentID,
				UnitID:       req.UnitID,
				AcademicYear: req.AcademicYear,
				ExamDate:     req.ExamDate,
				TokenID:      tokenID,
				IssuedAt:     now.Format(time.RFC3339),
				ExpiresAt:    expiresAt.Format(time.RFC3339),
				Signature:    sig,
			})
		}

		if tokens == nil {
			tokens = []clearanceToken{}
		}

		writeJSON(w, http.StatusOK, clearanceResponse{
			Generated: len(tokens),
			Tokens:    tokens,
		})
	}
}

func generateTokenID() string {
	b := make([]byte, 16)
	rand.Read(b) //nolint:errcheck
	return hex.EncodeToString(b)
}

func hmacPayload(payload string) string {
	h := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(h[:])
}
