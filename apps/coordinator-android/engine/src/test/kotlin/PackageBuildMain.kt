import ug.qaat.engine.AttendanceRecord
import ug.qaat.engine.SessionPackage

/** Prints a session-package JSON built from sample records (for the Go contract parser).
 *  args: <sessionId> <coordinatorId> <studentHash1> <fp1> <seq1> [<studentHash2> <fp2> <seq2> ...] */
fun main(args: Array<String>) {
    val sessionId = args[0]; val coordinatorId = args[1]
    val records = mutableListOf<AttendanceRecord>()
    var i = 2; var n = 1
    while (i + 2 < args.size + 1 && i + 2 <= args.size) {
        records.add(
            AttendanceRecord(
                logId = "log-$n",
                sessionId = sessionId,
                studentIdHash = args[i],
                deviceFingerprintHash = args[i + 1],
                sequenceNumber = args[i + 2].toInt(),
                checkinTimestamp = "2026-06-29T10:0$n:00Z",
            )
        )
        i += 3; n++
    }
    print(SessionPackage.build(sessionId, coordinatorId, records, sealedAt = "2026-06-29T11:00:00Z"))
}
