package ug.qaat.coordinator.net

import android.content.Context
import io.ktor.client.*
import io.ktor.client.engine.okhttp.*
import io.ktor.client.plugins.HttpTimeout
import io.ktor.client.request.*
import ug.qaat.coordinator.BuildConfig
import ug.qaat.coordinator.R
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

    /** Best-effort pings to wake the sleeping free-tier backend BEFORE the user submits, so the
     *  cold start overlaps with typing instead of stalling sign-in. Hits BOTH the gateway and the
     *  auth-service (the actual sleeper that login is proxied to). Failures are ignored. */
    suspend fun warmUp() {
        runCatching { client().get(BuildConfig.AUTH_WARM_URL) }   // the real sleeper (login upstream)
        runCatching { client().get("$baseUrl/health") }
    }

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
                // Strict validation first — system trust, then the embedded roots.
                try { sysTm.checkServerTrusted(chain, authType); return } catch (_: Exception) {}
                try { embTm.checkServerTrusted(chain, authType); return }
                catch (strict: Exception) {
                    // Last resort: validate the chain to a trusted anchor with the TIME CHECK
                    // NEUTRALIZED, so a wrong phone clock (the #1 field failure — phones with no SIM
                    // never NITZ-sync, so their date is wrong and a perfectly valid cert looks
                    // "not yet valid / expired") can no longer break sign-in. We STILL fully verify
                    // the signature chain to a trusted root here, and the hostname is verified
                    // separately, so this does NOT open MITM: an attacker cannot present a chain that
                    // validates to GTS/GlobalSign for this host. The only thing given up is detecting
                    // a genuinely expired/revoked server cert — the intended offline-first tradeoff.
                    dateNeutralServerTrusted(chain ?: throw strict, sysTm, embTm, strict)
                }
            }
            override fun getAcceptedIssuers(): Array<X509Certificate> = sysTm.acceptedIssuers + embTm.acceptedIssuers
        }
        val ssl = SSLContext.getInstance("TLS").apply { init(null, arrayOf<TrustManager>(composite), null) }
        ssl to composite
    }

    /** Turn a raw network exception into a calm, human message — no "Socket timeout has
     *  expired […socket_timeout=unknown] ms" jargon leaking to coordinators. */
    fun friendly(t: Throwable): String {
        val m = (t.message ?: "").lowercase()
        return when {
            "timeout" in m || "timed out" in m ->
                "The server is waking up (it sleeps when idle). Please wait a few seconds and tap again."
            "unable to resolve host" in m || "failed to connect" in m || "unreachable" in m || "connect" in m ->
                "Can't reach the server — check your internet connection and try again."
            // A wrong phone clock makes a perfectly valid certificate look expired / not-yet-valid.
            // This is the #1 cause of a "secure connection" failure on a freshly set-up phone, so
            // call it out explicitly instead of the misleading "update the app".
            "not yet valid" in m || "expired" in m || "not valid" in m || "current time" in m || "validity" in m ->
                "Secure connection failed — your phone's date & time look wrong. Set them to automatic (network) date/time, then try again."
            "trust anchor" in m || "certif" in m || "handshake" in m || "ssl" in m ->
                "Secure connection failed. First set your phone's date & time to automatic and try again; if it persists, update the app or check the server address in the login screen."
            else -> t.message?.takeIf { it.isNotBlank() && it.length < 120 } ?: "Something went wrong. Please try again."
        }
    }

    /** Validate [chain] to a trusted anchor (system OR embedded roots) with the validity-DATE check
     *  neutralized — we validate "as of" the presented cert's own notBefore, so whatever cert the
     *  server rotates to is always in-window regardless of the device clock. Signature-chain trust is
     *  still enforced; revocation is disabled so offline phones don't stall. Rethrows the original
     *  [cause] if the chain genuinely doesn't chain to a trusted root. */
    private fun dateNeutralServerTrusted(
        chain: Array<out X509Certificate>,
        sysTm: X509TrustManager,
        embTm: X509TrustManager,
        cause: Exception,
    ) {
        runCatching {
            val anchorCerts = (sysTm.acceptedIssuers.asList() + embTm.acceptedIssuers.asList()).toSet()
            // The CertPath must not contain a trust anchor itself, so drop any chain cert that IS an
            // anchor (e.g. a root the server also sent); anchors are supplied separately below.
            val pathCerts = chain.filter { it !in anchorCerts }.ifEmpty { chain.toList() }
            val certPath = CertificateFactory.getInstance("X.509").generateCertPath(pathCerts)
            val anchors = anchorCerts.map { TrustAnchor(it, null) }.toSet()
            val params = PKIXParameters(anchors).apply {
                isRevocationEnabled = false
                date = chain[0].notBefore   // "as of" the leaf's own start → clock-independent
            }
            CertPathValidator.getInstance("PKIX").validate(certPath, params)
        }.getOrElse { throw cause }
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
        // Generous timeouts: the cloud runs on a free tier that SLEEPS after ~15 min, so the
        // first request has to wait out a 30–50s cold start. Without this, Ktor/OkHttp's short
        // default timeouts throw "Socket timeout has expired" on login and leave attendance
        // uploads stuck on PENDING_SYNC. These ceilings let a cold start finish and succeed.
        install(HttpTimeout) {
            connectTimeoutMillis = 30_000
            socketTimeoutMillis = 60_000
            requestTimeoutMillis = 120_000
        }
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
