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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import ug.qaat.coordinator.net.AuthClient
import ug.qaat.coordinator.net.NotificationClient
import ug.qaat.coordinator.net.StudentHomeClient
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
    // Bumped by the top-bar refresh; every page keys its fetch on it and reloads.
    var reloadKey by remember { mutableStateOf(0) }

    // The KIU student portal takes over the whole screen (with its own back button) when opened.
    if (showPortal) {
        StudentPortalScreen(regNo = AppState.studentId, onClose = { showPortal = false })
        return
    }
    var unread by remember { mutableStateOf(0) }
    // Refresh the unread badge whenever the tab changes (so it clears after reading the inbox).
    LaunchedEffect(tab, reloadKey) { runCatching { unread = NotificationClient().unread() } }

    Scaffold(
        containerColor = (if (!AppState.darkTheme) appBackgroundColor(AppState.branding) else null)
            ?: MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                colors = if (navColor != null) TopAppBarDefaults.topAppBarColors(
                    containerColor = navColor, titleContentColor = onNav!!, actionIconContentColor = onNav,
                ) else TopAppBarDefaults.topAppBarColors(),
                title = { BrandHeader(AppState.branding) },
                actions = {
                    // Refresh + light/dark toggle, matching the coordinator app's top bar.
                    IconButton(onClick = { reloadKey++ }) {
                        BarIcon(NavIcons.Sync, "Refresh", onNav ?: MaterialTheme.colorScheme.primary)
                    }
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
                NavigationBarItem(tab == 0, { tab = 0 }, icon = { TabGlyph(NavIcons.Home, "Home") }, label = { Text("Home") }, colors = itemColors)
                NavigationBarItem(tab == 1, { tab = 1 }, icon = { TabGlyph(NavIcons.Attend, "Attend") }, label = { Text("Attend") }, colors = itemColors)
                NavigationBarItem(tab == 2, { tab = 2 }, icon = { TabGlyph(NavIcons.Attendance, "Attendance") }, label = { Text("Attendance") }, colors = itemColors)
                NavigationBarItem(tab == 3, { tab = 3 }, colors = itemColors, label = { Text("Alerts") },
                    icon = { if (unread > 0) BadgedBox(badge = { Badge { Text("$unread") } }) { TabGlyph(NavIcons.Alerts, "Alerts") } else TabGlyph(NavIcons.Alerts, "Alerts") })
                NavigationBarItem(tab == 4, { tab = 4 }, icon = { TabGlyph(NavIcons.Profile, "Profile") }, label = { Text("Profile") }, colors = itemColors)
            }
        },
    ) { pad ->
        Column(Modifier.padding(pad).fillMaxSize()) {
            Box(Modifier.weight(1f)) {
                when (tab) {
                    0 -> StudentHome(reloadKey)
                    1 -> StudentAttend()
                    2 -> StudentProgress(reloadKey)
                    3 -> StudentNotifications(reloadKey, onRead = { runCatching { } })
                    else -> StudentProfile(reloadKey, onOpenPortal = { showPortal = true })
                }
            }
        }
    }
}

/** The student's notification inbox — messages from their lecturers and coordinator. Tapping a
 *  message marks it read. Fully cloud-backed (fetched when the phone is online). */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun StudentNotifications(reloadKey: Int, onRead: () -> Unit) {
    val scope = rememberCoroutineScope()
    var items by remember { mutableStateOf<List<NotificationClient.Notif>?>(null) }
    fun load() { scope.launch { items = NotificationClient().inbox() } }
    LaunchedEffect(reloadKey) { load() }

    Column(Modifier.fillMaxSize().padding(16.dp)) {
        Text("Notifications", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(8.dp))
        when {
            items == null -> Box(Modifier.fillMaxWidth().padding(top = 40.dp), Alignment.Center) { CircularProgressIndicator() }
            items!!.isEmpty() -> Text("No notifications yet.", color = MaterialTheme.colorScheme.onSurfaceVariant)
            else -> LazyColumn(verticalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.weight(1f)) {
                items(items!!, key = { it.id }) { n ->
                    NotificationCard(
                        n = n,
                        onOpen = { if (!n.read) scope.launch { NotificationClient().markRead(n.id); load(); onRead() } },
                        onDismiss = {
                            // Drop it locally at once so the ✕ feels instant, then confirm with the server.
                            items = items?.filterNot { it.id == n.id }
                            scope.launch { NotificationClient().dismiss(n.id); onRead() }
                        },
                    )
                }
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
private fun StudentProgress(reloadKey: Int) {
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
    LaunchedEffect(reloadKey) { load() }

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

/** Home — greets the student, then their cohort's weekly timetable and the units they take.
 *  Deliberately just those two things, as the coordinator's Home is. */
@Composable
private fun StudentHome(reloadKey: Int) {
    var home by remember { mutableStateOf<StudentHomeClient.Home?>(null) }
    var loading by remember { mutableStateOf(true) }
    LaunchedEffect(reloadKey) {
        loading = true
        home = runCatching { StudentHomeClient().fetch() }.getOrNull()
        home?.let { AppState.cohortLabel = it.cohort }
        loading = false
    }

    val name = home?.fullName?.takeIf { it.isNotBlank() }
        ?: AppState.coordinatorName?.takeIf { it.isNotBlank() } ?: "there"

    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(20.dp)) {
        Text("Welcome, $name 👋", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.ExtraBold)
        (home?.cohort ?: AppState.cohortLabel)?.takeIf { it.isNotBlank() }?.let {
            Text(it, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        Spacer(Modifier.height(18.dp))

        if (loading && home == null) {
            Box(Modifier.fillMaxWidth().padding(top = 40.dp), Alignment.Center) { CircularProgressIndicator() }
            return@Column
        }

        Text("Timetable", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(6.dp))
        val slots = home?.timetable.orEmpty()
        if (slots.isEmpty()) {
            Text("No timetable published for your cohort yet.",
                style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        } else {
            // Grouped by weekday so the week reads top-to-bottom.
            slots.groupBy { it.dayOfWeek }.toSortedMap().forEach { (day, daySlots) ->
                Text(dayName(day), style = MaterialTheme.typography.labelLarge,
                    fontWeight = FontWeight.Bold, modifier = Modifier.padding(top = 10.dp, bottom = 4.dp))
                daySlots.sortedBy { it.startTime }.forEach { sl ->
                    Surface(color = MaterialTheme.colorScheme.surfaceVariant, shape = MaterialTheme.shapes.medium,
                        modifier = Modifier.fillMaxWidth().padding(bottom = 6.dp)) {
                        Row(Modifier.padding(12.dp), verticalAlignment = Alignment.CenterVertically) {
                            Column(Modifier.weight(1f)) {
                                Text(sl.unitName.ifBlank { sl.unitId }, fontWeight = FontWeight.SemiBold)
                                Text(
                                    listOfNotNull(
                                        sl.unitId.takeIf { it.isNotBlank() },
                                        sl.room.takeIf { it.isNotBlank() },
                                        sl.lecturerName.takeIf { it.isNotBlank() },
                                    ).joinToString(" · "),
                                    style = MaterialTheme.typography.labelSmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                )
                            }
                            Text(
                                sl.startTime + (if (sl.durationMinutes > 0) " · ${sl.durationMinutes}m" else ""),
                                style = MaterialTheme.typography.labelMedium, fontWeight = FontWeight.Bold,
                            )
                        }
                    }
                }
            }
        }

        Spacer(Modifier.height(20.dp))
        val units = home?.units.orEmpty()
        val current = units.filter { it.current }
        val rest = units.filterNot { it.current }

        Text("Units", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(6.dp))
        when {
            units.isEmpty() -> Text("No units registered on your programme yet.",
                style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            current.isEmpty() -> Text("Nothing is tagged for Year ${home?.year} Semester ${home?.semester} yet — your programme's full unit list is below.",
                style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            else -> current.forEach { UnitRow(it) }
        }

        // The rest of the roadmap, so a student can always see where the semester sits in the
        // whole programme — and so an untagged year/semester never renders as "you have no units".
        if (rest.isNotEmpty()) {
            Spacer(Modifier.height(14.dp))
            Text("Other units on your programme", style = MaterialTheme.typography.labelLarge,
                fontWeight = FontWeight.Bold, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.height(6.dp))
            rest.forEach { UnitRow(it, dimmed = true) }
        }
        Spacer(Modifier.height(24.dp))
    }
}

/** One unit on the student's roadmap. `dimmed` marks a unit outside the current year/semester. */
@Composable
private fun UnitRow(u: StudentHomeClient.Unit, dimmed: Boolean = false) {
    val alpha = if (dimmed) 0.55f else 1f
    Surface(
        color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = if (dimmed) 0.45f else 1f),
        shape = MaterialTheme.shapes.medium,
        modifier = Modifier.fillMaxWidth().padding(bottom = 6.dp),
    ) {
        Row(Modifier.padding(12.dp), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text(u.unitName.ifBlank { u.unitId }, fontWeight = FontWeight.SemiBold,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = alpha))
                Text(
                    listOfNotNull(u.unitId.takeIf { it.isNotBlank() },
                        u.lecturerName.takeIf { it.isNotBlank() }).joinToString(" · "),
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = alpha),
                )
            }
            if (u.year > 0) Text("Y${u.year}/S${u.semester}",
                style = MaterialTheme.typography.labelSmall, fontWeight = FontWeight.Bold,
                color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = alpha))
        }
    }
}

private fun dayName(d: Int) = when (d) {
    1 -> "Monday"; 2 -> "Tuesday"; 3 -> "Wednesday"; 4 -> "Thursday"
    5 -> "Friday"; 6 -> "Saturday"; 7 -> "Sunday"; else -> "Unscheduled"
}

/** Profile — a whole page, so it shows the student's whole record rather than three lines. */
@Composable
private fun StudentProfile(reloadKey: Int, onOpenPortal: () -> Unit) {
    var showChangePw by remember { mutableStateOf(false) }
    if (showChangePw) ChangePasswordDialog(onClose = { showChangePw = false })

    var home by remember { mutableStateOf<StudentHomeClient.Home?>(null) }
    LaunchedEffect(reloadKey) { home = runCatching { StudentHomeClient().fetch() }.getOrNull() }

    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(24.dp)) {
        Text("Profile", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(14.dp))

        // Identity card.
        Surface(color = MaterialTheme.colorScheme.surfaceVariant, shape = MaterialTheme.shapes.large,
            modifier = Modifier.fillMaxWidth()) {
            Column(Modifier.padding(16.dp)) {
                Text(home?.fullName?.takeIf { it.isNotBlank() } ?: AppState.coordinatorName.orEmpty(),
                    style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
                AppState.role?.let {
                    Text(it, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
        }
        Spacer(Modifier.height(16.dp))

        ProfileField("Registration number", home?.studentId ?: AppState.studentId.orEmpty())
        ProfileField("Email", home?.email.orEmpty())
        ProfileField("Course", home?.course.orEmpty())
        ProfileField("Level of study", home?.level.orEmpty())
        ProfileField("Study session", home?.sessionType.orEmpty())
        ProfileField("Intake", home?.intake.orEmpty())
        ProfileField("Year of study", home?.year?.takeIf { it > 0 }?.let { "Year $it" }.orEmpty())
        ProfileField("Semester", home?.semester?.takeIf { it > 0 }?.let { "Semester $it" }.orEmpty())
        ProfileField("Academic year", home?.academicYear.orEmpty())
        ProfileField("Cohort", home?.cohort ?: AppState.cohortLabel.orEmpty())
        // This semester's units, not the whole programme roadmap that Home also lists.
        ProfileField("Units this semester", home?.units?.count { it.current }?.takeIf { it > 0 }?.toString().orEmpty())

        Spacer(Modifier.height(22.dp))
        Button(onClick = onOpenPortal, Modifier.fillMaxWidth()) { Text("🎓  Student portal") }
        Spacer(Modifier.height(8.dp))
        OutlinedButton(onClick = { showChangePw = true }, Modifier.fillMaxWidth()) { Text("🔑  Change password") }
        Spacer(Modifier.height(8.dp))
        OutlinedButton(onClick = { signOut() }, Modifier.fillMaxWidth()) {
            Text("Sign out", color = MaterialTheme.colorScheme.error)
        }
        Spacer(Modifier.height(24.dp))
    }
}

/** One label/value row. Blank values are skipped rather than shown as an empty line. */
@Composable
private fun ProfileField(label: String, value: String) {
    if (value.isBlank()) return
    Column(Modifier.fillMaxWidth().padding(vertical = 6.dp)) {
        Text(label, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(value, style = MaterialTheme.typography.bodyMedium)
    }
    HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = .4f))
}
