package ug.qaat.patroller

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import ug.qaat.patroller.net.Net
import ug.qaat.patroller.store.Store
import ug.qaat.patroller.ui.PatrolRoot

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        // Guarded startup (same shape as the coordinator app): surface a prior crash, then run init
        // inside runCatching so a wrong-clock/Keystore device still reaches setContent and opens.
        runCatching {
            val f = java.io.File(filesDir, "last_crash.txt")
            if (f.exists()) { AppState.lastCrash = f.readText(); f.delete() }
        }
        runCatching {
            Net.init(applicationContext)
            Store.init(applicationContext)
        }.onFailure {
            android.util.Log.e("QAAT", "startup init failed", it)
            if (AppState.lastCrash == null) AppState.lastCrash = "Startup init failed:\n" + android.util.Log.getStackTraceString(it)
        }
        setContent { PatrolRoot() }
    }
}
