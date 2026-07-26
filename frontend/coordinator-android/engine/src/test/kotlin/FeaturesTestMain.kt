import ug.qaat.engine.AttendanceTrends
import ug.qaat.engine.ChronicAbsentee

/** Off-device verification of the chronic-absentee + attendance-trend analytics. */
fun main() {
    var fail = 0
    fun ok(name: String, cond: Boolean) { println((if (cond) "PASS" else "FAIL") + "  $name"); if (!cond) fail++ }

    // ── Chronic absentee ───────────────────────────────────────────────────────
    val sessions = listOf("s1", "s2", "s3", "s4", "s5")
    val attended = mapOf(
        "A" to setOf("s1", "s2"),                       // 3 consecutive misses, 40% → WARNING
        "B" to emptySet(),                              // 5 misses, 0% → CRITICAL
        "C" to setOf("s1", "s2", "s3", "s4", "s5"),     // perfect → not flagged
        "D" to setOf("s1", "s2", "s3", "s4"),           // 1 miss, 80% → not flagged
    )
    val flagged = ChronicAbsentee().evaluate(sessions, attended, listOf("A", "B", "C", "D"))
    ok("absentee: 2 flagged", flagged.size == 2)
    ok("absentee: CRITICAL first = B", flagged.firstOrNull()?.studentId == "B" && flagged[0].status == ChronicAbsentee.Status.CRITICAL)
    ok("absentee: B consec=5, pct=0, lastAttended=null", flagged[0].consecutiveMissed == 5 && flagged[0].attendancePct == 0.0 && flagged[0].lastAttendedSession == null)
    val a = flagged.firstOrNull { it.studentId == "A" }
    ok("absentee: A WARNING consec=3 last=s2", a != null && a.status == ChronicAbsentee.Status.WARNING && a.consecutiveMissed == 3 && a.lastAttendedSession == "s2")
    ok("absentee: C and D not flagged", flagged.none { it.studentId == "C" || it.studentId == "D" })

    // ── Attendance trends ──────────────────────────────────────────────────────
    val t = AttendanceTrends(thresholdPct = 75.0).compute(
        listOf(
            AttendanceTrends.WeekInput("W1", 45, 60), // 75%
            AttendanceTrends.WeekInput("W2", 30, 60), // 50%
            AttendanceTrends.WeekInput("W3", 54, 60), // 90%
        )
    )
    ok("trend: current=90", t.currentWeekRate == 90.0)
    ok("trend: avg≈71.67", kotlin.math.abs(t.semesterAverage - 71.6667) < 0.01)
    ok("trend: best=W3, worst=W2", t.bestWeek?.label == "W3" && t.worstWeek?.label == "W2")
    ok("trend: 1 week below threshold (W2)", t.weeksBelowThreshold == 1)
    ok("trend: RISING (50→90)", t.trend == AttendanceTrends.Trend.RISING)
    ok("trend: 3 weeks", t.totalSessions == 3 && t.weeks.size == 3)

    println(if (fail == 0) "\nALL_FEATURE_TESTS_PASSED" else "\n$fail FAILED")
    if (fail != 0) kotlin.system.exitProcess(1)
}
