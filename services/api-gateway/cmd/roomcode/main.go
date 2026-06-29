// Command roomcode prints the server's room code for a secret at a fixed unix time,
// so the Kotlin engine's RoomCode port can be verified against it.
// Usage: go run ./cmd/roomcode <secret> <unixSeconds>
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/qaat/api-gateway/internal/checkin"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("usage: roomcode <secret> <unixSeconds>")
		os.Exit(64)
	}
	t, _ := strconv.ParseInt(os.Args[2], 10, 64)
	fmt.Print(checkin.Derive([]byte(os.Args[1]), time.Unix(t, 0)))
}
