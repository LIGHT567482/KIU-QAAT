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
import kotlinx.coroutines.launch
import ug.qaat.coordinator.net.FreeRoom
import ug.qaat.coordinator.net.FreeRoomsClient

/**
 * THE ROOM YOU CAN ACTUALLY USE.
 *
 * The timetabled room is locked, double-booked, being repainted, or an exam has it — and a class is
 * standing in the corridor. This lists every room in the institution that is free right now: all
 * colleges, all departments, all blocks, checked against the timetable AND against sessions other
 * coordinators have open at this second.
 *
 * WHY THE WHOLE INSTITUTION. The free room is usually somebody else's. A list bounded by the
 * coordinator's own department would hide exactly the rooms that are available, which is how this
 * ends with a class in a corridor while three empty halls sit one block away.
 *
 * PICKING ONE IS A DECLARATION, not a silent substitution. It marks the session as a provision, and
 * the QA monitors are told before they set off — because otherwise a monitor walks to the
 * timetabled room, finds it empty, and files "not taught" against a lecturer who is teaching thirty
 * metres away, and nobody can afterwards prove otherwise.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun FreeRoomPicker(
    onDismiss: () -> Unit,
    onChosen: (room: FreeRoom, reason: String) -> Unit,
) {
    val scope = rememberCoroutineScope()
    var rooms by remember { mutableStateOf<List<FreeRoom>?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var onlyFree by remember { mutableStateOf(true) }
    var chosen by remember { mutableStateOf<FreeRoom?>(null) }
    var reason by remember { mutableStateOf("") }

    suspend fun load() {
        val token = AppState.token
        if (token == null) { error = "Not signed in"; rooms = emptyList(); return }
        val list = runCatching { FreeRoomsClient().list(token) }
            .getOrElse { error = "Could not check the rooms — you need signal for this."; emptyList() }
        rooms = list
        if (list.isEmpty() && error == null) error = "No rooms came back."
    }
    LaunchedEffect(Unit) { load() }

    ModalBottomSheet(onDismissRequest = onDismiss) {
        Column(
            Modifier.fillMaxWidth().padding(horizontal = 20.dp).padding(bottom = 24.dp)
                .verticalScroll(rememberScrollState()),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Text("Find a free room", fontWeight = FontWeight.Bold, fontSize = 18.sp)
            Text(
                "Every room in the institution, checked against the timetable and against sessions " +
                    "running right now. Needs signal — a cached answer would send two classes to one room.",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )

            if (rooms == null) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                    Spacer(Modifier.width(10.dp)); Text("Checking every room…")
                }
            }
            error?.let { Text(it, color = MaterialTheme.colorScheme.error) }

            chosen?.let { r ->
                // Confirming is a second, deliberate step: the reason is the only account anyone
                // will ever have of WHY the lecture moved, and it is asked for while the
                // coordinator still remembers.
                Surface(
                    color = MaterialTheme.colorScheme.secondaryContainer,
                    shape = MaterialTheme.shapes.medium, modifier = Modifier.fillMaxWidth(),
                ) {
                    Column(Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        Text("Use ${r.name}?", fontWeight = FontWeight.Bold)
                        Text(
                            "${r.building.ifBlank { "no block listed" }} · ${r.school.ifBlank { "no college" }}" +
                                if (r.capacity > 0) " · seats ${r.capacity}" else "",
                            style = MaterialTheme.typography.labelSmall,
                        )
                        OutlinedTextField(
                            value = reason, onValueChange = { reason = it },
                            label = { Text("Why the timetabled room could not be used") },
                            supportingText = { Text("Optional, but it is the only record of the reason.") },
                            modifier = Modifier.fillMaxWidth(),
                        )
                        Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                            Button(
                                modifier = Modifier.weight(1f),
                                onClick = { onChosen(r, reason.trim()); onDismiss() },
                            ) { Text("Use this room") }
                            OutlinedButton(
                                modifier = Modifier.weight(1f),
                                onClick = { chosen = null; reason = "" },
                            ) { Text("Back") }
                        }
                        Text(
                            "The QA monitors, the QA office and the lecturer are told immediately, " +
                                "so nobody visits the empty room.",
                            style = MaterialTheme.typography.labelSmall,
                        )
                    }
                }
            }

            if (chosen == null) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Checkbox(checked = onlyFree, onCheckedChange = { onlyFree = it })
                    Text("Free only")
                    Spacer(Modifier.weight(1f))
                    TextButton(onClick = { scope.launch { rooms = null; error = null; load() } }) { Text("Refresh") }
                }
                // Grouped by block: a coordinator with a waiting class cares about distance first.
                val list = (rooms ?: emptyList()).filter { !onlyFree || it.free }
                if (rooms != null && list.isEmpty()) {
                    Text(
                        "Nothing is free right now anywhere in the institution. Untick “Free only” " +
                            "to see what is holding each room, and until when.",
                        style = MaterialTheme.typography.bodySmall,
                    )
                }
                list.groupBy { it.building.ifBlank { "Unlisted block" } }.forEach { (block, inBlock) ->
                    Text(
                        "$block · ${inBlock.count { it.free }} free of ${inBlock.size}",
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    inBlock.forEach { r ->
                        Surface(
                            shape = MaterialTheme.shapes.small, tonalElevation = 1.dp,
                            modifier = Modifier.fillMaxWidth(),
                        ) {
                            Row(
                                Modifier.padding(12.dp), verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(10.dp),
                            ) {
                                Column(Modifier.weight(1f)) {
                                    Text(r.name, fontWeight = FontWeight.SemiBold)
                                    Text(
                                        buildString {
                                            append(r.school.ifBlank { "no college" })
                                            if (r.capacity > 0) append(" · seats ${r.capacity}")
                                            if (!r.free) {
                                                append(" · ${r.occupiedBy}")
                                                if (r.occupiedUntil.isNotBlank()) append(" until ${r.occupiedUntil}")
                                                if (r.occupiedNote.isNotBlank()) append(" — ${r.occupiedNote}")
                                            }
                                        },
                                        style = MaterialTheme.typography.labelSmall,
                                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    )
                                }
                                if (r.free) {
                                    Button(onClick = { chosen = r }) { Text("Choose") }
                                } else {
                                    Text("IN USE", style = MaterialTheme.typography.labelSmall,
                                        color = MaterialTheme.colorScheme.error)
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}
