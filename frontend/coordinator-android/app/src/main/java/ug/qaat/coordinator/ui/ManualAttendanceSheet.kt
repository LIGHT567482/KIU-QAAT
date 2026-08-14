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
import ug.qaat.coordinator.net.ExtraUnit
import ug.qaat.coordinator.net.ManualEntry
import ug.qaat.coordinator.net.PatrolClient
import ug.qaat.coordinator.net.PatrolReference
import ug.qaat.coordinator.net.RefItem
import ug.qaat.coordinator.student.Fingerprint
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
@OptIn(ExperimentalMaterial3Api::class)
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
    var department by remember { mutableStateOf("") }
    var students by remember { mutableStateOf("") }
    var taught by remember { mutableStateOf(true) }
    var remarks by remember { mutableStateOf("") }
    var isComp by remember { mutableStateOf(false) }
    // The lecture being made good: a real date and a real time, both required. Free text here is
    // what made the old field decorative — "last week" cannot be matched to any missed lecture.
    var compDate by remember { mutableStateOf("") }   // YYYY-MM-DD
    var compTime by remember { mutableStateOf("") }   // HH:MM
    // WHEN THE LECTURE RUNS, both ends. The start alone says nothing about what the hour was worth.
    var begins by remember { mutableStateOf(LocalTime.now().format(DateTimeFormatter.ofPattern("HH:mm"))) }
    var ends by remember { mutableStateOf("") }
    // The OTHER unit codes this same hour delivered — see the header.
    var extras by remember { mutableStateOf(listOf<ExtraUnit>()) }

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
        ref.unitDepartments[unitId]?.takeIf { it.isNotBlank() }?.let {
            if (department.isBlank()) department = it
        }
    }

    // Every college and department this one lecture belongs to, each named ONCE. Two unit codes in
    // the same college is the common case, and repeating its name would read as two colleges;
    // two codes in different ones is exactly what a reader has to be able to see at a glance.
    val colleges = (listOf(school) + extras.map { it.school })
        .map { it.trim() }.filter { it.isNotBlank() }.distinctBy { it.lowercase() }
    val departments = (listOf(department) + extras.map { it.department })
        .map { it.trim() }.filter { it.isNotBlank() }.distinctBy { it.lowercase() }

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
            OutlinedTextField(
                value = department, onValueChange = { department = it },
                label = { Text("Department") },
                supportingText = {
                    Text("Two codes for one lecture usually share a college and differ by department.")
                },
                singleLine = true, modifier = Modifier.fillMaxWidth(),
            )

            // ── The other unit codes this same hour delivers ────────────────────────────────
            ExtraUnitsSection(
                ref = ref, extras = extras, onChange = { extras = it },
                primaryUnitId = unitId, primarySchool = school,
            )
            if (colleges.size > 1 || departments.size > 1) {
                Surface(
                    color = MaterialTheme.colorScheme.secondaryContainer,
                    shape = MaterialTheme.shapes.small, modifier = Modifier.fillMaxWidth(),
                ) {
                    Column(Modifier.padding(10.dp)) {
                        Text("This lecture spans", fontWeight = FontWeight.SemiBold, fontSize = 12.sp)
                        if (colleges.isNotEmpty()) Text("Colleges: " + colleges.joinToString(", "), fontSize = 12.sp)
                        if (departments.isNotEmpty()) Text("Departments: " + departments.joinToString(", "), fontSize = 12.sp)
                    }
                }
            }

            // ── When it runs ────────────────────────────────────────────────────────────────
            Text("Lecture time", fontWeight = FontWeight.SemiBold)
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                TimeField("Begins", begins, Modifier.weight(1f)) { begins = it }
                TimeField("Ends", ends, Modifier.weight(1f)) { ends = it }
            }
            val badSpan = ends.isNotBlank() && !endsAfterHHMM(begins, ends)
            if (badSpan) {
                Text("The end has to be after the start.", color = MaterialTheme.colorScheme.error,
                    style = MaterialTheme.typography.labelSmall)
            }

            Row(verticalAlignment = Alignment.CenterVertically) {
                Checkbox(checked = isComp, onCheckedChange = { isComp = it })
                Text("This is a compensation lecture")
            }
            if (isComp) {
                // REQUIRED, and a real date and time — not typed prose. A compensation that cannot
                // be matched to the lecture it replaces cannot be counted as having replaced it,
                // and a lecturer missed a Tuesday with two timetabled hours is owed the distinction
                // between them.
                Text(
                    "Which lecture is this making good?",
                    fontWeight = FontWeight.SemiBold, style = MaterialTheme.typography.bodyMedium,
                )
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    DateField("Its date", compDate, Modifier.weight(1f)) { compDate = it }
                    TimeField("It began at", compTime, Modifier.weight(1f)) { compTime = it }
                }
                Text(
                    if (compDate.isBlank() || compTime.isBlank())
                        "Both are needed. Without the time, a Tuesday with two lectures cannot say which one this replaces."
                    else "Making good the lecture of $compDate at $compTime.",
                    style = MaterialTheme.typography.labelSmall,
                    color = if (compDate.isBlank() || compTime.isBlank())
                        MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            OutlinedTextField(
                value = remarks, onValueChange = { remarks = it },
                label = { Text("Remarks (optional)") }, modifier = Modifier.fillMaxWidth(),
            )
            Text(
                "Recorded as today, $begins" + if (ends.isNotBlank()) "–$ends" else "" + ".",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )

            // The verdict is still two deliberate buttons rather than a toggle and a Save, for the
            // same reason it is on the round: a record that says whether a named person was
            // teaching should never be one stray tap away.
            Text("Is the lecturer teaching?", fontWeight = FontWeight.SemiBold)
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                // The verdict buttons are disabled rather than allowed to fail on the server: a
                // monitor in a corridor who taps "Teaching" and gets a rejection has to work out
                // which of eight fields was wrong, and the two that can be wrong are named here.
                val blocked = badSpan || (isComp && (compDate.isBlank() || compTime.isBlank()))

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
                            department = department.trim(),
                            studentsCounted = students.toIntOrNull() ?: 0,
                            sessionDate = LocalDate.now().toString(),
                            timeOfDay = begins, endTime = ends,
                            taught = isTeaching, remarks = remarks.trim(),
                            isCompensation = isComp,
                            compensationForAt = if (isComp) "$compDate $compTime" else "",
                            alsoUnits = extras,
                        )
                        val msg = runCatching { PatrolClient().manual(token, Fingerprint.get(ctx), entry) }
                            .getOrElse { it.message ?: "Could not record it" }
                        busy = false
                        if (msg == null) { onRecorded("Recorded — ${unitText.ifBlank { unitId }}"); onDismiss() }
                        else error = msg
                    }
                }
                Button(
                    modifier = Modifier.weight(1f).height(54.dp), enabled = !busy && !blocked,
                    onClick = { submit(true) },
                ) { Text("✓  Teaching", fontWeight = FontWeight.Bold) }
                Button(
                    modifier = Modifier.weight(1f).height(54.dp), enabled = !busy && !blocked,
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

/**
 * THE OTHER UNIT CODES THIS SAME HOUR DELIVERS.
 *
 * One class, one lecturer, one room — and two or three unit codes, because the same taught content
 * is required by several programmes and each codes and names it differently. A monitor who could
 * only record one of them left every student on the other codes with a lecture QA never saw, and
 * credited the lecturer with one unit for an hour that delivered three.
 *
 * Picking one fills in its college and department from the curriculum, which is what distinguishes
 * the codes from one another — they usually share a college and differ by department. A code that
 * is not in the curriculum can still be typed, on the same terms as the primary unit: the whole
 * point of this form is the lecture the system does not know about.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ExtraUnitsSection(
    ref: PatrolReference,
    extras: List<ExtraUnit>,
    onChange: (List<ExtraUnit>) -> Unit,
    primaryUnitId: String,
    primarySchool: String,
) {
    Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Text("Other course units in this same lecture", fontWeight = FontWeight.SemiBold)
        Text(
            "Add the other codes this hour also covers, so their students are not left without a record.",
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )

        extras.forEachIndexed { i, e ->
            Surface(shape = MaterialTheme.shapes.small, tonalElevation = 1.dp,
                modifier = Modifier.fillMaxWidth()) {
                Column(Modifier.padding(10.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Text("Unit ${i + 2}", fontWeight = FontWeight.SemiBold, fontSize = 12.sp,
                            modifier = Modifier.weight(1f))
                        TextButton(onClick = { onChange(extras.filterIndexed { j, _ -> j != i }) }) {
                            Text("Remove")
                        }
                    }
                    PickOrType(
                        label = "Course unit", options = ref.units,
                        selectedId = e.unitId, typed = e.unitName,
                        onPick = { item ->
                            val id = item?.id.orEmpty()
                            val d = ref.unitDefaults[id]
                            onChange(extras.toMutableList().also { l ->
                                l[i] = e.copy(
                                    unitId = id, unitName = item?.label.orEmpty(),
                                    classGroup = e.classGroup.ifBlank { d?.first.orEmpty() },
                                    // A second code in the same college inherits it rather than
                                    // being left blank — the college belongs to the lecture as much
                                    // as to the unit, and a blank would read as "unknown".
                                    school = e.school.ifBlank { d?.second?.ifBlank { primarySchool } ?: primarySchool },
                                    department = e.department.ifBlank { ref.unitDepartments[id].orEmpty() },
                                )
                            })
                        },
                        onType = { t ->
                            onChange(extras.toMutableList().also { l ->
                                l[i] = e.copy(unitName = t, unitId = "")
                            })
                        },
                    )
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        OutlinedTextField(
                            value = e.classGroup,
                            onValueChange = { v -> onChange(extras.toMutableList().also { l -> l[i] = e.copy(classGroup = v) }) },
                            label = { Text("Class / Group") }, singleLine = true, modifier = Modifier.weight(1f),
                        )
                        OutlinedTextField(
                            value = e.department,
                            onValueChange = { v -> onChange(extras.toMutableList().also { l -> l[i] = e.copy(department = v) }) },
                            label = { Text("Department") }, singleLine = true, modifier = Modifier.weight(1f),
                        )
                    }
                    OutlinedTextField(
                        value = e.school,
                        onValueChange = { v -> onChange(extras.toMutableList().also { l -> l[i] = e.copy(school = v) }) },
                        label = { Text("School / College") },
                        supportingText = {
                            Text(
                                if (e.school.isNotBlank() && e.school.equals(primarySchool, true))
                                    "Same college as the first unit — it is recorded once."
                                else "A different college from the first unit."
                            )
                        },
                        singleLine = true, modifier = Modifier.fillMaxWidth(),
                    )
                    if (e.unitId.isNotBlank() && e.unitId.equals(primaryUnitId, true)) {
                        Text("This is the same unit as the first one — it will not be recorded twice.",
                            color = MaterialTheme.colorScheme.error,
                            style = MaterialTheme.typography.labelSmall)
                    }
                }
            }
        }

        OutlinedButton(
            onClick = { onChange(extras + ExtraUnit(school = primarySchool)) },
            modifier = Modifier.fillMaxWidth(),
        ) { Text(if (extras.isEmpty()) "+ Add another course unit" else "+ Add one more") }
    }
}

/** A time field that only ever holds HH:MM. Typed digits are shaped as they are entered, so the
 *  form cannot carry prose where the record needs a clock time. */
@Composable
private fun TimeField(label: String, value: String, modifier: Modifier = Modifier, onChange: (String) -> Unit) {
    OutlinedTextField(
        value = value,
        onValueChange = { raw ->
            val d = raw.filter { it.isDigit() }.take(4)
            onChange(when {
                d.length <= 2 -> d
                else -> d.substring(0, 2) + ":" + d.substring(2)
            })
        },
        label = { Text(label) }, placeholder = { Text("HH:MM") },
        keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(keyboardType = KeyboardType.Number),
        singleLine = true, modifier = modifier,
        isError = value.isNotBlank() && !isHHMM(value),
    )
}

/** A date field that only ever holds YYYY-MM-DD, for the same reason. */
@Composable
private fun DateField(label: String, value: String, modifier: Modifier = Modifier, onChange: (String) -> Unit) {
    OutlinedTextField(
        value = value,
        onValueChange = { raw ->
            val d = raw.filter { it.isDigit() }.take(8)
            onChange(when {
                d.length <= 4 -> d
                d.length <= 6 -> d.substring(0, 4) + "-" + d.substring(4)
                else -> d.substring(0, 4) + "-" + d.substring(4, 6) + "-" + d.substring(6)
            })
        },
        label = { Text(label) }, placeholder = { Text("YYYY-MM-DD") },
        keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(keyboardType = KeyboardType.Number),
        singleLine = true, modifier = modifier,
        isError = value.isNotBlank() && runCatching { LocalDate.parse(value) }.isFailure,
    )
}

private fun isHHMM(s: String): Boolean =
    s.length == 5 && s[2] == ':' &&
        (s.substring(0, 2).toIntOrNull() ?: 99) in 0..23 &&
        (s.substring(3, 5).toIntOrNull() ?: 99) in 0..59

private fun hhmmToMinutes(s: String): Int =
    if (!isHHMM(s)) -1 else s.substring(0, 2).toInt() * 60 + s.substring(3, 5).toInt()

/** A class that "ends" before it began is a typo, and storing it would put a negative hour into
 *  contact-time reporting. */
internal fun endsAfterHHMM(begins: String, ends: String): Boolean {
    val b = hhmmToMinutes(begins); val e = hhmmToMinutes(ends)
    return b >= 0 && e >= 0 && e > b
}
