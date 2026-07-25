package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

type studentQRResponse struct {
	StudentID    string `json:"student_id"`
	SerialNumber string `json:"serial_number"`
	Token        string `json:"token"`
	URL          string `json:"url"`
	Image        string `json:"image"`
}

func StudentMyQR(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		tenantID := middleware.GetTenantID(r.Context())
		if userID == "" || tenantID == "" {
			writeJSON(w, http.StatusUnauthorized, errBody("UNAUTHORIZED", "not authenticated"))
			return
		}

		conn, err := adminPool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "could not connect"))
			return
		}
		defer conn.Release()

		// Look up the student's details from the user_id (JWT sub claim).
		var studentID, email, courseID, academicYear, qrSerial string
		err = conn.QueryRow(r.Context(), `
			SELECT se.student_id, se.email, se.course_id, se.academic_year,
			       COALESCE(se.qr_serial_number, '')
			FROM students_extended se
			JOIN users u ON u.email = se.email AND u.tenant_id = se.tenant_id
			WHERE u.user_id = $1 AND u.tenant_id = $2
			  AND u.role = 'STUDENT' AND u.is_active = true
			  AND se.enrollment_status = 'ACTIVE'`,
			userID, tenantID).Scan(&studentID, &email, &courseID, &academicYear, &qrSerial)
		if err != nil {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "no active student record for this account"))
			return
		}

		// Call the QR generator service to produce a signed QR token + image.
		// Pass the student's own JWT through — the QR generator verifies RS256
		// signatures itself and does not check roles, only tenant binding.
		qrGenBase := os.Getenv("QR_GENERATOR_URL")
		if qrGenBase == "" {
			qrGenBase = "http://qr-generator:3002"
		}

		body, _ := json.Marshal(map[string]string{
			"student_id": studentID,
		})

		rctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		qrReq, err := http.NewRequestWithContext(rctx, http.MethodPost, qrGenBase+"/api/v1/qr/token", bytes.NewReader(body))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "could not build request"))
			return
		}
		qrReq.Header.Set("Content-Type", "application/json")
		qrReq.Header.Set("X-Tenant-ID", tenantID)
		if auth := r.Header.Get("Authorization"); auth != "" {
			qrReq.Header.Set("Authorization", auth)
		}

		resp, err := http.DefaultClient.Do(qrReq)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, errBody("QR_UNAVAILABLE", "QR generator unreachable"))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			writeJSON(w, http.StatusBadGateway, errBody("QR_FAILED", "QR generator returned an error"))
			return
		}

		var qrData studentQRResponse
		if err := json.NewDecoder(resp.Body).Decode(&qrData); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "could not parse QR response"))
			return
		}
		qrData.StudentID = studentID

		writeJSON(w, http.StatusOK, qrData)
	}
}
