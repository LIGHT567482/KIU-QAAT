package ug.qaat.coordinator.student

import io.ktor.client.request.*
import io.ktor.client.statement.*
import io.ktor.http.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject
import ug.qaat.coordinator.net.Net

/** Binds this phone to the student's reg (one-device-one-student, global) after login, and returns
 *  the 12h attendance cooldown (set when this was a device switch). Online; best-effort. */
class RegisterDeviceClient {
    private val http = Net.client()
    private val base = Net.baseUrl

    data class Result(val attendBlockUntilMs: Long)

    suspend fun register(reg: String, fingerprint: String): Result = withContext(Dispatchers.IO) {
        val resp = http.post("$base/api/v1/student/register-device") {
            contentType(ContentType.Application.Json)
            setBody(JSONObject().put("reg_number", reg).put("device_fingerprint", fingerprint).toString())
        }
        require(resp.status.value in 200..299) {
            runCatching { JSONObject(resp.bodyAsText()).optString("message", "") }.getOrNull()
                ?.takeIf { it.isNotBlank() } ?: "Device registration failed (${resp.status.value})."
        }
        val j = JSONObject(resp.bodyAsText())
        Result(parseInstantMs(j.optString("attend_block_until", "")))
    }

    private fun parseInstantMs(s: String): Long =
        if (s.isBlank()) 0L else runCatching { java.time.Instant.parse(s).toEpochMilli() }.getOrDefault(0L)
}
