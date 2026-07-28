package ug.qaat.student.net

import io.ktor.client.request.*
import io.ktor.client.statement.*
import io.ktor.http.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject

/**
 * One-time online onboarding. The student scans their personal QR card; we (1) qr-login to prove
 * it's a real, active QR and mint a student token, then (2) fetch the canonical signed credential
 * from my-qr and hand it back to store. After this the app never needs the internet again.
 *
 * Mirrors backend handlers: POST /api/v1/student/qr-login {qr} -> {access_token},
 * GET /api/v1/student/my-qr (Bearer) -> {student_id, serial_number, token, ...}.
 */
class OnboardClient {
    private val http = Net.client()
    private val base = Net.baseUrl

    data class Result(val credential: String, val studentId: String)

    suspend fun onboard(scannedQr: String): Result = withContext(Dispatchers.IO) {
        val loginResp = http.post("$base/api/v1/student/qr-login") {
            contentType(ContentType.Application.Json)
            setBody(JSONObject().put("qr", scannedQr).toString())
        }
        require(loginResp.status.value in 200..299) {
            friendly(loginResp.bodyAsText(), "Sign-in failed (${loginResp.status.value}). Is this your QR card?")
        }
        val token = JSONObject(loginResp.bodyAsText()).optString("access_token", "")
        require(token.isNotBlank()) { "Sign-in did not return a token." }

        val meResp = http.get("$base/api/v1/student/my-qr") { header("Authorization", "Bearer $token") }
        require(meResp.status.value in 200..299) {
            friendly(meResp.bodyAsText(), "Could not load your credential (${meResp.status.value}).")
        }
        val me = JSONObject(meResp.bodyAsText())
        // Prefer the canonical fresh token; fall back to the scanned card if the generator is down.
        val credential = me.optString("token", "").ifBlank { scannedQr }
        Result(credential, me.optString("student_id", ""))
    }

    private fun friendly(body: String, fallback: String): String =
        runCatching { JSONObject(body).optString("message", "") }.getOrNull()?.takeIf { it.isNotBlank() } ?: fallback
}
