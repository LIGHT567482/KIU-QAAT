package ug.qaat.coordinator.data

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.withContext
import ug.qaat.coordinator.db.AppDao
import ug.qaat.coordinator.db.PresentDisplayEntity
import ug.qaat.engine.AttendanceTrends
import ug.qaat.engine.ChronicAbsentee
import java.time.LocalDate
import java.time.temporal.WeekFields

/**
 * Bridges Room ↔ the verified engine. The analytics (ChronicAbsentee, AttendanceTrends)
 * are proven off-device; this only assembles their inputs from local SQLite.
 */
class Repository(private val dao: AppDao) {

    // ── Live roster (active session) ────────────────────────────────────────────
    fun liveRoster(sessionId: String): Flow<List<PresentDisplayEntity>> = dao.liveDisplay(sessionId)
    fun presentCount(sessionId: String): Flow<Int> = dao.presentCount(sessionId)

    // ── Chronic absentees for a unit ─────────────────────────────────────────────
    // NOTE: the offline roster holds privacy hashes (no names for never-present students),
    // so `studentId` here is the hash. Carry display names in the manifest if you want
    // real names for absentees; present students' names come from their QR (live roster).
    suspend fun chronicAbsentees(unitId: String): List<ChronicAbsentee.Flagged> =
        withContext(Dispatchers.IO) {
            val sessions = dao.sessionsForUnit(unitId)
            val sessionIds = sessions.map { it.sessionId }
            if (sessionIds.isEmpty()) return@withContext emptyList()
            val students = dao.rosterHashes(unitId)
            val attendedBy = dao.attendanceForSessions(sessionIds)
                .groupBy({ it.studentIdHash }, { it.sessionId })
                .mapValues { it.value.toSet() }
            ChronicAbsentee().evaluate(sessionIds, attendedBy, students)
        }

    // ── Attendance trend (per ISO week) ──────────────────────────────────────────
    suspend fun trends(unitId: String): AttendanceTrends.Summary =
        withContext(Dispatchers.IO) {
            val sessions = dao.sessionsForUnit(unitId)
            val perSession = dao.attendanceForSessions(sessions.map { it.sessionId })
                .groupingBy { it.sessionId }.eachCount()
            val weeks = sessions
                .groupBy { isoWeek(it.sessionDate) }
                .toSortedMap()
                .map { (wk, ss) ->
                    AttendanceTrends.WeekInput(
                        label = "W$wk",
                        attended = ss.sumOf { perSession[it.sessionId] ?: 0 },
                        possible = ss.sumOf { it.enrolled },
                    )
                }
            AttendanceTrends().compute(weeks)
        }

    private fun isoWeek(date: String): Int =
        try { LocalDate.parse(date).get(WeekFields.ISO.weekOfWeekBasedYear()) } catch (_: Exception) { 0 }
}
