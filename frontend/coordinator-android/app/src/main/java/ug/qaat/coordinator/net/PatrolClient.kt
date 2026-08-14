package ug.qaat.coordinator.net

import io.ktor.client.request.*
import io.ktor.client.statement.*
import io.ktor.http.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import ug.qaat.coordinator.db.PatrolLogEntity
import ug.qaat.coordinator.db.PatrolSlotEntity
import ug.qaat.coordinator.ui.AppState

/**
 * The QA monitor's cloud calls: claim this handset, pull today's timetable, push the round.
 *
 * Every call carries `X-Device-Fingerprint`. A monitor record accuses a named lecturer of not
 * having taught, so the gateway will only accept one from the handset the monitor claimed —
 * a stolen or shared token replayed from another phone is refused with DEVICE_NOT_BOUND. The
 * client cannot fake this usefully: the server stores the first fingerprint it sees for that
 * account and compares, so the worst a tampered client achieves is locking itself out.
 */
class PatrolClient {
    private val http = Net.client()
    private val base = Net.baseUrl

    /** Raised when the gateway rejects this handset. Carries the message the monitor should see. */
    class DeviceRejected(message: String) : Exception(message)

    private fun HttpRequestBuilder.auth(token: String, fingerprint: String) {
        header("Authorization", "Bearer $token")
        header("X-Device-Fingerprint", fingerprint)
    }

    /**
     * Claim this handset for the signed-in monitor. First call binds; later calls from the SAME
     * phone are a no-op confirmation; a call from a different phone is refused until an admin
     * releases the binding. Returns null when the server accepted it.
     */
    suspend fun bindDevice(token: String, fingerprint: String): String? = withContext(Dispatchers.IO) {
        val r = http.post("$base/api/v1/patrol/bind-device") {
            auth(token, fingerprint)
            contentType(ContentType.Application.Json)
            setBody(JSONObject().put("device_fingerprint", fingerprint).toString())
        }
        if (r.status.value in 200..299) return@withContext null
        runCatching { JSONObject(r.bodyAsText()).optString("message") }.getOrNull()
            ?.takeIf { it.isNotBlank() }
            ?: "This phone is not authorised for monitoring (${r.status.value})."
    }

    // ── The monitor PIN: the second factor in front of the round ──────────────────
    //
    // The handset binding above answers "which phone"; the PIN answers "who is holding it". The
    // PIN itself never comes back from the server — only whether one has been set — so the app
    // cannot cache it, show it, or check it locally.

    /** What the PIN screen should show: set it, ask for it, or report the lockout. */
    data class PinState(val isSet: Boolean, val locked: Boolean, val lockedUntil: String, val attemptsLeft: Int)

    suspend fun pinState(): PinState? = withContext(Dispatchers.IO) {
        val token = AppState.token ?: return@withContext null
        runCatching {
            val r = http.get("$base/api/v1/patrol/pin") { header("Authorization", "Bearer $token") }
            if (r.status.value !in 200..299) return@runCatching null
            val o = JSONObject(r.bodyAsText())
            PinState(
                o.optBoolean("pin_set"), o.optBoolean("locked"),
                o.optString("locked_until"), o.optInt("attempts_left", 5),
            )
        }.getOrNull()
    }

    /** Set the PIN (first time) or change it. Returns null on success, else a message to show. */
    suspend fun setPin(pin: String, currentPin: String? = null): String? = withContext(Dispatchers.IO) {
        val token = AppState.token ?: return@withContext "Not signed in"
        runCatching {
            val body = JSONObject().put("pin", pin)
            if (!currentPin.isNullOrBlank()) body.put("current_pin", currentPin)
            val r = http.post("$base/api/v1/patrol/pin") {
                header("Authorization", "Bearer $token"); contentType(ContentType.Application.Json)
                setBody(body.toString())
            }
            if (r.status.value in 200..299) null
            else JSONObject(r.bodyAsText()).optString("message").ifBlank { "Couldn't save your PIN." }
        }.getOrElse { "Couldn't reach the server. You need to be online to set your PIN." }
    }

    /** Result of an attempt: null message = the round may open. */
    data class PinAttempt(val ok: Boolean, val message: String, val attemptsLeft: Int, val locked: Boolean)

    suspend fun verifyPin(pin: String): PinAttempt = withContext(Dispatchers.IO) {
        val token = AppState.token
            ?: return@withContext PinAttempt(false, "Your session has ended. Sign in again.", 0, false)
        runCatching {
            val r = http.post("$base/api/v1/patrol/pin/verify") {
                header("Authorization", "Bearer $token"); contentType(ContentType.Application.Json)
                setBody(JSONObject().put("pin", pin).toString())
            }
            if (r.status.value in 200..299) return@runCatching PinAttempt(true, "", 5, false)
            val o = JSONObject(r.bodyAsText())
            PinAttempt(
                ok = false,
                message = o.optString("message").ifBlank { "That PIN is not right." },
                attemptsLeft = o.optInt("attempts_left", 0),
                locked = o.optString("error") == "PIN_LOCKED",
            )
        }.getOrElse {
            // The PIN is verified server-side ON PURPOSE — a locally-checkable secret on a stolen
            // handset is no secret. So being offline here means the round cannot open, and the
            // monitor is told plainly rather than left tapping.
            PinAttempt(false, "You need to be online to unlock monitoring. Find signal and try again.", 0, false)
        }
    }

    /** GET /api/v1/patrol/manifest — today's timetabled slots. */
    suspend fun manifest(token: String, fingerprint: String): List<PatrolSlotEntity> = withContext(Dispatchers.IO) {
        val r = http.get("$base/api/v1/patrol/manifest") { auth(token, fingerprint) }
        if (r.status.value == 403) throw DeviceRejected(deviceMessage(r.bodyAsText()))
        require(r.status.value in 200..299) { "manifest fetch failed (${r.status.value})" }
        val arr = JSONObject(r.bodyAsText()).optJSONArray("slots") ?: JSONArray()
        (0 until arr.length()).map { i ->
            val o = arr.getJSONObject(i)
            PatrolSlotEntity(
                unitId = o.optString("unit_id"),
                unitName = o.optString("unit_name", o.optString("unit_id")),
                courseCode = o.optString("course_code", ""),
                lecturerStaffId = o.optString("lecturer_staff_id", ""),
                lecturerName = o.optString("lecturer_name", ""),
                room = o.optString("room", ""),
                dayOfWeek = o.optInt("day_of_week", 0),
                startTime = o.optString("start_time", ""),
                durationMinutes = o.optInt("duration_minutes", 60),
                offeringId = o.optString("offering_id", ""),
                cohort = o.optString("cohort", ""),
                alsoHere = alsoHereOf(o),
            )
        }.filter { it.unitId.isNotBlank() && it.startTime.isNotBlank() }
    }

    /**
     * GET /api/v1/patrol/search?by=lecturer|unit&q=…
     *
     * The round shows nothing until this returns something. Searching is what makes a
     * tick evidence of having visited a room rather than of having scrolled a list —
     * see the handler comment for why the browse-everything screen was removed.
     *
     * Returns null when the phone is offline, which the caller distinguishes from an
     * empty result: "no network" and "no such lecturer" need different words on screen.
     */
    suspend fun search(token: String, fingerprint: String, mode: String, query: String): List<PatrolSlotEntity>? =
        withContext(Dispatchers.IO) {
            val q = query.trim()
            if (q.isEmpty()) return@withContext emptyList()
            val r = runCatching {
                http.get("$base/api/v1/patrol/search") {
                    auth(token, fingerprint)
                    url { parameters.append("by", mode); parameters.append("q", q) }
                }
            }.getOrNull() ?: return@withContext null
            if (r.status.value == 403) throw DeviceRejected(deviceMessage(r.bodyAsText()))
            if (!r.status.isSuccess()) return@withContext null
            val arr = JSONObject(r.bodyAsText()).optJSONArray("results") ?: JSONArray()
            (0 until arr.length()).map { i ->
                val o = arr.getJSONObject(i)
                PatrolSlotEntity(
                    unitId = o.optString("unit_id"),
                    unitName = o.optString("unit_name", o.optString("unit_id")),
                    courseCode = o.optString("course_code", ""),
                    lecturerStaffId = o.optString("lecturer_staff_id", ""),
                    lecturerName = o.optString("lecturer_name", ""),
                    room = o.optString("room", ""),
                    dayOfWeek = o.optInt("day_of_week", 0),
                    startTime = o.optString("start_time", ""),
                    durationMinutes = o.optInt("duration_minutes", 60),
                    offeringId = o.optString("offering_id", ""),
                    cohort = o.optString("cohort", ""),
                    alsoHere = alsoHereOf(o),
                )
            }.filter { it.unitId.isNotBlank() }
        }

    /** POST /api/v1/patrol/sync {logs:[…]} — true once the batch is durably stored. */
    suspend fun sync(token: String, fingerprint: String, logs: List<PatrolLogEntity>): Boolean = withContext(Dispatchers.IO) {
        if (logs.isEmpty()) return@withContext true
        val arr = JSONArray()
        logs.forEach { l ->
            arr.put(JSONObject()
                .put("unit_id", l.unitId).put("unit_name", l.unitName).put("course_code", l.courseCode)
                .put("lecturer_id", l.lecturerId).put("lecturer_name", l.lecturerName).put("room", l.room)
                .put("session_date", l.sessionDate).put("scheduled_time", l.scheduledTime)
                .put("taught", l.taught).put("taken_at", l.takenAt)
                .put("offering_id", l.offeringId)
                .put("found_venue", l.foundVenue).put("found_start_time", l.foundStartTime)
                .put("found_date", l.foundDate).put("venue_changed", l.venueChanged)
                .put("remarks", l.remarks)
                .put("is_compensation", l.isCompensation)
                .put("compensation_for", l.compensationFor))
        }
        val r = http.post("$base/api/v1/patrol/sync") {
            auth(token, fingerprint)
            contentType(ContentType.Application.Json)
            setBody(JSONObject().put("logs", arr).toString())
        }
        if (r.status.value == 403) throw DeviceRejected(deviceMessage(r.bodyAsText()))
        r.status.isSuccess()
    }

    private fun deviceMessage(body: String): String =
        runCatching { JSONObject(body).optString("message") }.getOrNull()
            ?.takeIf { it.isNotBlank() }
            ?: "This phone is not the one registered for your monitor account."

    companion object {
        /** The signed-in monitor's token, or null when the session has gone. */
        val token: String? get() = AppState.token
    }

    /** GET /api/v1/patrol/reference — the pick-lists, in one call so the form cannot half-load. */
    suspend fun reference(token: String, fingerprint: String): PatrolReference = withContext(Dispatchers.IO) {
        val r = http.get("$base/api/v1/patrol/reference") { auth(token, fingerprint) }
        if (!r.status.isSuccess()) return@withContext PatrolReference()
        val o = JSONObject(r.bodyAsText())
        fun arr(key: String) = o.optJSONArray(key) ?: JSONArray()
        val rooms = (0 until arr("rooms").length()).map {
            val x = arr("rooms").getJSONObject(it)
            RefItem(x.optString("venue_id"), x.optString("name").ifBlank { x.optString("venue_id") },
                x.optString("building"))
        }
        val defaults = mutableMapOf<String, Pair<String, String>>()
        val depts = mutableMapOf<String, String>()
        val units = (0 until arr("units").length()).map {
            val x = arr("units").getJSONObject(it)
            defaults[x.optString("unit_id")] = x.optString("class_group") to x.optString("school")
            depts[x.optString("unit_id")] = x.optString("department")
            RefItem(x.optString("unit_id"), x.optString("unit_name").ifBlank { x.optString("unit_id") },
                x.optString("course_id"))
        }
        val lecturers = (0 until arr("lecturers").length()).map {
            val x = arr("lecturers").getJSONObject(it)
            RefItem(x.optString("staff_id"), x.optString("full_name").ifBlank { x.optString("staff_id") },
                x.optString("department"))
        }
        val schools = (0 until arr("schools").length()).map { arr("schools").getString(it) }
        PatrolReference(rooms, units, lecturers, schools, defaults, depts)
    }

    /**
     * POST /api/v1/patrol/manual — file a lecture that is not on the timetable.
     *
     * Unlike a tick, this is sent immediately rather than queued: the monitor is describing a
     * lecture from scratch, and holding that in a local queue means the one record nobody else
     * has is also the one most likely to be lost with the handset. Returns the server's message
     * on refusal so the form can say WHICH field it wants, rather than "failed".
     */
    suspend fun manual(token: String, fingerprint: String, e: ManualEntry): String? = withContext(Dispatchers.IO) {
        val payload = JSONObject()
            .put("room_id", e.roomId).put("room", e.room)
            .put("unit_id", e.unitId).put("unit_name", e.unitName)
            .put("lecturer_staff_id", e.lecturerStaffId).put("lecturer_name", e.lecturerName)
            .put("class_group", e.classGroup).put("school", e.school).put("department", e.department)
            .put("students_counted", e.studentsCounted)
            .put("session_date", e.sessionDate)
            .put("time_of_day", e.timeOfDay).put("end_time", e.endTime)
            .put("taught", e.taught).put("remarks", e.remarks)
            .put("is_compensation", e.isCompensation)
            .put("compensation_for_at", e.compensationForAt)
            .put("also_units", org.json.JSONArray().apply {
                e.alsoUnits.forEach {
                    put(JSONObject()
                        .put("unit_id", it.unitId).put("unit_name", it.unitName)
                        .put("class_group", it.classGroup).put("school", it.school)
                        .put("department", it.department))
                }
            })
        val r = http.post("$base/api/v1/patrol/manual") {
            auth(token, fingerprint); contentType(ContentType.Application.Json); setBody(payload.toString())
        }
        if (r.status.value == 403) throw DeviceRejected(deviceMessage(r.bodyAsText()))
        if (r.status.isSuccess()) null
        else runCatching { JSONObject(r.bodyAsText()).optString("message") }.getOrNull()
            ?.takeIf { it.isNotBlank() } ?: "Could not record it (${r.status.value})"
    }

}

/**
 * The reference lists behind the manual-entry form, and the call that files one.
 *
 * Kept in this file because they belong to the same round as the search and the sync above, and
 * because they share its device header: a manual record accuses a named lecturer exactly as a tick
 * does, and it must be answerable to the same handset.
 */

/**
 * The other unit codes running in this same hour, room and lecturer, flattened to one line.
 *
 * Flattened rather than modelled because it is only ever read: the monitor needs to SEE that the
 * hour also covers BIT 3110 and CSC 3103 before they tick it, and one string survives the offline
 * cache without a second table. The server sends the structure; nothing here needs to query it.
 */
private fun alsoHereOf(o: JSONObject): String {
    val arr = o.optJSONArray("also_here") ?: return ""
    return (0 until arr.length()).mapNotNull { i ->
        val u = arr.optJSONObject(i) ?: return@mapNotNull null
        val code = u.optString("unit_id")
        val name = u.optString("unit_name")
        val cohort = u.optString("cohort")
        listOfNotNull(
            code.takeIf { it.isNotBlank() },
            name.takeIf { it.isNotBlank() && !it.equals(code, true) },
        ).joinToString(" — ").let { if (cohort.isBlank()) it else "$it · $cohort" }
            .takeIf { it.isNotBlank() }
    }.joinToString(" | ")
}

/** One entry of a pick-list. `id` is what the server keys on, `label` what the monitor reads. */
data class RefItem(val id: String, val label: String, val extra: String = "")

/** Everything the manual form offers to pick from, fetched in ONE call and cached for the round. */
data class PatrolReference(
    val rooms: List<RefItem> = emptyList(),
    val units: List<RefItem> = emptyList(),
    val lecturers: List<RefItem> = emptyList(),
    val schools: List<String> = emptyList(),
    /** unit_id → the class/group and college the curriculum already knows, so picking a unit
     *  fills them in and the monitor is not asked to retype what the system has. */
    val unitDefaults: Map<String, Pair<String, String>> = emptyMap(),
    /** unit_id → the DEPARTMENT that owns it. Two unit codes for one lecture usually share a
     *  college and differ by department, so the department is the field that tells them apart. */
    val unitDepartments: Map<String, String> = emptyMap(),
)

/** One extra course unit the same hour of teaching also delivered. */
data class ExtraUnit(
    val unitId: String = "", val unitName: String = "",
    val classGroup: String = "", val school: String = "", val department: String = "",
)

/** What the monitor saw, for a lecture with nothing on the timetable to tick. */
data class ManualEntry(
    val roomId: String = "", val room: String = "",
    val unitId: String = "", val unitName: String = "",
    val lecturerStaffId: String = "", val lecturerName: String = "",
    val classGroup: String = "", val school: String = "", val department: String = "",
    val studentsCounted: Int = 0,
    val sessionDate: String = "",
    /** When the lecture BEGAN and when it was due to END, both HH:MM. */
    val timeOfDay: String = "", val endTime: String = "",
    val taught: Boolean = true, val remarks: String = "",
    val isCompensation: Boolean = false,
    /** The date AND START TIME of the lecture being made good, "YYYY-MM-DD HH:MM". Required
     *  whenever isCompensation is set — the server refuses the record without it, because a
     *  compensation nobody can match to a missed lecture is a claim rather than a record. */
    val compensationForAt: String = "",
    /** The other unit codes this same hour delivered. */
    val alsoUnits: List<ExtraUnit> = emptyList(),
)
