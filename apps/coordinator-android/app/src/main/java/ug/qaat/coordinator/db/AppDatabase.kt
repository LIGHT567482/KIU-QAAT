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
)

/** Closed-session history (per unit) so absentee/trend analytics can run offline. */
@Entity(tableName = "sessions", indices = [Index("unitId")])
data class SessionEntity(
    @PrimaryKey val sessionId: String,
    val unitId: String,
    val sessionDate: String,     // ISO date, used to order chronologically + group by week
    val status: String,
    val enrolled: Int,
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

    @Query("SELECT * FROM sessions ORDER BY sessionDate DESC LIMIT 50")
    fun recentSessions(): kotlinx.coroutines.flow.Flow<List<SessionEntity>>

    @Query("SELECT sessionId, studentIdHash FROM attendance_logs WHERE sessionId IN (:sessionIds)")
    fun attendanceForSessions(sessionIds: List<String>): List<SessionStudent>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    fun putPresentDisplay(row: PresentDisplayEntity)

    // Live feed for the active session, newest first.
    @Query("SELECT * FROM present_display WHERE sessionId = :s ORDER BY checkinTimestamp DESC")
    fun liveDisplay(s: String): kotlinx.coroutines.flow.Flow<List<PresentDisplayEntity>>

    @Query("SELECT COUNT(*) FROM present_display WHERE sessionId = :s AND status = 'PRESENT'")
    fun presentCount(s: String): kotlinx.coroutines.flow.Flow<Int>
}

/** Projection for grouping attendance by session (for analytics). */
data class SessionStudent(val sessionId: String, val studentIdHash: String)

@Database(
    entities = [BindingEntity::class, AttendanceEntity::class, RosterEntity::class,
        SessionEntity::class, PresentDisplayEntity::class],
    version = 1,
)
abstract class AppDatabase : RoomDatabase() {
    abstract fun dao(): AppDao
}
