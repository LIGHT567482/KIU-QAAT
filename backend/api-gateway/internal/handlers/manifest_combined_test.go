package handlers

// WHEN THE COORDINATOR SEES A CODE FIELD AT ALL.
//
// A combined class — one lecturer, one room, one hour, several cohorts — happens sometimes. It is
// not the normal shape of a lecture, and the coordinator of an ordinary one must never be shown a
// box asking for three digits that nobody is going to read out. The phone cannot decide that for
// itself: its manifest carries only its own offering's slots, so it never sees the cohort next door.
// The server decides, by giving a slot a combined_class_key only when the lecture really is shared,
// and the phone treats "this slot has a key" as "show the field".
//
// So this file tests the one predicate that whole behaviour rests on, against a real database —
// it is SQL, and a mock would only be agreeing with my reading of it. Like the rest of the repo's
// integration tests it SKIPS unless a connection is supplied:
//
//	DB_URL='postgres://qaat:qaat@localhost:5434/qaat?sslmode=disable' go test ./internal/handlers/ -run Combined -v
//
// Everything it writes happens inside a transaction that is always rolled back, so it can be run
// against a working database without leaving anything behind.

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func combinedTx(t *testing.T) (pgx.Tx, string) {
	t.Helper()
	url := os.Getenv("DB_URL")
	if url == "" {
		t.Skip("set DB_URL to run the combined-class manifest test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil || pool.Ping(ctx) != nil {
		t.Skipf("database unavailable: %v", err)
	}
	t.Cleanup(pool.Close)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	// ALWAYS rolled back. The rows below are deliberately timetabled into a room and hour no real
	// lecture uses, but a test that writes to a live institution's timetable should not be trusted
	// to tidy up after itself either.
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })

	var tenantID string
	if err := tx.QueryRow(ctx,
		`SELECT tenant_id::text FROM tenants
		  WHERE tenant_id <> '00000000-0000-0000-0000-000000000000'
		  ORDER BY created_at LIMIT 1`).Scan(&tenantID); err != nil {
		t.Skipf("no institution in this database: %v", err)
	}
	return tx, tenantID
}

// fixture seeds one offering with one unit and one slot, and returns the offering id.
type slotSpec struct {
	unit     string
	room     string // "" = an unroomed slot
	lecturer *string
}

func seedOffering(t *testing.T, tx pgx.Tx, tenantID, suffix string, spec slotSpec) string {
	t.Helper()
	ctx := context.Background()

	var courseID string
	if err := tx.QueryRow(ctx,
		`SELECT course_id FROM courses WHERE tenant_id = $1 ORDER BY course_id LIMIT 1`,
		tenantID).Scan(&courseID); err != nil {
		t.Skipf("no course in this database: %v", err)
	}

	var offeringID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO course_offerings (tenant_id, course_id, session_type, study_year, semester, intake)
		 VALUES ($1, $2, 'Day', 1, 1, $3) RETURNING offering_id::text`,
		tenantID, courseID, "ZZTEST-"+suffix).Scan(&offeringID); err != nil {
		t.Fatalf("seed offering %s: %v", suffix, err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO course_units (unit_id, tenant_id, course_id, name)
		 VALUES ($1, $2, $3, $4)`,
		spec.unit, tenantID, courseID, "ZZ Test Unit "+suffix); err != nil {
		t.Fatalf("seed unit %s: %v", suffix, err)
	}
	// Saturday at 23:00 in a room no timetable uses — far away from anything real, so the EXISTS
	// below is answering about these rows and nothing else.
	if _, err := tx.Exec(ctx,
		`INSERT INTO timetable_slots
		     (tenant_id, offering_id, unit_id, day_of_week, start_time, duration_minutes, room, lecturer_id)
		 VALUES ($1, $2::uuid, $3, 6, '23:00', 60, $4, $5::uuid)`,
		tenantID, offeringID, spec.unit, spec.room, spec.lecturer); err != nil {
		t.Fatalf("seed slot %s: %v", suffix, err)
	}
	return offeringID
}

func seedLecturer(t *testing.T, tx pgx.Tx, tenantID, name string) *string {
	t.Helper()
	var id string
	if err := tx.QueryRow(context.Background(),
		`INSERT INTO lecturers (tenant_id, full_name) VALUES ($1, $2) RETURNING lecturer_id::text`,
		tenantID, name).Scan(&id); err != nil {
		t.Fatalf("seed lecturer: %v", err)
	}
	return &id
}

// slotsFor runs the query the manifest actually ships and returns what the phone would receive.
func slotsFor(t *testing.T, tx pgx.Tx, offeringID, tenantID string) []manifestSlot {
	t.Helper()
	rows, err := tx.Query(context.Background(), manifestSlotsQuery, offeringID, tenantID)
	if err != nil {
		t.Fatalf("manifest slots query: %v", err)
	}
	defer rows.Close()
	return scanManifestSlots(rows)
}

// THE CASE THE FEATURE EXISTS FOR. Two cohorts, different unit codes, one lecturer, one room, one
// hour. Both must come back with a key, and it must be the SAME key — that identical string is what
// makes one spoken number open both registers.
func TestCombinedClass_TwoCohortsShareOneKey(t *testing.T) {
	tx, tenantID := combinedTx(t)
	lecturer := seedLecturer(t, tx, tenantID, "ZZ Test Lecturer")

	offA := seedOffering(t, tx, tenantID, "A", slotSpec{unit: "ZZTEST-U1", room: "ZZ Test Block 1", lecturer: lecturer})
	offB := seedOffering(t, tx, tenantID, "B", slotSpec{unit: "ZZTEST-U2", room: "ZZ Test Block 1", lecturer: lecturer})

	a, b := slotsFor(t, tx, offA, tenantID), slotsFor(t, tx, offB, tenantID)
	if len(a) != 1 || len(b) != 1 {
		t.Fatalf("expected one slot per offering, got %d and %d", len(a), len(b))
	}
	if a[0].CombinedClassKey == "" || b[0].CombinedClassKey == "" {
		t.Fatalf("a shared lecture must carry a combined-class key; got %q and %q",
			a[0].CombinedClassKey, b[0].CombinedClassKey)
	}
	if a[0].CombinedClassKey != b[0].CombinedClassKey {
		t.Errorf("cohorts of one lecture derived different keys (%q vs %q) — one spoken code could "+
			"not open both registers", a[0].CombinedClassKey, b[0].CombinedClassKey)
	}
	if a[0].LecturerID == "" {
		t.Error("the slot must carry the lecturer id; without it the phone cannot derive anything")
	}
}

// THE CASE THAT IS NEARLY EVERY LECTURE. One cohort alone in a room. No key, therefore no derived
// code, therefore no code field on the coordinator's screen.
func TestCombinedClass_OrdinaryLectureCarriesNoKey(t *testing.T) {
	tx, tenantID := combinedTx(t)
	lecturer := seedLecturer(t, tx, tenantID, "ZZ Test Lecturer")

	off := seedOffering(t, tx, tenantID, "SOLO", slotSpec{unit: "ZZTEST-U3", room: "ZZ Test Block 9", lecturer: lecturer})

	got := slotsFor(t, tx, off, tenantID)
	if len(got) != 1 {
		t.Fatalf("expected one slot, got %d", len(got))
	}
	if got[0].CombinedClassKey != "" {
		t.Errorf("an unshared lecture must carry no key, got %q — this is what would put a "+
			"code-entry field in front of every coordinator all day", got[0].CombinedClassKey)
	}
}

// An unroomed lecture cannot be a combined class: there is no room to share. It must get no key
// even if something else happens to sit at the same hour.
func TestCombinedClass_UnroomedSlotCarriesNoKey(t *testing.T) {
	tx, tenantID := combinedTx(t)
	lecturer := seedLecturer(t, tx, tenantID, "ZZ Test Lecturer")

	off := seedOffering(t, tx, tenantID, "NOROOM", slotSpec{unit: "ZZTEST-U4", room: "", lecturer: lecturer})

	got := slotsFor(t, tx, off, tenantID)
	if len(got) != 1 {
		t.Fatalf("expected one slot, got %d", len(got))
	}
	if got[0].CombinedClassKey != "" {
		t.Errorf("an unroomed slot must carry no key, got %q", got[0].CombinedClassKey)
	}
}

// Two bookings of one room at one hour with NOBODY assigned are not a combined class — they are two
// unassigned bookings, which is the thing migration 091 was written about. Migration 099 still
// permits them (both lecturers are NULL, so its "the lecturers differ" test is not met), so the
// manifest has to exclude them itself. It does, because NULL is never equal to NULL.
func TestCombinedClass_UnassignedBookingsAreNotACombinedClass(t *testing.T) {
	tx, tenantID := combinedTx(t)

	offA := seedOffering(t, tx, tenantID, "NOBODY1", slotSpec{unit: "ZZTEST-U5", room: "ZZ Test Block 2", lecturer: nil})
	seedOffering(t, tx, tenantID, "NOBODY2", slotSpec{unit: "ZZTEST-U6", room: "ZZ Test Block 2", lecturer: nil})

	got := slotsFor(t, tx, offA, tenantID)
	if len(got) != 1 {
		t.Fatalf("expected one slot, got %d", len(got))
	}
	if got[0].CombinedClassKey != "" {
		t.Errorf("two unassigned bookings must not read as one lecture, got key %q", got[0].CombinedClassKey)
	}
}

// The room is free text somebody types. "ZZ Test Block 1" and " zz test block 1 " are one room, and
// if the match were case- or space-sensitive the two cohorts would look like separate lectures and
// neither would get a key.
func TestCombinedClass_RoomMatchIgnoresCaseAndPadding(t *testing.T) {
	tx, tenantID := combinedTx(t)
	lecturer := seedLecturer(t, tx, tenantID, "ZZ Test Lecturer")

	offA := seedOffering(t, tx, tenantID, "CASE1", slotSpec{unit: "ZZTEST-U7", room: "ZZ Test Block 3", lecturer: lecturer})
	offB := seedOffering(t, tx, tenantID, "CASE2", slotSpec{unit: "ZZTEST-U8", room: "  zz test BLOCK 3 ", lecturer: lecturer})

	a, b := slotsFor(t, tx, offA, tenantID), slotsFor(t, tx, offB, tenantID)
	if len(a) != 1 || len(b) != 1 {
		t.Fatalf("expected one slot per offering, got %d and %d", len(a), len(b))
	}
	if a[0].CombinedClassKey == "" || b[0].CombinedClassKey == "" {
		t.Fatalf("a room typed two ways is still one room; got keys %q and %q",
			a[0].CombinedClassKey, b[0].CombinedClassKey)
	}
	if a[0].CombinedClassKey != b[0].CombinedClassKey {
		t.Errorf("the same room typed differently derived different keys (%q vs %q) — the cohorts "+
			"would each wait for a code the other never says", a[0].CombinedClassKey, b[0].CombinedClassKey)
	}
}
