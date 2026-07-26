package ug.qaat.engine

/**
 * Chronic-absentee detection (spec Feature 3) — pure logic over local attendance.
 * Flags students who missed >= [consecutiveThreshold] consecutive sessions, and/or
 * whose attendance % is below [lowAttendancePct]. Both ⇒ CRITICAL, one ⇒ WARNING.
 * Computed on-device from SQLite; no internet needed.
 */
class ChronicAbsentee(
    private val consecutiveThreshold: Int = 3,
    private val lowAttendancePct: Double = 25.0,
) {
    enum class Status { WARNING, CRITICAL }

    data class Flagged(
        val studentId: String,
        val consecutiveMissed: Int,
        val attendancePct: Double,
        val status: Status,
        val lastAttendedSession: String?, // null = never attended
    )

    /**
     * @param sessionsChrono session ids in chronological order (oldest → newest)
     * @param attendedBy     studentId → set of session ids they attended
     * @param students       all enrolled student ids for the unit
     */
    fun evaluate(
        sessionsChrono: List<String>,
        attendedBy: Map<String, Set<String>>,
        students: List<String>,
    ): List<Flagged> {
        if (sessionsChrono.isEmpty()) return emptyList()
        val total = sessionsChrono.size
        val out = mutableListOf<Flagged>()
        for (s in students) {
            val attended = attendedBy[s] ?: emptySet()
            // Consecutive misses, counting back from the most recent session.
            var consec = 0
            for (sid in sessionsChrono.asReversed()) {
                if (sid in attended) break
                consec++
            }
            val pct = attended.size.toDouble() / total * 100.0
            val lowFlag = pct < lowAttendancePct
            val consecFlag = consec >= consecutiveThreshold
            val status = when {
                lowFlag && consecFlag -> Status.CRITICAL
                lowFlag || consecFlag -> Status.WARNING
                else -> null
            } ?: continue
            val lastAttended = sessionsChrono.lastOrNull { it in attended }
            out.add(Flagged(s, consec, pct, status, lastAttended))
        }
        // Most urgent first: CRITICAL before WARNING, then by consecutive misses desc.
        return out.sortedWith(compareByDescending<Flagged> { it.status == Status.CRITICAL }.thenByDescending { it.consecutiveMissed })
    }
}
