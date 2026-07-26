import ug.qaat.crypto.QrVerify
import ug.qaat.crypto.VaultCrypto

/**
 * Verifies the Kotlin QR-signature + roster-hash primitives against vectors produced
 * by Node (the qr-generator scheme). args: <publicKeyPem> <body> <signatureB64>
 *   <student_id> <tenant_id> <course_id> <full_name> <academic_year> <serial_number>
 *   <expiry_date> <issued_at> <rosterKey> <rosterId> <expectedRosterHash>
 */
fun main(args: Array<String>) {
    val pem = args[0]; val body = args[1]; val sig = args[2]
    val p = QrVerify.QrPayload(args[3], args[4], args[5], args[6], args[7], args[8], args[9], args[10])
    val rosterKey = args[11]; val rosterId = args[12]; val expectedHash = args[13]

    var ok = true

    // 1. Verify the Node signature over the Node body.
    if (!QrVerify.verify(pem, body, sig)) { System.err.println("RSA_VERIFY_FAIL (node body)"); ok = false }
    else println("RSA_VERIFY_OK: SHA256withRSA signature verified")

    // 2. Canonicalization parity: Kotlin rebuilds the same body Node signed.
    val rebuilt = QrVerify.canonicalBody(p)
    if (rebuilt != body) { System.err.println("CANONICAL_BODY_MISMATCH\n  node=$body\n  kt  =$rebuilt"); ok = false }
    else println("CANONICAL_BODY_OK: Kotlin rebuilds Node's exact JSON body")

    // 3. Same signature verifies via the rebuilt payload path.
    if (!QrVerify.verifyPayload(pem, p, sig)) { System.err.println("RSA_VERIFY_PAYLOAD_FAIL"); ok = false }
    else println("RSA_VERIFY_PAYLOAD_OK")

    // 4. Roster hash parity: HMAC-SHA256(key, regNo) hex matches the server.
    val h = VaultCrypto.hmacHex(rosterKey, rosterId)
    if (h != expectedHash) { System.err.println("ROSTER_HASH_MISMATCH\n  want=$expectedHash\n  got =$h"); ok = false }
    else println("ROSTER_HASH_OK: HMAC-SHA256 student-id hash matches the manifest scheme")

    if (!ok) kotlin.system.exitProcess(1)
    println("ALL_QR_PARITY_OK")
}
