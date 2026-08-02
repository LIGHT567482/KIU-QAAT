package ug.qaat.coordinator.db

import androidx.room.*

// Local SQLite schema (spec §10), encrypted at rest via SQLCipher (see SessionService).

@Entity(tableName = "device_bindings", primaryKeys = ["studentIdHash"])
data class BindingEntity(
    val studentIdHash: String,
    @ColumnInfo(index = true) val fingerprintHash: String,
    val academicYear: String,
    val firstBoundAt: String,
)

@Entity(
    tableName = "attendance_logs",
    indices = [Index("sessionId"), Index(value = ["sessionId", "studentIdHash"], unique = true)],
)
data class AttendanceEntity(
    @PrimaryKey val logId: String,
    val sessionId: String,
    val studentIdHash: String,
    val deviceFingerprintHash: String,
    val sequenceNumber: Int,
    val checkinTimestamp: String,
    val entryMethod: String,
    val synced: Boolean = false,
)

@Entity(tableName = "roster", primaryKeys = ["unitId", "studentIdHash"])
data class RosterEntity(
    val unitId: String,
    val studentIdHash: String,
    val qrSerialNumber: String,
    // Display fields so chronic absentees (incl. never-present students) show a real reg-no/name
    // OFFLINE instead of the privacy hash. Default "" for rows cached before this column existed.
    val studentId: String = "",
    val fullName: String = "",
)

/** Closed-session history (per unit) so absentee/trend analytics can run offline. */
@Entity(tableName = "sessions", indices = [Index("unitId")])
data class SessionEntity(
    @PrimaryKey val sessionId: String,
    val unitId: String,
    val sessionDate: String,     // ISO date, used to order chronologically + group by week
    val status: String,          // sync status: OPEN → CLOSED → SYNCED / PENDING_SYNC
    val enrolled: Int,
    // How it ended: "MANUAL" (coordinator/lecturer) or "AUTO_CLOSED" (past scheduled duration + 5m).
    // Shown on the Sync/audit log; also uploaded so the dashboards distinguish them.
    val closedReason: String? = null,
)

/**
 * Human-readable check-in capture for the LIVE roster. The durable ledger keeps the
 * privacy-preserving hash; this transient row keeps the name + reg-no the student's QR
 * revealed at check-in, so the coordinator sees real names without an online lookup.
 */
@Entity(tableName = "present_display", primaryKeys = ["sessionId", "studentId"])
data class PresentDisplayEntity(
    val sessionId: String,
    val studentId: String,       // reg-no from the scanned QR
    val fullName: String,
    val checkinTimestamp: String,
    val status: String,          // PRESENT or a rejection reason (for the live feed)
)

/**
 * One timetabled slot for today, cached so the QA patroller works with no signal.
 *
 * The patroller used to be a separate app with its own plain-SQLite database. It now lives in
 * this one, which means its cached timetable and its queued observations are encrypted at rest
 * by the same SQLCipher key as everything else — a patrol round is a record of who was and
 * wasn't teaching, and that is not something to leave in the clear on a phone.
 */
@Entity(tableName = "patrol_slots", primaryKeys = ["unitId", "startTime"])
data class PatrolSlotEntity(
    val unitId: String,
    val unitName: String,
    val courseCode: String,
    val lecturerStaffId: String,
    val lecturerName: String,
    val room: String,
    val dayOfWeek: Int,
    val startTime: String,        // "HH:MM"
    val durationMinutes: Int,
)

/** A patrol observation captured in the field; uploaded when the phone is back online. */
@Entity(tableName = "patrol_logs", indices = [Index("sessionDate")])
data class PatrolLogEntity(
    @PrimaryKey val id: String,
    val unitId: String,
    val unitName: String,
    val courseCode: String,
    val lecturerId: String,       // lecturer staff id
    val lecturerName: String,
    val room: String,
    val sessionDate: String,      // YYYY-MM-DD
    val scheduledTime: String,    // HH:MM
    val taught: Boolean,
    val takenAt: String,          // RFC3339
    val synced: Boolean = false,
)

@Dao
interface AppDao {
    @Query("SELECT * FROM device_bindings WHERE fingerprintHash = :fp LIMIT 1")
    fun bindingByFingerprint(fp: String): BindingEntity?

    @Query("SELECT * FROM device_bindings WHERE studentIdHash = :h LIMIT 1")
    fun bindingByStudent(h: String): BindingEntity?

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    fun putBinding(b: BindingEntity)

    @Query("SELECT COUNT(*) FROM attendance_logs WHERE sessionId = :s AND studentIdHash = :h")
    fun attendanceCountFor(s: String, h: String): Int

    @Query("SELECT COUNT(*) FROM attendance_logs WHERE sessionId = :s AND deviceFingerprintHash = :fp AND studentIdHash != :h")
    fun deviceUsedByOtherCount(s: String, fp: String, h: String): Int

    @Query("SELECT COUNT(*) FROM attendance_logs WHERE sessionId = :s")
    fun attendanceCount(s: String): Int

    // Append-only: inserts never replace; corrections are new rows (engine never updates).
    @Insert(onConflict = OnConflictStrategy.ABORT)
    fun addAttendance(a: AttendanceEntity)

    @Query("SELECT * FROM attendance_logs WHERE sessionId = :s ORDER BY sequenceNumber")
    fun rosterForSession(s: String): List<AttendanceEntity>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    fun upsertRoster(rows: List<RosterEntity>)

    @Query("SELECT studentIdHash FROM roster WHERE unitId = :u")
    fun rosterHashes(u: String): List<String>

    @Query("SELECT * FROM roster WHERE unitId = :u")
    fun roster(u: String): List<RosterEntity>

    // ── Session history + live display ──────────────────────────────────────────
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    fun upsertSession(s: SessionEntity)

    @Query("SELECT * FROM sessions WHERE unitId = :u ORDER BY sessionDate")
    fun sessionsForUnit(u: String): List<SessionEntity>

    // Closed sessions whose sealed package hasn't reached the backend yet (offline close
    // or a failed upload) — the retry/sync-now path re-seals and uploads these.
    @Query("SELECT * FROM sessions WHERE status IN ('PENDING_SYNC','CLOSED') ORDER BY sessionDate")
    fun pendingSyncSessions(): List<SessionEntity>

    @Query("SELECT COUNT(*) FROM sessions WHERE status IN ('PENDING_SYNC','CLOSED')")
    fun pendingSyncCount(): Int

    @Query("SELECT * FROM sessions ORDER BY sessionDate DESC LIMIT 50")
    fun recentSessions(): kotlinx.coroutines.flow.Flow<List<SessionEntity>>

    // Sync audit shows only COMPLETE logs: a session that is no longer OPEN (it was closed) AND
    // actually captured at least one check-in. Merely-attempted sessions (opened, or closed with
    // nobody marked) are excluded so the audit lists real attendance runs, not every attempt.
    @Query("""SELECT * FROM sessions
              WHERE status != 'OPEN'
                AND sessionId IN (SELECT DISTINCT sessionId FROM attendance_logs)
              ORDER BY sessionDate DESC LIMIT 50""")
    fun completedSessions(): kotlinx.coroutines.flow.Flow<List<SessionEntity>>

    @Query("SELECT sessionId, studentIdHash FROM attendance_logs WHERE sessionId IN (:sessionIds)")
    fun attendanceForSessions(sessionIds: List<String>): List<SessionStudent>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    fun putPresentDisplay(row: PresentDisplayEntity)

    // Live feed for the active session, newest first.
    @Query("SELECT * FROM present_display WHERE sessionId = :s ORDER BY checkinTimestamp DESC")
    fun liveDisplay(s: String): kotlinx.coroutines.flow.Flow<List<PresentDisplayEntity>>

    @Query("SELECT COUNT(*) FROM present_display WHERE sessionId = :s AND status = 'PRESENT'")
    fun presentCount(s: String): kotlinx.coroutines.flow.Flow<Int>

    // ── QA patrol (offline round) ───────────────────────────────────────────────
    @Query("DELETE FROM patrol_slots") fun clearPatrolSlots()

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    fun putPatrolSlots(rows: List<PatrolSlotEntity>)

    /** Replace the cached day wholesale — a re-fetched manifest is the truth, not an addition. */
    @Transaction
    fun replacePatrolSlots(rows: List<PatrolSlotEntity>) { clearPatrolSlots(); putPatrolSlots(rows) }

    @Query("SELECT * FROM patrol_slots ORDER BY startTime")
    fun patrolSlots(): List<PatrolSlotEntity>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    fun putPatrolLog(log: PatrolLogEntity)

    @Query("SELECT * FROM patrol_logs WHERE synced = 0 ORDER BY takenAt")
    fun unsyncedPatrolLogs(): List<PatrolLogEntity>

    @Query("UPDATE patrol_logs SET synced = 1 WHERE id = :id")
    fun markPatrolLogSynced(id: String)

    @Query("SELECT * FROM patrol_logs WHERE sessionDate = :date ORDER BY takenAt DESC")
    fun patrolLogsForDay(date: String): kotlinx.coroutines.flow.Flow<List<PatrolLogEntity>>

    @Query("SELECT COUNT(*) FROM patrol_logs WHERE synced = 0")
    fun pendingPatrolCount(): Int

    /** Signing out of a patroller account must not leave their round on the handset. */
    @Query("DELETE FROM patrol_logs") fun clearPatrolLogs()

    // ── Sign-out wipe ───────────────────────────────────────────────────────────
    // Everything cached here belongs to the ACCOUNT that was signed in: a cohort roster, that
    // cohort's check-ins, the session history, the patrol round. One handset is shared between
    // coordinators and lent to students, so leaving it behind means the next person signs in and
    // sees the previous one's cohort — and their check-ins would validate against a stale roster.
    //
    // Only ever called once sign-out has established there is nothing left to upload; see
    // performSignOut, which refuses while a session is open and asks before discarding a pending
    // sync. Room's own @Transaction keeps the wipe all-or-nothing.
    @Query("DELETE FROM attendance_logs") fun clearAttendance()
    @Query("DELETE FROM roster") fun clearRoster()
    @Query("DELETE FROM sessions") fun clearSessions()
    @Query("DELETE FROM present_display") fun clearPresentDisplay()
    @Query("DELETE FROM device_bindings") fun clearBindings()

    @androidx.room.Transaction
    fun clearAllForSignOut() {
        clearAttendance(); clearRoster(); clearSessions(); clearPresentDisplay(); clearBindings()
        clearPatrolLogs(); clearPatrolSlots()
    }
}

/** Projection for grouping attendance by session (for analytics). */
data class SessionStudent(val sessionId: String, val studentIdHash: String)

@Database(
    entities = [BindingEntity::class, AttendanceEntity::class, RosterEntity::class,
        SessionEntity::class, PresentDisplayEntity::class,
        PatrolSlotEntity::class, PatrolLogEntity::class],
    version = 4,
)
abstract class AppDatabase : RoomDatabase() {
    abstract fun dao(): AppDao
}

/** v1→v2: adds sessions.closedReason (MANUAL | AUTO_CLOSED). A real migration — NOT destructive —
 *  so a coordinator's pending, not-yet-synced sessions are preserved across the app update. */
val MIGRATION_1_2 = object : androidx.room.migration.Migration(1, 2) {
    override fun migrate(db: androidx.sqlite.db.SupportSQLiteDatabase) {
        db.execSQL("ALTER TABLE sessions ADD COLUMN closedReason TEXT")
    }
}

/** v2→v3: adds roster.studentId + roster.fullName (reg-no/name for offline absentee display). */
val MIGRATION_2_3 = object : androidx.room.migration.Migration(2, 3) {
    override fun migrate(db: androidx.sqlite.db.SupportSQLiteDatabase) {
        db.execSQL("ALTER TABLE roster ADD COLUMN studentId TEXT NOT NULL DEFAULT ''")
        db.execSQL("ALTER TABLE roster ADD COLUMN fullName TEXT NOT NULL DEFAULT ''")
    }
}

/** v3→v4: the QA patrol tables, moved in from the retired standalone patroller app. Additive —
 *  a coordinator upgrading keeps every pending session; the new tables simply start empty. */
val MIGRATION_3_4 = object : androidx.room.migration.Migration(3, 4) {
    override fun migrate(db: androidx.sqlite.db.SupportSQLiteDatabase) {
        db.execSQL(
            """CREATE TABLE IF NOT EXISTS patrol_slots (
                 unitId TEXT NOT NULL, unitName TEXT NOT NULL, courseCode TEXT NOT NULL,
                 lecturerStaffId TEXT NOT NULL, lecturerName TEXT NOT NULL, room TEXT NOT NULL,
                 dayOfWeek INTEGER NOT NULL, startTime TEXT NOT NULL, durationMinutes INTEGER NOT NULL,
                 PRIMARY KEY(unitId, startTime))"""
        )
        db.execSQL(
            """CREATE TABLE IF NOT EXISTS patrol_logs (
                 id TEXT NOT NULL, unitId TEXT NOT NULL, unitName TEXT NOT NULL,
                 courseCode TEXT NOT NULL, lecturerId TEXT NOT NULL, lecturerName TEXT NOT NULL,
                 room TEXT NOT NULL, sessionDate TEXT NOT NULL, scheduledTime TEXT NOT NULL,
                 taught INTEGER NOT NULL, takenAt TEXT NOT NULL, synced INTEGER NOT NULL,
                 PRIMARY KEY(id))"""
        )
        db.execSQL("CREATE INDEX IF NOT EXISTS index_patrol_logs_sessionDate ON patrol_logs (sessionDate)")
    }
}
