package ug.qaat.coordinator.ui

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import ug.qaat.coordinator.db.SessionEntity
import ug.qaat.coordinator.di.Graph

/**
 * Spec Feature 4 (sync audit) — minimal: recent sessions + their sync status. A full
 * audit log (per-upload chunks/retries/network/result) needs a sync-upload tracking
 * table; this is the build-ready shell reading session status from SQLite.
 */
@Composable
fun SyncAuditScreen() {
    val sessions by Graph.db.dao().recentSessions().collectAsStateWithLifecycle(emptyList())
    Column(Modifier.fillMaxSize().padding(16.dp)) {
        Text("Sync audit", style = MaterialTheme.typography.titleLarge)
        if (sessions.isEmpty()) { Text("No sessions yet."); return }
        LazyColumn {
            items(sessions) { s: SessionEntity ->
                val synced = s.status.equals("SYNCED", ignoreCase = true)
                ListItem(
                    headlineContent = { Text("${s.unitId} · ${s.sessionDate}") },
                    supportingContent = {
                        Text(s.sessionId.take(8) + (if (s.closedReason == "AUTO_CLOSED") "  ·  ⏱ auto-closed" else ""))
                    },
                    trailingContent = {
                        Text(if (synced) "SYNCED" else s.status,
                            color = if (synced) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error)
                    },
                )
                HorizontalDivider()
            }
        }
    }
}
