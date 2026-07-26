package ug.qaat.coordinator.service

import android.app.*
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.IBinder
import androidx.core.app.NotificationCompat
import androidx.room.Room
import io.ktor.server.engine.*
import kotlinx.coroutines.*
import ug.qaat.coordinator.db.AppDatabase
import ug.qaat.coordinator.hotspot.HotspotManager
import ug.qaat.coordinator.server.InRoomServer
import ug.qaat.coordinator.store.RoomStore
import ug.qaat.engine.CheckinValidator
import ug.qaat.engine.SessionManager
import java.util.concurrent.atomic.AtomicReference

/**
 * The foreground service that keeps the whole in-room hub alive for the session:
 * the Ktor server, SQLite, the session manager, and the hotspot. Spec §13.
 *
 * Holds shared state the UI observes (room code, roster count, hotspot SSID). Wiring
 * an active session (manifest → ActiveSession + room-code secret → server.setActiveSession)
 * is done from the session-open flow.
 */
class SessionService : Service() {
    companion object {
        const val CHANNEL = "qaat_session"
        val hotspot = AtomicReference<HotspotManager.Info?>(null)
        val session = AtomicReference<SessionManager?>(null)
        // Nullable (NOT lateinit): if onCreate fails before wiring the server, or a session is
        // opened before the service has finished starting, callers see null and degrade instead
        // of crashing with UninitializedPropertyAccessException. Readiness is observable via
        // AppState.serverReady, which gates the "Start taking attendance" button.
        @Volatile var server: InRoomServer? = null; private set
    }

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private var ktor: ApplicationEngine? = null
    private lateinit var hotspotMgr: HotspotManager

    override fun onCreate() {
        super.onCreate()
        createChannel()
        // Must call startForeground within 5s of startForegroundService, OR the system kills
        // the process with ForegroundServiceDidNotStartInTimeException (the classic ~6s crash).
        // Pass the FGS type explicitly on API 29+ (matches the manifest). If it still fails
        // (e.g. a background auto-restart on Android 14, which is disallowed for this type),
        // stop cleanly instead of lingering as a zombie the system will kill.
        val foregrounded = runCatching {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q)
                startForeground(1, notification("Starting…"), android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_CONNECTED_DEVICE)
            else
                startForeground(1, notification("Starting…"))
        }.isSuccess
        if (!foregrounded) { stopSelf(); return }

        // The whole in-room startup is guarded — a missing asset, a busy port, or a hotspot
        // permission issue updates the notice instead of closing the app.
        try {
            ug.qaat.coordinator.di.Graph.init(this)
            val store = RoomStore(ug.qaat.coordinator.di.Graph.db.dao())
            val validator = CheckinValidator(store)

            val attend = runCatching { assets.open("attend.html").bufferedReader().use { it.readText() } }.getOrDefault("")
            val gate = runCatching { assets.open("gate.html").bufferedReader().use { it.readText() } }.getOrDefault("")
            val srv = InRoomServer(validator, attend, gate)
            server = srv
            ktor = runCatching { srv.start(8080) }.getOrNull()
            // The server object is now usable for setLive/clear; let the UI enable "open session".
            ug.qaat.coordinator.ui.AppState.serverReady = true

            if (ug.qaat.coordinator.ui.AppState.useSystemHotspot) {
                // System-hotspot mode: the coordinator runs their OWN phone hotspot, named after
                // the cohort, so students in a shared multi-coordinator room pick the right network
                // by name. We do NOT start a hotspot — we only poll for our IP on it so the
                // check-in + lecturer-gate URLs point at the right gateway.
                scope.launch {
                    repeat(120) {
                        val ip = HotspotManager.detectApIp()
                        if (ip != null) {
                            ug.qaat.coordinator.ui.AppState.inRoomIp = ip
                            ug.qaat.coordinator.ui.AppState.hotspotUp = true
                            update("Serving on your hotspot ($ip). Students: join your cohort's Wi-Fi.")
                            return@launch
                        }
                        ug.qaat.coordinator.ui.AppState.hotspotUp = false
                        update("Turn ON your phone's hotspot (name it after your cohort). Students join it.")
                        delay(3000)
                    }
                }
            } else {
                hotspotMgr = HotspotManager(this)
                hotspotMgr.start(
                    onReady = { info ->
                        hotspot.set(info)
                        ug.qaat.coordinator.ui.AppState.hotspotSsid = info.ssid
                        ug.qaat.coordinator.ui.AppState.hotspotPass = info.passphrase
                        ug.qaat.coordinator.ui.AppState.hotspotUp = true
                        update("Hotspot: ${info.ssid} — tell students to disconnect after ✓")
                    },
                    onError = { update("Wi-Fi hotspot unavailable ($it). Turn on your phone hotspot instead; check-in still works.") },
                )
            }
        } catch (e: Throwable) {
            update("Session server error: ${e.message}")
        }
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        // A null intent means the OS auto-restarted us after killing the process. We must NOT
        // resurrect an in-room session from the background — Android 14 forbids starting this
        // FGS type off a background context, which would crash. The coordinator re-opens the
        // session from the UI (a foreground start), so return NOT_STICKY and don't linger.
        if (intent == null) { stopSelf(); return START_NOT_STICKY }
        return START_NOT_STICKY
    }
    override fun onBind(intent: Intent?): IBinder? = null

    override fun onDestroy() {
        scope.cancel()
        ug.qaat.coordinator.ui.AppState.serverReady = false
        runCatching { ktor?.stop(100, 200) }
        runCatching { server?.clear() }             // null if startup never got this far
        server = null
        runCatching { if (this::hotspotMgr.isInitialized) hotspotMgr.stop() }
        super.onDestroy()
    }

    private fun update(text: String) =
        (getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager).notify(1, notification(text))

    private fun notification(text: String): Notification =
        NotificationCompat.Builder(this, CHANNEL)
            .setContentTitle("QAAT session running")
            .setContentText(text)
            .setSmallIcon(android.R.drawable.ic_dialog_info)
            .setOngoing(true)
            .build()

    private fun createChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            (getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager)
                .createNotificationChannel(NotificationChannel(CHANNEL, "Session", NotificationManager.IMPORTANCE_LOW))
        }
    }
}
