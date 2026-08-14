package ug.qaat.coordinator.student

import io.ktor.client.request.*
import io.ktor.client.request.forms.*
import io.ktor.client.statement.*
import io.ktor.http.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject

/** Talks to the coordinator's in-room server over the hotspot LAN — fully offline. */
class CheckinClient(private val baseUrl: String, context: android.content.Context? = null) {
    private val http = StudentNet.lanClient(context)

    data class Session(
        val active: Boolean, val lecturerStarted: Boolean, val unitName: String,
        val cohort: String, val sessionId: String = "",
        /** The hub asking us to let the Wi-Fi slot go, and why. See SlotWarden. */
        val evict: Boolean = false, val evictReason: String? = null,
    )
    data class CheckinResult(
        val present: Boolean, val alreadyPresent: Boolean, val reason: String?,
        val evict: Boolean = false, val evictReason: String? = null,
    )

    /** Ask the hub what is running. [reg] is passed so a student who has already attended can be
     *  turned away on sight, rather than holding a slot to be told so by a check-in that fails. */
    suspend fun session(reg: String = ""): Session? = withContext(Dispatchers.IO) {
        runCatching {
            val url = if (reg.isBlank()) "$baseUrl/session"
                      else "$baseUrl/session?reg=" + java.net.URLEncoder.encode(reg, "UTF-8")
            val r = http.get(url)
            if (r.status.value !in 200..299) return@runCatching null
            val j = JSONObject(r.bodyAsText())
            Session(
                active = j.optString("active") == "true",
                lecturerStarted = j.optString("lecturer_started") == "true",
                unitName = j.optString("unit_name", ""),
                cohort = j.optString("cohort", ""),
                sessionId = j.optString("session_id", ""),
                evict = j.optString("evict") == "true",
                evictReason = j.optString("evict_reason", "").ifBlank { null },
            )
        }.getOrNull()
    }

    /**
     * Tell the hub we are letting go, so the slot frees in the coordinator's count immediately
     * rather than being inferred from our silence half a minute later.
     *
     * Best-effort and deliberately silent on failure: this is a courtesy sent on a connection we
     * are about to drop, and the actual release — [SlotLease.release] — does not depend on it
     * arriving. Failing to say goodbye must never stop us leaving.
     */
    suspend fun leave() {
        withContext(Dispatchers.IO) { runCatching { http.submitForm("$baseUrl/leave", parameters { }) } }
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
        CheckinResult(
            present = status == "PRESENT", alreadyPresent = reason == "DUPLICATE_SCAN", reason = reason,
            evict = j.optString("evict") == "true",
            evictReason = j.optString("evict_reason", "").ifBlank { null },
        )
    }
}
