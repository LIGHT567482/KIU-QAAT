package ug.qaat.coordinator.net

import io.ktor.client.request.*
import io.ktor.client.statement.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject
import ug.qaat.coordinator.ui.AppState

/** The student's Home tab: their full profile, their cohort's units and its weekly timetable.
 *  One round trip, scoped server-side to the student's own cohort. */
class StudentHomeClient {
    private val http = Net.client()
    private val base = Net.baseUrl

    data class Slot(
        val unitId: String, val unitName: String, val dayOfWeek: Int,
        val startTime: String, val durationMinutes: Int, val room: String, val lecturerName: String,
    )

    /** One unit on the student's programme. `current` marks the year/semester they are sitting. */
    data class Unit(
        val unitId: String, val unitName: String, val lecturerName: String,
        val year: Int, val semester: Int, val current: Boolean,
    )

    data class Home(
        val studentId: String, val fullName: String, val email: String,
        val course: String, val level: String, val intake: String, val sessionType: String,
        val academicYear: String, val year: Int, val semester: Int, val cohort: String,
        val units: List<Unit>, val timetable: List<Slot>,
    )

    suspend fun fetch(): Home? = withContext(Dispatchers.IO) {
        val token = AppState.token ?: return@withContext null
        runCatching {
            val r = http.get("$base/api/v1/student/home") { header("Authorization", "Bearer $token") }
            if (r.status.value !in 200..299) return@runCatching null
            val o = JSONObject(r.bodyAsText())

            val units = o.optJSONArray("units")?.let { arr ->
                (0 until arr.length()).map { i ->
                    val u = arr.getJSONObject(i)
                    Unit(
                        u.optString("unit_id"), u.optString("unit_name"), u.optString("lecturer_name"),
                        u.optInt("year"), u.optInt("semester"), u.optBoolean("current", true),
                    )
                }
            } ?: emptyList()

            val slots = o.optJSONArray("timetable")?.let { arr ->
                (0 until arr.length()).map { i ->
                    val s = arr.getJSONObject(i)
                    Slot(
                        s.optString("unit_id"), s.optString("unit_name"), s.optInt("day_of_week"),
                        s.optString("start_time"), s.optInt("duration_minutes"),
                        s.optString("room"), s.optString("lecturer_name"),
                    )
                }
            } ?: emptyList()

            Home(
                o.optString("student_id"), o.optString("full_name"), o.optString("email"),
                o.optString("course"), o.optString("level"), o.optString("intake"),
                o.optString("session_type"), o.optString("academic_year"),
                o.optInt("year"), o.optInt("semester"), o.optString("cohort"), units, slots,
            )
        }.getOrNull()
    }
}
