package ug.qaat.patroller

import android.app.Application
import android.util.Log
import java.io.File

/** Persists any uncaught crash so a "silent close" becomes a readable stack trace (mirrors the
 *  coordinator app). */
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
