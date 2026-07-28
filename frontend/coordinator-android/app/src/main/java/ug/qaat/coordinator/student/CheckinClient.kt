package ug.qaat.coordinator.student

import io.ktor.client.request.*
import io.ktor.client.request.forms.*
import io.ktor.client.statement.*
import io.ktor.http.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject

/** Talks to the coordinator's in-room server over the hotspot LAN — fully offline. */
class CheckinClient(private val baseUrl: String) {
    private val http = StudentNet.lanClient()

    data class Session(val active: Boolean, val lecturerStarted: Boolean, val unitName: String, val cohort: String)
    data class CheckinResult(val present: Boolean, val alreadyPresent: Boolean, val reason: String?)

    suspend fun session(): Session? = withContext(Dispatchers.IO) {
        runCatching {
            val r = http.get("$baseUrl/session")
            if (r.status.value !in 200..299) return@runCatching null
            val j = JSONObject(r.bodyAsText())
            Session(
                active = j.optString("active") == "true",
                lecturerStarted = j.optString("lecturer_started") == "true",
                unitName = j.optString("unit_name", ""),
                cohort = j.optString("cohort", ""),
            )
        }.getOrNull()
    }

    // Check-in by REGISTRATION NUMBER (the student_id from the login token). Identity is the reg;
    // presence is being on the coordinator's hotspot LAN; per-lecture device lock via the fingerprint.
    suspend fun attend(regNumber: String, fingerprint: String): CheckinResult = withContext(Dispatchers.IO) {
        val r = http.submitForm("$baseUrl/checkin", parameters {
            append("reg_number", regNumber); append("fingerprint", fingerprint)
        })
        val j = JSONObject(r.bodyAsText())
        val status = j.optString("status", "REJECTED")
        val reason = j.optString("reason", "").ifBlank { null }
        CheckinResult(present = status == "PRESENT", alreadyPresent = reason == "DUPLICATE_SCAN", reason = reason)
    }
}
