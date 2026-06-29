package ug.qaat.coordinator

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import ug.qaat.coordinator.di.Graph
import ug.qaat.coordinator.ui.CoordinatorApp

/**
 * Hosts the coordinator UI (bottom-nav across the verified-feature screens):
 * Session (live roster + room code + announce), Absentees, Trends, Sync audit.
 *
 * To exercise it on the emulator, wire a "dev: open session" that pulls the manifest
 * and calls SessionService.server.setLive(...) + sets AppState.currentSessionId/UnitId —
 * see EMULATOR_TESTING.md §3.
 */
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        Graph.init(applicationContext)
        setContent { CoordinatorApp() }   // CoordinatorApp owns the branded MaterialTheme
    }
}
