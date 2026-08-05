package ug.qaat.coordinator.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
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
                            BrandHeader(AppState.branding)
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

    // SEARCH FIRST. The round used to open on every timetabled session in the institution,
    // each with a pair of tick buttons — a screen that invites ticking without visiting,
    // since the lecturer, room and time are all on it and nothing distinguishes a slot the
    // patroller stood in front of from one they scrolled past. A patrol record is weighed
    // against the coordinator's own precisely because it comes from someone who was there,
    // so the patroller now looks a lecturer or unit up before anything appears.
    var mode by remember { mutableStateOf("lecturer") }   // "lecturer" | "unit"
    var query by remember { mutableStateOf("") }
    var results by remember { mutableStateOf<List<PatrolSlotEntity>?>(null) }   // null = nothing searched yet
    var searching by remember { mutableStateOf(false) }
    var chosen by remember { mutableStateOf<PatrolSlotEntity?>(null) }
    var note by remember { mutableStateOf<String?>(null) }
    var pending by remember { mutableStateOf(0) }
    val logs by dao.patrolLogsForDay(today()).collectAsStateWithLifecycle(emptyList())

    // The cached day still refreshes in the background: it is what makes the search work
    // with no signal, which is most of a round in a concrete building.
    suspend fun refreshCache() {
        val token = AppState.token
        if (token != null) {
            runCatching { PatrolClient().manifest(token, Fingerprint.get(ctx)) }
                .onSuccess { fresh -> withContext(Dispatchers.IO) { dao.replacePatrolSlots(fresh) } }
                .onFailure { if (it is PatrolClient.DeviceRejected) note = it.message }
        }
        syncPending(ctx)?.let { note = it }
        pending = withContext(Dispatchers.IO) { dao.pendingPatrolCount() }
    }
    LaunchedEffect(reloadKey) { refreshCache() }

    fun runSearch() = scope.launch {
        val q = query.trim()
        if (q.isEmpty()) { results = null; return@launch }
        searching = true; note = null
        val token = AppState.token
        val online = if (token != null) {
            runCatching { PatrolClient().search(token, Fingerprint.get(ctx), mode, q) }
                .onFailure { if (it is PatrolClient.DeviceRejected) note = it.message }
                .getOrNull()
        } else null

        results = if (online != null) online else {
            // Offline: search the cached day. Same matching rule as the server — prefix,
            // case-insensitive, staff id or name — so the patroller does not get a
            // different answer depending on whether the signal happened to be up.
            note = "Offline — searching today's cached timetable."
            val needle = q.lowercase()
            // TODAY's slots out of the cached week. The cache used to hold whichever single day
            // the phone last had signal on, so a patroller who refreshed on Monday walked
            // Tuesday's round against Monday's timetable — wrong lecturers, wrong rooms, and
            // nothing on screen to say so. The whole week is cached now and narrowed here.
            val dow = LocalDate.now().dayOfWeek.value
            withContext(Dispatchers.IO) { dao.patrolSlotsForDay(dow) }.filter { s ->
                if (mode == "lecturer")
                    s.lecturerStaffId.lowercase().startsWith(needle) || s.lecturerName.lowercase().startsWith(needle)
                else
                    s.unitId.lowercase().startsWith(needle) || s.unitName.lowercase().startsWith(needle)
            }.sortedBy { it.startTime }
        }
        searching = false
    }

    fun tick(s: PatrolSlotEntity, taught: Boolean, found: FoundElsewhere?) = scope.launch {
        withContext(Dispatchers.IO) {
            dao.putPatrolLog(PatrolLogEntity(
                id = UUID.randomUUID().toString(),
                unitId = s.unitId, unitName = s.unitName, courseCode = s.courseCode,
                lecturerId = s.lecturerStaffId, lecturerName = s.lecturerName, room = s.room,
                sessionDate = today(), scheduledTime = s.startTime, taught = taught,
                takenAt = Instant.now().toString(),
                offeringId = s.offeringId,
                foundVenue = found?.venue.orEmpty(),
                foundStartTime = found?.time.orEmpty(),
                foundDate = found?.date.orEmpty(),
                venueChanged = found != null,
                remarks = found?.remarks.orEmpty(),
            ))
        }
        chosen = null
        syncPending(ctx)?.let { note = it }
        pending = withContext(Dispatchers.IO) { dao.pendingPatrolCount() }
    }

    // Keyed by COHORT as well as unit and time, matching the server's ux_patrol_logs_slot. Two
    // intakes can run the same unit at the same hour in different rooms; without the offering,
    // ticking one of them showed the OTHER as "already marked TAUGHT" — so the patroller would
    // walk past a lecture believing it had been recorded, and the one that was never visited
    // carried a verdict nobody had witnessed.
    fun slotKey(unitId: String, time: String, offeringId: String) = "$unitId@$time#$offeringId"
    val done = logs.associateBy { slotKey(it.unitId, it.scheduledTime, it.offeringId) }

    // The confirm sheet: one lecture, one green tick, one red cross.
    chosen?.let { slot ->
        PatrolConfirmSheet(
            slot = slot,
            onDismiss = { chosen = null },
            onDecide = { taught, found -> tick(slot, taught, found) },
        )
    }

    Column(Modifier.fillMaxSize().padding(16.dp)) {
        Text("Patrol — ${AppState.coordinatorName.orEmpty().ifBlank { "QA" }}", fontWeight = FontWeight.Bold, fontSize = 18.sp)
        Text(
            "Staff ID: ${AppState.staffId.orEmpty().ifBlank { "—" }}" +
                if (pending > 0) "  ·  $pending pending sync" else "  ·  all synced",
            style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Text("Search the lecturer or the unit you are standing in front of, then record whether they are teaching. Works offline.",
            style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(vertical = 6.dp))

        // Which way to search. Both find the same lectures; the patroller uses whichever
        // identifier the door or the badge in front of them happens to carry.
        Row(Modifier.fillMaxWidth().padding(bottom = 8.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            FilterChip(selected = mode == "lecturer", onClick = { mode = "lecturer"; results = null },
                label = { Text("By lecturer ID") })
            FilterChip(selected = mode == "unit", onClick = { mode = "unit"; results = null },
                label = { Text("By unit code") })
        }
        OutlinedTextField(
            value = query,
            onValueChange = { query = it },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
            label = { Text(if (mode == "lecturer") "Lecturer staff ID or name" else "Unit code or name") },
            placeholder = { Text(if (mode == "lecturer") "e.g. KIU/044" else "e.g. CS201") },
            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
            keyboardActions = KeyboardActions(onSearch = { runSearch() }),
            trailingIcon = {
                TextButton(onClick = { runSearch() }, enabled = query.isNotBlank() && !searching) {
                    Text(if (searching) "…" else "Search")
                }
            },
        )

        note?.let {
            Spacer(Modifier.height(8.dp))
            Surface(color = MaterialTheme.colorScheme.errorContainer, shape = MaterialTheme.shapes.small,
                modifier = Modifier.fillMaxWidth()) {
                Text(it, Modifier.padding(10.dp), style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onErrorContainer)
            }
        }
        Spacer(Modifier.height(12.dp))

        // Read the hits into a local before branching. The LazyColumn's content lambda runs
        // later, outside this composition, and by then the state can have been cleared back
        // to null — a filter chip or an emptied query does exactly that — so a `results!!`
        // inside the lambda is a crash waiting for the patroller's next tap.
        val hits = results
        when {
            searching -> Box(Modifier.fillMaxWidth().padding(top = 40.dp), Alignment.Center) { CircularProgressIndicator() }
            // Nothing searched yet is NOT the same as nothing found, and must not read as it.
            hits == null -> Text(
                "No lecturer is shown until you search. Enter the staff ID or the unit code for the room you are at.",
                color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(top = 20.dp))
            hits.isEmpty() -> Text(
                "Nothing timetabled today matches “${query.trim()}”. Check the ID, or try searching the other way.",
                color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(top = 20.dp))
            else -> LazyColumn(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                items(hits) { s ->
                    PatrolResultCard(s, done[slotKey(s.unitId, s.startTime, s.offeringId)]) { chosen = s }
                }
            }
        }
    }
}

/** What the patroller found, when it was not where the timetable said. */
private data class FoundElsewhere(
    val venue: String,
    val time: String,
    val date: String,
    val remarks: String,
)

/** One search hit. Tapping it opens the confirm sheet — the card itself cannot tick, so a
 *  verdict is always two deliberate actions rather than one thumb on a scrolling list. */
@Composable
private fun PatrolResultCard(
    s: PatrolSlotEntity,
    recorded: PatrolLogEntity?,
    onOpen: () -> Unit,
) {
    Surface(
        color = MaterialTheme.colorScheme.surfaceVariant,
        shape = MaterialTheme.shapes.medium,
        onClick = onOpen,
    ) {
        Column(Modifier.fillMaxWidth().padding(12.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(s.startTime, fontWeight = FontWeight.Bold, fontFamily = FontFamily.Monospace)
                Spacer(Modifier.width(8.dp))
                Text(s.unitName.ifBlank { s.unitId } + if (s.courseCode.isNotBlank()) "  (${s.courseCode})" else "",
                    fontWeight = FontWeight.SemiBold)
            }
            Text("Lecturer: ${s.lecturerName.ifBlank { s.lecturerStaffId.ifBlank { "—" } }}",
                style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Text("Room: ${s.room.ifBlank { "—" }}",
                style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            if (s.cohort.isNotBlank()) {
                Text(s.cohort, style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            if (recorded != null) {
                Spacer(Modifier.height(6.dp))
                Text(
                    (if (recorded.taught) "✓ Already marked TAUGHT" else "✗ Already marked NOT TAUGHT") +
                        if (!recorded.synced) " · queued" else "",
                    color = if (recorded.taught) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error,
                    style = MaterialTheme.typography.labelSmall, fontWeight = FontWeight.Bold,
                )
            }
        }
    }
}

/**
 * The verdict screen: is this lecturer teaching, yes or no.
 *
 * The "found somewhere else" section is collapsed by default because the lecture being where
 * the timetable said is the ordinary case. When it is not, the patroller is the only person
 * who knows — so recording the real room, time or date here is what turns a moved lecture
 * from a false "not taught" into a fact somebody can act on.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun PatrolConfirmSheet(
    slot: PatrolSlotEntity,
    onDismiss: () -> Unit,
    onDecide: (taught: Boolean, found: FoundElsewhere?) -> Unit,
) {
    var moved by remember { mutableStateOf(false) }
    var venue by remember { mutableStateOf(slot.room) }
    var time by remember { mutableStateOf(slot.startTime) }
    var date by remember { mutableStateOf(today()) }
    var remarks by remember { mutableStateOf("") }

    // Only counts as a change if something actually differs — ticking the box and editing
    // nothing must not raise an alert telling a lecturer their lecture moved to where it was.
    fun found(): FoundElsewhere? {
        if (!moved) return null
        val changed = venue.trim() != slot.room.trim() ||
            time.trim() != slot.startTime.trim() ||
            date.trim() != today()
        return if (changed || remarks.isNotBlank())
            FoundElsewhere(venue.trim(), time.trim(), date.trim(), remarks.trim()) else null
    }

    ModalBottomSheet(onDismissRequest = onDismiss) {
        Column(Modifier.fillMaxWidth().padding(20.dp)) {
            Text(slot.unitName.ifBlank { slot.unitId }, fontWeight = FontWeight.Bold, fontSize = 18.sp)
            if (slot.courseCode.isNotBlank()) {
                Text(slot.courseCode, style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            Spacer(Modifier.height(6.dp))
            Text("Lecturer: ${slot.lecturerName.ifBlank { slot.lecturerStaffId.ifBlank { "—" } }}")
            Text("Timetabled: ${slot.room.ifBlank { "no room" }} at ${slot.startTime}",
                style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            if (slot.cohort.isNotBlank()) {
                Text(slot.cohort, style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
            }

            Spacer(Modifier.height(20.dp))
            Text("Is the lecturer teaching?", fontWeight = FontWeight.SemiBold)
            Spacer(Modifier.height(10.dp))
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                Button(
                    modifier = Modifier.weight(1f).height(56.dp),
                    onClick = { onDecide(true, found()) },
                ) { Text("✓  Yes", fontSize = 18.sp, fontWeight = FontWeight.Bold) }
                Button(
                    modifier = Modifier.weight(1f).height(56.dp),
                    colors = ButtonDefaults.buttonColors(containerColor = MaterialTheme.colorScheme.error),
                    onClick = { onDecide(false, found()) },
                ) { Text("✗  No", fontSize = 18.sp, fontWeight = FontWeight.Bold) }
            }

            Spacer(Modifier.height(18.dp))
            Row(verticalAlignment = Alignment.CenterVertically,
                modifier = Modifier.fillMaxWidth().clickable { moved = !moved }) {
                Checkbox(checked = moved, onCheckedChange = { moved = it })
                Column {
                    Text("Found somewhere else?", fontWeight = FontWeight.SemiBold)
                    Text("Different room, time or day — the lecturer, their HOD and QA are told.",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
            if (moved) {
                Spacer(Modifier.height(8.dp))
                OutlinedTextField(venue, { venue = it }, singleLine = true,
                    modifier = Modifier.fillMaxWidth(), label = { Text("Room where you found it") })
                Spacer(Modifier.height(8.dp))
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    OutlinedTextField(time, { time = it }, singleLine = true,
                        modifier = Modifier.weight(1f), label = { Text("Start (HH:MM)") })
                    OutlinedTextField(date, { date = it }, singleLine = true,
                        modifier = Modifier.weight(1f), label = { Text("Date") })
                }
                Spacer(Modifier.height(8.dp))
                OutlinedTextField(remarks, { remarks = it },
                    modifier = Modifier.fillMaxWidth(), label = { Text("Note (optional)") })
            }
            Spacer(Modifier.height(24.dp))
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
