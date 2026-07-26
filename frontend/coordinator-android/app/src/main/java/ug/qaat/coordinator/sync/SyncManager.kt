package ug.qaat.coordinator.sync

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import ug.qaat.coordinator.di.Graph
import ug.qaat.coordinator.net.Net
import ug.qaat.coordinator.net.SyncClient
import ug.qaat.coordinator.ui.AppState
import ug.qaat.engine.AttendanceRecord
import ug.qaat.engine.SessionPackage
import java.time.Instant

/**
 * Uploads every closed session whose sealed package hasn't reached the backend yet — the
 * attendances "waiting to be synced". Each is re-sealed from the durable Room ledger and
 * pushed via the same chunked protocol the PWA uses; only a confirmed 200 flips it to
 * SYNCED, so nothing is ever lost if the coordinator is offline at close time.
 */
object SyncManager {
    suspend fun pendingCount(): Int = withContext(Dispatchers.IO) { Graph.db.dao().pendingSyncCount() }

    /** @return (synced, failed). Safe to call anytime; a no-op when nothing is pending. */
    suspend fun syncPending(): Pair<Int, Int> = withContext(Dispatchers.IO) {
        val token = AppState.token ?: return@withContext 0 to 0
        val userId = AppState.userId ?: return@withContext 0 to 0
        val bindingKey = AppState.deviceBindingKey ?: return@withContext 0 to 0
        val dao = Graph.db.dao()
        var ok = 0; var fail = 0
        for (s in dao.pendingSyncSessions()) {
            val records = dao.rosterForSession(s.sessionId).map {
                AttendanceRecord(it.logId, it.sessionId, it.studentIdHash, it.deviceFingerprintHash,
                    it.sequenceNumber, it.checkinTimestamp, it.entryMethod)
            }
            val pkg = SessionPackage.build(
                s.sessionId, userId, records, sealedAt = Instant.now().toString(),
                unitId = s.unitId, sessionDate = s.sessionDate,
            )
            val success = runCatching { SyncClient(Net.baseUrl, token).upload(userId, s.sessionId, bindingKey, pkg) }
                .getOrDefault(false)
            if (success) { dao.upsertSession(s.copy(status = "SYNCED")); ok++ }
            else { dao.upsertSession(s.copy(status = "PENDING_SYNC")); fail++ }
        }
        ok to fail
    }
}
