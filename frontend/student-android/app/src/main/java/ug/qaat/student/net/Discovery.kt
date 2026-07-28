package ug.qaat.student.net

import android.content.Context
import android.net.nsd.NsdManager
import android.net.nsd.NsdServiceInfo
import io.ktor.client.request.*
import io.ktor.client.statement.*
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.withTimeoutOrNull
import java.util.concurrent.atomic.AtomicBoolean
import kotlin.coroutines.resume

/**
 * Finds the coordinator's in-room server on the hotspot LAN so the student never types an address
 * (this is what removes the "can't be reached" IP-guessing for the app path). Primary path: NSD/mDNS
 * discovery of "_qaat._tcp". Fallback: probe the conventional hotspot gateways — a phone's own
 * hotspot is 192.168.43.1, the app-owned LocalOnlyHotspot is 192.168.49.1 — at :8080/session.
 * Returns "http://host:port" or null.
 */
class Discovery(context: Context) {
    private val nsd = context.applicationContext.getSystemService(Context.NSD_SERVICE) as NsdManager
    private val lan = Net.lanClient()

    suspend fun find(timeoutMs: Long = 6000): String? {
        val viaNsd = withTimeoutOrNull(timeoutMs) { discoverViaNsd() }
        if (viaNsd != null) return viaNsd
        for (ip in listOf("192.168.43.1", "192.168.49.1")) {
            val url = "http://$ip:8080"
            if (probe(url)) return url
        }
        return null
    }

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
