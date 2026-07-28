package ug.qaat.coordinator.net

import io.ktor.client.request.*
import io.ktor.client.statement.*
import io.ktor.http.*
import org.json.JSONObject

/**
 * Coordinator login — mirrors the PWA exactly:
 *   GET  /api/v1/auth/tenant-lookup?email=  → { tenant_id }
 *   POST /api/v1/auth/login {email,password,totp_code,tenant_id}
 *        → { access_token, jti, role, user_id, expires_in, device_binding_key }
 *
 * device_binding_key is the secret the Sealer needs (HKDF → AES/HMAC). Keep it only
 * in memory / Android Keystore — never log it.
 */
class AuthClient {
    private val http = Net.client()
    private val base = Net.baseUrl

    private companion object {
        const val NON_JSON = "The server returned a web page instead of data — it's likely waking up (it sleeps when idle). Wait ~30s and tap Sign in again."
        fun String.looksJson() = trimStart().let { it.startsWith("{") || it.startsWith("[") }
        fun httpErr(code: Int): String = when (code) {
            in 500..599 -> "The server is waking up or busy (HTTP $code). Wait ~30s and try again."
            401, 403 -> "Incorrect email or password."
            404 -> "No account found for that email."
            else -> "Sign in failed (HTTP $code)."
        }
    }

    data class Result(
        val token: String, val jti: String?, val role: String,
        val userId: String, val tenantId: String, val deviceBindingKey: String?,
        val fullName: String, val title: String, val registrationNo: String,
        // Unified app-login extras: the student's reg-number / the lecturer's staff-id, and the
        // institution slug the user signed in with (kept for later reg-scoped online calls).
        val studentId: String = "", val staffId: String = "", val org: String = "",
    )

    /**
     * Unified KIU QAAT sign-in for EVERY role. identifier = email (staff/coordinator) OR reg-number
     * (student) OR staff-id (lecturer); org = institution id/domain. Hits the gateway's app-login,
     * which resolves the account, reuses the real /auth/login, and augments with student_id/staff_id.
     * @return Result on success; null with [onMfaRequired] if the server wants a TOTP code.
     */
    suspend fun appLogin(identifier: String, password: String, org: String, totp: String?, onMfaRequired: () -> Unit): Result? {
        val body = JSONObject()
            .put("identifier", identifier).put("password", password)
            .put("org", org).put("totp_code", totp ?: "")
        var text = ""; var status = 0
        for (attempt in 1..6) {
            val r = http.post("$base/api/v1/auth/app-login") {
                contentType(ContentType.Application.Json); setBody(body.toString())
            }
            text = r.bodyAsText(); status = r.status.value
            if (text.looksJson() || status < 500) break
            if (attempt < 6) kotlinx.coroutines.delay(6000)
        }
        if (!text.looksJson())
            throw IllegalStateException(if (status in 200..299) NON_JSON else httpErr(status))
        val j = JSONObject(text)
        if (status == 403 && j.optString("error") == "MFA_REQUIRED") { onMfaRequired(); return null }
        require(status in 200..299) { j.optString("message").ifBlank { httpErr(status) } }
        return Result(
            token = j.getString("access_token"),
            jti = j.optString("jti").takeIf { it.isNotEmpty() },
            role = j.optString("role", ""),
            userId = j.getString("user_id"),
            tenantId = j.optString("tenant_id", ""),
            deviceBindingKey = j.optString("device_binding_key").takeIf { it.isNotEmpty() },
            fullName = j.optString("full_name", ""),
            title = j.optString("title", ""),
            registrationNo = j.optString("registration_number", ""),
            studentId = j.optString("student_id", ""),
            staffId = j.optString("staff_id", ""),
            org = j.optString("org", org),
        )
    }

    /** Change the signed-in coordinator's password. @return error message, or null on success. */
    suspend fun changePassword(token: String, current: String, newPassword: String): String? {
        val r = http.post("$base/api/v1/auth/change-password") {
            header("Authorization", "Bearer $token"); contentType(ContentType.Application.Json)
            setBody(JSONObject().put("current_password", current).put("new_password", newPassword).toString())
        }
        if (r.status.value in 200..299) return null
        return runCatching { JSONObject(r.bodyAsText()).optString("message", "Could not change password") }
            .getOrDefault("Could not change password")
    }

    suspend fun tenantLookup(email: String): String {
        val r = http.get("$base/api/v1/auth/tenant-lookup") { url { parameters.append("email", email) } }
        val text = r.bodyAsText()
        if (!r.status.isSuccess())
            throw IllegalStateException(if (text.looksJson()) JSONObject(text).optString("message").ifBlank { httpErr(r.status.value) } else httpErr(r.status.value))
        if (!text.looksJson()) throw IllegalStateException(NON_JSON)
        return JSONObject(text).getString("tenant_id")
    }

    /** @return Result on success; null with [mfaRequired]=true if the server wants a TOTP code. */
    suspend fun login(email: String, password: String, totp: String?, onMfaRequired: () -> Unit): Result? {
        val tenantId = tenantLookup(email)
        val body = JSONObject()
            .put("email", email).put("password", password)
            .put("totp_code", totp ?: "").put("tenant_id", tenantId)
        // Cold-start resilience: a sleeping free-tier auth-service answers with a 502/HTML page while
        // it wakes (~15–60s). Auto-retry through the wake so a FRESH login (e.g. on a phone with no
        // saved session) just completes after a short wait, instead of showing "waking up" over and
        // over. A JSON body — even a 401 — means the service actually answered, so we stop retrying.
        var text = ""; var status = 0
        for (attempt in 1..6) {
            val r = http.post("$base/api/v1/auth/login") {
                contentType(ContentType.Application.Json); setBody(body.toString())
            }
            text = r.bodyAsText(); status = r.status.value
            if (text.looksJson() || status < 500) break   // service answered
            if (attempt < 6) kotlinx.coroutines.delay(6000) // still waking — wait, then retry
        }
        if (!text.looksJson())
            throw IllegalStateException(if (status in 200..299) NON_JSON else httpErr(status))
        val j = JSONObject(text)
        if (status == 403 && j.optString("error") == "MFA_REQUIRED") { onMfaRequired(); return null }
        require(status in 200..299) { j.optString("message").ifBlank { httpErr(status) } }
        return Result(
            token = j.getString("access_token"),
            jti = j.optString("jti").takeIf { it.isNotEmpty() },
            role = j.optString("role", "COORDINATOR"),
            userId = j.getString("user_id"),
            tenantId = tenantId,
            deviceBindingKey = j.optString("device_binding_key").takeIf { it.isNotEmpty() },
            fullName = j.optString("full_name", ""),
            title = j.optString("title", ""),
            registrationNo = j.optString("registration_number", ""),
        )
    }
}
