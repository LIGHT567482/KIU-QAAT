package ug.qaat.coordinator

import org.junit.Test
import ug.qaat.coordinator.server.InRoomServer
import ug.qaat.crypto.QrVerify
import ug.qaat.crypto.VaultCrypto
import ug.qaat.engine.*
import java.net.HttpURLConnection
import java.net.ServerSocket
import java.net.URL
import java.net.URLEncoder
import java.security.KeyPairGenerator
import java.security.Signature
import java.util.Base64
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * Headless end-to-end proof of the in-room student check-in over the SAME HTTP path the phone
 * hub uses (the real [InRoomServer] + [CheckinValidator] + a [Store]). Mirrors how
 * SessionController.open() wires a live session. Verifies:
 *   • a student's scanned QR (posted after joining the hotspot) is recorded as PRESENT;
 *   • the attendance row lands in the store — this is the "waiting to be synced" ledger the
 *     coordinator's app later uploads on close();
 *   • the coordinator's live-roster (onCheckin) row is produced for the on-screen list;
 *   • one-phone-one-person holds (duplicate + other-student-same-device are rejected);
 *   • the lecturer rotating-code gate opens the session (/gate?rc=…);
 *   • the card path /checkin?t=<token> is served (the URL printed on ID cards).
 *
 * Room persistence itself is the only piece not covered here (it needs on-device instrumentation);
 * the Room DAO is a thin adapter over the exact Store contract asserted below.
 */
class InRoomCheckinE2ETest {

    // A signed student QR built exactly like the backend/qr-generator (canonical body + RSA-SHA256),
    // plus the base64url `t` token that ID cards encode.
    private class Card(val rawQr: String, val token: String, val serial: String, val studentId: String, val fullName: String)

    private val kp = KeyPairGenerator.getInstance("RSA").apply { initialize(2048) }.generateKeyPair()
    private val pubPem = "-----BEGIN PUBLIC KEY-----\n" +
        Base64.getEncoder().encodeToString(kp.public.encoded) + "\n-----END PUBLIC KEY-----"
    private val hashKey = "test-student-hash-key"
    private val tenantId = "tenant-x"
    private val academicYear = "2026"

    private fun card(studentId: String, fullName: String, serial: String, tenant: String = tenantId): Card {
        val body = QrVerify.canonicalBody(
            QrVerify.QrPayload(studentId, tenant, "CS101", fullName, academicYear, serial, "2027-12-31", "2026-01-01T00:00:00Z")
        )
        val sig = Signature.getInstance("SHA256withRSA").apply { initSign(kp.private); update(body.toByteArray(Charsets.UTF_8)) }
        val sigB64 = Base64.getEncoder().encodeToString(sig.sign())
        val rawQr = body.dropLast(1) + ",\"signature\":\"$sigB64\"}"        // insert signature into the flat object
        val token = Base64.getUrlEncoder().withoutPadding().encodeToString(rawQr.toByteArray(Charsets.UTF_8))
        return Card(rawQr, token, serial, studentId, fullName)
    }

    private fun freePort(): Int = ServerSocket(0).use { it.localPort }

    private fun http(method: String, url: String, form: String? = null): Pair<Int, String> {
        val c = (URL(url).openConnection() as HttpURLConnection).apply {
            requestMethod = method
            connectTimeout = 3000; readTimeout = 3000
            if (form != null) {
                doOutput = true
                setRequestProperty("Content-Type", "application/x-www-form-urlencoded")
                outputStream.use { it.write(form.toByteArray(Charsets.UTF_8)) }
            }
        }
        val code = c.responseCode
        val text = (if (code in 200..299) c.inputStream else c.errorStream)?.bufferedReader()?.use { it.readText() } ?: ""
        return code to text
    }

    private fun form(vararg pairs: Pair<String, String>) =
        pairs.joinToString("&") { (k, v) -> "$k=${URLEncoder.encode(v, "UTF-8")}" }

    @Test
    fun student_checkin_records_attendance_pending_sync() {
        val alice = card("STU-ALICE", "Alice A.", "SER-ALICE-1")
        val bob = card("STU-BOB", "Bob B.", "SER-BOB-1")
        val aliceHash = VaultCrypto.hmacHex(hashKey, alice.studentId)
        val bobHash = VaultCrypto.hmacHex(hashKey, bob.studentId)

        val store = InMemoryStore()
        val validator = CheckinValidator(store)
        val session = ActiveSession(
            sessionId = "sess-e2e",
            tenantId = tenantId,
            academicYear = academicYear,
            institutionPublicKeyPem = pubPem,
            studentHashKey = hashKey,
            rosterHashes = setOf(aliceHash, bobHash),
            rosterSerials = mapOf(aliceHash to alice.serial, bobHash to bob.serial),
        )
        val secret = ByteArray(32).also { java.security.SecureRandom().nextBytes(it) }
        val staffId = "KIU/STAFF/001"
        var gateState = GateState.NOT_STARTED
        val displayRows = mutableListOf<Pair<String, String>>()   // (fullName, status)

        val server = InRoomServer(validator, "<html>ATTEND-PAGE</html>", "<html>GATE-PAGE</html>")
        server.setLive(
            InRoomServer.Live(
                session = session,
                roomCodeSecret = secret,
                gateContext = {
                    LecturerGateContext(
                        assignedStaffId = staffId, roomCodeSecret = secret, gateState = gateState,
                        attended = store.attendanceCount(session.sessionId), enrolled = 2, ratio = 0.5, requireBiometric = false,
                    )
                },
                onGate = { action, _ -> if (action == GateAction.START) gateState = GateState.STARTED else gateState = GateState.ENDED },
                lecturerStarted = { gateState == GateState.STARTED },
                onCheckin = { fields, result -> displayRows.add(fields.fullName to (result.reason?.name ?: result.status.name)) },
            )
        )

        val port = freePort()
        val engine = server.start(port)
        try {
            val base = "http://127.0.0.1:$port"
            // wait until the server is accepting connections
            var up = false
            repeat(50) { if (!up) runCatching { if (http("GET", "$base/attend").first == 200) up = true } ; if (!up) Thread.sleep(100) }
            assertTrue(up, "server did not come up")

            // Card URL path served (this is what a scanned ID card opens).
            val (checkinCode, checkinBody) = http("GET", "$base/checkin?t=${alice.token}")
            assertEquals(200, checkinCode)
            assertTrue(checkinBody.contains("ATTEND-PAGE"), "/checkin should serve the attend page")

            // The token decodes back to the exact signed QR (mirrors attend.html's b64url decode).
            val decoded = String(Base64.getUrlDecoder().decode(alice.token), Charsets.UTF_8)
            assertEquals(alice.rawQr, decoded, "card token must round-trip to the signed QR")

            // Before the lecturer starts, students are held off.
            val (_, preStart) = http("POST", "$base/submit", form("qr" to alice.rawQr, "fingerprint" to "fp-alice"))
            assertTrue(preStart.contains("LECTURER_NOT_STARTED"), "expected LECTURER_NOT_STARTED, got $preStart")

            // Lecturer gate: a wrong code is rejected; the live rotating code STARTS the session.
            val (_, badGate) = http("POST", "$base/gate", form("staff_id" to staffId, "room_code" to "000000", "fingerprint" to "fp-lect", "biometric_verified" to "false"))
            assertTrue(badGate.contains("REJECTED"), "wrong rotating code should be rejected, got $badGate")
            val liveCode = RoomCode.derive(secret, System.currentTimeMillis() / 1000)
            val (_, startBody) = http("POST", "$base/gate", form("staff_id" to staffId, "room_code" to liveCode, "fingerprint" to "fp-lect", "biometric_verified" to "false"))
            assertTrue(startBody.contains("STARTED"), "valid gate should START, got $startBody")

            // Student scans their QR → PRESENT, and the ledger row is written (pending sync).
            val (okCode, okBody) = http("POST", "$base/submit", form("qr" to alice.rawQr, "fingerprint" to "fp-alice"))
            assertEquals(200, okCode)
            assertTrue(okBody.contains("PRESENT"), "expected PRESENT, got $okBody")
            assertEquals(1, store.all().size, "attendance ledger should hold exactly one pending-sync row")
            assertEquals(aliceHash, store.all()[0].studentIdHash)
            assertEquals("fp-alice", store.all()[0].deviceFingerprintHash)
            assertEquals(session.sessionId, store.all()[0].sessionId)
            assertTrue(displayRows.any { it.first == "Alice A." && it.second == "PRESENT" }, "coordinator live-roster row missing: $displayRows")

            // One phone, one attendance: the same student on the same phone can't double-count…
            val (_, dupBody) = http("POST", "$base/submit", form("qr" to alice.rawQr, "fingerprint" to "fp-alice"))
            assertTrue(dupBody.contains("DUPLICATE_SCAN"), "expected DUPLICATE_SCAN, got $dupBody")
            // …nor can a DIFFERENT student reuse the same phone.
            val (_, sharedDevice) = http("POST", "$base/submit", form("qr" to bob.rawQr, "fingerprint" to "fp-alice"))
            assertTrue(sharedDevice.contains("DEVICE_"), "same phone, other student should be blocked, got $sharedDevice")

            // Ledger still holds only Alice's single row.
            assertEquals(1, store.all().size)

            // ── "Receive logs from ANY phone, as long as the QR is of THIS cohort" ──────────────
            // A DIFFERENT student on a DIFFERENT phone checks in fine — no pre-registration of the
            // phone is needed; the roster/serial/signature is what authorises the log.
            val (_, bobBody) = http("POST", "$base/submit", form("qr" to bob.rawQr, "fingerprint" to "fp-bob"))
            assertTrue(bobBody.contains("PRESENT"), "a second cohort phone should be accepted, got $bobBody")
            assertEquals(2, store.all().size, "both cohort students should now be in the ledger")

            // A QR for ANOTHER tenant/cohort is refused (cohort scoping).
            val foreign = card("STU-X", "X", "SER-X", tenant = "some-other-tenant")
            val (_, foreignBody) = http("POST", "$base/submit", form("qr" to foreign.rawQr, "fingerprint" to "fp-x"))
            assertTrue(foreignBody.contains("TENANT_MISMATCH"), "foreign-cohort QR must be rejected, got $foreignBody")

            // A valid signature for THIS tenant but a student NOT on the roster is refused.
            val stranger = card("STU-EVE", "Eve", "SER-EVE")
            val (_, strangerBody) = http("POST", "$base/submit", form("qr" to stranger.rawQr, "fingerprint" to "fp-eve"))
            assertTrue(strangerBody.contains("NOT_ON_ROSTER"), "off-roster QR must be rejected, got $strangerBody")

            assertEquals(2, store.all().size, "rejected scans must not touch the ledger")
            println("E2E OK — PRESENT recorded for 2 phones (seqs ${store.all().map { it.sequenceNumber }}), duplicates+shared-device blocked, foreign-cohort + off-roster refused, gate STARTED, card /checkin?t served.")
        } finally {
            engine.stop(100, 200)
        }
    }

    @Test
    fun regNumber_checkin_and_reachability_selftest() {
        val aliceReg = "STU-ALICE"; val bobReg = "STU-BOB"
        val aliceHash = VaultCrypto.hmacHex(hashKey, aliceReg)
        val bobHash = VaultCrypto.hmacHex(hashKey, bobReg)
        val store = InMemoryStore()
        val validator = CheckinValidator(store)
        val session = ActiveSession(
            sessionId = "sess-reg", tenantId = tenantId, academicYear = academicYear,
            institutionPublicKeyPem = pubPem, studentHashKey = hashKey,
            rosterHashes = setOf(aliceHash, bobHash),
            rosterSerials = mapOf(aliceHash to "SER-A", bobHash to "SER-B"),
        )
        val secret = ByteArray(32).also { java.security.SecureRandom().nextBytes(it) }
        val reached = java.util.concurrent.atomic.AtomicInteger(0)
        val server = InRoomServer(validator, "<html>ATTEND-PAGE</html>", "<html>GATE</html>",
            onClientReached = { reached.incrementAndGet() })
        server.setLive(
            InRoomServer.Live(
                session = session, roomCodeSecret = secret,
                gateContext = {
                    LecturerGateContext(
                        assignedStaffId = "KIU/STAFF/001", roomCodeSecret = secret, gateState = GateState.STARTED,
                        attended = store.attendanceCount(session.sessionId), enrolled = 2, ratio = 0.5, requireBiometric = false,
                    )
                },
                onGate = { _, _ -> }, lecturerStarted = { true },
            )
        )
        val port = freePort()
        val engine = server.start(port)
        try {
            val base = "http://127.0.0.1:$port"
            var up = false
            repeat(50) { if (!up) runCatching { if (http("GET", "$base/attend").first == 200) up = true }; if (!up) Thread.sleep(100) }
            assertTrue(up, "server did not come up")

            // Reachability self-test: fetching the check-in page ticks the client-reached counter.
            val before = reached.get()
            http("GET", "$base/attend")
            assertTrue(reached.get() > before, "GET /attend must tick the reachability self-test counter")

            // Reg-number check-in → PRESENT, ledger row written.
            val (okCode, okBody) = http("POST", "$base/checkin", form("reg_number" to aliceReg, "fingerprint" to "fp-alice"))
            assertEquals(200, okCode)
            assertTrue(okBody.contains("PRESENT"), "expected PRESENT, got $okBody")
            assertEquals(1, store.all().size)
            assertEquals(aliceHash, store.all()[0].studentIdHash)

            // Same reg again (a rejoin / auto-resubmit) → DUPLICATE_SCAN: cannot double-count even after
            // forgetting the Wi-Fi or clearing the browser (the coordinator phone remembers).
            val (_, dup) = http("POST", "$base/checkin", form("reg_number" to aliceReg, "fingerprint" to "fp-alice"))
            assertTrue(dup.contains("DUPLICATE_SCAN"), "expected DUPLICATE_SCAN, got $dup")

            // Different reg on the SAME phone → blocked (device already bound to Alice).
            val (_, shared) = http("POST", "$base/checkin", form("reg_number" to bobReg, "fingerprint" to "fp-alice"))
            assertTrue(shared.contains("DEVICE_"), "same phone, other student must be blocked, got $shared")

            // Bob on HIS OWN phone → PRESENT.
            val (_, bobBody) = http("POST", "$base/checkin", form("reg_number" to bobReg, "fingerprint" to "fp-bob"))
            assertTrue(bobBody.contains("PRESENT"), "second student on own phone should be PRESENT, got $bobBody")
            assertEquals(2, store.all().size)

            // Unknown reg-number → NOT_ON_ROSTER.
            val (_, stranger) = http("POST", "$base/checkin", form("reg_number" to "STU-NOBODY", "fingerprint" to "fp-z"))
            assertTrue(stranger.contains("NOT_ON_ROSTER"), "off-roster reg must be rejected, got $stranger")

            assertEquals(2, store.all().size, "rejected reg check-ins must not touch the ledger")
            println("REG E2E OK — reg-number PRESENT x2, duplicate + shared-device blocked, off-roster refused, self-test reached=${reached.get()}")
        } finally {
            engine.stop(100, 200)
        }
    }
}
