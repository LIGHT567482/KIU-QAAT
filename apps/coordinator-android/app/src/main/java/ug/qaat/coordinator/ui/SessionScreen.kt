package ug.qaat.coordinator.ui

import android.content.Intent
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.launch
import ug.qaat.coordinator.db.PresentDisplayEntity
import ug.qaat.coordinator.di.Graph
import ug.qaat.coordinator.service.SessionService

/**
 * The in-room screen: rotating room code (projected), live PRESENT/REJECTED feed,
 * the manual-rotation reminder, broadcast announcements, start/end. Spec Part 4 + Feature 1.
 */
@Composable
fun SessionScreen(onOpenSession: () -> Unit) {
    val scope = rememberCoroutineScope()
    val sessionId = AppState.currentSessionId

    val roster by (if (sessionId != null) Graph.repo.liveRoster(sessionId) else flowOf(emptyList()))
        .collectAsStateWithLifecycle(emptyList())
    val present by (if (sessionId != null) Graph.repo.presentCount(sessionId) else flowOf(0))
        .collectAsStateWithLifecycle(0)

    Column(Modifier.fillMaxSize().padding(16.dp)) {
        // Room code
        Surface(color = MaterialTheme.colorScheme.inverseSurface, shape = MaterialTheme.shapes.medium) {
            Column(Modifier.fillMaxWidth().padding(16.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                Text("ROOM CODE", color = MaterialTheme.colorScheme.inverseOnSurface, fontSize = 12.sp)
                Text(AppState.roomCode, color = MaterialTheme.colorScheme.inverseOnSurface,
                    fontSize = 48.sp, fontFamily = FontFamily.Monospace)
                Text("changes in ${AppState.secondsLeft}s · ${AppState.hotspotSsid ?: "hotspot off"}",
                    color = MaterialTheme.colorScheme.inverseOnSurface, fontSize = 11.sp)
            }
        }
        Spacer(Modifier.height(8.dp))
        Surface(color = MaterialTheme.colorScheme.tertiaryContainer, shape = MaterialTheme.shapes.small) {
            Text("📴 Tell students to turn Wi-Fi OFF the moment they see ✓ — only ~10 fit at once.",
                Modifier.padding(12.dp), textAlign = TextAlign.Center)
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
        if (sessionId == null) {
            Button(onClick = onOpenSession, Modifier.fillMaxWidth()) { Text("Open a session") }
        } else {
            OutlinedButton(
                onClick = { ug.qaat.coordinator.session.SessionController.close() },
                Modifier.fillMaxWidth(),
            ) { Text("End session + sync") }
        }
    }
}
