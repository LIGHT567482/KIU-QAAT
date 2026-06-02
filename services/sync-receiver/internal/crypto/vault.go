// Package crypto verifies and decrypts session packages uploaded by the
// Coordinator PWA. It mirrors, byte-for-byte, the WebCrypto operations the PWA
// performs in apps/coordinator-pwa/src/crypto/vault-crypto.ts and sealer.ts:
//
//   - The device binding key is a hex string issued by auth-service and stored
//     AES-256-GCM encrypted under the shared master key (AAD = user_id).
//   - From it, HKDF-SHA256 derives an AES-256-GCM vault key (info
//     "coordinator-vault-key") and an HMAC-SHA256 key (info
//     "coordinator-vault-hmac"), both with salt "QAAT-IndexedDB-Salt-v1".
//   - The PWA uploads base64(iv12 || ciphertext || tag) and an HMAC over that
//     base64 text. We authenticate the HMAC, then decrypt.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"regexp"

	"golang.org/x/crypto/hkdf"
)

var (
	hkdfSalt   = []byte("QAAT-IndexedDB-Salt-v1")
	hkdfAESInf = []byte("coordinator-vault-key")
	hkdfMACInf = []byte("coordinator-vault-hmac")
	hex64      = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
)

func masterKey() ([]byte, error) {
	h := os.Getenv("KEY_ENCRYPTION_KEY")
	if !hex64.MatchString(h) {
		return nil, fmt.Errorf("KEY_ENCRYPTION_KEY must be a 64-char hex string (32 bytes / AES-256)")
	}
	b, _ := hex.DecodeString(h)
	return b, nil
}

// DecryptBindingKey reverses auth-service's EncryptWithMasterKey: AES-256-GCM
// over base64(nonce || ciphertext || tag) with the user_id bound as AAD.
func DecryptBindingKey(ciphertext, userID string) (string, error) {
	key, err := masterKey()
	if err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode binding key: %w", err)
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
		return "", fmt.Errorf("binding key ciphertext too short")
	}
	nonce, ct := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, ct, []byte(userID))
	if err != nil {
		return "", fmt.Errorf("binding key auth failed: %w", err)
	}
	return string(pt), nil
}

func deriveVaultKeys(bindingKey string) (aesKey, hmacKey []byte, err error) {
	secret := []byte(bindingKey)
	aesKey = make([]byte, 32)
	if _, err = io.ReadFull(hkdf.New(sha256.New, secret, hkdfSalt, hkdfAESInf), aesKey); err != nil {
		return nil, nil, err
	}
	hmacKey = make([]byte, 32)
	if _, err = io.ReadFull(hkdf.New(sha256.New, secret, hkdfSalt, hkdfMACInf), hmacKey); err != nil {
		return nil, nil, err
	}
	return aesKey, hmacKey, nil
}

// VerifyAndDecryptPackage authenticates the uploaded ciphertext against the
// device-bound HMAC, then decrypts it to the plaintext session-package JSON.
// expectedHMACHex is the hex HMAC the PWA computed over the (base64) payload.
func VerifyAndDecryptPackage(bindingKey string, payload []byte, expectedHMACHex string) ([]byte, error) {
	aesKey, hmacKey, err := deriveVaultKeys(bindingKey)
	if err != nil {
		return nil, err
	}

	// Authenticate first (encrypt-then-MAC over the transmitted bytes).
	mac := hmac.New(sha256.New, hmacKey)
	mac.Write(payload)
	want, err := hex.DecodeString(expectedHMACHex)
	if err != nil || !hmac.Equal(want, mac.Sum(nil)) {
		return nil, fmt.Errorf("package HMAC verification failed")
	}

	// The transmitted payload is the base64 text of iv(12) || ciphertext || tag.
	combined, err := base64.StdEncoding.DecodeString(string(payload))
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(combined) < gcm.NonceSize() {
		return nil, fmt.Errorf("payload too short")
	}
	nonce, ct := combined[:gcm.NonceSize()], combined[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("package decryption failed: %w", err)
	}
	return plaintext, nil
}
