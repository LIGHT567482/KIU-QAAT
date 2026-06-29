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
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.unit.dp
import ug.qaat.coordinator.net.BrandingClient

/** Parse "#RRGGBB" → Color, or null. */
private fun parseHex(hex: String): Color? = runCatching {
    if (!hex.startsWith("#")) return null
    Color(android.graphics.Color.parseColor(hex))
}.getOrNull()

/** A Material3 colour scheme tinted by the tenant's brand colour (falls back to default blue). */
@Composable
fun brandedColorScheme(branding: BrandingClient.Branding?): ColorScheme {
    val base = lightColorScheme()
    val brand = branding?.brandColor?.let { parseHex(it) } ?: return base
    return base.copy(primary = brand, secondary = brand)
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
                Text((branding?.name ?: "Q").take(1).uppercase(), color = MaterialTheme.colorScheme.onPrimary)
            }
        }
    }
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

/** Logo + institution name, for the app bar. */
@Composable
fun BrandHeader(branding: BrandingClient.Branding?) {
    Row(verticalAlignment = Alignment.CenterVertically) {
        BrandLogo(branding)
        Spacer(Modifier.width(8.dp))
        Column {
            Text(branding?.name ?: "QAAT", style = MaterialTheme.typography.titleMedium)
            branding?.motto?.takeIf { it.isNotBlank() }?.let {
                Text(it, style = MaterialTheme.typography.labelSmall)
            }
        }
    }
}
