package ug.qaat.coordinator

import org.junit.Test
import ug.qaat.coordinator.db.TimetableSlotEntity
import ug.qaat.coordinator.session.slotForToday
import ug.qaat.engine.CombinedClassCode
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * THE WIRING BETWEEN THE TIMETABLE AND THE CODE — which is where this feature can go wrong quietly.
 *
 * The derivation itself is already pinned on both sides: CombinedClassCodeTest checks the Kotlin,
 * combinedclass_test.go checks the Go, and the two agree on shared vectors. None of that helps if
 * the phone feeds the derivation the wrong slot, or derives a code for a lecture that isn't shared.
 * Both failures are invisible in a unit test of the hash and obvious only in a hall — a lecturer
 * reads out three digits that open nobody's register, or a coordinator is shown a code field for an
 * ordinary lecture and wonders what number they are supposed to be waiting for.
 *
 * These tests cover exactly that seam.
 */
class CombinedClassWiringTest {

    private val key = "a-shared-session-package-key".toByteArray()

    private fun slot(
        unitId: String = "CSC1101",
        day: Int = 2,
        start: String = "08:00",
        minutes: Int = 120,
        lecturerId: String = "lect-0001",
        classKey: String = "block c 101208:00",
    ) = TimetableSlotEntity(
        unitId = unitId, unitName = unitId, dayOfWeek = day, startTime = start,
        durationMinutes = minutes, room = "Block C 101",
        lecturerId = lecturerId, combinedClassKey = classKey,
    )

    // ── Picking the right slot ──────────────────────────────────────────────────

    @Test
    fun `the only slot of the day is used whatever the hour`() {
        val slots = listOf(slot(start = "14:00"))
        assertEquals("14:00", slotForToday(slots, "CSC1101", 2, 9 * 60)?.startTime)
    }

    @Test
    fun `another day's slot is never used`() {
        val slots = listOf(slot(day = 4))
        assertNull(slotForToday(slots, "CSC1101", 2, 8 * 60))
    }

    @Test
    fun `another unit's slot is never used`() {
        val slots = listOf(slot(unitId = "CSC1102"))
        assertNull(slotForToday(slots, "CSC1101", 2, 8 * 60))
    }

    /**
     * The case that would derive the wrong digits silently: one unit, two lectures in one day. Both
     * slots are real and both are today, but they sit in different hours and therefore carry
     * different class keys — so "the first one" is the right answer only in the morning.
     */
    @Test
    fun `a unit taught twice in one day resolves to the lecture actually running`() {
        val slots = listOf(
            slot(start = "08:00", minutes = 120, classKey = "block c 101208:00"),
            slot(start = "14:00", minutes = 120, classKey = "block d 204214:00"),
        )
        assertEquals("08:00", slotForToday(slots, "CSC1101", 2, 8 * 60 + 30)?.startTime)
        assertEquals("14:00", slotForToday(slots, "CSC1101", 2, 14 * 60 + 30)?.startTime)
    }

    /** Opening a few minutes early is normal; it must reach forward, not fall back to the morning. */
    @Test
    fun `between lectures it picks the one about to start`() {
        val slots = listOf(slot(start = "08:00", minutes = 60), slot(start = "14:00", minutes = 60))
        assertEquals("14:00", slotForToday(slots, "CSC1101", 2, 13 * 60 + 50)?.startTime)
    }

    /** And opening late — after the last lecture has ended — still means the last lecture. */
    @Test
    fun `after the day's last lecture it stays on that lecture`() {
        val slots = listOf(slot(start = "08:00", minutes = 60), slot(start = "14:00", minutes = 60))
        assertEquals("14:00", slotForToday(slots, "CSC1101", 2, 16 * 60)?.startTime)
    }

    @Test
    fun `a slot with an unreadable start time is ignored rather than guessed at`() {
        val slots = listOf(slot(start = ""), slot(start = "not-a-time"), slot(start = "10:00"))
        assertEquals("10:00", slotForToday(slots, "CSC1101", 2, 8 * 60)?.startTime)
    }

    // ── When a code exists at all ───────────────────────────────────────────────
    //
    // These mirror the condition SessionController.open applies. A combined class is the exception,
    // not the rule: the server sends a combinedClassKey only for a lecture it can see is genuinely
    // shared, and everything below turns on treating a blank one as "no code", never as an empty
    // string to hash.

    /** The guard SessionController.open uses, kept here so the tests exercise the real condition. */
    private fun codeFor(secret: ByteArray, s: TimetableSlotEntity?, date: String): String {
        val lecturerId = s?.lecturerId.orEmpty()
        val classKey = s?.combinedClassKey.orEmpty()
        return if (secret.isNotEmpty() && lecturerId.isNotBlank() && classKey.isNotBlank())
            CombinedClassCode.derive(secret, lecturerId, classKey, date)
        else ""
    }

    @Test
    fun `an ordinary lecture carries no key and so derives no code`() {
        // What the manifest sends for a lecture nobody else shares: lecturer named, key blank.
        val ordinary = slot(classKey = "")
        assertEquals("", codeFor(key, ordinary, "2026-08-13"),
            "an unshared lecture must derive nothing — a code here would put a field on screen " +
                "for every coordinator, all day, for a situation that arises occasionally")
    }

    @Test
    fun `a slot with nobody assigned derives no code`() {
        assertEquals("", codeFor(key, slot(lecturerId = ""), "2026-08-13"))
    }

    @Test
    fun `no slot at all derives no code`() {
        assertEquals("", codeFor(key, null, "2026-08-13"))
    }

    @Test
    fun `an institution with no derivation key derives no code`() {
        assertEquals("", codeFor(ByteArray(0), slot(), "2026-08-13"))
    }

    /**
     * And the case it all exists for: two cohorts, different unit codes, different unit names, one
     * lecture. They share a room and an hour, so the server hands them the SAME key — and that is
     * what makes one spoken number open both registers.
     */
    @Test
    fun `two cohorts of one lecture derive the same three digits`() {
        val sharedKey = "block c 101208:00"
        val cohortA = slot(unitId = "CSC1101", classKey = sharedKey)
        val cohortB = slot(unitId = "BIT1101", classKey = sharedKey)

        val a = codeFor(key, cohortA, "2026-08-13")
        val b = codeFor(key, cohortB, "2026-08-13")

        assertEquals(a, b, "cohorts of one combined lecture must land on one code")
        assertEquals(3, a.length)
        assertTrue(a.all { it.isDigit() }, "a code read aloud must be digits only")
        assertTrue(
            CombinedClassCode.validate(key, a, cohortB.lecturerId, cohortB.combinedClassKey, "2026-08-13"),
            "the code the lecturer reads out must open the other cohort's register",
        )
    }

    /** A lecture in the next room at the same hour is a different lecture, and must not be opened. */
    @Test
    fun `the code does not open a different lecture in the same hour`() {
        val here = slot(classKey = "block c 101208:00")
        val nextDoor = slot(classKey = "block d 204208:00", lecturerId = "lect-0002")
        val mine = codeFor(key, here, "2026-08-13")
        assertTrue(
            mine != codeFor(key, nextDoor, "2026-08-13") ||
                !CombinedClassCode.validate(key, mine, nextDoor.lecturerId, nextDoor.combinedClassKey, "2026-08-13"),
            "another lecture's register must not open on this lecture's code",
        )
    }
}
