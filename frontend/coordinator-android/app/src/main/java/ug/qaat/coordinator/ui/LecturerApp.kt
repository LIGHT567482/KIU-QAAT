package ug.qaat.coordinator.ui

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import kotlinx.coroutines.launch
import ug.qaat.coordinator.net.LecturerClient
import ug.qaat.coordinator.net.NotificationClient
import ug.qaat.coordinator.store.SessionStore
import ug.qaat.coordinator.student.CheckinClient
import ug.qaat.coordinator.student.Discovery
import ug.qaat.coordinator.student.Fingerprint
import ug.qaat.coordinator.student.GateClient

/**
 * The LECTURER experience: join the coordinator's hotspot and START / END the session over the
 * LAN (no QR), see his roster & attendance across sessions/cohorts, read + send notifications.
 * Green KIU chrome + dark/light theme, like the coordinator app.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LecturerApp() {
    val navColor = navBarColor(AppState.branding)
    val onNav = navColor?.let { onNavColor(it) }
    var tab by remember { mutableStateOf(0) }
    var unread by remember { mutableStateOf(0) }
    var reloadKey by remember { mutableStateOf(0) }
    LaunchedEffect(tab, reloadKey) { runCatching { unread = NotificationClient().unread() } }
    var showChangePw by remember { mutableStateOf(false) }
    if (showChangePw) ChangePasswordDialog(onClose = { showChangePw = false })

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
                        BrandLogo(AppState.branding, size = 30); Spacer(Modifier.width(8.dp)); Text("KIU QAAT", fontWeight = FontWeight.Bold)
                    }
                },
                actions = {
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
                NavigationBarItem(tab == 0, { tab = 0 }, icon = { TabGlyph(NavIcons.Session, "Session") }, label = { Text("Session") }, colors = itemColors)
                NavigationBarItem(tab == 1, { tab = 1 }, icon = { TabGlyph(NavIcons.Calendar, "Calendar") }, label = { Text("Calendar") }, colors = itemColors)
                NavigationBarItem(tab == 2, { tab = 2 }, icon = { TabGlyph(NavIcons.Roster, "Roster") }, label = { Text("Roster") }, colors = itemColors)
                NavigationBarItem(tab == 3, { tab = 3 }, colors = itemColors, label = { Text("Alerts") },
                    icon = { if (unread > 0) BadgedBox(badge = { Badge { Text("$unread") } }) { TabGlyph(NavIcons.Alerts, "Alerts") } else TabGlyph(NavIcons.Alerts, "Alerts") })
                NavigationBarItem(tab == 4, { tab = 4 }, icon = { TabGlyph(NavIcons.Profile, "Profile") }, label = { Text("Profile") }, colors = itemColors)
            }
        },
    ) { pad ->
        Column(Modifier.padding(pad).fillMaxSize()) {
            Box(Modifier.weight(1f)) {
                when (tab) {
                    0 -> LecturerSessionTab()
                    1 -> LecturerCalendarTab()
                    2 -> LecturerRosterTab()
                    3 -> LecturerAlertsTab()
                    else -> LecturerProfileTab(onChangePw = { showChangePw = true })
                }
            }
        }
    }
}

@Composable
private fun LecturerHeader() {
    Column(Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 10.dp)) {
        // Show the lecturer's academic title(s) with their name, e.g. "Dr. Jane Yuma".
        val name = AppState.coordinatorName?.takeIf { it.isNotBlank() }
        val title = AppState.coordinatorTitle?.takeIf { it.isNotBlank() }
        if (name != null) Text(listOfNotNull(title, name).joinToString(" "), fontWeight = FontWeight.Bold, fontSize = 17.sp)
        AppState.staffId?.let { Text("Staff ID: $it", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant) }
    }
}

/** Join the coordinator's hotspot → START / END the session (records the lecturer's attendance
 *  + timestamps on the coordinator's hub). Presence proof = being on the hotspot. No QR. */
@Composable
private fun LecturerSessionTab() {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var baseUrl by remember { mutableStateOf<String?>(null) }
    var session by remember { mutableStateOf<CheckinClient.Session?>(null) }
    var searching by remember { mutableStateOf(true) }
    var busy by remember { mutableStateOf(false) }
    var msg by remember { mutableStateOf<String?>(null) }

    suspend fun refresh(showSpinner: Boolean = true) {
        if (showSpinner) { searching = true; msg = null }
        val url = Discovery(ctx).find(); baseUrl = url
        session = url?.let { CheckinClient(it).session() }
        searching = false
        if (url == null) msg = "Couldn't find the coordinator's hotspot. Connect to their class Wi-Fi (mobile data OFF)."
    }
    LaunchedEffect(Unit) { refresh() }
    // Poll every ~4s so the tab clears itself when the coordinator's hotspot drops and re-arms for
    // the next session/unit — the lecturer never has to hit refresh between rounds.
    LaunchedEffect(Unit) {
        while (true) {
            kotlinx.coroutines.delay(4000)
            if (!busy && !searching) runCatching { refresh(showSpinner = false) }
        }
    }

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
                        "STARTED" -> "✓ Session started — your attendance is logged. Students can now check in."
                        "ENDED" -> "✓ Session ended — your attendance + end time are sealed."
                        else -> "Not accepted: ${(r.reason ?: "try again").replace('_', ' ').lowercase()}. Retry."
                    }
                    session = CheckinClient(url).session(); busy = false
                }
                .onFailure { msg = "Couldn't reach the class server — make sure mobile data is OFF and you're on the coordinator's Wi-Fi."; busy = false }
        }
    }

    Column(Modifier.fillMaxSize().padding(24.dp), horizontalAlignment = Alignment.CenterHorizontally) {
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
    }
}

/** Roster & attendance analytics: filter by unit, toggle enrolled vs attended-only, sort, and
 *  drill into any session to see who was present or absent. Consumes the lecturer endpoints. */
@Composable
private fun LecturerRosterTab() {
    val scope = rememberCoroutineScope()
    var units by remember { mutableStateOf<List<LecturerClient.Unit>>(emptyList()) }
    var unitFilter by remember { mutableStateOf<String?>(null) }
    var attendedOnly by remember { mutableStateOf(false) }
    var sortBy by remember { mutableStateOf("name") }   // name | cohort | unit | attended
    var rows by remember { mutableStateOf<List<LecturerClient.RosterRow>?>(null) }
    var sessions by remember { mutableStateOf<List<LecturerClient.Session>>(emptyList()) }
    var openSession by remember { mutableStateOf<LecturerClient.Session?>(null) }
    // Daily shift: default to TODAY's sessions only (the whole day's records are kept server-side,
    // but the roster shows today's shift and rolls over each day). Toggle off to see history.
    var todayOnly by remember { mutableStateOf(true) }

    fun reload() {
        rows = null
        scope.launch {
            val c = LecturerClient()
            if (units.isEmpty()) units = c.units()
            rows = c.roster(if (attendedOnly) "attended" else "enrolled", unitFilter)
            sessions = c.sessions(unitFilter)
        }
    }
    LaunchedEffect(unitFilter, attendedOnly) { reload() }

    if (openSession != null) SessionAttendanceDialog(openSession!!) { openSession = null }

    Column(Modifier.fillMaxSize().padding(horizontal = 16.dp)) {
        // Unit filter (if he handles more than one).
        if (units.size > 1) {
            LazyRowChips(
                labelFor = { it?.name ?: "All units" },
                options = listOf<LecturerClient.Unit?>(null) + units,
                selected = unitFilter,
                idOf = { it?.unitId },
                onSelect = { unitFilter = it?.unitId },
            )
        }
        Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(vertical = 4.dp)) {
            FilterChip(selected = !attendedOnly, onClick = { attendedOnly = false }, label = { Text("All who study") })
            Spacer(Modifier.width(8.dp))
            FilterChip(selected = attendedOnly, onClick = { attendedOnly = true }, label = { Text("Attended only") })
        }
        Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(bottom = 4.dp)) {
            Text("Sort:", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            listOf("name" to "Name", "cohort" to "Cohort", "unit" to "Unit", "attended" to "Attended").forEach { (k, lbl) ->
                TextButton(onClick = { sortBy = k }, contentPadding = PaddingValues(horizontal = 8.dp)) {
                    Text(lbl, fontWeight = if (sortBy == k) FontWeight.Bold else FontWeight.Normal,
                        color = if (sortBy == k) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
        }

        val sorted = (rows ?: emptyList()).sortedWith(compareBy {
            when (sortBy) { "cohort" -> it.cohort; "unit" -> it.unitName; "attended" -> "%05d".format(9999 - it.attendedCount); else -> it.fullName }
        })
        when {
            rows == null -> Box(Modifier.fillMaxWidth().padding(top = 40.dp), Alignment.Center) { CircularProgressIndicator() }
            sorted.isEmpty() -> Text("No students to show.", color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(top = 12.dp))
            else -> LazyColumn(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                item { Text("${sorted.size} student rows", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                items(sorted) { r ->
                    Surface(color = MaterialTheme.colorScheme.surfaceVariant, shape = MaterialTheme.shapes.small) {
                        Column(Modifier.fillMaxWidth().padding(10.dp)) {
                            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                                Text(r.fullName, fontWeight = FontWeight.SemiBold, modifier = Modifier.weight(1f))
                                Text("${r.attendedCount}×", fontWeight = FontWeight.Bold, color = MaterialTheme.colorScheme.primary)
                            }
                            Text("${r.studentId} · ${r.unitName}", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            if (r.cohort.isNotBlank()) Text(r.cohort, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    }
                }
                val today = java.time.LocalDate.now().toString()
                val shownSessions = sessions
                    .filter { !todayOnly || it.date == today }
                    .sortedWith(compareBy({ it.unitName }, { it.date }))   // grouped by unit, never mixed
                item {
                    Spacer(Modifier.height(10.dp))
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Text("Sessions", fontWeight = FontWeight.Bold, modifier = Modifier.weight(1f))
                        FilterChip(todayOnly, { todayOnly = true }, { Text("Today") }, modifier = Modifier.padding(end = 6.dp))
                        FilterChip(!todayOnly, { todayOnly = false }, { Text("All days") })
                    }
                    if (unitFilter == null && units.size > 1)
                        Text("Tip: pick a unit above to see just that unit's sessions.",
                            style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                if (shownSessions.isEmpty()) item { Text(if (todayOnly) "No sessions today yet." else "No sessions.", color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(top = 6.dp)) }
                if (shownSessions.isNotEmpty()) {
                    items(shownSessions) { s ->
                        Surface(color = MaterialTheme.colorScheme.surfaceVariant, shape = MaterialTheme.shapes.small, onClick = { openSession = s }) {
                            Row(Modifier.fillMaxWidth().padding(10.dp), verticalAlignment = Alignment.CenterVertically) {
                                Column(Modifier.weight(1f)) {
                                    Text("${s.unitName} · ${s.date}", fontWeight = FontWeight.SemiBold)
                                    if (s.cohort.isNotBlank()) Text(s.cohort, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                }
                                Text("${s.present}/${s.enrolled}", fontWeight = FontWeight.Bold)
                            }
                        }
                    }
                }
            }
        }
    }
}

/** Full-screen, Excel-style roster for ONE session: three columns (Reg no · Student name · Status),
 *  a green ✓ for present and a red ✗ for absent, present/absent/all filtering, and PDF/CSV export.
 *  It always loads the FULL list (so present + absent are both shown and exportable); the chips only
 *  change what's displayed. */
@Composable
private fun SessionAttendanceDialog(session: LecturerClient.Session, onClose: () -> Unit) {
    var view by remember { mutableStateOf("all") }   // present | absent | all (display filter)
    var all by remember { mutableStateOf<List<LecturerClient.SessStudent>?>(null) }
    LaunchedEffect(session.sessionId) { all = LecturerClient().sessionStudents(session.sessionId, "all") }

    Dialog(onDismissRequest = onClose, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Surface(Modifier.fillMaxSize()) {
            Column(Modifier.fillMaxSize().padding(16.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text(session.unitName.ifBlank { session.unitId }, fontWeight = FontWeight.Bold)
                        Text(listOf(session.cohort, session.date).filter { it.isNotBlank() }.joinToString(" · "),
                            style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    ExportRosterButton(
                        baseName = "roster_${session.unitName.ifBlank { session.unitId }}_${session.date}".replace(Regex("[^A-Za-z0-9_-]"), "_"),
                        title = "${session.unitName.ifBlank { session.unitId }} — ${session.date}",
                    ) { (all ?: emptyList()).map { RosterLine(it.studentId, it.fullName, it.present) } }
                    TextButton(onClick = onClose) { Text("Close") }
                }
                Spacer(Modifier.height(4.dp))
                Row {
                    listOf("all" to "All", "present" to "Present", "absent" to "Absent").forEach { (k, lbl) ->
                        FilterChip(view == k, { view = k }, { Text(lbl) }, modifier = Modifier.padding(end = 6.dp))
                    }
                }
                val rows = (all ?: emptyList()).filter { when (view) { "present" -> it.present; "absent" -> !it.present; else -> true } }
                    .sortedWith(compareBy({ !it.present }, { it.fullName }))
                Text("${(all ?: emptyList()).count { it.present }} present · ${(all ?: emptyList()).count { !it.present }} absent",
                    style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(vertical = 6.dp))
                // Header row (the three Excel columns).
                Row(Modifier.fillMaxWidth().padding(vertical = 4.dp)) {
                    Text("Reg no", Modifier.weight(1.1f), fontSize = 11.sp, fontWeight = FontWeight.Bold, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Text("Student name", Modifier.weight(1.5f), fontSize = 11.sp, fontWeight = FontWeight.Bold, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Text("Status", Modifier.weight(0.5f), fontSize = 11.sp, fontWeight = FontWeight.Bold, color = MaterialTheme.colorScheme.onSurfaceVariant, textAlign = TextAlign.Center)
                }
                HorizontalDivider()
                when {
                    all == null -> Box(Modifier.fillMaxWidth().padding(top = 40.dp), Alignment.Center) { CircularProgressIndicator() }
                    rows.isEmpty() -> Text("No students.", color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(top = 12.dp))
                    else -> LazyColumn(Modifier.weight(1f)) {
                        items(rows) { s ->
                            Row(Modifier.fillMaxWidth().padding(vertical = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                                Text(s.studentId, Modifier.weight(1.1f), fontSize = 12.sp, fontFamily = FontFamily.Monospace)
                                Text(s.fullName, Modifier.weight(1.5f), fontSize = 13.sp)
                                Box(Modifier.weight(0.5f), contentAlignment = Alignment.Center) {
                                    if (s.present) Text("✓", color = MaterialTheme.colorScheme.primary, fontWeight = FontWeight.Bold, fontSize = 18.sp)
                                    else Text("✗", color = MaterialTheme.colorScheme.error, fontWeight = FontWeight.Bold, fontSize = 18.sp)
                                }
                            }
                            HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.4f))
                        }
                    }
                }
            }
        }
    }
}

/** Inbox from the coordinator, and a composer to notify his students or the coordinator. */
@Composable
private fun LecturerAlertsTab() {
    val scope = rememberCoroutineScope()
    var inbox by remember { mutableStateOf<List<NotificationClient.Notif>?>(null) }
    var composing by remember { mutableStateOf(false) }
    fun load() { scope.launch { inbox = NotificationClient().inbox() } }
    LaunchedEffect(Unit) { load() }

    Column(Modifier.fillMaxSize().padding(16.dp)) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            Text("Notifications", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold, modifier = Modifier.weight(1f))
            Button(onClick = { composing = !composing }) { Text(if (composing) "Close" else "✎ New") }
        }
        if (composing) NotificationComposer(
            audiences = listOf("STUDENTS" to "My students", "COORDINATOR" to "Coordinator"),
            onSent = { composing = false; load() },
        )
        Spacer(Modifier.height(8.dp))
        NotificationInboxList(inbox) { load() }
    }
}

@Composable
private fun LecturerProfileTab(onChangePw: () -> Unit) {
    Column(Modifier.fillMaxSize().padding(24.dp)) {
        Text("Profile", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(12.dp))
        val titled = listOfNotNull(
            AppState.coordinatorTitle?.takeIf { it.isNotBlank() },
            AppState.coordinatorName?.takeIf { it.isNotBlank() },
        ).joinToString(" ")
        if (titled.isNotBlank()) Text(titled, fontWeight = FontWeight.SemiBold)
        AppState.staffId?.let { Text("Staff ID: $it", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        AppState.role?.let { Text(it, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        Spacer(Modifier.height(24.dp))
        OutlinedButton(onClick = onChangePw, Modifier.fillMaxWidth()) { Text("🔑  Change password") }
        Spacer(Modifier.height(8.dp))
        Button(onClick = { signOut() }, Modifier.fillMaxWidth()) { Text("Sign out") }
    }
}
