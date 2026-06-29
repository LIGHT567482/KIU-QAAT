package ug.qaat.coordinator.ui

import androidx.compose.foundation.layout.padding
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController

private data class Tab(val route: String, val label: String, val icon: String)

private val tabs = listOf(
    Tab("session", "Session", "▶"),
    Tab("absentees", "Absentees", "⚠"),
    Tab("trends", "Trends", "📈"),
    Tab("audit", "Sync", "⟳"),
)

/** Root: branded theme → gate on login → bottom-nav scaffold across the verified-feature screens. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CoordinatorApp() = MaterialTheme(colorScheme = brandedColorScheme(AppState.branding)) {
    if (!AppState.loggedIn) { LoginScreen(onLoggedIn = {}) ; return@MaterialTheme }  // recomposes once token is set
    val nav = rememberNavController()
    Scaffold(
        topBar = { TopAppBar(title = { BrandHeader(AppState.branding) }) },
        bottomBar = {
            val current by nav.currentBackStackEntryAsState()
            NavigationBar {
                tabs.forEach { t ->
                    val selected = current?.destination?.hierarchy?.any { it.route == t.route } == true
                    NavigationBarItem(
                        selected = selected,
                        onClick = { nav.navigate(t.route) { launchSingleTop = true; restoreState = true } },
                        icon = { Text(t.icon) },
                        label = { Text(t.label) },
                    )
                }
            }
        }
    ) { pad ->
        NavHost(nav, startDestination = "session", modifier = Modifier.padding(pad)) {
            composable("session") { SessionScreen(onOpenSession = { nav.navigate("open") }) }
            composable("open") { OpenSessionScreen(onOpened = { nav.navigate("session") { popUpTo("session"); launchSingleTop = true } }) }
            composable("absentees") { AbsenteeScreen() }
            composable("trends") { TrendsScreen() }
            composable("audit") { SyncAuditScreen() }
        }
    }
}
