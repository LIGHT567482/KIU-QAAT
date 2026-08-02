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
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch
import ug.qaat.coordinator.net.AuthClient
import ug.qaat.coordinator.net.NotificationClient
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
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun StudentRoleApp() {
    val ctx = LocalContext.current
    // Bind this phone to the student (one-device-one-student) once after login; capture any cooldown.
    LaunchedEffect(AppState.studentId) {
        val reg = AppState.studentId
        if (!reg.isNullOrBlank()) {
            runCatching { RegisterDeviceClient().register(reg, Fingerprint.get(ctx)) }
                .onSuccess { AppState.attendBlockUntil = it.attendBlockUntilMs; SessionStore.saveAttendBlockUntil(it.attendBlockUntilMs) }
        }
        if (AppState.attendBlockUntil == 0L) AppState.attendBlockUntil = SessionStore.attendBlockUntil()
    }

    val navColor = navBarColor(AppState.branding)
    val onNav = navColor?.let { onNavColor(it) }
    var tab by remember { mutableStateOf(0) }
    var showPortal by remember { mutableStateOf(false) }

    // The KIU student portal takes over the whole screen (with its own back button) when opened.
    if (showPortal) {
        StudentPortalScreen(regNo = AppState.studentId, onClose = { showPortal = false })
        return
    }
    var unread by remember { mutableStateOf(0) }
    // Refresh the unread badge whenever the tab changes (so it clears after reading the inbox).
    LaunchedEffect(tab) { runCatching { unread = NotificationClient().unread() } }

    Scaffold(
        containerColor = (if (!AppState.darkTheme) appBackgroundColor(AppState.branding) else null)
            ?: MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                colors = if (navColor != null) TopAppBarDefaults.topAppBarColors(
                    containerColor = navColor, titleContentColor = onNav!!, actionIconContentColor = onNav,
                ) else TopAppBarDefaults.topAppBarColors(),
                title = {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        BrandLogo(AppState.branding, size = 30)
                        Spacer(Modifier.width(8.dp))
                        Text("KIU QAAT", fontWeight = FontWeight.Bold)
                    }
                },
                actions = {
                    // Light/dark toggle, like the coordinator app.
                    IconButton(onClick = { AppState.darkTheme = !AppState.darkTheme; SessionStore.saveTheme(AppState.darkTheme) }) {
                        BarIcon(if (AppState.darkTheme) NavIcons.LightMode else NavIcons.DarkMode,
                            if (AppState.darkTheme) "Switch to light theme" else "Switch to dark theme",
                            onNav ?: MaterialTheme.colorScheme.primary)
                    }
                },
            )
        },
        bottomBar = {
            NavigationBar(containerColor = navColor ?: MaterialTheme.colorScheme.surface) {
                val itemColors = if (onNav != null) NavigationBarItemDefaults.colors(
                    selectedIconColor = onNav, selectedTextColor = onNav,
                    unselectedIconColor = onNav.copy(alpha = .65f), unselectedTextColor = onNav.copy(alpha = .65f),
                    indicatorColor = onNav.copy(alpha = .18f),
                ) else NavigationBarItemDefaults.colors()
                NavigationBarItem(tab == 0, { tab = 0 }, icon = { TabGlyph(NavIcons.Attend, "Attend") }, label = { Text("Attend") }, colors = itemColors)
                NavigationBarItem(tab == 1, { tab = 1 }, icon = { TabGlyph(NavIcons.Attendance, "Attendance") }, label = { Text("Attendance") }, colors = itemColors)
                NavigationBarItem(tab == 2, { tab = 2 }, colors = itemColors, label = { Text("Alerts") },
                    icon = { if (unread > 0) BadgedBox(badge = { Badge { Text("$unread") } }) { TabGlyph(NavIcons.Alerts, "Alerts") } else TabGlyph(NavIcons.Alerts, "Alerts") })
                NavigationBarItem(tab == 3, { tab = 3 }, icon = { TabGlyph(NavIcons.Profile, "Profile") }, label = { Text("Profile") }, colors = itemColors)
            }
        },
    ) { pad ->
        Column(Modifier.padding(pad).fillMaxSize()) {
            StudentHeader()
            Box(Modifier.weight(1f)) {
                when (tab) {
                    0 -> StudentAttend()
                    1 -> StudentProgress()
                    2 -> StudentNotifications(onRead = { runCatching { } })
                    else -> StudentProfile(onOpenPortal = { showPortal = true })
                }
            }
        }
    }
}

/** The KIU-branded page header: institution logo + KIU QAAT is in the top bar; here we show
 *  WHO is signed in — student name, registration number and (when known) their cohort. */
@Composable
private fun StudentHeader() {
    Column(Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 10.dp)) {
        AppState.coordinatorName?.takeIf { it.isNotBlank() }?.let {
            Text(it, fontWeight = FontWeight.Bold, fontSize = 17.sp)
        }
        AppState.studentId?.let {
            Text("Reg. no: $it", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        AppState.cohortLabel?.takeIf { it.isNotBlank() }?.let {
            Text(it, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

/** The student's notification inbox — messages from their lecturers and coordinator. Tapping a
 *  message marks it read. Fully cloud-backed (fetched when the phone is online). */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun StudentNotifications(onRead: () -> Unit) {
    val scope = rememberCoroutineScope()
    var items by remember { mutableStateOf<List<NotificationClient.Notif>?>(null) }
    fun load() { scope.launch { items = NotificationClient().inbox() } }
    LaunchedEffect(Unit) { load() }

    Column(Modifier.fillMaxSize().padding(16.dp)) {
        Text("Notifications", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(8.dp))
        when {
            items == null -> Box(Modifier.fillMaxWidth().padding(top = 40.dp), Alignment.Center) { CircularProgressIndicator() }
            items!!.isEmpty() -> Text("No notifications yet.", color = MaterialTheme.colorScheme.onSurfaceVariant)
            else -> LazyColumn(verticalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.weight(1f)) {
                items(items!!) { n ->
                    var expanded by remember { mutableStateOf(false) }
                    Surface(
                        color = if (!n.read) MaterialTheme.colorScheme.primary.copy(alpha = .08f) else MaterialTheme.colorScheme.surfaceVariant,
                        shape = MaterialTheme.shapes.medium,
                        onClick = {
                            expanded = !expanded
                            if (!n.read) scope.launch { NotificationClient().markRead(n.id); load(); onRead() }
                        },
                    ) {
                        Column(Modifier.fillMaxWidth().padding(12.dp)) {
                            Text(n.subject, fontWeight = if (n.read) FontWeight.SemiBold else FontWeight.Bold)
                            Text("from ${n.senderName} · ${n.senderRole.replace('_', ' ')}",
                                style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            if (expanded && n.body.isNotBlank()) { Spacer(Modifier.height(6.dp)); Text(n.body) }
                        }
                    }
                }
            }
        }
        OutlinedButton(onClick = { load() }, modifier = Modifier.fillMaxWidth().padding(top = 8.dp)) { Text("Refresh") }
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
    // True once this device has attended the CURRENT session (persisted for the day). Drives the
    // greyed, disabled ATTEND button + the "already attended, turn Wi-Fi off" message.
    var alreadyAttended by remember { mutableStateOf(false) }
    // The student's latest notification, surfaced HERE on the attendance page right after a
    // successful check-in (so they see any message from their lecturer/coordinator immediately).
    var latestNotif by remember { mutableStateOf<NotificationClient.Notif?>(null) }
    LaunchedEffect(success) { if (success) runCatching { latestNotif = NotificationClient().inbox().firstOrNull() } }

    suspend fun discover(showSpinner: Boolean = true) {
        if (showSpinner) { searching = true; status = null }
        val url = Discovery(ctx).find()
        val newSession = url?.let { CheckinClient(it).session() }
        // Reset the local "attended" view when the session changes (new sessionId = next round) or
        // the hotspot dropped, so the student is prompted to reconnect / can attend the next session.
        val newId = newSession?.sessionId ?: ""
        val oldId = session?.sessionId ?: ""
        if (newId != oldId) { success = false }
        baseUrl = url
        session = newSession
        alreadyAttended = newSession?.sessionId?.let { SessionStore.hasAttended(it) } == true
        if (alreadyAttended) success = true
        searching = false
        status = when {
            url == null -> "Couldn't find your coordinator. Connect to their class Wi-Fi (mobile data OFF)."
            newSession?.active != true -> "Connected, but no session is active yet — wait for it to start."
            else -> null
        }
    }
    LaunchedEffect(Unit) { discover() }
    // Poll every ~4s so the page reacts on its own: clears to "reconnect" when the hotspot drops,
    // and re-enables for the next session — no manual refresh needed.
    LaunchedEffect(Unit) {
        while (true) {
            kotlinx.coroutines.delay(4000)
            if (!busy && !searching) runCatching { discover(showSpinner = false) }
        }
    }

    Column(Modifier.fillMaxSize().padding(24.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        Spacer(Modifier.height(12.dp))
        Text("KIU QAAT", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        AppState.studentId?.let { Text(it, color = MaterialTheme.colorScheme.onSurfaceVariant, style = MaterialTheme.typography.labelMedium) }
        Spacer(Modifier.weight(1f))

        // Device-switch cooldown only applies when the anti-cheat is enabled (off during testing).
        val blocked = AppState.ENFORCE_DEVICE_LOCK && System.currentTimeMillis() < AppState.attendBlockUntil
        when {
            searching -> { CircularProgressIndicator(); Text("Finding your coordinator…", Modifier.padding(top = 12.dp)) }
            success -> {
                Text("✓", style = MaterialTheme.typography.displayMedium, color = MaterialTheme.colorScheme.primary)
                Text(if (alreadyAttended) "You already attended" else "You're marked present",
                    style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold, textAlign = TextAlign.Center)
                session?.let { Text("${it.unitName} · ${it.cohort}", textAlign = TextAlign.Center, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                Spacer(Modifier.height(12.dp))
                // Greyed, disabled button reinforces that they're done and must free the Wi-Fi slot.
                Button(enabled = false, modifier = Modifier.fillMaxWidth().height(64.dp), onClick = {}) {
                    Text("ATTENDED ✓", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
                }
                Spacer(Modifier.height(12.dp))
                Surface(color = MaterialTheme.colorScheme.errorContainer, shape = MaterialTheme.shapes.medium, modifier = Modifier.fillMaxWidth()) {
                    Text("📴 Turn your Wi-Fi OFF now so a classmate can connect and check in.",
                        Modifier.padding(14.dp), textAlign = TextAlign.Center, fontWeight = FontWeight.Bold,
                        color = MaterialTheme.colorScheme.onErrorContainer)
                }
                // Latest notification, shown right after checking in.
                latestNotif?.let { n ->
                    Spacer(Modifier.height(12.dp))
                    Surface(color = MaterialTheme.colorScheme.secondaryContainer, shape = MaterialTheme.shapes.medium, modifier = Modifier.fillMaxWidth()) {
                        Column(Modifier.padding(12.dp)) {
                            Text("🔔 ${n.subject}", fontWeight = FontWeight.Bold, color = MaterialTheme.colorScheme.onSecondaryContainer)
                            Text("from ${n.senderName}", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSecondaryContainer)
                            if (n.body.isNotBlank()) { Spacer(Modifier.height(4.dp)); Text(n.body, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSecondaryContainer) }
                        }
                    }
                }
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
                                    if (r.present || r.alreadyPresent) {
                                        success = true
                                        // Remember THIS session so the button stays greyed on reconnect (one per session).
                                        session?.sessionId?.let { SessionStore.markAttended(it); alreadyAttended = true }
                                    } else status = REASONS[r.reason] ?: "Not marked: ${r.reason ?: "try again"}"
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
            val reg = AppState.studentId
            if (reg.isNullOrBlank()) { error = "Sign in again to view your progress."; loading = false; return@launch }
            runCatching { ProgressClient().fetch(reg) }
                .onSuccess { data = it; loading = false }
                .onFailure { error = it.message ?: "Couldn't load progress"; loading = false }
        }
    }
    LaunchedEffect(Unit) { load() }

    Column(Modifier.fillMaxSize().padding(20.dp)) {
        Text("My attendance", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        data?.let { Text(it.fullName, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        data?.let { p ->
            val org = listOf(p.school, p.department).filter { it.isNotBlank() }.joinToString(" · ")
            if (org.isNotBlank()) Text(org, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
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
private fun StudentProfile(onOpenPortal: () -> Unit) {
    var showChangePw by remember { mutableStateOf(false) }
    if (showChangePw) ChangePasswordDialog(onClose = { showChangePw = false })
    Column(Modifier.fillMaxSize().padding(24.dp)) {
        Text("Profile", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(8.dp))
        // Opens the KIU student portal in-app, with the reg number auto-filled.
        Button(onClick = onOpenPortal, Modifier.fillMaxWidth()) { Text("🎓  Student portal") }
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
