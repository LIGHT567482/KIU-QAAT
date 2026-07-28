package ug.qaat.student.ui

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch
import ug.qaat.student.net.ProgressClient
import ug.qaat.student.store.StudentStore

/** Online "my attendance" view — per-unit % against the threshold + eligibility. Needs internet. */
@Composable
fun ProgressScreen() {
    val scope = rememberCoroutineScope()
    var data by remember { mutableStateOf<ProgressClient.Progress?>(null) }
    var loading by remember { mutableStateOf(true) }
    var error by remember { mutableStateOf<String?>(null) }

    fun load() {
        loading = true; error = null
        scope.launch {
            val reg = StudentStore.reg; val org = StudentStore.org
            if (reg.isNullOrBlank() || org.isNullOrBlank()) { error = "Re-onboard to view your progress."; loading = false; return@launch }
            runCatching { ProgressClient().fetch(reg, org) }
                .onSuccess { data = it; loading = false }
                .onFailure { error = it.message ?: "Couldn't load progress"; loading = false }
        }
    }
    LaunchedEffect(Unit) { load() }

    Column(Modifier.fillMaxSize().padding(20.dp)) {
        Text("My attendance", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        data?.let { Text(it.fullName, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        Spacer(Modifier.height(12.dp))

        when {
            loading -> Box(Modifier.fillMaxWidth().padding(top = 40.dp), Alignment.Center) { CircularProgressIndicator() }
            error != null -> Text(error!!, color = MaterialTheme.colorScheme.error)
            data?.units.isNullOrEmpty() -> Text("No attendance recorded yet.", color = MaterialTheme.colorScheme.onSurfaceVariant)
            else -> LazyColumn(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                items(data!!.units) { u ->
                    val eligible = u.status == "ELIGIBLE"
                    Surface(color = MaterialTheme.colorScheme.surfaceVariant, shape = MaterialTheme.shapes.medium) {
                        Column(Modifier.fillMaxWidth().padding(12.dp)) {
                            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                                Text(u.unitName.ifBlank { u.unitId }, Modifier.weight(1f), fontWeight = FontWeight.SemiBold)
                                Text("${"%.0f".format(u.pct)}%",
                                    fontWeight = FontWeight.Bold,
                                    color = if (eligible) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error)
                            }
                            LinearProgressIndicator(
                                progress = { (u.pct / 100.0).toFloat().coerceIn(0f, 1f) },
                                modifier = Modifier.fillMaxWidth().padding(vertical = 6.dp),
                            )
                            Text("${u.attended}/${u.held} attended · pass mark ${u.threshold}%",
                                style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            if (!eligible) Text(
                                "Exam-ineligible" + (u.deficit?.let { " — attend $it more to recover" } ?: ""),
                                style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.error)
                        }
                    }
                }
            }
        }
        Spacer(Modifier.weight(1f))
        OutlinedButton(onClick = { load() }, enabled = !loading, modifier = Modifier.fillMaxWidth()) { Text("Refresh") }
    }
}
