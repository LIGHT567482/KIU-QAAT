package ug.qaat.coordinator.ui

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.unit.dp
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import androidx.lifecycle.viewmodel.compose.viewModel
import kotlinx.coroutines.launch
import ug.qaat.coordinator.di.Graph
import ug.qaat.engine.AttendanceTrends

class TrendsViewModel : ViewModel() {
    var summary by mutableStateOf<AttendanceTrends.Summary?>(null); private set
    fun load(unitId: String) = viewModelScope.launch { summary = Graph.repo.trends(unitId) }
}

/** Spec Feature 2: weekly attendance trend + summary, drawn from local data. */
@Composable
fun TrendsScreen(vm: TrendsViewModel = viewModel()) {
    val unitId = AppState.currentUnitId
    LaunchedEffect(unitId) { if (unitId != null) vm.load(unitId) }
    val s = vm.summary

    Column(Modifier.fillMaxSize().padding(16.dp)) {
        Text("Attendance trend", style = MaterialTheme.typography.titleLarge)
        if (unitId == null) { Text("Open a session/unit first."); return }
        if (s == null) { LinearProgressIndicator(Modifier.fillMaxWidth()); return }
        if (s.weeks.isEmpty()) { Text("No completed sessions yet."); return }

        val line = MaterialTheme.colorScheme.primary
        val below = MaterialTheme.colorScheme.error
        val grid = MaterialTheme.colorScheme.outlineVariant
        val threshold = 75f

        Canvas(Modifier.fillMaxWidth().height(220.dp).padding(vertical = 12.dp)) {
            val w = size.width; val h = size.height
            fun x(i: Int) = if (s.weeks.size == 1) w / 2 else w * i / (s.weeks.size - 1)
            fun y(rate: Double) = h - (rate.toFloat() / 100f) * h
            // threshold (dashed)
            drawLine(grid, Offset(0f, y(threshold.toDouble())), Offset(w, y(threshold.toDouble())),
                strokeWidth = 2f, pathEffect = PathEffect.dashPathEffect(floatArrayOf(12f, 8f)))
            // line path
            val path = Path()
            s.weeks.forEachIndexed { i, p ->
                val px = x(i); val py = y(p.ratePct)
                if (i == 0) path.moveTo(px, py) else path.lineTo(px, py)
            }
            drawPath(path, color = line, style = Stroke(width = 4f))
            // points (red if below threshold)
            s.weeks.forEachIndexed { i, p ->
                drawCircle(if (p.belowThreshold) below else line, radius = 7f, center = Offset(x(i), y(p.ratePct)))
            }
        }

        @Composable fun stat(label: String, value: String) =
            ListItem(headlineContent = { Text(value) }, supportingContent = { Text(label) })

        stat("Current week", "%.0f%%".format(s.currentWeekRate))
        stat("Semester average", "%.0f%%".format(s.semesterAverage))
        s.bestWeek?.let { stat("Best week", "${it.label} · %.0f%%".format(it.ratePct)) }
        s.worstWeek?.let { stat("Worst week", "${it.label} · %.0f%%".format(it.ratePct)) }
        stat("Weeks below threshold", "${s.weeksBelowThreshold}")
        stat("Trend", when (s.trend) {
            AttendanceTrends.Trend.RISING -> "Rising ↑"
            AttendanceTrends.Trend.FALLING -> "Falling ↓"
            AttendanceTrends.Trend.STABLE -> "Stable →"
        })
    }
}
