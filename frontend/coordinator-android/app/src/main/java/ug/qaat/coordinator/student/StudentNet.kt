package ug.qaat.coordinator.student

import android.content.Context
import io.ktor.client.*
import io.ktor.client.engine.okhttp.*
import io.ktor.client.plugins.HttpTimeout

/** Short-timeout client for in-room LAN calls (discovery probe + check-in) — fast and local.
 *  Cloud calls (register-device, progress) reuse the app's shared [ug.qaat.coordinator.net.Net].
 *
 *  PINNED TO THE WI-FI when a context is supplied. The coordinator's hotspot has no uplink, so
 *  Android marks it unvalidated and keeps mobile data as the default network — every unbound socket
 *  then leaves over cellular, where the hotspot's address does not exist, and the app reports that
 *  it cannot find the coordinator. See [LanNetwork]. Passing the context is therefore not optional
 *  in practice; the null default exists only so an older call site fails soft rather than not
 *  compiling. */
internal object StudentNet {
    fun lanClient(context: Context? = null): HttpClient = HttpClient(OkHttp) {
        expectSuccess = false
        engine {
            LanNetwork.socketFactory(context)?.let { sf ->
                config { socketFactory(sf) }
            }
        }
        install(HttpTimeout) {
            requestTimeoutMillis = 6_000
            connectTimeoutMillis = 3_000
            socketTimeoutMillis = 6_000
        }
    }
}
