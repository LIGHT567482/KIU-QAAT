package checkin

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
)

// QRPayload is the signed body of a student's personal QR. The field order here
// MUST match services/qr-generator/src/crypto/rsa-keys.ts QRPayload, because the
// signature is computed over JSON.stringify(payload) and we re-serialise these
// fields to reconstruct the exact bytes that were signed.
type QRPayload struct {
	StudentID    string `json:"student_id"`
	TenantID     string `json:"tenant_id"`
	CourseID     string `json:"course_id"`
	FullName     string `json:"full_name"`
	AcademicYear string `json:"academic_year"`
	SerialNumber string `json:"serial_number"`
	ExpiryDate   string `json:"expiry_date"`
	IssuedAt     string `json:"issued_at"`
}

// SignedQR is the full payload encoded in the QR image: the signed body plus the
// detached RSA signature (base64) and tenant HMAC (hex).
type SignedQR struct {
	QRPayload
	Signature string `json:"signature"`
	HMAC      string `json:"hmac"`
}

// ParseQR decodes the raw QR JSON string a student submitted.
func ParseQR(raw string) (*SignedQR, error) {
	var q SignedQR
	if err := json.Unmarshal([]byte(raw), &q); err != nil {
		return nil, fmt.Errorf("malformed QR payload: %w", err)
	}
	if q.Signature == "" {
		return nil, errors.New("QR payload missing signature")
	}
	return &q, nil
}

// ParseQRToken decodes a base64url-encoded QR token (as embedded in the /checkin?t= URL).
func ParseQRToken(token string) (*SignedQR, error) {
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("invalid token encoding: %w", err)
	}
	return ParseQR(string(b))
}

// signedBody reconstructs the exact byte string that qr-generator signed:
// JSON.stringify of the six payload fields, in order, with no HTML escaping and
// no trailing newline (Go's json.Encoder appends one — we trim it).
func (q *SignedQR) signedBody() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // JSON.stringify does not escape < > &
	if err := enc.Encode(q.QRPayload); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// VerifySignature verifies the QR's RSA-2048 signature against the tenant's
// public key (SPKI PEM). This authenticates the QR's tenant_id claim: only the
// tenant holding the matching private key (in qr-generator) could have produced
// a valid signature, so a forged or cross-tenant QR fails here.
func (q *SignedQR) VerifySignature(publicKeyPEM string) error {
	pub, err := parsePublicKey(publicKeyPEM)
	if err != nil {
		return err
	}
	body, err := q.signedBody()
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(q.Signature)
	if err != nil {
		return fmt.Errorf("signature not valid base64: %w", err)
	}
	hashed := sha256.Sum256(body)
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, hashed[:], sig); err != nil {
		return errors.New("invalid signature")
	}
	return nil
}

func parsePublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("public key is not valid PEM")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not RSA")
	}
	return rsaKey, nil
}
