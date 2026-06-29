package ug.qaat.coordinator.server

import io.ktor.http.*
import io.ktor.server.application.*
import io.ktor.server.cio.*
import io.ktor.server.engine.*
import io.ktor.server.request.*
import io.ktor.server.response.*
import io.ktor.server.routing.*
import kotlinx.coroutines.flow.MutableSharedFlow
import ug.qaat.engine.*
import java.util.concurrent.atomic.AtomicReference

/**
 * The embedded in-room HTTP server (Ktor/CIO) bound to the hotspot interface.
 * Other phones reach it at http://<hotspot-ip>:8080. All security decisions are made
 * by the off-device-VERIFIED engine ([CheckinValidator], [RoomCode], [LecturerGate]);
 * this is only the HTTP wiring + the served pages.
 */
class InRoomServer(
    private val validator: CheckinValidator,
    private val attendPageHtml: String,
    private val lecturerPageHtml: String,
    private val lecturerGate: LecturerGate = LecturerGate(),
) {
    /** Live session state, set when the coordinator opens a session; cleared on close. */
    class Live(
        val session: ActiveSession,
        val roomCodeSecret: ByteArray,
        val gateContext: () -> LecturerGateContext,
        val onGate: (GateAction) -> Unit,
        // Called after each student submission with the QR's display fields + the result,
        // so the app can record the live-roster row (name/reg-no) and session history.
        val onCheckin: suspend (QrFields, ValidationResult) -> Unit = { _, _ -> },
    )
    private val live = AtomicReference<Live?>(null)

    fun setLive(l: Live) = live.set(l)
    fun clear() = live.set(null)

    // In-session announcements (spec Feature 1): the coordinator broadcasts; each
    // student's confirmation page holds an SSE connection on /events to receive them.
    // Separate from the check-in connection, so it doesn't affect the "kick".
    private val announcements = MutableSharedFlow<String>(extraBufferCapacity = 64)
    suspend fun broadcast(type: String, message: String) =
        announcements.emit("{\"type\":\"${type}\",\"message\":\"${message.replace("\"", "\\\"")}\"}")

    fun start(port: Int = 8080) = embeddedServer(CIO, port = port) {
        routing {
            get("/attend") { call.respondText(attendPageHtml, ContentType.Text.Html) }
            get("/gate") { call.respondText(lecturerPageHtml, ContentType.Text.Html) }

            // Student check-in: form qr, room_code, fingerprint.
            post("/submit") {
                val cur = live.get() ?: return@post call.json(mapOf("status" to "REJECTED", "reason" to "SESSION_NOT_ACTIVE"))
                val p = call.receiveParameters()
                if (!RoomCode.validate(cur.roomCodeSecret, p["room_code"] ?: "", System.currentTimeMillis() / 1000))
                    return@post call.json(mapOf("status" to "REJECTED", "reason" to "PROXIMITY_FAILED"))
                val qr = p["qr"] ?: ""
                val r = validator.validate(qr, cur.session, DeviceContext(p["fingerprint"] ?: ""))
                // Record the live-roster display row (name/reg-no come from the scanned QR).
                FlatJson.parseQr(qr)?.let { cur.onCheckin(it.fields, r) }
                call.json(buildMap { put("status", r.status.name); r.reason?.let { put("reason", it.name) } })
            }

            // Lecturer gate: form staff_id, room_code, fingerprint, biometric_verified.
            post("/gate") {
                val cur = live.get() ?: return@post call.json(mapOf("status" to "REJECTED", "reason" to "SESSION_NOT_ACTIVE"))
                val p = call.receiveParameters()
                val res = lecturerGate.evaluate(
                    staffId = p["staff_id"] ?: "",
                    roomCode = p["room_code"] ?: "",
                    biometricVerified = p["biometric_verified"] == "true",
                    ctx = cur.gateContext(),
                )
                if (res.ok && res.action != null) cur.onGate(res.action)
                call.json(buildMap {
                    put("status", if (res.ok) (if (res.action == GateAction.START) "STARTED" else "ENDED") else "REJECTED")
                    res.rejection?.let { put("reason", it.name) }
                })
            }

            // Coordinator broadcasts an announcement (form: type, message).
            post("/announce") {
                val p = call.receiveParameters()
                broadcast(p["type"] ?: "GENERAL", p["message"] ?: "")
                call.respondText("{\"ok\":true}", ContentType.Application.Json)
            }

            // Student confirmation page subscribes here (EventSource) for announcements.
            get("/events") {
                call.response.headers.append(HttpHeaders.CacheControl, "no-cache")
                call.respondTextWriter(contentType = ContentType.parse("text/event-stream")) {
                    announcements.collect { write("data: $it\n\n"); flush() }
                }
            }

            get("/status") {
                val cur = live.get()
                val code = cur?.let { RoomCode.derive(it.roomCodeSecret, System.currentTimeMillis() / 1000) } ?: ""
                call.json(mapOf("active" to (cur != null).toString(), "room_code" to code))
            }
        }
    }.also { it.start(wait = false) }

    // "Kick" = close the socket (Connection: close). The Wi-Fi slot frees on student disconnect.
    private suspend fun ApplicationCall.json(map: Map<String, String>) {
        response.headers.append(HttpHeaders.Connection, "close")
        respondText(map.entries.joinToString(",", "{", "}") { (k, v) -> "\"$k\":\"$v\"" }, ContentType.Application.Json)
    }
}
