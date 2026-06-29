package ug.qaat.crypto

import kotlin.math.ceil

/**
 * Sealer — the on-device port of apps/coordinator-pwa/src/sync/sealer.ts.
 * Seals a closed session into the exact package the Go sync-receiver expects:
 *   encryptedPayload = base64(iv || ciphertext || tag)   (AES-256-GCM, device key)
 *   hmac             = HMAC-SHA256(hmacKey, encryptedPayload-as-UTF8)  (hex)
 *   packageChecksum  = SHA-256(encryptedPayload-as-UTF8)              (hex)
 *   totalChunks      = ceil(encryptedPayload.length / 65536)
 */
object Sealer {
    const val CHUNK_SIZE = 65536 // must match the Sync Receiver

    data class SealedPackage(
        val encryptedPayload: String,
        val hmac: String,
        val packageChecksum: String,
        val totalChunks: Int,
    )

    /**
     * @param bindingKey   the server-issued device binding secret (hex string)
     * @param plaintextJson the JSON of { session, attendance_records, sealed_at, coordinator_id, package_version }
     */
    fun seal(bindingKey: String, plaintextJson: String): SealedPackage {
        val keys = VaultCrypto.deriveKeys(bindingKey)
        val encPayload = VaultCrypto.encrypt(keys.aesKey, plaintextJson)
        val hmac = VaultCrypto.hmacSign(keys.hmacKey, encPayload)
        val checksum = VaultCrypto.sha256(encPayload)
        val totalChunks = ceil(encPayload.length.toDouble() / CHUNK_SIZE).toInt()
        return SealedPackage(encPayload, hmac, checksum, totalChunks)
    }
}
