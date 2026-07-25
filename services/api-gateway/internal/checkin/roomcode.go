// Package checkin implements the online student check-in: rotating room codes
// (this file) and signed-QR verification (qr.go).
//
// The rotating room code is the proximity proof that replaces per-student BLE
// RSSI (which is impossible to read from a student's browser — iOS Safari has
// no Web Bluetooth). The coordinator displays a 6-digit code on the projector
// that changes every StepSeconds; a student must read it off the screen and
// submit it, which proves they are physically in the room. The code is an
// HMAC-based TOTP (RFC 6238 style) keyed by a per-session secret that never
// leaves the server.
package checkin

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"time"
)

const (
	// StepSeconds is how often the room code rotates. 10s is short enough that a
	// relayed code is stale before a remote confederate could use it, but long
	// enough to read off a projector and type from the back of a lecture hall.
	// The lecturer gate reuses this same code as its physical-presence digit code.
	StepSeconds = 10
	// Digits is the length of the displayed code.
	Digits = 6
	// skewSteps is how many steps on each side of "now" are accepted, to absorb
	// clock skew and the few seconds between a student reading the code and the
	// request landing. ±1 step → up to ~45s acceptance window.
	skewSteps = 1
)

var pow10 = [...]uint32{1, 10, 100, 1000, 10000, 100000, 1000000, 10000000}

// Derive returns the room code for the given secret at time t. The same secret
// + time-step always yields the same code, so the coordinator's display and the
// server's validation agree without the secret ever being shared with students.
func Derive(secret []byte, t time.Time) string {
	step := uint64(t.Unix() / StepSeconds)
	return deriveStep(secret, step)
}

func deriveStep(secret []byte, step uint64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], step)

	mac := hmac.New(sha256.New, secret)
	mac.Write(msg[:])
	sum := mac.Sum(nil)

	// Dynamic truncation (RFC 4226 §5.3).
	offset := sum[len(sum)-1] & 0x0f
	bin := (uint32(sum[offset]&0x7f) << 24) |
		(uint32(sum[offset+1]) << 16) |
		(uint32(sum[offset+2]) << 8) |
		uint32(sum[offset+3])

	code := bin % pow10[Digits]
	return zeroPad(code)
}

// Validate reports whether code is a valid room code for secret at time now,
// accepting ±skewSteps around the current step. Comparison is constant-time.
func Validate(secret []byte, code string, now time.Time) bool {
	if len(code) != Digits {
		return false
	}
	current := now.Unix() / StepSeconds
	for d := int64(-skewSteps); d <= skewSteps; d++ {
		candidate := deriveStep(secret, uint64(current+d))
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

// StaticCode returns a per-session code that does NOT rotate — used for STUDENTS.
// A student's proximity is proven by being connected to the coordinator's LAN
// hotspot (mandatory) plus their own device-bound QR, so their code need only
// identify the room; it stays constant for the whole session (easier to display
// and type). Derived deterministically from the session secret (salted with a
// fixed label so it never collides with a rotating step), so it needs no storage
// and is still effectively-random 6 digits, not guessable.
func StaticCode(secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte("student-static-v1"))
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	bin := (uint32(sum[offset]&0x7f) << 24) |
		(uint32(sum[offset+1]) << 16) |
		(uint32(sum[offset+2]) << 8) |
		uint32(sum[offset+3])
	return zeroPad(bin % pow10[Digits])
}

// ValidateStatic reports whether code equals the session's static student code
// (constant-time). The lecturer keeps the rotating Validate() above.
func ValidateStatic(secret []byte, code string) bool {
	if len(code) != Digits {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(StaticCode(secret)), []byte(code)) == 1
}

// SecondsRemaining returns how many seconds until the current code rotates.
// Used by the coordinator's display to render a countdown.
func SecondsRemaining(now time.Time) int {
	return StepSeconds - int(now.Unix()%StepSeconds)
}

func zeroPad(n uint32) string {
	buf := make([]byte, Digits)
	for i := Digits - 1; i >= 0; i-- {
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf)
}
