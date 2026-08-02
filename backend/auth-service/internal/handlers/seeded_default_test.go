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

		// A password the user has actually chosen is compared exactly, by the caller. Once the
		// forced change is done, this helper must never widen matching again.
		{"password already changed", "Student", false, "student", false},

		// The stored hash must really be a default. A user whose chosen password merely looks
		// like one is not opened by a different casing of it.
		{"hash is not a default", "studentX", true, "student", false},

		// One role's default never opens an account seeded with the other's.
		{"wrong word for this account", "Lecturer", true, "student", false},

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
