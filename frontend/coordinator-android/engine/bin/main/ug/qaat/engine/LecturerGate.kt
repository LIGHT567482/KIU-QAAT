package ug.qaat.engine

import kotlin.math.ceil
import kotlin.math.max

/**
 * Lecturer gate — port of services/api-gateway/internal/handlers/lecturer_gate_scan.go.
 * The lecturer scans the coordinator's gate QR and submits staff-ID + the live room code
 * (+ fingerprint/WebAuthn if enrolled). First valid scan = START; the second = END, which
 * only counts if a sufficient SHARE of enrolled students actually attended (anti-ghost-lecture).
 */
enum class GateState { NOT_STARTED, STARTED, ENDED }
enum class GateAction { START, END }
enum class GateRejection {
    NO_STAFF_ID, STAFF_ID_MISMATCH, BAD_ROOM_CODE, BIOMETRIC_REQUIRED, NO_QUORUM, ALREADY_ENDED,
}

data class GateResult(
    val ok: Boolean,
    val action: GateAction? = null,
    val rejection: GateRejection? = null,
    val requiredQuorum: Int = 0,
)

data class LecturerGateContext(
    val assignedStaffId: String?,     // null/blank ⇒ NO_STAFF_ID
    val roomCodeSecret: ByteArray,
    val gateState: GateState,
    val attended: Int,
    val enrolled: Int,
    val ratio: Double,                // lecturer_attendance_ratio (server default 0.5)
    val requireBiometric: Boolean,    // a WebAuthn passkey is enrolled for this lecturer
)

class LecturerGate(private val nowSeconds: () -> Long = { System.currentTimeMillis() / 1000 }) {

    fun evaluate(
        staffId: String,
        roomCode: String,
        biometricVerified: Boolean,
        ctx: LecturerGateContext,
    ): GateResult {
        val assigned = ctx.assignedStaffId?.trim().orEmpty()
        if (assigned.isEmpty()) return GateResult(false, rejection = GateRejection.NO_STAFF_ID)
        if (staffId.trim() != assigned) return GateResult(false, rejection = GateRejection.STAFF_ID_MISMATCH)

        if (!RoomCode.validate(ctx.roomCodeSecret, roomCode.trim(), nowSeconds()))
            return GateResult(false, rejection = GateRejection.BAD_ROOM_CODE)

        if (ctx.requireBiometric && !biometricVerified)
            return GateResult(false, rejection = GateRejection.BIOMETRIC_REQUIRED)

        return when (ctx.gateState) {
            GateState.NOT_STARTED -> GateResult(true, action = GateAction.START)
            GateState.ENDED -> GateResult(false, rejection = GateRejection.ALREADY_ENDED)
            GateState.STARTED -> {
                val required = max(1, ceil(ctx.ratio * ctx.enrolled).toInt())
                if (ctx.attended < required)
                    GateResult(false, rejection = GateRejection.NO_QUORUM, requiredQuorum = required)
                else
                    GateResult(true, action = GateAction.END, requiredQuorum = required)
            }
        }
    }
}
