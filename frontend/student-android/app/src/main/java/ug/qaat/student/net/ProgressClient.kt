package ug.qaat.student.net

import io.ktor.client.request.*
import io.ktor.client.statement.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject

/**
 * Online attendance-progress lookup (the one online operation besides onboarding). Reuses the
 * existing public portal endpoint: GET /api/v1/student/progress?reg=&org= →
 * {full_name, institution, units:[{unit_id, unit_name, sessions_held, sessions_attended,
 * attendance_percentage, threshold, status, deficit_sessions}]}.
 */
class ProgressClient {
    private val http = Net.client()
    private val base = Net.baseUrl

    data class UnitRow(
        val unitId: String, val unitName: String, val held: Int, val attended: Int,
        val pct: Double, val threshold: Int, val status: String, val deficit: Int?,
    )
    data class Progress(val fullName: String, val institution: String, val units: List<UnitRow>)

    suspend fun fetch(reg: String, org: String): Progress = withContext(Dispatchers.IO) {
        val r = http.get("$base/api/v1/student/progress") {
            url { parameters.append("reg", reg); parameters.append("org", org) }
        }
        require(r.status.value in 200..299) {
            runCatching { JSONObject(r.bodyAsText()).optString("message", "") }.getOrNull()
                ?.takeIf { it.isNotBlank() } ?: "Couldn't load progress (${r.status.value})."
        }
        val j = JSONObject(r.bodyAsText())
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
        Progress(j.optString("full_name", ""), j.optString("institution", ""), units)
    }
}
