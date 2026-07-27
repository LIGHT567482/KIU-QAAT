package ug.qaat.engine

/**
 * Builds the plaintext session-package JSON that the sealer wraps and the Go
 * sync-receiver applies. The server reads ONLY `attendance_records[]` with these
 * exact snake_case keys (services/sync-receiver/internal/sync/receiver.go
 * writeAttendanceLogs), inserting `student_id_hash` into attendance_logs.student_id.
 * The other wrapper fields mirror apps/coordinator-pwa/src/sync/sealer.ts.
 */
object SessionPackage {
    /**
     * The lecturer's physical-presence proof for this session (they scanned the gate to START, and
     * optionally to END). The sync-receiver seeds `lecturer_attendance_logs` from this — which both
     * shows the lecturer's attendance in the dashboards AND is what makes the session's student
     * attendance "verified" (the receiver rejects attendance for sessions with no lecturer scan).
     */
    data class LecturerScan(
        val lecturerId: String,          // the assigned staff ID (resolved to lecturer_id centrally)
        val scannedAt: String,           // START scan time (also gate_open_time)
        val fingerprintHash: String,
        val endedAt: String = "",        // END scan time (blank if the lecturer didn't scan to END)
        val endFingerprintHash: String = "",
    )

    fun build(
        sessionId: String,
        coordinatorId: String,
        records: List<AttendanceRecord>,
        sealedAt: String,
        unitId: String = "",        // lets the central sync-receiver create the session (phone-hub)
        sessionDate: String = "",
        lecturer: LecturerScan? = null,
        packageVersion: String = "1.0",
    ): String {
        val recsJson = records.joinToString(",", "[", "]") { r ->
            "{" +
                "\"log_id\":\"${esc(r.logId)}\"," +
                "\"session_id\":\"${esc(r.sessionId)}\"," +
                "\"student_id_hash\":\"${esc(r.studentIdHash)}\"," +
                "\"device_fingerprint_hash\":\"${esc(r.deviceFingerprintHash)}\"," +
                "\"sequence_number\":${r.sequenceNumber}," +
                "\"checkin_timestamp\":\"${esc(r.checkinTimestamp)}\"," +
                "\"entry_method\":\"${esc(r.entryMethod)}\"" +
                "}"
        }
        val lecturerJson = lecturer?.let { l ->
            ",\"lecturer\":{" +
                "\"lecturer_id\":\"${esc(l.lecturerId)}\"," +
                "\"scanned_at\":\"${esc(l.scannedAt)}\"," +
                "\"fingerprint_hash\":\"${esc(l.fingerprintHash)}\"," +
                "\"ended_at\":\"${esc(l.endedAt)}\"," +
                "\"end_fingerprint_hash\":\"${esc(l.endFingerprintHash)}\"" +
                "}"
        } ?: ""
        return "{" +
            "\"session\":{\"session_id\":\"${esc(sessionId)}\",\"unit_id\":\"${esc(unitId)}\",\"session_date\":\"${esc(sessionDate)}\"}," +
            "\"attendance_records\":$recsJson" +
            lecturerJson +                                   // ",\"lecturer\":{…}" or "" — no dangling comma
            ",\"sealed_at\":\"${esc(sealedAt)}\"," +
            "\"coordinator_id\":\"${esc(coordinatorId)}\"," +
            "\"package_version\":\"${esc(packageVersion)}\"" +
            "}"
    }

    private fun esc(s: String) = buildString {
        for (c in s) when (c) {
            '\\' -> append("\\\\"); '"' -> append("\\\"")
            '\n' -> append("\\n"); '\r' -> append("\\r"); '\t' -> append("\\t")
            else -> if (c < ' ') append("\\u%04x".format(c.code)) else append(c)
        }
    }
}
