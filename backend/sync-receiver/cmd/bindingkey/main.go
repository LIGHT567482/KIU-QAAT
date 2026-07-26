// Command bindingkey decrypts a coordinator's stored device_binding_key_enc to its
// plaintext key — dev/ops + sync end-to-end tests only (requires the master key).
// Usage: KEY_ENCRYPTION_KEY=<hex> go run ./cmd/bindingkey <device_binding_key_enc> <user_id>
package main

import (
	"fmt"
	"os"

	"github.com/qaat/sync-receiver/internal/crypto"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("usage: bindingkey <enc> <user_id>")
		os.Exit(64)
	}
	k, err := crypto.DecryptBindingKey(os.Args[1], os.Args[2])
	if err != nil {
		fmt.Println("ERR:", err)
		os.Exit(1)
	}
	fmt.Print(k)
}
