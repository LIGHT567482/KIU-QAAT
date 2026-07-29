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
