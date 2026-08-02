package ug.qaat.coordinator.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import ug.qaat.coordinator.net.NotificationClient
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.temporal.ChronoUnit

/**
 * One alert, drawn the same way in every inbox — student, lecturer and coordinator.
 *
 * Three things every alert carries:
 *  • WHO sent it, with their courtesy title ("Dr Jane Smith"), and in what role.
 *  • WHEN it was sent — a friendly relative age plus the exact date and time, because "2h ago"
 *    alone is useless when you are reconciling an attendance dispute a week later.
 *  • An ✕ that removes it from YOUR inbox. Other recipients of the same cohort-wide alert keep
 *    theirs; see DismissAppNotification on the gateway.
 */
@Composable
fun NotificationCard(
    n: NotificationClient.Notif,
    onOpen: () -> Unit,
    onDismiss: () -> Unit,
) {
    var expanded by remember(n.id) { mutableStateOf(false) }

    Surface(
        color = if (!n.read) MaterialTheme.colorScheme.primary.copy(alpha = .08f)
        else MaterialTheme.colorScheme.surfaceVariant,
        shape = MaterialTheme.shapes.medium,
    ) {
        Row(Modifier.fillMaxWidth().padding(start = 12.dp, top = 10.dp, bottom = 10.dp, end = 4.dp)) {
            Column(
                Modifier.weight(1f).clickableNoRipple { expanded = !expanded; onOpen() },
            ) {
                Text(n.subject, fontWeight = if (n.read) FontWeight.SemiBold else FontWeight.Bold)
                Text(
                    "from ${n.senderName} · ${n.senderRole.replace('_', ' ')}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                Text(
                    formatSentAt(n.createdAt),
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    fontSize = 11.sp,
                )
                if (expanded && n.body.isNotBlank()) {
                    Spacer(Modifier.height(6.dp))
                    Text(n.body)
                }
            }
            IconButton(onClick = onDismiss, modifier = Modifier.size(36.dp)) {
                Icon(
                    NavIcons.Close, contentDescription = "Dismiss this alert",
                    tint = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(18.dp),
                )
            }
        }
    }
}

/** "2h ago · Mon 12 Aug 2026, 14:05" — relative for scanning, absolute for the record. */
fun formatSentAt(iso: String): String {
    val instant = runCatching { Instant.parse(iso) }.getOrNull() ?: return iso
    val local = instant.atZone(ZoneId.systemDefault())
    val exact = local.format(DateTimeFormatter.ofPattern("EEE d MMM yyyy, HH:mm"))

    val mins = ChronoUnit.MINUTES.between(instant, Instant.now())
    val rel = when {
        mins < 1 -> "just now"
        mins < 60 -> "${mins}m ago"
        mins < 60 * 24 -> "${mins / 60}h ago"
        mins < 60 * 24 * 7 -> "${mins / (60 * 24)}d ago"
        else -> null
    }
    return if (rel == null) exact else "$rel · $exact"
}

/** A tap target without the ripple, so the card body and the ✕ don't fight over the highlight. */
@Composable
private fun Modifier.clickableNoRipple(onClick: () -> Unit): Modifier {
    val interaction = remember { MutableInteractionSource() }
    return this.clickable(interactionSource = interaction, indication = null, onClick = onClick)
}
