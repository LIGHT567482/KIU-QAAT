package ug.qaat.coordinator.ui

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import ug.qaat.coordinator.di.Graph
import ug.qaat.coordinator.net.LecturerTimetableClient

/**
 * THE LECTURER'S TIMETABLE ON THE PHONE.
 *
 * The phone showed a month calendar — which answers "how have I done", the record of taught and
 * missed days. It never answered the question a lecturer actually opens their phone to ask on the
 * way across campus: where am I meant to be, and when. That is a timetable, and this is it.
 *
 * MONDAY TO SUNDAY, always. Every other grid in this app belongs to one cohort, so it shows Mon–Fri
 * or Sat–Sun depending on which. A lecturer teaches ACROSS cohorts — the Day run on Tuesday, the
 * Weekend run on Saturday, the e-learning run on a Sunday evening — so a five-day grid would
 * silently swallow their weekend, and the days it hides are the ones people forget.
 *
 * OFFLINE IS NOT AN EMPTY WEEK. If the call fails, this falls back to the timetable already cached
 * for presence-claim matching and says plainly that it is showing the cached copy. An empty grid
 * would read as "nothing timetabled", which is the most damaging thing this screen could say to
 * someone standing outside a lecture room with no signal.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LecturerTimetableTab() {
    val scope = rememberCoroutineScope()
    var slots by remember { mutableStateOf<List<LecturerTimetableClient.Slot>>(emptyList()) }
    var loading by remember { mutableStateOf(true) }
    var cached by remember { mutableStateOf(false) }

    // Filters. A lecturer with four cohorts of the same unit needs to be able to cut the week down
    // to one of them; a lecturer with three slots does not, so the pickers only appear when there
    // is more than one value to pick between.
    var unit by remember { mutableStateOf<String?>(null) }
    var course by remember { mutableStateOf<String?>(null) }
    var session by remember { mutableStateOf<String?>(null) }
    var intake by remember { mutableStateOf<String?>(null) }
    var day by remember { mutableStateOf(0) }

    fun load() {
        loading = true
        scope.launch {
            val fresh = LecturerTimetableClient().week()
            if (fresh != null) {
                slots = fresh; cached = false
            } else {
                // The cache PresenceClient keeps. It has no cohort or delivery mode — those columns
                // were never needed offline — so those chips simply do not apply to it.
                val rows = withContext(Dispatchers.IO) {
                    runCatching { Graph.db.dao().timetable() }.getOrDefault(emptyList())
                }
                slots = rows.map {
                    LecturerTimetableClient.Slot(
                        unitId = it.unitId, offeringId = "", unitName = it.unitName, courseName = "",
                        sessionType = "", level = "", intake = "", studyYear = 0, semester = 0,
                        dayOfWeek = it.dayOfWeek, startTime = it.startTime,
                        durationMinutes = it.durationMinutes, room = it.room, building = "",
                        deliveryMode = "IN_PERSON", namedOnSlot = true, enrolled = 0,
                    )
                }
                cached = true
            }
            loading = false
        }
    }
    LaunchedEffect(Unit) { load() }

    val units = remember(slots) { slots.map { it.unitId }.filter { it.isNotBlank() }.distinct().sorted() }
    val courses = remember(slots) { slots.map { it.courseName }.filter { it.isNotBlank() }.distinct().sorted() }
    val sessions = remember(slots) { slots.map { it.sessionType }.filter { it.isNotBlank() }.distinct().sorted() }
    val intakes = remember(slots) { slots.map { it.intake }.filter { it.isNotBlank() }.distinct().sorted() }

    val shown = slots.filter {
        (unit == null || it.unitId == unit) &&
            (course == null || it.courseName == course) &&
            (session == null || it.sessionType == session) &&
            (intake == null || it.intake == intake) &&
            (day == 0 || it.dayOfWeek == day)
    }
    val filtered = unit != null || course != null || session != null || intake != null || day != 0

    Column(Modifier.fillMaxSize().padding(horizontal = 16.dp).verticalScroll(rememberScrollState())) {
        Row(Modifier.fillMaxWidth().padding(top = 10.dp), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text("My timetable", fontWeight = FontWeight.Bold,
                    style = MaterialTheme.typography.titleMedium)
                Text(
                    "Every unit you teach, in every cohort — Monday to Sunday",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            TextButton(onClick = { load() }, enabled = !loading) { Text(if (loading) "…" else "Refresh") }
        }

        if (cached && slots.isNotEmpty()) {
            Surface(
                color = MaterialTheme.colorScheme.secondaryContainer,
                shape = MaterialTheme.shapes.small, modifier = Modifier.fillMaxWidth().padding(top = 6.dp),
            ) {
                Text(
                    "No signal — showing the copy saved on this phone. Rooms may have moved since.",
                    Modifier.padding(10.dp), style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSecondaryContainer,
                )
            }
        }

        if (loading && slots.isEmpty()) {
            Row(Modifier.padding(vertical = 24.dp), verticalAlignment = Alignment.CenterVertically) {
                CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                Spacer(Modifier.width(10.dp)); Text("Loading your week…")
            }
        }

        if (!loading && slots.isEmpty()) {
            Surface(
                color = MaterialTheme.colorScheme.errorContainer, shape = MaterialTheme.shapes.medium,
                modifier = Modifier.fillMaxWidth().padding(top = 12.dp),
            ) {
                Text(
                    "Nothing is timetabled against you. If you are teaching, ask the TLC for your " +
                        "department to put your lectures on the timetable — until they are there, " +
                        "your students have no session to check in to.",
                    Modifier.padding(14.dp), style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onErrorContainer,
                )
            }
        }

        if (slots.isNotEmpty()) {
            Spacer(Modifier.height(8.dp))
            FilterPicker("Course unit", units, unit) { unit = it }
            if (courses.size > 1) FilterPicker("Course", courses, course) { course = it }
            if (sessions.size > 1) FilterPicker("Session", sessions, session) { session = it }
            if (intakes.size > 1) FilterPicker("Intake", intakes, intake) { intake = it }
            DayPicker(day) { day = it }
            if (filtered) {
                TextButton(onClick = { unit = null; course = null; session = null; intake = null; day = 0 }) {
                    Text("Clear filters")
                }
            }

            val hours = shown.sumOf { it.durationMinutes } / 60.0
            val weekend = shown.count { it.dayOfWeek >= 6 }
            val onlineN = shown.count { it.online }
            Text(
                buildString {
                    append("${shown.size} lecture${if (shown.size == 1) "" else "s"}")
                    if (filtered) append(" of ${slots.size}")
                    append(" · ${"%.1f".format(hours).removeSuffix(".0")} h a week")
                    if (weekend > 0) append(" · $weekend at the weekend")
                    if (onlineN > 0) append(" · $onlineN online")
                },
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(vertical = 6.dp),
            )

            if (shown.isEmpty()) {
                Text("Nothing matches those filters.", Modifier.padding(vertical = 20.dp),
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
            } else {
                TimetableGrid(
                    entries = shown.map { s ->
                        TtEntry(
                            name = s.unitId.ifBlank { s.unitName },
                            dayOfWeek = s.dayOfWeek,
                            start = s.startTime,
                            durationMin = s.durationMinutes,
                            // The cohort first: two blocks of the same unit in one week are only
                            // told apart by it. Then where — and ONLINE is a where, not a missing
                            // room, which a blank would be read as.
                            detail = listOf(
                                s.cohort,
                                if (s.online) "ONLINE" else s.room,
                                if (s.namedOnSlot) "" else "cover?",
                            ).filter { it.isNotBlank() }.joinToString(" · "),
                        )
                    },
                    sessionType = "",
                    days = listOf(1, 2, 3, 4, 5, 6, 7),   // a lecturer's week is the whole week
                )
                Spacer(Modifier.height(10.dp))
                // The grid is for finding a slot at a glance; the list underneath is where the
                // detail that will not fit in a 108dp column goes.
                shown.sortedWith(compareBy({ it.dayOfWeek }, { it.startTime })).forEach { SlotRow(it) }
            }
        }
        Spacer(Modifier.height(24.dp))
    }
}

@Composable
private fun SlotRow(s: LecturerTimetableClient.Slot) {
    Surface(
        shape = MaterialTheme.shapes.small, tonalElevation = 1.dp,
        modifier = Modifier.fillMaxWidth().padding(vertical = 3.dp),
    ) {
        Row(Modifier.padding(10.dp), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.width(58.dp)) {
                Text(DAYS[s.dayOfWeek], fontWeight = FontWeight.Bold, fontSize = 12.sp)
                Text(s.startTime, fontSize = 11.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            Column(Modifier.weight(1f)) {
                Text("${s.unitId} — ${s.unitName}", fontWeight = FontWeight.SemiBold, fontSize = 13.sp)
                if (s.cohort.isNotBlank()) {
                    Text(s.cohort, fontSize = 11.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                Text(
                    buildString {
                        if (s.online) append("Online — distance / e-learning, no room")
                        else if (s.room.isNotBlank()) {
                            append(s.room); if (s.building.isNotBlank()) append(" · ${s.building}")
                        } else append("No room on the timetable")
                        if (s.enrolled > 0) append(" · ${s.enrolled} students")
                    },
                    fontSize = 11.sp,
                    color = if (s.online || s.room.isBlank()) MaterialTheme.colorScheme.primary
                            else MaterialTheme.colorScheme.onSurfaceVariant,
                )
                if (!s.namedOnSlot) {
                    Text(
                        "The timetable does not name a lecturer here — you are on it through your unit assignment.",
                        fontSize = 10.sp, color = MaterialTheme.colorScheme.error,
                    )
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun FilterPicker(label: String, options: List<String>, selected: String?, onPick: (String?) -> Unit) {
    if (options.isEmpty()) return
    var open by remember { mutableStateOf(false) }
    ExposedDropdownMenuBox(expanded = open, onExpandedChange = { open = it }) {
        OutlinedTextField(
            value = selected ?: "All", onValueChange = {}, readOnly = true,
            label = { Text(label) },
            trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded = open) },
            modifier = Modifier.fillMaxWidth().menuAnchor(MenuAnchorType.PrimaryNotEditable),
        )
        ExposedDropdownMenu(expanded = open, onDismissRequest = { open = false }) {
            DropdownMenuItem(text = { Text("All") }, onClick = { onPick(null); open = false })
            options.forEach { o ->
                DropdownMenuItem(text = { Text(o) }, onClick = { onPick(o); open = false })
            }
        }
    }
    Spacer(Modifier.height(6.dp))
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun DayPicker(selected: Int, onPick: (Int) -> Unit) {
    var open by remember { mutableStateOf(false) }
    val label = if (selected == 0) "All days" else DAYS[selected]
    ExposedDropdownMenuBox(expanded = open, onExpandedChange = { open = it }) {
        OutlinedTextField(
            value = label, onValueChange = {}, readOnly = true,
            label = { Text("Day") },
            trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded = open) },
            modifier = Modifier.fillMaxWidth().menuAnchor(MenuAnchorType.PrimaryNotEditable),
        )
        ExposedDropdownMenu(expanded = open, onDismissRequest = { open = false }) {
            DropdownMenuItem(text = { Text("All days") }, onClick = { onPick(0); open = false })
            (1..7).forEach { d ->
                DropdownMenuItem(text = { Text(DAYS[d]) }, onClick = { onPick(d); open = false })
            }
        }
    }
    Spacer(Modifier.height(6.dp))
}

/**
 * The schedule tab: the TIMETABLE by default, with the month record one tap away.
 *
 * The tab is called Timetable because that is what a lecturer opens it for — where am I meant to
 * be. The month calendar answers a different question, "how have I done", and it is the only place
 * the taught/missed record is visible on the phone, so it is kept rather than replaced. Two
 * questions, two views, one tab, and the one people need in a corridor opens first.
 */
@Composable
fun LecturerScheduleTab() {
    var month by remember { mutableStateOf(false) }
    Column(Modifier.fillMaxSize()) {
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            FilterChip(selected = !month, onClick = { month = false }, label = { Text("Timetable") })
            FilterChip(selected = month, onClick = { month = true }, label = { Text("My month") })
        }
        Box(Modifier.weight(1f)) {
            if (month) LecturerCalendarTab() else LecturerTimetableTab()
        }
    }
}
