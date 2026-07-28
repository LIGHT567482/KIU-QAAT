package ug.qaat.student.net

import io.ktor.client.*
import io.ktor.client.engine.okhttp.*
import io.ktor.client.plugins.*
import ug.qaat.student.BuildConfig

/** HTTP clients. `client()` = the cloud (onboarding only, generous timeouts for free-tier
 *  cold-start). `lanClient()` = the coordinator's in-room server over the hotspot (fast, local). */
object Net {
    val baseUrl: String get() = BuildConfig.API_BASE

    fun client(): HttpClient = HttpClient(OkHttp) {
        expectSuccess = false
        install(HttpTimeout) {
            requestTimeoutMillis = 60_000
            connectTimeoutMillis = 30_000
            socketTimeoutMillis = 60_000
        }
    }

    fun lanClient(): HttpClient = HttpClient(OkHttp) {
        expectSuccess = false
        install(HttpTimeout) {
            requestTimeoutMillis = 6_000
            connectTimeoutMillis = 3_000
            socketTimeoutMillis = 6_000
        }
    }
}
