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
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
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
        Surface(color = MaterialTheme.colorScheme.primary, shape = RoundedCornerShape(6.dp),
            modifier = Modifier.size(size.dp)) {
            Box(contentAlignment = Alignment.Center) {
                Text((branding?.name ?: "KIU").take(1).uppercase(), color = MaterialTheme.colorScheme.onPrimary)
            }
        }
    }
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

/** Logo + full institution name, for the app bar. The name uses a small size and wraps
 *  to two lines so the WHOLE tenant name stays visible (no truncation). */
@Composable
fun BrandHeader(branding: BrandingClient.Branding?) {
    Row(verticalAlignment = Alignment.CenterVertically) {
        BrandLogo(branding, size = 30)
        Spacer(Modifier.width(8.dp))
        Column {
            Text(
                branding?.name ?: "KIU QAAT",
                fontSize = 13.sp, fontWeight = FontWeight.Bold, lineHeight = 15.sp,
                maxLines = 2, overflow = TextOverflow.Ellipsis,
            )
            branding?.motto?.takeIf { it.isNotBlank() }?.let {
                Text(it, fontSize = 10.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
    }
}
