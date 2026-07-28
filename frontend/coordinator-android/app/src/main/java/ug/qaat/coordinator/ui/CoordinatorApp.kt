package ug.qaat.coordinator.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AccountCircle
import androidx.compose.material3.*
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Popup
import androidx.compose.ui.window.PopupProperties
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import kotlinx.coroutines.CancellationException
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

/**
 * Silently re-login with the saved credentials and swap in a fresh token — so an EXPIRED session
 * (restored on launch) self-heals instead of leaving the app "logged in" with a dead token that
 * 401s every call ("No manifest — sign in while online"). Returns the new token, or null if we
 * can't (no saved creds, MFA required, or offline). @see SessionStore.credentials.
 */
private suspend fun refreshTokenSilently(): String? {
    val (identifier, pw, org) = SessionStore.appCredentials() ?: return null
    return runCatching {
        val res = AuthClient().appLogin(identifier, pw, org, null) { } ?: return@runCatching null   // MFA → can't do silently
        AppState.token = res.token; AppState.userId = res.userId; AppState.tenantId = res.tenantId
        AppState.deviceBindingKey = res.deviceBindingKey
        SessionStore.saveSession(res.token, res.userId, res.tenantId, res.deviceBindingKey, res.fullName,
            identifier, res.role, res.title, res.registrationNo, res.studentId, res.staffId, res.org.ifBlank { org })
        res.token
    }.getOrNull()
}

/** Turn a manifest-fetch failure into a plain-language reason for the coordinator, so a
 *  failed ⟳ says WHY instead of silently leaving "schedule hasn't loaded". */
private fun describeManifestFailure(t: Throwable?): String {
    val msg = t?.message.orEmpty()
    val cls = t?.let { it::class.java.simpleName }.orEmpty()
    return when {
        // Our own require(): "manifest fetch failed: <code>".
        "401" in msg -> "Your session has expired and couldn't be renewed. Sign out and sign in again while on internet."
        "403" in msg -> "This account isn't a coordinator on the server, or has no access to today's schedule."
        "500" in msg -> "The server couldn't build today's schedule (no active academic year set for your institution)."
        Regex("fetch failed: 5\\d\\d").containsMatchIn(msg) -> "The server is waking up or is temporarily down — wait ~30s and tap ⟳ again."
        // Ktor / JVM network exceptions (no imports needed — match by name).
        "UnresolvedAddress" in cls || "UnknownHost" in cls -> "No internet. You must be on a network that reaches the cloud (NOT the room hotspot) to load the schedule."
        "Timeout" in cls || "ConnectException" in cls || "Socket" in cls ->
            "Couldn't reach the server (timed out). Check internet and retry; the cloud may be starting up."
        t == null -> "Couldn't renew your session automatically. Sign out and sign in again while online."
        else -> "Couldn't load the schedule: ${msg.ifBlank { cls.ifBlank { "unknown error" } }}"
    }
}

/** Pull current data (branding + manifest + cohort) + upload pending attendance. Bumps
 *  refreshTick so screens reload. Driven by the top-nav ⟳ icon and on every open. */
private suspend fun refreshAll() {
    var t = AppState.token ?: return
    AppState.refreshing = true
    val dao = Graph.db.dao()
    // The manifest is the critical one. If it fails (most often an EXPIRED token → 401), try a
    // silent re-login once and retry, so the app recovers on its own instead of showing "No manifest".
    var lastErr: Throwable? = null
    val manifest = runCatching { ManifestClient(dao).fetchAndStore(t) }.getOrElse { first ->
        if (first is CancellationException) throw first          // normal cancellation — not a fetch failure
        val fresh = refreshTokenSilently()
        if (fresh != null) {
            t = fresh
            runCatching { ManifestClient(dao).fetchAndStore(t) }.getOrElse { retry ->
                if (retry is CancellationException) throw retry
                lastErr = retry; null
            }
        } else { lastErr = first; null }
    }
    if (manifest != null) {
        AppState.manifest = manifest; SessionStore.saveManifest(manifest); AppState.manifestError = null
    } else {
        AppState.manifestError = describeManifestFailure(lastErr)
    }
    runCatching { BrandingClient(t).fetch()?.let { AppState.branding = it; runCatching { SessionStore.saveBranding(it) } } }
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

/** Root: branded theme → gate on login → route by ROLE (coordinator / student / lecturer).
 *  ONE app "KIU QAAT" — the token's role decides which experience the user sees. */
@Composable
fun RootApp() = MaterialTheme(colorScheme = brandedColorScheme(AppState.branding, AppState.darkTheme)) {
    AppState.lastCrash?.let { trace -> CrashReportDialog(trace) { AppState.lastCrash = null } }
    if (!AppState.loggedIn) { LoginScreen(onLoggedIn = {}); return@MaterialTheme }
    // First sign-in with the seeded default password → force a private one before ANY role UI.
    if (AppState.forcePasswordChange) {
        Surface(Modifier.fillMaxSize()) {
            Box(Modifier.fillMaxSize().padding(24.dp), contentAlignment = Alignment.Center) {
                Text("Securing your account…", color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
        ChangePasswordDialog(onClose = { AppState.forcePasswordChange = false }, mandatory = true)
        return@MaterialTheme
    }
    when (AppState.role) {
        "STUDENT" -> StudentRoleApp()
        "LECTURER" -> LecturerApp()
        else -> CoordinatorApp()          // COORDINATOR + admin roles → the in-room hub
    }
}

/** Coordinator hub — the bottom-nav scaffold. Assumes a signed-in COORDINATOR. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CoordinatorApp() {
    // The in-room hotspot needs location / nearby-wifi — requested ONLY here (coordinator UI), so a
    // student/lecturer is never prompted for them. Fires when a coordinator first opens the hub.
    val hotspotPerms = rememberLauncherForActivityResult(
        androidx.activity.result.contract.ActivityResultContracts.RequestMultiplePermissions()) { }
    LaunchedEffect(Unit) {
        val ask = buildList {
            add(android.Manifest.permission.ACCESS_FINE_LOCATION)
            if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.TIRAMISU)
                add(android.Manifest.permission.NEARBY_WIFI_DEVICES)
        }.filter { androidx.core.content.ContextCompat.checkSelfPermission(Graph.appContext, it) != android.content.pm.PackageManager.PERMISSION_GRANTED }
        if (ask.isNotEmpty()) runCatching { hotspotPerms.launch(ask.toTypedArray()) }
    }

    // Keyed on loggedIn (a stable Boolean), NOT the token value — a silent token refresh
    // inside refreshAll() must not cancel-and-restart this effect ("coroutine scope left
    // the composition"). It fires once when the session appears, then refreshAll owns retries.
    LaunchedEffect(AppState.loggedIn) {
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

/** Change-password dialog. In [mandatory] mode (first sign-in with the seeded default password)
 *  it can't be dismissed — the user must set a private password before using the app. */
@Composable
internal fun ChangePasswordDialog(onClose: () -> Unit, mandatory: Boolean = false) {
    var cur by remember { mutableStateOf(if (mandatory) "Student" else "") }
    var next by remember { mutableStateOf("") }
    var confirm by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var err by remember { mutableStateOf<String?>(null) }
    var done by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    AlertDialog(
        onDismissRequest = { if (!mandatory) onClose() },   // mandatory → can't tap-away
        title = { Text(if (mandatory) "Set your password" else "Change password") },
        text = {
            if (done) Text("✓ Password changed.", color = MaterialTheme.colorScheme.primary)
            else Column {
                if (mandatory) Text("For your security, replace the default password before continuing.",
                    style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                err?.let { Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall) }
                PasswordField(cur, { cur = it }, if (mandatory) "Current (default) password" else "Current password", modifier = Modifier.fillMaxWidth())
                Spacer(Modifier.padding(4.dp))
                PasswordField(next, { next = it }, "New password (min 8)", modifier = Modifier.fillMaxWidth())
                Spacer(Modifier.padding(4.dp))
                PasswordField(confirm, { confirm = it }, "Confirm new password", modifier = Modifier.fillMaxWidth())
            }
        },
        confirmButton = {
            if (done) TextButton(onClick = onClose) { Text("Continue") }
            else TextButton(enabled = !busy && cur.isNotBlank() && next.length >= 8 && next == confirm, onClick = {
                busy = true; err = null
                scope.launch {
                    err = AppState.token?.let { AuthClient().changePassword(it, cur, next) } ?: "Not signed in"
                    if (err == null) { done = true; AppState.forcePasswordChange = false }
                    busy = false
                }
            }) { Text(if (busy) "Saving…" else "Update") }
        },
        dismissButton = { if (!done && !mandatory) TextButton(onClick = onClose) { Text("Cancel") } },
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

internal fun signOut() {
    SessionStore.clear()
    AppState.token = null; AppState.userId = null; AppState.tenantId = null
    AppState.deviceBindingKey = null; AppState.coordinatorName = null; AppState.coordinatorTitle = null
    AppState.coordinatorRegNo = null; AppState.coordinatorEmail = null; AppState.role = null
    AppState.manifest = null; AppState.cohortLabel = null
    AppState.studentId = null; AppState.staffId = null; AppState.org = null; AppState.attendBlockUntil = 0L
}
