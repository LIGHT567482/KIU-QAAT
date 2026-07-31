package ug.qaat.engine

import ug.qaat.crypto.QrVerify
import ug.qaat.crypto.VaultCrypto
import java.time.LocalDate
import java.time.ZoneOffset

/**
 * The in-room check-in validation chain — a faithful port of
 * apps/coordinator-pwa/src/qr/validator.ts (and the Go gateway gate order).
 * Pure: all I/O goes through [Store] and the injected clock/id functions, so the
 * whole chain is unit-tested off-device. Any failure returns immediately.
 */
class CheckinValidator(
    private val store: Store,
    // TESTING SWITCH: when false, the device anti-cheat is relaxed so ONE phone can check in MANY
    // students and a student can switch phones freely — needed to test with few handsets. The
    // roster check + one-attendance-per-student-per-session (DUPLICATE_SCAN) always stay on. Flip
    // back to true (via the caller) to re-enable one-student-one-phone before going live.
    private val enforceDeviceLock: Boolean = true,
    private val nowMillis: () -> Long = { System.currentTimeMillis() },
    private val nowIso: () -> String = { java.time.Instant.ofEpochMilli(System.currentTimeMillis()).toString() },
    private val newUuid: () -> String = { java.util.UUID.randomUUID().toString() },
) {
    fun validate(rawQr: String, session: ActiveSession, device: DeviceContext): ValidationResult {
        // ── Step 1: parse + RSA-2048 signature ──────────────────────────────────
        val qr = FlatJson.parseQr(rawQr)
            ?: return ValidationResult(ValidationStatus.REJECTED, RejectionReason.INVALID_SIGNATURE)
        val f = qr.fields
        val body = QrVerify.canonicalBody(
            QrVerify.QrPayload(f.studentId, f.tenantId, f.courseId, f.fullName, f.academicYear, f.serialNumber, f.expiryDate, f.issuedAt)
        )
        val sigOk = try { QrVerify.verify(session.institutionPublicKeyPem, body, qr.signatureB64) } catch (_: Exception) { false }
        if (!sigOk) return ValidationResult(ValidationStatus.REJECTED, RejectionReason.INVALID_SIGNATURE)

        // ── Step 2: expiry (malformed date ⇒ expired) ───────────────────────────
        val expiryMs = try {
            LocalDate.parse(f.expiryDate).atStartOfDay(ZoneOffset.UTC).toInstant().toEpochMilli()
        } catch (_: Exception) { return ValidationResult(ValidationStatus.REJECTED, RejectionReason.QR_EXPIRED) }
        if (expiryMs < nowMillis()) return ValidationResult(ValidationStatus.REJECTED, RejectionReason.QR_EXPIRED)

        // ── Step 3: tenant match ─────────────────────────────────────────────────
        if (f.tenantId != session.tenantId)
            return ValidationResult(ValidationStatus.REJECTED, RejectionReason.TENANT_MISMATCH)

        val studentIdHash = VaultCrypto.hmacHex(session.studentHashKey, f.studentId)

        // ── Steps 6a + 4: cross-student device lock + roster ─────────────────────
        rosterAndDeviceLock(studentIdHash, session, device)?.let { return ValidationResult(ValidationStatus.REJECTED, it) }

        // ── Step 4b: serial must match the current issued serial ─────────────────
        val currentSerial = session.rosterSerials[studentIdHash]
        if (currentSerial == null || f.serialNumber != currentSerial)
            return ValidationResult(ValidationStatus.REJECTED, RejectionReason.SERIAL_REVOKED)

        // ── Steps 5–7 + record ───────────────────────────────────────────────────
        return bindAndRecord(studentIdHash, session, device)
    }

    /**
     * Reg-number check-in (no student QR): presence is proven by the hotspot LAN, identity is the
     * typed registration number matched against the cached roster. Skips the QR-only steps
     * (signature/expiry/tenant/serial) but keeps the SAME anti-cheat as [validate] — device binding,
     * one-attendance-per-student, and one-device-per-session — all enforced on the coordinator phone.
     */
    fun validateReg(regNumber: String, session: ActiveSession, device: DeviceContext): ValidationResult {
        val reg = regNumber.trim()
        if (reg.isEmpty()) return ValidationResult(ValidationStatus.REJECTED, RejectionReason.NOT_ON_ROSTER)
        val studentIdHash = VaultCrypto.hmacHex(session.studentHashKey, reg)
        rosterAndDeviceLock(studentIdHash, session, device)?.let { return ValidationResult(ValidationStatus.REJECTED, it) }
        return bindAndRecord(studentIdHash, session, device)
    }

    /** Step 6a (a device is bound to a DIFFERENT student) + Step 4 (on the roster). Null = OK. */
    private fun rosterAndDeviceLock(studentIdHash: String, session: ActiveSession, device: DeviceContext): RejectionReason? {
        if (enforceDeviceLock) {
            store.bindingByFingerprint(device.fingerprintHash)?.let {
                if (it.studentIdHash != studentIdHash) return RejectionReason.DEVICE_BELONGS_TO_ANOTHER_STUDENT
            }
        }
        if (studentIdHash !in session.rosterHashes) return RejectionReason.NOT_ON_ROSTER
        return null
    }

    /** Steps 5 (bind device on first check-in), 6 (duplicate), 7 (one device), then append PRESENT. */
    private fun bindAndRecord(studentIdHash: String, session: ActiveSession, device: DeviceContext): ValidationResult {
        val stored = store.bindingByStudent(studentIdHash)
        if (stored != null) {
            if (enforceDeviceLock && stored.fingerprintHash != device.fingerprintHash)
                return ValidationResult(ValidationStatus.REJECTED, RejectionReason.DEVICE_MISMATCH, auditFlag = "DEVICE_MISMATCH")
        } else {
            store.putBinding(DeviceBinding(studentIdHash, device.fingerprintHash, session.academicYear, nowIso()))
        }
        if (store.hasAttendance(session.sessionId, studentIdHash))
            return ValidationResult(ValidationStatus.REJECTED, RejectionReason.DUPLICATE_SCAN)
        if (enforceDeviceLock && store.deviceUsedByOther(session.sessionId, device.fingerprintHash, studentIdHash))
            return ValidationResult(ValidationStatus.REJECTED, RejectionReason.DEVICE_ALREADY_USED)
        store.addAttendance(
            AttendanceRecord(
                logId = newUuid(),
                sessionId = session.sessionId,
                studentIdHash = studentIdHash,
                deviceFingerprintHash = device.fingerprintHash,
                sequenceNumber = store.attendanceCount(session.sessionId) + 1,
                checkinTimestamp = nowIso(),
            )
        )
        return ValidationResult(ValidationStatus.PRESENT, studentIdHash = studentIdHash)
    }
}
