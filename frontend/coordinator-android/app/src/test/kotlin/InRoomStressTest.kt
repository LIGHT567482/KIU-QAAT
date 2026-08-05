package ug.qaat.coordinator

import org.junit.Test
import ug.qaat.coordinator.server.InRoomServer
import ug.qaat.crypto.VaultCrypto
import ug.qaat.engine.*
import java.net.HttpURLConnection
import java.net.ServerSocket
import java.net.URL
import java.net.URLEncoder
import java.util.Collections
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicLong
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * THOUSANDS of students checking in at once, against one coordinator's phone.
 *
 * [InRoomConcurrencyTest] proves correctness for a room of forty. This asks a different question:
 * what happens at the scale of a shared hall or a whole year group arriving together — the case
 * the institution actually worries about, and the one nobody can rehearse by hand because it needs
 * a thousand handsets.
 *
 * It checks in over the REG-NUMBER path, not the QR one, because that is what ships ("No QR
 * anywhere" — see AGENTS.md) and because it isolates the right thing: an RSA verification per
 * request would make this a benchmark of the crypto library rather than of the hub's bookkeeping,
 * which is where concurrency bugs actually live.
 *
 * What must hold, and why each would matter in a real hall:
 *
 *  1. **Nobody is dropped.** Every student who submits gets an answer. A refused connection is a
 *     student standing in the room, marked absent, with nothing they can do about it.
 *  2. **Nobody is counted twice.** One row per student, and one distinct sequence number per row —
 *     that ordering is what the sealed package uploaded to the server is built on.
 *  3. **It stays responsive.** Latency is reported rather than asserted tightly, because the CI
 *     machine is not a phone; the assertion is only that the whole storm completes, and the
 *     numbers are printed so a regression is visible in the log.
 *
 * The store is synchronised, mirroring SQLite's serialised writers, so the question stays "is the
 * VALIDATOR's read-then-write atomic" rather than "is the toy store thread-safe".
 */
class InRoomStressTest {

    /** How many students storm the hub. Override with -Dqaat.stress.students=5000 to push harder. */
    private val students: Int = Integer.getInteger("qaat.stress.students", 2000)

    /** Simultaneous in-flight requests. A phone's hotspot does not carry thousands of sockets at
     *  once either; this is the realistic ceiling, and the rest queue behind it exactly as they
     *  would in a room. */
    private val concurrency: Int = Integer.getInteger("qaat.stress.concurrency", 250)

    private val hashKey = "stress-student-hash-key"
    private val tenantId = "tenant-stress"

    private class SyncStore : Store {
        private val bindings = mutableListOf<DeviceBinding>()
        private val attendance = mutableListOf<AttendanceRecord>()

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
            connectTimeout = 20_000; readTimeout = 60_000
            doOutput = true
            setRequestProperty("Content-Type", "application/x-www-form-urlencoded")
            outputStream.use { it.write(form.toByteArray(Charsets.UTF_8)) }
        }
        val code = c.responseCode
        code to ((if (code in 200..299) c.inputStream else c.errorStream)?.bufferedReader()?.use { it.readText() } ?: "")
    } catch (e: Exception) {
        -1 to (e.javaClass.simpleName + ": " + (e.message ?: ""))
    }

    private fun form(vararg p: Pair<String, String>) =
        p.joinToString("&") { (k, v) -> "$k=${URLEncoder.encode(v, "UTF-8")}" }

    private fun awaitUp(base: String) {
        repeat(120) {
            runCatching {
                val c = (URL("$base/attend").openConnection() as HttpURLConnection)
                c.connectTimeout = 500; c.readTimeout = 500
                if (c.responseCode == 200) return
            }
            Thread.sleep(100)
        }
        error("in-room server did not come up")
    }

    @Test
    fun thousands_of_students_checking_in_at_once_are_each_recorded_exactly_once() {
        val regs = (1..students).map { "STRESS-%06d".format(it) }
        val hashes = regs.map { VaultCrypto.hmacHex(hashKey, it) }
        val store = SyncStore()
        val session = ActiveSession(
            sessionId = "sess-stress",
            tenantId = tenantId,
            academicYear = "2026",
            institutionPublicKeyPem = "",          // unused on the reg-number path
            studentHashKey = hashKey,
            rosterHashes = hashes.toSet(),
            rosterSerials = emptyMap(),
        )
        // The device lock is off, as it is in the shipping build (AppState.ENFORCE_DEVICE_LOCK):
        // with it on, one synthetic fingerprint per student is the only thing this could model,
        // and the interesting contention — everyone racing for the same ledger — is unchanged.
        val validator = CheckinValidator(store, enforceDeviceLock = false)
        val secret = ByteArray(32).also { java.security.SecureRandom().nextBytes(it) }
        val server = InRoomServer(validator, "<html>ATTEND</html>", "<html>GATE</html>")
        server.setLive(
            InRoomServer.Live(
                session = session,
                roomCodeSecret = secret,
                gateContext = {
                    LecturerGateContext(
                        assignedStaffId = "KIU/STAFF/001", roomCodeSecret = secret,
                        gateState = GateState.STARTED, attended = store.attendanceCount(session.sessionId),
                        enrolled = students, ratio = 1.0, requireBiometric = false,
                    )
                },
                onGate = { _, _ -> },
                lecturerStarted = { true },
                onCheckin = { _, _ -> },
            )
        )
        val port = freePort()
        val engine = server.start(port)
        try {
            val base = "http://127.0.0.1:$port"
            awaitUp(base)

            val ready = CountDownLatch(1)
            val present = AtomicInteger()
            val outcomes = ConcurrentHashMap<String, AtomicInteger>()
            val transport = Collections.synchronizedList(mutableListOf<String>())
            val latencies = Collections.synchronizedList(mutableListOf<Long>())
            val slowest = AtomicLong()

            val pool = Executors.newFixedThreadPool(concurrency)
            val done = CountDownLatch(students)
            regs.forEachIndexed { i, reg ->
                pool.submit {
                    ready.await()                                  // release them together
                    val t0 = System.nanoTime()
                    val (code, body) = post("$base/checkin", form("reg_number" to reg, "fingerprint" to "fp-$i"))
                    val ms = (System.nanoTime() - t0) / 1_000_000
                    latencies.add(ms)
                    slowest.updateAndGet { maxOf(it, ms) }
                    when {
                        code == -1 -> transport.add(body)
                        body.contains("PRESENT") -> present.incrementAndGet()
                        else -> outcomes.computeIfAbsent("HTTP $code ${body.take(60)}") { AtomicInteger() }.incrementAndGet()
                    }
                    done.countDown()
                }
            }
            val wall = System.nanoTime()
            ready.countDown()
            val finished = done.await(5, TimeUnit.MINUTES)
            val elapsedMs = (System.nanoTime() - wall) / 1_000_000
            pool.shutdownNow()

            val sorted = latencies.sorted()
            fun q(p: Double) = if (sorted.isEmpty()) 0L else sorted[minOf(sorted.size - 1, (p * sorted.size).toInt())]
            println(
                """
                |
                |── in-room hub under load ──────────────────────────────
                |students        $students
                |concurrency     $concurrency
                |wall clock      ${elapsedMs}ms
                |throughput      ${if (elapsedMs > 0) students * 1000L / elapsedMs else 0} check-ins/s
                |recorded        ${present.get()}
                |latency p50     ${q(0.50)}ms
                |latency p95     ${q(0.95)}ms
                |latency p99     ${q(0.99)}ms
                |latency max     ${slowest.get()}ms
                |""".trimMargin()
            )

            assertTrue(finished, "the hub did not answer $students simultaneous check-ins within 5 minutes")
            assertTrue(
                transport.isEmpty(),
                "${transport.size} connections were dropped under load — a dropped connection is a " +
                    "student in the room who is marked absent: ${transport.take(3)}",
            )
            assertEquals(
                students, present.get(),
                "every student who submitted must be recorded (non-PRESENT answers: " +
                    "${outcomes.mapValues { it.value.get() }})",
            )
            val rows = store.all()
            assertEquals(students, rows.size, "the ledger should hold exactly one row per student")
            assertEquals(students, rows.map { it.studentIdHash }.distinct().size, "no duplicate ledger rows")
            assertEquals(
                students, rows.map { it.sequenceNumber }.distinct().size,
                "every record needs its own sequence number — duplicates make the sealed package " +
                    "ambiguous about what happened in the room",
            )
        } finally {
            runCatching { engine.stop(0, 0) }
        }
    }
}
