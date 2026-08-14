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
import androidx.compose.ui.graphics.vector.ImageVector
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

private data class Tab(val route: String, val label: String, val icon: ImageVector)

private val tabs = listOf(
    Tab("dashboard", "Home", NavIcons.Home),
    Tab("session", "Attendance", NavIcons.Session),
    Tab("absentees", "Absentees", NavIcons.Absentees),
    Tab("trends", "Trends", NavIcons.Trends),
    Tab("alerts", "Alerts", NavIcons.Alerts),
    Tab("audit", "Sync", NavIcons.Sync),
)

/**
 * Silently re-login with the saved credentials and swap in a fresh token — so an EXPIRED session
 * (restored on launch) self-heals instead of leaving the app "logged in" with a dead token that
 * 401s every call ("No manifest — sign in while online"). Returns the new token, or null if we
 * can't (no saved creds, MFA required, or offline). @see SessionStore.credentials.
 */
private suspend fun refreshTokenSilently(): String? {
    val (identifier, pw, _) = SessionStore.appCredentials() ?: return null
    return runCatching {
        val res = AuthClient().appLogin(identifier, pw, null) { } ?: return@runCatching null   // MFA → can't do silently
        AppState.token = res.token; AppState.userId = res.userId; AppState.tenantId = res.tenantId
        AppState.deviceBindingKey = res.deviceBindingKey
        SessionStore.saveSession(res.token, res.userId, res.tenantId, res.deviceBindingKey, res.fullName,
            identifier, res.role, res.title, res.registrationNo, res.studentId, res.staffId, res.org.ifBlank { AppState.org.orEmpty() })
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

/**
 * Root: branded theme → gate on login → route by ROLE. ONE app "KIU QAAT" — the token's role
 * decides which experience the user sees, and an unrecognised role gets none of them.
 *
 * The final branch matters. It used to be `else -> CoordinatorApp()`, which meant ANY role the app
 * did not recognise was handed the coordinator's in-room hub: a QA monitor, a dean, a QA
 * department rep signing in got the screen that opens sessions, runs the class hotspot and holds
 * the roster. Their token was correctly scoped so the server refused the calls — but the app put
 * the controls in front of them and let them try. Unknown now means no role UI at all, rather than
 * the most powerful one.
 */
@Composable
fun RootApp() = MaterialTheme(colorScheme = brandedColorScheme(AppState.branding, AppState.darkTheme)) {
    AppState.lastCrash?.let { trace -> CrashReportDialog(trace) { AppState.lastCrash = null } }
    // One full-screen Box so the faint institution-logo watermark sits on EVERY page (login,
    // the mandatory password change, and all three role experiences).
    Box(Modifier.fillMaxSize()) {
        when {
            !AppState.loggedIn -> LoginScreen(onLoggedIn = {})
            // First sign-in with the seeded default password → force a private one before ANY role UI.
            AppState.forcePasswordChange -> {
                Surface(Modifier.fillMaxSize()) {
                    Box(Modifier.fillMaxSize().padding(24.dp), contentAlignment = Alignment.Center) {
                        // A way OUT of this screen. It is the only one with no navigation of its
                        // own, so a user who cannot complete the change — offline, or signed in as
                        // the wrong person — was stuck here with nothing but the app switcher, and
                        // the next launch auto-restored the same session straight back into it.
                        Column(horizontalAlignment = Alignment.CenterHorizontally) {
                            Text("Securing your account…", color = MaterialTheme.colorScheme.onSurfaceVariant)
                            Spacer(Modifier.height(20.dp))
                            SignOutButton(modifier = Modifier, label = "Sign out instead")
                        }
                    }
                }
                ChangePasswordDialog(onClose = { AppState.forcePasswordChange = false }, mandatory = true)
            }
            AppState.role == "STUDENT" -> StudentRoleApp()
            AppState.role == "LECTURER" -> LecturerApp()
            AppState.role == "QA_PATROLLER" -> PatrolRoleApp()
            AppState.role == "COORDINATOR" -> CoordinatorApp()   // the in-room hub
            else -> NoPhoneUiScreen(AppState.role)               // web-dashboard roles: no phone UI
        }
        BrandWatermark(AppState.branding)   // faint, non-interactive; drawn over every screen
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
    var showPortal by remember { mutableStateOf(false) }
    if (showChangePw) ChangePasswordDialog(onClose = { showChangePw = false })
    // The portal takes over the whole screen, with its own back button.
    if (showPortal) {
        StudentPortalScreen(regNo = AppState.coordinatorRegNo, onClose = { showPortal = false })
        return
    }

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
                        BarIcon(NavIcons.Sync, if (AppState.refreshing) "Refreshing" else "Refresh",
                            onNav ?: MaterialTheme.colorScheme.primary)
                    }
                    Box {
                        IconButton(onClick = { showProfile = !showProfile }) {
                            BarIcon(NavIcons.Account, "Profile", onNav ?: Color.White)
                        }
                        if (showProfile) ProfilePopup(
                            onClose = { showProfile = false },
                            onChangePw = { showProfile = false; showChangePw = true },
                            onOpenPortal = { showProfile = false; showPortal = true },
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
                        icon = { TabGlyph(t.icon, t.label) }, label = { Text(t.label) }, colors = itemColors,
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
            composable("alerts") { CoordinatorAlertsScreen() }
            composable("audit") { SyncAuditScreen() }
        }
    }
}

/** Corner popup: the coordinator's identity + cohort + settings (theme, change password,
 *  sign out), with an × to close. Everything that used to crowd the top bar lives here. */
@Composable
private fun ProfilePopup(onClose: () -> Unit, onChangePw: () -> Unit, onOpenPortal: () -> Unit) {
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
                // The same student portal the students get, opened in-app with the coordinator's
                // own registration number copied ready to paste (the portal is an external site,
                // so it cannot be filled programmatically — see StudentPortalScreen).
                Spacer(Modifier.height(8.dp))
                Button(onClick = onOpenPortal, modifier = Modifier.fillMaxWidth()) { Text("🎓  Student portal") }
                HorizontalDivider(Modifier.padding(vertical = 10.dp))
                TextButton(onClick = { AppState.darkTheme = !AppState.darkTheme; SessionStore.saveTheme(AppState.darkTheme) }, modifier = Modifier.fillMaxWidth()) {
                    Text((if (AppState.darkTheme) "☀  Light mode" else "☾  Dark mode"), modifier = Modifier.fillMaxWidth())
                }
                TextButton(onClick = onChangePw, modifier = Modifier.fillMaxWidth()) { Text("🔑  Change password", modifier = Modifier.fillMaxWidth()) }
                // The shared control, not a bare callback: it is what refuses to sign out of an
                // open session and warns about attendance still waiting to upload. Rendered INSIDE
                // the popup so its dialogs have somewhere to live — closing the popup first would
                // dispose them mid-decision.
                SignOutButton(label = "⎋  Sign out")
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

/** Coordinator notifications: read messages (e.g. from a lecturer) and send to his cohort's
 *  students (all or one) or his course's lecturers (all or one). */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun CoordinatorAlertsScreen() {
    val scope = rememberCoroutineScope()
    var inbox by remember { mutableStateOf<List<ug.qaat.coordinator.net.NotificationClient.Notif>?>(null) }
    var composing by remember { mutableStateOf(false) }
    fun load() { scope.launch { inbox = ug.qaat.coordinator.net.NotificationClient().inbox() } }
    LaunchedEffect(Unit) { load() }
    Column(Modifier.fillMaxSize().padding(16.dp)) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            Text("Notifications", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold, modifier = Modifier.weight(1f))
            Button(onClick = { composing = !composing }) { Text(if (composing) "Close" else "✎ New") }
        }
        if (composing) CoordinatorComposer(onSent = { composing = false; load() })
        Spacer(Modifier.height(8.dp))
        NotificationInboxList(inbox) { load() }
    }
}

/** Coordinator composer with a recipient picker: all students / a specific student / all
 *  lecturers / a specific lecturer. White card background. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun CoordinatorComposer(onSent: () -> Unit) {
    val scope = rememberCoroutineScope()
    var audience by remember { mutableStateOf("STUDENTS") }   // STUDENTS | STUDENT | LECTURERS | LECTURER
    var targetId by remember { mutableStateOf<String?>(null) }
    var targetName by remember { mutableStateOf("") }
    var subject by remember { mutableStateOf("") }
    var body by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var err by remember { mutableStateOf<String?>(null) }
    var students by remember { mutableStateOf<List<ug.qaat.coordinator.net.DashboardClient.Student>>(emptyList()) }
    var lecturers by remember { mutableStateOf<List<ug.qaat.coordinator.net.DashboardClient.Lecturer>>(emptyList()) }
    LaunchedEffect(Unit) {
        val t = AppState.token ?: return@LaunchedEffect
        runCatching { students = ug.qaat.coordinator.net.DashboardClient().students(t) }
        runCatching { lecturers = ug.qaat.coordinator.net.DashboardClient().lecturers(t) }
    }

    Surface(color = MaterialTheme.colorScheme.surface, shape = MaterialTheme.shapes.medium, tonalElevation = 1.dp,
        border = androidx.compose.foundation.BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant),
        modifier = Modifier.fillMaxWidth().padding(vertical = 8.dp)) {
        Column(Modifier.padding(12.dp)) {
            Text("Send to:", style = MaterialTheme.typography.labelMedium)
            Column {
                Row {
                    FilterChip(audience == "STUDENTS", { audience = "STUDENTS"; targetId = null }, { Text("All students") }, modifier = Modifier.padding(end = 6.dp))
                    FilterChip(audience == "STUDENT", { audience = "STUDENT"; targetId = null; targetName = "" }, { Text("A student") })
                }
                Row(Modifier.padding(top = 4.dp)) {
                    FilterChip(audience == "LECTURERS", { audience = "LECTURERS"; targetId = null }, { Text("All lecturers") }, modifier = Modifier.padding(end = 6.dp))
                    FilterChip(audience == "LECTURER", { audience = "LECTURER"; targetId = null; targetName = "" }, { Text("A lecturer") })
                }
            }
            if (audience == "STUDENT" || audience == "LECTURER") {
                var open by remember { mutableStateOf(false) }
                Box {
                    OutlinedButton(onClick = { open = true }, modifier = Modifier.fillMaxWidth().padding(top = 6.dp)) {
                        Text(if (targetName.isBlank()) "Pick a ${if (audience == "STUDENT") "student" else "lecturer"}" else targetName)
                    }
                    DropdownMenu(expanded = open, onDismissRequest = { open = false }) {
                        if (audience == "STUDENT") students.forEach { s ->
                            DropdownMenuItem(text = { Text("${s.fullName} · ${s.studentId}") }, onClick = { targetId = s.studentId; targetName = s.fullName; open = false })
                        } else lecturers.forEach { l ->
                            DropdownMenuItem(text = { Text(l.fullName) }, onClick = { targetId = l.lecturerId; targetName = l.fullName; open = false })
                        }
                    }
                }
            }
            err?.let { Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.labelSmall) }
            OutlinedTextField(subject, { subject = it }, label = { Text("Subject") }, singleLine = true, modifier = Modifier.fillMaxWidth().padding(top = 6.dp))
            Spacer(Modifier.height(6.dp))
            OutlinedTextField(body, { body = it }, label = { Text("Message") }, modifier = Modifier.fillMaxWidth().heightIn(min = 80.dp))
            Button(enabled = !busy && subject.isNotBlank() && (audience == "STUDENTS" || audience == "LECTURERS" || targetId != null),
                modifier = Modifier.padding(top = 8.dp), onClick = {
                    busy = true; err = null
                    scope.launch {
                        err = ug.qaat.coordinator.net.NotificationClient().send(audience, null, subject.trim(), body, targetId)
                        busy = false; if (err == null) onSent()
                    }
                }) { Text(if (busy) "Sending…" else "Send") }
        }
    }
}

/**
 * The seeded first-login password for a role that is created FOR someone rather than BY them.
 * The word is always the role itself, and it mirrors the server's own DefaultPasswordFor
 * (backend/api-gateway/internal/handlers/default_passwords.go). Roles whose password their
 * creator chose have no default, so nothing is pre-filled and they type what they were given.
 */
private fun defaultPasswordForRole(role: String?): String = when (role) {
    "STUDENT" -> "student"
    "LECTURER" -> "lecturer"
    "QA_PATROLLER" -> "monitor"
    else -> ""
}

/**
 * The thing the user typed into the sign-in screen — a staff email, a registration number or a
 * staff id. Needed to sign them back in with their NEW password the moment they set one.
 *
 * Two sources because a QA monitor has only the second: their saved credentials are erased the
 * instant their role is known (a lost handset must not resume a monitor round), so by the time they
 * change a password from the profile there is nothing under `cred_id`. The session's own record of
 * who signed in survives that erasure, because it is not a password.
 */
private fun signInIdentifier(): String? =
    SessionStore.appCredentials()?.first?.takeIf { it.isNotBlank() }
        ?: AppState.coordinatorEmail?.takeIf { it.isNotBlank() }

/**
 * Adopt a fresh sign-in as THE session: memory, then disk, so a close-and-reopen resumes on the
 * new token rather than the one it replaced.
 *
 * [keepCredentials] is false for anyone whose saved password was deliberately erased — writing it
 * back here would quietly undo the monitor rule that a handset can never walk back into a round.
 */
private fun adoptSession(res: AuthClient.Result, identifier: String, password: String, keepCredentials: Boolean) {
    AppState.token = res.token
    AppState.userId = res.userId
    AppState.tenantId = res.tenantId
    AppState.deviceBindingKey = res.deviceBindingKey
    AppState.coordinatorName = res.fullName
    AppState.coordinatorEmail = identifier
    AppState.role = res.role
    AppState.coordinatorTitle = res.title
    AppState.coordinatorRegNo = res.registrationNo
    AppState.studentId = res.studentId.ifBlank { null }
    AppState.staffId = res.staffId.ifBlank { null }
    SessionStore.saveSession(
        res.token, res.userId, res.tenantId, res.deviceBindingKey,
        res.fullName, identifier, res.role, res.title, res.registrationNo,
        res.studentId, res.staffId, "",
    )
    if (keepCredentials) SessionStore.saveAppCredentials(identifier, password, "")
}

/** Change-password dialog. In [mandatory] mode (first sign-in with the seeded default password)
 *  it can't be dismissed — the user must set a private password before using the app. */
@Composable
internal fun ChangePasswordDialog(onClose: () -> Unit, mandatory: Boolean = false) {
    // Pre-fill the CURRENT field with this ROLE's seeded default, not with the word "Student".
    //
    // It was hardcoded to "Student" for everyone. A lecturer or a monitor was therefore shown
    // somebody else's password, tapped "Save & proceed", and was told their existing password was
    // wrong — on the one screen with no way forward and no way around. Even a student was shown
    // the wrong casing of their own. The defaults are public (they are the role's own name), so
    // filling the right one in costs nothing and turns first sign-in into: type a new password
    // twice, proceed.
    var cur by remember { mutableStateOf(if (mandatory) defaultPasswordForRole(AppState.role) else "") }
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
                    // EVERYTHING in a runCatching. changePassword and appLogin both THROW on a
                    // dead network — they do not return a message — and this coroutine has no
                    // parent to catch that, so a first sign-in on a weak connection took the whole
                    // app down on the one screen with no way back.
                    err = runCatching {
                        val identifier = signInIdentifier()
                        val keepCredentials = SessionStore.appCredentials() != null

                        // "Not signed in" was a dead end, and it was reachable: the token can
                        // lapse while this dialog sits open. Recover instead of reporting — the
                        // password in the CURRENT field is, by definition, one that works.
                        var token = AppState.token
                        if (token == null && identifier != null) {
                            token = AuthClient().appLogin(identifier, cur, null) {}?.also {
                                adoptSession(it, identifier, cur, keepCredentials)
                            }?.token
                        }
                        if (token == null) {
                            return@runCatching "Your session has ended. Sign in again to set your password."
                        }

                        val failure = AuthClient().changePassword(token, cur, next)
                        if (failure != null) return@runCatching failure

                        // SIGN IN AGAIN, with the password they just chose.
                        //
                        // The change leaves the app holding a token minted for the old one and
                        // saved credentials that no longer open anything. Both keep working right
                        // up until the token lapses — and then the silent renewal presents a dead
                        // password and the person is dropped at the sign-in screen, or every panel
                        // that needs a token reports "Not signed in", moments after they were told
                        // their account was secured. A fresh sign-in here makes the session
                        // unambiguously good before any role UI asks anything of it, and it is
                        // also what clears force_password_change on the server's copy.
                        //
                        // Best effort, never a barrier: the change itself has already succeeded,
                        // and the old token stays valid (the server does not revoke it), so if
                        // this cannot complete — offline, rate-limited, an MFA prompt we have
                        // nowhere to show — the user still proceeds on the session they have.
                        if (identifier != null) {
                            runCatching {
                                AuthClient().appLogin(identifier, next, null) {}
                                    ?.let { adoptSession(it, identifier, next, keepCredentials) }
                            }.onFailure {
                                // Keep the working token; just make sure the saved password is the
                                // real one so the NEXT renewal succeeds even though this did not.
                                if (keepCredentials) SessionStore.saveAppCredentials(identifier, next, "")
                            }
                        }
                        null
                    }.getOrElse { e ->
                        if (e is CancellationException) throw e
                        ug.qaat.coordinator.net.Net.friendly(e)
                    }

                    if (err == null) {
                        AppState.forcePasswordChange = false
                        // Mandatory (temp-password) flow: save AND go straight into the dashboard.
                        if (mandatory) onClose() else done = true
                    }
                    busy = false
                }
            }) { Text(if (busy) "Saving…" else if (mandatory) "Save & proceed" else "Update") }
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

/**
 * Shown to the oversight, org and administrator roles, which do their work on the web dashboards.
 * They can sign in — the account is real — but there is nothing here for them, and saying so is
 * the honest answer. It is also the safe one: the alternative was showing them the hub.
 */
@Composable
internal fun NoPhoneUiScreen(role: String?) {
    Surface(Modifier.fillMaxSize()) {
        Box(Modifier.fillMaxSize().padding(28.dp), contentAlignment = Alignment.Center) {
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                BrandLogo(AppState.branding, size = 72)
                Spacer(Modifier.height(16.dp))
                Text("Signed in as ${role?.replace('_', ' ')?.lowercase() ?: "an unrecognised role"}",
                    style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold,
                    textAlign = androidx.compose.ui.text.style.TextAlign.Center)
                Spacer(Modifier.height(10.dp))
                Text("This role works on the QAAT web dashboard, not on the phone app. Open the dashboard in your browser and sign in with the same details.",
                    textAlign = androidx.compose.ui.text.style.TextAlign.Center,
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
                Spacer(Modifier.height(28.dp))
                SignOutButton()
            }
        }
    }
}

// signOut() used to live here: a bare "null out some AppState and clear prefs" that each role
// screen called directly. It left the foreground service (and with it the hotspot and the in-room
// HTTP server) running, left the cached cohort roster and check-ins in Room for whoever signed in
// next, and silently stranded any attendance not yet uploaded. The full teardown, the checks that
// guard it and the one shared button are now in SignOut.kt — use SignOutButton().
