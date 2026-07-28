package ug.qaat.coordinator.ui

import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch
import ug.qaat.coordinator.student.CheckinClient
import ug.qaat.coordinator.student.Discovery
import ug.qaat.coordinator.student.Fingerprint
import ug.qaat.coordinator.student.GateClient

/**
 * The LECTURER experience in the unified app: join the coordinator's hotspot, then START / END the
 * session over the LAN (no QR scanning). The rotating room code is read live from GET /status; the
 * lecturer's staff-id comes from the login token. Presence proof = being on the hotspot.
 */
@Composable
fun LecturerApp() {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var baseUrl by remember { mutableStateOf<String?>(null) }
    var session by remember { mutableStateOf<CheckinClient.Session?>(null) }
    var searching by remember { mutableStateOf(true) }
    var busy by remember { mutableStateOf(false) }
    var msg by remember { mutableStateOf<String?>(null) }
    var showChangePw by remember { mutableStateOf(false) }
    if (showChangePw) ChangePasswordDialog(onClose = { showChangePw = false })

    suspend fun refresh() {
        searching = true; msg = null
        val url = Discovery(ctx).find()
        baseUrl = url
        session = url?.let { CheckinClient(it).session() }
        searching = false
        if (url == null) msg = "Couldn't find the coordinator. Join their class Wi-Fi, then retry."
    }
    LaunchedEffect(Unit) { refresh() }

    fun gate() {
        val url = baseUrl ?: return
        busy = true; msg = null
        scope.launch {
            val gc = GateClient(url)
            val st = gc.status()
            if (st == null || st.roomCode.isBlank()) { msg = "No live session code — ask the coordinator to project the gate, then retry."; busy = false; return@launch }
            runCatching { gc.gate(AppState.staffId ?: "", st.roomCode, Fingerprint.get(ctx)) }
                .onSuccess { r ->
                    msg = when (r.status) {
                        "STARTED" -> "✓ Session started — students can now check in."
                        "ENDED" -> "✓ Session ended — attendance is sealed."
                        else -> "Not accepted: ${(r.reason ?: "try again").replace('_', ' ').lowercase()}. Retry."
                    }
                    session = CheckinClient(url).session()
                    busy = false
                }
                .onFailure { msg = "Couldn't reach the class server — make sure mobile data is OFF and you're on the coordinator's Wi-Fi."; busy = false }
        }
    }

    Column(Modifier.fillMaxSize().padding(24.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        Spacer(Modifier.height(16.dp))
        Text("KIU QAAT — Lecturer", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        AppState.coordinatorName?.takeIf { it.isNotBlank() }?.let { Text(it, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        AppState.staffId?.let { Text("Staff ID: $it", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }

        Spacer(Modifier.weight(1f))
        val started = session?.lecturerStarted == true
        when {
            searching -> { CircularProgressIndicator(); Text("Finding the class…", Modifier.padding(top = 12.dp)) }
            session?.active == true -> {
                Text(if (started) "Session running" else "Ready to start", color = MaterialTheme.colorScheme.onSurfaceVariant)
                Text(session!!.unitName.ifBlank { "this session" }, style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold, textAlign = TextAlign.Center)
                if (session!!.cohort.isNotBlank()) Text(session!!.cohort, color = MaterialTheme.colorScheme.onSurfaceVariant, textAlign = TextAlign.Center)
                Spacer(Modifier.height(24.dp))
                Button(enabled = !busy, modifier = Modifier.fillMaxWidth().height(60.dp), onClick = { gate() }) {
                    Text(if (busy) "Please wait…" else if (started) "END SESSION" else "START SESSION",
                        style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
                }
            }
            else -> Text(msg ?: "No active session on the coordinator yet.", textAlign = TextAlign.Center, color = MaterialTheme.colorScheme.error)
        }
        msg?.takeIf { session?.active == true }?.let { Text(it, Modifier.padding(top = 12.dp), textAlign = TextAlign.Center) }

        Spacer(Modifier.weight(1f))
        OutlinedButton(onClick = { scope.launch { refresh() } }, enabled = !searching, modifier = Modifier.fillMaxWidth()) { Text("Refresh") }
        Spacer(Modifier.height(8.dp))
        Row(Modifier.fillMaxWidth()) {
            TextButton(onClick = { showChangePw = true }, Modifier.weight(1f)) { Text("Change password") }
            TextButton(onClick = { signOut() }, Modifier.weight(1f)) { Text("Sign out") }
        }
    }
}
