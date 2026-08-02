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
import ug.qaat.coordinator.net.NotificationClient

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
fun NotificationComposer(audiences: List<Pair<String, String>>, onSent: () -> Unit) {
    val scope = rememberCoroutineScope()
    var audience by remember { mutableStateOf(audiences.first().first) }
    var subject by remember { mutableStateOf("") }
    var body by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var err by remember { mutableStateOf<String?>(null) }

    Surface(color = MaterialTheme.colorScheme.surface, shape = MaterialTheme.shapes.medium,
        modifier = Modifier.fillMaxWidth().padding(vertical = 8.dp)) {
        Column(Modifier.padding(12.dp)) {
            Text("Send to:", style = MaterialTheme.typography.labelMedium)
            Row {
                audiences.forEach { (k, lbl) ->
                    FilterChip(selected = audience == k, onClick = { audience = k }, label = { Text(lbl) }, modifier = Modifier.padding(end = 6.dp))
                }
            }
            err?.let { Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.labelSmall) }
            OutlinedTextField(subject, { subject = it }, label = { Text("Subject") }, singleLine = true, modifier = Modifier.fillMaxWidth())
            Spacer(Modifier.height(6.dp))
            OutlinedTextField(body, { body = it }, label = { Text("Message") }, modifier = Modifier.fillMaxWidth().heightIn(min = 80.dp))
            Button(enabled = !busy && subject.isNotBlank(), modifier = Modifier.padding(top = 8.dp), onClick = {
                busy = true; err = null
                scope.launch { err = NotificationClient().send(audience, null, subject.trim(), body); busy = false; if (err == null) onSent() }
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
    // Locally dismissed ids, so the ✕ feels instant while the server call is in flight.
    val dismissed = remember { mutableStateListOf<String>() }
    val visible = items?.filterNot { it.id in dismissed }
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
                        scope.launch { NotificationClient().dismiss(n.id); onChanged() }
                    },
                )
            }
        }
    }
}
