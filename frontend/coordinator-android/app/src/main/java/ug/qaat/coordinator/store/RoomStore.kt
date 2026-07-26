package ug.qaat.coordinator.store

import ug.qaat.coordinator.db.AppDao
import ug.qaat.coordinator.db.AttendanceEntity
import ug.qaat.coordinator.db.BindingEntity
import ug.qaat.engine.AttendanceRecord
import ug.qaat.engine.DeviceBinding
import ug.qaat.engine.Store

/** Room-backed implementation of the engine's storage boundary. */
class RoomStore(private val dao: AppDao) : Store {
    override fun bindingByFingerprint(fingerprintHash: String): DeviceBinding? =
        dao.bindingByFingerprint(fingerprintHash)?.toDomain()

    override fun bindingByStudent(studentIdHash: String): DeviceBinding? =
        dao.bindingByStudent(studentIdHash)?.toDomain()

    override fun putBinding(binding: DeviceBinding) =
        dao.putBinding(BindingEntity(binding.studentIdHash, binding.fingerprintHash, binding.academicYear, binding.firstBoundAt))

    override fun hasAttendance(sessionId: String, studentIdHash: String): Boolean =
        dao.attendanceCountFor(sessionId, studentIdHash) > 0

    override fun deviceUsedByOther(sessionId: String, fingerprintHash: String, studentIdHash: String): Boolean =
        dao.deviceUsedByOtherCount(sessionId, fingerprintHash, studentIdHash) > 0

    override fun attendanceCount(sessionId: String): Int = dao.attendanceCount(sessionId)

    override fun addAttendance(record: AttendanceRecord) = dao.addAttendance(
        AttendanceEntity(
            record.logId, record.sessionId, record.studentIdHash, record.deviceFingerprintHash,
            record.sequenceNumber, record.checkinTimestamp, record.entryMethod,
        )
    )

    private fun BindingEntity.toDomain() = DeviceBinding(studentIdHash, fingerprintHash, academicYear, firstBoundAt)
}
