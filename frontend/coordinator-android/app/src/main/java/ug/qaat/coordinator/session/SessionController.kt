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
    // The lecturer's physical-presence proof: when they scanned the gate to START/END + the device
    // fingerprint. Uploaded in the session package so the central dashboards show their attendance.
    private var lecturerStartAt: String = ""
    private var lecturerStartFp: String = ""
    private var lecturerEndAt: String = ""
    private var lecturerEndFp: String = ""
    private var sessionCode: String = ""   // the ONE daily code (lecturer+session) for this unit, or ""
    private var enrolled: Int = 0
    private var current: SessionEntity? = null
    // Wall-clock deadline after which a forgotten session auto-closes (scheduled duration + 5m grace).
    private var autoCloseAtMillis: Long = Long.MAX_VALUE
    private val sm get() = SessionService.session.get()

    /** Open a session for [unitId] with the present [lecturerStaffId]. Requires login + manifest. */
    fun open(unitId: String, lecturerStaffId: String) = scope.launch {
        // A coordinator runs ONE session at a time — refuse to open a second while one is live
        // (the current must be ended, manually or by the auto-close, first).
        if (current != null || AppState.currentSessionId != null) {
            AppState.sessionNotice = "You already have a live session. End it before starting another."
            return@launch
        }
        val token = AppState.token ?: return@launch
        val tenantId = AppState.tenantId ?: return@launch
        val m = AppState.manifest ?: return@launch
        // The foreground service must have built the in-room server first. If it hasn't (service
        // still starting, or onCreate failed), bail instead of crashing on an unset server — the
        // UI keeps the button disabled via AppState.serverReady, so this is the last-ditch guard.
        val server = SessionService.server ?: return@launch
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
        lecturerStartAt = ""; lecturerStartFp = ""; lecturerEndAt = ""; lecturerEndFp = ""
        this@SessionController.lecturerStaffId = lecturerStaffId
        sessionCode = m.units.firstOrNull { it.unitId == unitId }?.sessionCode ?: ""
        AppState.currentLecturerHasCode = sessionCode.isNotBlank()
        AppState.lecturerStartedHere = false
        AppState.currentSessionCode = null
        enrolled = roster.size

        val entity = SessionEntity(session.sessionId, unitId, LocalDate.now().toString(), "OPEN", enrolled)
        current = entity
        Graph.db.dao().upsertSession(entity)

        // The state machine the lecturer gate drives (IDLE → PENDING_LECTURER → ACTIVE).
        SessionService.session.set(SessionManager(session.sessionId).apply { open() })

        val dao = Graph.db.dao()
        // Unit name + cohort for the student app's GET /session (so it shows what it's checking into).
        val unit = m.units.firstOrNull { it.unitId == unitId }
        val unitName = unit?.unitName?.takeIf { it.isNotBlank() } ?: unitId
        val cohort = AppState.cohortLabel?.takeIf { it.isNotBlank() } ?: ""
        // Auto-close deadline: scheduled duration (fallback 120m) + 5m grace after the hour it runs.
        val durationMin = (unit?.durationMinutes?.takeIf { it > 0 } ?: 120)
        autoCloseAtMillis = System.currentTimeMillis() + (durationMin + 5) * 60_000L
        server.setLive(
            InRoomServer.Live(
                session = session,
                roomCodeSecret = secret,
                unitId = unitId,
                unitName = unitName,
                cohort = cohort,
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
                onGate = { action, fingerprint ->
                    val at = Instant.now().toString()
                    when (action) {
                        GateAction.START -> { gateState = GateState.STARTED; lecturerStartAt = at; lecturerStartFp = fingerprint; AppState.lecturerStartedHere = true; AppState.currentSessionCode = sessionCode.takeIf { it.isNotBlank() }; runCatching { sm?.lecturerStarted() } }
                        GateAction.END -> { gateState = GateState.ENDED; lecturerEndAt = at; lecturerEndFp = fingerprint }
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

    /** Multi-coordinator case: mark the assigned lecturer present when the lecturer is physically on
     *  ANOTHER coordinator's hotspot and can't START here in person. Takes the ONE daily code (which
     *  covers both the lecturer and the session), delivered offline in this unit's manifest. Returns
     *  true if it matches — students may then check in exactly as if the lecturer had STARTed. */
    fun startLecturerByCode(entered: String): Boolean {
        if (sessionCode.isBlank() || entered.trim() != sessionCode) return false
        if (gateState != GateState.STARTED) {
            gateState = GateState.STARTED
            lecturerStartAt = Instant.now().toString()
            lecturerStartFp = "daily-code:$sessionCode"   // presence proof carried into the sealed package
            AppState.lecturerStartedHere = true
            runCatching { sm?.lecturerStarted() }
        }
        return true
    }

    /** Close the session: tear the room down IMMEDIATELY (stop the server + hotspot), then seal +
     *  upload in the background. Teardown must NOT wait on the (possibly slow) network upload. */
    fun close(auto: Boolean = false) = scope.launch {
        ticker?.cancel()
        autoCloseAtMillis = Long.MAX_VALUE
        val s = current ?: return@launch
        val userId = AppState.userId ?: return@launch
        val bindingKey = AppState.deviceBindingKey
        val closedReason = if (auto) "AUTO_CLOSED" else "MANUAL"
        // Snapshot the lecturer's presence proof before teardown. START is what makes the session's
        // student attendance "verified" server-side; END adds their contact hours.
        val lecturerScan = lecturerStartAt.takeIf { it.isNotBlank() }?.let {
            SessionPackage.LecturerScan(
                lecturerId = lecturerStaffId, scannedAt = it, fingerprintHash = lecturerStartFp,
                endedAt = lecturerEndAt, endFingerprintHash = lecturerEndFp,
            )
        }

        // 1) TEAR DOWN NOW — before any network. Stopping the service runs onDestroy, which stops the
        //    Ktor server and (in app-hotspot mode) closes the LocalOnlyHotspot immediately.
        runCatching { SessionService.server?.clear() }
        current = null
        AppState.currentSessionId = null
        AppState.currentUnitId = null
        AppState.currentLecturerHasCode = false
        AppState.lecturerStartedHere = false
        AppState.currentSessionCode = null
        AppState.hotspotSsid = null
        AppState.hotspotPass = null
        AppState.hotspotUp = false
        AppState.serverReady = false
        runCatching { Graph.appContext.stopService(Intent(Graph.appContext, SessionService::class.java)) }
        // A phone's OWN system hotspot cannot be switched off by an app (Android restriction), so
        // send the coordinator straight to the hotspot toggle to turn it off there and then.
        if (AppState.useSystemHotspot) runCatching {
            Graph.appContext.startActivity(
                Intent(android.provider.Settings.ACTION_WIRELESS_SETTINGS).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            )
        }

        // 2) Seal + upload in the background. The room is already closed regardless of the result.
        Graph.db.dao().upsertSession(s.copy(status = "CLOSED", closedReason = closedReason))
        val records = Graph.db.dao().rosterForSession(s.sessionId).map {
            AttendanceRecord(it.logId, it.sessionId, it.studentIdHash, it.deviceFingerprintHash,
                it.sequenceNumber, it.checkinTimestamp, it.entryMethod)
        }
        if (bindingKey != null) {
            val pkg = SessionPackage.build(
                s.sessionId, userId, records, sealedAt = Instant.now().toString(),
                unitId = s.unitId, sessionDate = s.sessionDate, lecturer = lecturerScan,
                // Central session_status: AUTO_CLOSED vs CLOSED — surfaces in the cloud dashboards.
                sessionStatus = if (auto) "AUTO_CLOSED" else "CLOSED",
            )
            val ok = runCatching { SyncClient(Net_base(), AppState.token ?: "").upload(userId, s.sessionId, bindingKey, pkg) }
                .getOrDefault(false)
            Graph.db.dao().upsertSession(s.copy(status = if (ok) "SYNCED" else "PENDING_SYNC", closedReason = closedReason))
        }
    }

    private fun startTicker() {
        ticker?.cancel()
        ticker = scope.launch {
            // No student room code any more — students prove presence by being on the hotspot.
            // Only the lecturer code rotates (read off-screen for the START/END gate).
            while (isActive) {
                // Auto-close a forgotten session once past its deadline (duration + 5m grace), so the
                // coordinator isn't blocked from starting the next one and the log shows it was auto-closed.
                if (System.currentTimeMillis() >= autoCloseAtMillis && current != null) {
                    AppState.sessionNotice = "Session auto-closed — the scheduled time elapsed and it wasn't ended."
                    close(auto = true)
                    return@launch
                }
                val now = System.currentTimeMillis() / 1000
                AppState.lecturerCode = RoomCode.derive(secret, now) // rotating — lecturer
                AppState.secondsLeft = RoomCode.secondsRemaining(now)
                delay(1000)
            }
        }
    }

    private fun Net_base() = ug.qaat.coordinator.net.Net.baseUrl
}
