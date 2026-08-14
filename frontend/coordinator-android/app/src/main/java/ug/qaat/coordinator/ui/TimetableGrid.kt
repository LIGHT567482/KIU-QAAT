package ug.qaat.coordinator.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import java.time.LocalDate

/**
 * The weekly timetable, drawn once and shared by every role that shows one.
 *
 * The coordinator's dashboard and the student's home screen used to draw the week two different
 * ways — a Time × Day grid on one, a list of weekday headings on the other — so the same week
 * looked like two different timetables depending on who was holding the phone. Both now render
 * through [TimetableGrid]; only the mapping into [TtEntry] differs.
 */
internal val DAYS = arrayOf("", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun")

/** One block on the grid, whatever screen it came from. */
internal data class TtEntry(
    val name: String,
    val dayOfWeek: Int,          // 1=Mon … 7=Sun
    val start: String,           // "HH:MM"
    val durationMin: Int,
    /** Room · lecturer · phone — whatever the caller has; blank parts are dropped. */
    val detail: String = "",
)

/**
 * Time × Day grid: a Time column on the left, day columns across, and each entry rendered as a
 * block COVERING all the cells for its running time (08:00–11:00 = a 3-hour tall block).
 *
 * [sessionType] picks the columns — a weekend cohort gets Sat/Sun instead of Mon–Fri. Entries
 * falling outside the chosen columns are still shown, under "Not on this week's days", so a slot
 * can never silently vanish because its cohort was mis-tagged.
 *
 * [days] overrides that choice outright, for the one caller whose week is not a cohort's week: a
 * LECTURER teaches across cohorts, so their grid must run Monday to Sunday. Deriving that from a
 * session type would mean inventing a fake one, and the next reader would have to work out what
 * "weekend" meant on a screen showing five weekdays.
 */
@Composable
internal fun TimetableGrid(
    entries: List<TtEntry>,
    sessionType: String,
    unscheduledNames: List<String> = emptyList(),
    days: List<Int>? = null,
) {
    @Suppress("NAME_SHADOWING")
    val days = days ?: if (sessionType.lowercase().contains("weekend")) listOf(6, 7) else listOf(1, 2, 3, 4, 5)
    val scheduled = entries.filter { it.dayOfWeek in days && it.start.isNotBlank() }
    val offDays = entries.filter { it.dayOfWeek !in days && it.start.isNotBlank() }
    val today = LocalDate.now().dayOfWeek.value

    var lo = 8; var hi = 19
    scheduled.forEach {
        val h = ttHourOf(it.start)
        if (h < lo) lo = h
        if (h + ttSpan(it.durationMin) > hi) hi = h + ttSpan(it.durationMin)
    }
    lo = lo.coerceIn(0, 23); hi = hi.coerceIn(lo + 1, 24)   // clamp to a real clock — no 24:00/25:00 rows
    val hours = (lo until hi).toList()
    val rowH = 40.dp
    val dayW = 108.dp
    val timeW = 46.dp
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
                        Text(ampmHour(h), fontSize = 9.sp, color = muted, modifier = Modifier.padding(top = 2.dp))
                    }
                }
            }
            // Day columns
            days.forEach { d ->
                val isToday = d == today
                Column {
                    Box(
                        Modifier.width(dayW).height(26.dp)
                            .background(if (isToday) brand else MaterialTheme.colorScheme.surfaceVariant),
                        contentAlignment = Alignment.Center,
                    ) {
                        Text(
                            DAYS[d] + if (isToday) " •" else "", fontSize = 10.sp, fontWeight = FontWeight.Bold,
                            color = if (isToday) MaterialTheme.colorScheme.onPrimary else muted,
                        )
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
                        // blocks — height ∝ duration so they cover their full time
                        scheduled.filter { it.dayOfWeek == d }.forEach { e ->
                            val sh = ttHourOf(e.start); val span = ttSpan(e.durationMin)
                            Box(
                                Modifier.offset(y = rowH * (sh - lo).toFloat())
                                    .height(rowH * span.toFloat()).fillMaxWidth().padding(1.5.dp),
                            ) {
                                Surface(
                                    color = MaterialTheme.colorScheme.surface, shape = RoundedCornerShape(5.dp),
                                    border = BorderStroke(1.dp, brand), modifier = Modifier.fillMaxSize(),
                                ) {
                                    Column(Modifier.padding(horizontal = 4.dp, vertical = 3.dp)) {
                                        Text(e.name, fontWeight = FontWeight.SemiBold, fontSize = 10.sp,
                                            lineHeight = 11.sp, maxLines = 2, color = brand)
                                        Text(timeRange(e.start, e.durationMin), fontSize = 8.sp,
                                            lineHeight = 10.sp, color = muted, maxLines = 1)
                                        if (e.detail.isNotBlank()) {
                                            Text(e.detail, fontSize = 8.sp, lineHeight = 10.sp, color = muted,
                                                maxLines = if (span > 1) 3 else 1)
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
        if (offDays.isNotEmpty()) {
            Text("Not on this week's days", fontWeight = FontWeight.Bold, fontSize = 12.sp,
                color = MaterialTheme.colorScheme.tertiary, modifier = Modifier.padding(top = 12.dp, bottom = 4.dp))
            offDays.forEach {
                Text("• ${it.name} — ${DAYS.getOrNull(it.dayOfWeek).orEmpty()} ${it.start}".trim(),
                    fontSize = 12.sp, color = muted)
            }
        }
        if (unscheduledNames.isNotEmpty()) {
            Text("Not yet scheduled", fontWeight = FontWeight.Bold, fontSize = 12.sp,
                color = MaterialTheme.colorScheme.tertiary, modifier = Modifier.padding(top = 12.dp, bottom = 4.dp))
            unscheduledNames.forEach { Text("• $it", fontSize = 12.sp, color = muted) }
        }
    }
}

internal fun ttHourOf(hhmm: String) = hhmm.split(":").getOrNull(0)?.toIntOrNull() ?: 0

internal fun ttSpan(mins: Int): Int { val m = if (mins <= 0) 60 else mins; return maxOf(1, (m + 59) / 60) }

internal fun ampmHour(h: Int): String {
    val hh = ((h % 24) + 24) % 24
    val suffix = if (hh < 12) "AM" else "PM"
    val h12 = if (hh % 12 == 0) 12 else hh % 12
    return "$h12 $suffix"
}

/** "14:30" → "2:30 PM". */
internal fun ampm(hhmm: String): String {
    if (hhmm.isBlank()) return ""
    val p = hhmm.split(":"); if (p.size < 2) return hhmm
    val h = p[0].toIntOrNull() ?: return hhmm
    val suffix = if (h < 12) "AM" else "PM"
    val h12 = when { h % 12 == 0 -> 12; else -> h % 12 }
    return "$h12:${p[1]} $suffix"
}

/** "09:00" + 120 → "9:00 AM – 11:00 AM". */
internal fun timeRange(start: String, mins: Int): String {
    if (start.isBlank()) return ""
    if (mins <= 0) return ampm(start)
    val p = start.split(":"); if (p.size < 2) return ampm(start)
    val total = (p[0].toIntOrNull() ?: return ampm(start)) * 60 + (p[1].toIntOrNull() ?: return ampm(start)) + mins
    val end = "%02d:%02d".format((total / 60) % 24, total % 60)
    return "${ampm(start)} – ${ampm(end)}"
}
