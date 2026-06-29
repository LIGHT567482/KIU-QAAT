import ug.qaat.engine.*

/** Off-device verification of the lecturer gate chain (START/END + every rejection). */
fun main() {
    val secret = "gate-secret".toByteArray()
    val t = 1782604800L
    val code = RoomCode.derive(secret, t)              // a valid live code at time t
    val gate = LecturerGate(nowSeconds = { t })
    var fail = 0

    fun ctx(state: GateState, attended: Int = 0, enrolled: Int = 0, ratio: Double = 0.5,
            staff: String? = "KIU/STAFF/001", bio: Boolean = false) =
        LecturerGateContext(staff, secret, state, attended, enrolled, ratio, bio)

    fun check(name: String, r: GateResult, ok: Boolean, action: GateAction? = null, rej: GateRejection? = null) {
        val pass = r.ok == ok && r.action == action && r.rejection == rej
        println((if (pass) "PASS" else "FAIL") + "  $name -> ok=${r.ok} ${r.action ?: r.rejection ?: ""}")
        if (!pass) fail++
    }

    // START on a valid first scan.
    check("start", gate.evaluate("KIU/STAFF/001", code, false, ctx(GateState.NOT_STARTED)), true, GateAction.START)
    // No staff registered.
    check("no-staff-id", gate.evaluate("KIU/STAFF/001", code, false, ctx(GateState.NOT_STARTED, staff = " ")), false, rej = GateRejection.NO_STAFF_ID)
    // Wrong staff id.
    check("staff-mismatch", gate.evaluate("KIU/STAFF/999", code, false, ctx(GateState.NOT_STARTED)), false, rej = GateRejection.STAFF_ID_MISMATCH)
    // Bad / stale room code.
    check("bad-code", gate.evaluate("KIU/STAFF/001", "000000", false, ctx(GateState.NOT_STARTED)), false, rej = GateRejection.BAD_ROOM_CODE)
    // Biometric enrolled but not verified.
    check("biometric-required", gate.evaluate("KIU/STAFF/001", code, false, ctx(GateState.NOT_STARTED, bio = true)), false, rej = GateRejection.BIOMETRIC_REQUIRED)
    check("biometric-ok", gate.evaluate("KIU/STAFF/001", code, true, ctx(GateState.NOT_STARTED, bio = true)), true, GateAction.START)
    // END with quorum met: enrolled 60, ratio .5 → required 30; attended 30 → END.
    check("end-quorum-met", gate.evaluate("KIU/STAFF/001", code, false, ctx(GateState.STARTED, attended = 30, enrolled = 60)), true, GateAction.END)
    // END without quorum: attended 29 < 30 → NO_QUORUM.
    check("end-no-quorum", gate.evaluate("KIU/STAFF/001", code, false, ctx(GateState.STARTED, attended = 29, enrolled = 60)), false, rej = GateRejection.NO_QUORUM)
    // Quorum floor: enrolled 0 → required max(1,0)=1; attended 1 → END.
    check("end-floor", gate.evaluate("KIU/STAFF/001", code, false, ctx(GateState.STARTED, attended = 1, enrolled = 0)), true, GateAction.END)
    // Already ended.
    check("already-ended", gate.evaluate("KIU/STAFF/001", code, false, ctx(GateState.ENDED, attended = 60, enrolled = 60)), false, rej = GateRejection.ALREADY_ENDED)

    println(if (fail == 0) "\nALL_LECTURER_GATE_TESTS_PASSED" else "\n$fail FAILED")
    if (fail != 0) kotlin.system.exitProcess(1)
}
