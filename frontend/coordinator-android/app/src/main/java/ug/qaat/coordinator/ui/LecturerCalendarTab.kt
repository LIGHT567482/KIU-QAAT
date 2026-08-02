package ug.qaat.coordinator.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch
import ug.qaat.coordinator.net.LecturerCalendarClient
import java.time.LocalDate
import java.time.YearMonth
import java.time.format.DateTimeFormatter

/**
 * The lecturer's teaching calendar, organised around COURSE UNITS.
 *
 * A lecturer thinks in units — "when do I teach CSE 2420?" — not in cohorts, and a unit taught to
 * both the Day and Weekend runs of a programme is still one thing they teach. So every block on a
 * day is titled by its unit, and the cohorts attending sit underneath it as badges rather than
 * splitting into separate entries.
 *
 *  • Master unit filter above the grid narrows the month to a single unit.
 *  • Tapping a block opens a drawer: overall attendance for that unit session, then the SAME
 *    figures split per cohort — which is how you see Cohort A at 95% and Cohort B at 89% on one
 *    unit, instead of an average that hides both.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LecturerCalendarTab() {
    val scope = rememberCoroutineScope()
    var month by remember { mutableStateOf(YearMonth.now()) }
    var unitFilter by remember { mutableStateOf<String?>(null) }
    var cal by remember { mutableStateOf<LecturerCalendarClient.Calendar?>(null) }
    var loading by remember { mutableStateOf(true) }
    var open by remember { mutableStateOf<LecturerCalendarClient.Event?>(null) }

    fun load() {
        loading = true
        scope.launch {
            val from = month.atDay(1).toString()
            val to = month.atEndOfMonth().toString()
            cal = LecturerCalendarClient().fetch(from, to, unitFilter)
            loading = false
        }
    }
    LaunchedEffect(month, unitFilter) { load() }

    val events = cal?.events.orEmpty()
    val byDate = events.groupBy { it.date }.toSortedMap()

    Column(Modifier.fillMaxSize().padding(horizontal = 16.dp)) {
        // ── Month stepper ───────────────────────────────────────────────────────
        Row(Modifier.fillMaxWidth().padding(vertical = 10.dp), verticalAlignment = Alignment.CenterVertically) {
            TextButton(onClick = { month = month.minusMonths(1) }) { Text("‹ Prev") }
            Text(
                month.format(DateTimeFormatter.ofPattern("MMMM yyyy")),
                Modifier.weight(1f), textAlign = androidx.compose.ui.text.style.TextAlign.Center,
                fontWeight = FontWeight.Bold, style = MaterialTheme.typography.titleMedium,
            )
            TextButton(onClick = { month = month.plusMonths(1) }) { Text("Next ›") }
        }

        // ── Master unit filter ──────────────────────────────────────────────────
        var menu by remember { mutableStateOf(false) }
        val unitLabel = cal?.units?.firstOrNull { it.unitId == unitFilter }
            ?.let { "${it.unitId} — ${it.unitName}" } ?: "All my units"
        ExposedDropdownMenuBox(expanded = menu, onExpandedChange = { menu = it }) {
            OutlinedTextField(
                value = unitLabel, onValueChange = {}, readOnly = true,
                label = { Text("Course unit") },
                trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded = menu) },
                modifier = Modifier.fillMaxWidth().menuAnchor(MenuAnchorType.PrimaryNotEditable),
            )
            ExposedDropdownMenu(expanded = menu, onDismissRequest = { menu = false }) {
                DropdownMenuItem(text = { Text("All my units") }, onClick = { unitFilter = null; menu = false })
                cal?.units?.forEach { u ->
                    DropdownMenuItem(
                        text = { Text("${u.unitId} — ${u.unitName}") },
                        onClick = { unitFilter = u.unitId; menu = false },
                    )
                }
            }
        }
        Spacer(Modifier.height(10.dp))

        when {
            loading && cal == null ->
                Box(Modifier.fillMaxWidth().padding(top = 40.dp), Alignment.Center) { CircularProgressIndicator() }

            events.isEmpty() -> Text(
                if (unitFilter == null) "Nothing timetabled this month."
                else "This unit has nothing timetabled this month.",
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )

            else -> LazyColumn(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                byDate.forEach { (date, dayEvents) ->
                    item(key = "d-$date") { DayHeading(date) }
                    items(dayEvents, key = { "${it.unitId}-${it.date}-${it.startTime}" }) { ev ->
                        UnitBlock(ev) { open = ev }
                    }
                }
                item { Spacer(Modifier.height(24.dp)) }
            }
        }
    }

    open?.let { ev -> UnitDrawer(ev) { open = null } }
}

@Composable
private fun DayHeading(date: String) {
    val d = runCatching { LocalDate.parse(date) }.getOrNull()
    val label = d?.format(DateTimeFormatter.ofPattern("EEEE d MMM")) ?: date
    val isToday = d == LocalDate.now()
    Row(Modifier.fillMaxWidth().padding(top = 12.dp, bottom = 2.dp), verticalAlignment = Alignment.CenterVertically) {
        Text(
            label, fontWeight = FontWeight.Bold,
            style = MaterialTheme.typography.labelLarge,
            color = if (isToday) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurface,
        )
        if (isToday) {
            Spacer(Modifier.width(6.dp))
            Text("TODAY", fontSize = 10.sp, fontWeight = FontWeight.Bold, color = MaterialTheme.colorScheme.primary)
        }
    }
}

/** One unit session: the UNIT is the title; the cohorts attending are badges beneath it. */
@Composable
private fun UnitBlock(ev: LecturerCalendarClient.Event, onClick: () -> Unit) {
    Surface(
        color = MaterialTheme.colorScheme.surfaceVariant,
        shape = MaterialTheme.shapes.medium,
        modifier = Modifier.fillMaxWidth().clickable(onClick = onClick),
    ) {
        Column(Modifier.padding(12.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text("${ev.unitId}: ${ev.unitName}", fontWeight = FontWeight.Bold)
                    Text(
                        listOfNotNull(
                            ev.startTime.takeIf { it.isNotBlank() },
                            ev.durationMinutes.takeIf { it > 0 }?.let { "${it}m" },
                            ev.room.takeIf { it.isNotBlank() },
                        ).joinToString(" · "),
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
                if (ev.held) {
                    Text(
                        "${ev.pct.toInt()}%", fontWeight = FontWeight.Bold,
                        color = if (ev.pct >= 75) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error,
                    )
                } else {
                    Text("scheduled", style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
            if (ev.cohorts.isNotEmpty()) {
                Spacer(Modifier.height(6.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(6.dp),
                    modifier = Modifier.horizontalScroll(rememberScrollState())) {
                    ev.cohorts.forEach { c -> CohortBadge(c.label.ifBlank { c.sessionType }) }
                }
            }
        }
    }
}

@Composable
private fun CohortBadge(label: String) {
    if (label.isBlank()) return
    Surface(
        color = MaterialTheme.colorScheme.primary.copy(alpha = .12f),
        shape = RoundedCornerShape(999.dp),
    ) {
        Text(
            label, fontSize = 10.sp, fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.primary,
            modifier = Modifier.padding(horizontal = 8.dp, vertical = 3.dp),
        )
    }
}

/** Details for one unit session: overall first, then the same numbers split per cohort. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun UnitDrawer(ev: LecturerCalendarClient.Event, onClose: () -> Unit) {
    ModalBottomSheet(onDismissRequest = onClose) {
        Column(Modifier.fillMaxWidth().padding(horizontal = 20.dp).padding(bottom = 28.dp)) {
            Text("${ev.unitId}: ${ev.unitName}", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
            Text(
                listOfNotNull(
                    runCatching { LocalDate.parse(ev.date).format(DateTimeFormatter.ofPattern("EEEE d MMM yyyy")) }.getOrNull(),
                    ev.startTime.takeIf { it.isNotBlank() },
                    ev.room.takeIf { it.isNotBlank() },
                ).joinToString(" · "),
                style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Spacer(Modifier.height(14.dp))

            if (!ev.held) {
                Surface(color = MaterialTheme.colorScheme.surfaceVariant, shape = MaterialTheme.shapes.medium,
                    modifier = Modifier.fillMaxWidth()) {
                    Text("Not held yet — this is a timetabled slot.", Modifier.padding(12.dp),
                        style = MaterialTheme.typography.bodySmall)
                }
                Spacer(Modifier.height(14.dp))
            }

            // Overall for the unit session.
            StatRow("Unit attendance", ev.present, ev.enrolled, ev.pct, bold = true)
            HorizontalDivider(Modifier.padding(vertical = 10.dp))

            Text("By cohort", style = MaterialTheme.typography.labelLarge, fontWeight = FontWeight.Bold)
            Spacer(Modifier.height(4.dp))
            if (ev.cohorts.isEmpty()) {
                Text("No cohort is mapped to this slot.", style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
            } else {
                Column(Modifier.verticalScroll(rememberScrollState())) {
                    ev.cohorts.forEach { c ->
                        StatRow(c.label.ifBlank { c.sessionType }.ifBlank { "Cohort" }, c.present, c.enrolled, c.pct)
                        c.coordinatorName.takeIf { it.isNotBlank() }?.let {
                            Text("coordinator: $it", style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                modifier = Modifier.padding(bottom = 6.dp))
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun StatRow(label: String, present: Int, enrolled: Int, pct: Double, bold: Boolean = false) {
    Column(Modifier.fillMaxWidth().padding(vertical = 4.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(label, Modifier.weight(1f),
                fontWeight = if (bold) FontWeight.Bold else FontWeight.SemiBold)
            Text(
                if (enrolled > 0) "${pct.toInt()}%" else "—",
                fontWeight = FontWeight.Bold,
                color = when {
                    enrolled == 0 -> MaterialTheme.colorScheme.onSurfaceVariant
                    pct >= 75 -> MaterialTheme.colorScheme.primary
                    else -> MaterialTheme.colorScheme.error
                },
            )
        }
        Text("$present of $enrolled present", style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant)
        if (enrolled > 0) {
            LinearProgressIndicator(
                progress = { (pct / 100.0).toFloat().coerceIn(0f, 1f) },
                modifier = Modifier.fillMaxWidth().padding(top = 4.dp),
            )
        }
    }
}
