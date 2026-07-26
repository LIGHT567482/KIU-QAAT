package ug.qaat.coordinator.ui

import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch
import ug.qaat.coordinator.R
import ug.qaat.coordinator.di.Graph
import ug.qaat.coordinator.net.AuthClient
import ug.qaat.coordinator.net.ManifestClient
import ug.qaat.coordinator.net.Net
import ug.qaat.coordinator.store.SessionStore

/** Coordinator login → stores token + device binding key, then pulls the daily manifest. */
@Composable
fun LoginScreen(onLoggedIn: () -> Unit) {
    val scope = rememberCoroutineScope()
    var email by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var totp by remember { mutableStateOf("") }
    var needsMfa by remember { mutableStateOf(false) }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }

    Box(Modifier.fillMaxSize()) {
    Column(
        Modifier.fillMaxSize().padding(24.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        Image(
            painter = painterResource(R.drawable.qaat_logo),
            contentDescription = "QAAT",
            modifier = Modifier.size(112.dp),
        )
        Spacer(Modifier.height(14.dp))
        Text("QAAT", style = MaterialTheme.typography.headlineMedium)
        Text("Coordinator sign-in", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.height(24.dp))
        OutlinedTextField(email, { email = it }, label = { Text("Email") }, singleLine = true, modifier = Modifier.fillMaxWidth())
        Spacer(Modifier.height(8.dp))
        OutlinedTextField(password, { password = it }, label = { Text("Password") }, singleLine = true,
            visualTransformation = PasswordVisualTransformation(), modifier = Modifier.fillMaxWidth())
        if (needsMfa) {
            Spacer(Modifier.height(8.dp))
            OutlinedTextField(totp, { totp = it }, label = { Text("Authenticator code") }, singleLine = true, modifier = Modifier.fillMaxWidth())
        }
        error?.let { Spacer(Modifier.height(8.dp)); Text(it, color = MaterialTheme.colorScheme.error) }

        Spacer(Modifier.height(16.dp))
        Button(
            enabled = !busy && email.isNotBlank() && password.isNotBlank(),
            modifier = Modifier.fillMaxWidth(),
            onClick = {
                busy = true; error = null
                scope.launch {
                    runCatching {
                        val res = AuthClient().login(email.trim(), password, totp.ifBlank { null }) { needsMfa = true }
                            ?: return@runCatching   // MFA prompt shown
                        AppState.token = res.token; AppState.userId = res.userId
                        AppState.tenantId = res.tenantId; AppState.deviceBindingKey = res.deviceBindingKey
                        AppState.coordinatorName = res.fullName; AppState.coordinatorEmail = email.trim(); AppState.role = res.role
                        AppState.coordinatorTitle = res.title; AppState.coordinatorRegNo = res.registrationNo
                        val m = ManifestClient(Graph.db.dao()).fetchAndStore(res.token)
                        AppState.manifest = m
                        // Persist for auto-login next time + fully-offline attendance.
                        SessionStore.saveSession(res.token, res.userId, res.tenantId, res.deviceBindingKey, res.fullName, email.trim(), res.role, res.title, res.registrationNo)
                        SessionStore.saveCredentials(email.trim(), password)
                        SessionStore.saveManifest(m)
                        // Adopt the tenant's logo + colours app-wide (best-effort) + cache
                        // them so the same branding shows on auto-login / offline.
                        AppState.branding = runCatching { ug.qaat.coordinator.net.BrandingClient(res.token).fetch() }.getOrNull()
                        AppState.branding?.let { runCatching { SessionStore.saveBranding(it) } }
                        onLoggedIn()
                    }.onFailure { error = it.message ?: "Login failed" }
                    busy = false
                }
            },
        ) { Text(if (busy) "Signing in…" else "Sign in") }
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
