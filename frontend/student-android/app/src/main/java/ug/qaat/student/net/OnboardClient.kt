package ug.qaat.student.net

import io.ktor.client.request.*
import io.ktor.client.statement.*
import io.ktor.http.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject

/**
 * One-time online onboarding — NO QR. The student enters their registration number + institution;
 * we bind this device to that reg (POST /api/v1/student/register-device), enforcing one-device-one-
 * student globally. After this the app works fully offline: check-in is by reg-number over the
 * coordinator's hotspot LAN.
 *
 * Mirrors handlers.RegisterDevice: {reg_number, org, device_fingerprint} -> {student_id, full_name}.
 */
class OnboardClient {
    private val http = Net.client()
    private val base = Net.baseUrl

    data class Result(val reg: String, val fullName: String, val attendBlockUntilMs: Long)

    suspend fun register(reg: String, org: String, fingerprint: String): Result = withContext(Dispatchers.IO) {
        val resp = http.post("$base/api/v1/student/register-device") {
            contentType(ContentType.Application.Json)
            setBody(
                JSONObject()
                    .put("reg_number", reg)
                    .put("org", org)
                    .put("device_fingerprint", fingerprint)
                    .toString()
            )
        }
        require(resp.status.value in 200..299) {
            friendly(resp.bodyAsText(), "Registration failed (${resp.status.value}).")
        }
        val j = JSONObject(resp.bodyAsText())
        Result(j.optString("student_id", reg), j.optString("full_name", ""), parseInstantMs(j.optString("attend_block_until", "")))
    }

    // RFC3339 → epoch millis (0 when blank/unparseable).
    private fun parseInstantMs(s: String): Long =
        if (s.isBlank()) 0L else runCatching { java.time.Instant.parse(s).toEpochMilli() }.getOrDefault(0L)

    private fun friendly(body: String, fallback: String): String =
        runCatching { JSONObject(body).optString("message", "") }.getOrNull()?.takeIf { it.isNotBlank() } ?: fallback
}
