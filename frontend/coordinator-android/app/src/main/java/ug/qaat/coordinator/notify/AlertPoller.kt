package ug.qaat.coordinator.notify

import android.content.Context
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.NetworkType
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import ug.qaat.coordinator.net.Net
import ug.qaat.coordinator.net.NotificationClient
import ug.qaat.coordinator.store.SessionStore
import ug.qaat.coordinator.ui.AppState
import java.util.concurrent.TimeUnit

/**
 * Delivery of in-app alerts as pop-up notifications, on two clocks.
 *
 * WHY POLLING. This build has no Firebase/FCM: the institution runs its own backend, phones are
 * routinely offline in the room, and adding Google Play Services would break the offline-first
 * hub. So the phone asks, rather than being told. Two schedules cover the two cases:
 *
 *  • [start] — a ~45s loop while the process is alive. This is what makes an alert sent during a
 *    lecture pop up seconds later, app open or not, as long as Android hasn't reclaimed us.
 *  • [schedule] — a WorkManager periodic job, 15 minutes (the platform minimum), which survives
 *    the app being swiped away or the process being killed. Slower, but it always arrives.
 *
 * Both funnel into [AlertNotifier], which is what guarantees an alert pops up exactly once no
 * matter how many times either clock fires.
 */
object AlertPoller {
    private const val WORK_NAME = "qaat_alert_poll"
    private const val FOREGROUND_INTERVAL_MS = 45_000L

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    @Volatile private var running = false

    /** Start the in-process loop. Safe to call more than once; only the first call starts it. */
    fun start(context: Context) {
        // Net holds a lateinit context for the embedded cert; building a client before it is set
        // throws. This runs from Application.onCreate, ahead of MainActivity's own init, so seed it
        // here too — the call is idempotent.
        Net.init(context)
        AlertNotifier.init(context)
        if (running) return
        running = true
        scope.launch {
            // The first wait is short but non-zero: this starts in Application.onCreate, before
            // MainActivity has restored the saved session, so an immediate poll would always find
            // a null token and then sleep a full interval for nothing.
            var first = true
            while (true) {
                delay(if (first) 5_000L else FOREGROUND_INTERVAL_MS)
                first = false
                // Only poll for a signed-in account. A signed-out phone must not be talking to the
                // backend at all, and there is no inbox to read.
                if (AppState.token != null) runCatching { pollOnce() }
            }
        }
    }

    /** Register the background job. Idempotent — KEEP leaves an already-scheduled job alone. */
    fun schedule(context: Context) {
        val work = PeriodicWorkRequestBuilder<AlertWorker>(15, TimeUnit.MINUTES)
            .setConstraints(Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build())
            .build()
        runCatching {
            WorkManager.getInstance(context).enqueueUniquePeriodicWork(
                WORK_NAME, ExistingPeriodicWorkPolicy.KEEP, work,
            )
        }
    }

    /** Stop polling and forget which alerts were shown — called on sign-out. */
    fun stop(context: Context) {
        runCatching { WorkManager.getInstance(context).cancelUniqueWork(WORK_NAME) }
        AlertNotifier.reset()
    }

    /** One fetch → pop up whatever is new. Shared by both clocks. */
    suspend fun pollOnce() {
        val inbox = NotificationClient().inbox()
        if (inbox.isNotEmpty()) AlertNotifier.notifyNew(inbox)
    }

    /**
     * The background half. Runs in a FRESH process when the app has been closed, so nothing is in
     * memory: the token, the pinned certificate and the notifier all have to be re-established
     * before the first request. Without this rehydration the worker would silently fetch nothing.
     */
    class AlertWorker(appContext: Context, params: WorkerParameters) : CoroutineWorker(appContext, params) {
        override suspend fun doWork(): Result {
            runCatching {
                Net.init(applicationContext)              // the embedded, pinned QAAT cert
                SessionStore.init(applicationContext)
                if (AppState.token == null) SessionStore.restore()
                AlertNotifier.init(applicationContext)
            }
            if (AppState.token == null) return Result.success()   // signed out — nothing to poll
            return runCatching { pollOnce(); Result.success() }.getOrElse { Result.retry() }
        }
    }
}
