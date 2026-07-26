package ug.qaat.engine

/**
 * In-room engine models — pure JVM (no Android deps) so the whole check-in engine
 * compiles and is unit-tested off-device. The Android app supplies a Room-backed
 * [Store]; everything else here is shared.
 */

/** The 8 signed fields of a student QR (order matters — see crypto-core QrVerify). */
data class QrFields(
    val studentId: String,
    val tenantId: String,
    val courseId: String,
    val fullName: String,
    val academicYear: String,
    val serialNumber: String,
    val expiryDate: String,   // "YYYY-MM-DD"
    val issuedAt: String,
)

/** A parsed QR submission = the signed fields + the signature (and tenant HMAC, unused here). */
data class SubmittedQr(val fields: QrFields, val signatureB64: String)

/** The active session context, hydrated from the daily manifest + the open session. */
data class ActiveSession(
    val sessionId: String,
    val tenantId: String,
    val academicYear: String,
    val institutionPublicKeyPem: String,
    val studentHashKey: String,                 // per-tenant HMAC key (manifest)
    val rosterHashes: Set<String>,              // hmac(student_id) for enrolled students
    val rosterSerials: Map<String, String>,     // studentIdHash -> current qr_serial_number
)

/** Device-identity signal measured on the submitting (student's) handset. */
data class DeviceContext(val fingerprintHash: String)

data class DeviceBinding(
    val studentIdHash: String,
    val fingerprintHash: String,
    val academicYear: String,
    val firstBoundAt: String,
)

data class AttendanceRecord(
    val logId: String,
    val sessionId: String,
    val studentIdHash: String,
    val deviceFingerprintHash: String,
    val sequenceNumber: Int,
    val checkinTimestamp: String,
    val entryMethod: String = "QR_SCAN",
)

enum class ValidationStatus { PRESENT, REJECTED }

enum class RejectionReason {
    INVALID_SIGNATURE, QR_EXPIRED, TENANT_MISMATCH, NOT_ON_ROSTER, SERIAL_REVOKED,
    DEVICE_MISMATCH, DEVICE_BELONGS_TO_ANOTHER_STUDENT, DUPLICATE_SCAN, DEVICE_ALREADY_USED,
    SESSION_NOT_ACTIVE, GATE_NOT_OPEN,
}

data class ValidationResult(
    val status: ValidationStatus,
    val reason: RejectionReason? = null,
    val studentIdHash: String? = null,
    val auditFlag: String? = null,
)
