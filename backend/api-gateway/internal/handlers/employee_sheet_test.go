package handlers

// Parsing tests for the biometric terminal's daily export. No database: these cover the value
// conversions that decide whether the real file HR uploads is understood or quietly mangled —
// which is the part most likely to differ between "works on my CSV" and "works on theirs".

import (
	"strings"
	"testing"
)

// The header row of the actual export, verbatim.
const sampleHeader = `Emp No.,AC-No.,No.,Name,Auto-Assign,Date,Timetable,On duty,Off duty,Clock In,Clock Out,Normal,Real time,Late,Early,Absent,OT Time,Work Time,Exception,Must C/In,Must C/Out,Department,NDays,WeekEnd,Holiday,ATT_Time,NDays_OT,WeekEnd_OT,Holiday_OT`

func TestSheetHeadersMapToColumns(t *testing.T) {
	got := map[string]bool{}
	for _, h := range strings.Split(sampleHeader, ",") {
		got[normaliseSheetHeader(h)] = true
	}
	// Every column the report or the alerts actually read must resolve. A header that falls
	// through unmapped becomes a silently empty column rather than an error.
	for _, want := range []string{
		"emp_no", "ac_no", "seq_no", "full_name", "auto_assign", "work_date", "timetable",
		"on_duty", "off_duty", "clock_in", "clock_out", "normal", "real_time", "late", "early",
		"absent", "ot_time", "work_time", "exception", "must_cin", "must_cout", "department",
		"ndays", "weekend", "holiday", "att_time", "ndays_ot", "weekend_ot", "holiday_ot",
	} {
		if !got[want] {
			t.Errorf("column %q did not map from the export's header row", want)
		}
	}
}

func TestLooksLikeDaySheet(t *testing.T) {
	if !looksLikeDaySheet(strings.Split(sampleHeader, ",")) {
		t.Error("the real export was not recognised as a day sheet")
	}
	// The older punch upload must NOT be mistaken for it — importing one as the other stores
	// the wrong shape without complaining.
	punch := []string{"staff_id", "title", "full_name", "datetime", "in/out", "comment"}
	if looksLikeDaySheet(punch) {
		t.Error("the punch export was misread as a day sheet")
	}
}

func TestParseSheetDayIsDayFirst(t *testing.T) {
	// 27/07/2026 is 27 July, not an invalid month. Getting this backwards silently files a
	// month of attendance under the wrong dates.
	got, ok := parseSheetDay("27/07/2026")
	if !ok {
		t.Fatal("27/07/2026 was rejected")
	}
	if got.Day() != 27 || int(got.Month()) != 7 || got.Year() != 2026 {
		t.Errorf("27/07/2026 parsed as %s, want 2026-07-27", got.Format("2006-01-02"))
	}
	// An unambiguous day-first date must not be read month-first.
	if d, ok := parseSheetDay("31/07/2026"); !ok || d.Day() != 31 {
		t.Errorf("31/07/2026 parsed as %v (ok=%v)", d, ok)
	}
	for _, bad := range []string{"", "not a date", "13"} {
		if _, ok := parseSheetDay(bad); ok {
			t.Errorf("parseSheetDay(%q) was accepted", bad)
		}
	}
}

func TestSheetBoolTreatsBlankAsFalse(t *testing.T) {
	// The export spells "no" as an empty cell — Must C/In is "True" or nothing at all.
	for _, yes := range []string{"True", "true", "TRUE", "yes", "1"} {
		if !sheetBool(yes) {
			t.Errorf("sheetBool(%q) = false, want true", yes)
		}
	}
	for _, no := range []string{"", "  ", "False", "no", "0"} {
		if sheetBool(no) {
			t.Errorf("sheetBool(%q) = true, want false", no)
		}
	}
}

func TestSheetNumKeepsBlankDistinctFromZero(t *testing.T) {
	// "no overtime recorded" and "zero overtime" are different claims, and the export
	// distinguishes them by leaving the cell empty.
	if sheetNum("") != nil {
		t.Error("a blank numeric cell became a value; want NULL")
	}
	if v := sheetNum("1.97"); v == nil || *v != 1.97 {
		t.Errorf("sheetNum(\"1.97\") = %v, want 1.97", v)
	}
	if sheetNum("not a number") != nil {
		t.Error("an unparseable numeric cell became a value; want NULL")
	}
}

// The late/early derivation, which is what the alerts fire on. The terminal's own Late and Early
// columns are blank in most of the sample rows even where the times clearly differ, which is why
// this is computed rather than trusted.
func TestLateAndEarlyDerivation(t *testing.T) {
	cases := []struct {
		name                  string
		clockIn, onDuty       string
		clockOut, offDuty     string
		wantLate, wantEarlyOK bool
	}{
		// From the sample: 07:53 in against 08:00 on-duty is EARLY arrival, not late.
		{"arrived before on-duty", "07:53", "08:00", "18:58", "17:00", false, false},
		// 08:21 against 08:00 is late; 17:00 off-duty with no clock-out is not "early".
		{"arrived after on-duty", "08:21", "08:00", "", "17:00", true, false},
		// 17:17 against 17:00 is a normal finish.
		{"left after off-duty", "07:55", "08:00", "17:17", "17:00", false, false},
		// A genuine early departure.
		{"left before off-duty", "07:55", "08:00", "16:30", "17:00", false, true},
		// A day with no punches at all is ABSENT — calling it "late" would be a different
		// and wrong accusation.
		{"no punches", "", "08:00", "", "17:00", false, false},
	}
	for _, c := range cases {
		late := false
		if ci, ok1 := hhmmMinutes(c.clockIn); ok1 {
			if od, ok2 := hhmmMinutes(c.onDuty); ok2 {
				late = ci > od
			}
		}
		early := false
		if co, ok1 := hhmmMinutes(c.clockOut); ok1 {
			if od, ok2 := hhmmMinutes(c.offDuty); ok2 {
				early = co < od
			}
		}
		if late != c.wantLate {
			t.Errorf("%s: late = %v, want %v", c.name, late, c.wantLate)
		}
		if early != c.wantEarlyOK {
			t.Errorf("%s: early = %v, want %v", c.name, early, c.wantEarlyOK)
		}
	}
}

func TestHHMMMinutes(t *testing.T) {
	if m, ok := hhmmMinutes("08:00"); !ok || m != 480 {
		t.Errorf("hhmmMinutes(08:00) = (%d,%v), want (480,true)", m, ok)
	}
	if m, ok := hhmmMinutes("18:58"); !ok || m != 18*60+58 {
		t.Errorf("hhmmMinutes(18:58) = (%d,%v)", m, ok)
	}
	for _, bad := range []string{"", "  ", "8", "not a time"} {
		if _, ok := hhmmMinutes(bad); ok {
			t.Errorf("hhmmMinutes(%q) was accepted", bad)
		}
	}
}
