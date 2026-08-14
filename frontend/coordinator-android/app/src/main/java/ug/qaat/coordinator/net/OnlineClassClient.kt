package ug.qaat.coordinator.net

import io.ktor.client.request.*
import io.ktor.client.statement.*
import io.ktor.http.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject
import ug.qaat.coordinator.ui.AppState

/**
 * DISTANCE LEARNING, FROM THE LECTURER'S PHONE.
 *
 * On campus a coordinator opens the room, the lecturer scans at the door, and the students' phones
 * prove they are on that room's hotspot. A distance cohort has none of it — no room, no door, no
 * hotspot — so the lecturer starts their own class and their sign-in is the identity proof the QR
 * scan was providing.
 *
 * THE CODE ROTATES EVERY TEN SECONDS, and that is the point rather than an inconvenience. The
 * on-campus student code is fixed for the whole session because the network gate is doing the real
 * work; with no network gate a fixed code would be forwarded once and reused by the entire year
 * group. Rotating narrows that to about half a minute. It is honestly weaker than standing in a
 * hall — which is why the record says ONLINE everywhere it is read, and never counts as a physical
 * attendance.
 *
 * ONLINE ONLY, deliberately. A cached code is a wrong code within ten seconds, and a class the
 * server does not know has started is a class no student can check in to.
 */
class OnlineClassClient {
    private val http = Net.client()
    private val base = Net.baseUrl

    data class Live(
        val sessionId: String,
        val unitId: String,
        val unitName: String,
        val cohortLabel: String,
        val studentCode: String,
        val secondsRemaining: Int,
        val present: Int,
        val enrolled: Int,
    )

    /** GET — the class running now, or null. */
    suspend fun current(): Live? = withContext(Dispatchers.IO) {
        val token = AppState.token ?: return@withContext null
        runCatching {
            val r = http.get("$base/api/v1/lecturer/online-class") {
                header("Authorization", "Bearer $token")
            }
            if (r.status.value == 204 || r.status.value !in 200..299) return@runCatching null
            parse(JSONObject(r.bodyAsText()))
        }.getOrNull()
    }

    /** POST — start one. Returns the class, or a message written for the lecturer. */
    suspend fun start(unitId: String, offeringId: String, fingerprint: String): Result<Live> =
        withContext(Dispatchers.IO) {
            val token = AppState.token ?: return@withContext Result.failure(Exception("Sign in again."))
            runCatching {
                val r = http.post("$base/api/v1/lecturer/online-class") {
                    header("Authorization", "Bearer $token")
                    contentType(ContentType.Application.Json)
                    setBody(
                        JSONObject()
                            .put("unit_id", unitId)
                            .put("offering_id", offeringId)
                            .put("fingerprint", fingerprint)
                            .toString()
                    )
                }
                val j = JSONObject(r.bodyAsText())
                if (r.status.value in 200..299) parse(j)
                else throw Exception(j.optString("message").ifBlank { "Could not start the class." })
            }.recoverCatching {
                throw Exception(it.message ?: "You need internet to run an online class.")
            }
        }

    /** POST /end — books the contact hours. Returns null on success, else why not. */
    suspend fun end(): String? = withContext(Dispatchers.IO) {
        val token = AppState.token ?: return@withContext "Sign in again."
        runCatching {
            val r = http.post("$base/api/v1/lecturer/online-class/end") {
                header("Authorization", "Bearer $token")
                contentType(ContentType.Application.Json)
                setBody("{}")
            }
            if (r.status.value in 200..299) null
            else JSONObject(r.bodyAsText()).optString("message").ifBlank { "Could not end the class." }
        }.getOrElse { "You need internet to end an online class." }
    }

    private fun parse(j: JSONObject) = Live(
        sessionId = j.optString("session_id"),
        unitId = j.optString("unit_id"),
        unitName = j.optString("unit_name").ifBlank { j.optString("unit_id") },
        cohortLabel = j.optString("cohort_label"),
        studentCode = j.optString("student_code"),
        secondsRemaining = j.optInt("seconds_remaining"),
        present = j.optInt("present"),
        enrolled = j.optInt("enrolled"),
    )
}
