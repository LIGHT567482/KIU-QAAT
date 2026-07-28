package ug.qaat.student.ui

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
import ug.qaat.student.net.CheckinClient
import ug.qaat.student.net.Discovery
import ug.qaat.student.store.StudentStore
import ug.qaat.student.util.Fingerprint

/** Human-readable text for the coordinator's rejection reasons (mirrors the browser page). */
private val REASONS = mapOf(
    "NOT_ON_ROSTER" to "You're not on this class's roster.",
    "SERIAL_REVOKED" to "Your QR was reissued — re-onboard with your latest card.",
    "DUPLICATE_SCAN" to "You're already marked present.",
    "DEVICE_ALREADY_USED" to "This phone already checked in another student for this lecture.",
    "DEVICE_MISMATCH" to "This isn't the device you first checked in with.",
    "DEVICE_BELONGS_TO_ANOTHER_STUDENT" to "This phone is registered to another student.",
    "INVALID_SIGNATURE" to "Your credential couldn't be verified.",
    "QR_EXPIRED" to "Your credential has expired — ask for a reissue.",
    "TENANT_MISMATCH" to "Wrong institution for this session.",
    "SESSION_NOT_ACTIVE" to "No active session yet — wait for your coordinator to start.",
    "LECTURER_NOT_STARTED" to "Waiting for the lecturer to start the session.",
)

/**
 * The whole student experience: on open, auto-discover the coordinator on the hotspot LAN, show the
 * active unit/cohort, and expose a single ATTEND button. Fully offline.
 */
@Composable
fun AttendScreen(onReonboard: () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var baseUrl by remember { mutableStateOf<String?>(null) }
    var session by remember { mutableStateOf<CheckinClient.Session?>(null) }
    var searching by remember { mutableStateOf(true) }
    var status by remember { mutableStateOf<String?>(null) }
    var success by remember { mutableStateOf(false) }
    var busy by remember { mutableStateOf(false) }

    suspend fun discover() {
        searching = true; status = null; success = false; session = null
        val url = Discovery(ctx).find()
        baseUrl = url
        session = url?.let { CheckinClient(it).session() }
        searching = false
        status = when {
            url == null -> "Couldn't find your coordinator. Join their Wi-Fi (mobile data OFF), then retry."
            session?.active != true -> "Connected, but no session is active yet."
            else -> null
        }
    }
    LaunchedEffect(Unit) { discover() }

    Column(Modifier.fillMaxSize().padding(24.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        Spacer(Modifier.height(16.dp))
        Text("QAAT Attend", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        StudentStore.reg?.let { Text(it, color = MaterialTheme.colorScheme.onSurfaceVariant, style = MaterialTheme.typography.labelMedium) }

        Spacer(Modifier.weight(1f))
        when {
            searching -> { CircularProgressIndicator(); Text("Finding your coordinator…", Modifier.padding(top = 12.dp)) }
            success -> {
                Text("✓", style = MaterialTheme.typography.displayMedium, color = MaterialTheme.colorScheme.primary)
                Text("You're marked present", style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold, textAlign = TextAlign.Center)
                session?.let { Text("${it.unitName} · ${it.cohort}", textAlign = TextAlign.Center, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                Text("You can turn Wi-Fi off now so a classmate can connect.", Modifier.padding(top = 8.dp), textAlign = TextAlign.Center, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            session?.active == true -> {
                Text("Attendance for", color = MaterialTheme.colorScheme.onSurfaceVariant)
                Text(session!!.unitName.ifBlank { "this session" }, style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold, textAlign = TextAlign.Center)
                if (session!!.cohort.isNotBlank()) Text(session!!.cohort, color = MaterialTheme.colorScheme.onSurfaceVariant, textAlign = TextAlign.Center)
                Spacer(Modifier.height(24.dp))
                Button(
                    enabled = !busy,
                    modifier = Modifier.fillMaxWidth().height(64.dp),
                    onClick = {
                        busy = true; status = null
                        scope.launch {
                            val fp = Fingerprint.get(ctx)
                            runCatching { CheckinClient(baseUrl!!).attend(StudentStore.reg!!, fp) }
                                .onSuccess { r ->
                                    if (r.present || r.alreadyPresent) success = true
                                    else status = REASONS[r.reason] ?: "Not marked: ${r.reason ?: "try again"}"
                                    busy = false
                                }
                                .onFailure {
                                    status = "Couldn't reach the class server — make sure mobile data is OFF and you're on the coordinator's Wi-Fi."
                                    busy = false
                                }
                        }
                    },
                ) { Text(if (busy) "Marking…" else "ATTEND", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold) }
            }
            else -> Text(status ?: "No active session.", textAlign = TextAlign.Center, color = MaterialTheme.colorScheme.error)
        }
        status?.takeIf { !success && session?.active == true }?.let {
            Text(it, color = MaterialTheme.colorScheme.error, textAlign = TextAlign.Center, modifier = Modifier.padding(top = 12.dp))
        }

        Spacer(Modifier.weight(1f))
        OutlinedButton(onClick = { scope.launch { discover() } }, enabled = !searching, modifier = Modifier.fillMaxWidth()) {
            Text(if (success) "Done" else "Retry / refresh")
        }
        TextButton(onClick = onReonboard) { Text("Not you? Re-onboard", style = MaterialTheme.typography.labelSmall) }
    }
}
