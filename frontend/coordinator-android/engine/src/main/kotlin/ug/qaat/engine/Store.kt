package ug.qaat.engine

/**
 * Storage boundary for the in-room engine. The Android app implements this with
 * Room/SQLite; tests and the device spike use [InMemoryStore]. Keeping the engine
 * storage-agnostic is what lets the whole validation chain be unit-tested off-device.
 */
interface Store {
    /** Step 6a: any device binding for this fingerprint (regardless of student). */
    fun bindingByFingerprint(fingerprintHash: String): DeviceBinding?

    /** Step 5: the binding for this student (if the device was already bound). */
    fun bindingByStudent(studentIdHash: String): DeviceBinding?

    /** Step 5 first-scan: bind this device to this student for the year. */
    fun putBinding(binding: DeviceBinding)

    /** Step 6: has this student already a record in this session? */
    fun hasAttendance(sessionId: String, studentIdHash: String): Boolean

    /** Step 7: did this device already record a DIFFERENT student in this session? */
    fun deviceUsedByOther(sessionId: String, fingerprintHash: String, studentIdHash: String): Boolean

    /** Append-only sequence number for the next record in this session. */
    fun attendanceCount(sessionId: String): Int

    /** Append-only insert of a confirmed attendance record. */
    fun addAttendance(record: AttendanceRecord)
}

/** Reference in-memory implementation (tests + the Phase-0 device spike). */
class InMemoryStore : Store {
    private val bindings = mutableListOf<DeviceBinding>()
    private val attendance = mutableListOf<AttendanceRecord>()

    override fun bindingByFingerprint(fingerprintHash: String): DeviceBinding? =
        bindings.firstOrNull { it.fingerprintHash == fingerprintHash }

    override fun bindingByStudent(studentIdHash: String): DeviceBinding? =
        bindings.firstOrNull { it.studentIdHash == studentIdHash }

    override fun putBinding(binding: DeviceBinding) {
        bindings.removeAll { it.studentIdHash == binding.studentIdHash }
        bindings.add(binding)
    }

    override fun hasAttendance(sessionId: String, studentIdHash: String): Boolean =
        attendance.any { it.sessionId == sessionId && it.studentIdHash == studentIdHash }

    override fun deviceUsedByOther(sessionId: String, fingerprintHash: String, studentIdHash: String): Boolean =
        attendance.any { it.sessionId == sessionId && it.deviceFingerprintHash == fingerprintHash && it.studentIdHash != studentIdHash }

    override fun attendanceCount(sessionId: String): Int =
        attendance.count { it.sessionId == sessionId }

    override fun addAttendance(record: AttendanceRecord) { attendance.add(record) }

    fun all(): List<AttendanceRecord> = attendance.toList()
}
