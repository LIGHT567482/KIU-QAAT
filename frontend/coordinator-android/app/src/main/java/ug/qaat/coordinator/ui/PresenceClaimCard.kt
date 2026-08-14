package ug.qaat.coordinator.ui

import android.Manifest
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import ug.qaat.coordinator.db.PresenceClaimEntity
import ug.qaat.coordinator.di.Graph
import ug.qaat.coordinator.presence.PresenceCapture
import ug.qaat.coordinator.sync.PresenceSync
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter

/**
 * "I am in the room" — the lecturer's side of the monitor record, in their Alerts tab.
 *
 * It sits under Alerts because that is where a lecturer arrives after being told they were
 * recorded as NOT TAUGHT: the answer belongs next to the accusation, not three tabs away in a
 * settings screen nobody opens while standing in front of a class.
 *
 * ONE PRESS. There is a note field, and it is optional, because the moment this exists to capture
 * is a lecturer in a room who has about four seconds of attention to spare. Anything that must be
 * typed before the record is made is a record that does not get made.
 *
 * It never refuses. No signal, no satellite fix, nothing timetabled at that moment — the claim is
 * still filed, and says which of those happened. Refusing would lose the only thing that cannot be
 * reconstructed afterwards: that the lecturer said this at 14:07, and not after the complaint.
 */
@Composable
fun PresenceClaimCard() {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val dao = remember { Graph.db.dao() }

    var busy by remember { mutableStateOf(false) }
    var note by remember { mutableStateOf("") }
    var showNote by remember { mutableStateOf(false) }
    var last by remember { mutableStateOf<PresenceClaimEntity?>(null) }
    val claims by dao.recentPresenceClaims().collectAsState(initial = emptyList())
    val pending = claims.count { !it.synced }

    fun file() {
        busy = true
        scope.launch {
            last = runCatching { PresenceCapture.captureAndFile(ctx, dao, note) }.getOrNull()
            note = ""; showNote = false
            // Try to send it straight away. This normally FAILS — the phone is in a lecture room,
            // which is why the whole feature exists — and that is fine: the claim is already
            // durable, and PresenceSync will carry it up on the next pass that has signal.
            runCatching { withContext(Dispatchers.IO) { PresenceSync.syncPending() } }
            busy = false
        }
    }

    // Location is asked for HERE and only here. A lecturer who never presses this button is never
    // prompted, and the prompt arrives attached to the reason for it — which is also the only
    // moment the answer means anything to them.
    val askLocation = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { file() }

    Card(
        Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(14.dp),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant),
    ) {
        Column(Modifier.padding(14.dp)) {
            Text("Were you marked absent for a lecture you taught?",
                fontWeight = FontWeight.SemiBold, fontSize = 15.sp)
            Spacer(Modifier.height(4.dp))
            Text(
                "Press this while you are in the room. It records where you are, the time, and " +
                    "which lecture that is on your timetable — and files it even with no network. " +
                    "Quality Assurance reads it beside the monitor record for the same lecture.",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Spacer(Modifier.height(10.dp))

            if (showNote) {
                OutlinedTextField(
                    note, { note = it },
                    label = { Text("Anything to add (optional)") },
                    placeholder = { Text("e.g. moved to LR7, projector failed in LR3") },
                    singleLine = false, maxLines = 3, modifier = Modifier.fillMaxWidth(),
                )
                Spacer(Modifier.height(8.dp))
            }

            Row(verticalAlignment = Alignment.CenterVertically) {
                Button(
                    enabled = !busy,
                    onClick = {
                        if (androidx.core.content.ContextCompat.checkSelfPermission(
                                ctx, Manifest.permission.ACCESS_FINE_LOCATION,
                            ) == android.content.pm.PackageManager.PERMISSION_GRANTED
                        ) file() else askLocation.launch(Manifest.permission.ACCESS_FINE_LOCATION)
                    },
                    modifier = Modifier.weight(1f),
                ) { Text(if (busy) "Recording…" else "📍  I am in the room") }
                Spacer(Modifier.width(8.dp))
                TextButton(onClick = { showNote = !showNote }, enabled = !busy) {
                    Text(if (showNote) "Hide note" else "Add note")
                }
            }

            if (busy) {
                Spacer(Modifier.height(8.dp))
                LinearProgressIndicator(Modifier.fillMaxWidth())
                Spacer(Modifier.height(4.dp))
                Text("Getting a location fix — this can take a moment indoors.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
            }

            last?.let { c ->
                Spacer(Modifier.height(10.dp))
                Surface(color = MaterialTheme.colorScheme.primaryContainer, shape = RoundedCornerShape(10.dp)) {
                    Column(Modifier.padding(10.dp).fillMaxWidth()) {
                        Text("✓ Recorded at ${timeOf(c.capturedAt)}", fontWeight = FontWeight.SemiBold,
                            color = MaterialTheme.colorScheme.onPrimaryContainer)
                        Text(describe(c), style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onPrimaryContainer)
                    }
                }
            }

            if (claims.isNotEmpty()) {
                Spacer(Modifier.height(12.dp))
                HorizontalDivider()
                Spacer(Modifier.height(8.dp))
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text("Your records", fontWeight = FontWeight.SemiBold, modifier = Modifier.weight(1f))
                    // The count of what has not reached the server yet, stated plainly. A lecturer
                    // relying on this in a dispute has to be able to see whether it got there.
                    if (pending > 0) AssistChip(
                        onClick = { scope.launch { withContext(Dispatchers.IO) { runCatching { PresenceSync.syncPending() } } } },
                        label = { Text("$pending waiting — send now") },
                    )
                }
                Spacer(Modifier.height(4.dp))
                // THREE, not the lot. This card sits above the notification list, which is a
                // LazyColumn taking whatever height is left — a card that grows without bound
                // squeezes the inbox to nothing and then clips itself, since neither it nor its
                // parent scrolls. Three is enough to answer "did my last one go up?", and the
                // count below says what else is there.
                claims.take(3).forEach { c ->
                    Column(Modifier.fillMaxWidth().padding(vertical = 5.dp)) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(timeOf(c.capturedAt), fontWeight = FontWeight.SemiBold, fontSize = 13.sp)
                            Spacer(Modifier.width(8.dp))
                            Text(if (c.synced) "synced" else "waiting",
                                style = MaterialTheme.typography.labelSmall,
                                color = if (c.synced) MaterialTheme.colorScheme.primary
                                        else MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                        Text(describe(c), style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant)
                        if (c.note.isNotBlank()) Text("“${c.note}”",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                }
                if (claims.size > 3) Text(
                    "+ ${claims.size - 3} earlier — all of them are with Quality Assurance.",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
    }
}

/**
 * One line describing what was actually captured, in the words the lecturer needs to judge whether
 * it will help them. Vague reassurance would be worse than useless here: someone relying on this
 * in a dispute has to know if the phone got no fix, or matched the wrong lecture.
 */
private fun describe(c: PresenceClaimEntity): String {
    val where = when (c.locationStatus) {
        "OK" -> "Location recorded" + (c.accuracyMetres?.let { " (±${it.toInt()} m)" } ?: "")
        "NO_FIX" -> "No location fix — the time and lecture are still recorded"
        "PERMISSION_DENIED" -> "Location permission was refused — only the time is recorded"
        "DISABLED" -> "Location is turned off on this phone — only the time is recorded"
        else -> "Location unavailable"
    }
    val which = when (c.matchKind) {
        "IN_SLOT" -> "during ${c.unitName} (${c.scheduledTime})"
        "NEAR_SLOT" -> {
            val m = c.minutesFromStart ?: 0
            if (m < 0) "${-m} min before ${c.unitName} (${c.scheduledTime})"
            else "${m} min into ${c.unitName} (${c.scheduledTime})"
        }
        "NEAREST" -> "nothing running — nearest was ${c.unitName} at ${c.scheduledTime}"
        else -> "nothing timetabled for you today"
    }
    return "$where · $which"
}

/** The phone's own local time for a stored RFC3339 instant. */
private fun timeOf(iso: String): String = runCatching {
    DateTimeFormatter.ofPattern("EEE d MMM, HH:mm")
        .format(Instant.parse(iso).atZone(ZoneId.systemDefault()))
}.getOrDefault(iso)
