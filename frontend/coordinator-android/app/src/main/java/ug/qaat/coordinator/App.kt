package ug.qaat.coordinator

import android.app.Application
import android.util.Log
import ug.qaat.coordinator.notify.AlertNotifier
import ug.qaat.coordinator.notify.AlertPoller
import java.io.File

/**
 * Persists any uncaught crash to filesDir/last_crash.txt BEFORE the process dies, then lets
 * the platform's default handler run. MainActivity reads + clears the file on next launch and
 * surfaces it once, so a "silent close" becomes a readable stack trace (screenshot + report)
 * instead of vanishing.
 *
 * Also the earliest point in the process, which is where alert delivery is started from: the
 * background poll must run whether the app was opened by the user or woken by WorkManager.
 */
class App : Application() {
    override fun onCreate() {
        super.onCreate()
        val previous = Thread.getDefaultUncaughtExceptionHandler()
        Thread.setDefaultUncaughtExceptionHandler { thread, error ->
            runCatching {
                File(filesDir, "last_crash.txt").writeText(
                    "at ${System.currentTimeMillis()}\non ${thread.name}\n\n" + Log.getStackTraceString(error)
                )
            }
            Log.e("QAAT", "uncaught crash", error)
            previous?.uncaughtException(thread, error)
        }
        // Guarded: alert pop-ups are a convenience, and nothing here may keep the app from opening.
        runCatching {
            AlertNotifier.init(this)
            AlertPoller.start(this)
            AlertPoller.schedule(this)
        }.onFailure { Log.w("QAAT", "alert delivery not started", it) }
    }
}
