package ug.qaat.coordinator.presence

import android.Manifest
import android.content.Context
import android.content.pm.PackageManager
import android.location.Location
import android.location.LocationListener
import android.location.LocationManager
import android.os.Looper
import androidx.core.content.ContextCompat
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeoutOrNull
import ug.qaat.coordinator.db.AppDao
import ug.qaat.coordinator.db.PresenceClaimEntity
import ug.qaat.coordinator.db.TimetableSlotEntity
import java.time.Instant
import java.time.LocalDate
import java.time.LocalDateTime
import java.util.UUID
import kotlin.coroutines.resume

/**
 * "I was here." The lecturer's own record of a moment the QA patrol also records.
 *
 * A patrol tick is one person's account, timestamped and filed against a named lecturer, and until
 * now it was the only account there was. A lecturer teaching in a room the patroller never reached
 * could answer only with their word, days later, against a record made at the time. This is a
 * record made at the time by the other party.
 *
 * EVERYTHING HERE WORKS WITH NO NETWORK, which is the whole point — the rooms where this is needed
 * are the rooms with no signal, and a lecturer who could have reached the server at the time would
 * not be having this argument. GPS is a satellite fix, not an internet lookup. The timetable it
 * matches against was cached the last time there was signal. The claim goes into the encrypted
 * local database and uploads later.
 *
 * WHAT IT IS AND IS NOT. A phone reports the coordinates it is handed, and someone determined can
 * hand it false ones. This is evidence, not proof, and nothing in the system treats it as proof:
 * it does not overturn a patrol tick, mark attendance, or feed a report. Its worth is that it is
 * CONTEMPORANEOUS and SPECIFIC — a claim filed at 14:07 at a fixed point, matched to the 14:00
 * slot, is a different object from a recollection offered a week later.
 */
object PresenceCapture {

    /** How long to wait for a fresh satellite fix before falling back. Indoors a first fix can
     *  take a while, and a lecturer standing there watching a spinner will give up long before the
     *  GPS does — so we wait a reasonable moment and then say plainly what we got. */
    private const val FIX_TIMEOUT_MS = 12_000L

    /** How old a cached fix may be and still describe where the phone is NOW. Beyond this the
     *  coordinates are from somewhere the lecturer has since walked away from, and filing them
     *  would be worse than filing none: a reviewer would read a precise, confident, wrong point.
     *  Past it we file NO_FIX and let the claim stand on its timestamp alone. */
    private const val STALE_FIX_MS = 5 * 60 * 1000L

    /** Minutes either side of a timetabled slot that still count as "this lecture" — a class that
     *  started late, or ran over, or a lecturer who pressed the button while packing up. */
    private const val GRACE_MINUTES = 20

    /** What the phone managed to establish about where it is. */
    data class Fix(
        val latitude: Double?,
        val longitude: Double?,
        val accuracyMetres: Double?,
        /** OK | NO_FIX | PERMISSION_DENIED | DISABLED */
        val status: String,
    ) {
        companion object {
            fun none(status: String) = Fix(null, null, null, status)
        }
    }

    /** Which timetabled lecture the phone believes this moment belongs to. */
    data class SlotMatch(
        val slot: TimetableSlotEntity?,
        /** IN_SLOT | NEAR_SLOT | NEAREST | NONE */
        val kind: String,
        /** Signed minutes from the slot's start: negative = early, positive = late. */
        val minutesFromStart: Int?,
    )

    // ── Matching: a pure function, so it can be tested without a phone ───────────

    /**
     * Decide which of the lecturer's timetabled slots [at] falls in.
     *
     * The four answers are deliberately different things, because a reviewer needs to tell them
     * apart. IN_SLOT is the strong case. NEAR_SLOT is a class that started late or ran over.
     * NEAREST names the closest lecture that day for context, and says so rather than pretending
     * to a match. NONE means nothing was timetabled for this lecturer that day at all — which is
     * itself worth recording: it may be the lecturer who is wrong about the day.
     *
     * Slots with dayOfWeek 0 (unscheduled, or a slot the timetable never got a day for) are
     * included for the same reason the patrol round includes them: absence of a day is not
     * evidence of the wrong day, and dropping them would hide a real lecture.
     */
    fun match(slots: List<TimetableSlotEntity>, at: LocalDateTime): SlotMatch {
        val dow = at.dayOfWeek.value                       // 1 = Monday … 7 = Sunday
        val nowMin = at.hour * 60 + at.minute
        val today = slots.filter { it.dayOfWeek == dow || it.dayOfWeek == 0 }
        if (today.isEmpty()) return SlotMatch(null, "NONE", null)

        // Pair each slot with how far into it we are. A slot whose start time is unparseable is
        // dropped rather than defaulted to midnight, which would make it the "nearest" to nothing.
        val scored = today.mapNotNull { s ->
            val start = parseHhMm(s.startTime) ?: return@mapNotNull null
            Triple(s, start, nowMin - start)
        }
        if (scored.isEmpty()) return SlotMatch(null, "NONE", null)

        // Two lectures can be timetabled over each other — a double-booking, or the same lecturer
        // down for two cohorts. Prefer the one we are furthest INTO rather than the first in the
        // list, since that is the one actually being taught right now.
        val inSlot = scored.filter { (s, _, delta) -> delta >= 0 && delta < s.durationMinutes.coerceAtLeast(1) }
        if (inSlot.isNotEmpty()) {
            val best = inSlot.minByOrNull { it.third }!!
            return SlotMatch(best.first, "IN_SLOT", best.third)
        }

        val near = scored.filter { (s, _, delta) ->
            delta in -GRACE_MINUTES until 0 ||
                (delta >= s.durationMinutes && delta <= s.durationMinutes + GRACE_MINUTES)
        }
        if (near.isNotEmpty()) {
            val best = near.minByOrNull { kotlin.math.abs(it.third) }!!
            return SlotMatch(best.first, "NEAR_SLOT", best.third)
        }

        val nearest = scored.minByOrNull { kotlin.math.abs(it.third) }!!
        return SlotMatch(nearest.first, "NEAREST", nearest.third)
    }

    /** "HH:MM" → minutes since midnight, or null if it is not that. */
    private fun parseHhMm(s: String): Int? {
        val parts = s.trim().split(":")
        if (parts.size < 2) return null
        val h = parts[0].toIntOrNull() ?: return null
        val m = parts[1].toIntOrNull() ?: return null
        if (h !in 0..23 || m !in 0..59) return null
        return h * 60 + m
    }

    // ── Location ────────────────────────────────────────────────────────────────

    /**
     * Where the phone is, best effort, with the failure modes named rather than swallowed.
     *
     * Asks GPS and network together and takes whichever answers first: indoors the network
     * provider is often the only one that will, and a coarse fix that says "this building" is
     * worth incomparably more to a reviewer than nothing. Falls back to a cached fix only if it is
     * recent enough to still describe where the phone is.
     */
    suspend fun locate(context: Context): Fix = withContext(Dispatchers.Main) {
        if (ContextCompat.checkSelfPermission(context, Manifest.permission.ACCESS_FINE_LOCATION)
            != PackageManager.PERMISSION_GRANTED
        ) return@withContext Fix.none("PERMISSION_DENIED")

        val lm = context.getSystemService(Context.LOCATION_SERVICE) as? LocationManager
            ?: return@withContext Fix.none("DISABLED")

        val providers = listOf(LocationManager.GPS_PROVIDER, LocationManager.NETWORK_PROVIDER)
            .filter { runCatching { lm.isProviderEnabled(it) }.getOrDefault(false) }
        if (providers.isEmpty()) return@withContext Fix.none("DISABLED")

        val fresh = withTimeoutOrNull(FIX_TIMEOUT_MS) { awaitFix(lm, providers) }
        if (fresh != null) return@withContext fresh.toFix()

        // Nothing fresh in time. A cached fix is acceptable only while it still describes where
        // the phone is — see STALE_FIX_MS.
        val cached = providers
            .mapNotNull { p -> runCatching { lm.getLastKnownLocation(p) }.getOrNull() }
            .maxByOrNull { it.time }
        val ageMs = cached?.let { System.currentTimeMillis() - it.time }
        if (cached != null && ageMs != null && ageMs <= STALE_FIX_MS) return@withContext cached.toFix()
        Fix.none("NO_FIX")
    }

    private fun Location.toFix() = Fix(latitude, longitude, accuracy.toDouble(), "OK")

    /** First fix from any enabled provider. Every listener is removed on the way out — including
     *  on cancellation, which is what happens on the timeout above. */
    private suspend fun awaitFix(lm: LocationManager, providers: List<String>): Location? =
        suspendCancellableCoroutine { cont ->
            val listeners = mutableListOf<LocationListener>()
            fun cleanUp() {
                listeners.forEach { runCatching { lm.removeUpdates(it) } }
                listeners.clear()
            }
            providers.forEach { p ->
                val l = object : LocationListener {
                    override fun onLocationChanged(location: Location) {
                        if (cont.isActive) { cleanUp(); cont.resume(location) }
                    }
                    // Required by the pre-30 interface; a provider going quiet is not an error we
                    // can act on — the timeout is what resolves that case.
                    @Deprecated("Deprecated in Java")
                    override fun onStatusChanged(provider: String?, status: Int, extras: android.os.Bundle?) {}
                    override fun onProviderEnabled(provider: String) {}
                    override fun onProviderDisabled(provider: String) {}
                }
                listeners += l
                runCatching {
                    @Suppress("MissingPermission")
                    lm.requestLocationUpdates(p, 0L, 0f, l, Looper.getMainLooper())
                }
            }
            if (listeners.isEmpty() && cont.isActive) cont.resume(null)
            cont.invokeOnCancellation { cleanUp() }
        }

    // ── Filing ──────────────────────────────────────────────────────────────────

    /**
     * Capture and FILE, in that order, and never fail to file.
     *
     * The claim is written even when the phone could not get a fix, could not see the timetable,
     * or has no lecture timetabled at that moment. A lecturer pressing this button is making a
     * statement about where they are, and the system's job is to record that they made it, when.
     * Refusing because a satellite was slow would lose the one thing that cannot be recovered
     * later — the fact that they said it at 14:07 and not after the complaint.
     */
    suspend fun captureAndFile(context: Context, dao: AppDao, note: String = ""): PresenceClaimEntity {
        val now = LocalDateTime.now()
        val fix = locate(context)
        val slots = withContext(Dispatchers.IO) { runCatching { dao.timetableForDay(now.dayOfWeek.value) }.getOrDefault(emptyList()) }
        val m = match(slots, now)

        val claim = PresenceClaimEntity(
            id = UUID.randomUUID().toString(),
            latitude = fix.latitude, longitude = fix.longitude,
            accuracyMetres = fix.accuracyMetres, locationStatus = fix.status,
            capturedAt = Instant.now().toString(),
            sessionDate = LocalDate.now().toString(),
            unitId = m.slot?.unitId.orEmpty(),
            unitName = m.slot?.unitName.orEmpty(),
            room = m.slot?.room.orEmpty(),
            dayOfWeek = m.slot?.dayOfWeek ?: 0,
            scheduledTime = m.slot?.startTime.orEmpty(),
            matchKind = m.kind,
            minutesFromStart = m.minutesFromStart,
            note = note.trim(),
            synced = false,
        )
        withContext(Dispatchers.IO) { dao.putPresenceClaim(claim) }
        return claim
    }
}
