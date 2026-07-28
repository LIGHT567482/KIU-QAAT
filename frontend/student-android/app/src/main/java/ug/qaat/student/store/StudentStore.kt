package ug.qaat.student.store

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey

/**
 * Encrypted-at-rest storage of the student's onboarding result: the signed QR credential (what we
 * submit to the coordinator's /submit), the reg-number, and the display name. Once set, the app
 * works fully offline and never asks the student to log in again.
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

    val onboarded: Boolean get() = !credential.isNullOrBlank()
    val credential: String? get() = prefs.getString("qr", null)
    val reg: String? get() = prefs.getString("reg", null)
    val fullName: String? get() = prefs.getString("name", null)

    fun save(credential: String, reg: String, fullName: String) {
        prefs.edit().putString("qr", credential).putString("reg", reg).putString("name", fullName).apply()
    }

    fun clear() { prefs.edit().clear().apply() }
}
