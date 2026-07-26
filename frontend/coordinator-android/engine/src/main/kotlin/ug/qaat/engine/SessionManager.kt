package ug.qaat.engine

/**
 * Session lifecycle — port of apps/coordinator-pwa/src/session/state-machine.ts.
 * IDLE → PENDING_LECTURER → ACTIVE → CLOSING → CLOSED, with the same auto-expiry
 * rules (check-in window T+120m from gate-open; hard force-close T+180m from start).
 */
enum class SessionState { IDLE, PENDING_LECTURER, ACTIVE, CLOSING, CLOSED }

class SessionManager(
    val sessionId: String,
    private val nowMillis: () -> Long = { System.currentTimeMillis() },
) {
    var state: SessionState = SessionState.IDLE; private set
    var startedAtMs: Long = 0; private set
    var gateOpenAtMs: Long = 0; private set
    var closedAtMs: Long = 0; private set

    companion object {
        const val CHECKIN_WINDOW_MIN = 120L
        const val FORCE_CLOSE_MIN = 180L
    }

    /** Coordinator opens the session; awaits the lecturer gate. */
    fun open() {
        require(state == SessionState.IDLE) { "session already started" }
        state = SessionState.PENDING_LECTURER
        startedAtMs = nowMillis()
    }

    /** Lecturer passes the START gate → check-in window opens. */
    fun lecturerStarted() {
        require(state == SessionState.PENDING_LECTURER) { "not awaiting lecturer" }
        state = SessionState.ACTIVE
        gateOpenAtMs = nowMillis()
    }

    /** Whether students may currently check in (active + inside the T+120 window). */
    fun checkinOpen(): Boolean =
        state == SessionState.ACTIVE && nowMillis() < gateOpenAtMs + CHECKIN_WINDOW_MIN * 60_000

    fun beginClose() { if (state == SessionState.ACTIVE) state = SessionState.CLOSING }

    fun close() { state = SessionState.CLOSED; closedAtMs = nowMillis() }

    /**
     * Apply auto-expiry; returns true if the session was force-closed.
     * Hard kill at T+180m from start regardless of coordinator action.
     */
    fun tickAutoExpiry(): Boolean {
        if (state == SessionState.CLOSED) return false
        if (startedAtMs > 0 && nowMillis() >= startedAtMs + FORCE_CLOSE_MIN * 60_000) {
            close(); return true
        }
        return false
    }
}
