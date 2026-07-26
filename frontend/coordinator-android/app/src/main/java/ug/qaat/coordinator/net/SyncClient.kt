package ug.qaat.coordinator.net

import io.ktor.client.request.*
import io.ktor.client.statement.*
import io.ktor.http.*
import ug.qaat.crypto.Sealer

/**
 * Seals a closed session and uploads it to the central backend via the SAME chunked
 * protocol the PWA uses (POST /api/v1/sync/{init,chunk,complete}). The sealed package
 * format is the VERIFIED `crypto-core` Sealer — the live sync-receiver already accepts it.
 * Uses the shared pinned client (Net.client) so TLS to the QAAT backend works everywhere.
 */
class SyncClient(private val baseUrl: String, private val bearer: String) {
    private val http = Net.client()
    private val chunkSize = Sealer.CHUNK_SIZE

    /** @param bindingKey the device binding secret (from login); @param plaintextJson the session package JSON. */
    suspend fun upload(coordinatorId: String, sessionId: String, bindingKey: String, plaintextJson: String): Boolean {
        val pkg = Sealer.seal(bindingKey, plaintextJson)

        val initBody = """{"coordinator_id":"$coordinatorId","session_ids":["$sessionId"],""" +
            """"total_chunks":${pkg.totalChunks},"package_checksum":"${pkg.packageChecksum}","package_hmac":"${pkg.hmac}"}"""
        val init = http.post("$baseUrl/api/v1/sync/init") {
            header("Authorization", "Bearer $bearer"); contentType(ContentType.Application.Json); setBody(initBody)
        }
        if (!init.status.isSuccess()) return false
        val uploadId = Regex("\"upload_id\"\\s*:\\s*\"([^\"]+)\"").find(init.bodyAsText())?.groupValues?.get(1) ?: return false

        val payload = pkg.encryptedPayload
        for (i in 0 until pkg.totalChunks) {
            val chunk = payload.substring(i * chunkSize, minOf((i + 1) * chunkSize, payload.length))
            val res = http.post("$baseUrl/api/v1/sync/chunk/$uploadId/$i") {
                header("Authorization", "Bearer $bearer")
                contentType(ContentType.Application.OctetStream)
                setBody(chunk.toByteArray(Charsets.UTF_8))
            }
            if (!res.status.isSuccess()) return false
        }
        return http.post("$baseUrl/api/v1/sync/complete/$uploadId") {
            header("Authorization", "Bearer $bearer")
        }.status.isSuccess()
    }

    fun close() = http.close()
}
