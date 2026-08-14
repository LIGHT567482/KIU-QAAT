package ug.qaat.coordinator

import org.junit.Test
import ug.qaat.engine.SlotWarden
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * THE EVICTION POLICY, TESTED ON A CLOCK WE CONTROL.
 *
 * Every rule in [SlotWarden] is a statement about elapsed time — fifteen seconds holding a slot,
 * ten more before you count as ignoring us, thirty with nothing moving. None of that can be checked
 * by tapping at a phone: you would be waiting on a real stopwatch for a behaviour that only shows
 * up in a full hall, and a fifteen-second timeout that fires at fourteen or sixteen looks identical
 * from the outside. So the clock is injected and these tests move it by hand.
 *
 * The cases below are the ones that decide whether a lecture hall drains or stalls.
 */
class SlotWardenTest {

    /** A warden whose clock we drive. */
    private class Clock(var t: Long = 1_000_000L)

    private fun warden(c: Clock) = SlotWarden(nowMillis = { c.t })

    // ── The hold timer ──────────────────────────────────────────────────────────

    @Test
    fun `a fresh client is kept, and evicted the moment the hold elapses`() {
        val c = Clock()
        val w = warden(c)

        assertEquals(SlotWarden.Verdict.KEEP, w.touch("10.0.0.5"), "first contact starts the turn")

        c.t += w.holdMillis - 1
        assertEquals(SlotWarden.Verdict.KEEP, w.touch("10.0.0.5"), "still inside the hold")

        c.t += 1
        assertEquals(SlotWarden.Verdict.EVICT_EXPIRED, w.touch("10.0.0.5"), "exactly at the hold, they go")
    }

    /**
     * The loophole that would make the whole thing pointless: if activity refreshed the lease, the
     * app polling every few seconds — which is exactly what the student app does — would renew its
     * own slot forever and never be evicted at all.
     */
    @Test
    fun `polling does not refresh the lease`() {
        val c = Clock()
        val w = warden(c)
        w.touch("10.0.0.5")

        // Poll every 4s, as the student screen does, right up to the hold.
        repeat(3) {
            c.t += 4_000
            assertEquals(SlotWarden.Verdict.KEEP, w.touch("10.0.0.5"))
        }
        c.t += 4_000 // 16s total
        assertEquals(SlotWarden.Verdict.EVICT_EXPIRED, w.touch("10.0.0.5"),
            "a busy-looking client must not be able to hold a slot indefinitely")
    }

    // ── Having already attended ─────────────────────────────────────────────────

    @Test
    fun `a settled client is evicted immediately, well inside its hold`() {
        val c = Clock()
        val w = warden(c)
        w.touch("10.0.0.5", "S24B13001")

        c.t += 2_000
        w.settle("10.0.0.5", "S24B13001")

        assertEquals(SlotWarden.Verdict.EVICT_DONE, w.touch("10.0.0.5", "S24B13001"),
            "there is nothing left for them to do here")
    }

    /**
     * The one that stops a second turn: reconnecting picks up a new DHCP lease, so address-keyed
     * eviction alone would hand a student who has already attended a fresh fifteen seconds every
     * time they rejoined.
     */
    @Test
    fun `already-attended survives a change of address`() {
        val c = Clock()
        val w = warden(c)
        w.touch("10.0.0.5", "S24B13001")
        w.settle("10.0.0.5", "S24B13001")

        c.t += 60_000
        assertEquals(SlotWarden.Verdict.EVICT_DONE, w.touch("10.0.0.99", "S24B13001"),
            "same student, new IP — still done")
    }

    @Test
    fun `registration number matching ignores case and surrounding space`() {
        val c = Clock()
        val w = warden(c)
        w.settle("10.0.0.5", "S24B13001")
        assertEquals(SlotWarden.Verdict.EVICT_DONE, w.touch("10.0.0.7", "  s24b13001 "))
    }

    /**
     * Sweeping exists to stop the map growing all lecture. It must forget a client that merely ran
     * out of time — they are entitled to come back and try again — without ever forgetting one that
     * attended, which would hand them a second turn an hour later.
     */
    @Test
    fun `sweeping returns a turn to a non-attendee but never to an attendee`() {
        val c = Clock()
        val w = warden(c)

        w.touch("10.0.0.5")                      // never finished
        w.touch("10.0.0.6", "S24B13002")
        w.settle("10.0.0.6", "S24B13002")        // finished

        c.t += w.holdMillis * 6                  // past the sweep cutoff

        assertEquals(SlotWarden.Verdict.KEEP, w.touch("10.0.0.5"),
            "a student who was evicted without attending may try again")
        assertEquals(SlotWarden.Verdict.EVICT_DONE, w.touch("10.0.0.6", "S24B13002"),
            "having attended is not forgettable")
    }

    /**
     * THE WHOLE POINT, ON A FULL HALL: every student who has been recorded present is told to go,
     * and the slots they were holding become available to students who have not attended yet.
     *
     * The per-client tests above each prove one step of this. This proves the property they exist
     * for, on a hall at capacity, because that is the only condition under which any of it matters
     * — a cap of ~10 against two hundred students is either a queue that drains or a lecture that
     * never finishes registering.
     *
     * Note what is asserted about occupancy at each stage. Attending does NOT free the radio;
     * being told to leave and actually leaving does. So the sequence is: fill → all present → all
     * evicted on their next contact → released → occupancy back to zero → the next students in.
     */
    @Test
    fun `every attended student is evicted and their slot goes to a waiting student`() {
        val c = Clock()
        val w = warden(c)

        val cap = SlotWarden.ASSUMED_CAP
        val firstWave = (1..cap).map { "10.0.0.$it" to "S24B%05d".format(it) }

        // A full hall: every slot taken, nobody finished yet.
        firstWave.forEach { (ip, _) -> assertEquals(SlotWarden.Verdict.KEEP, w.touch(ip)) }
        assertEquals(cap, w.occupancy(), "the hall should be at capacity")

        // Every one of them checks in successfully.
        firstWave.forEach { (ip, reg) -> w.settle(ip, reg) }

        // EVERY attendee is told to leave on their next contact — not some, not most.
        firstWave.forEach { (ip, reg) ->
            assertEquals(SlotWarden.Verdict.EVICT_DONE, w.touch(ip, reg),
                "attended student $reg was not evicted")
        }

        // Obeying the eviction is what actually frees the radio.
        firstWave.forEach { (ip, _) -> w.release(ip) }
        assertEquals(0, w.occupancy(), "every freed slot should be back in the pool")

        // And the next wave gets in — the queue moved, which is the entire objective.
        val secondWave = (1..cap).map { "10.0.1.$it" }
        secondWave.forEach { ip ->
            assertEquals(SlotWarden.Verdict.KEEP, w.touch(ip),
                "a waiting student could not take a freed slot")
        }
        assertEquals(cap, w.occupancy())

        // A returning attendee never displaces one of them, even on a fresh address.
        assertEquals(SlotWarden.Verdict.EVICT_DONE, w.touch("10.0.9.9", firstWave[0].second),
            "an attended student came back and took a slot from someone still waiting")
    }

    // ── Occupancy ───────────────────────────────────────────────────────────────

    @Test
    fun `occupancy counts everyone believed associated, and release frees a slot at once`() {
        val c = Clock()
        val w = warden(c)
        w.touch("10.0.0.5"); w.touch("10.0.0.6"); w.touch("10.0.0.7")
        assertEquals(3, w.occupancy())

        // Finishing does NOT free the radio — only letting go does.
        w.settle("10.0.0.6", "S24B13002")
        assertEquals(3, w.occupancy(), "attended but still connected is still occupying a slot")

        w.release("10.0.0.6")
        assertEquals(2, w.occupancy())
    }

    @Test
    fun `counters track distinct clients and completions`() {
        val c = Clock()
        val w = warden(c)
        w.touch("10.0.0.5", "A"); w.touch("10.0.0.5", "A"); w.touch("10.0.0.6", "B")
        w.settle("10.0.0.5", "A")

        assertEquals(2, w.seenCount(), "the same address twice is one client")
        assertEquals(1, w.settledCount())
    }

    @Test
    fun `reset clears the previous lecture entirely`() {
        val c = Clock()
        val w = warden(c)
        w.touch("10.0.0.5", "S24B13001")
        w.settle("10.0.0.5", "S24B13001")

        w.reset()

        assertEquals(0, w.occupancy())
        assertEquals(0, w.settledCount())
        assertEquals(SlotWarden.Verdict.KEEP, w.touch("10.0.0.5", "S24B13001"),
            "last lecture's attendance must not evict anyone from this one")
    }

    // ── jammed(): the gate on the force-drop button ──────────────────────────────

    /**
     * The regression this whole predicate turns on. When every slot is held by a client that has
     * been told to go and hasn't, the button must appear — that is precisely the situation it
     * exists for. An earlier version counted only leases still INSIDE their hold, so a fully stuck
     * hall reported an occupancy of zero and the button stayed hidden exactly when it was needed.
     */
    @Test
    fun `a hall held entirely by stuck clients is jammed`() {
        val c = Clock()
        val w = warden(c)
        repeat(SlotWarden.ASSUMED_CAP) { w.touch("10.0.0.$it") }

        // Everyone runs out of time and is told to leave...
        c.t += w.holdMillis
        repeat(SlotWarden.ASSUMED_CAP) { w.touch("10.0.0.$it") }
        assertFalse(w.jammed(), "just evicted — they deserve a moment to act on it")

        // ...and none of them acts on it.
        c.t += SlotWarden.STUCK_GRACE_MILLIS + SlotWarden.STALL_MILLIS
        assertTrue(w.jammed(), "full, ignoring us, and nothing moving")
    }

    @Test
    fun `a busy hall that is still turning students over is not jammed`() {
        val c = Clock()
        val w = warden(c)
        repeat(SlotWarden.ASSUMED_CAP) { w.touch("10.0.0.$it") }
        c.t += w.holdMillis + SlotWarden.STUCK_GRACE_MILLIS + 1
        repeat(SlotWarden.ASSUMED_CAP) { w.touch("10.0.0.$it") }

        // Somebody just completed, so the queue is demonstrably moving.
        w.settle("10.0.0.1", "S24B13001")

        assertFalse(w.jammed(), "a hall that is still making progress must not offer the reset")
    }

    @Test
    fun `an empty or half-full hall is never jammed`() {
        val c = Clock()
        val w = warden(c)
        assertFalse(w.jammed(), "nothing has even happened yet")

        w.touch("10.0.0.5")
        c.t += w.holdMillis + SlotWarden.STUCK_GRACE_MILLIS + SlotWarden.STALL_MILLIS
        w.touch("10.0.0.5")
        assertFalse(w.jammed(), "one stuck client with seven free slots keeps nobody out")
    }
}
