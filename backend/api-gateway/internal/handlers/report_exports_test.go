package handlers

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sampleTable() reportTable {
	return reportTable{
		Title:    "Lecturer Attendance",
		Subtitle: "2 lecturer(s)",
		Headers:  []string{"Lecturer", "Department", "Sessions", "Total hrs"},
		Weights:  []float64{3, 2, 1, 1},
		Rows: [][]string{
			{"Dr. Jane Smith", "Computer Science", "12", "18.5"},
			{"Prof. John Okello", "Nursing", "7", "10.0"},
		},
	}
}

func TestWriteReportXLSXProducesAWorkbook(t *testing.T) {
	rec := httptest.NewRecorder()
	writeReportXLSX(rec, "lecturer-attendance.xlsx", sampleTable())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "spreadsheetml") {
		t.Errorf("Content-Type = %q, want an xlsx type", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "lecturer-attendance.xlsx") {
		t.Errorf("Content-Disposition = %q, want the filename", cd)
	}
	body := rec.Body.Bytes()
	// An .xlsx is a zip: it must start with the local-file-header magic.
	if !bytes.HasPrefix(body, []byte("PK\x03\x04")) {
		t.Fatalf("body is not a zip archive (first bytes %q)", body[:min(4, len(body))])
	}
	// Round-trip through the project's own reader: header row + both data rows.
	rows, err := parseXLSX(body)
	if err != nil {
		t.Fatalf("parseXLSX: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3 (header + 2)", len(rows))
	}
	if rows[0][0] != "Lecturer" {
		t.Errorf("header[0] = %q, want %q", rows[0][0], "Lecturer")
	}
	if rows[1][0] != "Dr. Jane Smith" {
		t.Errorf("row1[0] = %q, want %q", rows[1][0], "Dr. Jane Smith")
	}
}

func TestWriteReportCSVProducesAReadableFile(t *testing.T) {
	rec := httptest.NewRecorder()
	writeReportCSV(rec, "lecturer-attendance.csv", sampleTable())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("Content-Type = %q, want text/csv", ct)
	}
	rows, err := csv.NewReader(bytes.NewReader(rec.Body.Bytes())).ReadAll()
	if err != nil {
		t.Fatalf("output is not valid CSV: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d records, want 3 (header + 2)", len(rows))
	}
	if rows[0][0] != "Lecturer" || rows[1][0] != "Dr. Jane Smith" {
		t.Errorf("unexpected content: header=%v row1=%v", rows[0], rows[1])
	}
}

// A cell holding a comma or a quote must survive the round trip — employee-attendance
// comments routinely contain both.
func TestWriteReportCSVQuotesAwkwardCells(t *testing.T) {
	tbl := reportTable{
		Headers: []string{"Staff ID", "Comment"},
		Rows:    [][]string{{"KIU/001", `Checked in 08:12, out 17:03; noted "late"`}},
	}
	rec := httptest.NewRecorder()
	writeReportCSV(rec, "x.csv", tbl)
	rows, err := csv.NewReader(bytes.NewReader(rec.Body.Bytes())).ReadAll()
	if err != nil {
		t.Fatalf("not valid CSV: %v", err)
	}
	if got := rows[1][1]; got != `Checked in 08:12, out 17:03; noted "late"` {
		t.Errorf("cell round-tripped as %q", got)
	}
}

// A row shorter than the header must be padded, or the CSV becomes ragged and readers reject it.
func TestWriteReportCSVPadsShortRows(t *testing.T) {
	tbl := reportTable{Headers: []string{"A", "B", "C"}, Rows: [][]string{{"only-one"}}}
	rec := httptest.NewRecorder()
	writeReportCSV(rec, "x.csv", tbl)
	rows, err := csv.NewReader(bytes.NewReader(rec.Body.Bytes())).ReadAll()
	if err != nil {
		t.Fatalf("ragged CSV rejected by the reader: %v", err)
	}
	if len(rows[1]) != 3 {
		t.Errorf("row has %d fields, want 3", len(rows[1]))
	}
}

func TestWriteReportPDFProducesAPDF(t *testing.T) {
	rec := httptest.NewRecorder()
	writeReportPDF(rec, "lecturer-attendance.pdf", "Kampala International University", sampleTable())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type = %q, want application/pdf", ct)
	}
	body := rec.Body.Bytes()
	if !bytes.HasPrefix(body, []byte("%PDF-")) {
		t.Fatalf("body is not a PDF (first bytes %q)", body[:min(8, len(body))])
	}
	if !bytes.Contains(body, []byte("%%EOF")) {
		t.Error("PDF is not terminated with an EOF marker")
	}
	if len(body) < 1000 {
		t.Errorf("PDF is only %d bytes — suspiciously empty", len(body))
	}
}

// An empty result set must still render a valid, readable file rather than erroring.
func TestWriteReportHandlesNoRows(t *testing.T) {
	empty := reportTable{Title: "Student Attendance", Headers: []string{"Reg No.", "Name"}}

	rec := httptest.NewRecorder()
	writeReportPDF(rec, "x.pdf", "", empty)
	if rec.Code != http.StatusOK || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF-")) {
		t.Errorf("empty PDF: status %d, %d bytes", rec.Code, rec.Body.Len())
	}

	rec = httptest.NewRecorder()
	writeReportXLSX(rec, "x.xlsx", empty)
	rows, err := parseXLSX(rec.Body.Bytes())
	if err != nil {
		t.Fatalf("empty xlsx: parseXLSX: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("empty xlsx: got %d rows, want 1 (header only)", len(rows))
	}
}

// captureJSON must forward a failing report's own status and body untouched, so an
// export never turns a 403 into a corrupt download.
func TestCaptureJSONForwardsFailures(t *testing.T) {
	failing := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusForbidden, errBody("NOT_ALLOWED", "nope"))
	}
	rec := httptest.NewRecorder()
	body, ok := captureJSON(failing, rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if ok {
		t.Fatal("captureJSON reported success for a 403")
	}
	if body != nil {
		t.Errorf("body = %q, want nil on failure", body)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "NOT_ALLOWED") {
		t.Errorf("body = %q, want the original error forwarded", rec.Body.String())
	}
}
