package ug.qaat.coordinator.store

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import org.json.JSONArray
import org.json.JSONObject
import ug.qaat.coordinator.db.AppDao
import ug.qaat.coordinator.db.RosterEntity
import ug.qaat.coordinator.net.BrandingClient
import ug.qaat.coordinator.net.ManifestClient
import ug.qaat.coordinator.ui.AppState

/**
 * Encrypted-at-rest persistence (Android Keystore) of everything the coordinator needs to
 * (a) reopen the app WITHOUT logging in again and (b) take attendance FULLY OFFLINE, even
 * on a different network from the server:
 *   • the session (token, ids, device-binding key, name/role) — restored on launch;
 *   • the sign-in credentials — so an expired token is silently refreshed in the background;
 *   • the daily manifest essentials (RSA public key, student-hash key, units) — the roster
 *     itself is already cached in Room, so validation runs with no server.
 */
object SessionStore {
    private lateinit var prefs: SharedPreferences

    fun init(context: Context) {
        if (::prefs.isInitialized) return
        val key = MasterKey.Builder(context).setKeyScheme(MasterKey.KeyScheme.AES256_GCM).build()
        prefs = EncryptedSharedPreferences.create(
            context, "qaat_session", key,
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
        )
    }

    // ── Session + credentials ────────────────────────────────────────────────
    fun saveSession(token: String, userId: String, tenantId: String, bindingKey: String?,
                    name: String, email: String, role: String, title: String = "", regNo: String = "") {
        prefs.edit().apply {
            putString("token", token); putString("userId", userId); putString("tenantId", tenantId)
            putString("bindingKey", bindingKey); putString("name", name)
            putString("email", email); putString("role", role)
            putString("title", title); putString("regNo", regNo)
        }.apply()
    }

    /** Stored so an expired token can be refreshed without bothering the coordinator. */
    fun saveCredentials(email: String, password: String) {
        prefs.edit().putString("cred_email", email).putString("cred_pw", password).apply()
    }
    fun credentials(): Pair<String, String>? {
        val e = prefs.getString("cred_email", null) ?: return null
        val p = prefs.getString("cred_pw", null) ?: return null
        return e to p
    }

    val hasSession: Boolean get() = ::prefs.isInitialized && prefs.getString("token", null) != null

    // Light/dark preference — persisted so the chosen theme survives restarts.
    fun saveTheme(dark: Boolean) { if (::prefs.isInitialized) prefs.edit().putBoolean("dark", dark).apply() }
    fun restoreTheme() { if (::prefs.isInitialized) AppState.darkTheme = prefs.getBoolean("dark", false) }

    // Backend URL override (so the coordinator can point the SAME build at the local server
    // for testing or the cloud domain for production, without rebuilding).
    fun saveServerUrl(url: String?) { if (::prefs.isInitialized) prefs.edit().putString("server_url", url?.trim()).apply() }
    fun serverUrl(): String? = if (::prefs.isInitialized) prefs.getString("server_url", null) else null

    /** Restore the session SCALARS into AppState (safe on the main thread — no DB). The
     *  cached manifest is hydrated separately (loadManifest) off the main thread.
     *  @return true if a session was restored. */
    fun restore(): Boolean {
        if (!::prefs.isInitialized) return false
        val token = prefs.getString("token", null) ?: return false
        AppState.token = token
        AppState.userId = prefs.getString("userId", null)
        AppState.tenantId = prefs.getString("tenantId", null)
        AppState.deviceBindingKey = prefs.getString("bindingKey", null)
        AppState.coordinatorName = prefs.getString("name", null)
        AppState.coordinatorTitle = prefs.getString("title", null)
        AppState.coordinatorRegNo = prefs.getString("regNo", null)
        AppState.coordinatorEmail = prefs.getString("email", null)
        AppState.role = prefs.getString("role", null)
        AppState.branding = restoreBranding()   // tenant identity shows offline too
        return true
    }

    // ── Tenant branding (so the app shows the same identity as the dashboards, even
    //    after auto-login / offline) ───────────────────────────────────────────────
    fun saveBranding(b: BrandingClient.Branding) {
        prefs.edit().apply {
            putString("b_name", b.name); putString("b_motto", b.motto); putString("b_logo", b.logoUrl)
            putString("b_brand", b.brandColor); putString("b_bg", b.backgroundColor)
            putString("b_sidebar", b.sidebarColor); putString("b_footer", b.footerColor)
            putString("b_textlight", b.textColorLight); putString("b_textdark", b.textColorDark)
        }.apply()
    }
    private fun restoreBranding(): BrandingClient.Branding? {
        val name = prefs.getString("b_name", null) ?: return null
        return BrandingClient.Branding(
            name = name,
            motto = prefs.getString("b_motto", "") ?: "",
            logoUrl = prefs.getString("b_logo", "") ?: "",
            brandColor = prefs.getString("b_brand", "") ?: "",
            backgroundColor = prefs.getString("b_bg", "") ?: "",
            sidebarColor = prefs.getString("b_sidebar", "") ?: "",
            footerColor = prefs.getString("b_footer", "") ?: "",
            textColorLight = prefs.getString("b_textlight", "") ?: "",
            textColorDark = prefs.getString("b_textdark", "") ?: "",
        )
    }

    fun clear() { if (::prefs.isInitialized) prefs.edit().clear().apply() }

    // ── Cached daily manifest (roster lives in Room; meta lives here) ─────────
    fun saveManifest(m: ManifestClient.Parsed) {
        val units = JSONArray().apply {
            m.units.forEach {
                put(JSONObject().put("unit_id", it.unitId).put("unit_name", it.unitName)
                    .put("lecturer_staff_id", it.lecturerStaffId).put("lecturer_name", it.lecturerName)
                    .put("lecturer_phone", it.lecturerPhone))
            }
        }
        prefs.edit().apply {
            putString("m_year", m.academicYear)
            putString("m_pubkey", m.institutionPublicKeyPem)
            putString("m_hashkey", m.studentHashKey)
            putString("m_units", units.toString())
            putLong("m_at", System.currentTimeMillis())
        }.apply()
    }

    fun manifestDownloadedAt(): Long = if (::prefs.isInitialized) prefs.getLong("m_at", 0L) else 0L

    /** Rebuild the cached manifest from stored meta + the Room-cached roster. Call OFF the
     *  main thread (it reads Room). Enables opening a session with no server connection. */
    fun loadManifest(dao: AppDao): ManifestClient.Parsed? {
        val pubkey = prefs.getString("m_pubkey", null) ?: return null
        val units = ArrayList<ManifestClient.UnitInfo>()
        val roster = LinkedHashMap<String, List<RosterEntity>>()
        runCatching {
            val arr = JSONArray(prefs.getString("m_units", "[]"))
            for (i in 0 until arr.length()) {
                val u = arr.getJSONObject(i)
                val id = u.getString("unit_id")
                units.add(ManifestClient.UnitInfo(id, u.optString("unit_name", id),
                    u.optString("lecturer_staff_id", ""), u.optString("lecturer_name", ""), u.optString("lecturer_phone", "")))
                roster[id] = dao.roster(id)   // roster cached in Room survives offline
            }
        }
        return ManifestClient.Parsed(
            academicYear = prefs.getString("m_year", "") ?: "",
            institutionPublicKeyPem = pubkey,
            studentHashKey = prefs.getString("m_hashkey", "") ?: "",
            units = units,
            rosterByUnit = roster,
        )
    }
}
