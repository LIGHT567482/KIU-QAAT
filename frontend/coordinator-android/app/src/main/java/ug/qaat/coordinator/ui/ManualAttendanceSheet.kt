package ug.qaat.coordinator.ui

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch
import ug.qaat.coordinator.net.ManualEntry
import ug.qaat.coordinator.net.PatrolClient
import ug.qaat.coordinator.net.PatrolReference
import ug.qaat.coordinator.net.RefItem
import java.time.LocalDate
import java.time.LocalTime
import java.time.format.DateTimeFormatter

/**
 * MANUAL ATTENDANCE — the lecture that was taught but never timetabled.
 *
 * The round is generated from the timetable, so every tick is ABOUT a slot. A great many real
 * lectures have no slot: a unit added after the schedule was locked, a make-up hour agreed in a
 * corridor, a class moved into a free room, a visiting lecturer covering a week. Standing in front
 * of one of those, a monitor could previously do nothing, or tick whichever slot looked closest —
 * which files a true observation under the wrong lecture, and that is worse than the silence.
 *
 * EVERY FIELD IS PICK-OR-TYPE, and that is the whole design. A list alone would refuse to record
 * the exact lectures this exists for (the unit not yet in the curriculum, the lecturer hired last
 * week). Free text alone would produce a drift of near-miss spellings that no report can group. So
 * the known list is offered first and a typed value is always accepted — and picking a unit fills
 * in its class/group and college, because the monitor should not retype what the system knows.
 */
@Composable
fun ManualAttendanceSheet(onDismiss: () -> Unit, onRecorded: (String) -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()

    var ref by remember { mutableStateOf(PatrolReference()) }
    var loading by remember { mutableStateOf(true) }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }

    var roomId by remember { mutableStateOf("") }
    var roomText by remember { mutableStateOf("") }
    var unitId by remember { mutableStateOf("") }
    var unitText by remember { mutableStateOf("") }
    var lecturerId by remember { mutableStateOf("") }
    var lecturerText by remember { mutableStateOf("") }
    var classGroup by remember { mutableStateOf("") }
    var school by remember { mutableStateOf("") }
    var students by remember { mutableStateOf("") }
    var taught by remember { mutableStateOf(true) }
    var remarks by remember { mutableStateOf("") }
    var isComp by remember { mutableStateOf(false) }
    var compFor by remember { mutableStateOf("") }
    val time = remember { LocalTime.now().format(DateTimeFormatter.ofPattern("HH:mm")) }

    LaunchedEffect(Unit) {
        val token = AppState.token
        if (token != null) {
            ref = runCatching { PatrolClient().reference(token, Fingerprint.get(ctx)) }
                .getOrElse { PatrolReference() }
        }
        loading = false
    }

    // Picking a unit carries its cohort and college across, so the two fields the monitor would
    // otherwise have to know off by heart arrive filled in — still editable, because the whole
    // point of this form is the case where the curriculum is not the last word.
    fun chooseUnit(item: RefItem?) {
        unitId = item?.id.orEmpty()
        unitText = item?.label.orEmpty()
        ref.unitDefaults[unitId]?.let { (cg, sch) ->
            if (classGroup.isBlank()) classGroup = cg
            if (school.isBlank()) school = sch
        }
    }

    ModalBottomSheet(onDismissRequest = onDismiss) {
        Column(
            Modifier.fillMaxWidth().padding(horizontal = 20.dp).padding(bottom = 24.dp)
                .verticalScroll(rememberScrollState()),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Text("Attendance taken manually", fontWeight = FontWeight.Bold, fontSize = 18.sp)
            Text(
                "For a lecture that is happening but is not on the timetable. Pick from the lists " +
                    "where you can, type where you cannot.",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )

            if (loading) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                    Spacer(Modifier.width(10.dp))
                    Text("Loading rooms, units and lecturers…")
                }
            }
            error?.let {
                Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.labelMedium)
            }

            PickOrType(
                label = "Room", options = ref.rooms,
                selectedId = roomId, typed = roomText,
                onPick = { roomId = it?.id.orEmpty(); roomText = it?.label.orEmpty() },
                onType = { roomText = it; roomId = "" },
            )
            PickOrType(
                label = "Course unit", options = ref.units,
                selectedId = unitId, typed = unitText,
                onPick = { chooseUnit(it) },
                onType = { unitText = it; unitId = "" },
            )
            OutlinedTextField(
                value = classGroup, onValueChange = { classGroup = it },
                label = { Text("Class / Group  (e.g. 2:1)") },
                singleLine = true, modifier = Modifier.fillMaxWidth(),
            )
            PickOrType(
                label = "Lecturer", options = ref.lecturers,
                selectedId = lecturerId, typed = lecturerText,
                onPick = { lecturerId = it?.id.orEmpty(); lecturerText = it?.label.orEmpty() },
                onType = { lecturerText = it; lecturerId = "" },
            )
            OutlinedTextField(
                value = students, onValueChange = { s -> students = s.filter { it.isDigit() }.take(4) },
                label = { Text("Number of students in the room") },
                keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(keyboardType = KeyboardType.Number),
                singleLine = true, modifier = Modifier.fillMaxWidth(),
            )
            OutlinedTextField(
                value = school, onValueChange = { school = it },
                label = { Text("School / College") },
                supportingText = {
                    Text(
                        if (unitId.isNotBlank()) "Taken from the course unit — change it if it is wrong."
                        else "No unit picked, so type the college."
                    )
                },
                singleLine = true, modifier = Modifier.fillMaxWidth(),
            )

            Row(verticalAlignment = Alignment.CenterVertically) {
                Checkbox(checked = isComp, onCheckedChange = { isComp = it })
                Text("This is a compensation lecture")
            }
            if (isComp) {
                OutlinedTextField(
                    value = compFor, onValueChange = { compFor = it },
                    label = { Text("For the lecture of (YYYY-MM-DD, optional)") },
                    singleLine = true, modifier = Modifier.fillMaxWidth(),
                )
            }
            OutlinedTextField(
                value = remarks, onValueChange = { remarks = it },
                label = { Text("Remarks (optional)") }, modifier = Modifier.fillMaxWidth(),
            )
            Text(
                "Recorded as today at $time.",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )

            // The verdict is still two deliberate buttons rather than a toggle and a Save, for the
            // same reason it is on the round: a record that says whether a named person was
            // teaching should never be one stray tap away.
            Text("Is the lecturer teaching?", fontWeight = FontWeight.SemiBold)
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                fun submit(isTeaching: Boolean) {
                    taught = isTeaching
                    busy = true; error = null
                    scope.launch {
                        val token = AppState.token
                        if (token == null) { error = "Not signed in"; busy = false; return@launch }
                        val entry = ManualEntry(
                            roomId = roomId, room = roomText,
                            unitId = unitId, unitName = unitText,
                            lecturerStaffId = lecturerId, lecturerName = lecturerText,
                            classGroup = classGroup.trim(), school = school.trim(),
                            studentsCounted = students.toIntOrNull() ?: 0,
                            sessionDate = LocalDate.now().toString(), timeOfDay = time,
                            taught = isTeaching, remarks = remarks.trim(),
                            isCompensation = isComp, compensationFor = compFor.trim(),
                        )
                        val msg = runCatching { PatrolClient().manual(token, Fingerprint.get(ctx), entry) }
                            .getOrElse { it.message ?: "Could not record it" }
                        busy = false
                        if (msg == null) { onRecorded("Recorded — ${unitText.ifBlank { unitId }}"); onDismiss() }
                        else error = msg
                    }
                }
                Button(
                    modifier = Modifier.weight(1f).height(54.dp), enabled = !busy,
                    onClick = { submit(true) },
                ) { Text("✓  Teaching", fontWeight = FontWeight.Bold) }
                Button(
                    modifier = Modifier.weight(1f).height(54.dp), enabled = !busy,
                    colors = ButtonDefaults.buttonColors(containerColor = MaterialTheme.colorScheme.error),
                    onClick = { submit(false) },
                ) { Text("✗  Not teaching", fontWeight = FontWeight.Bold) }
            }
        }
    }
}

/**
 * One field that is a dropdown AND a text box.
 *
 * Two controls for one fact would be a trap — the monitor fills in the box, ignores the list, and
 * the form has to guess which they meant. Here they are the same field: choosing from the list
 * writes the text, and editing the text clears the choice, so what is on screen is always exactly
 * what will be sent.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun PickOrType(
    label: String,
    options: List<RefItem>,
    selectedId: String,
    typed: String,
    onPick: (RefItem?) -> Unit,
    onType: (String) -> Unit,
) {
    var open by remember { mutableStateOf(false) }
    val matches = remember(typed, options) {
        if (typed.isBlank()) options.take(60)
        else options.filter {
            it.label.contains(typed, true) || it.id.contains(typed, true)
        }.take(60)
    }

    Column(Modifier.fillMaxWidth()) {
        ExposedDropdownMenuBox(expanded = open, onExpandedChange = { open = it }) {
            OutlinedTextField(
                value = typed,
                onValueChange = { onType(it); open = true },
                label = { Text(label) },
                singleLine = true,
                trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded = open) },
                supportingText = {
                    Text(
                        if (selectedId.isNotBlank()) "From the list: $selectedId"
                        else if (typed.isNotBlank()) "Typed — not on the list, which is allowed"
                        else "Choose one, or type it"
                    )
                },
                modifier = Modifier.fillMaxWidth().menuAnchor(),
            )
            ExposedDropdownMenu(expanded = open, onDismissRequest = { open = false }) {
                if (matches.isEmpty()) {
                    DropdownMenuItem(
                        text = { Text("Nothing matches — what you typed will be used") },
                        onClick = { open = false },
                    )
                }
                matches.forEach { item ->
                    DropdownMenuItem(
                        text = {
                            Column {
                                Text(item.label)
                                if (item.id.isNotBlank() || item.extra.isNotBlank()) {
                                    Text(
                                        listOf(item.id, item.extra).filter { it.isNotBlank() }.joinToString(" · "),
                                        style = MaterialTheme.typography.labelSmall,
                                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    )
                                }
                            }
                        },
                        onClick = { onPick(item); open = false },
                    )
                }
            }
        }
    }
}
