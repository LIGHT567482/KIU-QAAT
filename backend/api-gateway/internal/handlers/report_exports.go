package handlers

// Downloadable reports — every report in the Reports hub can be taken away as an Excel
// workbook, a CSV or a PDF, whichever the admin needs:
//
//	GET /api/v1/dashboard/qa/student-attendance/export.{xlsx,csv,pdf}
//	GET /api/v1/dashboard/lecturer-attendance/export.{xlsx,csv,pdf}          (coordinator record)
//	GET /api/v1/dashboard/lecturer-attendance/patrol/export.{xlsx,csv,pdf}   (QA patrol record)
//	GET /api/v1/admin/tenants/{tenant_id}/employee-attendance/export.{xlsx,csv,pdf}
//	GET /api/v1/reports/lecturer-teaching/export.{xlsx,csv,pdf}
//
// Each exporter reuses the SAME handler the on-screen report is drawn from — invoked
// here and its JSON captured — so a download can never drift from the table the admin
// is looking at, and every filter, role guard and org scope applies unchanged.

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// reportTable is the shape both writers render: a titled grid of strings.
type reportTable struct {
	Title    string
	Subtitle string
	Headers  []string
	// Relative column weights for the PDF. Empty = equal columns.
	Weights []float64
	Rows    [][]string
}

// captureJSON runs an existing report handler against this request and hands back the
// JSON it produced. A non-200 is forwarded to the client verbatim (ok=false), so an
// export inherits the report's own error handling — including its authorisation.
func captureJSON(h http.HandlerFunc, w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	rec := httptest.NewRecorder()
	h(rec, r)
	if rec.Code != http.StatusOK {
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
		return nil, false
	}
	return rec.Body.Bytes(), true
}

// reportTenantName resolves the institution name for the PDF letterhead. Best-effort:
// an unnamed tenant just yields a header without one.
func reportTenantName(r *http.Request, pool *pgxpool.Pool, tenantID string) string {
	var name string
	_ = pool.QueryRow(r.Context(), `SELECT COALESCE(name,'') FROM tenants WHERE tenant_id = $1`, tenantID).Scan(&name)
	return name
}

// writeReportCSV sends the table as a .csv download — the format spreadsheets, SIS imports and
// statistics tools all read without ceremony.
func writeReportCSV(w http.ResponseWriter, filename string, t reportTable) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	cw := csv.NewWriter(w)
	_ = cw.Write(t.Headers)
	for _, row := range t.Rows {
		// Pad short rows so every line has the same column count.
		if len(row) < len(t.Headers) {
			padded := make([]string, len(t.Headers))
			copy(padded, row)
			row = padded
		}
		_ = cw.Write(row)
	}
	cw.Flush()
}

// writeReport dispatches one table to whichever format the caller asked for. `stem` is the
// filename without an extension.
func writeReport(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, format, stem string, t reportTable) {
	switch format {
	case "csv":
		writeReportCSV(w, stem+".csv", t)
	case "pdf":
		writeReportPDF(w, stem+".pdf", reportTenantName(r, pool, middleware.GetTenantID(r.Context())), t)
	default: // xlsx
		writeReportXLSX(w, stem+".xlsx", t)
	}
}

// formatFloat1 renders a rate/average to one decimal place.
func formatFloat1(f float64) string { return fmt.Sprintf("%.1f", f) }

// writeReportXLSX sends the table as an .xlsx download.
func writeReportXLSX(w http.ResponseWriter, filename string, t reportTable) {
	rows := make([][]string, 0, len(t.Rows)+1)
	rows = append(rows, t.Headers)
	rows = append(rows, t.Rows...)
	data, err := buildXLSX(rows)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	_, _ = w.Write(data)
}

// writeReportPDF sends the table as a landscape A4 PDF: institution letterhead, a
// banded grid that repeats its header on every page, and a page counter.
func writeReportPDF(w http.ResponseWriter, filename, institution string, t reportTable) {
	const (
		pageW   = 297.0 // A4 landscape
		margin  = 12.0
		usableW = pageW - 2*margin
	)
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(margin, margin, margin)
	pdf.SetAutoPageBreak(true, 16)

	// Column widths from the relative weights (equal when none were given).
	weights := t.Weights
	if len(weights) != len(t.Headers) {
		weights = make([]float64, len(t.Headers))
		for i := range weights {
			weights[i] = 1
		}
	}
	total := 0.0
	for _, x := range weights {
		total += x
	}
	widths := make([]float64, len(weights))
	for i, x := range weights {
		widths[i] = usableW * x / total
	}

	header := func() {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetFillColor(30, 41, 59)
		pdf.SetTextColor(255, 255, 255)
		for i, h := range t.Headers {
			pdf.CellFormat(widths[i], 7, h, "1", 0, "L", true, 0, "")
		}
		pdf.Ln(-1)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(15, 23, 42)
	}
	// Repeat the column header after every automatic page break.
	pdf.SetHeaderFunc(func() {
		if pdf.PageNo() == 1 {
			return
		}
		header()
	})
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "", 7)
		pdf.SetTextColor(100, 116, 139)
		pdf.CellFormat(0, 6, fmt.Sprintf("Page %d", pdf.PageNo()), "", 0, "C", false, 0, "")
	})

	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 15)
	pdf.SetTextColor(15, 23, 42)
	title := t.Title
	if institution != "" {
		title = institution + " — " + t.Title
	}
	pdf.CellFormat(0, 8, title, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(100, 116, 139)
	sub := t.Subtitle
	if sub != "" {
		sub += " · "
	}
	pdf.CellFormat(0, 5, sub+"Generated "+time.Now().Format("2006-01-02 15:04"), "", 1, "L", false, 0, "")
	pdf.Ln(3)

	header()
	for i, row := range t.Rows {
		if i%2 == 0 {
			pdf.SetFillColor(248, 250, 252)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		for j := range t.Headers {
			cell := ""
			if j < len(row) {
				cell = row[j]
			}
			// Clip rather than wrap, so every record stays on exactly one line.
			for pdf.GetStringWidth(cell) > widths[j]-2 && len(cell) > 1 {
				cell = cell[:len(cell)-1]
			}
			pdf.CellFormat(widths[j], 6, cell, "1", 0, "L", true, 0, "")
		}
		pdf.Ln(-1)
	}
	if len(t.Rows) == 0 {
		pdf.SetFont("Helvetica", "I", 9)
		pdf.SetTextColor(100, 116, 139)
		pdf.CellFormat(0, 8, "No records match the selected filters.", "", 1, "L", false, 0, "")
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	_ = pdf.Output(w)
}

// ─── Student attendance ───────────────────────────────────────────────────────

// GET /api/v1/dashboard/qa/student-attendance/export.{xlsx,csv,pdf}
func QAStudentAttendanceReport(pool *pgxpool.Pool, format string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		list, err := queryStudentAttendance(r.Context(), pool, tenantID, qaFilters(r))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		t := reportTable{
			Title:    "Student Attendance",
			Subtitle: itoa(len(list)) + " student(s)",
			Headers:  []string{"Reg No.", "Name", "Course", "Level", "Session", "Year", "Sem", "Held", "Attended", "%"},
			Weights:  []float64{2.2, 3, 3.4, 1.4, 1.4, 0.7, 0.7, 0.8, 1, 0.9},
		}
		for _, s := range list {
			t.Rows = append(t.Rows, []string{
				s.StudentID, s.FullName, s.Course, s.Level, s.Session,
				itoa(s.Year), itoa(s.Semester), itoa(s.Held), itoa(s.Attended),
				formatFloat1(s.Percentage),
			})
		}
		writeReport(w, r, pool, format, "student-attendance", t)
	}
}

// ─── Lecturer attendance ──────────────────────────────────────────────────────

type lecturerSummaryExport struct {
	LecturerID        string  `json:"lecturer_id"`
	LecturerName      string  `json:"lecturer_name"`
	Department        string  `json:"department"`
	Email             string  `json:"email"`
	TotalSessions     int     `json:"total_sessions"`
	TotalContactHours float64 `json:"total_contact_hours"`
	AvgContactHours   float64 `json:"avg_contact_hours"`
	LastSessionDate   string  `json:"last_session_date"`
}

func lecturerAttendanceTable(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) (reportTable, bool) {
	body, ok := captureJSON(LecturerAttendanceSummaryForCaller(pool), w, r)
	if !ok {
		return reportTable{}, false
	}
	var rows []lecturerSummaryExport
	if err := json.Unmarshal(body, &rows); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
		return reportTable{}, false
	}
	t := reportTable{
		Title:    "Lecturer Attendance",
		Subtitle: fmt.Sprintf("%d lecturer(s)", len(rows)),
		Headers:  []string{"Lecturer", "Department", "Email", "Sessions", "Total hrs", "Avg hrs", "Last session"},
		Weights:  []float64{3, 2.6, 3, 1.1, 1.2, 1.1, 1.5},
	}
	for _, s := range rows {
		t.Rows = append(t.Rows, []string{
			s.LecturerName, s.Department, s.Email, itoa(s.TotalSessions),
			fmt.Sprintf("%.1f", s.TotalContactHours), fmt.Sprintf("%.1f", s.AvgContactHours),
			s.LastSessionDate,
		})
	}
	return t, true
}

// GET /api/v1/dashboard/lecturer-attendance/export.{xlsx,csv,pdf}
func LecturerAttendanceExport(pool *pgxpool.Pool, format string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if t, ok := lecturerAttendanceTable(w, r, pool); ok {
			writeReport(w, r, pool, format, "lecturer-attendance", t)
		}
	}
}

// ─── Employee attendance ──────────────────────────────────────────────────────

type employeeReportExport struct {
	From string `json:"from"`
	To   string `json:"to"`
	Rows []struct {
		StaffID  string `json:"staff_id"`
		Title    string `json:"title"`
		FullName string `json:"full_name"`
		Contact  string `json:"contact"`
		Comment  string `json:"comment"`
	} `json:"rows"`
}

func employeeAttendanceTable(w http.ResponseWriter, r *http.Request, adminPool *pgxpool.Pool) (reportTable, bool) {
	body, ok := captureJSON(EmployeeAttendanceReport(adminPool), w, r)
	if !ok {
		return reportTable{}, false
	}
	var rep employeeReportExport
	if err := json.Unmarshal(body, &rep); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
		return reportTable{}, false
	}
	t := reportTable{
		Title:    "Employee Attendance",
		Subtitle: fmt.Sprintf("%s to %s · %d employee(s)", rep.From, rep.To, len(rep.Rows)),
		Headers:  []string{"Staff ID", "Title", "Name", "Contact", "Comment"},
		Weights:  []float64{1.8, 1, 2.8, 2.4, 7},
	}
	for _, e := range rep.Rows {
		t.Rows = append(t.Rows, []string{e.StaffID, e.Title, e.FullName, e.Contact, e.Comment})
	}
	return t, true
}

// GET /api/v1/admin/tenants/{tenant_id}/employee-attendance/export.{xlsx,csv,pdf}
func EmployeeAttendanceExport(adminPool *pgxpool.Pool, format string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if t, ok := employeeAttendanceTable(w, r, adminPool); ok {
			writeReport(w, r, adminPool, format, "employee-attendance", t)
		}
	}
}

// ─── Lecturer teaching (QA patrols) ───────────────────────────────────────────

type teachingReportExport struct {
	Rows []struct {
		LecturerID string `json:"lecturer_id"`
		Name       string `json:"full_name"`
		Department string `json:"department"`
		School     string `json:"school"`
		Taught     int    `json:"taught"`
		Patrolled  int    `json:"patrolled"`
	} `json:"rows"`
	TotalTaught    int `json:"total_taught"`
	TotalPatrolled int `json:"total_patrolled"`
}

func teachingReportTable(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) (reportTable, bool) {
	body, ok := captureJSON(LecturerTeachingReport(pool), w, r)
	if !ok {
		return reportTable{}, false
	}
	var rep teachingReportExport
	if err := json.Unmarshal(body, &rep); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
		return reportTable{}, false
	}
	pct := func(a, b int) string {
		if b == 0 {
			return "—"
		}
		return fmt.Sprintf("%.0f%%", float64(a)/float64(b)*100)
	}
	t := reportTable{
		Title: "Lecturer Teaching (patrol rollup)",
		Subtitle: fmt.Sprintf("%d lecturer(s) · %d of %d patrols found teaching (%s)",
			len(rep.Rows), rep.TotalTaught, rep.TotalPatrolled, pct(rep.TotalTaught, rep.TotalPatrolled)),
		Headers: []string{"Lecturer", "Staff ID", "Department", "School", "Taught", "Patrolled", "Rate"},
		Weights: []float64{3, 2, 2.6, 2.6, 1, 1.2, 1},
	}
	for _, l := range rep.Rows {
		name := l.Name
		if name == "" {
			name = l.LecturerID
		}
		t.Rows = append(t.Rows, []string{
			name, l.LecturerID, l.Department, l.School,
			itoa(l.Taught), itoa(l.Patrolled), pct(l.Taught, l.Patrolled),
		})
	}
	return t, true
}

// GET /api/v1/reports/lecturer-teaching/export.{xlsx,csv,pdf}
func LecturerTeachingExport(pool *pgxpool.Pool, format string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if t, ok := teachingReportTable(w, r, pool); ok {
			writeReport(w, r, pool, format, "lecturer-teaching", t)
		}
	}
}
