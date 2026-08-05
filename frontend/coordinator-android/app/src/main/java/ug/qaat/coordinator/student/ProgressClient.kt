package ug.qaat.coordinator.student

import io.ktor.client.request.*
import io.ktor.client.statement.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject
import ug.qaat.coordinator.net.Net

/** Online attendance progress: GET /api/v1/student/progress?reg=&org=. */
class ProgressClient {
    private val http = Net.client()
    private val base = Net.baseUrl

    data class UnitRow(val unitId: String, val unitName: String, val held: Int, val attended: Int,
                       val pct: Double, val threshold: Int, val status: String, val deficit: Int?)
    data class Progress(val fullName: String, val institution: String, val units: List<UnitRow>,
                        val school: String = "", val department: String = "")

    suspend fun fetch(reg: String): Progress = withContext(Dispatchers.IO) {
        // Backed off rather than failed on 429. This is the endpoint a whole cohort opens at
        // once — the hour eligibility is published — and they share one campus IP, so being
        // refused is the normal case for the students at the back rather than an error.
        val body = Net.retrying(
            call = { http.get("$base/api/v1/student/progress") { url { parameters.append("reg", reg) } } },
            parse = { r ->
                require(r.status.value in 200..299) {
                    runCatching { JSONObject(r.bodyAsText()).optString("message", "") }.getOrNull()
                        ?.takeIf { it.isNotBlank() } ?: "Couldn't load progress (${r.status.value})."
                }
                r.bodyAsText()
            },
        )
        val j = JSONObject(body)
        val units = ArrayList<UnitRow>()
        j.optJSONArray("units")?.let { arr ->
            for (i in 0 until arr.length()) {
                val u = arr.getJSONObject(i)
                units.add(UnitRow(
                    u.optString("unit_id", ""), u.optString("unit_name", ""),
                    u.optInt("sessions_held", 0), u.optInt("sessions_attended", 0),
                    u.optDouble("attendance_percentage", 0.0), u.optInt("threshold", 0),
                    u.optString("status", ""),
                    if (u.isNull("deficit_sessions")) null else u.optInt("deficit_sessions"),
                ))
            }
        }
        Progress(j.optString("full_name", ""), j.optString("institution", ""), units,
            j.optString("school", ""), j.optString("department", ""))
    }
}
