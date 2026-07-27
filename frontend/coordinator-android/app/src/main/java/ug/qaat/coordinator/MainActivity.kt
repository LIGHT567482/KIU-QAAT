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
import ug.qaat.coordinator.ui.CoordinatorApp

/**
 * Hosts the coordinator UI. Requests the runtime permissions the in-room hotspot/server
 * need (location / nearby-wifi / notifications) so "Take attendance" doesn't fail.
 */
class MainActivity : ComponentActivity() {
    private val permLauncher =
        registerForActivityResult(ActivityResultContracts.RequestMultiplePermissions()) { /* best-effort */ }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        Graph.init(applicationContext)
        Net.init(applicationContext)      // load the embedded, pinned QAAT cert
        SessionStore.init(applicationContext)
        SessionStore.restoreTheme()
        Net.setBaseUrl(SessionStore.serverUrl())   // runtime server override (local ⇄ cloud)
        SessionStore.hotspotIp()?.let { ug.qaat.coordinator.ui.AppState.manualHotspotIp = it; ug.qaat.coordinator.ui.AppState.inRoomIp = it }
        // Auto-login: restore the cached session so the app opens straight to work — no
        // re-login. The cached manifest (for fully-offline attendance) is hydrated off the
        // main thread by CoordinatorApp.
        runCatching { SessionStore.restore() }
        // Surface (and clear) a previous run's uncaught crash so it isn't a silent close.
        runCatching {
            val f = java.io.File(filesDir, "last_crash.txt")
            if (f.exists()) { ug.qaat.coordinator.ui.AppState.lastCrash = f.readText(); f.delete() }
        }
        requestRuntimePermissions()
        setContent { CoordinatorApp() }   // CoordinatorApp owns the branded MaterialTheme
    }

    /** LocalOnlyHotspot needs location (all APIs) or NEARBY_WIFI_DEVICES (API 33+); the
     *  foreground notification needs POST_NOTIFICATIONS (API 33+). Ask for whatever's missing. */
    private fun requestRuntimePermissions() {
        val needed = mutableListOf(Manifest.permission.ACCESS_FINE_LOCATION)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            needed += Manifest.permission.NEARBY_WIFI_DEVICES
            needed += Manifest.permission.POST_NOTIFICATIONS
        }
        val toAsk = needed.filter {
            ContextCompat.checkSelfPermission(this, it) != PackageManager.PERMISSION_GRANTED
        }
        if (toAsk.isNotEmpty()) runCatching { permLauncher.launch(toAsk.toTypedArray()) }
    }
}
