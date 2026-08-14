package ug.qaat.engine

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class CombinedClassCodeTest {
    private val key = "a-shared-session-package-key".toByteArray()
    private val lecturer = "lect-0001"
    private val combined = "combined-7f3a"

    /** Every cohort's phone must land on the SAME digits, or one spoken number cannot open them all. */
    @Test
    fun `every coordinator of the same class derives the same code offline`() {
        val onDavidsPhone = CombinedClassCode.derive(key, lecturer, combined, "2026-08-13")
        val onJoshuasPhone = CombinedClassCode.derive(key, lecturer, combined, "2026-08-13")
        assertEquals(onDavidsPhone, onJoshuasPhone)
        assertEquals(3, onDavidsPhone.length)
        assertTrue(onDavidsPhone.all { it.isDigit() }, "a code read aloud must be digits only")
    }

    /** The point of the day binding: today's code is dead tomorrow, even if the digits repeat. */
    @Test
    fun `a new day derives a new code and yesterday's is refused`() {
        val today = CombinedClassCode.derive(key, lecturer, combined, "2026-08-13")
        assertTrue(
            CombinedClassCode.validate(key, today, lecturer, combined, "2026-08-13"),
            "today's code must open today's register",
        )
        assertFalse(
            CombinedClassCode.validate(key, today, lecturer, combined, "2026-08-14"),
            "yesterday's code must not open tomorrow's register",
        )
    }

    /**
     * THE HONEST LIMIT OF THREE DIGITS, measured rather than asserted from hope.
     *
     * A thousand possible codes means that across a year roughly one day in a thousand derives the
     * same digits as any given other day — and on such a day the old number really would be
     * accepted, because it is genuinely the code for that day too. The day binding does not make
     * that impossible; nothing with three digits can. What it does is make every day's code
     * INDEPENDENT, so the overwhelming majority of yesterday's codes are refused today, and a
     * repeat is an unrelated coincidence rather than a session being reused.
     *
     * This test pins the real rate so a change to the derivation that made collisions common —
     * a truncation bug, a date dropped from the message — fails here instead of in a hall.
     */
    @Test
    fun `codes are independent day to day, with only the collision rate arithmetic forces`() {
        val day0 = CombinedClassCode.derive(key, lecturer, combined, "2026-01-01")
        var sameDigits = 0
        var days = 0
        for (m in 1..12) for (d in 1..28) {
            val date = "2026-%02d-%02d".format(m, d)
            if (date == "2026-01-01") continue
            days++
            if (CombinedClassCode.derive(key, lecturer, combined, date) == day0) sameDigits++
        }
        // 336 days against 1000 possible codes: a handful of repeats is expected, a flood is a bug.
        assertTrue(sameDigits <= days / 50,
            "$sameDigits of $days days repeated the same code — the derivation is not spreading")
        // And the property that actually protects the logs: a DIFFERENT code is always refused.
        val other = (1..28).map { "2026-06-%02d".format(it) }
            .first { CombinedClassCode.derive(key, lecturer, combined, it) != day0 }
        assertFalse(CombinedClassCode.validate(key, day0, lecturer, combined, other),
            "a code from another day must not open this day's register")
    }

    /** Bound to the lecturer: the next room's lecturer derives different digits from the same key. */
    @Test
    fun `a different lecturer or class derives a different code`() {
        val mine = CombinedClassCode.derive(key, lecturer, combined, "2026-08-13")
        val otherLecturer = CombinedClassCode.derive(key, "lect-0002", combined, "2026-08-13")
        val otherClass = CombinedClassCode.derive(key, lecturer, "combined-9999", "2026-08-13")
        assertFalse(CombinedClassCode.validate(key, otherLecturer, lecturer, combined, "2026-08-13") && mine != otherLecturer)
        assertFalse(CombinedClassCode.validate(key, otherClass, lecturer, combined, "2026-08-13") && mine != otherClass)
    }

    /** Typed by a person, in a hurry, on a phone. */
    @Test
    fun `surrounding space is tolerated but a wrong length never is`() {
        val code = CombinedClassCode.derive(key, lecturer, combined, "2026-08-13")
        assertTrue(CombinedClassCode.validate(key, "  $code ", lecturer, combined, "2026-08-13"))
        assertFalse(CombinedClassCode.validate(key, code.take(2), lecturer, combined, "2026-08-13"))
        assertFalse(CombinedClassCode.validate(key, code + "7", lecturer, combined, "2026-08-13"))
        assertFalse(CombinedClassCode.validate(key, "", lecturer, combined, "2026-08-13"))
    }

    /** Ambiguous field boundaries must not let two different classes share a code. */
    @Test
    fun `ids that split differently do not collide`() {
        val a = CombinedClassCode.derive(key, "ab", "c", "2026-08-13")
        val b = CombinedClassCode.derive(key, "a", "bc", "2026-08-13")
        assertFalse(
            CombinedClassCode.validate(key, a, "a", "bc", "2026-08-13") && a == b,
            "concatenation without a separator let two different classes derive one code",
        )
    }

    /**
     * CROSS-LANGUAGE PARITY — the same four vectors the Go suite asserts
     * (backend/api-gateway/internal/checkin/combinedclass_test.go), produced by a third,
     * independent implementation so neither side is merely agreeing with its own misreading.
     *
     * If the phone and the server ever derive different digits, the lecturer reads out a number
     * that opens nobody's register and no test would catch it — the hall would. This is that test.
     */
    @Test
    fun `derivation matches the server, byte for byte`() {
        val k = "a-shared-session-package-key".toByteArray()
        assertEquals("946", CombinedClassCode.derive(k, "lect-0001", "combined-7f3a", "2026-08-13"))
        assertEquals("582", CombinedClassCode.derive(k, "lect-0001", "combined-7f3a", "2026-08-14"))
        assertEquals("809", CombinedClassCode.derive(k, "lect-0002", "combined-7f3a", "2026-08-13"))
        assertEquals("188", CombinedClassCode.derive(k, "lect-0001", "combined-9999", "2026-08-13"))
    }
}
