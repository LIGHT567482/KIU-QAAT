package ug.qaat.coordinator.net

import io.ktor.client.request.*
import io.ktor.client.statement.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject
import ug.qaat.coordinator.ui.AppState

/** The lecturer's unit-centric teaching calendar: one entry per timetabled unit session, with the
 *  cohorts attending it and their attendance broken out separately. */
class LecturerCalendarClient {
    private val http = Net.client()
    private val base = Net.baseUrl

    data class Cohort(
        val offeringId: String, val label: String, val sessionType: String,
        val coordinatorName: String, val enrolled: Int, val present: Int, val pct: Double,
    )

    data class Event(
        val date: String, val unitId: String, val unitName: String,
        val startTime: String, val durationMinutes: Int, val room: String,
        val held: Boolean, val status: String,
        val enrolled: Int, val present: Int, val pct: Double,
        val cohorts: List<Cohort>,
        /** Whether the LECTURER gated in for this slot, and for how long. Distinct from [held]
         *  (a coordinator opened a room) and from [pct] (how many students came). */
        val lecturerPresent: Boolean = false,
        val contactHours: Double = 0.0,
    )

    data class UnitOption(val unitId: String, val unitName: String)

    data class Calendar(val from: String, val to: String, val units: List<UnitOption>, val events: List<Event>)

    suspend fun fetch(from: String, to: String, unitId: String? = null): Calendar? = withContext(Dispatchers.IO) {
        val token = AppState.token ?: return@withContext null
        runCatching {
            val q = buildString {
                append("$base/api/v1/lecturer/calendar?from=$from&to=$to")
                if (!unitId.isNullOrBlank()) append("&unit_id=$unitId")
            }
            val r = http.get(q) { header("Authorization", "Bearer $token") }
            if (r.status.value !in 200..299) return@runCatching null
            val o = JSONObject(r.bodyAsText())

            val units = o.optJSONArray("units")?.let { arr ->
                (0 until arr.length()).map {
                    val u = arr.getJSONObject(it)
                    UnitOption(u.optString("unit_id"), u.optString("unit_name"))
                }
            } ?: emptyList()

            val events = o.optJSONArray("events")?.let { arr ->
                (0 until arr.length()).map { i ->
                    val e = arr.getJSONObject(i)
                    val cohorts = e.optJSONArray("cohorts")?.let { ca ->
                        (0 until ca.length()).map { j ->
                            val c = ca.getJSONObject(j)
                            Cohort(
                                c.optString("offering_id"), c.optString("label"), c.optString("session_type"),
                                c.optString("coordinator_name"), c.optInt("enrolled"),
                                c.optInt("present"), c.optDouble("pct", 0.0),
                            )
                        }
                    } ?: emptyList()
                    Event(
                        e.optString("date"), e.optString("unit_id"), e.optString("unit_name"),
                        e.optString("start_time"), e.optInt("duration_minutes"), e.optString("room"),
                        e.optBoolean("held"), e.optString("status"),
                        e.optInt("enrolled"), e.optInt("present"), e.optDouble("pct", 0.0), cohorts,
                        e.optBoolean("lecturer_present"), e.optDouble("contact_hours", 0.0),
                    )
                }
            } ?: emptyList()

            Calendar(o.optString("from"), o.optString("to"), units, events)
        }.getOrNull()
    }
}
