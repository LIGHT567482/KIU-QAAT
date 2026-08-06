package ug.qaat.coordinator.net

import io.ktor.client.request.*
import io.ktor.client.statement.*
import io.ktor.http.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import ug.qaat.coordinator.db.AppDao
import ug.qaat.coordinator.db.PresenceClaimEntity
import ug.qaat.coordinator.db.TimetableSlotEntity

/**
 * The two cloud calls behind the lecturer's "I was here" button.
 *
 * Neither is on the critical path of filing a claim, and that is the design rather than an
 * accident. The button works with the phone in flight mode: the timetable it matches against was
 * cached the last time there was signal, and the claim it writes goes into Room and is uploaded
 * whenever the network comes back. A lecturer disputing a patrol tick is, almost by definition,
 * someone who had no signal in that room at that time.
 */
class PresenceClient {
    private val http = Net.client()
    private val base = Net.baseUrl

    /**
     * Pull the lecturer's WHOLE WEEK and replace the cached copy.
     *
     * The whole week, not today: a phone caches what it is given, so a lecturer whose last signal
     * was Monday would spend Tuesday matching against Monday's timetable — offline, confidently,
     * with nothing on screen to say so.
     *
     * Replace, not merge — a slot the timetable office DELETED has to disappear from the phone
     * too, and merging would keep it forever.
     *
     * @return the number of slots cached, or null if the call did not succeed (in which case the
     *         previous cache is left exactly as it was; a failed refresh must never empty it).
     */
    suspend fun refreshTimetable(token: String, dao: AppDao): Int? = withContext(Dispatchers.IO) {
        val r = http.get("$base/api/v1/lecturer/timetable") { header("Authorization", "Bearer $token") }
        if (!r.status.isSuccess()) return@withContext null
        val arr = runCatching { JSONObject(r.bodyAsText()).optJSONArray("slots") }.getOrNull()
            ?: return@withContext null
        val rows = (0 until arr.length()).map { i ->
            val o = arr.getJSONObject(i)
            TimetableSlotEntity(
                unitId = o.optString("unit_id", ""),
                unitName = o.optString("unit_name", ""),
                dayOfWeek = o.optInt("day_of_week", 0),
                startTime = o.optString("start_time", ""),
                durationMinutes = o.optInt("duration_minutes", 60),
                room = o.optString("room", ""),
            )
        }.filter { it.unitId.isNotBlank() && it.startTime.isNotBlank() }
        dao.replaceTimetable(rows)
        rows.size
    }

    /**
     * Upload queued claims. @return true once the batch is durably stored.
     *
     * The claim ids were minted on the phone and the server's primary key rejects duplicates, so
     * re-sending a batch after a timeout — the ordinary outcome on the connection these are filed
     * on — cannot file the same moment twice. That is what lets the caller retry freely.
     */
    suspend fun upload(token: String, claims: List<PresenceClaimEntity>): Boolean = withContext(Dispatchers.IO) {
        if (claims.isEmpty()) return@withContext true
        val arr = JSONArray()
        claims.forEach { c ->
            val o = JSONObject()
                .put("claim_id", c.id)
                .put("location_status", c.locationStatus)
                .put("captured_at", c.capturedAt)
                .put("session_date", c.sessionDate)
                .put("unit_id", c.unitId).put("unit_name", c.unitName).put("room", c.room)
                .put("day_of_week", c.dayOfWeek).put("scheduled_time", c.scheduledTime)
                .put("match_kind", c.matchKind).put("note", c.note)
            // JSONObject.put(String, Object) writes a JSON null for a Kotlin null, which is what
            // the server reads as "no fix" — but put(String, double) cannot express one, so the
            // nullable numbers go through the object overload deliberately.
            o.put("latitude", c.latitude as Any?)
            o.put("longitude", c.longitude as Any?)
            o.put("accuracy_metres", c.accuracyMetres as Any?)
            o.put("minutes_from_start", c.minutesFromStart as Any?)
            arr.put(o)
        }
        val r = http.post("$base/api/v1/lecturer/presence-claims") {
            header("Authorization", "Bearer $token")
            contentType(ContentType.Application.Json)
            setBody(JSONObject().put("claims", arr).toString())
        }
        r.status.isSuccess()
    }
}
