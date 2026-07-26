import ug.qaat.crypto.Sealer
import ug.qaat.engine.AttendanceRecord
import ug.qaat.engine.SessionPackage

/** Build a real session package + seal it (the full coordinator close→upload path).
 *  args: <bindingKey> <sessionId> <coordinatorId> <studentHash> <fingerprint> <seq> */
fun main(args: Array<String>) {
    val (bindingKey, sessionId, coordinatorId) = args
    val record = AttendanceRecord(
        logId = java.util.UUID.randomUUID().toString(),
        sessionId = sessionId,
        studentIdHash = args[3],
        deviceFingerprintHash = args[4],
        sequenceNumber = args[5].toInt(),
        checkinTimestamp = "2026-06-29T10:00:00Z",
    )
    val pkgJson = SessionPackage.build(sessionId, coordinatorId, listOf(record), sealedAt = "2026-06-29T11:00:00Z")
    val sealed = Sealer.seal(bindingKey, pkgJson)
    fun esc(s: String) = s.replace("\\", "\\\\").replace("\"", "\\\"")
    println("{\"encryptedPayload\":\"${esc(sealed.encryptedPayload)}\",\"hmac\":\"${sealed.hmac}\",\"packageChecksum\":\"${sealed.packageChecksum}\",\"totalChunks\":${sealed.totalChunks}}")
}
