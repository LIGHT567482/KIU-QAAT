import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import ug.qaat.coordinator.db.TimetableSlotEntity
import ug.qaat.coordinator.presence.PresenceCapture
import java.time.LocalDateTime

/**
 * Which lecture a presence claim belongs to.
 *
 * This runs on the phone, offline, and its answer is what a QA officer reads months later beside a
 * patrol tick — so a wrong match is not a cosmetic defect. Naming the wrong lecture would put a
 * lecturer's own evidence against the wrong accusation, and "nearest" quietly presented as "during"
 * would make a claim look stronger than it is.
 *
 * Thursday 6 August 2026 throughout: a Thursday, so day_of_week 4.
 */
class PresenceMatchTest {

    private fun slot(unit: String, dow: Int, start: String, mins: Int = 60) =
        TimetableSlotEntity(
            unitId = unit, unitName = "Unit $unit", dayOfWeek = dow,
            startTime = start, durationMinutes = mins, room = "LR1",
        )

    private fun at(h: Int, m: Int) = LocalDateTime.of(2026, 8, 6, h, m)

    @Test
    fun `inside the lecture is IN_SLOT with the minutes elapsed`() {
        val m = PresenceCapture.match(listOf(slot("A", 4, "14:00", 60)), at(14, 7))
        assertEquals("IN_SLOT", m.kind)
        assertEquals("A", m.slot?.unitId)
        assertEquals(7, m.minutesFromStart)
    }

    @Test
    fun `the first minute counts and the last does not`() {
        val s = listOf(slot("A", 4, "14:00", 60))
        assertEquals("IN_SLOT", PresenceCapture.match(s, at(14, 0)).kind)
        assertEquals("IN_SLOT", PresenceCapture.match(s, at(14, 59)).kind)
        // 15:00 is the NEXT hour, not this lecture — it is inside the grace window instead.
        assertEquals("NEAR_SLOT", PresenceCapture.match(s, at(15, 0)).kind)
    }

    @Test
    fun `arriving early and running over are NEAR_SLOT, signed either way`() {
        val s = listOf(slot("A", 4, "14:00", 60))
        val early = PresenceCapture.match(s, at(13, 50))
        assertEquals("NEAR_SLOT", early.kind)
        assertEquals(-10, early.minutesFromStart)

        val over = PresenceCapture.match(s, at(15, 15))
        assertEquals("NEAR_SLOT", over.kind)
        assertEquals(75, over.minutesFromStart)
    }

    @Test
    fun `well outside every slot names the nearest rather than claiming a match`() {
        // 11:30 — 3h30 after the 08:00 and 2h30 before the 14:00, so the 14:00 is nearer. The KIND
        // is what carries the weight: the dashboard renders NEAREST as "nothing was running", so
        // naming a lecture here never reads as a claim to have been in it.
        val m = PresenceCapture.match(listOf(slot("A", 4, "14:00"), slot("B", 4, "08:00")), at(11, 30))
        assertEquals("NEAREST", m.kind)
        assertEquals("A", m.slot?.unitId)
        assertEquals(-150, m.minutesFromStart)
    }

    @Test
    fun `another day's lectures are not this day's`() {
        // Everything timetabled for Monday; the claim is filed on Thursday.
        val m = PresenceCapture.match(listOf(slot("A", 1, "14:00"), slot("B", 1, "16:00")), at(14, 7))
        assertEquals("NONE", m.kind)
        assertNull(m.slot)
    }

    @Test
    fun `a slot with no day recorded is still considered`() {
        // dayOfWeek 0 means the timetable never carried a day. Dropping it would hide a real
        // lecture from a lecturer standing in it — same rule the patrol round follows.
        val m = PresenceCapture.match(listOf(slot("A", 0, "14:00")), at(14, 10))
        assertEquals("IN_SLOT", m.kind)
        assertEquals("A", m.slot?.unitId)
    }

    @Test
    fun `two lectures over each other resolve to the one being taught now`() {
        // A double-booking, or one lecturer down for two cohorts. The one we are furthest INTO is
        // the one actually running; picking the first in the list would be arbitrary.
        val m = PresenceCapture.match(
            listOf(slot("EARLY", 4, "14:00", 120), slot("LATE", 4, "15:00", 60)), at(15, 10),
        )
        assertEquals("IN_SLOT", m.kind)
        assertEquals("LATE", m.slot?.unitId)
        assertEquals(10, m.minutesFromStart)
    }

    @Test
    fun `an unreadable start time is dropped, not treated as midnight`() {
        // A slot defaulted to 00:00 would be "nearest" to nothing and would beat a real lecture.
        val m = PresenceCapture.match(
            listOf(slot("BAD", 4, "not-a-time"), slot("GOOD", 4, "14:00")), at(14, 5),
        )
        assertEquals("IN_SLOT", m.kind)
        assertEquals("GOOD", m.slot?.unitId)
    }

    @Test
    fun `an empty timetable is NONE, not a crash`() {
        val m = PresenceCapture.match(emptyList(), at(14, 0))
        assertEquals("NONE", m.kind)
        assertNull(m.slot)
        assertNull(m.minutesFromStart)
    }
}
