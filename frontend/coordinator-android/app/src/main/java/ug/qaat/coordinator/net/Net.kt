package ug.qaat.coordinator.net

import android.content.Context
import io.ktor.client.*
import io.ktor.client.engine.okhttp.*
import ug.qaat.coordinator.BuildConfig
import ug.qaat.coordinator.R
import java.security.KeyStore
import java.security.cert.CertificateFactory
import java.security.cert.X509Certificate
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLContext
import javax.net.ssl.TrustManager
import javax.net.ssl.TrustManagerFactory
import javax.net.ssl.X509TrustManager

/**
 * Backend base URL + a shared Ktor client that works for BOTH deployments at once, from a
 * single build — so you can test/maintain against your local server AND ship to the world:
 *
 *   • URL is switchable at RUNTIME (Net.setBaseUrl / the login "server" field), defaulting
 *     to BuildConfig.API_BASE. Point it at your LAN IP for testing or your cloud domain for
 *     production without rebuilding.
 *   • TLS trusts BOTH the phone's normal CA store (your cloud domain's real Let's Encrypt/CA
 *     cert) AND the app-embedded self-signed QAAT cert (your local backend). Hostname
 *     verification stays strict for public hosts (production security) and is relaxed only
 *     for private LAN IPs (so the self-signed local server works at any IP the router hands
 *     out). No cert install on the phone, ever.
 */
object Net {
    private lateinit var appContext: Context
    private var override: String? = null

    fun init(context: Context) { appContext = context.applicationContext }

    /** Effective backend URL: a runtime override if set, else the build default. */
    val baseUrl: String get() = override?.takeIf { it.isNotBlank() } ?: BuildConfig.API_BASE
    fun setBaseUrl(url: String?) { override = url?.trim()?.removeSuffix("/")?.takeIf { it.isNotBlank() } }

    // Composite trust: try the system CAs first (real cloud cert), then the embedded cert
    // (local self-signed). A server is trusted if EITHER accepts it.
    private val trust: Pair<SSLContext, X509TrustManager> by lazy {
        val sysTmf = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm())
        sysTmf.init(null as KeyStore?)
        val sysTm = sysTmf.trustManagers.first { it is X509TrustManager } as X509TrustManager

        val cf = CertificateFactory.getInstance("X.509")
        // Bundle = local self-signed cert + the cloud roots (GTS Root R4 / GlobalSign Root CA)
        // that anchor the onrender.com chain. Embedding them means the cloud cert validates
        // even on older phones whose OS CA store predates the Google Trust Services roots
        // (the "Trust anchor for certification path not found" case).
        val cas = appContext.resources.openRawResource(R.raw.qaat_ca).use { cf.generateCertificates(it) }
        val ks = KeyStore.getInstance(KeyStore.getDefaultType()).apply {
            load(null, null)
            cas.forEachIndexed { i, c -> setCertificateEntry("qaat$i", c) }
        }
        val embTmf = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm())
        embTmf.init(ks)
        val embTm = embTmf.trustManagers.first { it is X509TrustManager } as X509TrustManager

        val composite = object : X509TrustManager {
            override fun checkClientTrusted(chain: Array<out X509Certificate>?, authType: String?) =
                sysTm.checkClientTrusted(chain, authType)
            override fun checkServerTrusted(chain: Array<out X509Certificate>?, authType: String?) {
                try { sysTm.checkServerTrusted(chain, authType) }
                catch (e: Exception) { embTm.checkServerTrusted(chain, authType) }
            }
            override fun getAcceptedIssuers(): Array<X509Certificate> = sysTm.acceptedIssuers + embTm.acceptedIssuers
        }
        val ssl = SSLContext.getInstance("TLS").apply { init(null, arrayOf<TrustManager>(composite), null) }
        ssl to composite
    }

    private fun isPrivateHost(h: String): Boolean =
        h == "localhost" || h.startsWith("127.") ||
        h.matches(Regex("^(10\\.|192\\.168\\.|172\\.(1[6-9]|2[0-9]|3[01])\\.).*"))

    // ONE shared client. Building a fresh OkHttp engine on every call (5 client classes ×
    // every screen) leaked sockets/file descriptors and surfaced as intermittent socket
    // errors; a single reused client is both correct and lighter. The base URL is passed
    // per-request (full URL), so runtime server switching still works.
    private val shared: HttpClient by lazy { build() }
    fun client(): HttpClient = shared

    private fun build(): HttpClient = HttpClient(OkHttp) {
        engine {
            config {
                val (ssl, tm) = trust
                sslSocketFactory(ssl.socketFactory, tm)
                val strict = HttpsURLConnection.getDefaultHostnameVerifier()
                // Strict for public domains (cloud); relaxed only for private LAN IPs (local).
                hostnameVerifier { host, session -> strict.verify(host, session) || isPrivateHost(host) }
            }
        }
    }
}
