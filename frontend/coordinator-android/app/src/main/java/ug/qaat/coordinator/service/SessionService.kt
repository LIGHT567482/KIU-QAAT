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
        lateinit var server: InRoomServer; private set
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
            server = InRoomServer(validator, attend, gate)
            ktor = runCatching { server.start(8080) }.getOrNull()

            hotspotMgr = HotspotManager(this)
            hotspotMgr.start(
                onReady = { info ->
                    hotspot.set(info)
                    ug.qaat.coordinator.ui.AppState.hotspotSsid = info.ssid
                    ug.qaat.coordinator.ui.AppState.hotspotPass = info.passphrase
                    update("Hotspot: ${info.ssid} — tell students to disconnect after ✓")
                },
                onError = { update("Wi-Fi hotspot unavailable ($it). Share a Wi-Fi/hotspot manually; check-in still works.") },
            )
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
        runCatching { ktor?.stop(100, 200) }
        runCatching { server.clear() }              // no-op if startup never got this far
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
