package ug.qaat.coordinator.ui

import android.graphics.BitmapFactory
import android.util.Base64
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.luminance
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import ug.qaat.coordinator.R
import ug.qaat.coordinator.net.BrandingClient

/** Parse "#RRGGBB" → Color, or null. */
private fun parseHex(hex: String): Color? = runCatching {
    if (!hex.startsWith("#")) return null
    Color(android.graphics.Color.parseColor(hex))
}.getOrNull()

/** The tenant's admin-sidebar colour — used to tint the app's top bar + bottom nav so the
 *  phone chrome matches the admin dashboard. Falls back to the brand colour. */
fun navBarColor(b: BrandingClient.Branding?): Color? =
    b?.sidebarColor?.let { parseHex(it) } ?: b?.brandColor?.let { parseHex(it) }

/** Readable content colour (white on dark chrome, near-black on light). */
fun onNavColor(bg: Color): Color =
    if (bg.luminance() > 0.55f) Color(0xFF0F172A) else Color.White

/** The tenant's page background colour (the admin dashboard's content background), used
 *  to paint the app's content area between the header and the bottom nav. */
fun appBackgroundColor(b: BrandingClient.Branding?): Color? = b?.backgroundColor?.let { parseHex(it) }

/** A Material3 colour scheme that inherits the tenant's brand + background colours (the
 *  same values the admin dashboards use), so the coordinator app looks like the tenant's.
 *  Honours the light/dark preference like the PWA's theme toggle. */
@Composable
fun brandedColorScheme(branding: BrandingClient.Branding?, dark: Boolean = false): ColorScheme {
    var s = if (dark) darkColorScheme() else lightColorScheme()
    branding?.brandColor?.let { parseHex(it) }?.let { s = s.copy(primary = it, secondary = it, tertiary = it) }
    // Tenant page background applies in light mode only (a tenant's light bg would be
    // unreadable in dark mode); dark mode keeps Material's dark surfaces.
    if (!dark) branding?.backgroundColor?.let { parseHex(it) }?.let { s = s.copy(background = it) }
    // Per-theme text colour the super-admin set — so text stays legible on the tenant's
    // background in each mode (applied to on-background / on-surface content).
    val textHex = if (dark) branding?.textColorDark else branding?.textColorLight
    textHex?.let { parseHex(it) }?.let { s = s.copy(onBackground = it, onSurface = it) }
    return s
}

/** Tenant logo: decodes a `data:` base64 image; else shows the brand initial.
 *  (For remote https logos add Coil's AsyncImage; most tenants store a base64 data-URL.) */
@Composable
fun BrandLogo(branding: BrandingClient.Branding?, size: Int = 32) {
    val url = branding?.logoUrl.orEmpty()
    val bmp = rememberDataUrlBitmap(url)
    if (bmp != null) {
        Image(bmp, contentDescription = branding?.name, contentScale = ContentScale.Fit,
            modifier = Modifier.size(size.dp).clip(RoundedCornerShape(6.dp)))
    } else {
        // The BUNDLED logo, not a coloured letter tile.
        //
        // This drew a box with "K" in it whenever the server's branding had not arrived — which
        // is every launch before the first fetch returns, every offline launch, and every launch
        // where the fetch fails. The app has shipped the real mark as a drawable all along and
        // used it only on the login screen, so the one screen that greeted you had the logo and
        // every screen after it had a letter. That is why the header still looked unchanged.
        Image(
            painter = painterResource(ug.qaat.coordinator.R.drawable.qaat_logo),
            contentDescription = branding?.name ?: "KIU QAAT",
            contentScale = ContentScale.Fit,
            modifier = Modifier.size(size.dp).clip(RoundedCornerShape(6.dp)),
        )
    }
}

/** Bottom-nav tab icon. Draws a flat vector silhouette in the bar's own content colour — solid
 *  white on the branded bars — rather than an emoji, which every OEM renders as its own small
 *  multicolour picture. See [NavIcons]. */
@Composable
fun TabGlyph(icon: ImageVector, label: String? = null) {
    Icon(icon, contentDescription = label, modifier = Modifier.size(22.dp))
}

/** Top-bar action icon: same silhouette treatment, forced to the bar's content colour. */
@Composable
fun BarIcon(icon: ImageVector, label: String?, tint: Color) {
    Icon(icon, contentDescription = label, tint = tint, modifier = Modifier.size(22.dp))
}

/** A faint, centered institution-logo watermark for EVERY app screen. It's a plain Image with
 *  no pointer handler, so it is not a hit target — touches pass straight through to the UI
 *  beneath it. Place it as the last child of a full-screen Box so it overlays all content. */
@Composable
fun BoxScope.BrandWatermark(branding: BrandingClient.Branding?) {
    val bmp = rememberDataUrlBitmap(branding?.logoUrl.orEmpty()) ?: return
    Image(
        bmp, contentDescription = null, contentScale = ContentScale.Fit, alpha = 0.05f,
        modifier = Modifier.align(Alignment.Center).fillMaxWidth(0.6f),
    )
}

@Composable
private fun rememberDataUrlBitmap(url: String): androidx.compose.ui.graphics.ImageBitmap? {
    if (!url.startsWith("data:")) return null
    return runCatching {
        val b64 = url.substringAfter(",", "")
        val bytes = Base64.decode(b64, Base64.DEFAULT)
        BitmapFactory.decodeByteArray(bytes, 0, bytes.size).asImageBitmap()
    }.getOrNull()
}

/**
 * The app bar's identity: the institution logo, then "KIU QAAT", then the institution name.
 *
 * ONE header for every role, because there were four. The coordinator and student bars showed the
 * logo with the tenant name; the lecturer and monitor bars showed the logo with "KIU QAAT"; and
 * the student portal showed the words "Student portal" with no logo at all. Four bars meant the
 * product was recognisable on some screens and anonymous on others, and the one screen a student
 * is sent to from outside the app was the anonymous one.
 *
 * The product name is stated even when branding has loaded, rather than only as a fallback — an
 * institution's own name on its own screen tells you whose it is, not what it is.
 *
 * [compact] drops the secondary line for bars that are already carrying a lot.
 */
@Composable
fun BrandHeader(branding: BrandingClient.Branding?, compact: Boolean = false) {
    Row(verticalAlignment = Alignment.CenterVertically) {
        BrandLogo(branding, size = 30)
        Spacer(Modifier.width(8.dp))
        Column {
            Text(
                "KIU QAAT",
                fontSize = 15.sp, fontWeight = FontWeight.Bold, lineHeight = 17.sp,
                maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
            // The institution beneath the product, wrapping to two lines so a long tenant
            // name stays whole rather than being cut off mid-word.
            if (!compact) branding?.name?.takeIf { it.isNotBlank() }?.let {
                Text(it, fontSize = 10.sp, lineHeight = 12.sp, maxLines = 2, overflow = TextOverflow.Ellipsis)
            }
        }
    }
}
