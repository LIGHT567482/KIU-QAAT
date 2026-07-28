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
    // Called whenever a client actually fetches the check-in page — i.e. a phone on the hotspot
    // reached this server. Drives the coordinator's live "N devices reached this server" self-test,
    // which is the objective proof that client→host works on the current hardware.
    private val onClientReached: () -> Unit = {},
) {
    /** Live session state, set when the coordinator opens a session; cleared on close. */
    class Live(
        val session: ActiveSession,
        val roomCodeSecret: ByteArray,
        // The unit + cohort this session is for — served to connected students (GET /session) so
        // the student app can show "Attendance for: <unit> · <cohort>" before the one-tap check-in.
        val unitId: String = "",
        val unitName: String = "",
        val cohort: String = "",
        val gateContext: () -> LecturerGateContext,
        // (action, lecturerFingerprintHash) — the fingerprint lets us record the lecturer's
        // START/END presence proof into the uploaded package (lecturer_attendance_logs).
        val onGate: (GateAction, String) -> Unit,
        // Whether the lecturer has scanned to START — students may only check in after.
        val lecturerStarted: () -> Boolean = { true },
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
            // The student check-in page. Fetching it means a phone on the hotspot reached us, so it
            // ticks the reachability self-test. Served at /attend (the projected "Check in here" QR
            // points here) and /checkin (legacy card path; attend.html also handles ?qr= / ?t=).
            get("/attend") { onClientReached(); call.respondText(attendPageHtml, ContentType.Text.Html) }
            get("/checkin") { onClientReached(); call.respondText(attendPageHtml, ContentType.Text.Html) }
            get("/gate") { call.respondText(lecturerPageHtml, ContentType.Text.Html) }

            // Student check-in by REG-NUMBER (no QR): identity is the typed reg matched to the roster;
            // presence is being on the hotspot LAN. onCheckin fires only on PRESENT (so revisits/typos
            // don't spam the feed). The browser stores the reg in localStorage and auto-resubmits on
            // return — the server's DUPLICATE_SCAN guard makes that a harmless "already present".
            post("/checkin") {
                val cur = live.get() ?: return@post call.json(mapOf("status" to "REJECTED", "reason" to "SESSION_NOT_ACTIVE"))
                if (!cur.lecturerStarted())
                    return@post call.json(mapOf("status" to "REJECTED", "reason" to "LECTURER_NOT_STARTED"))
                val p = call.receiveParameters()
                val reg = p["reg_number"] ?: ""
                val r = validator.validateReg(reg, cur.session, DeviceContext(p["fingerprint"] ?: ""))
                if (r.status == ValidationStatus.PRESENT)
                    cur.onCheckin(QrFields(reg, "", "", "", "", "", "", ""), r)   // reg as the display id; no name offline
                call.json(buildMap { put("status", r.status.name); r.reason?.let { put("reason", it.name) } })
            }

            // Student check-in: form qr, fingerprint.
            post("/submit") {
                val cur = live.get() ?: return@post call.json(mapOf("status" to "REJECTED", "reason" to "SESSION_NOT_ACTIVE"))
                // Lecturer-started gate: no student attendance until the lecturer has scanned to START.
                if (!cur.lecturerStarted())
                    return@post call.json(mapOf("status" to "REJECTED", "reason" to "LECTURER_NOT_STARTED"))
                val p = call.receiveParameters()
                // No student room code: proximity IS being on the hotspot LAN. This server is only
                // reachable over the coordinator's hotspot, so a successful POST already proves the
                // student is physically in the room. One-device-one-person is enforced downstream by
                // the device fingerprint (DEVICE_ALREADY_USED / DUPLICATE_SCAN in the validator).
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
                val gateAction = res.action  // local capture: res.action is a cross-module property, not smart-castable
                if (res.ok && gateAction != null) cur.onGate(gateAction, p["fingerprint"] ?: "")
                call.json(buildMap {
                    put("status", if (res.ok) (if (gateAction == GateAction.START) "STARTED" else "ENDED") else "REJECTED")
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
                val lecturerCode = cur?.let { RoomCode.derive(it.roomCodeSecret, System.currentTimeMillis() / 1000) } ?: ""
                call.json(mapOf(
                    "active" to (cur != null).toString(),
                    "room_code" to lecturerCode,       // rotating — lecturer gate only
                ))
            }

            // Active session metadata for a connected student's app (offline, over the hotspot).
            // The app fetches this on connect to show WHICH unit it's checking into before the one
            // tap. Cohort-scoped by construction: this server only holds this coordinator's session.
            get("/session") {
                val cur = live.get()
                call.json(mapOf(
                    "active" to (cur != null).toString(),
                    "lecturer_started" to (cur?.lecturerStarted?.invoke() == true).toString(),
                    "unit_id" to (cur?.unitId ?: ""),
                    "unit_name" to (cur?.unitName ?: ""),
                    "cohort" to (cur?.cohort ?: ""),
                ))
            }
        }
    }.also { it.start(wait = false) }

    // "Kick" = close the socket (Connection: close). The Wi-Fi slot frees on student disconnect.
    private suspend fun ApplicationCall.json(map: Map<String, String>) {
        response.headers.append(HttpHeaders.Connection, "close")
        respondText(map.entries.joinToString(",", "{", "}") { (k, v) -> "\"$k\":\"$v\"" }, ContentType.Application.Json)
    }
}
