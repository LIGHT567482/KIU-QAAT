package checkin

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"strings"
)

// THE COMBINED-CLASS CODE, server side — a byte-for-byte port of
// engine/src/main/kotlin/ug/qaat/engine/CombinedClassCode.kt.
//
// One lecturer teaches several cohorts together; each cohort has its own coordinator running its
// own hotspot; a phone joins one hotspot at a time. So the lecturer gates in on whichever hotspot
// they joined and reads out three digits, and the other coordinators type them in to open their own
// registers. Every phone DERIVES the expected code rather than fetching one, so the handshake needs
// no network at all — which is essential, because those phones are not on the same Wi-Fi as each
// other and usually have no uplink.
//
// THIS FILE EXISTS SO THE SERVER AGREES WITH THE PHONES. The phones are the only thing that matters
// during the lecture; the server sees these codes later, when packages sync, and has to reach the
// same answer to validate what was recorded. Two implementations of one algorithm is exactly the
// arrangement that drifts silently — a different separator, a different truncation, a UTC date
// instead of a local one — and the failure would not appear until a hall full of students could not
// check in. The parity test pins both against vectors computed by a third, independent
// implementation, so neither side can drift without a red build.
//
// Read CombinedClassCode.kt for the reasoning about daily rotation and what "unique" honestly means
// with three digits; it is not repeated here, so the two cannot disagree in prose either.

const (
	// CombinedClassDigits is deliberately small: it is read aloud across a lecture hall.
	CombinedClassDigits = 3
	combinedClassMod    = 1000
	// Unit separator. Load-bearing: plain concatenation hashes ("ab","c") and ("a","bc") to the
	// same message, so two classes could derive one code by accident of where their ids split.
	// A control character cannot appear in an id, so no id can forge a different split.
	combinedClassSep    = "\x1f"
	combinedClassDomain = "combined-class-v1"
)

// CombinedClassCode returns the code for this lecturer and class on this LOCAL date.
//
// localDate must be yyyy-MM-dd in the institution's timezone, never UTC: a 22:00 Kampala lecture is
// already the next day in UTC, and deriving from UTC would roll the code over mid-lecture.
func CombinedClassCode(secret []byte, lecturerID, classKey, localDate string) string {
	msg := strings.Join([]string{combinedClassDomain, lecturerID, classKey, localDate}, combinedClassSep)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(msg))
	h := mac.Sum(nil)

	// RFC 4226 dynamic truncation: the offset comes from the low nibble of the last byte, so the
	// digits are drawn from a varying window of the hash rather than always its first bytes.
	off := int(h[len(h)-1] & 0x0F)
	bin := (uint32(h[off]&0x7F) << 24) |
		(uint32(h[off+1]) << 16) |
		(uint32(h[off+2]) << 8) |
		uint32(h[off+3])
	return fmt.Sprintf("%0*d", CombinedClassDigits, bin%combinedClassMod)
}

// ValidateCombinedClassCode reports whether a typed code is the one for this lecturer, class and
// day. There is NO grace for yesterday's code — the date is in the derivation precisely so a day's
// codes cannot carry into the next day's registers, and a grace window would undo that.
func ValidateCombinedClassCode(secret []byte, typed, lecturerID, classKey, localDate string) bool {
	clean := strings.TrimSpace(typed)
	if len(clean) != CombinedClassDigits {
		return false
	}
	want := CombinedClassCode(secret, lecturerID, classKey, localDate)
	return subtle.ConstantTimeCompare([]byte(want), []byte(clean)) == 1
}

// CombinedClassKey is the shared identity of one combined lecture, and it is DERIVED rather than
// stored. The institution's own rule is that a room holds units taught by one lecturer at one time,
// so that tuple already names the combined class exactly — every cohort sitting in it computes the
// same key from its own timetable, with no new column to populate and nothing to keep in step.
//
// Normalised because the room is free text somebody types: "Block C 101", "block c 101" and
// " Block C 101 " are one room, and if they were not folded together the cohorts would derive
// different keys and the spoken code would open nobody else's register.
func CombinedClassKey(room string, dayOfWeek int, startHHMM string) string {
	return strings.Join([]string{
		strings.ToLower(strings.TrimSpace(room)),
		fmt.Sprintf("%d", dayOfWeek),
		strings.TrimSpace(startHHMM),
	}, combinedClassSep)
}
