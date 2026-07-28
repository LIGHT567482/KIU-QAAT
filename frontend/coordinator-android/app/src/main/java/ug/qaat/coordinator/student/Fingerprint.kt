package ug.qaat.coordinator.student

import android.annotation.SuppressLint
import android.content.Context
import android.provider.Settings
import java.security.MessageDigest

/** Stable per-device fingerprint sent with every check-in + device registration. Must be STABLE
 *  for this install and unique per device (survives app-data-clear via ANDROID_ID). */
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
