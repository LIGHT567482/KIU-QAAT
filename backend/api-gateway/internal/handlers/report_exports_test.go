package handlers

import (
	"bytes"
	"compress/zlib"
	"encoding/csv"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-pdf/fpdf"
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
	// trimBOM, because the file deliberately opens with one so Excel reads it as UTF-8 —
	// see report_text.go. Go's csv.Reader hands it through on the first field.
	if trimBOM(rows[0][0]) != "Lecturer" || rows[1][0] != "Dr. Jane Smith" {
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

// ─── Encoding ────────────────────────────────────────────────────────────────
//
// The bug these pin was reported as "remove â€ from every report". It was not a stray string to
// delete: it was every non-ASCII character in every PDF, drawn one glyph per UTF-8 byte by a
// cp1252 core font. It reads as cosmetic, it is invisible in code review, and it comes straight
// back the moment someone writes a new CellFormat without the encoder — so it is pinned by the
// bytes, not by eye.

// pdfTextBytes returns everything a PDF actually draws, with the content streams inflated.
//
// WITHOUT THIS THE TEST BELOW WAS VACUOUS AND I PROVED IT: with the encoder deliberately disabled,
// searching the raw file for the mojibake byte sequence still PASSED, because fpdf deflates page
// content by default. A test for a byte sequence that is compressed out of sight asserts nothing.
func pdfTextBytes(t *testing.T, body []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	out.Write(body) // uncompressed objects (and anything not in a stream) count too
	rest := body
	for {
		i := bytes.Index(rest, []byte("stream"))
		if i < 0 {
			break
		}
		start := i + len("stream")
		// Skip the EOL that must follow the keyword.
		for start < len(rest) && (rest[start] == '\r' || rest[start] == '\n') {
			start++
		}
		j := bytes.Index(rest[start:], []byte("endstream"))
		if j < 0 {
			break
		}
		if zr, err := zlib.NewReader(bytes.NewReader(rest[start : start+j])); err == nil {
			if dec, err := io.ReadAll(zr); err == nil {
				out.Write(dec)
			}
			_ = zr.Close()
		}
		rest = rest[start+j:]
	}
	return out.Bytes()
}

// The exact sequence people were reading. An em dash written raw arrives as E2 80 94 and the font
// draws â, €, " — so the fix is proven by those three bytes NOT being adjacent in the output, and
// by the single cp1252 em dash (0x97) being there instead.
func TestPDFDoesNotEmitMojibake(t *testing.T) {
	tbl := reportTable{
		Title:    "Lecturer Attendance",
		Subtitle: "2 lecturer(s)",
		Headers:  []string{"Lecturer", "Cohort", "Sessions"},
		Rows: [][]string{
			// Every non-ASCII character these reports actually carry: the cohort separator, an
			// accented name, a curly apostrophe, and the arrow from a patrol remark.
			{"Dr. Zoë Nakato", "BSc CS · Yr2 · Sem1", "12"},
			{"Prof. O’Brien", "Nursing – Evening", "7"},
			{"Mr. Okello", "moved LR3 → LR7", "3"},
		},
	}
	rec := httptest.NewRecorder()
	writeReportPDF(rec, "x.pdf", "Kampala International University", tbl)
	body := rec.Body.Bytes()
	drawn := pdfTextBytes(t, body)

	if !bytes.HasPrefix(body, []byte("%PDF")) {
		t.Fatalf("not a PDF (first bytes %q)", body[:min(8, len(body))])
	}
	// The mojibake itself: the raw UTF-8 bytes of an em dash. Present anywhere in the content
	// stream means something reached the font unconverted.
	for _, bad := range []struct {
		name  string
		bytes []byte
	}{
		{"em dash U+2014", []byte{0xE2, 0x80, 0x94}},
		{"en dash U+2013", []byte{0xE2, 0x80, 0x93}},
		{"middle dot U+00B7", []byte{0xC2, 0xB7}},
		{"right quote U+2019", []byte{0xE2, 0x80, 0x99}},
	} {
		if bytes.Contains(drawn, bad.bytes) {
			t.Errorf("%s reached the PDF as raw UTF-8 (% x) — it will render as mojibake", bad.name, bad.bytes)
		}
	}
}

// The conversion itself, away from the PDF plumbing: the characters cp1252 CAN carry must survive
// as their single cp1252 byte, and the ones it cannot must become something a reader can act on
// rather than fpdf's bare '.'.
func TestPDFEncoderMapsWhatItCanAndSubstitutesWhatItCannot(t *testing.T) {
	enc := pdfEncoder(fpdf.New("P", "mm", "A4", ""))
	for _, c := range []struct {
		in   string
		want string
		why  string
	}{
		{"—", "\x97", "em dash is cp1252 0x97"},
		{"–", "\x96", "en dash is cp1252 0x96"},
		{"·", "\xb7", "middle dot is cp1252 0xb7 — the cohort separator"},
		{"’", "\x92", "curly apostrophe, in every O’Brien"},
		{"é", "\xe9", "accented letters must survive, not be approximated"},
		{"→", "->", "cp1252 has no arrow; '.' would read as a typo"},
		{"≥", ">=", "same for the comparison operators"},
		{"Plain ASCII", "Plain ASCII", "untouched"},
	} {
		if got := enc(c.in); got != c.want {
			t.Errorf("enc(%q) = % x, want % x — %s", c.in, got, c.want, c.why)
		}
	}
}

// Excel is what these are opened in, and it reads a downloaded .csv as the system codepage unless
// the bytes open with a BOM. Asserted on every CSV the system serves.
func TestCSVStartsWithTheUTF8BOM(t *testing.T) {
	rec := httptest.NewRecorder()
	writeReportCSV(rec, "x.csv", sampleTable())
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte{0xEF, 0xBB, 0xBF}) {
		t.Errorf("CSV does not start with the UTF-8 BOM — Excel will render non-ASCII as mojibake")
	}
}

// The clipper trims a byte at a time. That is only safe because the text is converted to
// single-byte cp1252 first; on raw UTF-8 it could stop mid-rune and leave a broken character —
// the code that makes text fit manufacturing the corruption it was there to avoid.
func TestPDFClippingNeverSplitsACharacter(t *testing.T) {
	tbl := reportTable{
		Headers: []string{"N"},
		Weights: []float64{1},
		// Far wider than any column, and non-ASCII throughout, so the clipper must run.
		Rows: [][]string{{strings.Repeat("Zoë · Ökello — Nakato ", 40)}},
	}
	rec := httptest.NewRecorder()
	writeReportPDF(rec, "x.pdf", "KIU", tbl)
	drawn := pdfTextBytes(t, rec.Body.Bytes())
	for _, bad := range [][]byte{{0xE2, 0x80, 0x94}, {0xC2, 0xB7}, {0xC3, 0xB6}} {
		if bytes.Contains(drawn, bad) {
			t.Errorf("clipped cell left raw UTF-8 (% x) in the PDF", bad)
		}
	}
}
