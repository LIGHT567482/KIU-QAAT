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

    companion object {
        private fun isPrivateV4(ip: String) =
            ip.startsWith("192.168.") || ip.startsWith("10.") ||
            Regex("^172\\.(1[6-9]|2[0-9]|3[01])\\.").containsMatchIn(ip)

        /** Every non-loopback private IPv4 this phone currently has, as "iface -> ip" — logged and
         *  surfaced so we can SEE what the hotspot gateway actually is when auto-detect struggles. */
        fun listPrivateV4(): List<Pair<String, String>> = runCatching {
            java.net.NetworkInterface.getNetworkInterfaces().toList()
                .flatMap { nif ->
                    nif.inetAddresses.toList().filterIsInstance<java.net.Inet4Address>()
                        .mapNotNull { it.hostAddress }.filter { isPrivateV4(it) }.map { nif.name to it }
                }
        }.getOrDefault(emptyList())

        /** Best-effort: the phone's own IPv4 on its hotspot/AP interface — the gateway CLIENTS hit.
         *  Robust to devices whose hotspot subnet isn't 192.168.43.x: prefers a tether/AP-named
         *  interface, then any address ending in .1 (the conventional gateway), then any private v4.
         *  Returns null until a hotspot address actually exists. */
        fun detectApIp(): String? = runCatching {
            val candidates = java.net.NetworkInterface.getNetworkInterfaces().toList()
                .filter { runCatching { it.isUp && !it.isLoopback }.getOrDefault(false) }
                .flatMap { nif ->
                    nif.inetAddresses.toList().filterIsInstance<java.net.Inet4Address>()
                        .mapNotNull { it.hostAddress }.filter { isPrivateV4(it) }.map { nif.name.lowercase() to it }
                }
            android.util.Log.i("QAAT_HUB", "network interfaces (private v4): $candidates")
            val apName = listOf("ap", "softap", "swlan", "wlan1", "rndis", "tether", "usb")
            // A hotspot HOST is always the .1 of its subnet. Only accept a .1 — never a client
            // address (e.g. the phone's own IP on a Wi-Fi it JOINED, like a home router's 192.168.1.8).
            candidates.firstOrNull { (name, ip) -> ip.endsWith(".1") && apName.any { name.contains(it) } }?.second
                ?: candidates.firstOrNull { (_, ip) -> ip.endsWith(".1") }?.second
        }.getOrNull()
    }

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
