package ug.qaat.coordinator.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
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
 * The lecturer's teaching calendar: a REAL month grid, every day of the month on it.
 *
 * WHY A GRID AND NOT A LIST. A course unit is timetabled weekly — "CSE 2420, every Monday 08:00" —
 * so what a lecturer needs to see is the shape of the month: which Mondays they taught, which one
 * they missed, which are still to come. The previous agenda list only rendered days that had a
 * class, so the month arrived as a handful of disconnected headings with the gaps invisible, and a
 * missed Monday looked exactly like a Monday that was never timetabled: absent from the screen.
 *
 * Every day now has a cell, and every cell carries a MARK ([DayMark]) that answers the one
 * question the calendar exists for — was I there?
 *
 *  • **Taught** — the lecturer gated in. Filled in the brand colour.
 *  • **Missed** — timetabled, the day has passed, and they never gated in. Filled in the error
 *    colour. This is the state the old view could not show at all.
 *  • **Partly** — more than one slot that day and only some were taught.
 *  • **Scheduled** — timetabled, still to come. Outlined.
 *  • **Free** — nothing timetabled. Plain.
 *
 * "Was I there?" is read from `lecturer_present` (the gate record in lecturer_attendance_logs), NOT
 * from whether a session existed — a coordinator can open a room around a lecturer who never came.
 * Student attendance is the second question, and it stays where it was: the per-cohort split in the
 * drawer, reached by tapping a day.
 *
 * A lecturer thinks in units, so the master unit filter still narrows the whole month to one unit.
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
    // The day whose classes are listed under the grid. Defaults to today when today is in view, so
    // opening the tab already answers "what am I teaching now?".
    var selected by remember { mutableStateOf(LocalDate.now()) }

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
    // Keep the selection inside the month on show; stepping to another month lands on its 1st
    // (or today, if today happens to be in that month).
    LaunchedEffect(month) {
        if (YearMonth.from(selected) != month) {
            selected = if (YearMonth.now() == month) LocalDate.now() else month.atDay(1)
        }
    }

    val events = cal?.events.orEmpty()
    val byDate = remember(events) { events.groupBy { it.date } }

    Column(Modifier.fillMaxSize().padding(horizontal = 16.dp).verticalScroll(rememberScrollState())) {
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
        Spacer(Modifier.height(12.dp))

        if (loading && cal == null) {
            Box(Modifier.fillMaxWidth().padding(top = 40.dp), Alignment.Center) { CircularProgressIndicator() }
            return@Column
        }

        // ── The month ───────────────────────────────────────────────────────────
        MonthGrid(month = month, byDate = byDate, selected = selected, onSelect = { selected = it })
        Spacer(Modifier.height(10.dp))
        MonthSummary(month, byDate)
        Spacer(Modifier.height(8.dp))
        Legend()

        HorizontalDivider(Modifier.padding(vertical = 14.dp))

        // ── The selected day's classes ──────────────────────────────────────────
        DayHeading(selected)
        Spacer(Modifier.height(6.dp))
        val dayEvents = byDate[selected.toString()].orEmpty()
            .sortedBy { it.startTime }
        if (dayEvents.isEmpty()) {
            Text(
                if (selected.dayOfWeek.value >= 6) "Nothing timetabled — weekend."
                else "Nothing timetabled on this day.",
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                style = MaterialTheme.typography.bodySmall,
            )
        } else {
            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                dayEvents.forEach { ev -> UnitBlock(ev) { open = ev } }
            }
        }
        Spacer(Modifier.height(28.dp))
    }

    open?.let { ev -> UnitDrawer(ev) { open = null } }
}

// ── The mark on a day ────────────────────────────────────────────────────────

/** What a single day of the month says about the lecturer's presence. */
private enum class DayMark { FREE, SCHEDULED, TAUGHT, PARTLY, MISSED }

/**
 * Reduce a day's slots to one mark.
 *
 * The past/future split is what separates MISSED from SCHEDULED, and it is decided by the calendar
 * date alone: a slot earlier TODAY that has not been gated into is not yet a miss — the lecturer
 * may be walking to it — so today is never marked missed, only partly or taught.
 */
private fun markFor(date: LocalDate, dayEvents: List<LecturerCalendarClient.Event>): DayMark {
    if (dayEvents.isEmpty()) return DayMark.FREE
    val taught = dayEvents.count { it.lecturerPresent }
    return when {
        taught == dayEvents.size -> DayMark.TAUGHT
        taught > 0 -> DayMark.PARTLY
        date.isBefore(LocalDate.now()) -> DayMark.MISSED
        else -> DayMark.SCHEDULED
    }
}

/**
 * The month as seven columns, Monday-first, with leading and trailing blanks so the weekdays line
 * up. Rendered as plain Rows rather than a LazyVerticalGrid because the whole tab already scrolls —
 * nesting a lazy grid in a scrolling column throws on an unbounded height constraint.
 */
@Composable
private fun MonthGrid(
    month: YearMonth,
    byDate: Map<String, List<LecturerCalendarClient.Event>>,
    selected: LocalDate,
    onSelect: (LocalDate) -> Unit,
) {
    Row(Modifier.fillMaxWidth()) {
        listOf("M", "T", "W", "T", "F", "S", "S").forEach { d ->
            Text(
                d, Modifier.weight(1f), textAlign = androidx.compose.ui.text.style.TextAlign.Center,
                style = MaterialTheme.typography.labelSmall, fontWeight = FontWeight.Bold,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
    Spacer(Modifier.height(4.dp))

    val first = month.atDay(1)
    val lead = first.dayOfWeek.value - 1          // Monday=1 → 0 blanks, Sunday=7 → 6 blanks
    val days = month.lengthOfMonth()
    val cells = lead + days
    val rows = (cells + 6) / 7                    // always enough rows to hold every day

    for (row in 0 until rows) {
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(3.dp)) {
            for (col in 0 until 7) {
                val dayNum = row * 7 + col - lead + 1
                if (dayNum < 1 || dayNum > days) {
                    Spacer(Modifier.weight(1f).aspectRatio(1f))
                } else {
                    val date = month.atDay(dayNum)
                    val evs = byDate[date.toString()].orEmpty()
                    DayCell(
                        date = date,
                        mark = markFor(date, evs),
                        count = evs.size,
                        isSelected = date == selected,
                        modifier = Modifier.weight(1f),
                        onClick = { onSelect(date) },
                    )
                }
            }
        }
        Spacer(Modifier.height(3.dp))
    }
}

/** One day. The mark is carried by the FILL, not by an easily-missed dot. */
@Composable
private fun DayCell(
    date: LocalDate,
    mark: DayMark,
    count: Int,
    isSelected: Boolean,
    modifier: Modifier = Modifier,
    onClick: () -> Unit,
) {
    val scheme = MaterialTheme.colorScheme
    val isToday = date == LocalDate.now()
    val fill = when (mark) {
        DayMark.TAUGHT -> scheme.primary
        DayMark.MISSED -> scheme.error
        DayMark.PARTLY -> scheme.tertiary
        DayMark.SCHEDULED -> scheme.primary.copy(alpha = .13f)
        DayMark.FREE -> Color.Transparent
    }
    val ink = when (mark) {
        DayMark.TAUGHT -> scheme.onPrimary
        DayMark.MISSED -> scheme.onError
        DayMark.PARTLY -> scheme.onTertiary
        DayMark.SCHEDULED -> scheme.primary
        DayMark.FREE -> scheme.onSurfaceVariant
    }

    Box(
        modifier
            .aspectRatio(1f)
            .clip(RoundedCornerShape(8.dp))
            .background(fill)
            .then(
                // Today is ringed, the tapped day is ringed thicker. Both survive any fill.
                if (isSelected) Modifier.border(2.dp, scheme.onSurface, RoundedCornerShape(8.dp))
                else if (isToday) Modifier.border(1.5.dp, scheme.primary, RoundedCornerShape(8.dp))
                else Modifier
            )
            .clickable(onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Text(
                "${date.dayOfMonth}",
                fontSize = 13.sp,
                fontWeight = if (mark == DayMark.FREE) FontWeight.Normal else FontWeight.Bold,
                color = ink,
            )
            // More than one class that day, so the cell is honest about how much it is summarising.
            if (count > 1) {
                Text("$count", fontSize = 8.sp, color = ink.copy(alpha = .8f), fontWeight = FontWeight.Bold)
            }
        }
    }
}

/** Taught / missed / to come for the whole month — the number the lecturer is actually judged on. */
@Composable
private fun MonthSummary(month: YearMonth, byDate: Map<String, List<LecturerCalendarClient.Event>>) {
    val all = byDate.entries
        .mapNotNull { (d, evs) -> runCatching { LocalDate.parse(d) }.getOrNull()?.let { it to evs } }
        .filter { YearMonth.from(it.first) == month }
    val slots = all.sumOf { it.second.size }
    if (slots == 0) return
    val taught = all.sumOf { (_, evs) -> evs.count { it.lecturerPresent } }
    val missed = all.sumOf { (d, evs) ->
        if (d.isBefore(LocalDate.now())) evs.count { !it.lecturerPresent } else 0
    }
    val upcoming = slots - taught - missed
    val pct = if (taught + missed > 0) taught * 100 / (taught + missed) else 0

    Surface(color = MaterialTheme.colorScheme.surfaceVariant, shape = MaterialTheme.shapes.medium,
        modifier = Modifier.fillMaxWidth()) {
        Row(Modifier.padding(horizontal = 12.dp, vertical = 10.dp), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text("$taught taught · $missed missed" + if (upcoming > 0) " · $upcoming to come" else "",
                    style = MaterialTheme.typography.labelLarge, fontWeight = FontWeight.Bold)
                Text("$slots timetabled ${if (slots == 1) "class" else "classes"} this month",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            if (taught + missed > 0) {
                Text("$pct%", fontWeight = FontWeight.Bold,
                    color = if (pct >= 75) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error)
            }
        }
    }
}

/** Without this the colours are a guess. */
@Composable
private fun Legend() {
    val scheme = MaterialTheme.colorScheme
    Row(Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
        horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        LegendChip(scheme.primary, "Taught")
        LegendChip(scheme.error, "Missed")
        LegendChip(scheme.tertiary, "Partly")
        LegendChip(scheme.primary.copy(alpha = .13f), "Scheduled")
    }
}

@Composable
private fun LegendChip(color: Color, label: String) {
    Row(verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.size(10.dp).clip(RoundedCornerShape(3.dp)).background(color))
        Spacer(Modifier.width(4.dp))
        Text(label, fontSize = 10.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

@Composable
private fun DayHeading(date: LocalDate) {
    val isToday = date == LocalDate.now()
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
        Text(
            date.format(DateTimeFormatter.ofPattern("EEEE d MMMM")), fontWeight = FontWeight.Bold,
            style = MaterialTheme.typography.titleSmall,
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
                // Two different facts, stacked: whether the LECTURER was there, and how many
                // students came. The first is the calendar's subject and leads.
                Column(horizontalAlignment = Alignment.End) {
                    PresenceTag(ev)
                    if (ev.held && ev.enrolled > 0) {
                        Text(
                            "${ev.pct.toInt()}% present", fontSize = 10.sp,
                            color = if (ev.pct >= 75) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error,
                        )
                    }
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

/**
 * "Taught 2h" / "Missed" / "Scheduled" for one slot — the same three states the grid uses, so a
 * cell's colour and the block underneath it can never disagree.
 */
@Composable
private fun PresenceTag(ev: LecturerCalendarClient.Event) {
    val past = runCatching { LocalDate.parse(ev.date).isBefore(LocalDate.now()) }.getOrDefault(false)
    val (label, color) = when {
        ev.lecturerPresent -> {
            val h = ev.contactHours
            (if (h > 0) "Taught · ${trimHours(h)}h" else "Taught") to MaterialTheme.colorScheme.primary
        }
        past -> "Missed" to MaterialTheme.colorScheme.error
        else -> "Scheduled" to MaterialTheme.colorScheme.onSurfaceVariant
    }
    Text(label, fontWeight = FontWeight.Bold, fontSize = 12.sp, color = color)
}

/** 2.0 → "2", 1.5 → "1.5" — hours read as hours, not as decimals. */
private fun trimHours(h: Double): String =
    if (h == h.toLong().toDouble()) h.toLong().toString() else h.toString()

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

            // Your own presence, stated plainly and first — it is the reason this slot is coloured
            // the way it is on the grid.
            val past = runCatching { LocalDate.parse(ev.date).isBefore(LocalDate.now()) }.getOrDefault(false)
            val (banner, tone) = when {
                ev.lecturerPresent -> {
                    val h = if (ev.contactHours > 0) " — ${trimHours(ev.contactHours)} contact hours recorded" else ""
                    "You taught this class$h." to MaterialTheme.colorScheme.primary
                }
                past && ev.held ->
                    "No gate record for you on this slot, though a session was opened." to MaterialTheme.colorScheme.error
                past -> "Missed — this timetabled class was never held." to MaterialTheme.colorScheme.error
                else -> "Scheduled — this class is still to come." to MaterialTheme.colorScheme.onSurfaceVariant
            }
            Surface(color = tone.copy(alpha = .10f), shape = MaterialTheme.shapes.medium,
                modifier = Modifier.fillMaxWidth()) {
                Text(banner, Modifier.padding(12.dp), style = MaterialTheme.typography.bodySmall, color = tone)
            }
            Spacer(Modifier.height(14.dp))

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
