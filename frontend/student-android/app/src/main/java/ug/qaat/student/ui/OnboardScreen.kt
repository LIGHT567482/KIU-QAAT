package ug.qaat.student.ui

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch
import ug.qaat.student.net.OnboardClient
import ug.qaat.student.store.StudentStore
import ug.qaat.student.util.Fingerprint

/**
 * One-time onboarding (needs internet once): enter your registration number + institution. This
 * registers THIS phone to you (one device = one student). After this the app opens straight to the
 * one-tap ATTEND and never needs the internet again.
 */
@Composable
fun OnboardScreen(onDone: () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var reg by remember { mutableStateOf("") }
    var org by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }

    Column(
        Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(24.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Spacer(Modifier.height(32.dp))
        Text("Set up QAAT Attend", style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold)
        Text("Enter your details once to register this phone. After this you just tap to attend — no internet needed in class.",
            style = MaterialTheme.typography.bodyMedium, textAlign = TextAlign.Center,
            color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(vertical = 12.dp))

        Spacer(Modifier.height(8.dp))
        OutlinedTextField(
            reg, { reg = it.trim() }, singleLine = true, modifier = Modifier.fillMaxWidth(),
            label = { Text("Registration number") }, placeholder = { Text("e.g. KIU/2021/1234") },
            enabled = !busy,
        )
        Spacer(Modifier.height(10.dp))
        OutlinedTextField(
            org, { org = it.trim() }, singleLine = true, modifier = Modifier.fillMaxWidth(),
            label = { Text("Institution ID") }, placeholder = { Text("e.g. kiu") },
            supportingText = { Text("Your institution's ID or domain (ask your coordinator if unsure).") },
            enabled = !busy,
        )

        Spacer(Modifier.height(20.dp))
        Button(
            enabled = !busy && reg.isNotBlank() && org.isNotBlank(),
            modifier = Modifier.fillMaxWidth(),
            onClick = {
                busy = true; error = null
                scope.launch {
                    val fp = Fingerprint.get(ctx)
                    runCatching { OnboardClient().register(reg, org, fp) }
                        .onSuccess { r -> StudentStore.save(r.reg, r.fullName.ifBlank { r.reg }); onDone() }
                        .onFailure { error = it.message ?: "Registration failed"; busy = false }
                }
            },
        ) { Text(if (busy) "Registering…" else "Register this phone") }

        if (busy) {
            Spacer(Modifier.height(14.dp)); LinearProgressIndicator(Modifier.fillMaxWidth())
            Text("Contacting the server… the first time can take up to a minute.",
                style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(top = 8.dp), textAlign = TextAlign.Center)
        }
        error?.let {
            Text(it, color = MaterialTheme.colorScheme.error, textAlign = TextAlign.Center, modifier = Modifier.padding(top = 12.dp))
        }
    }
}
