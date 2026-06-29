package ug.qaat.coordinator.net

import io.ktor.client.*
import io.ktor.client.engine.cio.*
import ug.qaat.coordinator.BuildConfig
import java.security.cert.X509Certificate
import javax.net.ssl.X509TrustManager

/** Backend base URL (BuildConfig.API_BASE) + a shared Ktor client. */
object Net {
    val baseUrl: String get() = BuildConfig.API_BASE

    /**
     * In DEBUG we trust all certs so a self-signed dev backend (10.0.2.2 / laptop LAN)
     * works without per-device cert installs. RELEASE uses the platform trust store.
     */
    fun client(): HttpClient = HttpClient(CIO) {
        engine {
            if (BuildConfig.DEBUG) {
                https {
                    trustManager = object : X509TrustManager {
                        override fun checkClientTrusted(c: Array<out X509Certificate>?, a: String?) {}
                        override fun checkServerTrusted(c: Array<out X509Certificate>?, a: String?) {}
                        override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
                    }
                }
            }
        }
    }
}
