package ug.qaat.patroller.db

import android.content.Context
import androidx.room.*
import kotlinx.coroutines.flow.Flow

/** Cached timetable slot for offline inference (one row per timetabled unit today). */
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

/** A patrol observation queued locally; uploaded when back online. */
@Entity(tableName = "patrol_logs")
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
interface PatrolDao {
    @Query("DELETE FROM patrol_slots") fun clearSlots()
    @Insert(onConflict = OnConflictStrategy.REPLACE) fun putSlots(rows: List<PatrolSlotEntity>)
    @Transaction fun replaceSlots(rows: List<PatrolSlotEntity>) { clearSlots(); putSlots(rows) }
    @Query("SELECT * FROM patrol_slots ORDER BY startTime") fun slots(): List<PatrolSlotEntity>

    @Insert(onConflict = OnConflictStrategy.REPLACE) fun putLog(log: PatrolLogEntity)
    @Query("SELECT * FROM patrol_logs WHERE synced = 0") fun unsynced(): List<PatrolLogEntity>
    @Query("UPDATE patrol_logs SET synced = 1 WHERE id = :id") fun markSynced(id: String)
    @Query("SELECT * FROM patrol_logs WHERE sessionDate = :date ORDER BY takenAt DESC")
    fun logsForDay(date: String): Flow<List<PatrolLogEntity>>
    @Query("SELECT COUNT(*) FROM patrol_logs WHERE synced = 0") fun pendingCount(): Int
}

@Database(entities = [PatrolSlotEntity::class, PatrolLogEntity::class], version = 1)
abstract class PatrolDb : RoomDatabase() {
    abstract fun dao(): PatrolDao
    companion object {
        @Volatile private var inst: PatrolDb? = null
        fun get(ctx: Context): PatrolDb = inst ?: synchronized(this) {
            inst ?: Room.databaseBuilder(ctx.applicationContext, PatrolDb::class.java, "qaat_patrol.db")
                .fallbackToDestructiveMigration().build().also { inst = it }
        }
    }
}
