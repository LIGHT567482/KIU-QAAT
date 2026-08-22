package upanel

import "testing"

func TestAttendancePctAndDeficit(t *testing.T) {
	if AttendancePct(0, 0) != 0 {
		t.Fatal("empty")
	}
	if AttendancePct(4, 3) != 75 {
		t.Fatalf("got %v", AttendancePct(4, 3))
	}
	if AttendancePct(3, 2) != 66.7 {
		t.Fatalf("got %v", AttendancePct(3, 2))
	}
	if DeficitSessions(4, 2, 75) != 1 { // need ceil(0.75*4)=3, attended 2 → +1
		t.Fatalf("deficit %d", DeficitSessions(4, 2, 75))
	}
	if DeficitSessions(4, 3, 75) != 0 {
		t.Fatalf("at bar %d", DeficitSessions(4, 3, 75))
	}
}
