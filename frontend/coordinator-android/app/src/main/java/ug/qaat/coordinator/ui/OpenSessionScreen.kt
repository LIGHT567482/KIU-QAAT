package ug.qaat.coordinator.ui

import android.content.Intent
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import ug.qaat.coordinator.service.SessionService
import ug.qaat.coordinator.session.SessionController
import ug.qaat.coordinator.store.SessionStore

/**
 * Pick today's unit, start the hotspot/server, and open the session. The LECTURER is
 * identified AUTOMATICALLY from the chosen unit (the assignment carried in the manifest) —
 * the coordinator never types a staff ID. A manual field only appears as a fallback when a
 * unit has no assigned lecturer on file.
 */
@Composable
fun OpenSessionScreen(onOpened: () -> Unit) {
    val ctx = LocalContext.current
    // The manifest carries the FULL cohort unit list (so the Timetable stays populated). For the
    // ATTENDANCE picker, a coordinator may only start a unit that is actually TIMETABLED for today
    // (has a day + start time). Un-timetabled units are never startable here. No fall-back to
    // "all units". The unit is startable for the WHOLE day — it is no longer held back until
    // 10 minutes before its slot.
    val allUnits = AppState.manifest?.units.orEmpty()
    val todayDow = java.time.LocalDate.now().dayOfWeek.value   // 1=Mon … 7=Sun
    // val nowMin = java.time.LocalTime.now().let { it.hour * 60 + it.minute }
    fun startMinutes(hhmm: String): Int? {
        val p = hhmm.split(":"); val h = p.getOrNull(0)?.toIntOrNull(); val m = p.getOrNull(1)?.toIntOrNull()
        return if (h != null && m != null) h * 60 + m else null
    }
    // OFF-TIMETABLE LECTURES: a make-up class, a unit added after the schedule was locked, a
    // visiting lecturer covering a week. These used to be unstartable, and the cost fell on the
    // STUDENTS — no session means no room code, no check-in, and no attendance for a lecture they
    // sat through, in a system where attendance decides exam eligibility. They can be started now,
    // but only after a deliberate second step, so today's timetable stays the default answer.
    var offTimetable by remember { mutableStateOf(false) }
    val scheduledUnits = allUnits.filter { u ->
        val start = u.startTime.takeIf { it.isNotBlank() }?.let(::startMinutes)
        // The "slot is due" gate is COMMENTED OUT: a unit used to appear only from 10 minutes
        // before its timetabled start (`&& nowMin >= start - 10`). A coordinator may now open
        // any unit timetabled for today at any point in the day. Restore the clause — and the
        // `nowMin` line above — to bring the 10-minute rule back.
        u.dayOfWeek == todayDow && start != null
    }
    val units = if (offTimetable) allUnits else scheduledUnits
    // Kept on AppState so it travels with the session that is actually created, the same way the
    // provision room does — the choice and the creation are separated by the hotspot coming up.
    LaunchedEffect(offTimetable) { AppState.sessionUnscheduled = offTimetable }
    var selectedUnit by remember(units.size) { mutableStateOf(units.firstOrNull()?.unitId) }
    // THE ROOM, when the timetabled one cannot be used. Held here so it travels with the session
    // being opened rather than being a note somebody makes afterwards.
    var roomPicker by remember { mutableStateOf(false) }
    var provisionRoom by remember { mutableStateOf<ug.qaat.coordinator.net.FreeRoom?>(null) }
    var provisionReason by remember { mutableStateOf("") }

    if (roomPicker) {
        FreeRoomPicker(
            onDismiss = { roomPicker = false },
            onChosen = { r, why ->
                provisionRoom = r; provisionReason = why
                AppState.provisionVenueId = r.venueId
                AppState.provisionNote = why
            },
        )
    }
    var manualStaffId by remember { mutableStateOf("") }

    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp)) {
        Text("Take attendance", style = MaterialTheme.typography.titleLarge)

        if (AppState.manifest == null) {
            Text("Today's schedule hasn't loaded yet. Tap the ⟳ at the top-right to refresh " +
                "(needs internet once — your session may have expired and is being renewed).",
                color = MaterialTheme.colorScheme.error)
            AppState.manifestError?.let { reason ->
                Spacer(Modifier.height(8.dp))
                Surface(color = MaterialTheme.colorScheme.errorContainer, shape = MaterialTheme.shapes.small,
                    modifier = Modifier.fillMaxWidth()) {
                    Text("Last refresh failed: $reason",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onErrorContainer,
                        modifier = Modifier.padding(10.dp))
                }
            }
            return
        }
        if (units.isEmpty() && !offTimetable) {
            Text("No unit is timetabled for today.",
                color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.height(8.dp))
            // The lecture is still happening, and refusing to start it does not stop it — it only
            // stops the STUDENTS being recorded, in a system where attendance decides who sits the
            // exam. So it can be started, deliberately, and it is marked as off-timetable.
            Text(
                "If a lecture is happening anyway — a make-up class, a unit added after the " +
                    "schedule was set — you can still take attendance for it. Students check in " +
                    "exactly as they always do; the record simply notes it was off-timetable.",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.height(8.dp))
            OutlinedButton(onClick = { offTimetable = true }) {
                Text("Take attendance for an off-timetable lecture")
            }
            return
        }
        if (offTimetable) {
            Surface(
                color = MaterialTheme.colorScheme.secondaryContainer, shape = MaterialTheme.shapes.small,
                modifier = Modifier.fillMaxWidth().padding(vertical = 8.dp),
            ) {
                Column(Modifier.padding(10.dp)) {
                    Text("Off-timetable lecture", fontWeight = FontWeight.SemiBold,
                        style = MaterialTheme.typography.bodyMedium)
                    Text("Every unit of your cohort is listed. Students check in normally; the " +
                        "attendance record will say this lecture was not on the timetable.",
                        style = MaterialTheme.typography.labelSmall)
                    TextButton(onClick = { offTimetable = false }) { Text("Back to today's timetable") }
                }
            }
        }

        Spacer(Modifier.height(12.dp))
        // Offered BEFORE the hotspot step, because deciding where the class is happening comes
        // before setting anything up in it.
        provisionRoom?.let { r ->
            Surface(
                color = MaterialTheme.colorScheme.secondaryContainer, shape = MaterialTheme.shapes.small,
                modifier = Modifier.fillMaxWidth().padding(bottom = 8.dp),
            ) {
                Column(Modifier.padding(10.dp)) {
                    Text("Running in ${r.name} (provision)", fontWeight = FontWeight.SemiBold,
                        style = MaterialTheme.typography.bodyMedium)
                    Text(
                        (if (provisionReason.isNotBlank()) "$provisionReason · " else "") +
                            "QA monitors have been told where to find this lecture.",
                        style = MaterialTheme.typography.labelSmall,
                    )
                    TextButton(onClick = {
                        provisionRoom = null; provisionReason = ""
                        AppState.provisionVenueId = ""; AppState.provisionNote = ""
                    }) { Text("Use the timetabled room instead") }
                }
            }
        }
        if (provisionRoom == null) {
            TextButton(onClick = { roomPicker = true }, modifier = Modifier.padding(bottom = 4.dp)) {
                Text("Timetabled room unavailable? Find a free one")
            }
        }

        Text("1. Start the room Wi-Fi + server", style = MaterialTheme.typography.titleSmall)

        // Two ways to bring up the room's Wi-Fi. Automatic (app-owned) needs no setup but gets an
        // OS-assigned random name; cohort-named uses the coordinator's OWN phone hotspot, which
        // they name after the cohort with a password they choose (essential in a shared room with
        // several coordinators). Persisted so the choice sticks.
        var systemMode by remember { mutableStateOf(AppState.useSystemHotspot) }
        Spacer(Modifier.height(6.dp))
        Row(Modifier.fillMaxWidth().selectable(!systemMode) { systemMode = false }.padding(vertical = 4.dp)) {
            RadioButton(!systemMode, { systemMode = false })
            Spacer(Modifier.width(6.dp))
            Column { Text("Automatic (recommended)", fontWeight = FontWeight.SemiBold)
                Text("The app creates the Wi-Fi + server at ${AppState.LOCAL_HOTSPOT_IP}. No setup — name/password are picked by Android.",
                    style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        }
        Row(Modifier.fillMaxWidth().selectable(systemMode) { systemMode = true }.padding(vertical = 4.dp)) {
            RadioButton(systemMode, { systemMode = true })
            Spacer(Modifier.width(6.dp))
            Column { Text("Use my phone's hotspot, named after the cohort", fontWeight = FontWeight.SemiBold)
                Text("You turn on your phone's own hotspot with a cohort name + password you choose. Best for shared rooms.",
                    style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        }

        var sysSsid by remember { mutableStateOf(AppState.systemHotspotSsid?.takeIf { it.isNotBlank() } ?: AppState.suggestedSsid()) }
        var sysPass by remember { mutableStateOf(AppState.systemHotspotPass.orEmpty()) }

        if (systemMode) {
            Surface(color = MaterialTheme.colorScheme.secondaryContainer, shape = MaterialTheme.shapes.small,
                modifier = Modifier.fillMaxWidth().padding(top = 6.dp)) {
                Column(Modifier.padding(12.dp)) {
                    Text("⚠ The app can't change your phone's hotspot — Android doesn't allow it. YOU set the " +
                        "name & password in Settings; the boxes below just tell the app what you chose, so it can " +
                        "show students what to join.",
                        style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSecondaryContainer)
                    Spacer(Modifier.height(10.dp))
                    Text("① In Android hotspot settings, set the name & a password (suggested name: ${AppState.suggestedSsid()}), then turn the hotspot ON.",
                        style = MaterialTheme.typography.labelMedium, fontWeight = FontWeight.SemiBold)
                    Spacer(Modifier.height(6.dp))
                    Text("‼ If students can't SEE your hotspot in their Wi-Fi list: in hotspot settings set the " +
                        "AP Band to 2.4 GHz (not 5 GHz — cheaper phones can't see 5 GHz), and make sure " +
                        "“Hidden network” is OFF. These two settings are the usual reason a named hotspot is invisible.",
                        style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.error)
                    Spacer(Modifier.height(6.dp))
                    OutlinedButton(onClick = {
                        runCatching { ctx.startActivity(Intent(android.provider.Settings.ACTION_WIRELESS_SETTINGS)) }
                    }) { Text("Open hotspot settings") }
                    Spacer(Modifier.height(12.dp))
                    Text("② Type the SAME name & password you just set, so the app can show them to students:",
                        style = MaterialTheme.typography.labelMedium, fontWeight = FontWeight.SemiBold)
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(sysSsid, { sysSsid = it }, singleLine = true, modifier = Modifier.fillMaxWidth(),
                        label = { Text("Wi-Fi name you set (SSID)") })
                    Spacer(Modifier.height(6.dp))
                    PasswordField(sysPass, { sysPass = it }, "Wi-Fi password you set", modifier = Modifier.fillMaxWidth())
                    if (sysPass.isNotEmpty() && sysPass.length < 8)
                        Text("Most phones require at least 8 characters.", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.error)
                }
            }
        }

        Spacer(Modifier.height(8.dp))
        val sysReady = !systemMode || (sysSsid.isNotBlank() && sysPass.length >= 8)
        Button(
            enabled = sysReady,
            onClick = {
                AppState.useSystemHotspot = systemMode
                if (systemMode) {
                    AppState.systemHotspotSsid = sysSsid.trim(); AppState.systemHotspotPass = sysPass
                    // These are shown to students as readable text on the live screen (no join QR).
                    AppState.hotspotSsid = sysSsid.trim(); AppState.hotspotPass = sysPass
                }
                SessionStore.saveSystemHotspot(systemMode, sysSsid, sysPass)
                ctx.startForegroundService(Intent(ctx, SessionService::class.java))
            },
        ) {
            Text(when {
                AppState.serverReady -> "Server running"
                systemMode -> "I've turned on my hotspot — start server"
                else -> "Start room Wi-Fi + server"
            })
        }
        AppState.serverError?.let { err ->
            Surface(color = MaterialTheme.colorScheme.errorContainer, shape = MaterialTheme.shapes.small,
                modifier = Modifier.fillMaxWidth().padding(top = 8.dp)) {
                Text("⚠ Server didn't start\n$err",
                    style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onErrorContainer,
                    modifier = Modifier.padding(10.dp))
            }
        }
        if (AppState.serverReady) {
            Text(
                if (AppState.hotspotUp) "✓ Ready — serving at ${AppState.inRoomIp}. Open “Take attendance” and read students the Wi-Fi name/password + check-in address shown there."
                else "Server on, bringing up the room Wi-Fi… (grant location/nearby-devices if asked).",
                style = MaterialTheme.typography.labelSmall,
                color = if (AppState.hotspotUp) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(top = 6.dp),
            )
        }
        // NOTE: the manual gateway-IP override lives on the live attendance screen (it appears there
        // only when a phone has joined but can't reach the server) — kept in ONE place to avoid
        // duplicating the same control here.

        Spacer(Modifier.height(16.dp))
        Text("2. Unit", style = MaterialTheme.typography.titleSmall)
        units.forEach { u ->
            Row(Modifier.fillMaxWidth().selectable(selectedUnit == u.unitId) { selectedUnit = u.unitId }.padding(vertical = 6.dp)) {
                RadioButton(selectedUnit == u.unitId, { selectedUnit = u.unitId })
                Spacer(Modifier.width(8.dp))
                Column {
                    Text("${u.unitName} (${u.unitId})")
                    if (u.lecturerName.isNotBlank()) Text("Lecturer: ${u.lecturerName}", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
        }

        // The lecturer is resolved from the selected unit's assignment.
        val sel = units.firstOrNull { it.unitId == selectedUnit }
        val autoStaffId = sel?.lecturerStaffId.orEmpty()
        val effectiveStaffId = autoStaffId.ifBlank { manualStaffId.trim() }

        Spacer(Modifier.height(16.dp))
        Text("3. Lecturer", style = MaterialTheme.typography.titleSmall)
        if (autoStaffId.isNotBlank()) {
            Surface(color = MaterialTheme.colorScheme.secondaryContainer, shape = MaterialTheme.shapes.small, modifier = Modifier.fillMaxWidth()) {
                Column(Modifier.padding(12.dp)) {
                    Text(sel?.lecturerName?.ifBlank { autoStaffId } ?: autoStaffId, fontWeight = FontWeight.SemiBold)
                    Text("Staff ID: $autoStaffId" + (sel?.lecturerPhone?.takeIf { it.isNotBlank() }?.let { " · $it" } ?: ""),
                        style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Text("Identified automatically for this unit.", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
        } else {
            Text("No lecturer is assigned to this unit yet — enter the present lecturer's staff ID.",
                style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error)
            Spacer(Modifier.height(6.dp))
            OutlinedTextField(manualStaffId, { manualStaffId = it }, singleLine = true, modifier = Modifier.fillMaxWidth(),
                placeholder = { Text("e.g. KIU/STAFF/001") })
        }

        Spacer(Modifier.height(20.dp))
        // Wait for the foreground service to finish building the in-room server before allowing
        // open() — otherwise it would touch an uninitialized server and crash.
        if (!AppState.serverReady) {
            Text("Tap “Start hotspot + server” above and wait for it to come up first.",
                style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.height(6.dp))
        }
        Button(
            enabled = AppState.serverReady && selectedUnit != null && effectiveStaffId.isNotBlank(),
            modifier = Modifier.fillMaxWidth(),
            onClick = { SessionController.open(selectedUnit!!, effectiveStaffId); onOpened() },
        ) { Text("Start taking attendance") }
    }
}
