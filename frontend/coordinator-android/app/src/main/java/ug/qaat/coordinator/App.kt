package ug.qaat.coordinator

import android.app.Application
import android.util.Log
import java.io.File

/**
 * Persists any uncaught crash to filesDir/last_crash.txt BEFORE the process dies, then lets
 * the platform's default handler run. MainActivity reads + clears the file on next launch and
 * surfaces it once, so a "silent close" becomes a readable stack trace (screenshot + report)
 * instead of vanishing.
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
    }
}
