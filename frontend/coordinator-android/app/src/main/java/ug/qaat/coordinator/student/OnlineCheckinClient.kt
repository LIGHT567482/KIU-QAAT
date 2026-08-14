package ug.qaat.coordinator.student

import io.ktor.client.request.*
import io.ktor.client.statement.*
import io.ktor.http.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import ug.qaat.coordinator.net.Net
import ug.qaat.coordinator.ui.AppState

/**
 * CHECKING IN TO A CLASS WITH NO ROOM.
 *
 * The ordinary student flow is entirely local: find the coordinator's hotspot, hit their in-room
 * server, be marked present because you are demonstrably on that network. A distance / e-learning
 * student has no hotspot to find, so that flow ends with "connect to your coordinator's Wi-Fi" —
 * advice that is simply false for them, and the reason these cohorts had no attendance at all.
 *
 * This is the cloud path instead, and it is deliberately NOT a relaxation of the local one. It
 * only ever sees sessions the server has marked ONLINE, which only a lecturer of an e-learning
 * cohort can open. A campus student pointing this at their own lecture gets nothing back, because
 * their session is not online — the network gate still decides their attendance, as it must.
 *
 * The code CHANGES EVERY TEN SECONDS. On campus the student code is fixed, because being on the
 * hotspot is the real proof and the code only names the room. Here there is no hotspot, so a fixed
 * code would be forwarded once and reused by the whole year. Rotating is the strongest thing left,
 * and it is honestly weaker than standing in a hall — which is why the record says ONLINE.
 */
class OnlineCheckinClient {
    private val http = Net.client()
    private val base = Net.baseUrl

    data class LiveOnline(
        val sessionId: String,
        val unitName: String,
        val lecturerHint: String,
        val alreadyCheckedIn: Boolean,
    )

    /**
     * The student's live ONLINE session, if one is running. Returns null for anything else —
     * including a live IN_PERSON session, which belongs to the hotspot flow and must not be
     * offered a typed-code shortcut around it.
     */
    suspend fun liveOnlineSession(): LiveOnline? = withContext(Dispatchers.IO) {
        val token = AppState.token ?: return@withContext null
        runCatching {
            val r = http.get("$base/api/v1/student/live-sessions") {
                header("Authorization", "Bearer $token")
            }
            if (r.status.value !in 200..299) return@runCatching null
            val arr = JSONArray(r.bodyAsText())
            for (i in 0 until arr.length()) {
                val o = arr.getJSONObject(i)
                if (o.optString("delivery_mode") != "ONLINE") continue
                return@runCatching LiveOnline(
                    sessionId = o.optString("session_id"),
                    unitName = o.optString("unit_name").ifBlank { o.optString("unit_id") },
                    lecturerHint = o.optString("coordinator_name"),
                    alreadyCheckedIn = o.optBoolean("already_checked_in"),
                )
            }
            null
        }.getOrNull()
    }

    /** POST /api/v1/student/checkin. Returns null on success, else a message for the student. */
    suspend fun attend(sessionId: String, code: String, fingerprint: String): String? =
        withContext(Dispatchers.IO) {
            val token = AppState.token ?: return@withContext "Sign in again to check in."
            runCatching {
                val r = http.post("$base/api/v1/student/checkin") {
                    header("Authorization", "Bearer $token")
                    contentType(ContentType.Application.Json)
                    setBody(
                        JSONObject()
                            .put("session_id", sessionId)
                            .put("room_code", code.trim())
                            .put("device_fingerprint", fingerprint)
                            .toString()
                    )
                }
                val j = runCatching { JSONObject(r.bodyAsText()) }.getOrNull()
                if (j?.optString("status") == "PRESENT") return@runCatching null
                // Each refusal is explained in the student's own terms. "CODE_NOT_CURRENT" told
                // plainly is actionable — read the screen again; a bare rejection is not, and a
                // student who cannot tell a stale code from a broken app stops trying.
                when (j?.optString("reason")) {
                    "CODE_NOT_CURRENT" ->
                        "That code has already changed. Read the one on screen now and type it quickly."
                    "NOT_IN_THIS_COHORT" ->
                        "You are not enrolled in this class, so you cannot be marked present for it."
                    "LECTURER_NOT_STARTED" ->
                        "Your lecturer has not started the class yet. Wait for the code to appear."
                    "GATE_NOT_OPEN" ->
                        "Check-in for this class has closed."
                    "DEVICE_ALREADY_USED" ->
                        "This phone has already checked someone else in to this class."
                    "SESSION_NOT_ACTIVE" ->
                        "That class is no longer running."
                    "NOT_ON_ROSTER" ->
                        "Your account is not linked to a student record. Tell your coordinator."
                    else -> "Not marked present — try again."
                }
            }.getOrElse { "Couldn't reach the server. You need internet for an online class." }
        }
}
