package ug.qaat.coordinator.student

import android.content.Context
import android.net.ConnectivityManager
import android.net.nsd.NsdManager
import android.net.nsd.NsdServiceInfo
import android.net.wifi.WifiManager
import io.ktor.client.request.*
import io.ktor.client.statement.*
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.withTimeoutOrNull
import java.util.concurrent.atomic.AtomicBoolean
import kotlin.coroutines.resume

/** Finds the coordinator's in-room server on the hotspot LAN. Strategy, most-reliable first:
 *   1. Probe the phone's ACTUAL default gateway (the coordinator's phone) — works on ANY hotspot
 *      subnet, so it fixes "site can't be reached" when the gateway isn't the usual 192.168.43.1.
 *   2. NSD/mDNS "_qaat._tcp" — clean, name-based, but filtered on some hotspot stacks.
 *   3. A short list of well-known hotspot gateways, for stacks that hide DHCP info from apps.
 *  Returns "http://host:port" or null. */
class Discovery(context: Context) {
    private val appContext = context.applicationContext
    private val nsd = appContext.getSystemService(Context.NSD_SERVICE) as NsdManager
    private val lan = StudentNet.lanClient()

    suspend fun find(timeoutMs: Long = 6000): String? {
        // 1) The real gateway of the network this phone joined = the coordinator's phone. Direct,
        //    fast, and subnet-agnostic — this is the primary "just works" path.
        for (ip in gatewayCandidates()) {
            val url = "http://$ip:8080"
            if (probe(url)) return url
        }
        // 2) NSD/mDNS by service name.
        val viaNsd = withTimeoutOrNull(timeoutMs) { discoverViaNsd() }
        if (viaNsd != null) return viaNsd
        // 3) Last-resort static gateways (Android LocalOnlyHotspot, common router defaults, iOS-style).
        for (ip in listOf("192.168.43.1", "192.168.49.1", "192.168.0.1", "192.168.1.1", "172.20.10.1")) {
            val url = "http://$ip:8080"
            if (probe(url)) return url
        }
        return null
    }

    /** Candidate gateway IPs for the Wi-Fi this phone is currently joined to, derived from DHCP.
     *  Ordered best-first and de-duplicated; invalid/zero addresses are dropped. */
    private fun gatewayCandidates(): List<String> {
        val out = LinkedHashSet<String>()
        // Modern path: the default route's gateway on the active network.
        runCatching {
            val cm = appContext.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
            cm.activeNetwork?.let { cm.getLinkProperties(it) }?.routes?.forEach { r ->
                if (r.isDefaultRoute) r.gateway?.hostAddress?.let { if (it.contains('.')) out.add(it) }
            }
        }
        // Legacy path (works even when the active-network default route is hidden): Wi-Fi DHCP info.
        runCatching {
            val wifi = appContext.getSystemService(Context.WIFI_SERVICE) as WifiManager
            @Suppress("DEPRECATION") val dhcp = wifi.dhcpInfo
            if (dhcp != null) {
                intToIp(dhcp.gateway)?.let { out.add(it) }
                intToIp(dhcp.serverAddress)?.let { out.add(it) }   // on a phone hotspot, usually == gateway
                // The gateway is almost always .1 of this phone's own subnet — a solid backstop.
                intToIp(dhcp.ipAddress)?.substringBeforeLast('.', "")?.takeIf { it.isNotBlank() }?.let { out.add("$it.1") }
            }
        }
        return out.filter { it != "0.0.0.0" && !it.startsWith("0.") }
    }

    // DhcpInfo stores addresses as little-endian ints (low byte = first octet). Returns null for 0.
    private fun intToIp(a: Int): String? =
        if (a == 0) null else "${a and 0xff}.${a shr 8 and 0xff}.${a shr 16 and 0xff}.${a shr 24 and 0xff}"

    private suspend fun probe(baseUrl: String): Boolean = runCatching {
        lan.get("$baseUrl/session").status.value in 200..299
    }.getOrDefault(false)

    private suspend fun discoverViaNsd(): String? = suspendCancellableCoroutine { cont ->
        val done = AtomicBoolean(false)
        lateinit var listener: NsdManager.DiscoveryListener
        fun finish(url: String?) {
            if (done.compareAndSet(false, true)) {
                runCatching { nsd.stopServiceDiscovery(listener) }
                if (cont.isActive) cont.resume(url)
            }
        }
        listener = object : NsdManager.DiscoveryListener {
            override fun onDiscoveryStarted(t: String) {}
            override fun onDiscoveryStopped(t: String) {}
            override fun onStartDiscoveryFailed(t: String, e: Int) = finish(null)
            override fun onStopDiscoveryFailed(t: String, e: Int) {}
            override fun onServiceLost(s: NsdServiceInfo) {}
            override fun onServiceFound(s: NsdServiceInfo) {
                runCatching {
                    nsd.resolveService(s, object : NsdManager.ResolveListener {
                        override fun onResolveFailed(si: NsdServiceInfo, e: Int) {}
                        override fun onServiceResolved(si: NsdServiceInfo) {
                            val host = si.host?.hostAddress ?: return
                            finish("http://$host:${si.port}")
                        }
                    })
                }
            }
        }
        runCatching { nsd.discoverServices("_qaat._tcp.", NsdManager.PROTOCOL_DNS_SD, listener) }
            .onFailure { finish(null) }
        cont.invokeOnCancellation { if (done.compareAndSet(false, true)) runCatching { nsd.stopServiceDiscovery(listener) } }
    }
}
