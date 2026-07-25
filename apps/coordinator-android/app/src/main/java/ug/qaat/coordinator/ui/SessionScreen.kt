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
        // Student room code — STATIC for the whole session (students on the hotspot type this).
        Surface(color = MaterialTheme.colorScheme.inverseSurface, shape = MaterialTheme.shapes.medium) {
            Column(Modifier.fillMaxWidth().padding(16.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                Text("STUDENT ROOM CODE", color = MaterialTheme.colorScheme.inverseOnSurface, fontSize = 12.sp)
                Text(AppState.roomCode, color = MaterialTheme.colorScheme.inverseOnSurface,
                    fontSize = 48.sp, fontFamily = FontFamily.Monospace)
                Text("students enter this · stays the same · ${AppState.hotspotSsid ?: "hotspot off"}",
                    color = MaterialTheme.colorScheme.inverseOnSurface, fontSize = 11.sp)
            }
        }
        Spacer(Modifier.height(8.dp))
        Surface(color = MaterialTheme.colorScheme.tertiaryContainer, shape = MaterialTheme.shapes.small) {
            Text("📴 Tell students to turn Wi-Fi OFF the moment they see ✓ — only ~10 fit at once.",
                Modifier.padding(12.dp), textAlign = TextAlign.Center)
        }

        // QR codes students + lecturer scan: join the Wi-Fi, open the check-in page, and
        // the lecturer gate. Rendered locally (ZXing) — works fully offline.
        Spacer(Modifier.height(10.dp))
        Row(Modifier.fillMaxWidth().horizontalScroll(rememberScrollState())) {
            val ssid = AppState.hotspotSsid; val pass = AppState.hotspotPass
            if (!ssid.isNullOrBlank()) QrCard("1 · Join Wi-Fi", wifiQrPayload(ssid, pass ?: ""), if (pass.isNullOrBlank()) ssid else "$ssid")
            QrCard("2 · Check-in page", "${AppState.inRoomBaseUrl}/attend", "Scan to open")
            QrCard("Lecturer gate", "${AppState.inRoomBaseUrl}/gate", "Lecturer scans")
        }
        // The lecturer's code ROTATES (they read this live off the screen when scanning).
        Spacer(Modifier.height(6.dp))
        Surface(color = MaterialTheme.colorScheme.secondaryContainer, shape = MaterialTheme.shapes.small) {
            Text("Lecturer code (rotating): ${AppState.lecturerCode} · changes in ${AppState.secondsLeft}s",
                Modifier.padding(10.dp), fontSize = 12.sp, fontFamily = FontFamily.Monospace, textAlign = TextAlign.Center)
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
                    scope.launch { runCatching { SessionService.server.broadcast("GENERAL", m) } }; msg = ""
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
