package ug.qaat.engine

/**
 * Builds the plaintext session-package JSON that the sealer wraps and the Go
 * sync-receiver applies. The server reads ONLY `attendance_records[]` with these
 * exact snake_case keys (services/sync-receiver/internal/sync/receiver.go
 * writeAttendanceLogs), inserting `student_id_hash` into attendance_logs.student_id.
 * The other wrapper fields mirror apps/coordinator-pwa/src/sync/sealer.ts.
 */
object SessionPackage {
    fun build(
        sessionId: String,
        coordinatorId: String,
        records: List<AttendanceRecord>,
        sealedAt: String,
        unitId: String = "",        // lets the central sync-receiver create the session (phone-hub)
        sessionDate: String = "",
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
        return "{" +
            "\"session\":{\"session_id\":\"${esc(sessionId)}\",\"unit_id\":\"${esc(unitId)}\",\"session_date\":\"${esc(sessionDate)}\"}," +
            "\"attendance_records\":$recsJson," +
            "\"sealed_at\":\"${esc(sealedAt)}\"," +
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
