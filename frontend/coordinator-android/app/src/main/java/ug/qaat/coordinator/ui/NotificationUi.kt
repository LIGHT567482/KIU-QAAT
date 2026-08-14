package ug.qaat.coordinator.ui

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch
import ug.qaat.coordinator.net.LecturerRecipientsClient
import ug.qaat.coordinator.net.NotificationClient
import ug.qaat.coordinator.notify.AlertNotifier

/** A horizontal, single-select chip row (e.g. unit filter). */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun <T> LazyRowChips(
    options: List<T>, selected: String?, idOf: (T) -> String?, labelFor: (T) -> String, onSelect: (T) -> Unit,
) {
    LazyRow(horizontalArrangement = Arrangement.spacedBy(6.dp), contentPadding = PaddingValues(vertical = 4.dp)) {
        items(options) { opt ->
            FilterChip(selected = idOf(opt) == selected, onClick = { onSelect(opt) }, label = { Text(labelFor(opt)) })
        }
    }
}

/** Compose + send a notification. `audiences` = (value, label) pairs valid for the sender role. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun NotificationComposer(
    audiences: List<Pair<String, String>>,
    onSent: () -> Unit,
    /**
     * Named people this sender may address, if their role has any.
     *
     * A lecturer's "the coordinator" was a broadcast to every coordinator of every cohort of every
     * course they touch. One unit is routinely taught to four cohorts with four coordinators, and
     * the message is almost always for exactly one — so when this is supplied, a specific person
     * can be picked, and the broadcast stays available as a deliberate choice rather than the only
     * one.
     */
    recipients: LecturerRecipientsClient.Recipients? = null,
) {
    val scope = rememberCoroutineScope()
    var audience by remember { mutableStateOf(audiences.first().first) }
    var subject by remember { mutableStateOf("") }
    var body by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var err by remember { mutableStateOf<String?>(null) }
    // The chosen person, or null for "everyone in that audience".
    var targetId by remember { mutableStateOf<String?>(null) }
    var targetLabel by remember { mutableStateOf("") }
    var search by remember { mutableStateOf("") }

    // Changing audience clears the recipient: a coordinator's id sent to a STUDENTS audience is a
    // silently wrong message, and keeping the old selection visible would invite exactly that.
    fun pickAudience(k: String) {
        audience = k; targetId = null; targetLabel = ""; search = ""
    }

    val coords = recipients?.coordinators.orEmpty()
    val studs = recipients?.students.orEmpty()
    val picking = when (audience) {
        "COORDINATOR", "COORDINATORS" -> coords.isNotEmpty()
        "STUDENT", "STUDENTS" -> studs.isNotEmpty()
        else -> false
    }
    // STUDENTS means the whole class; the singular is what needs a person. Sending to one student
    // therefore switches the audience as well as setting the target.
    val effectiveAudience = when {
        audience == "STUDENTS" && targetId != null -> "STUDENT"
        audience == "STUDENT" && targetId == null -> "STUDENTS"
        else -> audience
    }

    Surface(color = MaterialTheme.colorScheme.surface, shape = MaterialTheme.shapes.medium,
        modifier = Modifier.fillMaxWidth().padding(vertical = 8.dp)) {
        Column(Modifier.padding(12.dp)) {
            Text("Send to:", style = MaterialTheme.typography.labelMedium)
            Row {
                audiences.forEach { (k, lbl) ->
                    FilterChip(selected = audience == k, onClick = { pickAudience(k) },
                        label = { Text(lbl) }, modifier = Modifier.padding(end = 6.dp))
                }
            }

            if (picking) {
                Spacer(Modifier.height(6.dp))
                val isCoord = audience.startsWith("COORDINATOR")
                val all = if (isCoord) "Everyone (${coords.size})" else "The whole class (${studs.size})"
                Text(
                    if (targetId == null) "Going to: $all" else "Going to: $targetLabel",
                    style = MaterialTheme.typography.labelMedium,
                    fontWeight = FontWeight.Bold,
                    color = if (targetId == null) MaterialTheme.colorScheme.error
                            else MaterialTheme.colorScheme.primary,
                )
                if (targetId == null && isCoord && coords.size > 1) {
                    Text(
                        "You have ${coords.size} coordinators, one per cohort. Pick the one you mean, " +
                            "or send to all of them on purpose.",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }

                // A search box only where a list is long enough to need one. A lecturer with three
                // coordinators does not want to type; one with four hundred students does.
                if ((isCoord && coords.size > 8) || (!isCoord && studs.size > 8)) {
                    OutlinedTextField(
                        search, { search = it },
                        label = { Text(if (isCoord) "Find a coordinator" else "Find a student") },
                        singleLine = true, modifier = Modifier.fillMaxWidth().padding(top = 4.dp),
                    )
                }

                val q = search.trim().lowercase()
                LazyRow(horizontalArrangement = Arrangement.spacedBy(6.dp),
                    contentPadding = PaddingValues(vertical = 4.dp)) {
                    item {
                        FilterChip(selected = targetId == null,
                            onClick = { targetId = null; targetLabel = "" }, label = { Text(all) })
                    }
                    if (isCoord) {
                        items(coords.filter { q.isBlank() ||
                            it.label.lowercase().contains(q) || it.course.lowercase().contains(q) }) { c ->
                            FilterChip(
                                selected = targetId == c.userId,
                                onClick = { targetId = c.userId; targetLabel = c.label },
                                label = { Text(c.label) },
                            )
                        }
                    } else {
                        items(studs.filter { q.isBlank() ||
                            it.label.lowercase().contains(q) || it.cohort.lowercase().contains(q) }) { st ->
                            FilterChip(
                                selected = targetId == st.studentId,
                                onClick = { targetId = st.studentId; targetLabel = st.label },
                                label = { Text(st.label) },
                            )
                        }
                    }
                }
            }

            err?.let { Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.labelSmall) }
            OutlinedTextField(subject, { subject = it }, label = { Text("Subject") }, singleLine = true, modifier = Modifier.fillMaxWidth())
            Spacer(Modifier.height(6.dp))
            OutlinedTextField(body, { body = it }, label = { Text("Message") }, modifier = Modifier.fillMaxWidth().heightIn(min = 80.dp))
            Button(enabled = !busy && subject.isNotBlank(), modifier = Modifier.padding(top = 8.dp), onClick = {
                busy = true; err = null
                scope.launch {
                    err = NotificationClient().send(effectiveAudience, null, subject.trim(), body, targetId)
                    busy = false
                    if (err == null) onSent()
                }
            }) { Text(if (busy) "Sending…" else "Send") }
        }
    }
}

/** Shared inbox list with tap-to-expand + mark-read. `onChanged` is called after a read so the
 *  caller can refresh the list / unread badge. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun NotificationInboxList(items: List<NotificationClient.Notif>?, onChanged: () -> Unit) {
    val scope = rememberCoroutineScope()
    // Ids the server has ACCEPTED a dismissal for, plus the ones still in flight. Held here so the
    // ✕ feels instant and — more importantly — so the card stays gone across the refresh that
    // follows it. Without this the reload races the dismissal and the alert flickers back.
    val dismissed = remember { mutableStateListOf<String>() }
    var dismissError by remember { mutableStateOf<String?>(null) }
    val visible = items?.filterNot { it.id in dismissed }
    Column {
        // A dismissal that did not stick has to say why, rather than silently undoing itself.
        dismissError?.let {
            Text(it, color = MaterialTheme.colorScheme.error,
                style = MaterialTheme.typography.labelMedium, modifier = Modifier.padding(bottom = 6.dp))
        }
        when {
            items == null -> Box(Modifier.fillMaxWidth().padding(top = 30.dp), Alignment.Center) { CircularProgressIndicator() }
            visible.isNullOrEmpty() -> Text("No notifications yet.", color = MaterialTheme.colorScheme.onSurfaceVariant)
            else -> LazyColumn(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                items(visible, key = { it.id }) { n ->
                    NotificationCard(
                        n = n,
                        onOpen = { if (!n.read) scope.launch { NotificationClient().markRead(n.id); onChanged() } },
                        onDismiss = {
                            dismissed.add(n.id)
                            dismissError = null
                            scope.launch {
                                // Never notify about it again, whatever the server says — the reader
                                // has explicitly cleared it on this phone.
                                AlertNotifier.suppress(n.id)
                                val err = NotificationClient().dismiss(n.id)
                                if (err != null) {
                                    dismissed.remove(n.id)   // it is still in the inbox; say so
                                    dismissError = err
                                }
                                onChanged()
                            }
                        },
                    )
                }
            }
        }
    }
}
