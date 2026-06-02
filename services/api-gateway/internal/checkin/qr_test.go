package checkin

import (
	"os"
	"strings"
	"testing"
)

func loadFixture(t *testing.T) (raw, pubPEM string) {
	t.Helper()
	rawB, err := os.ReadFile("testdata/qr_valid.json")
	if err != nil {
		t.Fatalf("read qr fixture: %v", err)
	}
	pubB, err := os.ReadFile("testdata/qr_pubkey.pem")
	if err != nil {
		t.Fatalf("read pubkey fixture: %v", err)
	}
	return string(rawB), string(pubB)
}

// TestVerifyRealSignature proves cross-language byte-exactness: the fixture was
// signed by node:crypto exactly as qr-generator does, and Go reconstructs the
// same signed bytes and verifies. This is the highest-risk part of the port.
func TestVerifyRealSignature(t *testing.T) {
	raw, pub := loadFixture(t)
	q, err := ParseQR(raw)
	if err != nil {
		t.Fatalf("ParseQR: %v", err)
	}
	if q.StudentID != "CS-2024-0001" || q.TenantID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("parsed fields wrong: %+v", q.QRPayload)
	}
	if err := q.VerifySignature(pub); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
}

// C1 regression: a tampered field must invalidate the signature.
func TestTamperedPayloadRejected(t *testing.T) {
	raw, pub := loadFixture(t)
	tampered := strings.Replace(raw, "CS-2024-0001", "CS-2024-9999", 1)
	if tampered == raw {
		t.Fatal("test setup: nothing replaced")
	}
	q, err := ParseQR(tampered)
	if err != nil {
		t.Fatalf("ParseQR: %v", err)
	}
	if err := q.VerifySignature(pub); err == nil {
		t.Fatal("tampered student_id passed signature verification (C1 regression)")
	}
}

func TestTamperedSignatureRejected(t *testing.T) {
	raw, pub := loadFixture(t)
	q, err := ParseQR(raw)
	if err != nil {
		t.Fatalf("ParseQR: %v", err)
	}
	// Flip a character in the signature.
	if q.Signature[0] == 'A' {
		q.Signature = "B" + q.Signature[1:]
	} else {
		q.Signature = "A" + q.Signature[1:]
	}
	if err := q.VerifySignature(pub); err == nil {
		t.Fatal("tampered signature passed verification")
	}
}

func TestParseQRRejectsMalformed(t *testing.T) {
	if _, err := ParseQR("not json"); err == nil {
		t.Fatal("malformed QR accepted")
	}
	if _, err := ParseQR(`{"student_id":"x"}`); err == nil {
		t.Fatal("QR without signature accepted")
	}
}
