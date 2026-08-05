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
 * One cell of the coordinator's WEEKLY timetable grid — one row per day a unit runs.
 *
 * The manifest's per-unit session list carries only the earliest slot of the week, because that is
 * all the attendance picker needs. Rendering a timetable from it silently dropped every extra day:
 * a unit taught Monday and Thursday showed on Monday alone. Online the grid was right (it reads
 * the coordinator overview, which returns the full grid) and offline it was wrong — which is the
 * one state where the cached grid is the only copy the coordinator has.
 */
@Entity(tableName = "timetable_slots", primaryKeys = ["unitId", "dayOfWeek", "startTime"])
data class TimetableSlotEntity(
    val unitId: String,
    val unitName: String,
    val dayOfWeek: Int,           // 1=Mon…7=Sun
    val startTime: String,        // "HH:MM"
    val durationMinutes: Int,
    val room: String = "",
    val lecturerName: String = "",
    val lecturerPhone: String = "",
)

/**
 * One timetabled slot, cached so the QA patroller works with no signal.
 *
 * The patroller used to be a separate app with its own plain-SQLite database. It now lives in
 * this one, which means its cached timetable and its queued observations are encrypted at rest
 * by the same SQLCipher key as everything else — a patrol round is a record of who was and
 * wasn't teaching, and that is not something to leave in the clear on a phone.
 */
// The key is (unit, OFFERING, day, start), and every part of it earns its place.
//
// dayOfWeek, because the manifest now caches the whole week: a unit taught Monday 08:00 and
// Wednesday 08:00 is two slots, and keyed on (unit, start) alone the second overwrote the first.
//
// offeringId, because two cohorts — Day and Evening, or two intakes — can run the SAME unit at the
// SAME hour in different rooms with different lecturers. That is what the field below already
// exists to distinguish, and leaving it out of the key meant one of those two lectures silently
// replaced the other in the cache: the patroller could only ever find one of them, and the tick
// they filed carried the surviving row's offering. The server's own uniqueness key
// (ux_patrol_logs_slot) includes offering_id, so that tick landed against the wrong cohort.
@Entity(tableName = "patrol_slots", primaryKeys = ["unitId", "offeringId", "dayOfWeek", "startTime"])
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
    /** Which cohort's session. Part of the primary key — see the note above. */
    val offeringId: String = "",
    val cohort: String = "",
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
    val offeringId: String = "",
    /** Where/when the lecture was ACTUALLY found, when it had moved. Blank means it
     *  matched the timetable, which is the common case. */
    val foundVenue: String = "",
    val foundStartTime: String = "",
    val foundDate: String = "",
    val venueChanged: Boolean = false,
    val remarks: String = "",
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

    // ── Weekly timetable grid (offline) ─────────────────────────────────────────
    @Query("DELETE FROM timetable_slots") fun clearTimetable()

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    fun putTimetable(rows: List<TimetableSlotEntity>)

    /** A re-fetched grid is the truth, not an addition — a slot the admin DELETED must
     *  disappear from the phone too, which merging would never do. */
    @Transaction
    fun replaceTimetable(rows: List<TimetableSlotEntity>) { clearTimetable(); putTimetable(rows) }

    @Query("SELECT * FROM timetable_slots ORDER BY dayOfWeek, startTime")
    fun timetable(): List<TimetableSlotEntity>

    // ── QA patrol (offline round) ───────────────────────────────────────────────
    @Query("DELETE FROM patrol_slots") fun clearPatrolSlots()

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    fun putPatrolSlots(rows: List<PatrolSlotEntity>)

    /** Replace the cached day wholesale — a re-fetched manifest is the truth, not an addition. */
    @Transaction
    fun replacePatrolSlots(rows: List<PatrolSlotEntity>) { clearPatrolSlots(); putPatrolSlots(rows) }

    @Query("SELECT * FROM patrol_slots ORDER BY dayOfWeek, startTime")
    fun patrolSlots(): List<PatrolSlotEntity>

    /** The cached week narrowed to one weekday — what the patroller is actually walking today.
     *  Slots with no day recorded (0) are included: an unscheduled slot is not evidence of the
     *  wrong day, and dropping it would hide a real lecture from the round. */
    @Query("SELECT * FROM patrol_slots WHERE dayOfWeek = :dow OR dayOfWeek = 0 ORDER BY startTime")
    fun patrolSlotsForDay(dow: Int): List<PatrolSlotEntity>

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
        clearPatrolLogs(); clearPatrolSlots(); clearTimetable()
    }
}

/** Projection for grouping attendance by session (for analytics). */
data class SessionStudent(val sessionId: String, val studentIdHash: String)

@Database(
    entities = [BindingEntity::class, AttendanceEntity::class, RosterEntity::class,
        SessionEntity::class, PresentDisplayEntity::class,
        PatrolSlotEntity::class, PatrolLogEntity::class, TimetableSlotEntity::class],
    version = 6,
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

/** v4→v5: the patroller records WHERE they found the lecture, not just whether it happened.
 *
 *  Lecturers move rooms informally. A tick that could only say taught/not-taught against the
 *  timetabled slot either lost the move entirely or turned it into a false accusation — the
 *  patroller found nothing in A02 and had to mark "not taught" for a lecture that was running
 *  perfectly well in B04. Additive, with defaults, so a patroller mid-round upgrading the app
 *  keeps every queued tick. */
val MIGRATION_4_5 = object : androidx.room.migration.Migration(4, 5) {
    override fun migrate(db: androidx.sqlite.db.SupportSQLiteDatabase) {
        db.execSQL("ALTER TABLE patrol_slots ADD COLUMN offeringId TEXT NOT NULL DEFAULT ''")
        db.execSQL("ALTER TABLE patrol_slots ADD COLUMN cohort TEXT NOT NULL DEFAULT ''")
        db.execSQL("ALTER TABLE patrol_logs ADD COLUMN offeringId TEXT NOT NULL DEFAULT ''")
        db.execSQL("ALTER TABLE patrol_logs ADD COLUMN foundVenue TEXT NOT NULL DEFAULT ''")
        db.execSQL("ALTER TABLE patrol_logs ADD COLUMN foundStartTime TEXT NOT NULL DEFAULT ''")
        db.execSQL("ALTER TABLE patrol_logs ADD COLUMN foundDate TEXT NOT NULL DEFAULT ''")
        db.execSQL("ALTER TABLE patrol_logs ADD COLUMN venueChanged INTEGER NOT NULL DEFAULT 0")
        db.execSQL("ALTER TABLE patrol_logs ADD COLUMN remarks TEXT NOT NULL DEFAULT ''")
    }
}

/**
 * v5→v6: the coordinator's weekly timetable grid is cached, and the patroller caches the WHOLE
 * WEEK rather than one day.
 *
 * Both are offline-completeness fixes. The grid previously had to be rebuilt from the manifest's
 * one-slot-per-unit summary, so with no signal a unit taught twice a week showed once. The patrol
 * cache held whatever day it was last refreshed on, so a patroller who last had signal on Monday
 * searched Monday's timetable on Tuesday and was told, with no hint of trouble, that lectures were
 * where they had been the day before.
 *
 * patrol_slots is REBUILT rather than altered: its primary key gains dayOfWeek AND offeringId,
 * which SQLite cannot add in place. The offering matters for a second, independent reason — two
 * cohorts can run the same unit at the same hour in different rooms, and without it in the key one
 * of those lectures overwrote the other, so the patroller could never find it. The rows are
 * dropped, not copied: they are a cache of one stale day, the next refresh replaces them
 * wholesale, and copying them would carry that stale day across the very upgrade meant to end it.
 * Nothing the patroller RECORDED lives here; patrol_logs, the queue of unsynced ticks, is
 * untouched.
 */
val MIGRATION_5_6 = object : androidx.room.migration.Migration(5, 6) {
    override fun migrate(db: androidx.sqlite.db.SupportSQLiteDatabase) {
        db.execSQL(
            """CREATE TABLE IF NOT EXISTS timetable_slots (
                 unitId TEXT NOT NULL, unitName TEXT NOT NULL, dayOfWeek INTEGER NOT NULL,
                 startTime TEXT NOT NULL, durationMinutes INTEGER NOT NULL,
                 room TEXT NOT NULL DEFAULT '', lecturerName TEXT NOT NULL DEFAULT '',
                 lecturerPhone TEXT NOT NULL DEFAULT '',
                 PRIMARY KEY(unitId, dayOfWeek, startTime))"""
        )
        db.execSQL("DROP TABLE IF EXISTS patrol_slots")
        db.execSQL(
            """CREATE TABLE IF NOT EXISTS patrol_slots (
                 unitId TEXT NOT NULL, unitName TEXT NOT NULL, courseCode TEXT NOT NULL,
                 lecturerStaffId TEXT NOT NULL, lecturerName TEXT NOT NULL, room TEXT NOT NULL,
                 dayOfWeek INTEGER NOT NULL, startTime TEXT NOT NULL, durationMinutes INTEGER NOT NULL,
                 offeringId TEXT NOT NULL DEFAULT '', cohort TEXT NOT NULL DEFAULT '',
                 PRIMARY KEY(unitId, offeringId, dayOfWeek, startTime))"""
        )
    }
}
