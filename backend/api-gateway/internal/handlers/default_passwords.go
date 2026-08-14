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
	// A QA monitor is created by an administrator, who previously had to invent a password
	// and then get it to the monitor somehow — by message, or on paper. That is both a worse
	// secret than a public default and a worse handover, since a forgotten one costs an admin
	// round trip. Same treatment as the other two: a known first-login word, force_password_change
	// set, replaced before the round will open.
	//
	// The word follows the role's name, so it changed with it. Only accounts created FROM NOW ON
	// get it — a monitor seeded earlier still holds a hash of the old word and can still sign in
	// with it, which is the right outcome: renaming a role must not lock anybody out. Tell new
	// monitors "monitor"; anyone still holding an unused older account needs the old one.
	DefaultMonitorPassword = "monitor"

	// The oversight roles — ADMIN, VC, DVC, DEAN, HOD, TLC, the QA offices — are normally created
	// one at a time by an administrator who chooses a password on the spot. A BULK IMPORT has
	// nobody to do that: a file of eighty deans and heads of department cannot carry eighty
	// invented passwords, and it must not carry them in plaintext down a mailing list either.
	//
	// So imported accounts of those roles start on this one word, exactly as a student starts on
	// "student", with force_password_change set so it gets the person in ONCE and is replaced
	// before they reach any role UI.
	//
	// BE CLEAR ABOUT WHAT THIS COSTS. The word is public by design, so between the moment an
	// account is imported and the moment its owner first signs in, anyone who knows it can sign in
	// as them. That window is the price of bulk provisioning and it is why the import response
	// says so in as many words. Import shortly before you hand the accounts out, not months ahead.
	DefaultStaffPassword = "staff"
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
		return DefaultMonitorPassword
	}
	return ""
}

// ImportedPasswordFor is [DefaultPasswordFor] with a floor: every role gets a seeded first-login
// word, because a bulk import has no third option.
//
// The per-role word still WINS where there is one. A QA monitor arriving by import must be able to
// sign in with the same "monitor" everybody is told, rather than a second word that exists only
// because of how their account happened to be created — that difference would be invisible on
// screen and impossible for the person on the phone to them to diagnose.
func ImportedPasswordFor(role string) string {
	if p := DefaultPasswordFor(role); p != "" {
		return p
	}
	return DefaultStaffPassword
}
