package ug.qaat.coordinator.ui

import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch
import ug.qaat.coordinator.di.Graph
import ug.qaat.coordinator.net.AuthClient
import ug.qaat.coordinator.net.ManifestClient

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

    Column(
        Modifier.fillMaxSize().padding(24.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        Text("QAAT Coordinator", style = MaterialTheme.typography.headlineSmall)
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
                        AppState.manifest = ManifestClient(Graph.db.dao()).fetchAndStore(res.token)
                        // Adopt the tenant's logo + colours app-wide (best-effort).
                        AppState.branding = runCatching { ug.qaat.coordinator.net.BrandingClient(res.token).fetch() }.getOrNull()
                        onLoggedIn()
                    }.onFailure { error = it.message ?: "Login failed" }
                    busy = false
                }
            },
        ) { Text(if (busy) "Signing in…" else "Sign in") }
    }
}
