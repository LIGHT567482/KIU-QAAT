package checkin

import "testing"

// The vectors below were produced by a THIRD implementation of the algorithm (a short Python
// script), independent of both this file and CombinedClassCode.kt. Pinning both languages against
// an outside answer is the point: if the Go and Kotlin sides were only compared with each other, a
// shared misreading of the spec would agree with itself and still be wrong in the hall.
//
// CombinedClassCodeTest.kt asserts these same four values. If either side drifts — a changed
// separator, a different truncation window, a UTC date — exactly one of the two suites goes red,
// which is the signal that the phone and the server have stopped agreeing.
var vectorKey = []byte("a-shared-session-package-key")

func TestCombinedClassCode_MatchesIndependentVectors(t *testing.T) {
	cases := []struct {
		lecturer, class, date, want string
	}{
		{"lect-0001", "combined-7f3a", "2026-08-13", "946"},
		{"lect-0001", "combined-7f3a", "2026-08-14", "582"}, // next day, unrelated number
		{"lect-0002", "combined-7f3a", "2026-08-13", "809"}, // different lecturer
		{"lect-0001", "combined-9999", "2026-08-13", "188"}, // different class
	}
	for _, c := range cases {
		if got := CombinedClassCode(vectorKey, c.lecturer, c.class, c.date); got != c.want {
			t.Errorf("CombinedClassCode(%s, %s, %s) = %s, want %s (Go has drifted from the phones)",
				c.lecturer, c.class, c.date, got, c.want)
		}
	}
}

func TestCombinedClassCode_YesterdaysCodeIsRefusedToday(t *testing.T) {
	yesterday := CombinedClassCode(vectorKey, "lect-0001", "combined-7f3a", "2026-08-13")
	if !ValidateCombinedClassCode(vectorKey, yesterday, "lect-0001", "combined-7f3a", "2026-08-13") {
		t.Fatal("today's code must open today's register")
	}
	if ValidateCombinedClassCode(vectorKey, yesterday, "lect-0001", "combined-7f3a", "2026-08-14") {
		t.Error("yesterday's code opened tomorrow's register — the day binding is not applied")
	}
}

// A code is never checked in the abstract: it is checked against the class the coordinator is
// running. Another lecture's code must not open this one, which is what makes three digits safe.
func TestCombinedClassCode_AnotherLecturesCodeIsRefused(t *testing.T) {
	other := CombinedClassCode(vectorKey, "lect-0002", "combined-7f3a", "2026-08-13")
	if ValidateCombinedClassCode(vectorKey, other, "lect-0001", "combined-7f3a", "2026-08-13") &&
		other != CombinedClassCode(vectorKey, "lect-0001", "combined-7f3a", "2026-08-13") {
		t.Error("a different lecturer's code opened this register")
	}
}

func TestCombinedClassCode_RejectsWrongLength(t *testing.T) {
	for _, bad := range []string{"", "94", "9466", "  "} {
		if ValidateCombinedClassCode(vectorKey, bad, "lect-0001", "combined-7f3a", "2026-08-13") {
			t.Errorf("accepted %q as a code", bad)
		}
	}
}

// Every cohort of one combined lecture must derive the SAME key, or the spoken code opens only the
// register it was generated on — which is the entire problem this feature exists to solve.
func TestCombinedClassKey_IsStableAcrossTypedRoomSpelling(t *testing.T) {
	a := CombinedClassKey("Block C 101", 2, "14:00")
	b := CombinedClassKey("  block c 101 ", 2, "14:00")
	if a != b {
		t.Errorf("room spelling changed the class key: %q vs %q", a, b)
	}
	if CombinedClassKey("Block C 101", 2, "14:00") == CombinedClassKey("Block C 101", 3, "14:00") {
		t.Error("a different day produced the same class key")
	}
	if CombinedClassKey("Block C 101", 2, "14:00") == CombinedClassKey("Block C 102", 2, "14:00") {
		t.Error("a different room produced the same class key")
	}
}
