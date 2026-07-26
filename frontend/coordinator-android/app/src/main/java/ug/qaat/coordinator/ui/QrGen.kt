package ug.qaat.coordinator.ui

import android.graphics.Bitmap
import android.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import com.google.zxing.BarcodeFormat
import com.google.zxing.EncodeHintType
import com.google.zxing.qrcode.QRCodeWriter

/** Renders any string as a QR code bitmap (ZXing). Used for the Wi-Fi join QR, the
 *  student check-in page URL, and the lecturer gate URL shown in the in-room screen. */
fun qrImageBitmap(content: String, size: Int = 480): ImageBitmap? = runCatching {
    val hints = mapOf(EncodeHintType.MARGIN to 1)
    val matrix = QRCodeWriter().encode(content, BarcodeFormat.QR_CODE, size, size, hints)
    val bmp = Bitmap.createBitmap(size, size, Bitmap.Config.RGB_565)
    for (x in 0 until size) for (y in 0 until size) {
        bmp.setPixel(x, y, if (matrix.get(x, y)) Color.BLACK else Color.WHITE)
    }
    bmp.asImageBitmap()
}.getOrNull()

/** Standard Wi-Fi provisioning string a phone camera recognises to join a network. */
fun wifiQrPayload(ssid: String, pass: String): String =
    "WIFI:S:${ssid.replace(";", "\\;")};T:WPA;P:${pass.replace(";", "\\;")};;"
