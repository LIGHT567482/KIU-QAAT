// Command pkgparse unmarshals a session-package JSON into the SAME struct the
// sync-receiver uses in writeAttendanceLogs, proving the Kotlin SessionPackage
// builder produces records the server parses identically.
// Usage: echo '<packageJson>' | go run ./cmd/pkgparse
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	data, _ := io.ReadAll(os.Stdin)
	var pkg struct {
		AttendanceRecords []struct {
			LogID                 string `json:"log_id"`
			SessionID             string `json:"session_id"`
			StudentIDHash         string `json:"student_id_hash"`
			DeviceFingerprintHash string `json:"device_fingerprint_hash"`
			SequenceNumber        int    `json:"sequence_number"`
			CheckinTimestamp      string `json:"checkin_timestamp"`
			EntryMethod           string `json:"entry_method"`
		} `json:"attendance_records"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		fmt.Println("PARSE_FAIL:", err)
		os.Exit(1)
	}
	fmt.Printf("records=%d\n", len(pkg.AttendanceRecords))
	for _, r := range pkg.AttendanceRecords {
		fmt.Printf("%s|%s|%s|%s|%d|%s|%s\n",
			r.LogID, r.SessionID, r.StudentIDHash, r.DeviceFingerprintHash,
			r.SequenceNumber, r.CheckinTimestamp, r.EntryMethod)
	}
}
