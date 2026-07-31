package ug.qaat.coordinator.ui

import android.annotation.SuppressLint
import android.content.Intent
import android.net.Uri
import android.view.View
import android.webkit.WebChromeClient
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView

/** The KIU student self-service portal. Hard-coded destination. */
const val STUDENT_PORTAL_URL = "https://student.kiu.ac.ug/"

/**
 * Opens the KIU student portal INSIDE the app (a WebView), and auto-fills the signed-in
 * student's registration number into the portal's "Email or Registration number" box
 * (input name = emailRegistrationNo) as soon as each page finishes loading — so the student
 * doesn't have to type or remember it. A "browser" action hands the same page off to the
 * phone's external browser as a fallback (copying the reg number to the clipboard first,
 * since we can't auto-fill a page we don't host).
 */
@SuppressLint("SetJavaScriptEnabled")
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun StudentPortalScreen(regNo: String?, onClose: () -> Unit) {
    val ctx = LocalContext.current
    val navColor = navBarColor(AppState.branding)
    val onNav = navColor?.let { onNavColor(it) }

    Scaffold(
        topBar = {
            TopAppBar(
                colors = if (navColor != null) TopAppBarDefaults.topAppBarColors(
                    containerColor = navColor, titleContentColor = onNav!!,
                    navigationIconContentColor = onNav, actionIconContentColor = onNav,
                ) else TopAppBarDefaults.topAppBarColors(),
                navigationIcon = { IconButton(onClick = onClose) { Text("‹", fontSize = 26.sp) } },
                title = { Text("Student portal", fontWeight = FontWeight.Bold) },
                actions = {
                    TextButton(onClick = {
                        // Fallback: open in the phone's browser. Copy the reg number first so the
                        // student can paste it (an external page can't be auto-filled by us).
                        regNo?.takeIf { it.isNotBlank() }?.let { copyToClipboard(ctx, "Reg. no", it) }
                        runCatching { ctx.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(STUDENT_PORTAL_URL))) }
                    }) { Text("Browser", color = onNav ?: MaterialTheme.colorScheme.primary) }
                },
            )
        },
    ) { pad ->
        var progress by remember { mutableStateOf(0) }
        Box(Modifier.padding(pad).fillMaxSize()) {
        AndroidView(
            modifier = Modifier.fillMaxSize(),
            factory = { c ->
                WebView(c).apply {
                    // Hardware-accelerated rendering + caching so the portal paints faster and
                    // repeat opens are near-instant (the external site is the slow part; we cache
                    // what we can and stop re-fetching unchanged assets).
                    setLayerType(View.LAYER_TYPE_HARDWARE, null)
                    settings.javaScriptEnabled = true
                    settings.domStorageEnabled = true
                    settings.databaseEnabled = true
                    settings.cacheMode = WebSettings.LOAD_DEFAULT
                    settings.loadsImagesAutomatically = true
                    settings.blockNetworkImage = false
                    settings.mixedContentMode = WebSettings.MIXED_CONTENT_COMPATIBILITY_MODE
                    @Suppress("DEPRECATION") settings.setRenderPriority(WebSettings.RenderPriority.HIGH)
                    // Drive the top progress bar so the load never looks frozen.
                    webChromeClient = object : WebChromeClient() {
                        override fun onProgressChanged(view: WebView?, newProgress: Int) { progress = newProgress }
                    }
                    webViewClient = object : WebViewClient() {
                        override fun onPageFinished(view: WebView?, url: String?) {
                            val reg = regNo?.takeIf { it.isNotBlank() } ?: return
                            val safe = reg.replace("\\", "\\\\").replace("'", "\\'")
                            // Fill the portal's reg/email login box if it's on the page.
                            view?.evaluateJavascript(
                                "(function(){var i=document.querySelector('input[name=\"emailRegistrationNo\"]')" +
                                    "||document.querySelector('input[type=\"text\"]');" +
                                    "if(i){i.value='$safe';" +
                                    "i.dispatchEvent(new Event('input',{bubbles:true}));" +
                                    "i.dispatchEvent(new Event('change',{bubbles:true}));}})();",
                                null,
                            )
                        }
                    }
                    loadUrl(STUDENT_PORTAL_URL)
                }
            },
        )
        if (progress in 1..99)
            LinearProgressIndicator(
                progress = { progress / 100f },
                modifier = Modifier.fillMaxWidth().align(Alignment.TopCenter),
            )
        }
    }
}

private fun copyToClipboard(ctx: android.content.Context, label: String, text: String) {
    val cm = ctx.getSystemService(android.content.Context.CLIPBOARD_SERVICE) as? android.content.ClipboardManager
    cm?.setPrimaryClip(android.content.ClipData.newPlainText(label, text))
}
