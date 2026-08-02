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
const (
	DefaultStudentPassword  = "student"
	DefaultLecturerPassword = "lecturer"
)
