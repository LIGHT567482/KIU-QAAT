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
        prefs = runCatching { create(context) }.getOrElse { first ->
            // The encrypted keyset can end up unreadable (e.g. after a reinstall or a keystore
            // change), which would otherwise SILENTLY break auto-login + offline caching. Wipe the
            // corrupt store and recreate it so persistence keeps working.
            android.util.Log.w("QAAT_STORE", "encrypted prefs unreadable — recreating", first)
            runCatching {
                context.getSharedPreferences("qaat_session", Context.MODE_PRIVATE).edit().clear().commit()
                context.deleteSharedPreferences("qaat_session")
            }
            runCatching { create(context) }.getOrElse { second ->
                // Some devices can't back EncryptedSharedPreferences at all — most commonly when the
                // phone's CLOCK IS WRONG: the AndroidKeyStore key's certificate is "not valid yet",
                // so key creation throws. That is the SAME wrong-clock state that breaks TLS on
                // SIM-less phones. Rather than let this crash onCreate (app installs but "won't
                // run"), fall back to PLAIN prefs so the app still opens and works. At-rest
                // encryption is lost until the clock is corrected and the app reopened.
                android.util.Log.e("QAAT_STORE", "EncryptedSharedPreferences unavailable — using plain prefs", second)
                context.getSharedPreferences("qaat_session_plain", Context.MODE_PRIVATE)
            }
        }
    }

    private fun create(context: Context): SharedPreferences {
        val key = MasterKey.Builder(context).setKeyScheme(MasterKey.KeyScheme.AES256_GCM).build()
        return EncryptedSharedPreferences.create(
            context, "qaat_session", key,
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
        )
    }

    // ── Session + credentials ────────────────────────────────────────────────
    fun saveSession(token: String, userId: String, tenantId: String, bindingKey: String?,
                    name: String, email: String, role: String, title: String = "", regNo: String = "",
                    studentId: String = "", staffId: String = "", org: String = "") {
        prefs.edit().apply {
            putString("token", token); putString("userId", userId); putString("tenantId", tenantId)
            putString("bindingKey", bindingKey); putString("name", name)
            putString("email", email); putString("role", role)
            putString("title", title); putString("regNo", regNo)
            putString("studentId", studentId); putString("staffId", staffId); putString("org", org)
        }.apply()
    }

    /** Credentials for a silent re-login in the unified app (identifier + password + institution). */
    fun saveAppCredentials(identifier: String, password: String, org: String) {
        prefs.edit().putString("cred_id", identifier).putString("cred_pw", password).putString("cred_org", org).apply()
    }
    fun appCredentials(): Triple<String, String, String>? {
        val i = prefs.getString("cred_id", null) ?: return null
        val p = prefs.getString("cred_pw", null) ?: return null
        return Triple(i, p, prefs.getString("cred_org", "") ?: "")
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

    /** Student attendance-cooldown deadline (epoch millis) after a device switch. */
    fun saveAttendBlockUntil(ms: Long) { if (::prefs.isInitialized) prefs.edit().putLong("block_until", ms).apply() }
    fun attendBlockUntil(): Long = if (::prefs.isInitialized) prefs.getLong("block_until", 0L) else 0L

    // Sessions this device has already attended TODAY (one-attendance-per-session). Stored as a set
    // of session ids stamped with today's date; the whole set resets when the calendar day rolls over
    // (a new day = a fresh shift), so the student can attend the next day's sessions normally.
    private fun today(): String = java.time.LocalDate.now().toString()
    private fun attendedSet(): MutableSet<String> {
        if (!::prefs.isInitialized) return mutableSetOf()
        if (prefs.getString("attended_day", "") != today()) return mutableSetOf()   // stale → empty
        return prefs.getStringSet("attended_sessions", emptySet())!!.toMutableSet()
    }
    fun markAttended(sessionId: String) {
        if (!::prefs.isInitialized || sessionId.isBlank()) return
        val set = attendedSet().apply { add(sessionId) }
        prefs.edit().putString("attended_day", today()).putStringSet("attended_sessions", set).apply()
    }
    fun hasAttended(sessionId: String): Boolean =
        sessionId.isNotBlank() && attendedSet().contains(sessionId)

    // Light/dark preference — persisted so the chosen theme survives restarts.
    fun saveTheme(dark: Boolean) { if (::prefs.isInitialized) prefs.edit().putBoolean("dark", dark).apply() }
    fun restoreTheme() { if (::prefs.isInitialized) AppState.darkTheme = prefs.getBoolean("dark", false) }

    // Backend URL override (so the coordinator can point the SAME build at the local server
    // for testing or the cloud domain for production, without rebuilding).
    fun saveServerUrl(url: String?) { if (::prefs.isInitialized) prefs.edit().putString("server_url", url?.trim()).apply() }
    fun serverUrl(): String? = if (::prefs.isInitialized) prefs.getString("server_url", null) else null

    // Manual hotspot gateway IP (for phones that hide the hotspot interface from apps).
    fun saveHotspotIp(ip: String?) { if (::prefs.isInitialized) prefs.edit().putString("hotspot_ip", ip?.trim()?.takeIf { it.isNotBlank() }).apply() }
    fun hotspotIp(): String? = if (::prefs.isInitialized) prefs.getString("hotspot_ip", null) else null

    // System-hotspot mode toggle + the cohort-named SSID/password the coordinator configured, so
    // the choice and the join QR survive an app restart (they set the same hotspot each day).
    fun saveSystemHotspot(enabled: Boolean, ssid: String?, pass: String?) {
        if (!::prefs.isInitialized) return
        prefs.edit().putBoolean("sys_hotspot", enabled)
            .putString("sys_ssid", ssid?.trim()?.takeIf { it.isNotBlank() })
            .putString("sys_pass", pass?.takeIf { it.isNotBlank() }).apply()
    }
    fun restoreSystemHotspot() {
        if (!::prefs.isInitialized) return
        AppState.useSystemHotspot = prefs.getBoolean("sys_hotspot", false)
        AppState.systemHotspotSsid = prefs.getString("sys_ssid", null)
        AppState.systemHotspotPass = prefs.getString("sys_pass", null)
    }

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
        AppState.studentId = prefs.getString("studentId", null)
        AppState.staffId = prefs.getString("staffId", null)
        AppState.org = prefs.getString("org", null)
        AppState.branding = restoreBranding()   // tenant identity shows offline too
        restoreSystemHotspot()                  // hotspot-mode choice + cohort SSID/password
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
                    .put("lecturer_phone", it.lecturerPhone).put("duration_minutes", it.durationMinutes)
                    .put("day_of_week", it.dayOfWeek).put("start_time", it.startTime).put("venue_id", it.venueId)
                    .put("session_code", it.sessionCode))
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
                    u.optString("lecturer_staff_id", ""), u.optString("lecturer_name", ""), u.optString("lecturer_phone", ""),
                    u.optInt("duration_minutes", 0),
                    u.optInt("day_of_week", 0), u.optString("start_time", ""), u.optString("venue_id", ""),
                    u.optString("session_code", "")))
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
