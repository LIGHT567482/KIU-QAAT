package ug.qaat.coordinator.session

import android.content.Intent
import kotlinx.coroutines.*
import ug.qaat.coordinator.db.PresentDisplayEntity
import ug.qaat.coordinator.db.SessionEntity
import ug.qaat.coordinator.di.Graph
import ug.qaat.coordinator.net.SyncClient
import ug.qaat.coordinator.server.InRoomServer
import ug.qaat.coordinator.service.SessionService
import ug.qaat.coordinator.ui.AppState
import ug.qaat.engine.*
import java.security.SecureRandom
import java.time.Instant
import java.time.LocalDate
import java.util.UUID

/**
 * Turns "app + hotspot running" into "a live session students can check into", and
 * closes it (seal + sync). All the security logic it drives — the validator, the room
 * code, the lecturer gate, the sealer — is the off-device-verified engine.
 *
 * Phone-hub model: the session_id + room-code secret are generated LOCALLY (truly
 * offline). NOTE: the central backend must be able to create the session row from the
 * synced package (FK attendance_logs→sessions) — see BUILD_AND_TEST.md "central session".
 */
object SessionController {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private var ticker: Job? = null

    private var secret: ByteArray = ByteArray(0)
    private var gateState: GateState = GateState.NOT_STARTED
    private var lecturerStaffId: String = ""
    private var enrolled: Int = 0
    private var current: SessionEntity? = null
    private val sm get() = SessionService.session.get()

    /** Open a session for [unitId] with the present [lecturerStaffId]. Requires login + manifest. */
    fun open(unitId: String, lecturerStaffId: String) = scope.launch {
        val token = AppState.token ?: return@launch
        val tenantId = AppState.tenantId ?: return@launch
        val m = AppState.manifest ?: return@launch
        val roster = m.rosterByUnit[unitId].orEmpty()

        val session = ActiveSession(
            sessionId = UUID.randomUUID().toString(),
            tenantId = tenantId,
            academicYear = m.academicYear,
            institutionPublicKeyPem = m.institutionPublicKeyPem,
            studentHashKey = m.studentHashKey,
            rosterHashes = roster.map { it.studentIdHash }.toSet(),
            rosterSerials = roster.associate { it.studentIdHash to it.qrSerialNumber },
        )
        secret = ByteArray(32).also { SecureRandom().nextBytes(it) }
        gateState = GateState.NOT_STARTED
        this@SessionController.lecturerStaffId = lecturerStaffId
        enrolled = roster.size

        val entity = SessionEntity(session.sessionId, unitId, LocalDate.now().toString(), "OPEN", enrolled)
        current = entity
        Graph.db.dao().upsertSession(entity)

        // The state machine the lecturer gate drives (IDLE → PENDING_LECTURER → ACTIVE).
        SessionService.session.set(SessionManager(session.sessionId).apply { open() })

        val dao = Graph.db.dao()
        SessionService.server.setLive(
            InRoomServer.Live(
                session = session,
                roomCodeSecret = secret,
                gateContext = {
                    LecturerGateContext(
                        assignedStaffId = lecturerStaffId,
                        roomCodeSecret = secret,
                        gateState = gateState,
                        attended = dao.attendanceCount(session.sessionId),
                        enrolled = enrolled,
                        ratio = 0.5,
                        requireBiometric = false,
                    )
                },
                onGate = { action ->
                    when (action) {
                        GateAction.START -> { gateState = GateState.STARTED; runCatching { sm?.lecturerStarted() } }
                        GateAction.END -> gateState = GateState.ENDED
                    }
                },
                // Students may only check in once the lecturer has passed the START gate.
                lecturerStarted = { gateState == GateState.STARTED },
                onCheckin = { fields, result ->
                    // Live-roster row: name/reg-no come from the scanned QR (the durable
                    // ledger keeps the hash). The PRESENT attendance row is written by the validator.
                    dao.putPresentDisplay(
                        PresentDisplayEntity(
                            sessionId = session.sessionId,
                            studentId = fields.studentId,
                            fullName = fields.fullName,
                            checkinTimestamp = Instant.now().toString(),
                            status = result.reason?.name ?: result.status.name,
                        )
                    )
                },
            )
        )

        AppState.currentSessionId = session.sessionId
        AppState.currentUnitId = unitId
        startTicker()
    }

    /** Close the session and (when online) seal + upload it to the central backend. */
    fun close() = scope.launch {
        ticker?.cancel()
        val s = current ?: return@launch
        val userId = AppState.userId ?: return@launch
        val bindingKey = AppState.deviceBindingKey
        Graph.db.dao().upsertSession(s.copy(status = "CLOSED"))

        val records = Graph.db.dao().rosterForSession(s.sessionId).map {
            AttendanceRecord(it.logId, it.sessionId, it.studentIdHash, it.deviceFingerprintHash,
                it.sequenceNumber, it.checkinTimestamp, it.entryMethod)
        }
        if (bindingKey != null) {
            val pkg = SessionPackage.build(
                s.sessionId, userId, records, sealedAt = Instant.now().toString(),
                unitId = s.unitId, sessionDate = s.sessionDate,
            )
            val ok = runCatching { SyncClient(Net_base(), AppState.token ?: "").upload(userId, s.sessionId, bindingKey, pkg) }
                .getOrDefault(false)
            Graph.db.dao().upsertSession(s.copy(status = if (ok) "SYNCED" else "PENDING_SYNC"))
        }
        SessionService.server.clear()
        current = null
        AppState.currentSessionId = null
        AppState.hotspotSsid = null
        AppState.hotspotPass = null
        // Tear the room down: stopping the foreground service runs its onDestroy, which stops
        // the Wi-Fi hotspot + the Ktor server. The phone is then free for the next session —
        // the coordinator taps "Start hotspot" again for the next round, and the loop repeats.
        runCatching { Graph.appContext.stopService(Intent(Graph.appContext, SessionService::class.java)) }
    }

    private fun startTicker() {
        ticker?.cancel()
        ticker = scope.launch {
            AppState.roomCode = RoomCode.staticCode(secret) // static student code (does not rotate)
            while (isActive) {
                val now = System.currentTimeMillis() / 1000
                AppState.lecturerCode = RoomCode.derive(secret, now) // rotating — lecturer
                AppState.secondsLeft = RoomCode.secondsRemaining(now)
                delay(1000)
            }
        }
    }

    private fun Net_base() = ug.qaat.coordinator.net.Net.baseUrl
}
