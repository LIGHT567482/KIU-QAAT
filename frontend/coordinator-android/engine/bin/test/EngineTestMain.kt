import ug.qaat.engine.*
import ug.qaat.crypto.VaultCrypto
import java.time.Instant

/**
 * Off-device verification of the full check-in engine against a REAL node-signed QR.
 * args: <publicKeyPem> <rawSignedQrJson> <studentHashKey>
 * Exercises PRESENT + every rejection branch. Exit 0 iff all pass.
 */
fun main(args: Array<String>) {
    val pem = args[0]; val rawQr = args[1]; val hashKey = args[2]
    val f = FlatJson.parseQr(rawQr)!!.fields
    val studentHash = VaultCrypto.hmacHex(hashKey, f.studentId)
    val now = Instant.parse("2026-06-29T00:00:00Z").toEpochMilli()        // before the QR's 2027 expiry
    val afterExpiry = Instant.parse("2030-01-01T00:00:00Z").toEpochMilli()

    fun session(
        tenantId: String = f.tenantId,
        roster: Set<String> = setOf(studentHash),
        serials: Map<String, String> = mapOf(studentHash to f.serialNumber),
    ) = ActiveSession("sess1", tenantId, f.academicYear, pem, hashKey, roster, serials)

    var failures = 0
    fun check(name: String, got: ValidationResult, wantStatus: ValidationStatus, wantReason: RejectionReason? = null) {
        val ok = got.status == wantStatus && got.reason == wantReason
        println((if (ok) "PASS" else "FAIL") + "  $name -> ${got.status}${got.reason?.let { "/$it" } ?: ""}")
        if (!ok) { failures++; System.err.println("   expected $wantStatus${wantReason?.let { "/$it" } ?: ""}") }
    }
    fun validator(store: Store, clock: Long = now) =
        CheckinValidator(store, nowMillis = { clock }, nowIso = { "2026-06-29T00:00:00Z" }, newUuid = { "uuid-fixed" })

    // 1. Happy path → PRESENT, and a record is written.
    val s1 = InMemoryStore()
    check("present", validator(s1).validate(rawQr, session(), DeviceContext("fpA")), ValidationStatus.PRESENT)
    if ((s1 as InMemoryStore).all().size != 1) { failures++; println("FAIL  record-not-written") } else println("PASS  record-written (seq=${s1.all()[0].sequenceNumber})")

    // 2. Duplicate scan on the same store → DUPLICATE_SCAN.
    check("duplicate", validator(s1).validate(rawQr, session(), DeviceContext("fpA")), ValidationStatus.REJECTED, RejectionReason.DUPLICATE_SCAN)

    // 3. Tampered signature → INVALID_SIGNATURE.
    val tampered = rawQr.replaceFirst(f.serialNumber, f.serialNumber) // no-op; tamper the signature instead
        .let { val i = it.indexOf("\"signature\":\"") + 13; it.substring(0, i) + (if (it[i] == 'A') 'B' else 'A') + it.substring(i + 1) }
    check("bad-signature", validator(InMemoryStore()).validate(tampered, session(), DeviceContext("fpA")), ValidationStatus.REJECTED, RejectionReason.INVALID_SIGNATURE)

    // 4. Wrong tenant → TENANT_MISMATCH.
    check("tenant-mismatch", validator(InMemoryStore()).validate(rawQr, session(tenantId = "someone-else"), DeviceContext("fpA")), ValidationStatus.REJECTED, RejectionReason.TENANT_MISMATCH)

    // 5. Not on roster → NOT_ON_ROSTER.
    check("not-on-roster", validator(InMemoryStore()).validate(rawQr, session(roster = emptySet()), DeviceContext("fpA")), ValidationStatus.REJECTED, RejectionReason.NOT_ON_ROSTER)

    // 6. Superseded serial → SERIAL_REVOKED.
    check("serial-revoked", validator(InMemoryStore()).validate(rawQr, session(serials = mapOf(studentHash to "OLD-SERIAL")), DeviceContext("fpA")), ValidationStatus.REJECTED, RejectionReason.SERIAL_REVOKED)

    // 7. Device already bound to THIS student with a different fingerprint → DEVICE_MISMATCH.
    val s7 = InMemoryStore().apply { putBinding(DeviceBinding(studentHash, "fpOriginal", f.academicYear, "t")) }
    check("device-mismatch", validator(s7).validate(rawQr, session(), DeviceContext("fpA")), ValidationStatus.REJECTED, RejectionReason.DEVICE_MISMATCH)

    // 8. Fingerprint already bound to ANOTHER student → DEVICE_BELONGS_TO_ANOTHER_STUDENT.
    val s8 = InMemoryStore().apply { putBinding(DeviceBinding("other-student-hash", "fpA", f.academicYear, "t")) }
    check("device-belongs-other", validator(s8).validate(rawQr, session(), DeviceContext("fpA")), ValidationStatus.REJECTED, RejectionReason.DEVICE_BELONGS_TO_ANOTHER_STUDENT)

    // 9. Device already recorded a different student this session → DEVICE_ALREADY_USED.
    val s9 = InMemoryStore().apply { addAttendance(AttendanceRecord("l", "sess1", "another-hash", "fpA", 1, "t")) }
    check("device-already-used", validator(s9).validate(rawQr, session(), DeviceContext("fpA")), ValidationStatus.REJECTED, RejectionReason.DEVICE_ALREADY_USED)

    // 10. Expired QR → QR_EXPIRED.
    check("expired", validator(InMemoryStore(), clock = afterExpiry).validate(rawQr, session(), DeviceContext("fpA")), ValidationStatus.REJECTED, RejectionReason.QR_EXPIRED)

    println(if (failures == 0) "\nALL_ENGINE_TESTS_PASSED" else "\n$failures TEST(S) FAILED")
    if (failures != 0) kotlin.system.exitProcess(1)
}
