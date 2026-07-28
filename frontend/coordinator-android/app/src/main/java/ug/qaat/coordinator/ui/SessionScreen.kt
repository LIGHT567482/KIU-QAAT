package ug.qaat.coordinator.ui

import android.content.Intent
import androidx.compose.foundation.Image
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.launch
import ug.qaat.coordinator.db.PresentDisplayEntity
import ug.qaat.coordinator.di.Graph
import ug.qaat.coordinator.net.DashboardClient
import ug.qaat.coordinator.service.SessionService

/**
 * The in-room screen: rotating room code (projected), live PRESENT/REJECTED feed,
 * the manual-rotation reminder, broadcast announcements, start/end. Spec Part 4 + Feature 1.
 */
@Composable
fun SessionScreen(onOpenSession: () -> Unit) {
    val scope = rememberCoroutineScope()
    val sessionId = AppState.currentSessionId

    // Idle (no live session): the entry into taking attendance + emergency standby.
    if (sessionId == null) { AttendanceIdle(onOpenSession); return }

    val roster by (if (sessionId != null) Graph.repo.liveRoster(sessionId) else flowOf(emptyList()))
        .collectAsStateWithLifecycle(emptyList())
    val present by (if (sessionId != null) Graph.repo.presentCount(sessionId) else flowOf(0))
        .collectAsStateWithLifecycle(0)

    Column(Modifier.fillMaxSize().padding(16.dp)) {
        // Session identity = the COHORT (its natural name). Android assigns the actual Wi-Fi
        // name itself and an app can't rename it, so students JOIN by scanning the Wi-Fi QR
        // below rather than by reading a name; this header just tells everyone whose room it is.
        AppState.cohortLabel?.takeIf { it.isNotBlank() }?.let {
            Text(it, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold, textAlign = TextAlign.Center,
                modifier = Modifier.fillMaxWidth())
            Spacer(Modifier.height(8.dp))
        }
        // Students JOIN the Wi-Fi by reading the name + password below (no connect QR), then OPEN the
        // check-in address (scan or type), then type their reg-number. The self-test line shows the
        // instant a phone actually reaches this server — the objective proof it's working.
        Surface(color = MaterialTheme.colorScheme.inverseSurface, shape = MaterialTheme.shapes.medium) {
            Column(Modifier.fillMaxWidth().padding(16.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                Text("STUDENTS: JOIN THE WI-FI, OPEN CHECK-IN, TYPE REG-NUMBER", color = MaterialTheme.colorScheme.inverseOnSurface, fontSize = 12.sp, textAlign = TextAlign.Center)
                // The Wi-Fi credentials to read out / project (replaces the Connect-to-Wi-Fi QR).
                val ssid = AppState.hotspotSsid; val pass = AppState.hotspotPass
                if (!ssid.isNullOrBlank()) {
                    Spacer(Modifier.height(6.dp))
                    Text("Wi-Fi name:  $ssid", color = MaterialTheme.colorScheme.inverseOnSurface,
                        fontSize = 15.sp, fontWeight = FontWeight.Bold, fontFamily = FontFamily.Monospace, textAlign = TextAlign.Center)
                    if (!pass.isNullOrBlank())
                        Text("Password:  $pass", color = MaterialTheme.colorScheme.inverseOnSurface,
                            fontSize = 15.sp, fontWeight = FontWeight.Bold, fontFamily = FontFamily.Monospace, textAlign = TextAlign.Center)
                }
                Spacer(Modifier.height(6.dp))
                Text(
                    if (AppState.hotspotUp) "Check-in address: http://${AppState.inRoomIp}:8080"
                    else "Bringing up the room Wi-Fi…",
                    color = MaterialTheme.colorScheme.inverseOnSurface, fontSize = 13.sp,
                    fontFamily = FontFamily.Monospace, textAlign = TextAlign.Center)
                // Reachability self-test: > 0 = a phone genuinely reached the server on this hardware.
                Spacer(Modifier.height(4.dp))
                Text(
                    if (AppState.clientsReached > 0) "✓ ${AppState.clientsReached} device request(s) reached this server"
                    else "No device has reached the server yet — have a student open the address above.",
                    color = MaterialTheme.colorScheme.inverseOnSurface,
                    fontSize = 12.sp, fontWeight = if (AppState.clientsReached > 0) FontWeight.Bold else FontWeight.Normal,
                    textAlign = TextAlign.Center)
            }
        }

        // "Can't be reached" rescue: if a device has joined but none has reached the server, the served
        // gateway IP is probably wrong for this phone's hotspot. Let the coordinator read the real
        // Gateway off a joined phone (Wi-Fi → the network → Gateway) and set it live.
        if (AppState.hotspotUp && AppState.clientsReached == 0) {
            var ip by remember { mutableStateOf(AppState.manualHotspotIp ?: "") }
            Spacer(Modifier.height(8.dp))
            Surface(color = MaterialTheme.colorScheme.errorContainer, shape = MaterialTheme.shapes.small) {
                Column(Modifier.fillMaxWidth().padding(12.dp)) {
                    Text("Students see “can't be reached”? On a JOINED phone open Wi-Fi → this network → Gateway, and type that IP here.",
                        style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onErrorContainer)
                    AppState.hotspotDiag?.let { Text("This phone's addresses: $it", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onErrorContainer) }
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        OutlinedTextField(ip, { ip = it }, Modifier.weight(1f), singleLine = true,
                            placeholder = { Text(AppState.SYSTEM_HOTSPOT_IP) }, label = { Text("Gateway IP") })
                        Spacer(Modifier.width(8.dp))
                        TextButton(onClick = {
                            val v = ip.trim().ifBlank { null }
                            AppState.manualHotspotIp = v
                            v?.let { AppState.inRoomIp = it; AppState.hotspotUp = true }
                            ug.qaat.coordinator.store.SessionStore.saveHotspotIp(v)
                        }) { Text("Set") }
                    }
                }
            }
        }
        Spacer(Modifier.height(8.dp))
        Surface(color = MaterialTheme.colorScheme.tertiaryContainer, shape = MaterialTheme.shapes.small) {
            Text("📴 Tell students to turn Wi-Fi OFF the moment they see ✓ — only ~10 fit at once.",
                Modifier.padding(12.dp), textAlign = TextAlign.Center)
        }

        // Projected QRs: (1) Check-in page (opens http://<ip>:8080/attend — always the current IP, so
        // it always resolves to THIS coordinator), (2) rotating lecturer gate. Rendered locally (ZXing).
        // No Connect-to-Wi-Fi QR: students join by reading the Wi-Fi name + password shown above.
        Spacer(Modifier.height(10.dp))
        Row(Modifier.fillMaxWidth().horizontalScroll(rememberScrollState())) {
            QrCard("Check in here", "${AppState.inRoomBaseUrl}/attend", "scan or type the address")
            QrCard("Lecturer gate", "${AppState.inRoomBaseUrl}/gate?rc=${AppState.lecturerCode}",
                "Lecturer scans · rotates ${AppState.secondsLeft}s")
        }

        Spacer(Modifier.height(12.dp))
        Text("Present: $present", style = MaterialTheme.typography.titleMedium)

        // Live feed
        LazyColumn(Modifier.weight(1f)) {
            items(roster) { r: PresentDisplayEntity ->
                ListItem(
                    headlineContent = { Text(r.fullName.ifBlank { r.studentId }) },
                    supportingContent = { Text("${r.studentId} · ${r.checkinTimestamp.takeLast(8)}") },
                    trailingContent = {
                        val ok = r.status == "PRESENT"
                        Text(if (ok) "✓" else r.status, color = if (ok) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error)
                    },
                )
                HorizontalDivider()
            }
        }

        // Announce
        var msg by remember { mutableStateOf("") }
        Row(verticalAlignment = Alignment.CenterVertically) {
            OutlinedTextField(msg, { msg = it }, Modifier.weight(1f), placeholder = { Text("Announcement…") }, singleLine = true)
            Spacer(Modifier.width(8.dp))
            Button(onClick = {
                val m = msg.trim(); if (m.isNotEmpty()) {
                    scope.launch { runCatching { SessionService.server?.broadcast("GENERAL", m) } }; msg = ""
                }
            }) { Text("Send") }
        }

        Spacer(Modifier.height(8.dp))
        OutlinedButton(
            onClick = { ug.qaat.coordinator.session.SessionController.close() },
            Modifier.fillMaxWidth(),
        ) { Text("End session + sync") }
    }
}

/** A single QR card (title + code + caption), rendered locally so it works offline. */
@Composable
private fun QrCard(title: String, payload: String, caption: String) {
    val bmp = remember(payload) { qrImageBitmap(payload, 360) }
    Card(Modifier.padding(end = 8.dp).width(140.dp)) {
        Column(Modifier.padding(8.dp), horizontalAlignment = Alignment.CenterHorizontally) {
            Text(title, style = MaterialTheme.typography.labelMedium, fontWeight = FontWeight.Bold)
            Spacer(Modifier.height(4.dp))
            if (bmp != null) Image(bmp, contentDescription = title, modifier = Modifier.size(120.dp))
            else Text("—", Modifier.size(120.dp))
            Text(caption, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 1)
        }
    }
}

/** Shown on the Attendance tab when no session is running: the "Take attendance" entry
 *  plus the Emergency Standby feature (moved here from Home). */
@Composable
private fun AttendanceIdle(onOpenSession: () -> Unit) {
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp)) {
        Text("Attendance", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.ExtraBold)
        Text("Start a session so students can check in — this works even with no internet once today's data is downloaded.",
            fontSize = 12.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.height(16.dp))
        Button(onClick = onOpenSession, Modifier.fillMaxWidth()) { Text("Take attendance", fontWeight = FontWeight.Bold) }
        Spacer(Modifier.height(20.dp))
        StandbyCard(remember { DashboardClient() }, AppState.token)
    }
}
