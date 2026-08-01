package ug.qaat.patroller.net

import android.content.Context
import io.ktor.client.*
import io.ktor.client.engine.okhttp.*
import io.ktor.client.plugins.HttpTimeout
import io.ktor.client.request.*
import ug.qaat.patroller.BuildConfig
import ug.qaat.patroller.R
import java.security.KeyStore
import java.security.cert.CertPathValidator
import java.security.cert.CertificateFactory
import java.security.cert.PKIXParameters
import java.security.cert.TrustAnchor
import java.security.cert.X509Certificate
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLContext
import javax.net.ssl.TrustManager
import javax.net.ssl.TrustManagerFactory
import javax.net.ssl.X509TrustManager

/**
 * COPIED VERBATIM from the coordinator app's locked networking (ug.qaat.coordinator.net.Net) so the
 * patroller app installs & runs on EVERY phone exactly like the coordinator app. Do not "improve"
 * the TLS here — this is the baseline that was hard-won to work on cheap/old devices.
 */
object Net {
    private lateinit var appContext: Context
    private var override: String? = null

    fun init(context: Context) { appContext = context.applicationContext }

    val baseUrl: String get() = override?.takeIf { it.isNotBlank() } ?: BuildConfig.API_BASE
    fun setBaseUrl(url: String?) { override = url?.trim()?.removeSuffix("/")?.takeIf { it.isNotBlank() } }

    suspend fun warmUp() {
        runCatching { client().get(BuildConfig.AUTH_WARM_URL) }
        runCatching { client().get("$baseUrl/health") }
    }

    private val trust: Pair<SSLContext, X509TrustManager> by lazy {
        val sysTmf = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm())
        sysTmf.init(null as KeyStore?)
        val sysTm = sysTmf.trustManagers.first { it is X509TrustManager } as X509TrustManager

        val cf = CertificateFactory.getInstance("X.509")
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
                try { sysTm.checkServerTrusted(chain, authType); return } catch (_: Exception) {}
                try { embTm.checkServerTrusted(chain, authType); return }
                catch (strict: Exception) {
                    dateNeutralServerTrusted(chain ?: throw strict, sysTm, embTm, strict)
                }
            }
            override fun getAcceptedIssuers(): Array<X509Certificate> = sysTm.acceptedIssuers + embTm.acceptedIssuers
        }
        val ssl = SSLContext.getInstance("TLS").apply { init(null, arrayOf<TrustManager>(composite), null) }
        ssl to composite
    }

    fun friendly(t: Throwable): String {
        android.util.Log.w("QAAT_NET", "network call failed: ${t.javaClass.simpleName}: ${t.message}", t)
        val m = (t.message ?: "").lowercase()
        return when {
            "timeout" in m || "timed out" in m ->
                "The server is waking up (it sleeps when idle). Please wait a few seconds and tap again."
            "unable to resolve host" in m || "failed to connect" in m || "unreachable" in m || "connect" in m ->
                "Can't reach the server — check your internet connection and try again."
            "not yet valid" in m || "expired" in m || "not valid" in m || "current time" in m || "validity" in m ->
                "Secure connection failed — your phone's date & time look wrong. Set them to automatic (network) date/time, then try again."
            "trust anchor" in m || "certif" in m || "handshake" in m || "ssl" in m ->
                "Secure connection failed. First set your phone's date & time to automatic and try again; if it persists, update the app."
            else -> t.message?.takeIf { it.isNotBlank() && it.length < 120 } ?: "Something went wrong. Please try again."
        }
    }

    private fun dateNeutralServerTrusted(
        chain: Array<out X509Certificate>,
        sysTm: X509TrustManager,
        embTm: X509TrustManager,
        cause: Exception,
    ) {
        runCatching {
            val anchorCerts = (sysTm.acceptedIssuers.asList() + embTm.acceptedIssuers.asList()).toSet()
            val pathCerts = chain.filter { it !in anchorCerts }.ifEmpty { chain.toList() }
            val certPath = CertificateFactory.getInstance("X.509").generateCertPath(pathCerts)
            val anchors = anchorCerts.map { TrustAnchor(it, null) }.toSet()
            val params = PKIXParameters(anchors).apply {
                isRevocationEnabled = false
                date = chain[0].notBefore
            }
            CertPathValidator.getInstance("PKIX").validate(certPath, params)
        }.getOrElse { throw cause }
    }

    private fun isPrivateHost(h: String): Boolean =
        h == "localhost" || h.startsWith("127.") ||
        h.matches(Regex("^(10\\.|192\\.168\\.|172\\.(1[6-9]|2[0-9]|3[01])\\.).*"))

    private val shared: HttpClient by lazy { build() }
    fun client(): HttpClient = shared

    private fun build(): HttpClient = HttpClient(OkHttp) {
        install(HttpTimeout) {
            connectTimeoutMillis = 30_000
            socketTimeoutMillis = 60_000
            requestTimeoutMillis = 120_000
        }
        engine {
            config {
                val (ssl, tm) = trust
                sslSocketFactory(ssl.socketFactory, tm)
                connectionSpecs(listOf(
                    okhttp3.ConnectionSpec.MODERN_TLS,
                    okhttp3.ConnectionSpec.COMPATIBLE_TLS,
                    okhttp3.ConnectionSpec.CLEARTEXT,
                ))
                val strict = HttpsURLConnection.getDefaultHostnameVerifier()
                hostnameVerifier { host, session -> strict.verify(host, session) || isPrivateHost(host) }
            }
        }
    }
}
