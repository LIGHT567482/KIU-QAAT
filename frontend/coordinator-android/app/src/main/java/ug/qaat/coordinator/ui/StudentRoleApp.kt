package ug.qaat.coordinator.ui

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch
import ug.qaat.coordinator.net.AuthClient
import ug.qaat.coordinator.store.SessionStore
import ug.qaat.coordinator.student.CheckinClient
import ug.qaat.coordinator.student.Discovery
import ug.qaat.coordinator.student.Fingerprint
import ug.qaat.coordinator.student.ProgressClient
import ug.qaat.coordinator.student.RegisterDeviceClient

private val REASONS = mapOf(
    "NOT_ON_ROSTER" to "You're not on this class's roster.",
    "DUPLICATE_SCAN" to "You're already marked present.",
    "DEVICE_ALREADY_USED" to "This phone already checked in another student for this lecture.",
    "DEVICE_MISMATCH" to "This isn't the device you first checked in with.",
    "DEVICE_BELONGS_TO_ANOTHER_STUDENT" to "This phone is registered to another student.",
    "SESSION_NOT_ACTIVE" to "No active session yet — wait for your coordinator to start.",
    "LECTURER_NOT_STARTED" to "Waiting for the lecturer to start the session.",
)

/** The STUDENT experience inside the unified app: one-tap Attend + online My-attendance + Profile.
 *  Identity (reg-number, institution) comes from the login token via AppState. */
@Composable
fun StudentRoleApp() {
    val ctx = LocalContext.current
    // Bind this phone to the student (one-device-one-student) once after login; capture any cooldown.
    LaunchedEffect(AppState.studentId) {
        val reg = AppState.studentId; val org = AppState.org
        if (!reg.isNullOrBlank() && !org.isNullOrBlank()) {
            runCatching { RegisterDeviceClient().register(reg, org, Fingerprint.get(ctx)) }
                .onSuccess { AppState.attendBlockUntil = it.attendBlockUntilMs; SessionStore.saveAttendBlockUntil(it.attendBlockUntilMs) }
        }
        if (AppState.attendBlockUntil == 0L) AppState.attendBlockUntil = SessionStore.attendBlockUntil()
    }

    var tab by remember { mutableStateOf(0) }
    Scaffold(
        bottomBar = {
            NavigationBar {
                NavigationBarItem(tab == 0, { tab = 0}, icon = { Text("✓") }, label = { Text("Attend") })
                NavigationBarItem(tab == 1, { tab = 1 }, icon = { Text("📊") }, label = { Text("Attendance") })
                NavigationBarItem(tab == 2, { tab = 2 }, icon = { Text("☰") }, label = { Text("Profile") })
            }
        },
    ) { pad ->
        Box(Modifier.padding(pad)) {
            when (tab) {
                0 -> StudentAttend()
                1 -> StudentProgress()
                else -> StudentProfile()
            }
        }
    }
}

@Composable
private fun StudentAttend() {
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
        Spacer(Modifier.height(12.dp))
        Text("KIU QAAT", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        AppState.studentId?.let { Text(it, color = MaterialTheme.colorScheme.onSurfaceVariant, style = MaterialTheme.typography.labelMedium) }
        Spacer(Modifier.weight(1f))

        val blocked = System.currentTimeMillis() < AppState.attendBlockUntil
        when {
            searching -> { CircularProgressIndicator(); Text("Finding your coordinator…", Modifier.padding(top = 12.dp)) }
            success -> {
                Text("✓", style = MaterialTheme.typography.displayMedium, color = MaterialTheme.colorScheme.primary)
                Text("You're marked present", style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold, textAlign = TextAlign.Center)
                session?.let { Text("${it.unitName} · ${it.cohort}", textAlign = TextAlign.Center, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                Text("You can turn Wi-Fi off now so a classmate can connect.", Modifier.padding(top = 8.dp), textAlign = TextAlign.Center, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            session?.active == true && blocked -> {
                val until = java.text.SimpleDateFormat("EEE d MMM, h:mm a", java.util.Locale.getDefault()).format(java.util.Date(AppState.attendBlockUntil))
                Text("⏸", style = MaterialTheme.typography.displaySmall)
                Text("Attendance paused", style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold)
                Text("You recently switched phones. You can take attendance on this device from $until.",
                    textAlign = TextAlign.Center, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(top = 8.dp))
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
                            runCatching { CheckinClient(baseUrl!!).attend(AppState.studentId ?: "", fp) }
                                .onSuccess { r ->
                                    if (r.present || r.alreadyPresent) success = true
                                    else status = REASONS[r.reason] ?: "Not marked: ${r.reason ?: "try again"}"
                                    busy = false
                                }
                                .onFailure { status = "Couldn't reach the class server — make sure mobile data is OFF and you're on the coordinator's Wi-Fi."; busy = false }
                        }
                    },
                ) { Text(if (busy) "Marking…" else "ATTEND", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold) }
            }
            else -> Text(status ?: "No active session.", textAlign = TextAlign.Center, color = MaterialTheme.colorScheme.error)
        }
        status?.takeIf { !success && session?.active == true && !blocked }?.let {
            Text(it, color = MaterialTheme.colorScheme.error, textAlign = TextAlign.Center, modifier = Modifier.padding(top = 12.dp))
        }
        Spacer(Modifier.weight(1f))
        OutlinedButton(onClick = { scope.launch { discover() } }, enabled = !searching, modifier = Modifier.fillMaxWidth()) {
            Text(if (success) "Done" else "Retry / refresh")
        }
    }
}

@Composable
private fun StudentProgress() {
    val scope = rememberCoroutineScope()
    var data by remember { mutableStateOf<ProgressClient.Progress?>(null) }
    var loading by remember { mutableStateOf(true) }
    var error by remember { mutableStateOf<String?>(null) }

    fun load() {
        loading = true; error = null
        scope.launch {
            val reg = AppState.studentId; val org = AppState.org
            if (reg.isNullOrBlank() || org.isNullOrBlank()) { error = "Sign in again to view your progress."; loading = false; return@launch }
            runCatching { ProgressClient().fetch(reg, org) }
                .onSuccess { data = it; loading = false }
                .onFailure { error = it.message ?: "Couldn't load progress"; loading = false }
        }
    }
    LaunchedEffect(Unit) { load() }

    Column(Modifier.fillMaxSize().padding(20.dp)) {
        Text("My attendance", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        data?.let { Text(it.fullName, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        Spacer(Modifier.height(12.dp))
        when {
            loading -> Box(Modifier.fillMaxWidth().padding(top = 40.dp), Alignment.Center) { CircularProgressIndicator() }
            error != null -> Text(error!!, color = MaterialTheme.colorScheme.error)
            data?.units.isNullOrEmpty() -> Text("No attendance recorded yet.", color = MaterialTheme.colorScheme.onSurfaceVariant)
            else -> LazyColumn(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                items(data!!.units) { u ->
                    val eligible = u.status == "ELIGIBLE"
                    Surface(color = MaterialTheme.colorScheme.surfaceVariant, shape = MaterialTheme.shapes.medium) {
                        Column(Modifier.fillMaxWidth().padding(12.dp)) {
                            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                                Text(u.unitName.ifBlank { u.unitId }, Modifier.weight(1f), fontWeight = FontWeight.SemiBold)
                                Text("${"%.0f".format(u.pct)}%", fontWeight = FontWeight.Bold,
                                    color = if (eligible) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error)
                            }
                            LinearProgressIndicator(progress = { (u.pct / 100.0).toFloat().coerceIn(0f, 1f) },
                                modifier = Modifier.fillMaxWidth().padding(vertical = 6.dp))
                            Text("${u.attended}/${u.held} attended · pass mark ${u.threshold}%",
                                style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            if (!eligible) Text("Exam-ineligible" + (u.deficit?.let { " — attend $it more to recover" } ?: ""),
                                style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.error)
                        }
                    }
                }
            }
        }
        Spacer(Modifier.weight(1f))
        OutlinedButton(onClick = { load() }, enabled = !loading, modifier = Modifier.fillMaxWidth()) { Text("Refresh") }
    }
}

@Composable
private fun StudentProfile() {
    var showChangePw by remember { mutableStateOf(false) }
    if (showChangePw) ChangePasswordDialog(onClose = { showChangePw = false })
    Column(Modifier.fillMaxSize().padding(24.dp)) {
        Text("Profile", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(12.dp))
        AppState.coordinatorName?.takeIf { it.isNotBlank() }?.let { Text(it, fontWeight = FontWeight.SemiBold) }
        AppState.studentId?.let { Text("Reg. no: $it", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        AppState.role?.let { Text(it, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        Spacer(Modifier.height(24.dp))
        OutlinedButton(onClick = { showChangePw = true }, Modifier.fillMaxWidth()) { Text("🔑  Change password") }
        Spacer(Modifier.height(8.dp))
        Button(onClick = { signOut() }, Modifier.fillMaxWidth()) { Text("Sign out") }
    }
}
