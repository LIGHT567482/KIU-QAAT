package ug.qaat.student.util

import android.annotation.SuppressLint
import android.content.Context
import android.provider.Settings
import java.security.MessageDigest

/**
 * A stable per-device fingerprint sent with every check-in. The coordinator's validator uses it
 * to enforce one-device-one-student per lecture (DEVICE_ALREADY_USED / DEVICE_MISMATCH). It only
 * needs to be STABLE for this install and unique per device — it never leaves the LAN in the clear
 * (it's a hash).
 */
object Fingerprint {
    @SuppressLint("HardwareIds")
    fun get(context: Context): String {
        val androidId = runCatching {
            Settings.Secure.getString(context.contentResolver, Settings.Secure.ANDROID_ID)
        }.getOrNull().orEmpty()
        return sha256Hex("qaat-student|$androidId|${android.os.Build.MODEL}")
    }

    private fun sha256Hex(s: String): String =
        MessageDigest.getInstance("SHA-256").digest(s.toByteArray()).joinToString("") { "%02x".format(it) }
}
