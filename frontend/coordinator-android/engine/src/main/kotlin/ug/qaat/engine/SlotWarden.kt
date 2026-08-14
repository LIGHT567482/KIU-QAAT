package ug.qaat.engine

/**
 * WHO IS HOLDING A WI-FI SLOT, AND WHO SHOULD GIVE IT UP.
 *
 * THE PROBLEM. `LocalOnlyHotspot` admits about ten clients. A lecture hall has two hundred
 * students. The cap itself is survivable — a check-in is one POST and takes under a second, so ten
 * slots can clear a hall in a couple of minutes IF each slot turns over quickly. It does not turn
 * over. Android saves the network, auto-reconnects to it, and holds the association until something
 * makes it stop; the student who has already been marked present has no reason to do anything about
 * it. The old plan was to ask: "Turn your Wi-Fi OFF now so a classmate can connect." That is a
 * request made to the one person in the room with nothing left to gain by honouring it, and the
 * queue behind them pays for every student who ignores it.
 *
 * WHAT THIS IS. The hub's own account of who is occupying a slot and for how long, and its verdict
 * on whether they should still be there. Two rules, which are the two the coordinator actually
 * cares about:
 *
 *   EVICT_DONE     — this student has already been marked present. Their business here is finished
 *                    the moment the check-in returns; every further second is a slot a classmate
 *                    cannot have. Keyed by REGISTRATION NUMBER as well as by address, so coming
 *                    back on a fresh DHCP lease does not buy a second turn.
 *   EVICT_EXPIRED  — this client has held a slot for longer than [holdMillis] without finishing.
 *                    Something is wrong at their end — the lecturer has not started, the app is
 *                    sitting on a spinner, the phone associated and wandered off — and whatever it
 *                    is, it is not worth a slot while others wait. They are told to leave and come
 *                    back; the lease is NOT refreshed by activity, so a client that keeps polling
 *                    cannot hold a slot indefinitely by looking busy.
 *
 * WHAT IT IS NOT, AND THIS MATTERS. It is not a deauthenticator. No unprivileged Android app can
 * force a client off a hotspot: `setBlockedClientList`/`setMaxNumberOfClients` sit behind
 * `NETWORK_SETTINGS`, which is system-signature only, and `/proc/net/arp` has been closed to apps
 * since Android 10. So this decides, and the STUDENT APP acts — it drops its own connection when
 * told to (see SlotLease on the student side), which works because both ends of this conversation
 * are our code. A phone that is on the hotspot without running our app, or whose app has been
 * force-stopped mid-flow, cannot be evicted by anything short of restarting the hotspot itself;
 * that one blunt instrument is wired to a coordinator button, not to this class.
 *
 * Pure and clock-injected, so the whole eviction policy is unit-tested off-device — there is no way
 * to reason about a fifteen-second timeout by tapping at a phone.
 */
class SlotWarden(
    /** How long a client may hold a slot without completing a check-in. */
    val holdMillis: Long = 15_000L,
    private val nowMillis: () -> Long = { System.currentTimeMillis() },
) {
    companion object {
        /**
         * How many clients we ASSUME the hotspot admits.
         *
         * A guess, and unavoidably so: `LocalOnlyHotspot`'s client limit is a property of the Wi-Fi
         * driver, there is no API that reports it, and `registerSoftApCallback` — which would let us
         * count the clients the OS sees — is `@SystemApi` behind NETWORK_SETTINGS. Eight is the
         * conservative end of the range stock Android devices land in.
         *
         * Nothing depends on this being RIGHT. It is used only to decide when the hall looks
         * saturated enough to offer the coordinator the force-drop button ([jammed]); every eviction
         * decision is made per-client and needs no cap at all. If it is set too low the button is
         * offered slightly early, which is harmless because it is manual.
         */
        const val ASSUMED_CAP = 8

        /**
         * Extra time past [holdMillis] before a client that WAS told to leave is treated as stuck
         * rather than merely slow. An app obeying an eviction needs a moment to tear the connection
         * down; this is the allowance for that, beyond which the silence means nobody is listening.
         */
        const val STUCK_GRACE_MILLIS = 10_000L

        /** How long the hall must go without a single completed check-in before it counts as
         *  stalled rather than simply between students. */
        const val STALL_MILLIS = 30_000L
    }

    enum class Verdict {
        /** Still within their turn — leave them alone. */
        KEEP,

        /** Already marked present. Nothing left to do here. */
        EVICT_DONE,

        /** Held a slot past [holdMillis] without finishing. Come back and try again. */
        EVICT_EXPIRED,
    }

    private class Lease(
        val grantedAt: Long,
        var reg: String = "",
        var done: Boolean = false,
        /** When this client was FIRST told to leave, by either rule. Null while it is still within
         *  its turn. This — not the age of the lease — is what makes "told to go and still here"
         *  answerable, and the two are not the same thing: a student who finishes in three seconds
         *  is evicted long before their hold would have expired. */
        var evictedAt: Long? = null,
    )

    /** Clients we believe are still associated, keyed by address.
     *
     *  An entry is removed when the client REPORTS that it has gone (`release`, from the hub's
     *  /leave) or when it has been silent long enough to have certainly gone (`sweep`). So a
     *  well-behaved app that obeys an eviction disappears from here within a second, and one that
     *  ignores it stays — which is exactly the distinction [jammed] needs, and the reason
     *  occupancy is counted from this map rather than from a timer. */
    private val leases = LinkedHashMap<String, Lease>()

    /** Registration numbers that have finished, keyed by identity rather than address so a
     *  reconnect on a new IP is still recognised as somebody who has had their turn. */
    private val doneRegs = HashSet<String>()

    /** Distinct clients this session has seen, and how many finished — the coordinator's counters. */
    private var seen = 0
    private var settled = 0

    /** When the last student actually completed. A hall that is moving produces one of these every
     *  few seconds; one that has stalled produces none, which is half of what [jammed] looks for.
     *  Starts at the moment the warden is created so an idle session does not read as jammed before
     *  anybody has arrived. */
    private var lastSettleAt = nowMillis()

    /**
     * Register a request from [clientKey] and rule on whether they should still be holding a slot.
     *
     * Called on EVERY hub request, including the ones that do not check anybody in: the point of a
     * hold timer is to catch the client that is connected and NOT making progress, and that client
     * is by definition only ever seen on `/session` polls.
     *
     * [reg] is supplied when the caller knows who it is talking to. It is what makes EVICT_DONE
     * survive a change of address, so pass it wherever it is available.
     */
    @Synchronized
    fun touch(clientKey: String, reg: String = ""): Verdict {
        val now = nowMillis()
        sweep(now)
        val key = clientKey.ifBlank { "?" }
        val r = normalise(reg)

        val lease = leases[key] ?: Lease(now).also { leases[key] = it; seen++ }
        if (r.isNotEmpty()) lease.reg = r

        // Identity first: someone who has already attended is done wherever they call from. Note
        // the lease is created above even in this case — a phone that has attended and come back on
        // a new address is still occupying a slot, and refusing to record that would hide the very
        // clients [jammed] needs to see.
        val alreadyDone = lease.done || (r.isNotEmpty() && r in doneRegs)

        val verdict = when {
            alreadyDone -> Verdict.EVICT_DONE
            // Deliberately measured from grantedAt and never refreshed: the question is "how long
            // have you been connected", not "how recently were you active".
            now - lease.grantedAt >= holdMillis -> Verdict.EVICT_EXPIRED
            else -> Verdict.KEEP
        }
        // Stamp the moment we first said "go", so ignoring us becomes measurable.
        if (verdict != Verdict.KEEP && lease.evictedAt == null) lease.evictedAt = now
        return verdict
    }

    /**
     * This client has been marked present. Their slot is forfeit from now on.
     *
     * Called on a successful check-in AND on a duplicate — a student whose second tap is refused
     * with DUPLICATE_SCAN is, for slot purposes, in exactly the same position as one whose first
     * tap succeeded: there is nothing further they can accomplish by staying connected.
     */
    @Synchronized
    fun settle(clientKey: String, reg: String) {
        val r = normalise(reg)
        if (r.isNotEmpty()) doneRegs.add(r)
        val now = nowMillis()
        val key = clientKey.ifBlank { "?" }
        val lease = leases[key] ?: Lease(now).also { leases[key] = it; seen++ }
        if (!lease.done) {
            lease.done = true
            settled++
            lastSettleAt = now
            // Finishing IS being told to leave — the client is expected to release the moment it
            // reads this response, so the stuck-clock starts here rather than at the next poll.
            if (lease.evictedAt == null) lease.evictedAt = now
        }
        if (r.isNotEmpty()) lease.reg = r
    }

    /** The client says it has dropped the connection. Frees the slot immediately in the counters,
     *  so the coordinator's occupancy readout reflects a hall that is moving rather than lagging a
     *  sweep behind it. Their `done` standing is kept — [doneRegs] is not touched here. */
    @Synchronized
    fun release(clientKey: String) {
        leases.remove(clientKey.ifBlank { "?" })
    }

    /**
     * How many clients we believe are occupying a slot right now — the number to show beside the
     * hotspot's cap.
     *
     * Counts everyone still in [leases], finished or not, because the question the coordinator is
     * asking is "how full is the radio", and a student who has attended but not yet released is
     * still on it. Well-behaved clients drop out of this within a second of finishing (they call
     * /leave); the ones that linger are supposed to show up here, since that lingering is the whole
     * problem this class exists to make visible.
     */
    @Synchronized
    fun occupancy(): Int {
        sweep(nowMillis())
        return leases.size
    }

    /**
     * IS THE HALL ACTUALLY STUCK?
     *
     * This is the gate on the coordinator's "Free all slots" button, and it is deliberately hard to
     * satisfy. That button restarts the hotspot, which is the only way to force a client off — and
     * it drops EVERY phone and regenerates the SSID and passphrase, so every student in the room has
     * to rejoin with new credentials. Offering it during a session that is merely busy would invite
     * a coordinator to reach for it as a general "unstick things" control and punish forty students
     * who were doing nothing wrong. So it stays hidden until all three of these are true at once:
     *
     *  1. **Saturated** — the slots are full, so somebody is genuinely being kept out.
     *  2. **Somebody is ignoring us** — at least one client has been told to leave and is still
     *     here [STUCK_GRACE_MILLIS] later. This is the signature of the one case the warden cannot
     *     fix by itself: an app that was force-stopped mid-flow, or a phone on the hotspot that is
     *     not running our app at all.
     *  3. **Nothing is moving** — no completed check-in for [STALL_MILLIS]. A hall that is still
     *     turning students over does not need a reset, however full it looks.
     *
     * Any one or two of these on their own is an ordinary busy moment. All three together is the
     * specific situation the blunt instrument exists for.
     */
    @Synchronized
    fun jammed(): Boolean {
        val now = nowMillis()
        sweep(now)
        // Saturation counts EVERY client we still believe is associated, finished or not. A student
        // who has attended and not released is occupying the radio just as surely as one mid-turn,
        // and an earlier draft of this counted only unfinished leases within their hold — which
        // made a hall held entirely by stuck clients report an occupancy of zero, the exact
        // opposite of the truth, and meant the button could never appear when it was most needed.
        val saturated = leases.size >= ASSUMED_CAP
        val ignoring = leases.values.any { l -> l.evictedAt?.let { now - it > STUCK_GRACE_MILLIS } == true }
        val stalled = now - lastSettleAt > STALL_MILLIS
        return saturated && ignoring && stalled
    }

    /** Distinct clients seen, and how many completed — "47 of 180 checked in" on the hub screen. */
    @Synchronized
    fun seenCount(): Int = seen

    @Synchronized
    fun settledCount(): Int = settled

    /** New session, clean slate. Called from the hub whenever the live session changes, so last
     *  lecture's finished students are not still evicted from this one. */
    @Synchronized
    fun reset() {
        leases.clear()
        doneRegs.clear()
        seen = 0
        settled = 0
        lastSettleAt = nowMillis()
    }

    /**
     * Forget leases old enough that the client is certainly gone.
     *
     * A swept client that comes back gets a FRESH hold, which is the intended behaviour and not a
     * loophole: a student who was evicted without attending is supposed to be able to return and
     * try again, and by this point the hall has moved on several turns. The one thing that must not
     * be forgettable is having already attended, and that lives in [doneRegs], which sweeping never
     * touches.
     */
    private fun sweep(now: Long) {
        val cutoff = holdMillis * 5
        val stale = leases.entries.filter { now - it.value.grantedAt > cutoff }
        stale.forEach { leases.remove(it.key) }
    }

    private fun normalise(reg: String) = reg.trim().lowercase()
}
