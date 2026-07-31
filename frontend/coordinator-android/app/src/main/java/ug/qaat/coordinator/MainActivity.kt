package ug.qaat.coordinator

import android.Manifest
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.core.content.ContextCompat
import ug.qaat.coordinator.di.Graph
import ug.qaat.coordinator.net.Net
import ug.qaat.coordinator.store.SessionStore
import ug.qaat.coordinator.ui.RootApp

/**
 * Hosts the coordinator UI. Requests the runtime permissions the in-room hotspot/server
 * need (location / nearby-wifi / notifications) so "Take attendance" doesn't fail.
 */
class MainActivity : ComponentActivity() {
    private val permLauncher =
        registerForActivityResult(ActivityResultContracts.RequestMultiplePermissions()) { /* best-effort */ }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        // Surface (and clear) a previous run's uncaught crash FIRST — before any init that could
        // itself throw — so a repeat startup failure is shown once instead of a silent close loop.
        runCatching {
            val f = java.io.File(filesDir, "last_crash.txt")
            if (f.exists()) { ug.qaat.coordinator.ui.AppState.lastCrash = f.readText(); f.delete() }
        }
        // Startup init is GUARDED end-to-end: on a phone where any of this throws (classically a
        // wrong clock breaking the Keystore that backs EncryptedSharedPreferences), we still reach
        // setContent so the app OPENS and can show the problem — instead of installing but
        // "refusing to run". SessionStore.init already degrades to plain prefs on such devices.
        runCatching {
            Graph.init(applicationContext)
            Net.init(applicationContext)      // load the embedded, pinned QAAT cert
            SessionStore.init(applicationContext)
            SessionStore.restoreTheme()
            // Always use the built-in KIU cloud backend (BuildConfig.API_BASE) on every phone — no
            // per-device server field anymore. Clear any override a previous build may have saved, so
            // a phone that once had a wrong address typed in self-heals instead of failing to log in.
            Net.setBaseUrl(null)
            SessionStore.saveServerUrl(null)
            SessionStore.hotspotIp()?.let { ug.qaat.coordinator.ui.AppState.manualHotspotIp = it; ug.qaat.coordinator.ui.AppState.inRoomIp = it }
            // Auto-login: restore the cached session so the app opens straight to work — no re-login.
            // The cached manifest (for fully-offline attendance) is hydrated off the main thread by
            // CoordinatorApp.
            SessionStore.restore()
            // Instant, offline institution branding from the bundled brand.json (backend overrides
            // when online). Applied even on the login screen (before any session/fetch).
            if (ug.qaat.coordinator.ui.AppState.branding == null)
                ug.qaat.coordinator.net.BrandDefault.load(applicationContext)?.let { ug.qaat.coordinator.ui.AppState.branding = it }
        }.onFailure {
            android.util.Log.e("QAAT", "startup init failed", it)
            if (ug.qaat.coordinator.ui.AppState.lastCrash == null)
                ug.qaat.coordinator.ui.AppState.lastCrash = "Startup init failed:\n" + android.util.Log.getStackTraceString(it)
        }
        requestRuntimePermissions()
        setContent { RootApp() }   // RootApp owns the branded MaterialTheme + role routing
    }

    /** Notifications are useful for everyone; the hotspot permissions (location / nearby-wifi) are
     *  ONLY needed by a COORDINATOR running the in-room hub, so students/lecturers are never asked
     *  for them. A coordinator who logs in fresh is prompted from CoordinatorApp instead. */
    private fun requestRuntimePermissions() {
        val needed = mutableListOf<String>()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            needed += Manifest.permission.POST_NOTIFICATIONS
        }
        if (ug.qaat.coordinator.ui.AppState.role == "COORDINATOR") {
            needed += Manifest.permission.ACCESS_FINE_LOCATION
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU)
                needed += Manifest.permission.NEARBY_WIFI_DEVICES
        }
        val toAsk = needed.filter {
            ContextCompat.checkSelfPermission(this, it) != PackageManager.PERMISSION_GRANTED
        }
        if (toAsk.isNotEmpty()) runCatching { permLauncher.launch(toAsk.toTypedArray()) }
    }
}
