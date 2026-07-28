package ug.qaat.student.store

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey

/**
 * Encrypted-at-rest storage of the student's onboarding result: the registration number (the
 * identity we submit to the coordinator's /checkin) and the display name. Set once at onboarding
 * (which also binds this device to the reg server-side), then the app works fully offline.
 */
object StudentStore {
    private lateinit var prefs: SharedPreferences

    fun init(context: Context) {
        val ctx = context.applicationContext
        prefs = runCatching {
            val key = MasterKey.Builder(ctx).setKeyScheme(MasterKey.KeyScheme.AES256_GCM).build()
            EncryptedSharedPreferences.create(
                ctx, "qaat_student", key,
                EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
                EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
            )
        }.getOrElse {
            // Keystore unavailable (some older/rooted devices) — fall back to plain prefs so the
            // app still functions; the credential is a single-use signed QR, not a password.
            ctx.getSharedPreferences("qaat_student_plain", Context.MODE_PRIVATE)
        }
    }

    val onboarded: Boolean get() = !reg.isNullOrBlank()
    val reg: String? get() = prefs.getString("reg", null)
    val org: String? get() = prefs.getString("org", null)
    val fullName: String? get() = prefs.getString("name", null)
    /** Epoch millis until which attendance is paused after a device switch (0 = not paused). */
    val attendBlockUntil: Long get() = prefs.getLong("block_until", 0L)

    fun save(reg: String, org: String, fullName: String, attendBlockUntilMs: Long) {
        prefs.edit()
            .putString("reg", reg).putString("org", org).putString("name", fullName)
            .putLong("block_until", attendBlockUntilMs)
            .apply()
    }

    fun clear() { prefs.edit().clear().apply() }
}
