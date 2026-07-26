// Command seal produces a sealed session package (base64 payload, HMAC, checksum) from
// a binding key + plaintext on stdin — for end-to-end sync tests. Prints 3 lines.
// Usage: echo '<plaintextJson>' | go run ./cmd/seal <bindingKey>
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/qaat/sync-receiver/internal/crypto"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("usage: seal <bindingKey>  (plaintext on stdin)")
		os.Exit(64)
	}
	pt, _ := io.ReadAll(os.Stdin)
	ep, hm, cs, err := crypto.SealPackage(os.Args[1], pt)
	if err != nil {
		fmt.Println("ERR:", err)
		os.Exit(1)
	}
	fmt.Printf("%s\n%s\n%s\n", ep, hm, cs)
}
