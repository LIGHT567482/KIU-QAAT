package ug.qaat.coordinator.hotspot

import android.content.Context
import android.net.nsd.NsdManager
import android.net.nsd.NsdServiceInfo

/**
 * Advertises the in-room server over the hotspot LAN via NSD/mDNS as "_qaat._tcp" on port 8080,
 * so the STUDENT app can auto-discover the coordinator without typing an address (this is what
 * kills the "can't be reached" IP-guessing for the app path). Best-effort throughout: a
 * registration failure must never affect the running session.
 */
class NsdAdvertiser(context: Context) {
    private val nsd = context.applicationContext.getSystemService(Context.NSD_SERVICE) as NsdManager
    private var listener: NsdManager.RegistrationListener? = null

    fun register(cohort: String, port: Int = 8080) {
        unregister()
        val info = NsdServiceInfo().apply {
            // Name shown in the student app's discovery list; keep the cohort readable but short.
            serviceName = "QAAT" + cohort.take(24).let { if (it.isBlank()) "" else " $it" }
            serviceType = "_qaat._tcp."
            setPort(port)
            runCatching { setAttribute("cohort", cohort.take(60)) }
        }
        val l = object : NsdManager.RegistrationListener {
            override fun onServiceRegistered(s: NsdServiceInfo) {}
            override fun onRegistrationFailed(s: NsdServiceInfo, err: Int) {}
            override fun onServiceUnregistered(s: NsdServiceInfo) {}
            override fun onUnregistrationFailed(s: NsdServiceInfo, err: Int) {}
        }
        listener = l
        runCatching { nsd.registerService(info, NsdManager.PROTOCOL_DNS_SD, l) }
    }

    fun unregister() {
        listener?.let { l -> runCatching { nsd.unregisterService(l) } }
        listener = null
    }
}
