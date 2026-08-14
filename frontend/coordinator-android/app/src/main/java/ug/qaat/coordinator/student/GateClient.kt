package ug.qaat.coordinator.student

import io.ktor.client.request.*
import io.ktor.client.request.forms.*
import io.ktor.client.statement.*
import io.ktor.http.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject

/** Lecturer-side LAN calls to the coordinator's in-room server: read the live rotating code
 *  (GET /status) and START/END the session (POST /gate). Fully offline over the hotspot. */
class GateClient(private val baseUrl: String, context: android.content.Context? = null) {
    private val http = StudentNet.lanClient(context)

    data class Status(val active: Boolean, val roomCode: String)
    /** [combinedClassCode] is the three digits to read out to the other coordinators sharing this
     *  lecture — present only on a successful START of a combined class, "" otherwise. */
    data class GateResult(val status: String, val reason: String?, val combinedClassCode: String = "")

    suspend fun status(): Status? = withContext(Dispatchers.IO) {
        runCatching {
            val r = http.get("$baseUrl/status")
            if (r.status.value !in 200..299) return@runCatching null
            val j = JSONObject(r.bodyAsText())
            Status(j.optString("active") == "true", j.optString("room_code", ""))
        }.getOrNull()
    }

    suspend fun gate(staffId: String, roomCode: String, fingerprint: String): GateResult = withContext(Dispatchers.IO) {
        val r = http.submitForm("$baseUrl/gate", parameters {
            append("staff_id", staffId); append("room_code", roomCode)
            append("fingerprint", fingerprint); append("biometric_verified", "false")
        })
        val j = JSONObject(r.bodyAsText())
        GateResult(
            j.optString("status", "REJECTED"),
            j.optString("reason", "").ifBlank { null },
            j.optString("combined_class_code", ""),
        )
    }
}
