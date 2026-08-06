package handlers

// What a lecturer is allowed to read in a patrol alert.
//
// Two rules, and they pull in opposite directions, which is why they are pinned by tests rather
// than left to whoever edits the format string next:
//
//   - the alert must say WHICH lecture — a lecturer teaching one unit to three cohorts cannot act
//     on "you were recorded as NOT TAUGHT for Data Structures"
//   - the alert must NOT say WHO recorded it — the observation belongs to quality assurance, and
//     naming the patroller turns a record into one colleague accusing another
//
// The second rule is the fragile one. It is satisfied by an absence, and an absence is exactly
// what a later edit restores without noticing.

import (
	"strings"
	"testing"
)

func TestLectureWhen(t *testing.T) {
	cases := []struct {
		name, date, start, want string
	}{
		{"date and time", "2026-08-06", "14:00", "Thu 6 Aug 2026 at 14:00"},
		{"time only", "", "14:00", "at 14:00"},
		// A tick can arrive with a date the phone wrote in some other shape. Show it rather
		// than dropping the one field that says which day is meant.
		{"unparseable date is shown as given", "06/08/2026", "09:00", "06/08/2026 at 09:00"},
		{"date only", "2026-08-06", "", "Thu 6 Aug 2026"},
		{"neither", "", "", ""},
		{"whitespace is not a value", "   ", "  ", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := lectureWhen(c.date, c.start); got != c.want {
				t.Errorf("lectureWhen(%q, %q) = %q, want %q", c.date, c.start, got, c.want)
			}
		})
	}
}

// The patroller's name must not survive anywhere in a moved-lecture alert — not in the subject,
// not in the body, not in the note the patroller typed being attributed to them by name.
func TestVenueChangeMessageNamesNoPatroller(t *testing.T) {
	l := patrolLogIn{
		UnitName: "Data Structures", CourseCode: "BCS1201",
		Room: "LR3", SessionDate: "2026-08-06", ScheduledTime: "14:00",
		FoundVenue: "LR7", VenueChanged: true,
		Remarks: "Moved because the projector failed",
	}
	subject, body := venueChangeMessage(l)

	for _, forbidden := range []string{"Jane Doe", "patroller ", "Observed by"} {
		if strings.Contains(subject+"\n"+body, forbidden) {
			t.Errorf("alert leaks the observer (%q):\nsubject: %s\nbody: %s", forbidden, subject, body)
		}
	}
	// … and it still says which lecture, in both places.
	if !strings.Contains(subject, "14:00") {
		t.Errorf("subject does not say when: %s", subject)
	}
	for _, want := range []string{"Thu 6 Aug 2026 at 14:00", "LR3 → LR7", "Moved because the projector failed"} {
		if !strings.Contains(body, want) {
			t.Errorf("body is missing %q:\n%s", want, body)
		}
	}
}

// A tick with nothing but the timetabled slot must still produce a sentence — no bare "—",
// no dangling "at", no empty parentheses.
func TestVenueChangeMessageWithSparseFields(t *testing.T) {
	subject, body := venueChangeMessage(patrolLogIn{UnitName: "Data Structures", VenueChanged: true})
	if strings.Contains(subject, "—") {
		t.Errorf("subject has an empty time clause: %s", subject)
	}
	if !strings.Contains(body, "details differ from the published timetable") {
		t.Errorf("body does not fall back to a readable phrase:\n%s", body)
	}
	if strings.Contains(body, "()") || strings.Contains(body, " at .") {
		t.Errorf("body has an empty clause:\n%s", body)
	}
}
