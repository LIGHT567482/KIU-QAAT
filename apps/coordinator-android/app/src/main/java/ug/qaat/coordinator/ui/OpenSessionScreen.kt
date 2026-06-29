package ug.qaat.coordinator.ui

import android.content.Intent
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.selection.selectable
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import ug.qaat.coordinator.service.SessionService
import ug.qaat.coordinator.session.SessionController

/**
 * Pick today's unit + the present lecturer, start the hotspot/server, and open the
 * session (SessionController.open → server.setLive). After this, students can check in.
 */
@Composable
fun OpenSessionScreen(onOpened: () -> Unit) {
    val ctx = LocalContext.current
    val units = AppState.manifest?.units.orEmpty()
    var selectedUnit by remember { mutableStateOf(units.firstOrNull()?.unitId) }
    var staffId by remember { mutableStateOf("") }

    Column(Modifier.fillMaxSize().padding(16.dp)) {
        Text("Open a session", style = MaterialTheme.typography.titleLarge)

        if (AppState.manifest == null) { Text("No manifest — sign in while online first."); return }
        if (units.isEmpty()) { Text("No units scheduled for today in the manifest."); return }

        Spacer(Modifier.height(12.dp))
        Text("1. Start the room Wi-Fi + server", style = MaterialTheme.typography.titleSmall)
        Button(onClick = { ctx.startForegroundService(Intent(ctx, SessionService::class.java)) }) {
            Text(AppState.hotspotSsid?.let { "Hotspot: $it" } ?: "Start hotspot + server")
        }

        Spacer(Modifier.height(16.dp))
        Text("2. Unit", style = MaterialTheme.typography.titleSmall)
        units.forEach { u ->
            Row(Modifier.fillMaxWidth().selectable(selectedUnit == u.unitId) { selectedUnit = u.unitId }.padding(8.dp)) {
                RadioButton(selectedUnit == u.unitId, { selectedUnit = u.unitId })
                Spacer(Modifier.width(8.dp))
                Text("${u.unitName} (${u.unitId})")
            }
        }

        Spacer(Modifier.height(12.dp))
        Text("3. Lecturer staff ID (present today)", style = MaterialTheme.typography.titleSmall)
        OutlinedTextField(staffId, { staffId = it }, singleLine = true, modifier = Modifier.fillMaxWidth(),
            placeholder = { Text("e.g. KIU/STAFF/001") })

        Spacer(Modifier.height(20.dp))
        Button(
            enabled = selectedUnit != null && staffId.isNotBlank(),
            modifier = Modifier.fillMaxWidth(),
            onClick = { SessionController.open(selectedUnit!!, staffId.trim()); onOpened() },
        ) { Text("Open session") }
    }
}
