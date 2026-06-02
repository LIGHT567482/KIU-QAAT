package handlers

// Export handlers — plan.md Team D/B Week 8
//
// GET /api/v1/reports/vc/audit.pdf   — Role: VC   — PDF audit report
// GET /api/v1/reports/dqa/eligibility.csv — Role: DQA_DIRECTOR — CSV eligibility list

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qaat/api-gateway/internal/middleware"
)

// ─── PDF Audit Report ─────────────────────────────────────────────────────────

func VCAuditPDF(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		defer conn.Release()
		middleware.SetTenantConn(r.Context(), conn, tenantID) //nolint:errcheck

		// Fetch tenant name.
		var tenantName string
		conn.QueryRow(r.Context(), `SELECT name FROM tenants WHERE tenant_id = $1`, tenantID).
			Scan(&tenantName) //nolint:errcheck

		// Fetch session audit data.
		type sessionRow struct {
			SessionID   string
			UnitName    string
			VenueID     string
			CoordID     string
			Date        string
			Status      string
			StudentCount int
			AuditFlags  string
		}

		rows, err := conn.Query(r.Context(), `
			SELECT s.session_id, cu.name, COALESCE(s.venue_id,''),
			       s.coordinator_id, s.session_date::text, s.session_status::text,
			       COUNT(al.log_id),
			       COALESCE(array_to_string(s.audit_flags, ', '),'—')
			FROM sessions s
			JOIN course_units cu ON cu.unit_id = s.unit_id
			LEFT JOIN attendance_logs al ON al.session_id = s.session_id
			WHERE s.tenant_id = $1
			  AND s.session_date >= CURRENT_DATE - INTERVAL '30 days'
			GROUP BY s.session_id, cu.name
			ORDER BY s.session_date DESC
			LIMIT 500`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		var sessions []sessionRow
		for rows.Next() {
			var sr sessionRow
			rows.Scan(&sr.SessionID, &sr.UnitName, &sr.VenueID, &sr.CoordID,
				&sr.Date, &sr.Status, &sr.StudentCount, &sr.AuditFlags) //nolint:errcheck
			sessions = append(sessions, sr)
		}

		// Build PDF.
		pdf := fpdf.New("L", "mm", "A4", "")
		pdf.SetMargins(15, 15, 15)
		pdf.AddPage()

		// Header.
		pdf.SetFont("Helvetica", "B", 18)
		pdf.CellFormat(0, 10, "QAAT — Session Audit Report", "", 1, "C", false, 0, "")
		pdf.SetFont("Helvetica", "", 10)
		pdf.CellFormat(0, 6, tenantName+" · Generated: "+time.Now().UTC().Format("2006-01-02 15:04 UTC"), "", 1, "C", false, 0, "")
		pdf.Ln(6)

		// Table header.
		headers := []string{"Date", "Unit", "Venue", "Coordinator", "Status", "Students", "Flags"}
		widths  := []float64{22, 70, 28, 40, 28, 22, 57}

		pdf.SetFont("Helvetica", "B", 8)
		pdf.SetFillColor(30, 41, 59)
		pdf.SetTextColor(255, 255, 255)
		for i, h := range headers {
			pdf.CellFormat(widths[i], 7, h, "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)

		// Table rows.
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(0, 0, 0)
		for i, s := range sessions {
			if i%2 == 0 {
				pdf.SetFillColor(248, 250, 252)
			} else {
				pdf.SetFillColor(255, 255, 255)
			}
			vals := []string{s.Date, s.UnitName, s.VenueID, s.CoordID, s.Status, fmt.Sprint(s.StudentCount), s.AuditFlags}
			for j, v := range vals {
				pdf.CellFormat(widths[j], 6, v, "1", 0, "L", true, 0, "")
			}
			pdf.Ln(-1)
		}

		// Ghost lecture summary.
		var ghostCount int
		conn.QueryRow(r.Context(), `
			SELECT COUNT(*) FROM sessions
			WHERE tenant_id = $1 AND 'GHOST_LECTURE_SUSPECTED' = ANY(audit_flags)
			  AND session_date >= CURRENT_DATE - INTERVAL '30 days'`, tenantID).
			Scan(&ghostCount) //nolint:errcheck

		pdf.Ln(8)
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(0, 6, fmt.Sprintf("Ghost lectures suspected (last 30 days): %d", ghostCount), "", 1, "L", false, 0, "")

		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="qaat-audit-%s.pdf"`, time.Now().Format("2006-01-02")))
		pdf.Output(w) //nolint:errcheck
	}
}

// ─── CSV Eligibility Export ───────────────────────────────────────────────────

func DQAEligibilityCSV(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		defer conn.Release()
		middleware.SetTenantConn(r.Context(), conn, tenantID) //nolint:errcheck

		var threshold int
		conn.QueryRow(r.Context(), `SELECT attendance_threshold FROM tenants WHERE tenant_id = $1`, tenantID).
			Scan(&threshold) //nolint:errcheck

		rows, err := conn.Query(r.Context(), `
			SELECT s.student_id, se.full_name, se.email,
			       s.unit_id, s.unit_name, se.course_id,
			       s.sessions_held, s.sessions_attended,
			       s.attendance_percentage,
			       CASE WHEN s.attendance_percentage >= $2 THEN 'ELIGIBLE' ELSE 'INELIGIBLE' END
			FROM student_attendance_summary s
			JOIN students_extended se ON se.student_id = s.student_id
			WHERE s.tenant_id = $1
			ORDER BY se.course_id, s.unit_id, s.student_id`, tenantID, threshold)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="qaat-eligibility-%s.csv"`, time.Now().Format("2006-01-02")))

		cw := csv.NewWriter(w)
		cw.Write([]string{ //nolint:errcheck
			"student_id", "full_name", "email", "course_id",
			"unit_id", "unit_name", "sessions_held", "sessions_attended",
			"attendance_percentage", "status",
		})

		for rows.Next() {
			var studentID, fullName, email, unitID, unitName, courseID, status string
			var held, attended int
			var pct float64
			rows.Scan(&studentID, &fullName, &email, &unitID, &unitName, &courseID,
				&held, &attended, &pct, &status) //nolint:errcheck
			cw.Write([]string{ //nolint:errcheck
				studentID, fullName, email, courseID,
				unitID, unitName,
				fmt.Sprint(held), fmt.Sprint(attended),
				fmt.Sprintf("%.2f", pct), status,
			})
		}
		cw.Flush()
	}
}
