package ug.qaat.coordinator.ui

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import androidx.lifecycle.viewmodel.compose.viewModel
import kotlinx.coroutines.launch
import ug.qaat.coordinator.di.Graph
import ug.qaat.engine.ChronicAbsentee

class AbsenteeViewModel : ViewModel() {
    var rows by mutableStateOf<List<ChronicAbsentee.Flagged>>(emptyList()); private set
    var loading by mutableStateOf(false); private set
    var filter by mutableStateOf<ChronicAbsentee.Status?>(null)

    fun load(unitId: String) = viewModelScope.launch {
        loading = true
        rows = Graph.repo.chronicAbsentees(unitId)
        loading = false
    }

    fun visible(): List<ChronicAbsentee.Flagged> = filter?.let { f -> rows.filter { it.status == f } } ?: rows
}

/** Spec Feature 3: students who crossed the consecutive-absence / low-% thresholds. */
@Composable
fun AbsenteeScreen(vm: AbsenteeViewModel = viewModel()) {
    val unitId = AppState.currentUnitId
    LaunchedEffect(unitId) { if (unitId != null) vm.load(unitId) }

    Column(Modifier.fillMaxSize().padding(16.dp)) {
        Text("Chronic absentees", style = MaterialTheme.typography.titleLarge)
        if (unitId == null) { Text("Open a session/unit first."); return }

        Row(Modifier.padding(vertical = 8.dp)) {
            FilterChip(vm.filter == null, { vm.filter = null }, { Text("All") })
            Spacer(Modifier.width(8.dp))
            FilterChip(vm.filter == ChronicAbsentee.Status.CRITICAL, { vm.filter = ChronicAbsentee.Status.CRITICAL }, { Text("Critical") })
            Spacer(Modifier.width(8.dp))
            FilterChip(vm.filter == ChronicAbsentee.Status.WARNING, { vm.filter = ChronicAbsentee.Status.WARNING }, { Text("Warning") })
        }

        if (vm.loading) LinearProgressIndicator(Modifier.fillMaxWidth())
        if (!vm.loading && vm.rows.isEmpty()) Text("No flagged students. 🎉")

        val scope = rememberCoroutineScope()
        LazyColumn {
            items(vm.visible()) { f ->
                val critical = f.status == ChronicAbsentee.Status.CRITICAL
                ListItem(
                    headlineContent = { Text(f.studentId) }, // hash unless the manifest carries names
                    supportingContent = {
                        Text("missed ${f.consecutiveMissed} in a row · ${"%.0f".format(f.attendancePct)}%" +
                            (f.lastAttendedSession?.let { " · last: $it" } ?: " · never attended"))
                    },
                    trailingContent = {
                        AssistChip(
                            onClick = { scope.launch { /* TODO escalate: POST /api/v1/escalations with context */ } },
                            label = { Text(if (critical) "CRITICAL" else "WARNING") },
                            colors = AssistChipDefaults.assistChipColors(
                                containerColor = if (critical) MaterialTheme.colorScheme.errorContainer else MaterialTheme.colorScheme.tertiaryContainer
                            ),
                        )
                    },
                )
                HorizontalDivider()
            }
        }
    }
}
