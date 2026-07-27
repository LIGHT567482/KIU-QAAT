package ug.qaat.coordinator.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AccountCircle
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Popup
import androidx.compose.ui.window.PopupProperties
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import ug.qaat.coordinator.di.Graph
import ug.qaat.coordinator.net.AuthClient
import ug.qaat.coordinator.net.BrandingClient
import ug.qaat.coordinator.net.DashboardClient
import ug.qaat.coordinator.net.ManifestClient
import ug.qaat.coordinator.store.SessionStore
import ug.qaat.coordinator.sync.SyncManager

private data class Tab(val route: String, val label: String, val icon: String)

private val tabs = listOf(
    Tab("dashboard", "Home", "🏠"),
    Tab("session", "Attendance", "▶"),
    Tab("absentees", "Absentees", "⚠"),
    Tab("trends", "Trends", "📈"),
    Tab("audit", "Sync", "⟳"),
)

/** Pull current data (branding + manifest + cohort) + upload pending attendance. Bumps
 *  refreshTick so screens reload. Driven by the top-nav ⟳ icon and on every open. */
private suspend fun refreshAll() {
    val t = AppState.token ?: return
    AppState.refreshing = true
    val dao = Graph.db.dao()
    runCatching { BrandingClient(t).fetch()?.let { AppState.branding = it; runCatching { SessionStore.saveBranding(it) } } }
    runCatching { val m = ManifestClient(dao).fetchAndStore(t); AppState.manifest = m; SessionStore.saveManifest(m) }
    runCatching {
        DashboardClient().overview(t).offering?.let {
            AppState.cohortLabel = listOf(it.courseName, it.sessionType, "Year ${it.studyYear}", "Sem ${it.semester}", it.level, it.intake)
                .filter { s -> s.isNotBlank() }.joinToString(" · ")
        }
    }
    runCatching { SyncManager.syncPending() }
    AppState.refreshTick++
    AppState.refreshing = false
}

/** Root: branded theme → gate on login → bottom-nav scaffold. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CoordinatorApp() = MaterialTheme(colorScheme = brandedColorScheme(AppState.branding, AppState.darkTheme)) {
    AppState.lastCrash?.let { trace -> CrashReportDialog(trace) { AppState.lastCrash = null } }
    if (!AppState.loggedIn) { LoginScreen(onLoggedIn = {}) ; return@MaterialTheme }

    LaunchedEffect(AppState.token) {
        val t = AppState.token ?: return@LaunchedEffect
        if (AppState.manifest == null) {
            AppState.manifest = withContext(Dispatchers.IO) { runCatching { SessionStore.loadManifest(Graph.db.dao()) }.getOrNull() }
        }
        withContext(Dispatchers.IO) { runCatching { refreshAll() } }
    }

    val scope = rememberCoroutineScope()
    val navColor = navBarColor(AppState.branding)
    val onNav = navColor?.let { onNavColor(it) }
    var showProfile by remember { mutableStateOf(false) }
    var showChangePw by remember { mutableStateOf(false) }
    if (showChangePw) ChangePasswordDialog(onClose = { showChangePw = false })

    val nav = rememberNavController()
    Scaffold(
        containerColor = (if (!AppState.darkTheme) appBackgroundColor(AppState.branding) else null)
            ?: MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                colors = if (navColor != null) TopAppBarDefaults.topAppBarColors(
                    containerColor = navColor, titleContentColor = onNav!!, actionIconContentColor = onNav,
                ) else TopAppBarDefaults.topAppBarColors(),
                title = { BrandHeader(AppState.branding) },
                actions = {
                    // Just two icons — a refresh loop and a profile — to declutter the bar.
                    IconButton(onClick = { scope.launch { withContext(Dispatchers.IO) { runCatching { refreshAll() } } } }, enabled = !AppState.refreshing) {
                        Text(if (AppState.refreshing) "…" else "⟳", fontSize = 20.sp, color = onNav ?: MaterialTheme.colorScheme.primary)
                    }
                    Box {
                        IconButton(onClick = { showProfile = !showProfile }) {
                            Icon(Icons.Filled.AccountCircle, contentDescription = "Profile", tint = Color.White)
                        }
                        if (showProfile) ProfilePopup(
                            onClose = { showProfile = false },
                            onChangePw = { showProfile = false; showChangePw = true },
                            onSignOut = { showProfile = false; signOut() },
                        )
                    }
                },
            )
        },
        bottomBar = {
            val current by nav.currentBackStackEntryAsState()
            NavigationBar(containerColor = navColor ?: MaterialTheme.colorScheme.surface) {
                val itemColors = if (onNav != null) NavigationBarItemDefaults.colors(
                    selectedIconColor = onNav, selectedTextColor = onNav,
                    unselectedIconColor = onNav.copy(alpha = .65f), unselectedTextColor = onNav.copy(alpha = .65f),
                    indicatorColor = onNav.copy(alpha = .18f),
                ) else NavigationBarItemDefaults.colors()
                tabs.forEach { t ->
                    val selected = current?.destination?.hierarchy?.any { it.route == t.route } == true
                    NavigationBarItem(
                        selected = selected,
                        onClick = { nav.navigate(t.route) { launchSingleTop = true; restoreState = true } },
                        icon = { Text(t.icon) }, label = { Text(t.label) }, colors = itemColors,
                    )
                }
            }
        }
    ) { pad ->
        NavHost(nav, startDestination = "dashboard", modifier = Modifier.padding(pad)) {
            composable("dashboard") { DashboardScreen() }
            composable("session") { SessionScreen(onOpenSession = { nav.navigate("open") }) }
            composable("open") { OpenSessionScreen(onOpened = { nav.navigate("session") { popUpTo("dashboard"); launchSingleTop = true } }) }
            composable("absentees") { AbsenteeScreen() }
            composable("trends") { TrendsScreen() }
            composable("audit") { SyncAuditScreen() }
        }
    }
}

/** Corner popup: the coordinator's identity + cohort + settings (theme, change password,
 *  sign out), with an × to close. Everything that used to crowd the top bar lives here. */
@Composable
private fun ProfilePopup(onClose: () -> Unit, onChangePw: () -> Unit, onSignOut: () -> Unit) {
    Popup(alignment = Alignment.TopEnd, onDismissRequest = onClose, properties = PopupProperties(focusable = true)) {
        Surface(
            shape = RoundedCornerShape(14.dp), tonalElevation = 6.dp, shadowElevation = 8.dp,
            modifier = Modifier.padding(8.dp).widthIn(min = 240.dp, max = 300.dp),
        ) {
            Column(Modifier.padding(16.dp).verticalScroll(rememberScrollState())) {
                Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                    Text("Profile", fontWeight = FontWeight.Bold, modifier = Modifier.weight(1f))
                    TextButton(onClick = onClose, contentPadding = PaddingValues(4.dp)) { Text("✕") }
                }
                Spacer(Modifier.height(4.dp))
                val name = listOfNotNull(AppState.coordinatorTitle?.takeIf { it.isNotBlank() }, AppState.displayName).joinToString(" ")
                Text(name, fontWeight = FontWeight.SemiBold, fontSize = 15.sp)
                AppState.role?.let { Text(it, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                AppState.coordinatorRegNo?.takeIf { it.isNotBlank() }?.let { InfoLine("Reg no.", it) }
                AppState.coordinatorEmail?.takeIf { it.isNotBlank() }?.let { InfoLine("Email", it) }
                AppState.cohortLabel?.takeIf { it.isNotBlank() }?.let {
                    Spacer(Modifier.height(8.dp)); Text("Cohort", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Text(it, fontSize = 13.sp)
                }
                HorizontalDivider(Modifier.padding(vertical = 10.dp))
                TextButton(onClick = { AppState.darkTheme = !AppState.darkTheme; SessionStore.saveTheme(AppState.darkTheme) }, modifier = Modifier.fillMaxWidth()) {
                    Text((if (AppState.darkTheme) "☀  Light mode" else "☾  Dark mode"), modifier = Modifier.fillMaxWidth())
                }
                TextButton(onClick = onChangePw, modifier = Modifier.fillMaxWidth()) { Text("🔑  Change password", modifier = Modifier.fillMaxWidth()) }
                TextButton(onClick = onSignOut, modifier = Modifier.fillMaxWidth()) {
                    Text("⎋  Sign out", color = MaterialTheme.colorScheme.error, modifier = Modifier.fillMaxWidth())
                }
            }
        }
    }
}

@Composable
private fun InfoLine(label: String, value: String) {
    Spacer(Modifier.height(6.dp))
    Text(label, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    Text(value, fontSize = 13.sp)
}

/** Change-password dialog — mirrors the PWA header action. */
@Composable
private fun ChangePasswordDialog(onClose: () -> Unit) {
    var cur by remember { mutableStateOf("") }
    var next by remember { mutableStateOf("") }
    var confirm by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var err by remember { mutableStateOf<String?>(null) }
    var done by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    val pw = PasswordVisualTransformation()

    AlertDialog(
        onDismissRequest = onClose,
        title = { Text("Change password") },
        text = {
            if (done) Text("✓ Password changed.", color = MaterialTheme.colorScheme.primary)
            else Column {
                err?.let { Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall) }
                OutlinedTextField(cur, { cur = it }, label = { Text("Current password") }, singleLine = true, visualTransformation = pw, modifier = Modifier.fillMaxWidth())
                Spacer(Modifier.padding(4.dp))
                OutlinedTextField(next, { next = it }, label = { Text("New password (min 8)") }, singleLine = true, visualTransformation = pw, modifier = Modifier.fillMaxWidth())
                Spacer(Modifier.padding(4.dp))
                OutlinedTextField(confirm, { confirm = it }, label = { Text("Confirm new password") }, singleLine = true, visualTransformation = pw, modifier = Modifier.fillMaxWidth())
            }
        },
        confirmButton = {
            if (done) TextButton(onClick = onClose) { Text("Close") }
            else TextButton(enabled = !busy && cur.isNotBlank() && next.length >= 8 && next == confirm, onClick = {
                busy = true; err = null
                scope.launch {
                    err = AppState.token?.let { AuthClient().changePassword(it, cur, next) } ?: "Not signed in"
                    if (err == null) done = true
                    busy = false
                }
            }) { Text(if (busy) "Saving…" else "Update") }
        },
        dismissButton = { if (!done) TextButton(onClick = onClose) { Text("Cancel") } },
    )
}

/** Shows the previous run's uncaught crash once, so a silent close becomes a readable,
 *  copyable stack trace the coordinator can send for a fix. */
@Composable
private fun CrashReportDialog(trace: String, onClose: () -> Unit) {
    val clipboard = androidx.compose.ui.platform.LocalClipboardManager.current
    AlertDialog(
        onDismissRequest = onClose,
        title = { Text("The app closed unexpectedly last time") },
        text = {
            Column(Modifier.heightIn(max = 320.dp).verticalScroll(rememberScrollState())) {
                Text("Please screenshot or copy this and send it so it can be fixed:", style = MaterialTheme.typography.bodySmall)
                Spacer(Modifier.height(8.dp))
                Text(trace, fontSize = 11.sp, fontWeight = FontWeight.Normal)
            }
        },
        confirmButton = {
            TextButton(onClick = { clipboard.setText(androidx.compose.ui.text.AnnotatedString(trace)) }) { Text("Copy") }
        },
        dismissButton = { TextButton(onClick = onClose) { Text("Dismiss") } },
    )
}

private fun signOut() {
    SessionStore.clear()
    AppState.token = null; AppState.userId = null; AppState.tenantId = null
    AppState.deviceBindingKey = null; AppState.coordinatorName = null; AppState.coordinatorTitle = null
    AppState.coordinatorRegNo = null; AppState.coordinatorEmail = null; AppState.role = null
    AppState.manifest = null; AppState.cohortLabel = null
}
