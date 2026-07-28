package ug.qaat.coordinator.student

import io.ktor.client.*
import io.ktor.client.engine.okhttp.*
import io.ktor.client.plugins.HttpTimeout

/** Short-timeout client for in-room LAN calls (discovery probe + check-in) — fast and local.
 *  Cloud calls (register-device, progress) reuse the app's shared [ug.qaat.coordinator.net.Net]. */
internal object StudentNet {
    fun lanClient(): HttpClient = HttpClient(OkHttp) {
        expectSuccess = false
        install(HttpTimeout) {
            requestTimeoutMillis = 6_000
            connectTimeoutMillis = 3_000
            socketTimeoutMillis = 6_000
        }
    }
}
