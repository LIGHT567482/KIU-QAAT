package ug.qaat.coordinator.ui

import android.content.Intent
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
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
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
        // Fully hotspot-driven, no QR: students JOIN this Wi-Fi (name + password below), then open the
        // QAAT app and tap ATTEND — the app finds this hub on the LAN and checks them in directly. The
        // browser address is kept only as a no-app fallback. The self-test line shows the instant a
        // phone actually reaches this server — the objective proof it's working.
        Surface(color = MaterialTheme.colorScheme.inverseSurface, shape = MaterialTheme.shapes.medium) {
            Column(Modifier.fillMaxWidth().padding(16.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                Text("STUDENTS: JOIN THIS WI-FI, THEN OPEN THE QAAT APP AND TAP ATTEND", color = MaterialTheme.colorScheme.inverseOnSurface, fontSize = 12.sp, textAlign = TextAlign.Center)
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
                    if (AppState.hotspotUp) "No app? Open  http://${AppState.inRoomIp}:8080"
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

        // (The manual "Gateway IP" rescue field was removed — student/lecturer phones now auto-discover
        // the hub via the DHCP gateway + NSD, so it's no longer needed.)
        Spacer(Modifier.height(8.dp))
        Surface(color = MaterialTheme.colorScheme.tertiaryContainer, shape = MaterialTheme.shapes.small) {
            Text("📴 Tell students to turn Wi-Fi OFF the moment they see ✓ — only ~10 fit at once.",
                Modifier.padding(12.dp), textAlign = TextAlign.Center)
        }

        // Multi-coordinator daily code: if THIS unit's lecturer is shared across coordinators today
        // and hasn't STARTed here in person (they're in another coordinator's room), the coordinator
        // enters the lecturer's daily 4-digit code (read out by the lecturer) to unlock check-in here.
        if (AppState.currentLecturerHasCode && !AppState.lecturerStartedHere) {
            Spacer(Modifier.height(8.dp))
            var code by remember { mutableStateOf("") }
            var codeErr by remember { mutableStateOf<String?>(null) }
            Surface(color = MaterialTheme.colorScheme.secondaryContainer, shape = MaterialTheme.shapes.small) {
                Column(Modifier.fillMaxWidth().padding(12.dp)) {
                    Text("Lecturer teaching several rooms at once? Enter the code they read out to start attendance here.",
                        style = MaterialTheme.typography.labelMedium, fontWeight = FontWeight.SemiBold)
                    Spacer(Modifier.height(6.dp))
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        OutlinedTextField(
                            code, { v -> if (v.length <= 4 && v.all { it.isDigit() }) { code = v; codeErr = null } },
                            Modifier.weight(1f), singleLine = true, label = { Text("Lecturer/session code") },
                        )
                        Spacer(Modifier.width(8.dp))
                        Button(enabled = code.length == 4, onClick = {
                            if (ug.qaat.coordinator.session.SessionController.startLecturerByCode(code)) codeErr = null
                            else codeErr = "That code isn't right for this lecturer/session today."
                        }) { Text("Start") }
                    }
                    codeErr?.let { Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.labelSmall) }
                }
            }
        }
        // Revealed only after the lecturer STARTs in person here: the daily code they read out to
        // the OTHER coordinators teaching them at the same time so those rooms can start too.
        AppState.currentSessionCode?.let { c ->
            Spacer(Modifier.height(8.dp))
            Surface(color = MaterialTheme.colorScheme.tertiaryContainer, shape = MaterialTheme.shapes.small) {
                Column(Modifier.fillMaxWidth().padding(12.dp)) {
                    Text("Code for other rooms teaching this lecturer now:", style = MaterialTheme.typography.labelSmall)
                    Text(c, style = MaterialTheme.typography.displaySmall, fontWeight = FontWeight.ExtraBold, fontFamily = FontFamily.Monospace)
                    Text("Read this out — the other coordinator enters it to start their room.",
                        style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onTertiaryContainer)
                }
            }
        }
        if (AppState.currentLecturerHasCode && AppState.lecturerStartedHere && AppState.currentSessionCode == null) {
            Spacer(Modifier.height(8.dp))
            Text("✓ Lecturer marked present via daily code — students can check in.",
                style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.primary)
        }

        // No QR codes at all — the system is hotspot-driven. Students tap ATTEND and lecturers tap
        // START/END inside the app; both talk to this hub over the LAN (the app pulls the live gate
        // code itself from GET /status, so the lecturer never scans anything).
        Spacer(Modifier.height(12.dp))
        // The live roster is now behind a button that opens a dedicated page (keeps this screen tidy).
        var showRoster by remember { mutableStateOf(false) }
        Button(onClick = { showRoster = true }, Modifier.fillMaxWidth()) {
            Text("View live roster — $present present", fontWeight = FontWeight.Bold)
        }
        if (showRoster) LiveRosterDialog(roster, present) { showRoster = false }
        Spacer(Modifier.weight(1f))

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

/** The live roster on its own page (opened from the "View live roster" button): who is present now,
 *  newest first, updating live as students check in. */
@Composable
private fun LiveRosterDialog(roster: List<PresentDisplayEntity>, present: Int, onClose: () -> Unit) {
    Dialog(onDismissRequest = onClose, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Surface(Modifier.fillMaxSize()) {
            Column(Modifier.fillMaxSize().padding(16.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text("Live roster — $present present", fontWeight = FontWeight.Bold, modifier = Modifier.weight(1f))
                    TextButton(onClick = onClose) { Text("Close") }
                }
                Row(Modifier.fillMaxWidth().padding(vertical = 4.dp)) {
                    Text("Reg no", Modifier.weight(1.1f), fontSize = 11.sp, fontWeight = FontWeight.Bold, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Text("Student name", Modifier.weight(1.5f), fontSize = 11.sp, fontWeight = FontWeight.Bold, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Text("Status", Modifier.weight(0.5f), fontSize = 11.sp, fontWeight = FontWeight.Bold, color = MaterialTheme.colorScheme.onSurfaceVariant, textAlign = TextAlign.Center)
                }
                HorizontalDivider()
                if (roster.isEmpty()) Text("No one has checked in yet.", color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(top = 12.dp))
                else LazyColumn(Modifier.weight(1f)) {
                    items(roster) { r: PresentDisplayEntity ->
                        Row(Modifier.fillMaxWidth().padding(vertical = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                            Text(r.studentId, Modifier.weight(1.1f), fontSize = 12.sp, fontFamily = FontFamily.Monospace)
                            Text(r.fullName.ifBlank { "—" }, Modifier.weight(1.5f), fontSize = 13.sp)
                            Box(Modifier.weight(0.5f), contentAlignment = Alignment.Center) {
                                val ok = r.status == "PRESENT"
                                Text(if (ok) "✓" else "✗", color = if (ok) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error, fontWeight = FontWeight.Bold, fontSize = 18.sp)
                            }
                        }
                        HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.4f))
                    }
                }
            }
        }
    }
}

/** Shown on the Attendance tab when no session is running: the "Take attendance" entry
 *  plus the Emergency Standby feature (moved here from Home). */
@Composable
private fun AttendanceIdle(onOpenSession: () -> Unit) {
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp)) {
        Text("Attendance", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.ExtraBold)
        AppState.sessionNotice?.let { note ->
            Surface(color = MaterialTheme.colorScheme.tertiaryContainer, shape = MaterialTheme.shapes.small,
                modifier = Modifier.fillMaxWidth().padding(vertical = 8.dp)) {
                Row(Modifier.padding(12.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text(note, Modifier.weight(1f), style = MaterialTheme.typography.bodySmall)
                    TextButton(onClick = { AppState.sessionNotice = null }) { Text("Dismiss") }
                }
            }
        }
        Text("Start a session so students can check in — this works even with no internet once today's data is downloaded.",
            fontSize = 12.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.height(16.dp))
        Button(onClick = onOpenSession, Modifier.fillMaxWidth()) { Text("Take attendance", fontWeight = FontWeight.Bold) }
        Spacer(Modifier.height(20.dp))
        StandbyCard(remember { DashboardClient() }, AppState.token)
    }
}
