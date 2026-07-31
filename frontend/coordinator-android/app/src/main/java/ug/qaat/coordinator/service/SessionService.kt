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
import ug.qaat.coordinator.hotspot.NsdAdvertiser
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
        const val TAG = "QAAT_HUB"
        val hotspot = AtomicReference<HotspotManager.Info?>(null)
        val session = AtomicReference<SessionManager?>(null)
        // Nullable (NOT lateinit): if onCreate fails before wiring the server, or a session is
        // opened before the service has finished starting, callers see null and degrade instead
        // of crashing with UninitializedPropertyAccessException. Readiness is observable via
        // AppState.serverReady, which gates the "Start taking attendance" button.
        @Volatile var server: InRoomServer? = null; private set
        // Static so a fresh onCreate can stop an engine leaked by a previous instance still holding
        // :8080 (the "port busy" bind failure) before rebinding.
        @Volatile private var ktor: ApplicationEngine? = null
    }

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private lateinit var hotspotMgr: HotspotManager
    private var nsd: NsdAdvertiser? = null

    override fun onCreate() {
        super.onCreate()
        createChannel()
        // Must call startForeground within 5s of startForegroundService, OR the system kills
        // the process with ForegroundServiceDidNotStartInTimeException (the classic ~6s crash).
        // Pass the FGS type explicitly on API 29+ (matches the manifest). If it still fails
        // (e.g. a background auto-restart on Android 14, which is disallowed for this type),
        // stop cleanly instead of lingering as a zombie the system will kill.
        val fgResult = runCatching {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q)
                startForeground(1, notification("Starting…"), android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_CONNECTED_DEVICE)
            else
                startForeground(1, notification("Starting…"))
        }
        if (fgResult.isFailure) {
            val e = fgResult.exceptionOrNull()
            android.util.Log.e(TAG, "startForeground failed", e)
            ug.qaat.coordinator.ui.AppState.serverError =
                "Android blocked the foreground service: ${e?.javaClass?.simpleName}: ${e?.message}"
            stopSelf(); return
        }

        // Opening the DB + binding the port can take a few seconds, so run the whole hub startup
        // OFF the main thread (scope is Dispatchers.IO) to avoid an ANR. startForeground already ran.
        scope.launch { startHub() }
    }

    /** Build the DB/validator/pages and bind the HTTP server (with retry), then bring up/observe the
     *  hotspot. Each stage is labelled so a failure names the exact step on screen + in logcat. */
    private suspend fun startHub() {
        var stage = "start"
        try {
            ug.qaat.coordinator.ui.AppState.serverError = null
            stage = "init-database"
            ug.qaat.coordinator.di.Graph.init(this)
            stage = "build-validator"
            val store = RoomStore(ug.qaat.coordinator.di.Graph.db.dao())
            val validator = CheckinValidator(store, enforceDeviceLock = ug.qaat.coordinator.ui.AppState.ENFORCE_DEVICE_LOCK)

            stage = "read-assets"
            val attend = runCatching { assets.open("attend.html").bufferedReader().use { it.readText() } }.getOrDefault("")
            val gate = runCatching { assets.open("gate.html").bufferedReader().use { it.readText() } }.getOrDefault("")

            stage = "start-http-server"
            // A previous service instance (rapid restart / leaked engine) can still hold :8080. Stop
            // any lingering engine first, then bind — retrying because CIO binds asynchronously and a
            // just-released socket can briefly linger in TIME_WAIT.
            runCatching { ktor?.stop(0, 0) }; ktor = null
            // onClientReached ticks the reachability self-test each time a phone fetches the page.
            val srv = InRoomServer(validator, attend, gate,
                onClientReached = { ug.qaat.coordinator.ui.AppState.clientsReached++ })
            server = srv
            stage = "verify-listening"
            var listening = false
            for (attempt in 1..6) {
                val eng = runCatching { srv.start(8080) }.getOrNull()
                listening = awaitListening(8080, 1500)
                if (listening) { ktor = eng; break }
                runCatching { eng?.stop(0, 0) }              // this attempt didn't bind — clean it up
                android.util.Log.w(TAG, "port 8080 not up yet (attempt $attempt of 6); retrying")
                delay(700)                                    // let a lingering socket free
            }
            if (!listening) throw IllegalStateException("HTTP server did not bind on port 8080 after 6 tries (port busy or blocked). Reopen the app or reboot the phone.")
            android.util.Log.i(TAG, "hub listening on :8080")
            ug.qaat.coordinator.ui.AppState.serverReady = true

            // Advertise on the LAN so the student app auto-discovers us (no address typing).
            runCatching {
                nsd = NsdAdvertiser(this@SessionService).also {
                    it.register(ug.qaat.coordinator.ui.AppState.cohortLabel.orEmpty(), 8080)
                }
            }

            if (ug.qaat.coordinator.ui.AppState.useSystemHotspot) {
                // System-hotspot mode: the coordinator runs their OWN phone hotspot; we just poll for
                // our IP on it so the check-in + lecturer-gate URLs point at the right gateway.
                // A manual gateway IP (set by the coordinator, read off a joined phone) wins over
                // auto-detect — essential on phones that hide the hotspot interface from apps.
                val manual = ug.qaat.coordinator.ui.AppState.manualHotspotIp?.takeIf { it.isNotBlank() }
                if (manual != null) {
                    ug.qaat.coordinator.ui.AppState.inRoomIp = manual
                    ug.qaat.coordinator.ui.AppState.hotspotUp = true
                    update("Serving on your hotspot ($manual). Students: join your cohort's Wi-Fi.")
                    return
                }
                // Seed the check-in address with the CONVENTIONAL phone-hotspot gateway (192.168.43.1)
                // immediately — a coordinator's own hotspot is on this subnet, NOT the LocalOnlyHotspot
                // .49.1, so serving on .49.1 gives students "can't be reached". Show it right away
                // (don't wait for detection), then refine to the real gateway if we can read it.
                ug.qaat.coordinator.ui.AppState.inRoomIp = ug.qaat.coordinator.ui.AppState.SYSTEM_HOTSPOT_IP
                ug.qaat.coordinator.ui.AppState.hotspotUp = true
                update("Serving at ${ug.qaat.coordinator.ui.AppState.SYSTEM_HOTSPOT_IP}. Students: join your cohort's Wi-Fi, then open the Check-in address.")
                repeat(120) {
                    val ip = HotspotManager.detectApIp()
                    if (ip != null && ip != ug.qaat.coordinator.ui.AppState.inRoomIp) {
                        ug.qaat.coordinator.ui.AppState.inRoomIp = ip
                        ug.qaat.coordinator.ui.AppState.hotspotDiag = null
                        update("Serving on your hotspot ($ip). Students: join your cohort's Wi-Fi.")
                    }
                    if (ip == null) {
                        // Keep the seeded .43.1 address live; just record what interfaces we DO see so a
                        // coordinator whose gateway differs can read it and set the IP manually.
                        val diag = HotspotManager.listPrivateV4()
                        ug.qaat.coordinator.ui.AppState.hotspotDiag =
                            if (diag.isEmpty()) null
                            else diag.joinToString(", ") { "${it.first}=${it.second}" }
                    }
                    delay(3000)
                }
            } else {
                // LocalOnlyHotspot mode (the shipping model): the app OWNS the hotspot, so its gateway
                // is the fixed, known 192.168.49.1 (same on every coordinator). We set that as the
                // serving IP (a manual override or a successful detect wins if present). The callback
                // needs a Looper, so register it on the main thread.
                withContext(Dispatchers.Main) {
                    hotspotMgr = HotspotManager(this@SessionService)
                    hotspotMgr.start(
                        onReady = { info ->
                            hotspot.set(info)
                            ug.qaat.coordinator.ui.AppState.hotspotSsid = info.ssid
                            ug.qaat.coordinator.ui.AppState.hotspotPass = info.passphrase
                            ug.qaat.coordinator.ui.AppState.inRoomIp =
                                ug.qaat.coordinator.ui.AppState.manualHotspotIp?.takeIf { it.isNotBlank() }
                                    ?: HotspotManager.detectApIp()
                                    ?: ug.qaat.coordinator.ui.AppState.LOCAL_HOTSPOT_IP
                            ug.qaat.coordinator.ui.AppState.hotspotUp = true
                            update("Room Wi-Fi up: ${info.ssid}. Students scan the Connect QR to join.")
                        },
                        onError = { update("Couldn't start the room Wi-Fi ($it). Grant location/nearby-devices, or set the Hotspot IP manually.") },
                    )
                }
            }
        } catch (e: Throwable) {
            android.util.Log.e(TAG, "hub startup failed at stage=$stage", e)
            ug.qaat.coordinator.ui.AppState.serverReady = false
            ug.qaat.coordinator.ui.AppState.serverError =
                "Failed at [$stage]: ${e.javaClass.simpleName}: ${e.message}"
            update("Session server error at [$stage]: ${e.message}")
        }
    }

    /** Poll a localhost port until something is listening, or the timeout elapses. */
    private suspend fun awaitListening(port: Int, timeoutMs: Long): Boolean {
        val deadline = System.currentTimeMillis() + timeoutMs
        while (System.currentTimeMillis() < deadline) {
            val up = runCatching {
                java.net.Socket().use { it.connect(java.net.InetSocketAddress("127.0.0.1", port), 400) }; true
            }.getOrDefault(false)
            if (up) return true
            delay(200)
        }
        return false
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
        runCatching { nsd?.unregister() }; nsd = null
        runCatching { ktor?.stop(100, 200) }
        ktor = null
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
