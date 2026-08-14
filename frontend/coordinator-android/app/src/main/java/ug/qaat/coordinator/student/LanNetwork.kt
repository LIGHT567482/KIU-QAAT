package ug.qaat.coordinator.student

import android.content.Context
import android.net.ConnectivityManager
import android.net.LinkProperties
import android.net.Network
import android.net.NetworkCapabilities
import android.net.wifi.WifiManager
import javax.net.SocketFactory

/**
 * THE COORDINATOR'S HOTSPOT IS A NETWORK ANDROID DOES NOT WANT TO USE.
 *
 * The hotspot has no uplink — that is the whole design, it is a room, not an internet connection.
 * Android probes every Wi-Fi it joins, finds no internet, marks it "unvalidated", and keeps MOBILE
 * DATA as the default network. From then on every socket the app opens without saying otherwise
 * goes out over cellular, where 192.168.43.1 does not exist.
 *
 * So the lecturer connects to the hotspot, sees the Wi-Fi icon, and the app says it cannot find the
 * coordinator. Nothing is wrong with the hotspot, the server, or the code that talks to it: the
 * request never went near the Wi-Fi card. The old workaround was the line printed on half the
 * screens in this app — "make sure mobile data is OFF" — which is a real instruction that real
 * people forget while a hall waits, and which does not always work anyway, because Android will
 * quietly re-prefer a validated network the moment one appears.
 *
 * THE FIX IS TO NAME THE NETWORK. [socketFactory] hands back a factory bound to the Wi-Fi
 * transport, so LAN calls are pinned to the hotspot whatever Android would otherwise prefer.
 *
 * PER-CLIENT, NEVER bindProcessToNetwork(). Binding the whole process would pin the cloud calls to
 * the hotspot too — sync, notifications, the online-class code — and those need the internet the
 * hotspot does not have. Only the in-room client is pinned; everything else keeps the default route.
 */
internal object LanNetwork {

    /**
     * The Wi-Fi network this phone has joined, whether or not Android considers it usable.
     *
     * A LEASED slot wins outright when we hold one ([SlotLease]): that network is the hotspot, by
     * construction, handed to us by the system in `onAvailable`. There is nothing to search for and
     * nothing to guess, which is worth saying plainly because everything below this line is guesswork
     * — necessary guesswork, for students on Android 9 and for anyone who joined from Settings, but
     * guesswork all the same.
     */
    fun wifi(context: Context): Network? = SlotLease.network() ?: runCatching {
        val cm = context.applicationContext
            .getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
        // allNetworks rather than activeNetwork, deliberately: on an internet-less hotspot the
        // ACTIVE network is the cellular one, which is precisely the wrong answer and precisely
        // what made this look like a discovery bug.
        @Suppress("DEPRECATION")
        cm.allNetworks.firstOrNull { n ->
            cm.getNetworkCapabilities(n)?.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) == true
        }
    }.getOrNull()

    /** A socket factory pinned to the Wi-Fi, or null to leave the client on the default route. */
    fun socketFactory(context: Context?): SocketFactory? =
        context?.let { wifi(it)?.socketFactory }

    /**
     * Candidate gateway addresses for the joined Wi-Fi — the coordinator's phone.
     *
     * Read from the WI-FI network's own LinkProperties, not the active network's. Reading the
     * active network gave the cellular gateway on exactly the devices this needed to work on, so
     * the probe list did not contain the hotspot at all.
     */
    fun gateways(context: Context): List<String> {
        val app = context.applicationContext
        val out = LinkedHashSet<String>()

        runCatching {
            val cm = app.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
            val lp: LinkProperties? = wifi(app)?.let { cm.getLinkProperties(it) }
            lp?.routes?.forEach { r ->
                if (r.isDefaultRoute) r.gateway?.hostAddress?.let { if (it.contains('.')) out.add(it) }
            }
            // A hotspot often publishes no default route (it has nowhere to route TO). Its subnet
            // is still on the interface, and the coordinator's phone is .1 of it.
            lp?.linkAddresses?.forEach { la ->
                la.address?.hostAddress?.takeIf { it.contains('.') }?.let { ip ->
                    ip.substringBeforeLast('.', "").takeIf { it.isNotBlank() }?.let { out.add("$it.1") }
                }
            }
        }

        // Legacy DHCP path, for stacks that hide LinkProperties from apps.
        runCatching {
            val wm = app.getSystemService(Context.WIFI_SERVICE) as WifiManager
            @Suppress("DEPRECATION") val dhcp = wm.dhcpInfo
            if (dhcp != null) {
                intToIp(dhcp.gateway)?.let { out.add(it) }
                intToIp(dhcp.serverAddress)?.let { out.add(it) }
                intToIp(dhcp.ipAddress)?.substringBeforeLast('.', "")
                    ?.takeIf { it.isNotBlank() }?.let { out.add("$it.1") }
            }
        }

        return out.filter { it != "0.0.0.0" && !it.startsWith("0.") }
    }

    /** True when a Wi-Fi is joined at all — lets the UI say "you are not on any Wi-Fi" instead of
     *  "the coordinator could not be found", which sends people to the wrong problem. */
    fun onWifi(context: Context): Boolean = wifi(context) != null

    // DhcpInfo stores addresses as little-endian ints (low byte = first octet). Returns null for 0.
    private fun intToIp(a: Int): String? =
        if (a == 0) null else "${a and 0xff}.${a shr 8 and 0xff}.${a shr 16 and 0xff}.${a shr 24 and 0xff}"
}
