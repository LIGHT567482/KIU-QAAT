package ug.qaat.crypto

import java.security.MessageDigest
import java.security.SecureRandom
import java.util.Base64
import javax.crypto.Cipher
import javax.crypto.Mac
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

/**
 * VaultCrypto — the on-device port of apps/coordinator-pwa/src/crypto/vault-crypto.ts,
 * kept byte-for-byte compatible with the Go sync-receiver
 * (services/sync-receiver/internal/crypto/vault.go).
 *
 * Device key = HKDF-SHA256(secret = server-issued binding key (UTF-8),
 *   salt = "QAAT-IndexedDB-Salt-v1", info = "coordinator-vault-key" | "coordinator-vault-hmac").
 * Payloads are stored/transmitted as base64(iv(12) || ciphertext || tag) and
 * authenticated with HMAC-SHA256 over that base64 text.
 *
 * Pure JVM/JCA — no Android dependencies, so it runs in unit tests here and
 * drops unchanged into the Android app.
 */
object VaultCrypto {
    private val SALT = "QAAT-IndexedDB-Salt-v1".toByteArray(Charsets.UTF_8)
    private val INFO_AES = "coordinator-vault-key".toByteArray(Charsets.UTF_8)
    private val INFO_HMAC = "coordinator-vault-hmac".toByteArray(Charsets.UTF_8)
    private const val GCM_TAG_BITS = 128
    private const val IV_LEN = 12
    private val rng = SecureRandom()

    data class VaultKeys(val aesKey: ByteArray, val hmacKey: ByteArray)

    /** Derive the AES-256-GCM and HMAC-SHA256 keys from the server-issued binding secret. */
    fun deriveKeys(bindingKey: String): VaultKeys {
        val secret = bindingKey.toByteArray(Charsets.UTF_8)
        return VaultKeys(
            aesKey = hkdfSha256(secret, SALT, INFO_AES, 32),
            hmacKey = hkdfSha256(secret, SALT, INFO_HMAC, 32),
        )
    }

    /** AES-256-GCM encrypt → base64(iv || ciphertext || tag). Matches WebCrypto + Go. */
    fun encrypt(aesKey: ByteArray, plaintext: String): String {
        val iv = ByteArray(IV_LEN).also { rng.nextBytes(it) }
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.ENCRYPT_MODE, SecretKeySpec(aesKey, "AES"), GCMParameterSpec(GCM_TAG_BITS, iv))
        val ctAndTag = cipher.doFinal(plaintext.toByteArray(Charsets.UTF_8))
        val combined = ByteArray(iv.size + ctAndTag.size)
        System.arraycopy(iv, 0, combined, 0, iv.size)
        System.arraycopy(ctAndTag, 0, combined, iv.size, ctAndTag.size)
        return Base64.getEncoder().encodeToString(combined)
    }

    /** AES-256-GCM decrypt of base64(iv || ciphertext || tag). */
    fun decrypt(aesKey: ByteArray, b64: String): String {
        val combined = Base64.getDecoder().decode(b64)
        val iv = combined.copyOfRange(0, IV_LEN)
        val ctAndTag = combined.copyOfRange(IV_LEN, combined.size)
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.DECRYPT_MODE, SecretKeySpec(aesKey, "AES"), GCMParameterSpec(GCM_TAG_BITS, iv))
        return String(cipher.doFinal(ctAndTag), Charsets.UTF_8)
    }

    /** HMAC-SHA256(key, data-as-UTF8) → lowercase hex. */
    fun hmacSign(hmacKey: ByteArray, data: String): String =
        toHex(hmacRaw(hmacKey, data.toByteArray(Charsets.UTF_8)))

    /** SHA-256(data-as-UTF8) → lowercase hex. */
    fun sha256(data: String): String =
        toHex(MessageDigest.getInstance("SHA-256").digest(data.toByteArray(Charsets.UTF_8)))

    /**
     * Keyed student-id hash: HMAC-SHA256(keyStr, message) → lowercase hex.
     * Must match the per-tenant hash used by the gateway + the manifest roster.
     */
    fun hmacHex(keyStr: String, message: String): String =
        toHex(hmacRaw(keyStr.toByteArray(Charsets.UTF_8), message.toByteArray(Charsets.UTF_8)))

    // ── internals ──────────────────────────────────────────────────────────────
    private fun hmacRaw(key: ByteArray, data: ByteArray): ByteArray {
        val mac = Mac.getInstance("HmacSHA256")
        mac.init(SecretKeySpec(key, "HmacSHA256"))
        return mac.doFinal(data)
    }

    /** RFC 5869 HKDF-SHA256 (extract + expand), matching WebCrypto + Go x/crypto/hkdf. */
    private fun hkdfSha256(ikm: ByteArray, salt: ByteArray, info: ByteArray, length: Int): ByteArray {
        val prk = hmacRaw(salt, ikm)                       // extract
        val out = ByteArray(length)
        var t = ByteArray(0)
        var pos = 0
        var counter = 1
        while (pos < length) {
            val mac = Mac.getInstance("HmacSHA256")
            mac.init(SecretKeySpec(prk, "HmacSHA256"))
            mac.update(t)
            mac.update(info)
            mac.update(counter.toByte())
            t = mac.doFinal()
            val n = minOf(t.size, length - pos)
            System.arraycopy(t, 0, out, pos, n)
            pos += n
            counter++
        }
        return out
    }

    private fun toHex(b: ByteArray): String {
        val sb = StringBuilder(b.size * 2)
        for (x in b) sb.append("%02x".format(x))
        return sb.toString()
    }
}
