package handlers

import (
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/qaat/auth-service/internal/models"
)

// Students and lecturers do not choose a password when their account is made — they are added by
// registration number and staff ID respectively, and the system seeds a known first-login one that
// they must replace immediately. That word has been written down in two casings over the life of
// the system ("Student" when the seeding migrations ran, "student" in the docs handed to staff and
// in everything created since), so a perfectly correct default was being typed and rejected.
//
// seededDefaults lists every casing that has ever been seeded, grouped by the word people are told
// to type. Order matters only for speed.
var seededDefaults = map[string][]string{
	"student":  {"student", "Student"},
	"lecturer": {"lecturer", "Lecturer"},
}

// matchesSeededDefault reports whether `submitted` is a case-variant of the account's OWN, still
// untouched, seeded default password.
//
// This is deliberately narrow, and each condition is load-bearing:
//
//   - The account must still be flagged force_password_change. The instant the user picks a real
//     password that flag clears, and from then on their password is compared exactly like anyone
//     else's. Nothing here relaxes matching for a chosen password.
//   - The stored hash must itself verify against one of the known seeded spellings. That is proof
//     the account is genuinely sitting on a factory default rather than on a user's choice that
//     happens to resemble one.
//   - Only the word the account was actually seeded with is considered — "student" never opens an
//     account seeded with "lecturer".
//
// So the only extra password ever accepted is a different capitalisation of a password that is
// public knowledge anyway, on an account that cannot reach any role UI before changing it.
func matchesSeededDefault(user *models.User, submitted string) bool {
	if !user.ForcePasswordChange || submitted == "" {
		return false
	}
	variants, ok := seededDefaults[strings.ToLower(submitted)]
	if !ok {
		return false
	}
	for _, v := range variants {
		if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(v)) == nil {
			return true
		}
	}
	return false
}
