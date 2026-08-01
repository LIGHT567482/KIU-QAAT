package handlers

// Unit tests for the QA-rep workbook parsing. These need no database: they cover the value
// conversions that decide whether a real, human-filled monitoring sheet is understood or rejected —
// the part most likely to differ between "works on my CSV" and "works on the file the rep uploads".

import (
	"testing"
	"time"
)

func TestParseTaught(t *testing.T) {
	yes := []string{"YES", "yes", "Y", "true", "T", "1", "Taught", "present", " attended ", "✓", "x"}
	no := []string{"NO", "no", "N", "false", "F", "0", "Not Taught", "not_taught", "absent", "missed", "no show"}
	for _, s := range yes {
		if got, ok := parseTaught(s); !ok || !got {
			t.Errorf("parseTaught(%q) = (%v, %v), want (true, true)", s, got, ok)
		}
	}
	for _, s := range no {
		if got, ok := parseTaught(s); !ok || got {
			t.Errorf("parseTaught(%q) = (%v, %v), want (false, true)", s, got, ok)
		}
	}
	// Anything else must be refused rather than guessed — a misread here silently falsifies
	// a lecturer's teaching record.
	for _, s := range []string{"", "maybe", "n/a", "-", "?"} {
		if _, ok := parseTaught(s); ok {
			t.Errorf("parseTaught(%q) was accepted; want rejected", s)
		}
	}
}

func TestParseSheetDate(t *testing.T) {
	want := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	for _, s := range []string{"2026-07-06", "2026/07/06", "06/07/2026", "06-07-2026", "6 July 2026", "06 Jul 2026"} {
		got, ok := parseSheetDate(s)
		if !ok || !got.Equal(want) {
			t.Errorf("parseSheetDate(%q) = (%s, %v), want %s", s, got.Format("2006-01-02"), ok, want.Format("2006-01-02"))
		}
	}
	// Excel hands over a formatted date cell as a serial number, not text. 46209 = 2026-07-06.
	if got, ok := parseSheetDate("46209"); !ok || !got.Equal(want) {
		t.Errorf("parseSheetDate(excel serial) = (%s, %v), want %s", got.Format("2006-01-02"), ok, want.Format("2006-01-02"))
	}
	for _, s := range []string{"", "not a date", "13"} {
		if _, ok := parseSheetDate(s); ok {
			t.Errorf("parseSheetDate(%q) was accepted; want rejected", s)
		}
	}
}

func TestParseSheetTime(t *testing.T) {
	cases := map[string]string{
		"08:00":    "08:00",
		"8:00":     "08:00",
		"08:00:00": "08:00",
		"14:30":    "14:30",
		"2:30PM":   "14:30",
		"2:30 pm":  "14:30",
		// Excel stores a time-of-day as the fraction of a day: 1/3 of 24h = 08:00.
		"0.3333333333333333": "08:00",
		"0.5":                "12:00",
		// A datetime serial keeps its fractional part.
		"46209.5": "12:00",
		// Blank must round-trip blank: it is part of an observation's identity, so turning it
		// into "00:00" would make a re-upload miss the row it meant to correct.
		"": "",
	}
	for in, want := range cases {
		if got := parseSheetTime(in); got != want {
			t.Errorf("parseSheetTime(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeHeader(t *testing.T) {
	cases := map[string]string{
		"unit_id": "unit_id", "Unit ID": "unit_id", "unit-id": "unit_id", "Unit Code": "unit_id", "CODE": "unit_id",
		"Lecturer Staff ID": "lecturer_staff_id", "staff_no": "lecturer_staff_id", "lecturer_id": "lecturer_staff_id",
		"Lecturer": "lecturer_name", "Venue": "room", "Room No": "room",
		"Session Date": "date", "Start Time": "time", "Was Taught": "taught", "Taught?": "taught",
		"Comments": "remarks", "unknown column": "unknown_column",
	}
	for in, want := range cases {
		if got := normalizeHeader(in); got != want {
			t.Errorf("normalizeHeader(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeRoomCode(t *testing.T) {
	// The point of normalising is that "lr 101" and "LR 101" cannot become two different rooms.
	for _, in := range []string{"lr 101", "  LR   101 ", "Lr 101"} {
		if got := normalizeRoomCode(in); got != "LR 101" {
			t.Errorf("normalizeRoomCode(%q) = %q, want %q", in, got, "LR 101")
		}
	}
	if got := normalizeRoomCode("LR-101"); got != "LR-101" {
		t.Errorf("normalizeRoomCode kept separators wrong: %q", got)
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		`../../etc/passwd`:    "passwd",
		`C:\Users\me\qa.xlsx`: "qa.xlsx",
		"qa\"; drop\n.xlsx":   "qa drop.xlsx", // quote, semicolon and newline stripped
		"july report.xlsx":    "july report.xlsx",
	}
	for in, want := range cases {
		if got := sanitizeFilename(in); got != want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestQAWorkbookRoundTrip proves the template this service hands out is a workbook it can read
// back — the exact loop a rep performs (download template → fill in → upload).
func TestQAWorkbookRoundTrip(t *testing.T) {
	filled := [][]string{
		qaTemplateHeader,
		{"CS201", "Data Communication", "BSC-CS", "KIU/044", "Dr A Mwangi", "LR-101", "2026-07-06", "08:00", "YES", "all good"},
		{"CS305", "Compilers", "BSC-CS", "KIU/091", "B Nakato", "LR-204", "06/07/2026", "10:00", "no", "lecturer absent"},
	}
	blob, err := buildXLSX(filled)
	if err != nil {
		t.Fatalf("buildXLSX: %v", err)
	}
	if !looksXLSX(blob) {
		t.Fatal("buildXLSX produced something readTabular would treat as CSV")
	}
	rows, err := readQAWorkbook(blob)
	if err != nil {
		t.Fatalf("readQAWorkbook: %v", err)
	}
	if len(rows) != len(filled) {
		t.Fatalf("round trip produced %d rows, want %d", len(rows), len(filled))
	}

	col := map[string]int{}
	for i, h := range rows[0] {
		col[normalizeHeader(h)] = i
	}
	for _, name := range []string{"unit_id", "lecturer_staff_id", "date", "time", "taught", "remarks"} {
		if _, ok := col[name]; !ok {
			t.Fatalf("column %q went missing in the round trip; header was %v", name, rows[0])
		}
	}

	// Row 2 is the one written in day-first order and lower case — both must still read.
	get := func(r int, c string) string { return rows[r][col[c]] }
	if got := get(1, "unit_id"); got != "CS201" {
		t.Errorf("unit_id = %q, want CS201", got)
	}
	if taught, ok := parseTaught(get(1, "taught")); !ok || !taught {
		t.Errorf("row 1 taught = (%v,%v), want taught", taught, ok)
	}
	if taught, ok := parseTaught(get(2, "taught")); !ok || taught {
		t.Errorf("row 2 taught = (%v,%v), want not taught", taught, ok)
	}
	d1, ok1 := parseSheetDate(get(1, "date"))
	d2, ok2 := parseSheetDate(get(2, "date"))
	if !ok1 || !ok2 || !d1.Equal(d2) {
		t.Errorf("the two date spellings disagreed: %v/%v vs %v/%v", d1, ok1, d2, ok2)
	}
	if got := get(2, "remarks"); got != "lecturer absent" {
		t.Errorf("remarks = %q, want %q", got, "lecturer absent")
	}
}

// TestReadQAWorkbookCSV covers the rep who saves as CSV instead of .xlsx.
func TestReadQAWorkbookCSV(t *testing.T) {
	csv := "unit_id,date,time,taught\nCS201,2026-07-06,08:00,YES\n"
	rows, err := readQAWorkbook([]byte(csv))
	if err != nil {
		t.Fatalf("readQAWorkbook(csv): %v", err)
	}
	if len(rows) != 2 || rows[1][0] != "CS201" {
		t.Fatalf("csv parsed as %v", rows)
	}
	if _, err := readQAWorkbook([]byte{}); err == nil {
		t.Error("an empty upload was accepted; want an error")
	}
}

func TestQAScopeSQL(t *testing.T) {
	dept := qaScope{ScopeKind: "DEPARTMENT", Department: "Computer Science", School: "SCI"}
	clause, val, has := dept.scopeSQL("c.department", "c.school", 2)
	if !has || val != "Computer Science" || clause == "" {
		t.Errorf("department scope = (%q, %q, %v)", clause, val, has)
	}

	school := qaScope{ScopeKind: "SCHOOL", Department: "Computer Science", School: "SCI"}
	clause, val, has = school.scopeSQL("c.department", "c.school", 2)
	if !has || val != "SCI" {
		t.Errorf("school scope = (%q, %q, %v)", clause, val, has)
	}

	// An oversight role is unscoped and adds no filter at all.
	if clause, _, has = (qaScope{Unscoped: true}).scopeSQL("c.department", "c.school", 2); clause != "" || has {
		t.Errorf("unscoped role produced a filter: %q", clause)
	}

	// A scoped role whose org unit was never set must match NOTHING. Falling through to an
	// unfiltered query here would show one department's rep the whole institution.
	clause, _, has = (qaScope{ScopeKind: "DEPARTMENT"}).scopeSQL("c.department", "c.school", 2)
	if has || clause != " AND false" {
		t.Errorf("scope with no department = (%q, %v), want a match-nothing filter", clause, has)
	}
	if (qaScope{ScopeKind: "DEPARTMENT"}).noScopeMessage() == "" {
		t.Error("a rep with no department got no explanation of the empty page")
	}
}

func TestStartOfWeek(t *testing.T) {
	// Sunday must belong to the week that started six days earlier, not the one about to begin.
	sunday := time.Date(2026, 7, 12, 15, 0, 0, 0, time.UTC)
	if got := startOfWeek(sunday); got.Format("2006-01-02") != "2026-07-06" {
		t.Errorf("startOfWeek(Sunday) = %s, want 2026-07-06", got.Format("2006-01-02"))
	}
	monday := time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)
	if got := startOfWeek(monday); got.Format("2006-01-02") != "2026-07-06" {
		t.Errorf("startOfWeek(Monday) = %s, want 2026-07-06", got.Format("2006-01-02"))
	}
}
