package ug.qaat.coordinator.sync

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import ug.qaat.coordinator.di.Graph
import ug.qaat.coordinator.net.PresenceClient
import ug.qaat.coordinator.ui.AppState

/**
 * Carries the lecturer's queued presence claims up to the server, and keeps their cached week
 * fresh so the next claim can be matched with no signal.
 *
 * ONE AT A TIME, not one batch. A claim is an individual statement about a moment, and the flag
 * that says it arrived is per-claim: uploading five together and marking all five synced on one
 * 200 would be right, but uploading five and getting a partial failure would leave the phone with
 * no way to say which. Sending them singly costs a few requests on a connection that has already
 * come back and buys an exact answer.
 *
 * Nothing here ever throws at the caller. This runs from a refresh loop and from a button, and a
 * failed upload is the expected state — the claim is already durable in Room, and the next pass
 * takes it.
 */
object PresenceSync {

    suspend fun pendingCount(): Int = withContext(Dispatchers.IO) {
        runCatching { Graph.db.dao().pendingPresenceClaimCount() }.getOrDefault(0)
    }

    /** @return (sent, failed). A no-op when nothing is queued or there is no session. */
    suspend fun syncPending(): Pair<Int, Int> = withContext(Dispatchers.IO) {
        val token = AppState.token ?: return@withContext 0 to 0
        val dao = Graph.db.dao()
        val queued = runCatching { dao.unsyncedPresenceClaims() }.getOrDefault(emptyList())
        if (queued.isEmpty()) return@withContext 0 to 0
        val client = PresenceClient()
        var ok = 0; var fail = 0
        for (c in queued) {
            val sent = runCatching { client.upload(token, listOf(c)) }.getOrDefault(false)
            if (sent) { runCatching { dao.markPresenceClaimSynced(c.id) }; ok++ } else fail++
        }
        ok to fail
    }

    /**
     * Refresh the cached week AND flush the queue — the pair of things a lecturer's phone should
     * do whenever it finds itself online.
     *
     * Order matters. The upload goes first: a queued claim is a record that does not exist
     * anywhere else yet, while a stale timetable only affects claims not yet made.
     */
    suspend fun refresh(): Unit = withContext(Dispatchers.IO) {
        syncPending()
        val token = AppState.token ?: return@withContext
        runCatching { PresenceClient().refreshTimetable(token, Graph.db.dao()) }
    }
}
