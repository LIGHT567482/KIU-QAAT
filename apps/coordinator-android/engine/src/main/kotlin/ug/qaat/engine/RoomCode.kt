package ug.qaat.engine

import javax.crypto.Mac
import javax.crypto.spec.SecretKeySpec

/**
 * RoomCode — HMAC-SHA256 TOTP (RFC 6238/4226 style) port of
 * services/api-gateway/internal/checkin/roomcode.go. The native coordinator app
 * now GENERATES the displayed code and VALIDATES the submitted one, so both must
 * match the server's algorithm exactly (10s step, 6 digits, ±1 step window).
 */
object RoomCode {
    const val STEP_SECONDS = 10L
    const val DIGITS = 6
    private const val SKEW_STEPS = 1L
    private const val MOD = 1_000_000L // 10^DIGITS

    /** The code for a per-session secret at the given epoch-seconds. */
    fun derive(secret: ByteArray, epochSeconds: Long): String =
        deriveStep(secret, epochSeconds / STEP_SECONDS)

    /** Accepts the code within ±1 step of now (constant-time compare). */
    fun validate(secret: ByteArray, code: String, epochSeconds: Long): Boolean {
        if (code.length != DIGITS) return false
        val current = epochSeconds / STEP_SECONDS
        var ok = false
        for (d in -SKEW_STEPS..SKEW_STEPS) {
            if (constantTimeEquals(deriveStep(secret, current + d), code)) ok = true
        }
        return ok
    }

    /** Seconds until the current code rotates (for the coordinator's countdown). */
    fun secondsRemaining(epochSeconds: Long): Int = (STEP_SECONDS - epochSeconds % STEP_SECONDS).toInt()

    private fun deriveStep(secret: ByteArray, step: Long): String {
        val msg = ByteArray(8)
        var s = step
        for (i in 7 downTo 0) { msg[i] = (s and 0xff).toByte(); s = s ushr 8 } // big-endian uint64
        val mac = Mac.getInstance("HmacSHA256")
        mac.init(SecretKeySpec(secret, "HmacSHA256"))
        val sum = mac.doFinal(msg)
        val offset = (sum[sum.size - 1].toInt() and 0x0f)
        val bin = ((sum[offset].toInt() and 0x7f) shl 24) or
            ((sum[offset + 1].toInt() and 0xff) shl 16) or
            ((sum[offset + 2].toInt() and 0xff) shl 8) or
            (sum[offset + 3].toInt() and 0xff)
        val code = (bin.toLong() and 0xffffffffL) % MOD
        return code.toString().padStart(DIGITS, '0')
    }

    private fun constantTimeEquals(a: String, b: String): Boolean {
        if (a.length != b.length) return false
        var r = 0
        for (i in a.indices) r = r or (a[i].code xor b[i].code)
        return r == 0
    }
}
