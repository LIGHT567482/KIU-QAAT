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
