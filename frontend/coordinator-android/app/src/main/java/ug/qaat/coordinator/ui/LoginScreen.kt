package ug.qaat.coordinator.ui

import android.widget.Toast
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch
import ug.qaat.coordinator.R
import ug.qaat.coordinator.di.Graph
import ug.qaat.coordinator.net.AuthClient
import ug.qaat.coordinator.net.ManifestClient
import ug.qaat.coordinator.net.Net
import ug.qaat.coordinator.store.SessionStore

/** Unified KIU QAAT sign-in for every role (coordinator / student / lecturer). Identifier =
 *  email, registration number, or staff ID; the app routes by the returned role. */
@Composable
fun LoginScreen(onLoggedIn: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var identifier by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var totp by remember { mutableStateOf("") }
    var needsMfa by remember { mutableStateOf(false) }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }

    // Wake the free-tier backend the moment this screen shows, so its cold start (up to ~1 min)
    // overlaps with the user typing — turning a stalled sign-in into an instant one.
    LaunchedEffect(Unit) { Net.warmUp() }

    Box(Modifier.fillMaxSize()) {
    // Faint institution logo watermark behind everything.
    Image(
        painter = painterResource(R.drawable.qaat_logo),
        contentDescription = null,
        contentScale = ContentScale.Fit,
        modifier = Modifier.align(Alignment.Center).size(340.dp).alpha(0.05f),
    )
    Column(
        Modifier.fillMaxSize().padding(24.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        Image(
            painter = painterResource(R.drawable.qaat_logo),
            contentDescription = "KIU QAAT",
            modifier = Modifier.size(112.dp),
        )
        Spacer(Modifier.height(14.dp))
        Text("KIU QAAT", style = MaterialTheme.typography.headlineMedium)
        Text("Sign in — staff email, registration number, or staff ID",
            style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(top = 2.dp))
        Spacer(Modifier.height(24.dp))
        OutlinedTextField(identifier, { identifier = it.trim() }, label = { Text("Email / Reg. no / Staff ID") },
            singleLine = true, modifier = Modifier.fillMaxWidth())
        Spacer(Modifier.height(8.dp))
        PasswordField(password, { password = it }, "Password", modifier = Modifier.fillMaxWidth())
        if (needsMfa) {
            Spacer(Modifier.height(8.dp))
            OutlinedTextField(totp, { totp = it }, label = { Text("Authenticator code") }, singleLine = true, modifier = Modifier.fillMaxWidth())
        }
        error?.let { msg ->
            Spacer(Modifier.height(8.dp))
            Text(msg, color = MaterialTheme.colorScheme.error)
            // A "Secure connection failed" almost always means the phone's clock is wrong (phones with
            // no SIM never network-sync their time). Surface the exact fix inline.
            if (msg.contains("date", true) || msg.contains("time", true) || msg.contains("secure connection", true)) {
                Spacer(Modifier.height(6.dp))
                Surface(color = MaterialTheme.colorScheme.errorContainer, shape = MaterialTheme.shapes.small) {
                    Text("Fix: Settings → System → Date & time → turn ON “Set time automatically” (or set today’s correct date), then tap Sign in again. A wrong clock blocks the secure connection.",
                        Modifier.padding(10.dp), style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onErrorContainer)
                }
            }
        }

        Spacer(Modifier.height(16.dp))
        Button(
            enabled = !busy && identifier.isNotBlank() && password.isNotBlank(),
            modifier = Modifier.fillMaxWidth(),
            onClick = {
                busy = true; error = null
                scope.launch {
                    runCatching {
                        val res = AuthClient().appLogin(identifier.trim(), password, totp.ifBlank { null }) { needsMfa = true }
                            ?: return@runCatching   // MFA prompt shown
                        AppState.token = res.token; AppState.userId = res.userId
                        AppState.tenantId = res.tenantId; AppState.deviceBindingKey = res.deviceBindingKey
                        AppState.coordinatorName = res.fullName; AppState.coordinatorEmail = identifier.trim(); AppState.role = res.role
                        AppState.coordinatorTitle = res.title; AppState.coordinatorRegNo = res.registrationNo
                        AppState.studentId = res.studentId.ifBlank { null }
                        AppState.staffId = res.staffId.ifBlank { null }
                        AppState.forcePasswordChange = res.forcePasswordChange
                        // Persist the session IMMEDIATELY so auto-login survives a close.
                        SessionStore.saveSession(res.token, res.userId, res.tenantId, res.deviceBindingKey,
                            res.fullName, identifier.trim(), res.role, res.title, res.registrationNo,
                            res.studentId, res.staffId, "")
                        SessionStore.saveAppCredentials(identifier.trim(), password, "")
                        val greet = res.fullName.ifBlank { "back" }
                        Toast.makeText(ctx, "Welcome, $greet 👋", Toast.LENGTH_LONG).show()
                        onLoggedIn()
                        // Institution branding applies to every role.
                        runCatching {
                            ug.qaat.coordinator.net.BrandingClient(res.token).fetch()?.let {
                                AppState.branding = it; SessionStore.saveBranding(it)
                            }
                        }
                        // Only the COORDINATOR needs the daily manifest (roster + keys for the hub).
                        if (res.role == "COORDINATOR") runCatching {
                            val m = ManifestClient(Graph.db.dao()).fetchAndStore(res.token)
                            AppState.manifest = m; SessionStore.saveManifest(m)
                        }
                    }.onFailure { error = ug.qaat.coordinator.net.Net.friendly(it) }
                    busy = false
                }
            },
        ) { Text(if (busy) "Signing in…" else "Sign in") }
        if (busy) {
            Spacer(Modifier.height(10.dp))
            LinearProgressIndicator(Modifier.fillMaxWidth())
            Spacer(Modifier.height(6.dp))
            Text(
                "Contacting the server… the first sign-in can take up to a minute while it wakes up. Please keep waiting.",
                style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }

        // NOTE: there is deliberately NO editable "server address" field here. Every installed
        // phone always talks to the built-in KIU cloud backend (BuildConfig.API_BASE) so the app
        // works identically on every device with zero setup. A wrong address typed/saved on one
        // phone used to break sign-in with confusing, device-specific errors — that is gone now.
        // (To point a build at a different backend, rebuild with -Pqaat.apiBase=… — it's a
        // build-time choice, never something a user has to configure on the phone.)
    }
    // Footer statement — same as the web apps.
    Text(
        "Powered by LIGHT TECHNOLOGIES",
        style = MaterialTheme.typography.labelSmall,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
        modifier = Modifier.align(Alignment.BottomCenter).padding(bottom = 16.dp),
    )
    }
}
