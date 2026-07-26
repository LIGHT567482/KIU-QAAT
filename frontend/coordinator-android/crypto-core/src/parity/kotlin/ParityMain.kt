import ug.qaat.crypto.Sealer
import ug.qaat.crypto.VaultCrypto

/**
 * Parity harness (run on the JVM here): seal a known payload with a known binding
 * key and print the package as JSON. The Go verifier (services/sync-receiver/cmd/parity)
 * then decrypts it with the REAL sync-receiver crypto — proving the Kotlin coordinator
 * seal and the Go server decrypt are byte-compatible.
 *
 * args: <bindingKey> <plaintext>
 */
fun main(args: Array<String>) {
    val bindingKey = if (args.size > 0) args[0] else "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
    val plaintext = if (args.size > 1) args[1] else """{"session":{"id":"s1"},"attendance_records":[{"student_id":"NUT/CS/2024/001"}],"sealed_at":"2026-06-29T00:00:00Z","coordinator_id":"c1","package_version":"1.0"}"""

    val pkg = Sealer.seal(bindingKey, plaintext)

    // Self round-trip sanity (decrypt with the same derived key).
    val keys = VaultCrypto.deriveKeys(bindingKey)
    val rt = VaultCrypto.decrypt(keys.aesKey, pkg.encryptedPayload)
    if (rt != plaintext) { System.err.println("SELF_ROUNDTRIP_FAIL"); kotlin.system.exitProcess(2) }

    // Emit as simple JSON for the Go side to consume.
    fun esc(s: String) = s.replace("\\", "\\\\").replace("\"", "\\\"")
    println(
        "{\"encryptedPayload\":\"${esc(pkg.encryptedPayload)}\"," +
        "\"hmac\":\"${pkg.hmac}\"," +
        "\"packageChecksum\":\"${pkg.packageChecksum}\"," +
        "\"totalChunks\":${pkg.totalChunks}," +
        "\"plaintext\":\"${esc(plaintext)}\"}"
    )
}
