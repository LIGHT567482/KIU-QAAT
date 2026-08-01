package ug.qaat.patroller.store

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey

/**
 * Encrypted-at-rest session store (mirrors the coordinator app's SessionStore, including the
 * plain-prefs fallback so a wrong-clock/Keystore device still runs). Holds the login token +
 * the patroller's identity and credentials for silent re-login.
 */
object Store {
    private lateinit var prefs: SharedPreferences

    fun init(context: Context) {
        if (::prefs.isInitialized) return
        prefs = runCatching { create(context) }.getOrElse {
            runCatching {
                context.getSharedPreferences("qaat_patrol", Context.MODE_PRIVATE).edit().clear().commit()
                context.deleteSharedPreferences("qaat_patrol")
            }
            runCatching { create(context) }.getOrElse {
                // Device can't back EncryptedSharedPreferences (e.g. wrong clock) — fall back to plain
                // prefs so the app still runs instead of crashing on launch.
                context.getSharedPreferences("qaat_patrol_plain", Context.MODE_PRIVATE)
            }
        }
    }

    private fun create(context: Context): SharedPreferences {
        val key = MasterKey.Builder(context).setKeyScheme(MasterKey.KeyScheme.AES256_GCM).build()
        return EncryptedSharedPreferences.create(
            context, "qaat_patrol", key,
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
        )
    }

    fun save(token: String, name: String, staffId: String, org: String, identifier: String, password: String) {
        if (!::prefs.isInitialized) return
        prefs.edit().putString("token", token).putString("name", name).putString("staff_id", staffId)
            .putString("org", org).putString("id", identifier).putString("pw", password).apply()
    }
    val token get() = if (::prefs.isInitialized) prefs.getString("token", null) else null
    val name get() = if (::prefs.isInitialized) prefs.getString("name", "") ?: "" else ""
    val staffId get() = if (::prefs.isInitialized) prefs.getString("staff_id", "") ?: "" else ""
    val org get() = if (::prefs.isInitialized) prefs.getString("org", "") ?: "" else ""
    fun creds(): Triple<String, String, String>? {
        val i = prefs.getString("id", null) ?: return null
        val p = prefs.getString("pw", null) ?: return null
        return Triple(i, p, prefs.getString("org", "") ?: "")
    }
    fun clear() { if (::prefs.isInitialized) prefs.edit().clear().apply() }
}
