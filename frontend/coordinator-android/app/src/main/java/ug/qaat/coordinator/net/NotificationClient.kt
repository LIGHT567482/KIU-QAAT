package ug.qaat.coordinator.net

import io.ktor.client.request.*
import io.ktor.client.statement.*
import io.ktor.http.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import ug.qaat.coordinator.ui.AppState

/** Cross-role in-app notifications over the cloud API (online). Students/coordinators/lecturers
 *  read their inbox; lecturers/coordinators also send. Auth is the caller's bearer token. */
class NotificationClient {
    private val http = Net.client()
    private val base = Net.baseUrl

    data class Notif(
        val id: String, val senderName: String, val senderRole: String,
        val subject: String, val body: String, val createdAt: String, val read: Boolean,
    )

    suspend fun inbox(): List<Notif> = withContext(Dispatchers.IO) {
        val token = AppState.token ?: return@withContext emptyList()
        runCatching {
            val r = http.get("$base/api/v1/app-notifications") { header("Authorization", "Bearer $token") }
            if (r.status.value !in 200..299) return@runCatching emptyList<Notif>()
            val arr = JSONArray(r.bodyAsText())
            (0 until arr.length()).map { i ->
                val o = arr.getJSONObject(i)
                Notif(
                    o.getString("notification_id"), o.optString("sender_name"), o.optString("sender_role"),
                    o.optString("subject"), o.optString("body"), o.optString("created_at"), o.optBoolean("read"),
                )
            }
        }.getOrDefault(emptyList())
    }

    suspend fun unread(): Int = withContext(Dispatchers.IO) {
        val token = AppState.token ?: return@withContext 0
        runCatching {
            val r = http.get("$base/api/v1/app-notifications/unread-count") { header("Authorization", "Bearer $token") }
            if (r.status.value !in 200..299) 0 else JSONObject(r.bodyAsText()).optInt("unread", 0)
        }.getOrDefault(0)
    }

    /**
     * Dismiss from MY inbox (the ✕ on an alert). Other recipients keep their copy.
     *
     * Returns null on success, otherwise a human-readable reason. This USED to return a Boolean
     * that every caller threw away: when the server refused (a role that could read its inbox but
     * wasn't allowed to clear it), the card disappeared for a moment and then came back on the next
     * refresh with nothing on screen to explain why. A dismissal that did not stick has to say so.
     *
     * 404 counts as success — it means the alert is already gone from this inbox, which is exactly
     * what the ✕ was asking for.
     */
    suspend fun dismiss(id: String): String? = withContext(Dispatchers.IO) {
        val token = AppState.token ?: return@withContext "Not signed in"
        runCatching {
            val r = http.delete("$base/api/v1/app-notifications/$id") { header("Authorization", "Bearer $token") }
            when {
                r.status.value in 200..299 || r.status.value == 404 -> null
                r.status.value == 401 || r.status.value == 403 -> "You're not allowed to clear this alert."
                else -> "Couldn't clear it (${r.status.value}). Try again."
            }
        }.getOrElse { "Couldn't reach the server — the alert will come back until you're online." }
    }

    suspend fun markRead(id: String) {
        withContext(Dispatchers.IO) {
            val token = AppState.token ?: return@withContext
            runCatching { http.post("$base/api/v1/app-notifications/$id/read") { header("Authorization", "Bearer $token") } }
        }
    }

    /** Send (LECTURER/COORDINATOR). Returns null on success, else a human error. audience is
     *  STUDENTS / COORDINATOR (lecturer) or STUDENTS / LECTURERS / STUDENT / LECTURER (coordinator);
     *  for a specific STUDENT/LECTURER pass targetId (student_id or lecturer_id). */
    suspend fun send(audience: String, unitId: String?, subject: String, body: String, targetId: String? = null): String? = withContext(Dispatchers.IO) {
        val token = AppState.token ?: return@withContext "Not signed in"
        runCatching {
            val payload = JSONObject().put("audience", audience).put("subject", subject).put("body", body)
            if (!unitId.isNullOrBlank()) payload.put("unit_id", unitId)
            if (!targetId.isNullOrBlank()) payload.put("target_id", targetId)
            val r = http.post("$base/api/v1/app-notifications") {
                header("Authorization", "Bearer $token"); contentType(ContentType.Application.Json)
                setBody(payload.toString())
            }
            if (r.status.value in 200..299) null
            else runCatching { JSONObject(r.bodyAsText()).optString("message") }.getOrNull()?.takeIf { it.isNotBlank() }
                ?: "Failed to send (${r.status.value})"
        }.getOrElse { "Network error — check your connection." }
    }
}

/**
 * WHO A LECTURER CAN WRITE TO, BY NAME.
 *
 * "The coordinator" is not an address. One unit is routinely taught to a Day, an Evening, a Weekend
 * and an e-learning cohort, each with its own coordinator, and the message a lecturer has in mind
 * — "my Saturday class has no projector" — is for exactly one of them. Sent to all four it leaves
 * three wondering which room is meant and the one who should act assuming somebody else will.
 *
 * Each person therefore arrives WITH the cohort they belong to, because that is what tells two of
 * them apart: a lecturer knows their coordinators as "the Saturday one", not by name.
 */
class LecturerRecipientsClient {
    private val http = Net.client()
    private val base = Net.baseUrl

    data class Coordinator(
        val userId: String, val name: String, val code: String,
        val cohort: String, val course: String, val units: String,
    ) {
        val label: String get() = listOf(name.ifBlank { code }, cohort).filter { it.isNotBlank() }.joinToString(" · ")
    }

    data class Student(
        val studentId: String, val name: String, val cohort: String,
        val unitId: String, val unitName: String,
    ) {
        val label: String get() = listOf(name, studentId).filter { it.isNotBlank() }.joinToString(" · ")
    }

    data class Recipients(
        val coordinators: List<Coordinator> = emptyList(),
        val students: List<Student> = emptyList(),
    )

    suspend fun load(): Recipients = withContext(Dispatchers.IO) {
        val token = AppState.token ?: return@withContext Recipients()
        runCatching {
            val r = http.get("$base/api/v1/lecturer/recipients") {
                header("Authorization", "Bearer $token")
            }
            if (r.status.value !in 200..299) return@runCatching Recipients()
            val o = JSONObject(r.bodyAsText())
            val cs = o.optJSONArray("coordinators") ?: JSONArray()
            val ss = o.optJSONArray("students") ?: JSONArray()
            Recipients(
                coordinators = (0 until cs.length()).map {
                    val x = cs.getJSONObject(it)
                    Coordinator(
                        userId = x.optString("user_id"), name = x.optString("full_name"),
                        code = x.optString("coordinator_code"), cohort = x.optString("cohort"),
                        course = x.optString("course_name"), units = x.optString("units"),
                    )
                }.filter { it.userId.isNotBlank() },
                students = (0 until ss.length()).map {
                    val x = ss.getJSONObject(it)
                    Student(
                        studentId = x.optString("student_id"), name = x.optString("full_name"),
                        cohort = x.optString("cohort"), unitId = x.optString("unit_id"),
                        unitName = x.optString("unit_name"),
                    )
                }.filter { it.studentId.isNotBlank() },
            )
        }.getOrElse { Recipients() }
    }
}
