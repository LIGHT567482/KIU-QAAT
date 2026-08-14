package ug.qaat.coordinator.net

import io.ktor.client.request.*
import io.ktor.client.statement.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject
import ug.qaat.coordinator.ui.AppState

/**
 * The lecturer's WEEK, with the cohort attached to every slot.
 *
 * GET /api/v1/lecturer/timetable is the same endpoint [PresenceClient] already calls to cache the
 * week for offline slot-matching — deliberately the same one. Two endpoints answering "when does
 * this lecturer teach" would drift, and the screen a lecturer plans from would eventually disagree
 * with the week their phone matches a presence claim against. PresenceClient reads the handful of
 * fields it needs and ignores the rest; this reads all of them.
 */
class LecturerTimetableClient {
    private val http = Net.client()
    private val base = Net.baseUrl

    data class Slot(
        val unitId: String,
        /** Which cohort. Two e-learning runs of one unit are only distinguishable by it, and the
         *  online-class endpoint refuses to guess between them rather than put half a cohort's
         *  attendance under the other's roster. */
        val offeringId: String,
        val unitName: String,
        val courseName: String,
        val sessionType: String,      // Day · Evening · Weekend · Distance Learning …
        val level: String,
        val intake: String,
        val studyYear: Int,
        val semester: Int,
        val dayOfWeek: Int,           // 1 = Mon … 7 = Sun
        val startTime: String,        // "HH:MM"
        val durationMinutes: Int,
        val room: String,
        val building: String,
        /** ONLINE = a distance / e-learning cohort: no room, started by the lecturer themselves. */
        val deliveryMode: String,
        /** False when the timetable names nobody on the slot and they are here via the unit
         *  assignment — which may mean somebody else is covering it. */
        val namedOnSlot: Boolean,
        val enrolled: Int,
    ) {
        val online: Boolean get() = deliveryMode == "ONLINE"
        /** "Day · Y2 S1 · August Intake" — what tells two slots of the same unit apart. */
        val cohort: String get() = listOf(
            sessionType,
            if (studyYear > 0 && semester > 0) "Y$studyYear S$semester" else "",
            level, intake,
        ).filter { it.isNotBlank() }.joinToString(" · ")
    }

    /** Returns null when the call did not succeed, so the caller can fall back to its cache rather
     *  than show an empty week — which a lecturer would read as "nothing timetabled". */
    suspend fun week(): List<Slot>? = withContext(Dispatchers.IO) {
        val token = AppState.token ?: return@withContext null
        runCatching {
            val r = http.get("$base/api/v1/lecturer/timetable") {
                header("Authorization", "Bearer $token")
            }
            if (r.status.value !in 200..299) return@runCatching null
            val arr = JSONObject(r.bodyAsText()).optJSONArray("slots") ?: return@runCatching null
            (0 until arr.length()).map { i ->
                val o = arr.getJSONObject(i)
                Slot(
                    unitId = o.optString("unit_id"),
                    offeringId = o.optString("offering_id"),
                    unitName = o.optString("unit_name").ifBlank { o.optString("unit_id") },
                    courseName = o.optString("course_name"),
                    sessionType = o.optString("session_type"),
                    level = o.optString("level"),
                    intake = o.optString("intake"),
                    studyYear = o.optInt("study_year"),
                    semester = o.optInt("semester"),
                    dayOfWeek = o.optInt("day_of_week"),
                    startTime = o.optString("start_time"),
                    durationMinutes = o.optInt("duration_minutes", 60),
                    room = o.optString("room"),
                    building = o.optString("building"),
                    deliveryMode = o.optString("delivery_mode", "IN_PERSON"),
                    namedOnSlot = o.optBoolean("named_on_slot"),
                    enrolled = o.optInt("enrolled"),
                )
            }.filter { it.startTime.isNotBlank() && it.dayOfWeek in 1..7 }
        }.getOrNull()
    }
}
