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

func TestFillEmptyKpisUsesCensusWhenNativeIsZero(t *testing.T) {
	c := Census{Students: 7, Lecturers: 4, Courses: 3, Units: 9, Sessions: 92, Lectures: 84}
	var students, lecturers, courses, units, held, planned int
	var taught float64
	FillEmptyKpis(&students, &lecturers, &courses, &units, &held, &planned, &taught, c)
	if students != 7 || lecturers != 4 || held != 92 || planned != 92 {
		t.Fatalf("got students=%d lecturers=%d held=%d planned=%d", students, lecturers, held, planned)
	}
	if taught <= 0 {
		t.Fatalf("taught rate should come from U-Panel sittings, got %v", taught)
	}
	students = 400
	FillEmptyKpis(&students, &lecturers, &courses, &units, &held, &planned, &taught, c)
	if students != 400 {
		t.Fatalf("must not overwrite a live registry count, got %d", students)
	}
}
