package checkin

import (
	"testing"
	"time"
)

var testSecret = []byte("a-32-byte-test-secret-for-totp!!")

func TestDeriveDeterministicAndSixDigits(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	a := Derive(testSecret, now)
	b := Derive(testSecret, now)
	if a != b {
		t.Fatalf("Derive not deterministic: %q != %q", a, b)
	}
	if len(a) != Digits {
		t.Fatalf("code length = %d, want %d (code=%q)", len(a), Digits, a)
	}
	for _, c := range a {
		if c < '0' || c > '9' {
			t.Fatalf("code %q has non-digit %q", a, c)
		}
	}
}

func TestCodeRotatesAcrossSteps(t *testing.T) {
	base := time.Unix(1_700_000_000-(1_700_000_000%StepSeconds), 0) // step boundary
	a := Derive(testSecret, base)
	next := Derive(testSecret, base.Add(StepSeconds*time.Second))
	if a == next {
		t.Fatalf("code did not rotate across a step boundary: %q", a)
	}
}

func TestValidateAcceptsCurrentAndSkew(t *testing.T) {
	now := time.Unix(1_700_000_123, 0)
	code := Derive(testSecret, now)

	if !Validate(testSecret, code, now) {
		t.Fatal("current code rejected")
	}
	// Code read one step ago should still validate (±1 skew).
	if !Validate(testSecret, code, now.Add(StepSeconds*time.Second)) {
		t.Fatal("code one step old rejected (skew window should accept it)")
	}
	if !Validate(testSecret, code, now.Add(-StepSeconds*time.Second)) {
		t.Fatal("code one step future rejected (skew window should accept it)")
	}
}

func TestValidateRejectsStaleCode(t *testing.T) {
	now := time.Unix(1_700_000_123, 0)
	code := Derive(testSecret, now)
	// Two steps later is outside the ±1 window.
	stale := now.Add(2 * StepSeconds * time.Second)
	if Validate(testSecret, code, stale) {
		t.Fatal("stale code (2 steps old) was accepted — replay window too wide")
	}
}

func TestValidateRejectsWrongSecretAndShape(t *testing.T) {
	now := time.Unix(1_700_000_123, 0)
	code := Derive(testSecret, now)
	other := []byte("a-different-32-byte-secret-value!")
	if Validate(other, code, now) {
		t.Fatal("code validated under the wrong secret")
	}
	if Validate(testSecret, "12345", now) {
		t.Fatal("5-digit code accepted")
	}
	if Validate(testSecret, "1234567", now) {
		t.Fatal("7-digit code accepted")
	}
	if Validate(testSecret, "", now) {
		t.Fatal("empty code accepted")
	}
}

func TestSecondsRemaining(t *testing.T) {
	at := time.Unix(1_700_000_000-(1_700_000_000%StepSeconds), 0) // exact boundary
	if got := SecondsRemaining(at); got != StepSeconds {
		t.Fatalf("SecondsRemaining at boundary = %d, want %d", got, StepSeconds)
	}
	if got := SecondsRemaining(at.Add(time.Second)); got != StepSeconds-1 {
		t.Fatalf("SecondsRemaining 1s in = %d, want %d", got, StepSeconds-1)
	}
}
