package ug.qaat.coordinator.ui

import android.annotation.SuppressLint
import android.content.Intent
import android.net.Uri
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
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
        AndroidView(
            modifier = Modifier.padding(pad).fillMaxSize(),
            factory = { c ->
                WebView(c).apply {
                    settings.javaScriptEnabled = true
                    settings.domStorageEnabled = true
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
    }
}

private fun copyToClipboard(ctx: android.content.Context, label: String, text: String) {
    val cm = ctx.getSystemService(android.content.Context.CLIPBOARD_SERVICE) as? android.content.ClipboardManager
    cm?.setPrimaryClip(android.content.ClipData.newPlainText(label, text))
}
