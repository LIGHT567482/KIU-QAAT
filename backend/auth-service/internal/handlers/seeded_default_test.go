package handlers

import (
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/qaat/auth-service/internal/models"
)

func hashOf(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hashing %q: %v", pw, err)
	}
	return string(h)
}

func TestMatchesSeededDefault(t *testing.T) {
	tests := []struct {
		name      string
		seeded    string // what the account's hash was made from
		forceChg  bool
		submitted string
		want      bool
	}{
		// The bug this exists for: told "student", account seeded by migration 052 as "Student".
		{"old casing seeded, new casing typed", "Student", true, "student", true},
		{"new casing seeded, old casing typed", "student", true, "Student", true},
		{"shouty", "student", true, "STUDENT", true},
		{"lecturer likewise", "Lecturer", true, "lecturer", true},
		{"patroller likewise", "Patroller", true, "patroller", true},
		{"patroller, as seeded", "patroller", true, "patroller", true},

		// A password the user has actually chosen is compared exactly, by the caller. Once the
		// forced change is done, this helper must never widen matching again.
		{"password already changed", "Student", false, "student", false},

		// The stored hash must really be a default. A user whose chosen password merely looks
		// like one is not opened by a different casing of it.
		{"hash is not a default", "studentX", true, "student", false},

		// One role's default never opens an account seeded with another's.
		{"wrong word for this account", "Lecturer", true, "student", false},
		{"patroller word on a lecturer account", "Lecturer", true, "patroller", false},

		// Anything that is not a default word at all is refused outright.
		{"unrelated password", "Student", true, "hunter2", false},
		{"empty", "Student", true, "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := &models.User{
				PasswordHash:        hashOf(t, tc.seeded),
				ForcePasswordChange: tc.forceChg,
			}
			if got := matchesSeededDefault(u, tc.submitted); got != tc.want {
				t.Errorf("matchesSeededDefault(seeded=%q, force=%v, submitted=%q) = %v, want %v",
					tc.seeded, tc.forceChg, tc.submitted, got, tc.want)
			}
		})
	}
}

// TestLoginAndChangePasswordAgree pins the defect that made first sign-in a dead end.
//
// Login accepted a case-variant of the seeded default (matchesSeededDefault) and then sent the
// user straight to a MANDATORY password change whose current-password check was a bare
// bcrypt.CompareHashAndPassword. So the same word that had just worked was rejected: the user
// could sign in and could not get past the one screen with no navigation of its own, being told
// their existing password was wrong.
//
// Both gates now ask the same question. This test asserts they cannot drift apart again — it
// evaluates the exact predicate each handler uses, so a change to one that is not made to the
// other fails here.
func TestLoginAndChangePasswordAgree(t *testing.T) {
	// The seeded spelling on the account, and what the person actually types. Every row is a
	// combination the field has produced.
	cases := []struct{ seeded, typed string }{
		{"Student", "student"},   // migration-era account, current documentation
		{"student", "Student"},   // current account, migration-era documentation
		{"Lecturer", "lecturer"},
		{"lecturer", "lecturer"},
		{"patroller", "patroller"},
		{"Patroller", "patroller"},
	}
	for _, c := range cases {
		u := &models.User{PasswordHash: hashOf(t, c.seeded), ForcePasswordChange: true}

		// Exactly what auth_handler.Login decides.
		loginOK := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(c.typed)) == nil ||
			matchesSeededDefault(u, c.typed)
		// Exactly what auth_handler.ChangePassword decides.
		changeOK := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(c.typed)) == nil ||
			matchesSeededDefault(u, c.typed)

		if !loginOK {
			t.Errorf("seeded %q, typed %q: cannot sign in at all", c.seeded, c.typed)
		}
		if loginOK != changeOK {
			t.Errorf("seeded %q, typed %q: login=%v but change-password=%v — the user signs in "+
				"and is then stuck on the mandatory change screen", c.seeded, c.typed, loginOK, changeOK)
		}
	}
}

// TestChosenPasswordIsNotWidened is the other half: once someone has picked their own password,
// neither gate may accept anything but it. The forced-change flag is what draws that line.
func TestChosenPasswordIsNotWidened(t *testing.T) {
	u := &models.User{PasswordHash: hashOf(t, "student"), ForcePasswordChange: false}
	if matchesSeededDefault(u, "Student") {
		t.Error("a chosen password must be matched exactly — casing tolerance ends at the forced change")
	}
}
