package handlers

// The seeded first-login passwords for the two roles that are created FOR people rather than BY
// them. A student is added by their reg-number and a lecturer by their staff ID; neither is ever
// asked to choose a password at that moment, so the system has to put a known one there.
//
// These used to be spelled as string literals in five places (admin student creation, the SIS
// import, the lazy login provisioning for each role, and migration 052). They are constants now
// because one of those five — creating a student from the admin dashboard — quietly used a RANDOM
// password instead, which nobody could ever type. The account existed, the reg-number resolved, and
// the login still failed with "invalid email or password". A shared constant is the only way that
// stays fixed.
//
// Every account seeded with one of these is created with force_password_change = true, so the
// default gets the person in ONCE and is replaced before they reach any role UI.
// The word is always the role, so there is nothing to look up and nothing to be told twice.
const (
	DefaultStudentPassword  = "student"
	DefaultLecturerPassword = "lecturer"
	// A QA patroller is created by an administrator, who previously had to invent a password
	// and then get it to the patroller somehow — by message, or on paper. That is both a worse
	// secret than a public default and a worse handover, since a forgotten one costs an admin
	// round trip. Same treatment as the other two: a known first-login word, force_password_change
	// set, replaced before the round will open.
	DefaultPatrollerPassword = "patroller"
)

// DefaultPasswordFor returns the seeded first-login password for a role that is created FOR
// someone rather than BY them, or "" for a role whose password the creator chooses.
func DefaultPasswordFor(role string) string {
	switch role {
	case "STUDENT":
		return DefaultStudentPassword
	case "LECTURER":
		return DefaultLecturerPassword
	case "QA_PATROLLER":
		return DefaultPatrollerPassword
	}
	return ""
}
