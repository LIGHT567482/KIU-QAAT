package ug.qaat.coordinator.ui

import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import ug.qaat.coordinator.net.LecturerTimetableClient
import ug.qaat.coordinator.net.OnlineClassClient
import ug.qaat.coordinator.student.Fingerprint

/**
 * THE DISTANCE CLASS, ON THE PHONE.
 *
 * This used to live only on the web dashboard. That dashboard is gone, and without moving this
 * first, removing it would have quietly taken distance-learning attendance with it: an e-learning
 * lecture has no coordinator to open a room, so if the lecturer cannot start the class, no student
 * on that cohort can check in to anything, and attendance decides exam eligibility.
 *
 * The lecturer shares this screen in the live class. Everything needed is on it: the rotating code,
 * how long it lasts, and how many of the cohort have checked in so far.
 */
@Composable
fun OnlineClassPanel(slots: List<LecturerTimetableClient.Slot>) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()

    // Only the units that actually have an e-learning cohort. A "start an online class" button on a
    // lecturer who teaches nothing online is an invitation to try it and be refused.
    val onlineUnits = remember(slots) {
        slots.filter { it.online }.distinctBy { it.unitId }
    }

    var live by remember { mutableStateOf<OnlineClassClient.Live?>(null) }
    var busy by remember { mutableStateOf(false) }
    var err by remember { mutableStateOf<String?>(null) }
    var pick by remember { mutableStateOf(0) }

    LaunchedEffect(Unit) { live = OnlineClassClient().current() }
    // Poll only while a class is running. The code rotates every 10s, so 5s keeps what is on screen
    // usable; with nothing running there is nothing to poll for.
    LaunchedEffect(live?.sessionId) {
        while (live != null) {
            delay(5000)
            OnlineClassClient().current()?.let { live = it }
        }
    }

    if (onlineUnits.isEmpty() && live == null) return
    val chosen = onlineUnits.getOrNull(pick)

    Surface(
        color = if (live != null) MaterialTheme.colorScheme.primaryContainer
                else MaterialTheme.colorScheme.surface,
        shape = MaterialTheme.shapes.medium,
        tonalElevation = 2.dp,
        modifier = Modifier.fillMaxWidth().padding(vertical = 8.dp),
    ) {
        Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Text("Distance / e-learning class", fontWeight = FontWeight.Bold)
            Text(
                if (live != null) "Running — share this screen with your class"
                else "No room, so you start it yourself. You are marked present from the moment you do.",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            err?.let { Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.labelMedium) }

            val running = live
            if (running == null) {
                if (onlineUnits.size > 1) {
                    Text("Which unit?", style = MaterialTheme.typography.labelMedium)
                    LazyRowChips(
                        options = onlineUnits,
                        selected = chosen?.unitId,
                        idOf = { it.unitId },
                        labelFor = { "${it.unitId} · ${it.sessionType}" },
                        onSelect = { s -> pick = onlineUnits.indexOfFirst { it.unitId == s.unitId } },
                    )
                }
                chosen?.let { c ->
                    Text("${c.unitId} — ${c.unitName}", fontWeight = FontWeight.SemiBold)
                    if (c.cohort.isNotBlank()) {
                        Text(c.cohort, style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                }
                Button(
                    enabled = !busy && chosen != null,
                    modifier = Modifier.fillMaxWidth().height(52.dp),
                    onClick = {
                        val c = chosen ?: return@Button
                        busy = true; err = null
                        scope.launch {
                            OnlineClassClient().start(c.unitId, c.offeringId, Fingerprint.get(ctx))
                                .onSuccess { live = it }
                                .onFailure { err = it.message }
                            busy = false
                        }
                    },
                ) { Text(if (busy) "Starting…" else "Start the class", fontWeight = FontWeight.Bold) }
            } else {
                Text("${running.unitId} — ${running.unitName}", fontWeight = FontWeight.SemiBold)
                if (running.cohortLabel.isNotBlank()) {
                    Text(running.cohortLabel, style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                Row(verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(18.dp)) {
                    Column {
                        Text("CHECK-IN CODE", style = MaterialTheme.typography.labelSmall,
                            fontWeight = FontWeight.Bold)
                        Text(running.studentCode, fontSize = 40.sp, fontWeight = FontWeight.Bold,
                            fontFamily = FontFamily.Monospace, lineHeight = 44.sp)
                        Text("changes in ${running.secondsRemaining}s — read it out, do not paste it",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    Column {
                        Text("CHECKED IN", style = MaterialTheme.typography.labelSmall,
                            fontWeight = FontWeight.Bold)
                        Text("${running.present}/${running.enrolled}", fontSize = 28.sp,
                            fontWeight = FontWeight.Bold)
                    }
                }
                Text(
                    "A student who is not enrolled in this cohort cannot check in, whatever code they have.",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                OutlinedButton(
                    enabled = !busy,
                    modifier = Modifier.fillMaxWidth(),
                    onClick = {
                        busy = true; err = null
                        scope.launch {
                            val e = OnlineClassClient().end()
                            if (e == null) live = null else err = e
                            busy = false
                        }
                    },
                ) { Text(if (busy) "Ending…" else "End the class — this books your contact hours") }
            }
        }
    }
}
