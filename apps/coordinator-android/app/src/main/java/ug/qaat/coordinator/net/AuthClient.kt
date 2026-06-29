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

    data class Result(
        val token: String, val jti: String?, val role: String,
        val userId: String, val tenantId: String, val deviceBindingKey: String?,
    )

    suspend fun tenantLookup(email: String): String {
        val r = http.get("$base/api/v1/auth/tenant-lookup") { url { parameters.append("email", email) } }
        require(r.status.isSuccess()) { "No account found for that email." }
        return JSONObject(r.bodyAsText()).getString("tenant_id")
    }

    /** @return Result on success; null with [mfaRequired]=true if the server wants a TOTP code. */
    suspend fun login(email: String, password: String, totp: String?, onMfaRequired: () -> Unit): Result? {
        val tenantId = tenantLookup(email)
        val body = JSONObject()
            .put("email", email).put("password", password)
            .put("totp_code", totp ?: "").put("tenant_id", tenantId)
        val r = http.post("$base/api/v1/auth/login") {
            contentType(ContentType.Application.Json); setBody(body.toString())
        }
        val j = JSONObject(r.bodyAsText())
        if (r.status.value == 403 && j.optString("error") == "MFA_REQUIRED") { onMfaRequired(); return null }
        require(r.status.isSuccess()) { j.optString("message", "Login failed") }
        return Result(
            token = j.getString("access_token"),
            jti = j.optString("jti", null),
            role = j.optString("role", "COORDINATOR"),
            userId = j.getString("user_id"),
            tenantId = tenantId,
            deviceBindingKey = j.optString("device_binding_key", null),
        )
    }
}
