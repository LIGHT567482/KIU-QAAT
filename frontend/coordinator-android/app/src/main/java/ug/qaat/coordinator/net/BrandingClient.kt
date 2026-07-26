package ug.qaat.coordinator.net

import io.ktor.client.request.*
import io.ktor.client.statement.*
import org.json.JSONObject

/**
 * Fetches the authenticated tenant's branding (GET /api/v1/branding) so the app can
 * adopt the institution's logo + colours AFTER login — one shared app icon, tenant
 * identity inside. Mirrors the fields the dashboards + captive pages already use.
 */
class BrandingClient(private val token: String) {
    private val http = Net.client()
    private val base = Net.baseUrl

    data class Branding(
        val name: String,
        val motto: String,
        val logoUrl: String,        // https URL or a data: base64 image
        val brandColor: String,     // "#RRGGBB"
        val backgroundColor: String,
        val sidebarColor: String,   // the admin sidebar colour — used for the app nav + header
        val footerColor: String,
        val textColorLight: String, // per-theme text colour set by the super-admin
        val textColorDark: String,
    )

    suspend fun fetch(): Branding? {
        val r = http.get("$base/api/v1/branding") { header("Authorization", "Bearer $token") }
        if (r.status.value !in 200..299) return null
        val j = JSONObject(r.bodyAsText())
        return Branding(
            name = j.optString("name", "QAAT"),
            motto = j.optString("motto", j.optString("slogan", "")),
            logoUrl = j.optString("logo_url", ""),
            brandColor = j.optString("brand_color", ""),
            backgroundColor = j.optString("background_color", ""),
            sidebarColor = j.optString("sidebar_color", ""),
            footerColor = j.optString("footer_color", ""),
            textColorLight = j.optString("text_color_light", ""),
            textColorDark = j.optString("text_color_dark", ""),
        )
    }
}
