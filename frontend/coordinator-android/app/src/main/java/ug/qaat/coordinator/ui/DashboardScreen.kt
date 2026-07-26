package ug.qaat.coordinator.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import ug.qaat.coordinator.di.Graph
import ug.qaat.coordinator.db.SessionEntity
import ug.qaat.coordinator.net.DashboardClient
import ug.qaat.coordinator.net.ManifestClient
import ug.qaat.coordinator.store.SessionStore
import ug.qaat.coordinator.sync.SyncManager
import java.time.LocalDate

private val DAYS = arrayOf("", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun")

/**
 * The coordinator home — mirrors the coordinator PWA Dashboard: cohort card, live-sessions
 * panel, and Timetable / Units / Students / Last-roster tabs, plus refresh-and-sync.
 * Taking attendance lives on the "Attendance" tab.
 */
@Composable
fun DashboardScreen() {
    val token = AppState.token
    val scope = rememberCoroutineScope()
    val client = remember { DashboardClient() }

    var overview by remember { mutableStateOf<DashboardClient.Overview?>(null) }
    var students by remember { mutableStateOf<List<DashboardClient.Student>>(emptyList()) }
    var last by remember { mutableStateOf<DashboardClient.LastRoster?>(null) }
    var active by remember { mutableStateOf<List<DashboardClient.ActiveSession>>(emptyList()) }
    var loaded by remember { mutableStateOf(false) }
    var tab by remember { mutableStateOf(0) }
    var closing by remember { mutableStateOf<String?>(null) }

    suspend fun refreshActive() { token?.let { active = runCatching { client.activeSessions(it) }.getOrDefault(emptyList()) } }
    suspend fun loadOnline() {
        val t = token ?: return
        val m = AppState.manifest
        val ov = runCatching { client.overview(t) }.getOrNull()
        if (ov != null) {
            overview = ov
            ov.offering?.let {
                AppState.cohortLabel = listOf(it.courseName, it.sessionType, "Year ${it.studyYear}", "Sem ${it.semester}", it.level, it.intake)
                    .filter { s -> s.isNotBlank() }.joinToString(" · ")
            }
        } else if (m != null) {
            // Offline fallback: derive overview from cached manifest.
            val units = m.units.map { u ->
                DashboardClient.Unit(u.unitId, u.unitName, 0, 0, 0, "", 0, u.lecturerName, "", u.lecturerPhone)
            }
            overview = DashboardClient.Overview(null, units)
        }
        students = runCatching { client.students(t) }.getOrDefault(emptyList()).ifEmpty {
            // Offline fallback: derive student list from manifest roster hashes.
            if (m != null) {
                val seen = mutableSetOf<String>()
                m.rosterByUnit.values.flatten().mapNotNull { e ->
                    if (seen.add(e.studentIdHash))
                        DashboardClient.Student(e.studentIdHash, "—", 0, 0, "", "ACTIVE")
                    else null
                }
            } else emptyList()
        }
        last = runCatching { client.lastRoster(t) }.getOrNull() ?: withContext(Dispatchers.IO) {
            // Offline fallback: build last-roster from Room attendance. MUST run off the main
            // thread — Room throws IllegalStateException on a main-thread query (loadOnline is
            // launched from a Compose LaunchedEffect, which dispatches on the UI thread).
            val dao = Graph.db.dao()
            val recent = if (AppState.currentSessionId != null)
                listOfNotNull(dao.rosterForSession(AppState.currentSessionId!!).takeIf { it.isNotEmpty() }?.let {
                    SessionEntity(AppState.currentSessionId!!, "", "", "LOCAL", it.size)
                })
            else emptyList()
            val s = (recent + dao.pendingSyncSessions()).maxByOrNull { it.sessionDate }
            if (s != null) {
                val records = dao.rosterForSession(s.sessionId)
                val rows = records.map { r ->
                    DashboardClient.RosterRow(r.studentIdHash, "", "PRESENT", r.checkinTimestamp)
                }
                DashboardClient.LastRoster(s.unitId, s.sessionDate, records.size, records.size, rows)
            } else null
        }
        refreshActive()
    }

    // Reload when the token or the global refresh tick (the top-nav ⟳) changes.
    LaunchedEffect(token, AppState.refreshTick) { loadOnline(); loaded = true }

    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp)) {
        // Welcome — greet the coordinator by the name on their login credential.
        Text("Welcome, ${AppState.displayName} 👋", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.ExtraBold)
        Text("Tap ⟳ (top-right) to download today's data, then take attendance even with no internet.",
            fontSize = 12.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.height(12.dp))

        // Cohort details moved into the profile popup; only flag the unassigned case here.
        if (overview?.offering == null && loaded) {
            Card(Modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.tertiaryContainer)) {
                Text("You are not yet assigned to a session. Ask your administrator to add you as a coordinator of a session.",
                    Modifier.padding(14.dp), fontSize = 13.sp)
            }
            Spacer(Modifier.height(12.dp))
        }

        // ── Live sessions in progress ────────────────────────────────────────────
        if (active.isNotEmpty()) {
            Card(Modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.secondaryContainer)) {
                Column(Modifier.padding(14.dp)) {
                    Text("● Live session${if (active.size > 1) "s" else ""} in progress", fontWeight = FontWeight.Bold)
                    active.forEach { s ->
                        Row(Modifier.fillMaxWidth().padding(top = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                            Column(Modifier.weight(1f)) {
                                Text(s.unitName, fontWeight = FontWeight.SemiBold)
                                Text("${if (s.status == "PENDING_LECTURER") "Awaiting lecturer" else "Active"} · ${s.presentCount} checked in",
                                    fontSize = 12.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            }
                            Button(
                                onClick = {
                                    closing = s.sessionId
                                    scope.launch {
                                        token?.let { client.closeSession(it, s.sessionId) }
                                        refreshActive(); last = token?.let { runCatching { client.lastRoster(it) }.getOrNull() }
                                        closing = null
                                    }
                                },
                                enabled = closing != s.sessionId,
                                colors = ButtonDefaults.buttonColors(containerColor = MaterialTheme.colorScheme.error),
                            ) { Text(if (closing == s.sessionId) "Closing…" else "Close") }
                        }
                    }
                }
            }
            Spacer(Modifier.height(12.dp))
        }

        // (Emergency standby now lives on the Attendance screen.)

        // ── Tabs ─────────────────────────────────────────────────────────────────
        val tabTitles = listOf("Timetable", "Units (${overview?.units?.size ?: 0})", "Students (${students.size})", "Last roster")
        ScrollableTabRow(selectedTabIndex = tab, edgePadding = 0.dp) {
            tabTitles.forEachIndexed { i, title -> Tab(selected = tab == i, onClick = { tab = i }, text = { Text(title, fontSize = 13.sp) }) }
        }
        Spacer(Modifier.height(10.dp))

        when (tab) {
            0 -> TimetableView(overview?.units ?: emptyList(), overview?.offering?.sessionType ?: "")
            1 -> UnitsView(overview?.units ?: emptyList())
            2 -> StudentsView(students)
            3 -> LastRosterView(last)
        }

        Spacer(Modifier.height(20.dp))
        HorizontalDivider()
        Spacer(Modifier.height(10.dp))
        Text("At-risk students and sync status are in the Absentees and Sync tabs below.",
            fontSize = 12.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.height(24.dp))
    }
}

// Time × Day grid (like the admin dashboard): a Time column on the left, day columns
// across, and each unit rendered as a block that COVERS all the cells for its full
// running time (08:00–11:00 = a 3-hour tall block). Weekend cohorts show Sat/Sun.
@Composable
private fun TimetableView(units: List<DashboardClient.Unit>, sessionType: String) {
    val scheduled = units.filter { it.dayOfWeek in 1..7 && it.start.isNotBlank() }
    val unscheduled = units.filter { !(it.dayOfWeek in 1..7 && it.start.isNotBlank()) }
    if (scheduled.isEmpty() && unscheduled.isEmpty()) { EmptyNote("No units in this session yet. Ask your admin to add units, then set each unit's day & time when you open it."); return }
    val days = if (sessionType.lowercase().contains("weekend")) listOf(6, 7) else listOf(1, 2, 3, 4, 5)
    val today = LocalDate.now().dayOfWeek.value
    var lo = 8; var hi = 19
    scheduled.forEach { val h = ttHourOf(it.start); if (h < lo) lo = h; if (h + ttSpan(it.durationMin) > hi) hi = h + ttSpan(it.durationMin) }
    val hours = (lo until hi).toList()
    val rowH = 40.dp
    val dayW = 108.dp
    val timeW = 40.dp
    val brand = MaterialTheme.colorScheme.primary
    val line = MaterialTheme.colorScheme.outlineVariant
    val muted = MaterialTheme.colorScheme.onSurfaceVariant

    Column {
        Row(Modifier.horizontalScroll(rememberScrollState())) {
            // Time column
            Column {
                Box(Modifier.width(timeW).height(26.dp), contentAlignment = Alignment.Center) {
                    Text("Time", fontSize = 10.sp, fontWeight = FontWeight.Bold, color = brand)
                }
                hours.forEach { h ->
                    Box(Modifier.width(timeW).height(rowH), contentAlignment = Alignment.TopCenter) {
                        Text("%02d:00".format(h), fontSize = 10.sp, color = muted, modifier = Modifier.padding(top = 2.dp))
                    }
                }
            }
            // Day columns
            days.forEach { d ->
                val isToday = d == today
                Column {
                    Box(Modifier.width(dayW).height(26.dp).background(if (isToday) brand else MaterialTheme.colorScheme.surfaceVariant), contentAlignment = Alignment.Center) {
                        Text(DAYS[d] + if (isToday) " •" else "", fontSize = 10.sp, fontWeight = FontWeight.Bold,
                            color = if (isToday) MaterialTheme.colorScheme.onPrimary else muted)
                    }
                    Box(Modifier.width(dayW).height(rowH * hours.size.toFloat())) {
                        // hour gridlines
                        Column {
                            hours.forEach { _ ->
                                Box(Modifier.fillMaxWidth().height(rowH).drawBehind {
                                    drawLine(line, Offset(0f, size.height), Offset(size.width, size.height), 1f)
                                })
                            }
                        }
                        // unit blocks — height ∝ duration so they cover their full time
                        scheduled.filter { it.dayOfWeek == d }.forEach { u ->
                            val sh = ttHourOf(u.start); val span = ttSpan(u.durationMin)
                            Box(Modifier.offset(y = rowH * (sh - lo).toFloat()).height(rowH * span.toFloat()).fillMaxWidth().padding(1.5.dp)) {
                                Surface(color = MaterialTheme.colorScheme.surface, shape = RoundedCornerShape(5.dp),
                                    border = BorderStroke(1.dp, brand), modifier = Modifier.fillMaxSize()) {
                                    Column(Modifier.padding(horizontal = 4.dp, vertical = 3.dp)) {
                                        Text(u.name, fontWeight = FontWeight.SemiBold, fontSize = 10.sp, lineHeight = 11.sp, maxLines = 2, color = brand)
                                        Text(timeRange(u.start, u.durationMin), fontSize = 8.sp, lineHeight = 10.sp, color = muted, maxLines = 1)
                                        val extra = listOfNotNull(u.room.takeIf { it.isNotBlank() }, u.lecturers.takeIf { it.isNotBlank() },
                                            u.lecturerPhone.takeIf { it.isNotBlank() }?.let { "☎ $it" }).joinToString(" · ")
                                        if (extra.isNotBlank()) Text(extra, fontSize = 8.sp, lineHeight = 10.sp, color = muted, maxLines = if (span > 1) 3 else 1)
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
        if (unscheduled.isNotEmpty()) {
            Text("Not yet scheduled", fontWeight = FontWeight.Bold, fontSize = 12.sp,
                color = MaterialTheme.colorScheme.tertiary, modifier = Modifier.padding(top = 12.dp, bottom = 4.dp))
            unscheduled.forEach { Text("• ${it.name}", fontSize = 12.sp, color = muted) }
        }
    }
}

private fun ttHourOf(hhmm: String) = hhmm.split(":").getOrNull(0)?.toIntOrNull() ?: 0
private fun ttSpan(mins: Int): Int { val m = if (mins <= 0) 60 else mins; return maxOf(1, (m + 59) / 60) }

@Composable
private fun UnitsView(units: List<DashboardClient.Unit>) {
    if (units.isEmpty()) { EmptyNote("No units in this program yet."); return }
    Column {
        units.forEach { u ->
            Row(Modifier.fillMaxWidth().padding(vertical = 6.dp)) {
                Column(Modifier.weight(1f)) {
                    Text(u.name, fontWeight = FontWeight.SemiBold, fontSize = 13.sp)
                    val sched = if (u.start.isNotBlank()) "${if (u.dayOfWeek in 1..7) DAYS[u.dayOfWeek] + " " else ""}${u.start}" else "—"
                    Text("${u.unitId} · $sched" + if (u.lecturers.isNotBlank()) " · ${u.lecturers}" else "",
                        fontSize = 11.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
            HorizontalDivider()
        }
    }
}

// Three columns only — Reg No · Name · Status — since year/semester/intake are all the
// coordinator's own cohort (inherited), so they'd be identical for every row.
@Composable
private fun StudentsView(students: List<DashboardClient.Student>) {
    if (students.isEmpty()) { EmptyNote("No students enrolled in this session yet."); return }
    Column {
        Row(Modifier.fillMaxWidth().padding(vertical = 6.dp)) {
            Text("Reg No.", Modifier.weight(1.1f), fontSize = 11.sp, fontWeight = FontWeight.Bold, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Text("Student name", Modifier.weight(1.4f), fontSize = 11.sp, fontWeight = FontWeight.Bold, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Text("Status", Modifier.weight(0.7f), fontSize = 11.sp, fontWeight = FontWeight.Bold, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        HorizontalDivider()
        students.forEach { s ->
            Row(Modifier.fillMaxWidth().padding(vertical = 7.dp)) {
                Text(s.studentId, Modifier.weight(1.1f), fontSize = 12.sp, fontFamily = FontFamily.Monospace)
                Text(s.fullName, Modifier.weight(1.4f), fontSize = 12.sp, fontWeight = FontWeight.SemiBold)
                Text(s.status, Modifier.weight(0.7f), fontSize = 11.sp,
                    color = if (s.status.equals("ACTIVE", true)) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant)
            }
            HorizontalDivider()
        }
    }
}

@Composable
private fun LastRosterView(last: DashboardClient.LastRoster?) {
    if (last == null) { EmptyNote("No closed/synced session yet. Close a session to see its roster here."); return }
    Column {
        Text("${last.unitName} · ${last.date} · ${last.present}/${last.total} present",
            fontSize = 13.sp, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(bottom = 6.dp))
        last.roster.forEach { r ->
            Row(Modifier.fillMaxWidth().padding(vertical = 5.dp)) {
                Column(Modifier.weight(1f)) {
                    Text(r.fullName, fontWeight = FontWeight.SemiBold, fontSize = 13.sp)
                    Text(r.studentId, fontSize = 11.sp, fontFamily = FontFamily.Monospace, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                Text(r.status, fontSize = 11.sp,
                    color = if (r.status.equals("PRESENT", true)) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error)
            }
            HorizontalDivider()
        }
    }
}

@Composable
fun StandbyCard(client: DashboardClient, token: String?) {
    var open by remember { mutableStateOf(false) }
    var list by remember { mutableStateOf<List<DashboardClient.Standby>>(emptyList()) }
    var reg by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var err by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()

    suspend fun reload() { token?.let { list = runCatching { client.standby(it) }.getOrDefault(emptyList()) } }
    LaunchedEffect(open) { if (open) reload() }

    Card(Modifier.fillMaxWidth()) {
        Column(Modifier.padding(14.dp)) {
            Row(Modifier.fillMaxWidth().clickable { open = !open }, verticalAlignment = Alignment.CenterVertically) {
                Text("🆘 Emergency standby" + if (list.isNotEmpty()) " · ${list.size} active" else "",
                    fontWeight = FontWeight.Bold, fontSize = 13.sp, modifier = Modifier.weight(1f))
                Text(if (open) "▾" else "▸", color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            if (open) {
                Spacer(Modifier.height(8.dp))
                Text("Going to be absent? Authorise a student from your OWN cohort to run the session today. Read them the code — they sign in with the standby code. Valid until end of day.",
                    fontSize = 12.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
                err?.let { Text(it, color = MaterialTheme.colorScheme.error, fontSize = 12.sp, modifier = Modifier.padding(top = 6.dp)) }
                Row(Modifier.fillMaxWidth().padding(top = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                    OutlinedTextField(value = reg, onValueChange = { reg = it }, modifier = Modifier.weight(1f),
                        placeholder = { Text("Standby student's reg no.") }, singleLine = true,
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Text))
                    Spacer(Modifier.width(8.dp))
                    Button(
                        onClick = {
                            busy = true; err = null
                            scope.launch {
                                err = token?.let { client.issueStandby(it, reg.trim()) }
                                if (err == null) { reg = ""; reload() }
                                busy = false
                            }
                        },
                        enabled = !busy && reg.isNotBlank(),
                    ) { Text(if (busy) "…" else "Issue") }
                }
                list.forEach { s ->
                    Row(Modifier.fillMaxWidth().padding(top = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                        Column(Modifier.weight(1f)) {
                            Text(s.code, fontFamily = FontFamily.Monospace, fontWeight = FontWeight.ExtraBold, fontSize = 16.sp)
                            Text(if (s.deputyName.isNotBlank()) s.deputyName else s.deputyReg,
                                fontSize = 11.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                        TextButton(onClick = { scope.launch { token?.let { client.revokeStandby(it, s.id) }; reload() } }) { Text("Revoke") }
                    }
                }
            }
        }
    }
}

@Composable
private fun EmptyNote(msg: String) {
    Text(msg, color = MaterialTheme.colorScheme.onSurfaceVariant, fontSize = 13.sp,
        modifier = Modifier.fillMaxWidth().padding(20.dp))
}

/** "14:30" → "2:30 PM". */
private fun ampm(hhmm: String): String {
    if (hhmm.isBlank()) return ""
    val p = hhmm.split(":"); if (p.size < 2) return hhmm
    val h = p[0].toIntOrNull() ?: return hhmm
    val suffix = if (h < 12) "AM" else "PM"
    val h12 = when { h % 12 == 0 -> 12; else -> h % 12 }
    return "$h12:${p[1]} $suffix"
}

/** "09:00" + 120 → "9:00 AM – 11:00 AM". */
private fun timeRange(start: String, mins: Int): String {
    if (start.isBlank()) return ""
    if (mins <= 0) return ampm(start)
    val p = start.split(":"); if (p.size < 2) return ampm(start)
    val total = (p[0].toIntOrNull() ?: return ampm(start)) * 60 + (p[1].toIntOrNull() ?: return ampm(start)) + mins
    val end = "%02d:%02d".format((total / 60) % 24, total % 60)
    return "${ampm(start)} – ${ampm(end)}"
}
