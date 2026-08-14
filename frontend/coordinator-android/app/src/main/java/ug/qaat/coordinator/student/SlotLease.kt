package ug.qaat.coordinator.student

import android.content.Context
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.net.NetworkRequest
import android.net.wifi.WifiNetworkSpecifier
import android.os.Build
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.withTimeoutOrNull
import java.util.concurrent.atomic.AtomicReference
import kotlin.coroutines.resume

/**
 * A BORROWED SLOT ON THE COORDINATOR'S HOTSPOT, HELD FOR SECONDS AND THEN GIVEN BACK.
 *
 * The hotspot admits about ten clients and the hall holds two hundred, so the only way everybody
 * checks in is if each phone takes its turn and leaves. The old flow could not do that. The student
 * joined from Wi-Fi Settings, which makes Android SAVE the network and auto-reconnect to it, and
 * the app then asked them — politely, in a red box — to turn Wi-Fi off for the sake of the queue.
 * Roughly nobody did, and every phone that did not held a slot for the rest of the lecture.
 *
 * IT IS NOT POSSIBLE TO FIX THIS FROM THE OTHER END. An app cannot deauthenticate a client from a
 * hotspot it hosts: `setBlockedClientList` and `setMaxNumberOfClients` are behind NETWORK_SETTINGS,
 * which is system-signature only. And an app cannot drop its OWN Wi-Fi connection either —
 * `WifiManager.disconnect()` and `removeNetwork()` have been no-ops for apps since Android 10. So
 * neither the coordinator's phone nor a Settings-joined student phone can free a slot on purpose.
 *
 * [WifiNetworkSpecifier] is the one mechanism that can, and this class is built entirely around it.
 * The connection it creates belongs to the CALLBACK rather than to the device: while we hold the
 * callback we have the network, and the moment we unregister it Android tears the association down.
 * That is the release the rest of the system needs, and two further properties fall out of it that
 * matter as much as the release itself:
 *
 *   - The network is **never saved**, so there is no auto-reconnect to undo our work a second later.
 *   - The connection is **app-scoped**, so it cannot become the device's default route and strand
 *     the student's other traffic on a Wi-Fi with no uplink. [LanNetwork] documents at length the
 *     trouble that caused when the join was global; here the problem cannot arise.
 *
 * A SINGLETON ON PURPOSE. One phone can hold at most one slot, and a leaked callback IS a leaked
 * Wi-Fi connection — the exact failure this class exists to prevent. Keeping the state in one place
 * makes [release] unconditionally safe to call, from anywhere, as many times as you like.
 *
 * BELOW ANDROID 10 none of this exists, and [supported] is false. Those students keep the old
 * Settings-join flow and the old request to disconnect; there is no mechanism on that platform to
 * do better, and pretending otherwise would just fail silently.
 */
object SlotLease {

    /** Whether this device can hold and release a slot at all. False below Android 10. */
    val supported: Boolean get() = Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q

    /** How long a slot may be held before it is dropped no matter what.
     *
     *  Mirrors the hub's own hold (see SlotWarden), and is deliberately enforced HERE as well. The
     *  hub telling us to leave is the normal path, but a hub that has crashed, or gone out of range
     *  mid-turn, tells us nothing at all — and that is precisely the case where a stranded phone
     *  would sit on a slot for the rest of the lecture. A local timer needs nobody's cooperation. */
    const val HOLD_MILLIS = 15_000L

    /** How long to wait for the association before giving up. Joining a hotspot involves a system
     *  approval dialog on first use, so this allows for a student reading it and tapping. */
    private const val ACQUIRE_TIMEOUT_MS = 20_000L

    private val callback = AtomicReference<ConnectivityManager.NetworkCallback?>(null)
    private val bound = AtomicReference<Network?>(null)
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    private var watchdog: Job? = null

    /** The network we currently hold, or null. Hand this to [StudentNet] so LAN sockets go over the
     *  hotspot rather than out over cellular, where the coordinator's address does not exist. */
    fun network(): Network? = bound.get()

    val held: Boolean get() = bound.get() != null

    /**
     * Join the coordinator's hotspot and start the clock on our turn.
     *
     * Returns the network on success, or null if we could not join — a wrong passphrase, a student
     * who dismissed the system dialog, a hotspot that has gone away. Callers should treat null as
     * "not in the room" and say so, rather than retrying in a tight loop: each attempt puts a
     * dialog in front of the student.
     *
     * Any slot already held is released first. Asking for a second one is always a bug at the call
     * site, but resolving it by leaking the first would be a worse one.
     */
    suspend fun acquire(context: Context, ssid: String, passphrase: String): Network? {
        if (!supported) return null
        val name = ssid.trim().trim('"')            // WifiConfiguration.SSID arrives quoted; the
        if (name.isEmpty()) return null             // specifier builder wants it bare.
        release(context)

        val app = context.applicationContext
        val cm = app.getSystemService(Context.CONNECTIVITY_SERVICE) as? ConnectivityManager ?: return null

        val specifier = WifiNetworkSpecifier.Builder()
            .setSsid(name)
            .apply { if (passphrase.isNotBlank()) setWpa2Passphrase(passphrase) }
            .build()

        val request = NetworkRequest.Builder()
            .addTransportType(NetworkCapabilities.TRANSPORT_WIFI)
            // THE LINE THIS WHOLE CLASS FAILS WITHOUT. The coordinator's hotspot has no uplink —
            // that is the design, it is a room and not an internet connection. NetworkRequest
            // requires NET_CAPABILITY_INTERNET by default, so leaving it in means the request never
            // matches and the student waits out the timeout on a hotspot they are standing next to.
            .removeCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
            .setNetworkSpecifier(specifier)
            .build()

        val net = withTimeoutOrNull(ACQUIRE_TIMEOUT_MS) {
            suspendCancellableCoroutine { cont ->
                val cb = object : ConnectivityManager.NetworkCallback() {
                    override fun onAvailable(network: Network) {
                        if (cont.isActive) cont.resume(network)
                    }
                    override fun onUnavailable() {
                        if (cont.isActive) cont.resume(null)
                    }
                    // The hotspot went away under us (coordinator closed the session, walked out of
                    // range). The slot is gone whether we like it or not; drop our record of it so
                    // the UI stops believing it is connected.
                    override fun onLost(network: Network) {
                        bound.compareAndSet(network, null)
                    }
                }
                callback.set(cb)
                runCatching { cm.requestNetwork(request, cb) }
                    .onFailure { if (cont.isActive) cont.resume(null) }
                cont.invokeOnCancellation { /* release() below does the unregistering */ }
            }
        }

        if (net == null) {
            release(app)          // never leave a half-registered callback behind
            return null
        }
        bound.set(net)
        startWatchdog(app)
        return net
    }

    /**
     * Let the slot go. Idempotent, safe from any thread, and safe to call when nothing is held.
     *
     * This is the whole point of the class, so it is written to be impossible to get wrong: every
     * path out of a turn ends here, and calling it twice costs nothing.
     */
    fun release(context: Context) {
        watchdog?.cancel()
        watchdog = null
        val cb = callback.getAndSet(null) ?: run { bound.set(null); return }
        bound.set(null)
        val cm = context.applicationContext
            .getSystemService(Context.CONNECTIVITY_SERVICE) as? ConnectivityManager
        // Unregistering IS the disconnect: the association exists only for as long as this callback
        // is registered. Wrapped because unregistering a callback the system has already forgotten
        // throws, and a failure to release must never take the app down with it.
        runCatching { cm?.unregisterNetworkCallback(cb) }
    }

    /**
     * The backstop: drop the slot after [HOLD_MILLIS] whatever else happens.
     *
     * Everything else that releases a slot depends on something going right — the hub answering, the
     * check-in completing, the student's screen still being open. This depends on nothing. It is the
     * reason a phone that loses the hub mid-turn cannot hold a slot for the rest of the lecture.
     */
    private fun startWatchdog(context: Context) {
        watchdog?.cancel()
        watchdog = scope.launch {
            delay(HOLD_MILLIS)
            release(context)
        }
    }
}
