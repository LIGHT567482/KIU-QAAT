// Command parity decrypts a session package produced by the Kotlin crypto-core
// (apps/coordinator-android/crypto-core) using the REAL sync-receiver crypto,
// proving the Kotlin coordinator seal and the Go server decrypt are byte-compatible.
//
// Usage: go run ./cmd/parity <bindingKey> <encryptedPayload> <hmacHex> <checksumHex> <expectedPlaintext>
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/qaat/sync-receiver/internal/crypto"
)

func main() {
	if len(os.Args) != 6 {
		fmt.Println("usage: parity <bindingKey> <encryptedPayload> <hmacHex> <checksumHex> <expectedPlaintext>")
		os.Exit(64)
	}
	bindingKey, encPayload, hmacHex, checksumHex, expected := os.Args[1], os.Args[2], os.Args[3], os.Args[4], os.Args[5]

	// 1. Checksum parity: SHA-256 over the base64 payload text (as the receiver does).
	sum := sha256.Sum256([]byte(encPayload))
	if got := hex.EncodeToString(sum[:]); got != checksumHex {
		fmt.Printf("CHECKSUM_MISMATCH\n  kotlin=%s\n  go    =%s\n", checksumHex, got)
		os.Exit(1)
	}

	// 2. HMAC auth + AES-GCM decrypt with the device-derived keys (the real receiver path).
	pt, err := crypto.VerifyAndDecryptPackage(bindingKey, []byte(encPayload), hmacHex)
	if err != nil {
		fmt.Printf("DECRYPT/HMAC_FAIL: %v\n", err)
		os.Exit(1)
	}
	if string(pt) != expected {
		fmt.Printf("PLAINTEXT_MISMATCH\n  want=%s\n  got =%s\n", expected, string(pt))
		os.Exit(1)
	}
	fmt.Println("PARITY_OK: checksum match · HMAC verified · AES-GCM decrypted · plaintext identical")
}
