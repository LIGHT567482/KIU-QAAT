package ug.qaat.coordinator.net

import io.ktor.client.request.*
import io.ktor.client.statement.*
import io.ktor.http.*
import org.json.JSONArray
import org.json.JSONObject

/**
 * Online coordinator-dashboard data — the SAME endpoints the coordinator PWA
 * (apps/coordinator-pwa/src/pages/Dashboard.tsx) is built from, so the native app can
 * depict the same home: cohort card, timetable, units, students, last roster, the
 * live-sessions panel and emergency standby. Used while online; the in-room session
 * flow stays fully offline via the manifest + local engine.
 *
 * Shapes mirror services/api-gateway/internal/handlers/coordinator_dashboard.go.
 */
class DashboardClient {
    private val http = Net.client()
    private val base = Net.baseUrl

    data class Offering(
        val courseName: String, val sessionType: String, val studyYear: Int,
        val semester: Int, val level: String, val intake: String,
    )
    data class Unit(
        val unitId: String, val name: String, val year: Int, val semester: Int,
        val dayOfWeek: Int, val start: String, val durationMin: Int, val lecturers: String, val room: String,
        val lecturerPhone: String = "",
    )
    data class Overview(val offering: Offering?, val units: List<Unit>)
    data class Student(
        val studentId: String, val fullName: String, val year: Int, val semester: Int,
        val intake: String, val status: String,
    )
    data class RosterRow(val studentId: String, val fullName: String, val status: String, val checkinTime: String)
    data class LastRoster(val unitName: String, val date: String, val present: Int, val total: Int, val roster: List<RosterRow>)
    data class ActiveSession(val sessionId: String, val unitName: String, val status: String, val presentCount: Int, val gateOpen: String)
    data class Standby(val id: String, val code: String, val deputyReg: String, val deputyName: String, val expiresAt: String)

    private fun HttpRequestBuilder.auth(token: String) = header("Authorization", "Bearer $token")

    suspend fun overview(token: String): Overview {
        val r = http.get("$base/api/v1/coordinator/overview") { auth(token) }
        if (r.status.value !in 200..299) return Overview(null, emptyList())
        val j = JSONObject(r.bodyAsText())
        val offJson = j.optJSONObject("offering")
        val off = offJson?.let {
            Offering(it.optString("course_name"), it.optString("session_type"), it.optInt("study_year"),
                it.optInt("semester"), it.optString("level"), it.optString("intake"))
        }
        // Prefer the richer per-unit timetable slots when present (like the PWA does).
        val slots = j.optJSONArray("slots")
        val units = ArrayList<Unit>()
        if (slots != null && slots.length() > 0) {
            for (i in 0 until slots.length()) {
                val s = slots.getJSONObject(i)
                units.add(Unit(s.optString("unit_id"), s.optString("unit_name"), 0, 0,
                    s.optInt("day_of_week"), s.optString("start_time"), s.optInt("duration_minutes"),
                    s.optString("lecturer_name"), s.optString("room"), s.optString("lecturer_phone")))
            }
        } else j.optJSONArray("units")?.let { arr ->
            for (i in 0 until arr.length()) {
                val u = arr.getJSONObject(i)
                units.add(Unit(u.optString("unit_id"), u.optString("name"), u.optInt("year"), u.optInt("semester"),
                    u.optInt("day_of_week"), u.optString("session_start"), u.optInt("session_duration_minutes"),
                    u.optString("lecturers"), ""))
            }
        }
        return Overview(off, units)
    }

    suspend fun students(token: String): List<Student> {
        val r = http.get("$base/api/v1/coordinator/students") { auth(token) }
        if (r.status.value !in 200..299) return emptyList()
        val arr = JSONArray(r.bodyAsText())
        return (0 until arr.length()).map { i ->
            val s = arr.getJSONObject(i)
            Student(s.optString("student_id"), s.optString("full_name"), s.optInt("current_year"),
                s.optInt("semester"), s.optString("intake_session"), s.optString("enrollment_status"))
        }
    }

    suspend fun lastRoster(token: String): LastRoster? {
        val r = http.get("$base/api/v1/coordinator/last-roster") { auth(token) }
        if (r.status.value !in 200..299) return null
        val j = JSONObject(r.bodyAsText())
        val s = j.optJSONObject("session") ?: return null
        val rosterArr = j.optJSONArray("roster") ?: JSONArray()
        val rows = (0 until rosterArr.length()).map { i ->
            val e = rosterArr.getJSONObject(i)
            RosterRow(e.optString("student_id"), e.optString("full_name"), e.optString("status"), e.optString("checkin_time"))
        }
        return LastRoster(s.optString("unit_name"), s.optString("session_date"), s.optInt("present"), s.optInt("total"), rows)
    }

    suspend fun activeSessions(token: String): List<ActiveSession> {
        val r = http.get("$base/api/v1/coordinator/active-sessions") { auth(token) }
        if (r.status.value !in 200..299) return emptyList()
        val arr = JSONObject(r.bodyAsText()).optJSONArray("sessions") ?: return emptyList()
        return (0 until arr.length()).map { i ->
            val s = arr.getJSONObject(i)
            ActiveSession(s.optString("session_id"), s.optString("unit_name"), s.optString("status"),
                s.optInt("present_count"), s.optString("gate_open_time"))
        }
    }

    suspend fun closeSession(token: String, sessionId: String): Boolean =
        http.post("$base/api/v1/sessions/$sessionId/close") { auth(token) }.status.value in 200..299

    suspend fun standby(token: String): List<Standby> {
        val r = http.get("$base/api/v1/coordinator/standby") { auth(token) }
        if (r.status.value !in 200..299) return emptyList()
        val arr = JSONArray(r.bodyAsText())
        return (0 until arr.length()).map { i ->
            val s = arr.getJSONObject(i)
            Standby(s.optString("delegation_id"), s.optString("code"), s.optString("deputy_reg"),
                s.optString("deputy_name"), s.optString("expires_at"))
        }
    }

    /** @return error message on failure, null on success. */
    suspend fun issueStandby(token: String, deputyReg: String): String? {
        val r = http.post("$base/api/v1/coordinator/standby") {
            auth(token); contentType(ContentType.Application.Json)
            setBody(JSONObject().put("deputy_reg", deputyReg).toString())
        }
        if (r.status.value in 200..299) return null
        return JSONObject(r.bodyAsText()).optString("message", "Could not issue standby code")
    }

    suspend fun revokeStandby(token: String, id: String): Boolean =
        http.post("$base/api/v1/coordinator/standby/$id/revoke") { auth(token) }.status.value in 200..299
}
