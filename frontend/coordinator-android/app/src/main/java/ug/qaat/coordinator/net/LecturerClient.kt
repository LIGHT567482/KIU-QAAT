package ug.qaat.coordinator.net

import io.ktor.client.request.*
import io.ktor.client.statement.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import ug.qaat.coordinator.ui.AppState

/** Lecturer roster & analytics over the cloud API (online): his units, the students who study
 *  them (enrolled or only-attended), his sessions, and per-session present/absent. */
class LecturerClient {
    private val http = Net.client()
    private val base = Net.baseUrl

    data class Unit(val unitId: String, val name: String)
    data class RosterRow(
        val studentId: String, val fullName: String, val unitId: String, val unitName: String,
        val cohort: String, val courseName: String, val level: String, val attendedCount: Int,
    )
    data class Session(
        val sessionId: String, val unitId: String, val unitName: String, val date: String,
        val dayOfWeek: Int, val cohort: String, val present: Int, val enrolled: Int, val status: String,
    )
    data class SessStudent(val studentId: String, val fullName: String, val present: Boolean, val checkinAt: String)

    private suspend fun getArray(path: String): JSONArray? = withContext(Dispatchers.IO) {
        val token = AppState.token ?: return@withContext null
        runCatching {
            val r = http.get("$base$path") { header("Authorization", "Bearer $token") }
            if (r.status.value !in 200..299) null else JSONArray(r.bodyAsText())
        }.getOrNull()
    }

    suspend fun units(): List<Unit> = withContext(Dispatchers.IO) {
        val token = AppState.token ?: return@withContext emptyList()
        runCatching {
            val r = http.get("$base/api/v1/lecturer/overview") { header("Authorization", "Bearer $token") }
            if (r.status.value !in 200..299) return@runCatching emptyList<Unit>()
            val arr = JSONObject(r.bodyAsText()).optJSONArray("units") ?: return@runCatching emptyList<Unit>()
            (0 until arr.length()).map { i -> val o = arr.getJSONObject(i); Unit(o.getString("unit_id"), o.optString("name")) }
                .distinctBy { it.unitId }
        }.getOrDefault(emptyList())
    }

    suspend fun roster(scope: String, unitId: String?): List<RosterRow> {
        val q = buildString { append("?scope="); append(scope); if (!unitId.isNullOrBlank()) { append("&unit_id="); append(unitId) } }
        val arr = getArray("/api/v1/lecturer/roster$q") ?: return emptyList()
        return (0 until arr.length()).map { i ->
            val o = arr.getJSONObject(i)
            RosterRow(o.optString("student_id"), o.optString("full_name"), o.optString("unit_id"), o.optString("unit_name"),
                o.optString("cohort"), o.optString("course_name"), o.optString("unit_level"), o.optInt("attended_count"))
        }
    }

    suspend fun sessions(unitId: String?): List<Session> {
        val q = if (unitId.isNullOrBlank()) "" else "?unit_id=$unitId"
        val arr = getArray("/api/v1/lecturer/sessions$q") ?: return emptyList()
        return (0 until arr.length()).map { i ->
            val o = arr.getJSONObject(i)
            Session(o.optString("session_id"), o.optString("unit_id"), o.optString("unit_name"), o.optString("session_date"),
                o.optInt("day_of_week"), o.optString("cohort"), o.optInt("present_count"), o.optInt("enrolled_count"), o.optString("status"))
        }
    }

    suspend fun sessionStudents(sessionId: String, status: String): List<SessStudent> = withContext(Dispatchers.IO) {
        val token = AppState.token ?: return@withContext emptyList()
        runCatching {
            val r = http.get("$base/api/v1/lecturer/sessions/$sessionId/students?status=$status") { header("Authorization", "Bearer $token") }
            if (r.status.value !in 200..299) return@runCatching emptyList<SessStudent>()
            val arr = JSONObject(r.bodyAsText()).optJSONArray("students") ?: return@runCatching emptyList<SessStudent>()
            (0 until arr.length()).map { i ->
                val o = arr.getJSONObject(i)
                SessStudent(o.optString("student_id"), o.optString("full_name"), o.optBoolean("present"), o.optString("checkin_at"))
            }
        }.getOrDefault(emptyList())
    }
}
