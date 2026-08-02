package ug.qaat.patroller.ui

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import ug.qaat.patroller.AppState
import ug.qaat.patroller.db.PatrolDb
import ug.qaat.patroller.db.PatrolLogEntity
import ug.qaat.patroller.db.PatrolSlotEntity
import ug.qaat.patroller.net.Net
import ug.qaat.patroller.net.PatrolClient
import ug.qaat.patroller.store.Store
import java.time.LocalDate
import java.time.LocalTime
import java.util.UUID

private fun today() = LocalDate.now().toString()

/** Upload every queued (unsynced) patrol log; marks each synced on success. Safe anytime. */
suspend fun syncPending(ctx: android.content.Context): Boolean = withContext(Dispatchers.IO) {
    val token = AppState.token ?: return@withContext false
    val dao = PatrolDb.get(ctx).dao()
    val pending = dao.unsynced()
    if (pending.isEmpty()) return@withContext true
    val ok = runCatching { PatrolClient().sync(token, pending) }.getOrDefault(false)
    if (ok) pending.forEach { dao.markSynced(it.id) }
    ok
}

@Composable
fun PatrolRoot() {
    // Restore a saved session so the app opens straight to work.
    LaunchedEffect(Unit) {
        if (AppState.token == null) Store.token?.let {
            AppState.token = it; AppState.name = Store.name; AppState.staffId = Store.staffId
        }
    }
    MaterialTheme {
        Surface(Modifier.fillMaxSize()) {
            AppState.lastCrash?.let { trace ->
                AlertDialog(
                    onDismissRequest = { AppState.lastCrash = null },
                    confirmButton = { TextButton(onClick = { AppState.lastCrash = null }) { Text("Dismiss") } },
                    title = { Text("The app crashed last time") },
                    text = { Column(Modifier.heightIn(max = 360.dp).verticalScroll(rememberScrollState())) { Text(trace, fontSize = 11.sp, fontFamily = FontFamily.Monospace) } },
                )
            }
            if (AppState.loggedIn) PatrolScreen() else LoginScreen()
        }
    }
}

@Composable
private fun LoginScreen() {
    val scope = rememberCoroutineScope()
    var id by remember { mutableStateOf("") }
    var pw by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    // Pre-warm the sleeping free-tier backend while the user types.
    LaunchedEffect(Unit) { runCatching { Net.warmUp() } }

    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(24.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        Spacer(Modifier.height(48.dp))
        Text("KIU QAAT — Patrol", style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold)
        Text("QA patroller sign-in", color = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.height(24.dp))
        OutlinedTextField(id, { id = it.trim() }, label = { Text("Staff ID or email") }, singleLine = true, modifier = Modifier.fillMaxWidth())
        Spacer(Modifier.height(8.dp))
        OutlinedTextField(pw, { pw = it }, label = { Text("Password") }, singleLine = true, modifier = Modifier.fillMaxWidth())
        error?.let { Spacer(Modifier.height(8.dp)); Text(it, color = MaterialTheme.colorScheme.error) }
        Spacer(Modifier.height(16.dp))
        Button(
            enabled = !busy && id.isNotBlank() && pw.isNotBlank(),
            modifier = Modifier.fillMaxWidth(),
            onClick = {
                busy = true; error = null
                scope.launch {
                    runCatching { PatrolClient().login(id, pw) }
                        .onSuccess { r ->
                            if (r.role != "QA_PATROLLER") { error = "This app is for QA patrollers only (your role is ${r.role.ifBlank { "unknown" }})."; busy = false; return@launch }
                            Store.save(r.token, r.fullName, r.staffId, "", id, pw)
                            AppState.token = r.token; AppState.name = r.fullName; AppState.staffId = r.staffId
                            busy = false
                        }
                        .onFailure { error = Net.friendly(it); busy = false }
                }
            },
        ) { Text(if (busy) "Signing in…" else "Sign in") }
    }
}

@Composable
private fun PatrolScreen() {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val dao = remember { PatrolDb.get(ctx).dao() }
    var slots by remember { mutableStateOf<List<PatrolSlotEntity>>(emptyList()) }
    var loading by remember { mutableStateOf(true) }
    var note by remember { mutableStateOf<String?>(null) }
    var pending by remember { mutableStateOf(0) }
    val logs by dao.logsForDay(today()).collectAsStateWithLifecycle(emptyList())

    suspend fun refresh() {
        loading = true
        // Cached slots first (offline), then try the network to refresh + sync anything pending.
        slots = withContext(Dispatchers.IO) { dao.slots() }
        val token = AppState.token
        if (token != null) {
            runCatching { PatrolClient().manifest(token) }
                .onSuccess { fresh -> withContext(Dispatchers.IO) { dao.replaceSlots(fresh) }; slots = fresh; note = null }
                .onFailure { if (slots.isEmpty()) note = "Offline — showing cached timetable (none yet). Connect once to download today's timetable." }
        }
        syncPending(ctx)
        pending = withContext(Dispatchers.IO) { dao.pendingCount() }
        loading = false
    }
    LaunchedEffect(Unit) { refresh() }

    // The slots that overlap "now" (from 10 min before start to end) float to the top + are marked LIVE.
    val nowMin = LocalTime.now().let { it.hour * 60 + it.minute }
    fun startMin(s: String): Int { val p = s.split(":"); return (p.getOrNull(0)?.toIntOrNull() ?: 0) * 60 + (p.getOrNull(1)?.toIntOrNull() ?: 0) }
    fun isNow(s: PatrolSlotEntity): Boolean { val st = startMin(s.startTime); return nowMin in (st - 10)..(st + s.durationMinutes) }
    val ordered = slots.sortedWith(compareByDescending<PatrolSlotEntity> { isNow(it) }.thenBy { it.startTime })
    val doneUnits = logs.associateBy { it.unitId + "@" + it.scheduledTime }

    Column(Modifier.fillMaxSize().padding(16.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text("Patrol — ${AppState.name.ifBlank { "QA" }}", fontWeight = FontWeight.Bold, fontSize = 18.sp)
                Text("Staff ID: ${AppState.staffId.ifBlank { "—" }}" + if (pending > 0) "  ·  $pending pending sync" else "  ·  all synced",
                    style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            IconButton(onClick = { scope.launch { refresh() } }) {
                Icon(Icons.Filled.Refresh, contentDescription = "Refresh",
                    tint = MaterialTheme.colorScheme.onSurface, modifier = Modifier.size(22.dp))
            }
            TextButton(onClick = { Store.clear(); AppState.token = null }) { Text("Sign out") }
        }
        Text("Tick whether the timetabled lecturer is actually teaching. Works offline — logs sync automatically.",
            style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(vertical = 6.dp))
        note?.let { Surface(color = MaterialTheme.colorScheme.errorContainer, shape = MaterialTheme.shapes.small, modifier = Modifier.fillMaxWidth()) { Text(it, Modifier.padding(10.dp), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onErrorContainer) } }

        when {
            loading && slots.isEmpty() -> Box(Modifier.fillMaxWidth().padding(top = 40.dp), Alignment.Center) { CircularProgressIndicator() }
            ordered.isEmpty() -> Text("No timetabled sessions found for today.", color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(top = 20.dp))
            else -> LazyColumn(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                items(ordered) { s ->
                    val key = s.unitId + "@" + s.startTime
                    val done = doneUnits[key]
                    Surface(color = if (isNow(s)) MaterialTheme.colorScheme.secondaryContainer else MaterialTheme.colorScheme.surfaceVariant, shape = MaterialTheme.shapes.medium) {
                        Column(Modifier.fillMaxWidth().padding(12.dp)) {
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Text(s.startTime, fontWeight = FontWeight.Bold, fontFamily = FontFamily.Monospace)
                                if (isNow(s)) { Spacer(Modifier.width(8.dp)); Text("● NOW", color = MaterialTheme.colorScheme.primary, style = MaterialTheme.typography.labelSmall, fontWeight = FontWeight.Bold) }
                            }
                            Text(s.unitName.ifBlank { s.unitId } + (if (s.courseCode.isNotBlank()) "  (${s.courseCode})" else ""), fontWeight = FontWeight.SemiBold)
                            Text("Lecturer: ${s.lecturerName.ifBlank { s.lecturerStaffId.ifBlank { "—" } }}", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            Text("Room: ${s.room.ifBlank { "—" }}", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            Spacer(Modifier.height(8.dp))
                            if (done != null) {
                                Text(if (done.taught) "✓ Marked TAUGHT" else "✗ Marked NOT TAUGHT",
                                    color = if (done.taught) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error, fontWeight = FontWeight.Bold)
                            } else {
                                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                    Button(modifier = Modifier.weight(1f), onClick = { scope.launch { record(ctx, dao, s, true); syncPending(ctx); pending = withContext(Dispatchers.IO) { dao.pendingCount() } } }) { Text("Taught ✓") }
                                    OutlinedButton(modifier = Modifier.weight(1f), onClick = { scope.launch { record(ctx, dao, s, false); syncPending(ctx); pending = withContext(Dispatchers.IO) { dao.pendingCount() } } }) { Text("Not taught ✗") }
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

/** Save a patrol observation locally with the patroller's identity + an automatic timestamp. */
private suspend fun record(ctx: android.content.Context, dao: ug.qaat.patroller.db.PatrolDao, s: PatrolSlotEntity, taught: Boolean) =
    withContext(Dispatchers.IO) {
        dao.putLog(PatrolLogEntity(
            id = UUID.randomUUID().toString(),
            unitId = s.unitId, unitName = s.unitName, courseCode = s.courseCode,
            lecturerId = s.lecturerStaffId, lecturerName = s.lecturerName, room = s.room,
            sessionDate = today(), scheduledTime = s.startTime, taught = taught,
            takenAt = java.time.Instant.now().toString(),
        ))
    }
