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
 * The QA patroller's cloud calls: claim this handset, pull today's timetable, push the round.
 *
 * Every call carries `X-Device-Fingerprint`. A patrol record accuses a named lecturer of not
 * having taught, so the gateway will only accept one from the handset the patroller claimed —
 * a stolen or shared token replayed from another phone is refused with DEVICE_NOT_BOUND. The
 * client cannot fake this usefully: the server stores the first fingerprint it sees for that
 * account and compares, so the worst a tampered client achieves is locking itself out.
 */
class PatrolClient {
    private val http = Net.client()
    private val base = Net.baseUrl

    /** Raised when the gateway rejects this handset. Carries the message the patroller should see. */
    class DeviceRejected(message: String) : Exception(message)

    private fun HttpRequestBuilder.auth(token: String, fingerprint: String) {
        header("Authorization", "Bearer $token")
        header("X-Device-Fingerprint", fingerprint)
    }

    /**
     * Claim this handset for the signed-in patroller. First call binds; later calls from the SAME
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
            ?: "This phone is not authorised for patrol (${r.status.value})."
    }

    // ── The patrol PIN: the second factor in front of the round ──────────────────
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
            // patroller is told plainly rather than left tapping.
            PinAttempt(false, "You need to be online to unlock patrol. Find signal and try again.", 0, false)
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
            )
        }.filter { it.unitId.isNotBlank() && it.startTime.isNotBlank() }
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
                .put("taught", l.taught).put("taken_at", l.takenAt))
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
            ?: "This phone is not the one registered for your patrol account."

    companion object {
        /** The signed-in patroller's token, or null when the session has gone. */
        val token: String? get() = AppState.token
    }
}
