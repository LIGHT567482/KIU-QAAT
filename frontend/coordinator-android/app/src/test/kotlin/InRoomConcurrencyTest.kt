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
import java.util.Collections
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * A ROOM FULL OF PHONES, all scanning at once.
 *
 * The single-threaded [InRoomCheckinE2ETest] proves the check-in path is correct when requests
 * arrive one at a time. That is not how a lecture works: the coordinator opens the gate and forty
 * students scan within the same few seconds, over one phone's hotspot, against one embedded Ktor
 * server and one SQLite file.
 *
 * Two distinct things are asserted, because they fail differently:
 *
 *  1. **Throughput/robustness** — many DIFFERENT students at once must all be recorded, with no
 *     dropped connection, no 5xx and no lost ledger row.
 *  2. **Atomicity** — the same student scanning many times at once must be recorded EXACTLY ONCE.
 *     This is the one that bites: the validation chain reads ("has this student already checked
 *     in?") and then writes, and those two steps are not one atomic operation. Two requests
 *     interleaving between the read and the write both see "no" and both insert, and the student
 *     is counted twice — inflating attendance, which is the number the whole system exists to get
 *     right. A student double-tapping a slow page, or a phone retrying a request it thought had
 *     timed out, produces exactly this race in the field.
 *
 * The store below is synchronised, mirroring the real one: the app's [ug.qaat.coordinator.store.RoomStore]
 * writes to SQLite, which serialises writers. That deliberately isolates the question to the
 * VALIDATOR's read-then-write, rather than letting a toy store's thread-unsafety mask it.
 */
class InRoomConcurrencyTest {

    private class Card(val rawQr: String, val serial: String, val studentId: String, val fullName: String)

    private val kp = KeyPairGenerator.getInstance("RSA").apply { initialize(2048) }.generateKeyPair()
    private val pubPem = "-----BEGIN PUBLIC KEY-----\n" +
        Base64.getEncoder().encodeToString(kp.public.encoded) + "\n-----END PUBLIC KEY-----"
    private val hashKey = "test-student-hash-key"
    private val tenantId = "tenant-x"
    private val academicYear = "2026"

    private fun card(studentId: String, fullName: String, serial: String): Card {
        val body = QrVerify.canonicalBody(
            QrVerify.QrPayload(studentId, tenantId, "CS101", fullName, academicYear, serial, "2027-12-31", "2026-01-01T00:00:00Z")
        )
        val sig = Signature.getInstance("SHA256withRSA").apply { initSign(kp.private); update(body.toByteArray(Charsets.UTF_8)) }
        val rawQr = body.dropLast(1) + ",\"signature\":\"" + Base64.getEncoder().encodeToString(sig.sign()) + "\"}"
        return Card(rawQr, serial, studentId, fullName)
    }

    /** Thread-safe reference store — stands in for SQLite's serialised writes. */
    private class SyncStore : Store {
        private val bindings = Collections.synchronizedList(mutableListOf<DeviceBinding>())
        private val attendance = Collections.synchronizedList(mutableListOf<AttendanceRecord>())

        @Synchronized override fun bindingByFingerprint(fingerprintHash: String): DeviceBinding? =
            bindings.firstOrNull { it.fingerprintHash == fingerprintHash }
        @Synchronized override fun bindingByStudent(studentIdHash: String): DeviceBinding? =
            bindings.firstOrNull { it.studentIdHash == studentIdHash }
        @Synchronized override fun putBinding(binding: DeviceBinding) {
            bindings.removeAll { it.studentIdHash == binding.studentIdHash }; bindings.add(binding)
        }
        @Synchronized override fun hasAttendance(sessionId: String, studentIdHash: String): Boolean =
            attendance.any { it.sessionId == sessionId && it.studentIdHash == studentIdHash }
        @Synchronized override fun deviceUsedByOther(sessionId: String, fingerprintHash: String, studentIdHash: String): Boolean =
            attendance.any { it.sessionId == sessionId && it.deviceFingerprintHash == fingerprintHash && it.studentIdHash != studentIdHash }
        @Synchronized override fun attendanceCount(sessionId: String): Int =
            attendance.count { it.sessionId == sessionId }
        @Synchronized override fun addAttendance(record: AttendanceRecord) { attendance.add(record) }

        fun all(): List<AttendanceRecord> = synchronized(this) { attendance.toList() }
    }

    private fun freePort(): Int = ServerSocket(0).use { it.localPort }

    private fun post(url: String, form: String): Pair<Int, String> = try {
        val c = (URL(url).openConnection() as HttpURLConnection).apply {
            requestMethod = "POST"
            connectTimeout = 10_000; readTimeout = 20_000
            doOutput = true
            setRequestProperty("Content-Type", "application/x-www-form-urlencoded")
            outputStream.use { it.write(form.toByteArray(Charsets.UTF_8)) }
        }
        val code = c.responseCode
        code to ((if (code in 200..299) c.inputStream else c.errorStream)?.bufferedReader()?.use { it.readText() } ?: "")
    } catch (e: Exception) {
        -1 to (e.javaClass.simpleName + ": " + e.message)
    }

    private fun form(vararg p: Pair<String, String>) =
        p.joinToString("&") { (k, v) -> "$k=${URLEncoder.encode(v, "UTF-8")}" }

    /** Boots a live hub with `n` students on the roster and the lecturer already started. */
    private fun hub(cards: List<Card>): Triple<InRoomServer, SyncStore, Int> {
        val store = SyncStore()
        val hashes = cards.map { VaultCrypto.hmacHex(hashKey, it.studentId) }
        val session = ActiveSession(
            sessionId = "sess-concurrency",
            tenantId = tenantId,
            academicYear = academicYear,
            institutionPublicKeyPem = pubPem,
            studentHashKey = hashKey,
            rosterHashes = hashes.toSet(),
            rosterSerials = hashes.zip(cards.map { it.serial }).toMap(),
        )
        val secret = ByteArray(32).also { java.security.SecureRandom().nextBytes(it) }
        val server = InRoomServer(CheckinValidator(store), "<html>ATTEND</html>", "<html>GATE</html>")
        server.setLive(
            InRoomServer.Live(
                session = session,
                roomCodeSecret = secret,
                gateContext = {
                    LecturerGateContext(
                        assignedStaffId = "KIU/STAFF/001", roomCodeSecret = secret,
                        gateState = GateState.STARTED, attended = store.attendanceCount(session.sessionId),
                        enrolled = cards.size, ratio = 1.0, requireBiometric = false,
                    )
                },
                onGate = { _, _ -> },
                lecturerStarted = { true },          // gate already open — this test is about scans
                onCheckin = { _, _ -> },
            )
        )
        return Triple(server, store, freePort())
    }

    private fun awaitUp(base: String) {
        repeat(80) {
            runCatching {
                val c = (URL("$base/attend").openConnection() as HttpURLConnection)
                c.connectTimeout = 500; c.readTimeout = 500
                if (c.responseCode == 200) return
            }
            Thread.sleep(100)
        }
        error("in-room server did not come up")
    }

    /** 40 different students, all scanning in the same instant. Every one must be recorded once. */
    @Test
    fun a_whole_cohort_scanning_at_once_is_all_recorded() {
        val n = 40
        val cards = (1..n).map { card("STU-%03d".format(it), "Student $it", "SER-%03d".format(it)) }
        val (server, store, port) = hub(cards)
        val engine = server.start(port)
        try {
            val base = "http://127.0.0.1:$port"
            awaitUp(base)

            val ready = CountDownLatch(1)
            val present = AtomicInteger()
            val transport = Collections.synchronizedList(mutableListOf<String>())
            val pool = Executors.newFixedThreadPool(n)
            val done = CountDownLatch(n)
            cards.forEachIndexed { i, c ->
                pool.submit {
                    ready.await()                                  // release them all together
                    val (code, body) = post("$base/submit", form("qr" to c.rawQr, "fingerprint" to "fp-$i"))
                    when {
                        code == -1 -> transport.add(body)
                        body.contains("PRESENT") -> present.incrementAndGet()
                        else -> transport.add("HTTP $code: ${body.take(80)}")
                    }
                    done.countDown()
                }
            }
            ready.countDown()
            assertTrue(done.await(90, TimeUnit.SECONDS), "the hub did not answer $n simultaneous scans in time")
            pool.shutdown()

            assertTrue(transport.isEmpty(), "connections failed under load: ${transport.take(5)}")
            assertEquals(n, present.get(), "every student in the room should be marked PRESENT")
            assertEquals(n, store.all().size, "the ledger should hold exactly one row per student")
            assertEquals(n, store.all().map { it.studentIdHash }.distinct().size, "no duplicate ledger rows")
            // Sequence numbers are the append-only ordering the sealed package relies on.
            assertEquals(n, store.all().map { it.sequenceNumber }.distinct().size,
                "every record needs its own sequence number — duplicates corrupt the sealed package")
        } finally {
            runCatching { engine.stop(0, 0) }
        }
    }

    /**
     * The same student, scanning 12 times simultaneously — a double-tap, or a phone retrying a
     * request it believed had timed out. Exactly ONE attendance row may result.
     */
    @Test
    fun the_same_student_scanning_many_times_at_once_is_counted_once() {
        val c = card("STU-DUP", "Dup D.", "SER-DUP")
        val (server, store, port) = hub(listOf(c))
        val engine = server.start(port)
        try {
            val base = "http://127.0.0.1:$port"
            awaitUp(base)

            val attempts = 12
            val ready = CountDownLatch(1)
            val outcomes = ConcurrentHashMap<String, AtomicInteger>()
            val pool = Executors.newFixedThreadPool(attempts)
            val done = CountDownLatch(attempts)
            repeat(attempts) {
                pool.submit {
                    ready.await()
                    val (_, body) = post("$base/submit", form("qr" to c.rawQr, "fingerprint" to "fp-dup"))
                    val key = when {
                        body.contains("PRESENT") -> "PRESENT"
                        body.contains("DUPLICATE") -> "DUPLICATE"
                        else -> "OTHER"
                    }
                    outcomes.computeIfAbsent(key) { AtomicInteger() }.incrementAndGet()
                    done.countDown()
                }
            }
            ready.countDown()
            assertTrue(done.await(90, TimeUnit.SECONDS), "the hub did not answer $attempts simultaneous scans in time")
            pool.shutdown()

            val rows = store.all().count { it.studentIdHash == VaultCrypto.hmacHex(hashKey, c.studentId) }
            assertEquals(
                1, rows,
                "one student must produce exactly ONE attendance row, got $rows " +
                    "(outcomes: ${outcomes.mapValues { it.value.get() }}). More than one means the " +
                    "duplicate check and the insert are not atomic, and concurrent scans inflate attendance.",
            )
        } finally {
            runCatching { engine.stop(0, 0) }
        }
    }

    // ── Wi-Fi slots ─────────────────────────────────────────────────────────────

    /**
     * THE WIRE CONTRACT FOR LETTING GO.
     *
     * The hub cannot disconnect anybody — no unprivileged Android app can — so a student's phone
     * frees its Wi-Fi slot only because it is TOLD to and chooses to obey. That instruction is one
     * field on one JSON response, which makes it exactly the kind of thing that gets silently
     * dropped by a refactor and noticed a term later, when somebody wonders why the hall stopped
     * draining. So it is asserted here, on a real server over a real socket.
     *
     * Also asserts the ordering that matters: the check-in must still SUCCEED on the response that
     * carries the eviction. Being told to go is not being turned away — the student is marked
     * present and then released, in that order, and a phone that read `evict` as a rejection would
     * leave the room unrecorded.
     */
    @Test
    fun a_successful_checkin_is_told_to_release_its_slot() {
        val c = card("STU-EVICT", "Evicted Student", "SER-EVICT")
        val (server, _, port) = hub(listOf(c))
        val engine = server.start(port)
        try {
            val base = "http://127.0.0.1:$port"
            awaitUp(base)

            val (code, body) = post("$base/submit", form("qr" to c.rawQr, "fingerprint" to "fp-1"))
            assertEquals(200, code)
            assertTrue(body.contains("PRESENT"), "the student must be marked present: $body")
            assertTrue(body.contains("\"evict\":\"true\""),
                "a student who is now present must be told to free the slot, or the hall never drains: $body")
            assertTrue(body.contains("EVICT_DONE"), "and told WHY, so the app knows not to come back: $body")

            // The hub's own books agree: one student through, and the slot is still counted as
            // occupied until the phone actually reports that it has gone.
            assertEquals(1, server.warden.settledCount())
            assertEquals(1, server.warden.occupancy(), "attended but not yet released is still on the radio")

            val (leaveCode, _) = post("$base/leave", "")
            assertEquals(200, leaveCode)
            assertEquals(0, server.warden.occupancy(), "reporting the release frees the slot immediately")
        } finally {
            runCatching { engine.stop(0, 0) }
        }
    }

    /**
     * The warden's own read-modify-write, under the load it actually meets.
     *
     * `touch` reads a lease, decides, and writes back; `settle` does the same. That is the identical
     * shape as the duplicate-scan race above, and a lecture hall runs it on forty Ktor threads at
     * once. A lost update here does not corrupt attendance — the ledger is guarded separately — but
     * it does corrupt the coordinator's count of who is holding a slot, which is the number the
     * force-drop button is gated on. A jam that cannot be seen cannot be cleared.
     *
     * Driven directly rather than over HTTP because every request in this test file arrives from
     * 127.0.0.1: on a real hotspot each phone holds its own DHCP address, and the point here is
     * forty DISTINCT clients rather than forty requests.
     */
    @Test
    fun forty_phones_touching_at_once_are_counted_exactly_once_each() {
        val n = 40
        val warden = ug.qaat.engine.SlotWarden()
        val ready = CountDownLatch(1)
        val done = CountDownLatch(n)
        val pool = Executors.newFixedThreadPool(n)
        repeat(n) { i ->
            pool.submit {
                ready.await()
                warden.touch("10.0.0.$i", "STU-$i")
                warden.settle("10.0.0.$i", "STU-$i")
                done.countDown()
            }
        }
        ready.countDown()
        assertTrue(done.await(30, TimeUnit.SECONDS), "the warden deadlocked or lost threads")
        pool.shutdown()

        assertEquals(n, warden.seenCount(), "each distinct phone must be counted once")
        assertEquals(n, warden.settledCount(), "each completion must be counted once")
        assertEquals(n, warden.occupancy(), "nobody has reported leaving yet")

        repeat(n) { i -> warden.release("10.0.0.$i") }
        assertEquals(0, warden.occupancy())
    }
}
