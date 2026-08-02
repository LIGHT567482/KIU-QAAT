package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"regexp"
)

var hex64 = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// masterKey loads the shared key-encryption key (KEY_ENCRYPTION_KEY). Unlike the
// bcrypt-derived key used for TOTP secrets, this key is shared across services
// so that secrets which another service must read at rest — notably the device
// binding key consumed by sync-receiver — can be decrypted there too. It fails
// closed when the key is missing or malformed rather than using a default.
func masterKey() ([]byte, error) {
	h := os.Getenv("KEY_ENCRYPTION_KEY")
	if !hex64.MatchString(h) {
		return nil, fmt.Errorf("KEY_ENCRYPTION_KEY must be a 64-char hex string (32 bytes / AES-256)")
	}
	b, _ := hex.DecodeString(h)
	return b, nil
}

// EncryptWithMasterKey encrypts plaintext with AES-256-GCM under the shared
// master key, binding aad as additional authenticated data. Output format is
// base64(nonce || ciphertext || tag) — the same layout sync-receiver decodes.
func EncryptWithMasterKey(plaintext, aad string) (string, error) {
	key, err := masterKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), []byte(aad))
	return base64.StdEncoding.EncodeToString(ct), nil
}

// DecryptWithMasterKey reverses EncryptWithMasterKey.
func DecryptWithMasterKey(ciphertext, aad string) (string, error) {
	key, err := masterKey()
	if err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ct := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, ct, []byte(aad))
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// GenerateBindingKey returns a fresh 32-byte device binding secret as a 64-char
// hex string. The coordinator app feeds this into HKDF to derive its vault keys.
func GenerateBindingKey() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
