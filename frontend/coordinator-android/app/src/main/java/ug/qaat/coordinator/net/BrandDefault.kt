package ug.qaat.coordinator.net

import android.content.Context
import org.json.JSONObject

/**
 * The institution's bundled branding (assets/brand.json) — the instant, offline default shown
 * before any backend override. brand.json (repo root) is the single source of truth, synced here
 * by scripts/sync-brand.sh. Same shape the backend serves, so bundled == fetched.
 */
object BrandDefault {
    fun load(context: Context): BrandingClient.Branding? = runCatching {
        val raw = context.assets.open("brand.json").bufferedReader().use { it.readText() }
        val j = JSONObject(raw)
        BrandingClient.Branding(
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
    }.getOrNull()
}
