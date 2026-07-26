import ug.qaat.engine.RoomCode

/** args: <secret> <unixSeconds> <expectedCodeFromGo> */
fun main(args: Array<String>) {
    val secret = args[0].toByteArray(Charsets.UTF_8)
    val t = args[1].toLong()
    val expected = args[2]
    var fail = 0
    val got = RoomCode.derive(secret, t)
    if (got != expected) { fail++; System.err.println("DERIVE_MISMATCH go=$expected kt=$got") } else println("PASS derive == server ($got)")
    if (!RoomCode.validate(secret, expected, t)) { fail++; System.err.println("VALIDATE_FAIL current") } else println("PASS validate current code")
    // ±1 step window: a code from the previous step still validates.
    val prev = RoomCode.derive(secret, t - RoomCode.STEP_SECONDS)
    if (!RoomCode.validate(secret, prev, t)) { fail++; System.err.println("VALIDATE_FAIL prev-step") } else println("PASS validate prev-step code (±1 window)")
    // A code two steps old must NOT validate.
    val old = RoomCode.derive(secret, t - 3 * RoomCode.STEP_SECONDS)
    if (old != expected && RoomCode.validate(secret, old, t)) { fail++; System.err.println("STALE_ACCEPTED") } else println("PASS stale code rejected")
    if (fail == 0) println("ALL_ROOMCODE_PARITY_OK") else { println("$fail FAIL"); kotlin.system.exitProcess(1) }
}
