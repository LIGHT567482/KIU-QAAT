package ug.qaat.coordinator.hotspot

import android.content.Context
import android.net.wifi.WifiManager
import android.os.Build

/**
 * Starts the room's Wi-Fi via LocalOnlyHotspot — the only hotspot a non-rooted app
 * may start. STOCK ANDROID LIMITS (documented): the OS generates the SSID/passphrase
 * (no custom name), and the app cannot deauth/MAC-block clients — a checked-in
 * student must disconnect themselves to free a slot (manual rotation).
 */
class HotspotManager(private val context: Context) {
    data class Info(val ssid: String, val passphrase: String)

    private var reservation: WifiManager.LocalOnlyHotspotReservation? = null

    fun start(onReady: (Info) -> Unit, onError: (String) -> Unit) {
        // startLocalOnlyHotspot throws SecurityException without the runtime location/
        // nearby-wifi permission, and IllegalStateException in some states — catch both so a
        // hotspot problem never crashes the app; the session can still run over shared Wi-Fi.
        try {
            val wifi = context.applicationContext.getSystemService(Context.WIFI_SERVICE) as WifiManager
            wifi.startLocalOnlyHotspot(object : WifiManager.LocalOnlyHotspotCallback() {
                override fun onStarted(res: WifiManager.LocalOnlyHotspotReservation) {
                    reservation = res
                    val (ssid, pass) = readConfig(res)
                    onReady(Info(ssid, pass))
                }
                override fun onFailed(reason: Int) = onError("hotspot failed: $reason")
            }, null)
        } catch (e: Throwable) {
            onError(e.message ?: "hotspot permission/availability error")
        }
    }

    fun stop() { reservation?.close(); reservation = null }

    @Suppress("DEPRECATION")
    private fun readConfig(res: WifiManager.LocalOnlyHotspotReservation): Pair<String, String> =
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            val c = res.softApConfiguration
            (c.ssid ?: "") to (c.passphrase ?: "")
        } else {
            val c = res.wifiConfiguration
            (c?.SSID ?: "") to (c?.preSharedKey ?: "")
        }
}
