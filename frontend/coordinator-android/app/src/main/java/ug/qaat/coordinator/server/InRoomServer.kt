package ug.qaat.coordinator.server

import io.ktor.http.*
import io.ktor.server.application.*
import io.ktor.server.cio.*
import io.ktor.server.engine.*
import io.ktor.server.plugins.*
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
        // The three digits the lecturer reads out to the OTHER coordinators teaching this same
        // lecture, when it is a combined class; "" when it is not. Returned to the lecturer's phone
        // on a successful START, because that phone cannot work it out for itself — it holds no
        // manifest and no key. Which is the point: the code only exists once somebody has physically
        // gated in on a hotspot in the room. See CombinedClassCode.
        val combinedClassCode: () -> String = { "" },
        // Called after each student submission with the QR's display fields + the result,
        // so the app can record the live-roster row (name/reg-no) and session history.
        val onCheckin: suspend (QrFields, ValidationResult) -> Unit = { _, _ -> },
    )
    private val live = AtomicReference<Live?>(null)

    /**
     * Who is holding a Wi-Fi slot, and who should give it up. See [SlotWarden] for why this exists
     * and what it can and cannot do — the short version is that the hotspot admits about ten
     * clients, a hall holds two hundred, and nothing but this makes a slot turn over.
     *
     * Reset on every session change so last lecture's finished students are not still evicted from
     * this one.
     */
    val warden = SlotWarden()

    fun setLive(l: Live) { warden.reset(); live.set(l) }
    fun clear() { warden.reset(); live.set(null) }

    // In-session announcements (spec Feature 1): the coordinator broadcasts; each
    // student's confirmation page holds an SSE connection on /events to receive them.
    // Separate from the check-in connection, so it doesn't affect the "kick".
    private val announcements = MutableSharedFlow<String>(extraBufferCapacity = 64)
    suspend fun broadcast(type: String, message: String) =
        announcements.emit("{\"type\":\"${type}\",\"message\":\"${message.replace("\"", "\\\"")}\"}")

    fun start(port: Int = 8080) = embeddedServer(
        CIO,
        port = port,
        // A phone that dies mid-request — walks out of range, screen off, app killed — otherwise
        // holds its socket for CIO's 45-second default, well past the fifteen-second lease it was
        // granted. Ten seconds is comfortably longer than any request this hub serves and shorter
        // than a turn, so a dead client's socket is gone before its slot would have expired anyway.
        configure = { connectionIdleTimeoutSeconds = 10 },
    ) {
        routing {
            // The student check-in page. Fetching it means a phone on the hotspot reached us, so it
            // ticks the reachability self-test. Served at /attend (the projected "Check in here" QR
            // points here) and /checkin (legacy card path; attend.html also handles ?qr= / ?t=).
            get("/attend") { onClientReached(); warden.touch(call.clientKey()); call.respondText(attendPageHtml, ContentType.Text.Html) }
            get("/checkin") { onClientReached(); warden.touch(call.clientKey()); call.respondText(attendPageHtml, ContentType.Text.Html) }
            get("/gate") { call.respondText(lecturerPageHtml, ContentType.Text.Html) }

            // Student check-in by REG-NUMBER (no QR): identity is the typed reg matched to the roster;
            // presence is being on the hotspot LAN. onCheckin fires only on PRESENT (so revisits/typos
            // don't spam the feed). The browser stores the reg in localStorage and auto-resubmits on
            // return — the server's DUPLICATE_SCAN guard makes that a harmless "already present".
            post("/checkin") {
                val key = call.clientKey()
                val cur = live.get() ?: return@post call.json(mapOf("status" to "REJECTED", "reason" to "SESSION_NOT_ACTIVE"), warden.touch(key))
                val p = call.receiveParameters()
                val reg = p["reg_number"] ?: ""
                if (!cur.lecturerStarted())
                    return@post call.json(mapOf("status" to "REJECTED", "reason" to "LECTURER_NOT_STARTED"), warden.touch(key, reg))
                val r = validator.validateReg(reg, cur.session, DeviceContext(p["fingerprint"] ?: ""))
                if (r.status == ValidationStatus.PRESENT)
                    cur.onCheckin(QrFields(reg, "", "", "", "", "", "", ""), r)   // reg as the display id; no name offline
                // A DUPLICATE_SCAN settles the slot just as a PRESENT does: whether this tap was the
                // one that recorded them or a second tap on top of it, the student is marked present
                // and has no further business holding a slot.
                if (r.status == ValidationStatus.PRESENT || r.reason == RejectionReason.DUPLICATE_SCAN)
                    warden.settle(key, reg)
                call.json(buildMap { put("status", r.status.name); r.reason?.let { put("reason", it.name) } }, warden.touch(key, reg))
            }

            // Student check-in: form qr, fingerprint.
            post("/submit") {
                val key = call.clientKey()
                val cur = live.get() ?: return@post call.json(mapOf("status" to "REJECTED", "reason" to "SESSION_NOT_ACTIVE"), warden.touch(key))
                // Lecturer-started gate: no student attendance until the lecturer has scanned to START.
                if (!cur.lecturerStarted())
                    return@post call.json(mapOf("status" to "REJECTED", "reason" to "LECTURER_NOT_STARTED"), warden.touch(key))
                val p = call.receiveParameters()
                // No student room code: proximity IS being on the hotspot LAN. This server is only
                // reachable over the coordinator's hotspot, so a successful POST already proves the
                // student is physically in the room. One-device-one-person is enforced downstream by
                // the device fingerprint (DEVICE_ALREADY_USED / DUPLICATE_SCAN in the validator).
                val qr = p["qr"] ?: ""
                val r = validator.validate(qr, cur.session, DeviceContext(p["fingerprint"] ?: ""))
                // Record the live-roster display row (name/reg-no come from the scanned QR).
                val fields = FlatJson.parseQr(qr)?.also { cur.onCheckin(it.fields, r) }?.fields
                // The QR's student id is this path's registration number — it is what makes an
                // eviction stick when the phone comes back on a different address.
                val reg = fields?.studentId ?: ""
                if (r.status == ValidationStatus.PRESENT || r.reason == RejectionReason.DUPLICATE_SCAN)
                    warden.settle(key, reg)
                call.json(buildMap { put("status", r.status.name); r.reason?.let { put("reason", it.name) } }, warden.touch(key, reg))
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
                    // Only on a successful START, and only when this lecture is actually shared.
                    // Sending it on END would put a dead number on the lecturer's screen after the
                    // register closed; sending it on a REJECTED gate would hand the code to someone
                    // who just failed to prove they are the lecturer.
                    if (res.ok && gateAction == GateAction.START) {
                        cur.combinedClassCode().takeIf { it.isNotBlank() }
                            ?.let { put("combined_class_code", it) }
                    }
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
            // The client reporting that it has let go of the Wi-Fi. Frees the slot in the
            // coordinator's occupancy readout at once, instead of waiting for the sweep to infer it
            // from silence — and it is the signal that separates an app which obeyed an eviction
            // from one which ignored it, which is what [SlotWarden.jammed] turns on.
            post("/leave") {
                warden.release(call.clientKey())
                call.respondText("{\"ok\":true}", ContentType.Application.Json)
            }

            get("/session") {
                val cur = live.get()
                // The registration number, when the app knows it. Supplying it here is what lets a
                // student who has already attended be turned away on sight — before they occupy a
                // slot long enough to attempt a check-in that would only be refused.
                val reg = call.request.queryParameters["reg"] ?: ""
                val verdict = warden.touch(call.clientKey(), reg)
                call.json(mapOf(
                    "active" to (cur != null).toString(),
                    "lecturer_started" to (cur?.lecturerStarted?.invoke() == true).toString(),
                    // The session id lets a student device remember it already attended THIS session
                    // (one-per-session) and reset for the next one — it changes each session.
                    "session_id" to (cur?.session?.sessionId ?: ""),
                    "unit_id" to (cur?.unitId ?: ""),
                    "unit_name" to (cur?.unitName ?: ""),
                    "cohort" to (cur?.cohort ?: ""),
                ), verdict)
            }
        }
    }.also { it.start(wait = false) }

    /**
     * Which client this request came from, as the warden keys its leases.
     *
     * The hotspot's DHCP address, which is as good an identity as the network layer offers and is
     * good enough for a lease: it is stable for as long as a phone stays associated, which is the
     * entire lifetime of the thing being tracked. It is NOT relied on for anything that has to
     * survive a reconnect — that is what the registration number is for.
     */
    private fun ApplicationCall.clientKey(): String =
        runCatching { request.origin.remoteHost }.getOrDefault("").ifBlank { "?" }

    /**
     * Every JSON answer this hub gives, with the slot verdict stamped on it.
     *
     * `Connection: close` ends the HTTP conversation, but on its own it frees nothing — the phone
     * stays associated to the Wi-Fi and the slot stays gone. The `evict` flag is the part that
     * actually matters: it tells the student app to let the network go, which it can do because
     * both ends of this conversation are our code. A browser reading the same field can only
     * display it; see [SlotWarden] on why that asymmetry is unavoidable.
     */
    private suspend fun ApplicationCall.json(map: Map<String, String>, verdict: SlotWarden.Verdict? = null) {
        response.headers.append(HttpHeaders.Connection, "close")
        val body = buildMap {
            putAll(map)
            if (verdict != null && verdict != SlotWarden.Verdict.KEEP) {
                put("evict", "true")
                put("evict_reason", verdict.name)
            }
        }
        respondText(body.entries.joinToString(",", "{", "}") { (k, v) -> "\"$k\":\"$v\"" }, ContentType.Application.Json)
    }
}
