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

        // ── Step 6a: cross-student device lock ───────────────────────────────────
        store.bindingByFingerprint(device.fingerprintHash)?.let {
            if (it.studentIdHash != studentIdHash)
                return ValidationResult(ValidationStatus.REJECTED, RejectionReason.DEVICE_BELONGS_TO_ANOTHER_STUDENT)
        }

        // ── Step 4: roster lookup ────────────────────────────────────────────────
        if (studentIdHash !in session.rosterHashes)
            return ValidationResult(ValidationStatus.REJECTED, RejectionReason.NOT_ON_ROSTER)

        // ── Step 4b: serial must match the current issued serial ─────────────────
        val currentSerial = session.rosterSerials[studentIdHash]
        if (currentSerial == null || f.serialNumber != currentSerial)
            return ValidationResult(ValidationStatus.REJECTED, RejectionReason.SERIAL_REVOKED)

        // ── Step 5: hardware fingerprint (bind on first scan) ────────────────────
        val stored = store.bindingByStudent(studentIdHash)
        if (stored != null) {
            if (stored.fingerprintHash != device.fingerprintHash)
                return ValidationResult(ValidationStatus.REJECTED, RejectionReason.DEVICE_MISMATCH, auditFlag = "DEVICE_MISMATCH")
        } else {
            store.putBinding(DeviceBinding(studentIdHash, device.fingerprintHash, session.academicYear, nowIso()))
        }

        // ── Step 6: duplicate scan ───────────────────────────────────────────────
        if (store.hasAttendance(session.sessionId, studentIdHash))
            return ValidationResult(ValidationStatus.REJECTED, RejectionReason.DUPLICATE_SCAN)

        // ── Step 7: one device per session ───────────────────────────────────────
        if (store.deviceUsedByOther(session.sessionId, device.fingerprintHash, studentIdHash))
            return ValidationResult(ValidationStatus.REJECTED, RejectionReason.DEVICE_ALREADY_USED)

        // ── PRESENT: append-only record ──────────────────────────────────────────
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
