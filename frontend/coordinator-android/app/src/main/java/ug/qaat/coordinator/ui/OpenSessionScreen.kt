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
    val units = AppState.manifest?.units.orEmpty()
    var selectedUnit by remember { mutableStateOf(units.firstOrNull()?.unitId) }
    var manualStaffId by remember { mutableStateOf("") }

    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp)) {
        Text("Take attendance", style = MaterialTheme.typography.titleLarge)

        if (AppState.manifest == null) { Text("No manifest — sign in while online first."); return }
        if (units.isEmpty()) { Text("No units scheduled for today in the manifest."); return }

        Spacer(Modifier.height(12.dp))
        Text("1. Start the room Wi-Fi + server", style = MaterialTheme.typography.titleSmall)
        Text(
            "The app creates the room's Wi-Fi and the check-in server at a fixed address " +
                "(${AppState.LOCAL_HOTSPOT_IP}) — the same on every coordinator phone, so nothing to " +
                "configure. Students join by scanning the “Connect to Wi-Fi” QR shown on the next screen.",
            style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Spacer(Modifier.height(6.dp))
        Button(onClick = { AppState.useSystemHotspot = false; ctx.startForegroundService(Intent(ctx, SessionService::class.java)) }) {
            Text(if (AppState.serverReady) "Server running" else "Start room Wi-Fi + server")
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
                if (AppState.hotspotUp) "✓ Ready — room Wi-Fi up, serving at ${AppState.inRoomIp}. Show students the “Connect to Wi-Fi” QR."
                else "Server on, bringing up the room Wi-Fi… (grant location/nearby-devices if asked).",
                style = MaterialTheme.typography.labelSmall,
                color = if (AppState.hotspotUp) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(top = 6.dp),
            )
        }

        // Manual IP override (rare fallback): only needed if the LocalOnlyHotspot gateway on this
        // device is NOT 192.168.49.1. Read it off any joined student phone (Wi-Fi → Gateway) and set.
        Spacer(Modifier.height(8.dp))
        var manualIp by remember { mutableStateOf(AppState.manualHotspotIp ?: "") }
        OutlinedTextField(
            manualIp, { manualIp = it }, singleLine = true, modifier = Modifier.fillMaxWidth(),
            label = { Text("Hotspot IP (advanced — only if not ${AppState.LOCAL_HOTSPOT_IP})") },
            placeholder = { Text(AppState.LOCAL_HOTSPOT_IP) },
            trailingIcon = {
                TextButton(onClick = {
                    val v = manualIp.trim().ifBlank { null }
                    AppState.manualHotspotIp = v
                    v?.let { AppState.inRoomIp = it; AppState.hotspotUp = true }
                    SessionStore.saveHotspotIp(v)
                }) { Text("Set") }
            },
        )

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
