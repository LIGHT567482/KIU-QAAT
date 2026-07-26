package ug.qaat.engine

import kotlin.math.abs

/**
 * Attendance trend analytics (spec Feature 2) — pure logic over local data.
 * Given weekly (attended / possible) counts, computes the rate per week plus the
 * summary the coordinator dashboard shows. Computed on-device from SQLite.
 */
class AttendanceTrends(private val thresholdPct: Double = 75.0) {

    data class WeekInput(val label: String, val attended: Int, val possible: Int)
    data class WeekPoint(val label: String, val ratePct: Double, val belowThreshold: Boolean)
    enum class Trend { RISING, FALLING, STABLE }

    data class Summary(
        val weeks: List<WeekPoint>,
        val currentWeekRate: Double,
        val semesterAverage: Double,
        val bestWeek: WeekPoint?,
        val worstWeek: WeekPoint?,
        val totalSessions: Int,
        val weeksBelowThreshold: Int,
        val trend: Trend,
    )

    fun compute(weeks: List<WeekInput>): Summary {
        val points = weeks.map {
            val rate = if (it.possible > 0) it.attended.toDouble() / it.possible * 100.0 else 0.0
            WeekPoint(it.label, rate, rate < thresholdPct)
        }
        if (points.isEmpty())
            return Summary(emptyList(), 0.0, 0.0, null, null, 0, 0, Trend.STABLE)

        val current = points.last().ratePct
        // Semester average weighted by sessions (total attended / total possible).
        val totalAttended = weeks.sumOf { it.attended }
        val totalPossible = weeks.sumOf { it.possible }
        val avg = if (totalPossible > 0) totalAttended.toDouble() / totalPossible * 100.0 else 0.0
        val best = points.maxByOrNull { it.ratePct }
        val worst = points.minByOrNull { it.ratePct }
        val belowCount = points.count { it.belowThreshold }
        val trend = when {
            points.size < 2 -> Trend.STABLE
            else -> {
                val delta = points.last().ratePct - points[points.size - 2].ratePct
                if (abs(delta) < 1.0) Trend.STABLE else if (delta > 0) Trend.RISING else Trend.FALLING
            }
        }
        return Summary(points, current, avg, best, worst, weeks.size, belowCount, trend)
    }
}
