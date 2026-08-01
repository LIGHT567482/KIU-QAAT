package ug.qaat.patroller.net

import io.ktor.client.request.*
import io.ktor.client.statement.*
import io.ktor.http.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import ug.qaat.patroller.db.PatrolLogEntity
import ug.qaat.patroller.db.PatrolSlotEntity

/** Cloud calls for the patroller app: login, fetch today's timetable, upload queued patrol logs. */
class PatrolClient {
    private val http = Net.client()
    private val base = Net.baseUrl

    data class Login(val token: String, val role: String, val fullName: String, val staffId: String, val forceChange: Boolean)

    /** POST /api/v1/auth/app-login {identifier, password}. */
    suspend fun login(identifier: String, password: String): Login = withContext(Dispatchers.IO) {
        val r = http.post("$base/api/v1/auth/app-login") {
            contentType(ContentType.Application.Json)
            setBody(JSONObject().put("identifier", identifier).put("password", password).toString())
        }
        val j = JSONObject(r.bodyAsText())
        require(r.status.value in 200..299) { j.optString("message").ifBlank { "Sign-in failed (${r.status.value})." } }
        Login(
            token = j.getString("access_token"),
            role = j.optString("role", ""),
            fullName = j.optString("full_name", ""),
            staffId = j.optString("staff_id", ""),
            forceChange = j.optBoolean("force_password_change", false),
        )
    }

    /** GET /api/v1/patrol/manifest — today's timetabled slots. */
    suspend fun manifest(token: String): List<PatrolSlotEntity> = withContext(Dispatchers.IO) {
        val r = http.get("$base/api/v1/patrol/manifest") { header("Authorization", "Bearer $token") }
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

    /** POST /api/v1/patrol/sync {logs:[…]} — returns true on success. */
    suspend fun sync(token: String, logs: List<PatrolLogEntity>): Boolean = withContext(Dispatchers.IO) {
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
            header("Authorization", "Bearer $token")
            contentType(ContentType.Application.Json)
            setBody(JSONObject().put("logs", arr).toString())
        }
        r.status.isSuccess()
    }
}
