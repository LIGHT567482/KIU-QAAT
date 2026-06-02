package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const totpIssuer = "QAAT"

// GenerateTOTPSecret creates a new TOTP key for a user.
// Returns the secret key and an otpauth:// URL for QR rendering.
func GenerateTOTPSecret(email string) (secret, otpauthURL string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: email,
		Period:      30,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return "", "", fmt.Errorf("generate totp key: %w", err)
	}
	return key.Secret(), key.URL(), nil
}

// ValidateTOTPCode verifies a 6-digit TOTP code against the given secret.
// Allows ±1 period to account for clock drift.
func ValidateTOTPCode(secret, code string) bool {
	return totp.Validate(code, secret)
}

// EncryptSecret encrypts a TOTP secret with AES-256-GCM using a passphrase
// derived from the user's bcrypt hash (already available at call site).
// Stored as base64(nonce + ciphertext).
func EncryptSecret(plaintext, passphrase string) (string, error) {
	key := deriveKey(passphrase)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// DecryptSecret reverses EncryptSecret.
func DecryptSecret(ciphertext, passphrase string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	key := deriveKey(passphrase)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(data) < ns {
		return "", fmt.Errorf("ciphertext too short")
	}
	pt, err := gcm.Open(nil, data[:ns], data[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(pt), nil
}

// GenerateBackupCodes returns 8 random 8-character alphanumeric codes.
func GenerateBackupCodes() ([]string, error) {
	const count = 8
	codes := make([]string, count)
	b := make([]byte, 6)
	for i := range codes {
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		codes[i] = fmt.Sprintf("%x", b)[:8]
	}
	return codes, nil
}

func deriveKey(passphrase string) []byte {
	h := sha256.Sum256([]byte(passphrase + "-qaat-totp-v1"))
	return h[:]
}

// TOTPValidUntil returns the timestamp of the current TOTP period's end,
// useful for displaying a countdown to the user.
func TOTPValidUntil() time.Time {
	now := time.Now().Unix()
	remaining := 30 - (now % 30)
	return time.Unix(now+remaining, 0)
}
