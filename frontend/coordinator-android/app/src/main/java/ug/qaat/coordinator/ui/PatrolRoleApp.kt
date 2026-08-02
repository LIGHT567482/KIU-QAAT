package ug.qaat.coordinator.ui

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
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
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import ug.qaat.coordinator.db.PatrolLogEntity
import ug.qaat.coordinator.db.PatrolSlotEntity
import ug.qaat.coordinator.di.Graph
import ug.qaat.coordinator.net.NotificationClient
import ug.qaat.coordinator.net.PatrolClient
import ug.qaat.coordinator.store.SessionStore
import ug.qaat.coordinator.student.Fingerprint
import java.time.Instant
import java.time.LocalDate
import java.time.LocalTime
import java.util.UUID

/**
 * The QA PATROLLER experience, inside the one KIU QAAT app.
 *
 * This used to be a second APK (`ug.qaat.patroller`) that patrollers had to be sent separately,
 * with its own login, its own copy of the networking, and its own unencrypted database. It is now
 * a role branch like the student's and the lecturer's: same install, same sign-in, same proven
 * TLS stack — so it runs wherever the main app runs, which is every phone it has been put on.
 *
 * Folding it in removes the second app but not the separation, which is enforced in three places:
 *
 *  1. **Role routing** — [RootApp] sends `QA_PATROLLER` here and nowhere else. There is no path
 *     from this screen into the coordinator hub, the lecturer roster, or a student's record.
 *  2. **Handset binding** — the round is gated on [DeviceGate]. The gateway ties the patroller's
 *     account to the first phone that claims it and refuses patrol calls from any other, so a
 *     token lifted off this device buys an attacker nothing.
 *  3. **A PIN** — [PatrolPinGate], the second page after sign-in. The binding proves WHICH phone;
 *     the PIN proves WHO is holding it, which is the half a shared password defeats.
 *  4. **No silent re-login** — [PatrolRoleApp] drops the saved credentials the moment it opens.
 *     Every other role may resume without retyping a password; a patroller may not.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PatrolRoleApp() {
    val ctx = LocalContext.current
    val navColor = navBarColor(AppState.branding)
    val onNav = navColor?.let { onNavColor(it) }
    var tab by remember { mutableStateOf(0) }
    var unread by remember { mutableStateOf(0) }
    var reloadKey by remember { mutableStateOf(0) }
    var showChangePw by remember { mutableStateOf(false) }
    if (showChangePw) ChangePasswordDialog(onClose = { showChangePw = false })

    // A patroller's session is never resumable without a password. The credentials the shared
    // login screen saved for silent re-login are erased as soon as we know the role is this one,
    // so a phone that is picked up, lost or handed on cannot walk back into a patrol round.
    LaunchedEffect(Unit) { SessionStore.clearAppCredentials() }

    LaunchedEffect(tab, reloadKey) { runCatching { unread = NotificationClient().unread() } }

    // Device first, then person. Checking the handset before asking for the PIN means a patroller
    // on the wrong phone is told so immediately, rather than typing a PIN that was never going to
    // be accepted.
    DeviceGate {
      PatrolPinGate {
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
                            BrandLogo(AppState.branding, size = 30); Spacer(Modifier.width(8.dp))
                            Text("KIU QAAT", fontWeight = FontWeight.Bold)
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
                    NavigationBarItem(tab == 0, { tab = 0 }, icon = { TabGlyph(NavIcons.Patrol, "Patrol") }, label = { Text("Patrol") }, colors = itemColors)
                    NavigationBarItem(tab == 1, { tab = 1 }, colors = itemColors, label = { Text("Alerts") },
                        icon = { if (unread > 0) BadgedBox(badge = { Badge { Text("$unread") } }) { TabGlyph(NavIcons.Alerts, "Alerts") } else TabGlyph(NavIcons.Alerts, "Alerts") })
                    NavigationBarItem(tab == 2, { tab = 2 }, icon = { TabGlyph(NavIcons.Profile, "Profile") }, label = { Text("Profile") }, colors = itemColors)
                }
            },
        ) { pad ->
            Box(Modifier.padding(pad).fillMaxSize()) {
                when (tab) {
                    0 -> PatrolRoundTab(reloadKey)
                    1 -> PatrolAlertsTab()
                    else -> PatrolProfileTab(ctx, onChangePw = { showChangePw = true })
                }
            }
        }
      }
    }
}

// ── Handset binding ─────────────────────────────────────────────────────────────

private sealed interface GateState {
    data object Checking : GateState
    data object Allowed : GateState
    data class Refused(val message: String) : GateState
    data class Offline(val message: String) : GateState
}

/**
 * Claims this handset for the signed-in patroller before any patrol screen is shown, and blocks
 * the round outright if the server says this is not their phone.
 *
 * Offline is deliberately NOT a refusal: a patroller walking a corridor with no signal must still
 * be able to tick a room. The binding was proved when they signed in, and nothing they record
 * offline reaches the server until a sync — which is itself bound-checked. So a failure to reach
 * the gateway falls through to the round with a warning, while an actual 403 locks it.
 */
@Composable
private fun DeviceGate(content: @Composable () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var state by remember { mutableStateOf<GateState>(GateState.Checking) }

    suspend fun claim() {
        val token = AppState.token
        if (token == null) { state = GateState.Refused("Your session has ended. Sign in again."); return }
        val fp = Fingerprint.get(ctx)
        runCatching { PatrolClient().bindDevice(token, fp) }
            .onSuccess { rejection ->
                state = if (rejection == null) GateState.Allowed else GateState.Refused(rejection)
            }
            .onFailure { state = GateState.Offline(ug.qaat.coordinator.net.Net.friendly(it)) }
    }
    LaunchedEffect(Unit) { claim() }

    when (val s = state) {
        is GateState.Checking -> Box(Modifier.fillMaxSize(), Alignment.Center) {
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                CircularProgressIndicator()
                Text("Checking this phone…", Modifier.padding(top = 12.dp),
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
        is GateState.Refused -> PatrolLockedScreen(s.message)
        is GateState.Allowed -> content()
        is GateState.Offline -> Column(Modifier.fillMaxSize()) {
            Surface(color = MaterialTheme.colorScheme.errorContainer, modifier = Modifier.fillMaxWidth()) {
                Row(Modifier.padding(10.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text("Offline — this phone could not be re-checked. Your ticks are saved and will sync later.",
                        Modifier.weight(1f), style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onErrorContainer)
                    TextButton(onClick = { scope.launch { state = GateState.Checking; claim() } }) { Text("Retry") }
                }
            }
            Box(Modifier.weight(1f)) { content() }
        }
    }
}

/** Dead end for a handset the server will not accept. The only way out is signing out. */
@Composable
private fun PatrolLockedScreen(message: String) {
    val ctx = LocalContext.current
    Box(Modifier.fillMaxSize().padding(28.dp), Alignment.Center) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Text("🔒", fontSize = 40.sp)
            Spacer(Modifier.height(12.dp))
            Text("Patrol is locked on this phone", style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.Bold, textAlign = TextAlign.Center)
            Spacer(Modifier.height(8.dp))
            Text(message, textAlign = TextAlign.Center, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.height(8.dp))
            Text("A patrol account works on one registered handset. If you have changed phones, ask an administrator to release your device binding.",
                textAlign = TextAlign.Center, style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.height(24.dp))
            SignOutButton()
        }
    }
}

// ── The round ───────────────────────────────────────────────────────────────────

private fun today() = LocalDate.now().toString()

/** Push every queued observation. Returns the message to show, or null when all is well. */
private suspend fun syncPending(ctx: android.content.Context): String? {
    val token = AppState.token ?: return null
    val dao = Graph.db.dao()
    val pending = withContext(Dispatchers.IO) { dao.unsyncedPatrolLogs() }
    if (pending.isEmpty()) return null
    return runCatching { PatrolClient().sync(token, Fingerprint.get(ctx), pending) }
        .fold(
            onSuccess = { ok ->
                if (ok) withContext(Dispatchers.IO) { pending.forEach { dao.markPatrolLogSynced(it.id) } }
                if (ok) null else "The server did not accept the round — it stays saved and will retry."
            },
            onFailure = { if (it is PatrolClient.DeviceRejected) it.message else null },
        )
}

@Composable
private fun PatrolRoundTab(reloadKey: Int) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val dao = remember { Graph.db.dao() }
    var slots by remember { mutableStateOf<List<PatrolSlotEntity>>(emptyList()) }
    var loading by remember { mutableStateOf(true) }
    var note by remember { mutableStateOf<String?>(null) }
    var pending by remember { mutableStateOf(0) }
    val logs by dao.patrolLogsForDay(today()).collectAsStateWithLifecycle(emptyList())

    suspend fun refresh() {
        loading = true
        // Cached day first so the screen is usable instantly and offline, then the network.
        slots = withContext(Dispatchers.IO) { dao.patrolSlots() }
        val token = AppState.token
        if (token != null) {
            runCatching { PatrolClient().manifest(token, Fingerprint.get(ctx)) }
                .onSuccess { fresh ->
                    withContext(Dispatchers.IO) { dao.replacePatrolSlots(fresh) }
                    slots = fresh; note = null
                }
                .onFailure {
                    note = when {
                        it is PatrolClient.DeviceRejected -> it.message
                        slots.isEmpty() -> "Offline — no timetable cached yet. Connect once to download today's rounds."
                        else -> null
                    }
                }
        }
        syncPending(ctx)?.let { note = it }
        pending = withContext(Dispatchers.IO) { dao.pendingPatrolCount() }
        loading = false
    }
    LaunchedEffect(reloadKey) { refresh() }

    // Slots overlapping "now" (from 10 minutes before the start to the end) float up, marked NOW.
    val nowMin = LocalTime.now().let { it.hour * 60 + it.minute }
    fun startMin(s: String): Int {
        val p = s.split(":")
        return (p.getOrNull(0)?.toIntOrNull() ?: 0) * 60 + (p.getOrNull(1)?.toIntOrNull() ?: 0)
    }
    fun isNow(s: PatrolSlotEntity): Boolean {
        val st = startMin(s.startTime); return nowMin in (st - 10)..(st + s.durationMinutes)
    }
    val ordered = slots.sortedWith(compareByDescending<PatrolSlotEntity> { isNow(it) }.thenBy { it.startTime })
    val done = logs.associateBy { it.unitId + "@" + it.scheduledTime }

    fun tick(s: PatrolSlotEntity, taught: Boolean) = scope.launch {
        withContext(Dispatchers.IO) {
            dao.putPatrolLog(PatrolLogEntity(
                id = UUID.randomUUID().toString(),
                unitId = s.unitId, unitName = s.unitName, courseCode = s.courseCode,
                lecturerId = s.lecturerStaffId, lecturerName = s.lecturerName, room = s.room,
                sessionDate = today(), scheduledTime = s.startTime, taught = taught,
                takenAt = Instant.now().toString(),
            ))
        }
        syncPending(ctx)?.let { note = it }
        pending = withContext(Dispatchers.IO) { dao.pendingPatrolCount() }
    }

    Column(Modifier.fillMaxSize().padding(16.dp)) {
        Column(Modifier.fillMaxWidth()) {
            Text("Patrol — ${AppState.coordinatorName.orEmpty().ifBlank { "QA" }}", fontWeight = FontWeight.Bold, fontSize = 18.sp)
            Text(
                "Staff ID: ${AppState.staffId.orEmpty().ifBlank { "—" }}" +
                    if (pending > 0) "  ·  $pending pending sync" else "  ·  all synced",
                style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        Text("Tick whether the timetabled lecturer is actually teaching. Works offline — ticks sync automatically.",
            style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(vertical = 6.dp))
        note?.let {
            Surface(color = MaterialTheme.colorScheme.errorContainer, shape = MaterialTheme.shapes.small,
                modifier = Modifier.fillMaxWidth()) {
                Text(it, Modifier.padding(10.dp), style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onErrorContainer)
            }
        }

        when {
            loading && slots.isEmpty() -> Box(Modifier.fillMaxWidth().padding(top = 40.dp), Alignment.Center) { CircularProgressIndicator() }
            ordered.isEmpty() -> Text("No timetabled sessions for today.",
                color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(top = 20.dp))
            else -> LazyColumn(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                items(ordered) { s ->
                    PatrolSlotCard(s, isNow(s), done[s.unitId + "@" + s.startTime]) { taught -> tick(s, taught) }
                }
            }
        }
    }
}

@Composable
private fun PatrolSlotCard(
    s: PatrolSlotEntity,
    live: Boolean,
    recorded: PatrolLogEntity?,
    onTick: (Boolean) -> Unit,
) {
    Surface(
        color = if (live) MaterialTheme.colorScheme.secondaryContainer else MaterialTheme.colorScheme.surfaceVariant,
        shape = MaterialTheme.shapes.medium,
    ) {
        Column(Modifier.fillMaxWidth().padding(12.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(s.startTime, fontWeight = FontWeight.Bold, fontFamily = FontFamily.Monospace)
                if (live) {
                    Spacer(Modifier.width(8.dp))
                    Text("● NOW", color = MaterialTheme.colorScheme.primary,
                        style = MaterialTheme.typography.labelSmall, fontWeight = FontWeight.Bold)
                }
            }
            Text(s.unitName.ifBlank { s.unitId } + if (s.courseCode.isNotBlank()) "  (${s.courseCode})" else "",
                fontWeight = FontWeight.SemiBold)
            Text("Lecturer: ${s.lecturerName.ifBlank { s.lecturerStaffId.ifBlank { "—" } }}",
                style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Text("Room: ${s.room.ifBlank { "—" }}",
                style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.height(8.dp))
            if (recorded != null) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(if (recorded.taught) "✓ Marked TAUGHT" else "✗ Marked NOT TAUGHT",
                        color = if (recorded.taught) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error,
                        fontWeight = FontWeight.Bold, modifier = Modifier.weight(1f))
                    if (!recorded.synced) Text("queued", style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            } else {
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Button(modifier = Modifier.weight(1f), onClick = { onTick(true) }) { Text("Taught ✓") }
                    OutlinedButton(modifier = Modifier.weight(1f), onClick = { onTick(false) }) { Text("Not taught ✗") }
                }
            }
        }
    }
}

// ── Alerts + profile ────────────────────────────────────────────────────────────

/** Inbox only. A patroller reports by ticking a round, not by messaging staff directly, so there
 *  is deliberately no composer here — nothing they send could be attributed or acted on. */
@Composable
private fun PatrolAlertsTab() {
    val scope = rememberCoroutineScope()
    var inbox by remember { mutableStateOf<List<NotificationClient.Notif>?>(null) }
    fun load() { scope.launch { inbox = NotificationClient().inbox() } }
    LaunchedEffect(Unit) { load() }

    Column(Modifier.fillMaxSize().padding(16.dp)) {
        Text("Notifications", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(8.dp))
        NotificationInboxList(inbox) { load() }
    }
}

@Composable
private fun PatrolProfileTab(ctx: android.content.Context, onChangePw: () -> Unit) {
    Column(Modifier.fillMaxSize().padding(24.dp)) {
        Text("Profile", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(12.dp))
        AppState.coordinatorName?.takeIf { it.isNotBlank() }?.let { Text(it, fontWeight = FontWeight.SemiBold) }
        AppState.staffId?.let { Text("Staff ID: $it", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        Text("QA Patroller", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.height(16.dp))
        Surface(color = MaterialTheme.colorScheme.surfaceVariant, shape = MaterialTheme.shapes.small) {
            Text("This phone is registered to your patrol account. Rounds recorded on any other handset are rejected.",
                Modifier.padding(10.dp), style = MaterialTheme.typography.labelSmall)
        }
        Spacer(Modifier.height(24.dp))
        OutlinedButton(onClick = onChangePw, Modifier.fillMaxWidth()) { Text("🔑  Change password") }
        Spacer(Modifier.height(8.dp))
        // The PIN is changed from inside the round, where the patroller has already proved they
        // know the current one — so a handset left unlocked cannot be used to replace it.
        var changePin by remember { mutableStateOf(false) }
        OutlinedButton(onClick = { changePin = true }, Modifier.fillMaxWidth()) { Text("🛡  Change patrol PIN") }
        if (changePin) ChangePatrolPinDialog { changePin = false }
        Spacer(Modifier.height(8.dp))
        SignOutButton()
    }
}

// patrolSignOut used to live here, wiping the patrol round before delegating to the old signOut().
// Erasing the round is right — a patrol log names lecturers and must not outlive the session that
// produced it on a shared or surrendered handset — but it was the ONLY role that cleaned up after
// itself. That wipe is now part of the shared teardown (AppDao.clearAllForSignOut), so every role
// gets it, and this role also gets the unsynced-attendance warning it never had.
