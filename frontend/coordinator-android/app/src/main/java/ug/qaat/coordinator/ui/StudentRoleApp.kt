package ug.qaat.coordinator.ui

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import ug.qaat.coordinator.net.AuthClient
import ug.qaat.coordinator.net.NotificationClient
import ug.qaat.coordinator.net.StudentHomeClient
import ug.qaat.coordinator.store.SessionStore
import ug.qaat.coordinator.student.CheckinClient
import ug.qaat.coordinator.student.Discovery
import ug.qaat.coordinator.student.OnlineCheckinClient
import ug.qaat.coordinator.student.Fingerprint
import ug.qaat.coordinator.student.ProgressClient
import ug.qaat.coordinator.student.RegisterDeviceClient

private val REASONS = mapOf(
    "NOT_ON_ROSTER" to "You're not on this class's roster.",
    "DUPLICATE_SCAN" to "You're already marked present.",
    "DEVICE_ALREADY_USED" to "This phone already checked in another student for this lecture.",
    "DEVICE_MISMATCH" to "This isn't the device you first checked in with.",
    "DEVICE_BELONGS_TO_ANOTHER_STUDENT" to "This phone is registered to another student.",
    "SESSION_NOT_ACTIVE" to "No active session yet — wait for your coordinator to start.",
    "LECTURER_NOT_STARTED" to "Waiting for the lecturer to start the session.",
    // Only ~10 phones fit on the class Wi-Fi at once, so a turn that goes nowhere is given up
    // rather than held. Phrased as the queue it is, not as a failure the student caused.
    "EVICT_EXPIRED" to "Your turn on the class Wi-Fi ended. Reconnecting shortly…",
)

/** How long to wait before taking another turn after being moved on without attending.
 *  RANDOMISED, and that is the point: two hundred phones evicted within the same second would
 *  otherwise all come back in the same second, and the hall would thrash instead of drain. */
private val REJOIN_BACKOFF_MS: Long get() = 20_000L + (Math.random() * 20_000L).toLong()

/** The STUDENT experience inside the unified app: one-tap Attend + online My-attendance + Profile.
 *  Identity (reg-number, institution) comes from the login token via AppState. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun StudentRoleApp() {
    val ctx = LocalContext.current
    // Bind this phone to the student (one-device-one-student) once after login; capture any cooldown.
    LaunchedEffect(AppState.studentId) {
        val reg = AppState.studentId
        if (!reg.isNullOrBlank()) {
            runCatching { RegisterDeviceClient().register(reg, Fingerprint.get(ctx)) }
                .onSuccess { AppState.attendBlockUntil = it.attendBlockUntilMs; SessionStore.saveAttendBlockUntil(it.attendBlockUntilMs) }
        }
        if (AppState.attendBlockUntil == 0L) AppState.attendBlockUntil = SessionStore.attendBlockUntil()
    }

    val navColor = navBarColor(AppState.branding)
    val onNav = navColor?.let { onNavColor(it) }
    var tab by remember { mutableStateOf(0) }
    var showPortal by remember { mutableStateOf(false) }
    // Bumped by the top-bar refresh; every page keys its fetch on it and reloads.
    var reloadKey by remember { mutableStateOf(0) }

    // The KIU student portal takes over the whole screen (with its own back button) when opened.
    if (showPortal) {
        StudentPortalScreen(regNo = AppState.studentId, onClose = { showPortal = false })
        return
    }
    var unread by remember { mutableStateOf(0) }
    // Refresh the unread badge whenever the tab changes (so it clears after reading the inbox).
    LaunchedEffect(tab, reloadKey) { runCatching { unread = NotificationClient().unread() } }

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
                    // Refresh + light/dark toggle, matching the coordinator app's top bar.
                    IconButton(onClick = { reloadKey++ }) {
                        BarIcon(NavIcons.Sync, "Refresh", onNav ?: MaterialTheme.colorScheme.primary)
                    }
                    IconButton(onClick = { AppState.darkTheme = !AppState.darkTheme; SessionStore.saveTheme(AppState.darkTheme) }) {
                        BarIcon(if (AppState.darkTheme) NavIcons.LightMode else NavIcons.DarkMode,
                            if (AppState.darkTheme) "Switch to light theme" else "Switch to dark theme",
                            onNav ?: MaterialTheme.colorScheme.primary)
                    }
                },
            )
        },
        bottomBar = {
            NavigationBar(containerColor = navColor ?: MaterialTheme.colorScheme.surface) {
                val itemColors = if (onNav != null) NavigationBarItemDefaults.colors(
                    selectedIconColor = onNav, selectedTextColor = onNav,
                    unselectedIconColor = onNav.copy(alpha = .65f), unselectedTextColor = onNav.copy(alpha = .65f),
                    indicatorColor = onNav.copy(alpha = .18f),
                ) else NavigationBarItemDefaults.colors()
                NavigationBarItem(tab == 0, { tab = 0 }, icon = { TabGlyph(NavIcons.Home, "Home") }, label = { Text("Home") }, colors = itemColors)
                NavigationBarItem(tab == 1, { tab = 1 }, icon = { TabGlyph(NavIcons.Attend, "Attend") }, label = { Text("Attend") }, colors = itemColors)
                NavigationBarItem(tab == 2, { tab = 2 }, icon = { TabGlyph(NavIcons.Attendance, "Attendance") }, label = { Text("Attendance") }, colors = itemColors)
                NavigationBarItem(tab == 3, { tab = 3 }, colors = itemColors, label = { Text("Alerts") },
                    icon = { if (unread > 0) BadgedBox(badge = { Badge { Text("$unread") } }) { TabGlyph(NavIcons.Alerts, "Alerts") } else TabGlyph(NavIcons.Alerts, "Alerts") })
                NavigationBarItem(tab == 4, { tab = 4 }, icon = { TabGlyph(NavIcons.Profile, "Profile") }, label = { Text("Profile") }, colors = itemColors)
            }
        },
    ) { pad ->
        Column(Modifier.padding(pad).fillMaxSize()) {
            Box(Modifier.weight(1f)) {
                when (tab) {
                    0 -> StudentHome(reloadKey)
                    1 -> StudentAttend()
                    2 -> StudentProgress(reloadKey)
                    3 -> StudentNotifications(reloadKey, onRead = { runCatching { } })
                    else -> StudentProfile(reloadKey, onOpenPortal = { showPortal = true })
                }
            }
        }
    }
}

/** The student's notification inbox — messages from their lecturers and coordinator. Tapping a
 *  message marks it read. Fully cloud-backed (fetched when the phone is online). */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun StudentNotifications(reloadKey: Int, onRead: () -> Unit) {
    val scope = rememberCoroutineScope()
    var items by remember { mutableStateOf<List<NotificationClient.Notif>?>(null) }
    fun load() { scope.launch { items = NotificationClient().inbox() } }
    LaunchedEffect(reloadKey) { load() }

    Column(Modifier.fillMaxSize().padding(16.dp)) {
        Text("Notifications", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(8.dp))
        // The shared list, not a private copy of it. The copy that lived here dropped the card on
        // its own authority and never checked whether the server accepted the dismissal, so a
        // refused ✕ came back with no explanation.
        NotificationInboxList(items) { load(); onRead() }
    }
}

@Composable
private fun StudentAttend() {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var baseUrl by remember { mutableStateOf<String?>(null) }
    var session by remember { mutableStateOf<CheckinClient.Session?>(null) }
    var searching by remember { mutableStateOf(true) }
    var status by remember { mutableStateOf<String?>(null) }
    var success by remember { mutableStateOf(false) }
    var busy by remember { mutableStateOf(false) }
    // True once this device has attended the CURRENT session (persisted for the day). Drives the
    // greyed, disabled ATTEND button + the "already attended, turn Wi-Fi off" message.
    var alreadyAttended by remember { mutableStateOf(false) }
    // The student's latest notification, surfaced HERE on the attendance page right after a
    // successful check-in (so they see any message from their lecturer/coordinator immediately).
    var latestNotif by remember { mutableStateOf<NotificationClient.Notif?>(null) }
    LaunchedEffect(success) { if (success) runCatching { latestNotif = NotificationClient().inbox().firstOrNull() } }
    // A distance / e-learning class, which has no hotspot to find. Only consulted once the local
    // search has come up empty, so the hotspot remains the way attendance is taken for every
    // student who has one — this is the fallback for the students who never will.
    var online by remember { mutableStateOf<OnlineCheckinClient.LiveOnline?>(null) }
    var code by remember { mutableStateOf("") }

    // ── Holding a slot on the class Wi-Fi ────────────────────────────────────────
    // Only ~10 phones fit on the coordinator's hotspot at once, so a turn is something to take and
    // give back rather than something to keep. These three drive that: the credentials we join
    // with, and the moment we may next take a turn after being moved on.
    var wifiSsid by remember { mutableStateOf(SessionStore.classWifiSsid()) }
    var wifiPass by remember { mutableStateOf(SessionStore.classWifiPass()) }
    var waitingUntil by remember { mutableStateOf(0L) }
    var nowTick by remember { mutableStateOf(System.currentTimeMillis()) }

    /** Give the slot back. Says goodbye to the hub first — best-effort, on a connection we are
     *  about to drop — then actually lets go, which is the part that frees the slot. */
    suspend fun letGo() {
        baseUrl?.let { url -> runCatching { CheckinClient(url, ctx).leave() } }
        ug.qaat.coordinator.student.SlotLease.release(ctx)
    }

    // Leaving this screen gives the slot back. A student who checks in and switches to Home, or
    // drops the app in their pocket, must not keep a classmate off the Wi-Fi for the rest of the
    // hour — and before this, that was the single commonest way a slot was lost.
    DisposableEffect(Unit) { onDispose { ug.qaat.coordinator.student.SlotLease.release(ctx) } }

    suspend fun discover(showSpinner: Boolean = true) {
        if (showSpinner) { searching = true; status = null }
        val reg = AppState.studentId.orEmpty()

        // Take a turn on the class Wi-Fi if we do not already hold one. Skipped entirely when the
        // student is already on the network some other way (Android 9, or a Settings join), which
        // is why the old discovery path below is kept exactly as it was.
        val lease = ug.qaat.coordinator.student.SlotLease
        if (lease.supported && !lease.held && wifiSsid.isNotBlank() &&
            !ug.qaat.coordinator.student.LanNetwork.onWifi(ctx)
        ) {
            lease.acquire(ctx, wifiSsid, wifiPass)
        }

        val url = Discovery(ctx).find()
        val newSession = url?.let { CheckinClient(it, ctx).session(reg) }
        // Reset the local "attended" view when the session changes (new sessionId = next round) or
        // the hotspot dropped, so the student is prompted to reconnect / can attend the next session.
        val newId = newSession?.sessionId ?: ""
        val oldId = session?.sessionId ?: ""
        if (newId != oldId) { success = false }
        baseUrl = url
        session = newSession
        // No room found: is one of this student's classes running online? Telling a distance
        // student to "connect to their coordinator's Wi-Fi" is advice that can never be followed,
        // and it was the whole of what this screen used to say to them.
        online = if (newSession?.active == true) null
                 else runCatching { OnlineCheckinClient().liveOnlineSession() }.getOrNull()
        val liveId = newSession?.sessionId ?: online?.sessionId ?: ""
        alreadyAttended = (liveId.isNotBlank() && SessionStore.hasAttended(liveId)) ||
            online?.alreadyCheckedIn == true
        if (alreadyAttended) success = true
        searching = false

        // The hub moving us on. Two quite different things arrive by the same flag: we are finished
        // (nothing more to do, so let go and stay gone), or our turn ran out before we could finish
        // (let go, then come back after a randomised pause so the whole hall does not return at once).
        if (newSession?.evict == true || alreadyAttended) {
            letGo()
            if (newSession?.evictReason == "EVICT_EXPIRED" && !alreadyAttended) {
                waitingUntil = System.currentTimeMillis() + REJOIN_BACKOFF_MS
                status = REASONS["EVICT_EXPIRED"]
                return
            }
        }

        status = when {
            online != null -> null
            url == null && !lease.supported && !ug.qaat.coordinator.student.LanNetwork.onWifi(ctx) ->
                "You are not on any Wi-Fi. Join your coordinator's class network from Wi-Fi settings, then come back."
            url == null && wifiSsid.isBlank() -> null   // the join form below is the answer, not an error
            url == null ->
                "Couldn't reach the class server. Check the Wi-Fi name and password match what your coordinator is showing, and that they have started the session."
            newSession?.active != true -> "Connected, but no session is active yet — wait for it to start."
            else -> null
        }
    }
    LaunchedEffect(Unit) { discover() }

    // The screen keeps itself current — EXCEPT once the student is done. The old loop ran every 4
    // seconds for as long as the screen was open, including long after the ✓, which meant an
    // attended phone sat on a Wi-Fi slot AND kept the hub busy answering it. Stopping is now part
    // of the design rather than something the student is asked to do.
    LaunchedEffect(success, alreadyAttended) {
        if (success || alreadyAttended) return@LaunchedEffect
        while (true) {
            kotlinx.coroutines.delay(4000)
            nowTick = System.currentTimeMillis()
            if (busy || searching) continue
            if (nowTick < waitingUntil) continue          // serving out a backoff; stay off the air
            runCatching { discover(showSpinner = false) }
        }
    }

    Column(Modifier.fillMaxSize().padding(24.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        Spacer(Modifier.weight(1f))

        // Device-switch cooldown only applies when the anti-cheat is enabled (off during testing).
        val blocked = AppState.ENFORCE_DEVICE_LOCK && System.currentTimeMillis() < AppState.attendBlockUntil
        when {
            searching -> { CircularProgressIndicator(); Text("Finding your coordinator…", Modifier.padding(top = 12.dp)) }
            success -> {
                Text("✓", style = MaterialTheme.typography.displayMedium, color = MaterialTheme.colorScheme.primary)
                Text(if (alreadyAttended) "You already attended" else "You're marked present",
                    style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold, textAlign = TextAlign.Center)
                session?.let { Text("${it.unitName} · ${it.cohort}", textAlign = TextAlign.Center, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                Spacer(Modifier.height(12.dp))
                // Greyed, disabled button reinforces that they're done and must free the Wi-Fi slot.
                Button(enabled = false, modifier = Modifier.fillMaxWidth().height(64.dp), onClick = {}) {
                    Text("ATTENDED ✓", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
                }
                Spacer(Modifier.height(12.dp))
                // Was a red box asking the student to turn Wi-Fi off for the sake of the queue —
                // a favour requested of the one person in the room with nothing left to gain by
                // doing it. The app now hands the slot back itself, so this states what happened
                // instead of asking for anything.
                Surface(color = MaterialTheme.colorScheme.primaryContainer, shape = MaterialTheme.shapes.medium, modifier = Modifier.fillMaxWidth()) {
                    Text("✓ Disconnected from the class Wi-Fi — your slot is free for a classmate.",
                        Modifier.padding(14.dp), textAlign = TextAlign.Center,
                        color = MaterialTheme.colorScheme.onPrimaryContainer)
                }
                // Latest notification, shown right after checking in.
                latestNotif?.let { n ->
                    Spacer(Modifier.height(12.dp))
                    Surface(color = MaterialTheme.colorScheme.secondaryContainer, shape = MaterialTheme.shapes.medium, modifier = Modifier.fillMaxWidth()) {
                        Column(Modifier.padding(12.dp)) {
                            Text("🔔 ${n.subject}", fontWeight = FontWeight.Bold, color = MaterialTheme.colorScheme.onSecondaryContainer)
                            Text("from ${n.senderName}", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSecondaryContainer)
                            if (n.body.isNotBlank()) { Spacer(Modifier.height(4.dp)); Text(n.body, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSecondaryContainer) }
                        }
                    }
                }
            }
            // ── Waiting for a free slot ──────────────────────────────────────────────────
            // Our turn ended before the lecturer started, or before we could finish. The slot is
            // already back with the hall; we are counting down to taking another one. Showing the
            // countdown matters: a screen that simply said "waiting" would have every student in
            // the room tapping Retry, which is exactly the stampede the backoff exists to prevent.
            nowTick < waitingUntil -> {
                CircularProgressIndicator()
                Spacer(Modifier.height(12.dp))
                Text("Waiting for a free slot", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
                Text(
                    "Only about ten phones fit on the class Wi-Fi at once, so everyone takes a short " +
                        "turn. Yours comes back in ${((waitingUntil - nowTick) / 1000).coerceAtLeast(0)}s — " +
                        "leave this screen open.",
                    textAlign = TextAlign.Center, style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(top = 8.dp),
                )
            }
            // ── The distance / e-learning class ──────────────────────────────────────────
            online != null && !success -> {
                Text("Online class", color = MaterialTheme.colorScheme.onSurfaceVariant)
                Text(online!!.unitName, style = MaterialTheme.typography.headlineSmall,
                    fontWeight = FontWeight.Bold, textAlign = TextAlign.Center)
                Spacer(Modifier.height(6.dp))
                Text(
                    "Type the 6-digit code your lecturer is showing. It changes every 10 seconds, " +
                        "so read it and enter it straight away.",
                    textAlign = TextAlign.Center,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                Spacer(Modifier.height(16.dp))
                OutlinedTextField(
                    value = code,
                    onValueChange = { v -> code = v.filter { it.isDigit() }.take(6) },
                    label = { Text("Code on screen") },
                    singleLine = true,
                    keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(
                        keyboardType = androidx.compose.ui.text.input.KeyboardType.NumberPassword),
                    modifier = Modifier.fillMaxWidth(),
                )
                Spacer(Modifier.height(12.dp))
                Button(
                    enabled = !busy && code.length == 6,
                    modifier = Modifier.fillMaxWidth().height(64.dp),
                    onClick = {
                        busy = true; status = null
                        scope.launch {
                            val fp = Fingerprint.get(ctx)
                            val err = OnlineCheckinClient().attend(online!!.sessionId, code, fp)
                            if (err == null) {
                                success = true
                                SessionStore.markAttended(online!!.sessionId); alreadyAttended = true
                            } else {
                                status = err
                                // The code is cleared on a stale-code refusal only: retyping the
                                // same six digits that just expired would fail again, and a student
                                // reading a fresh code needs an empty box, not one to correct.
                                if (err.startsWith("That code has already changed")) code = ""
                            }
                            busy = false
                        }
                    },
                ) { Text(if (busy) "Marking…" else "ATTEND", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold) }
            }
            session?.active == true && blocked -> {
                val until = java.text.SimpleDateFormat("EEE d MMM, h:mm a", java.util.Locale.getDefault()).format(java.util.Date(AppState.attendBlockUntil))
                Text("⏸", style = MaterialTheme.typography.displaySmall)
                Text("Attendance paused", style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold)
                Text("You recently switched phones. You can take attendance on this device from $until.",
                    textAlign = TextAlign.Center, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(top = 8.dp))
            }
            session?.active == true -> {
                Text("Attendance for", color = MaterialTheme.colorScheme.onSurfaceVariant)
                Text(session!!.unitName.ifBlank { "this session" }, style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold, textAlign = TextAlign.Center)
                if (session!!.cohort.isNotBlank()) Text(session!!.cohort, color = MaterialTheme.colorScheme.onSurfaceVariant, textAlign = TextAlign.Center)
                Spacer(Modifier.height(24.dp))
                Button(
                    enabled = !busy,
                    modifier = Modifier.fillMaxWidth().height(64.dp),
                    onClick = {
                        busy = true; status = null
                        scope.launch {
                            val fp = Fingerprint.get(ctx)
                            runCatching { CheckinClient(baseUrl!!, ctx).attend(AppState.studentId ?: "", fp) }
                                .onSuccess { r ->
                                    if (r.present || r.alreadyPresent) {
                                        success = true
                                        // Remember THIS session so the button stays greyed on reconnect (one per session).
                                        session?.sessionId?.let { SessionStore.markAttended(it); alreadyAttended = true }
                                        // Marked present — so let the slot go NOW, without waiting to
                                        // be told to. The next student is standing there.
                                        letGo()
                                    } else {
                                        status = REASONS[r.reason] ?: "Not marked: ${r.reason ?: "try again"}"
                                        // Refused AND moved on: a turn we cannot use is a turn worth
                                        // giving back immediately.
                                        if (r.evict) {
                                            letGo()
                                            if (r.evictReason == "EVICT_EXPIRED")
                                                waitingUntil = System.currentTimeMillis() + REJOIN_BACKOFF_MS
                                        }
                                    }
                                    busy = false
                                }
                                .onFailure { status = "Couldn't reach the class server — check you're on the class Wi-Fi and that your coordinator has started the session."; busy = false }
                        }
                    },
                ) { Text(if (busy) "Marking…" else "ATTEND", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold) }
            }
            // ── We need the class Wi-Fi's name and password ──────────────────────────────
            // The app joins the network itself rather than sending the student to Wi-Fi settings,
            // which is the only way it can hand the slot BACK afterwards (Android has not let an
            // app drop its own Settings-joined Wi-Fi since version 10). The price is these two
            // fields — the same two the student used to type into Settings, read off the same
            // projected screen. Remembered, so in a class whose hotspot name does not change this
            // is typed once and never again.
            ug.qaat.coordinator.student.SlotLease.supported && baseUrl == null -> {
                Text("Join the class Wi-Fi", style = MaterialTheme.typography.headlineSmall,
                    fontWeight = FontWeight.Bold, textAlign = TextAlign.Center)
                Text(
                    "Copy the Wi-Fi name and password your coordinator is showing at the front. " +
                        "The app connects, marks you present, and disconnects on its own.",
                    textAlign = TextAlign.Center, style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(top = 6.dp, bottom = 14.dp),
                )
                OutlinedTextField(
                    value = wifiSsid, onValueChange = { wifiSsid = it }, singleLine = true,
                    label = { Text("Wi-Fi name") }, modifier = Modifier.fillMaxWidth(),
                )
                Spacer(Modifier.height(8.dp))
                OutlinedTextField(
                    value = wifiPass, onValueChange = { wifiPass = it }, singleLine = true,
                    label = { Text("Wi-Fi password") }, modifier = Modifier.fillMaxWidth(),
                    visualTransformation = androidx.compose.ui.text.input.PasswordVisualTransformation(),
                    keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(
                        keyboardType = androidx.compose.ui.text.input.KeyboardType.Password),
                )
                Spacer(Modifier.height(14.dp))
                Button(
                    enabled = !searching && wifiSsid.isNotBlank(),
                    modifier = Modifier.fillMaxWidth().height(56.dp),
                    onClick = {
                        SessionStore.saveClassWifi(wifiSsid, wifiPass)
                        scope.launch { discover() }
                    },
                ) { Text(if (searching) "Connecting…" else "CONNECT", fontWeight = FontWeight.Bold) }
                status?.let {
                    Text(it, color = MaterialTheme.colorScheme.error, textAlign = TextAlign.Center,
                        style = MaterialTheme.typography.bodySmall, modifier = Modifier.padding(top = 10.dp))
                }
            }
            else -> Text(status ?: "No active session.", textAlign = TextAlign.Center, color = MaterialTheme.colorScheme.error)
        }
        status?.takeIf { !success && !blocked && (session?.active == true || online != null) }?.let {
            Text(it, color = MaterialTheme.colorScheme.error, textAlign = TextAlign.Center, modifier = Modifier.padding(top = 12.dp))
        }
        Spacer(Modifier.weight(1f))
        OutlinedButton(onClick = { scope.launch { discover() } }, enabled = !searching, modifier = Modifier.fillMaxWidth()) {
            Text(if (success) "Done" else "Retry / refresh")
        }
    }
}

@Composable
private fun StudentProgress(reloadKey: Int) {
    val scope = rememberCoroutineScope()
    var data by remember { mutableStateOf<ProgressClient.Progress?>(null) }
    var loading by remember { mutableStateOf(true) }
    var error by remember { mutableStateOf<String?>(null) }

    fun load() {
        loading = true; error = null
        scope.launch {
            val reg = AppState.studentId
            if (reg.isNullOrBlank()) { error = "Sign in again to view your progress."; loading = false; return@launch }
            runCatching { ProgressClient().fetch(reg) }
                .onSuccess { data = it; loading = false }
                .onFailure { error = it.message ?: "Couldn't load progress"; loading = false }
        }
    }
    LaunchedEffect(reloadKey) { load() }

    Column(Modifier.fillMaxSize().padding(20.dp)) {
        Text("My attendance", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        data?.let { Text(it.fullName, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        data?.let { p ->
            val org = listOf(p.school, p.department).filter { it.isNotBlank() }.joinToString(" · ")
            if (org.isNotBlank()) Text(org, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        Spacer(Modifier.height(12.dp))
        // Same reason as the monitor round: the LazyColumn body is invoked after this
        // composition returns, so it must close over a value, not over the state holder.
        val units = data?.units
        when {
            loading -> Box(Modifier.fillMaxWidth().padding(top = 40.dp), Alignment.Center) { CircularProgressIndicator() }
            error != null -> Text(error.orEmpty(), color = MaterialTheme.colorScheme.error)
            units.isNullOrEmpty() -> Text("No attendance recorded yet.", color = MaterialTheme.colorScheme.onSurfaceVariant)
            else -> LazyColumn(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                items(units) { u ->
                    val eligible = u.status == "ELIGIBLE"
                    Surface(color = MaterialTheme.colorScheme.surfaceVariant, shape = MaterialTheme.shapes.medium) {
                        Column(Modifier.fillMaxWidth().padding(12.dp)) {
                            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                                Text(u.unitName.ifBlank { u.unitId }, Modifier.weight(1f), fontWeight = FontWeight.SemiBold)
                                Text("${"%.0f".format(u.pct)}%", fontWeight = FontWeight.Bold,
                                    color = if (eligible) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error)
                            }
                            LinearProgressIndicator(progress = { (u.pct / 100.0).toFloat().coerceIn(0f, 1f) },
                                modifier = Modifier.fillMaxWidth().padding(vertical = 6.dp))
                            Text("${u.attended}/${u.held} attended · pass mark ${u.threshold}%",
                                style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            if (!eligible) Text("Exam-ineligible" + (u.deficit?.let { " — attend $it more to recover" } ?: ""),
                                style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.error)
                        }
                    }
                }
            }
        }
        Spacer(Modifier.weight(1f))
        OutlinedButton(onClick = { load() }, enabled = !loading, modifier = Modifier.fillMaxWidth()) { Text("Refresh") }
    }
}

/** Home — greets the student, then their cohort's weekly timetable and the units they take.
 *  Deliberately just those two things, as the coordinator's Home is. */
@Composable
private fun StudentHome(reloadKey: Int) {
    var home by remember { mutableStateOf<StudentHomeClient.Home?>(null) }
    var loading by remember { mutableStateOf(true) }
    // The student's own attendance, summarised here rather than only on the Progress tab. The
    // number that decides whether they sit the exam is the reason they open the app; making them
    // find a second tab for it is what made this screen feel like a noticeboard.
    var progress by remember { mutableStateOf<ProgressClient.Progress?>(null) }
    LaunchedEffect(reloadKey) {
        loading = true
        home = runCatching { StudentHomeClient().fetch() }.getOrNull()
        home?.let { AppState.cohortLabel = it.cohort }
        loading = false
    }
    LaunchedEffect(reloadKey, AppState.studentId) {
        AppState.studentId?.takeIf { it.isNotBlank() }?.let { reg ->
            progress = runCatching { ProgressClient().fetch(reg) }.getOrNull()
        }
    }

    val name = home?.fullName?.takeIf { it.isNotBlank() }
        ?: AppState.coordinatorName?.takeIf { it.isNotBlank() } ?: "there"

    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(20.dp)) {
        Text("Welcome, $name 👋", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.ExtraBold)
        (home?.cohort ?: AppState.cohortLabel)?.takeIf { it.isNotBlank() }?.let {
            Text(it, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        Spacer(Modifier.height(18.dp))

        if (loading && home == null) {
            Box(Modifier.fillMaxWidth().padding(top = 40.dp), Alignment.Center) { CircularProgressIndicator() }
            return@Column
        }

        // ── Today, and what's next ───────────────────────────────────────────
        // The whole week was here and today was not called out, so a student had to find the right
        // weekday heading to answer the only question they open the app with.
        val allSlots = home?.timetable.orEmpty()
        TodayStrip(allSlots)

        // ── Where their attendance stands ────────────────────────────────────
        progress?.let { EligibilityStrip(it) }

        Text("Timetable", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(6.dp))
        val slots = allSlots
        if (slots.isEmpty()) {
            Text("No timetable published for your cohort yet.",
                style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        } else {
            // The SAME Time × Day grid the coordinator sees, so a student comparing their phone
            // with the one at the front of the room is looking at one timetable, not two layouts
            // of it. This used to be a list of weekday headings, which made the week read as a
            // different thing depending on the role.
            TimetableGrid(
                entries = slots.map { sl ->
                    TtEntry(
                        name = sl.unitName.ifBlank { sl.unitId },
                        dayOfWeek = sl.dayOfWeek,
                        start = sl.startTime,
                        durationMin = sl.durationMinutes,
                        detail = listOfNotNull(
                            sl.room.takeIf { it.isNotBlank() },
                            sl.lecturerName.takeIf { it.isNotBlank() },
                        ).joinToString(" · "),
                    )
                },
                sessionType = home?.sessionType.orEmpty(),
            )
        }

        Spacer(Modifier.height(20.dp))
        val units = home?.units.orEmpty()
        val current = units.filter { it.current }
        val rest = units.filterNot { it.current }

        Text("Units", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(6.dp))
        when {
            units.isEmpty() -> Text("No units registered on your programme yet.",
                style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            current.isEmpty() -> Text("Nothing is tagged for Year ${home?.year} Semester ${home?.semester} yet — your programme's full unit list is below.",
                style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            else -> current.forEach { UnitRow(it) }
        }

        // The rest of the roadmap, so a student can always see where the semester sits in the
        // whole programme — and so an untagged year/semester never renders as "you have no units".
        if (rest.isNotEmpty()) {
            Spacer(Modifier.height(14.dp))
            Text("Other units on your programme", style = MaterialTheme.typography.labelLarge,
                fontWeight = FontWeight.Bold, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.height(6.dp))
            rest.forEach { UnitRow(it, dimmed = true) }
        }
        Spacer(Modifier.height(24.dp))
    }
}

/**
 * Today's classes, with the next one still to come picked out.
 *
 * Built from the SAME weekly timetable already on this screen — no extra call — by filtering to
 * today's weekday and comparing start times against the clock. A day with nothing on it says so,
 * because "no classes today" is a genuinely useful answer and a blank space is not.
 */
@Composable
private fun TodayStrip(slots: List<StudentHomeClient.Slot>) {
    val today = java.time.LocalDate.now().dayOfWeek.value        // Mon=1 … Sun=7, as the API uses
    val now = java.time.LocalTime.now()
    val mine = slots.filter { it.dayOfWeek == today }.sortedBy { it.startTime }
    // The first class that has not started yet. Times are "HH:MM"; an unparseable one is treated
    // as still to come rather than silently dropped.
    val next = mine.firstOrNull { sl ->
        runCatching { java.time.LocalTime.parse(sl.startTime) }.getOrNull()?.isAfter(now) ?: true
    }

    Surface(
        color = MaterialTheme.colorScheme.primary.copy(alpha = .09f),
        shape = MaterialTheme.shapes.medium,
        modifier = Modifier.fillMaxWidth().padding(bottom = 14.dp),
    ) {
        Column(Modifier.padding(14.dp)) {
            Text(
                "Today · ${java.time.LocalDate.now().format(java.time.format.DateTimeFormatter.ofPattern("EEEE d MMM"))}",
                style = MaterialTheme.typography.labelLarge, fontWeight = FontWeight.Bold,
                color = MaterialTheme.colorScheme.primary,
            )
            Spacer(Modifier.height(6.dp))
            when {
                mine.isEmpty() -> Text(
                    "No classes timetabled today.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                else -> {
                    mine.forEach { sl ->
                        val isNext = sl === next
                        Row(
                            Modifier.fillMaxWidth().padding(vertical = 3.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Text(
                                sl.startTime,
                                style = MaterialTheme.typography.labelMedium,
                                fontWeight = if (isNext) FontWeight.ExtraBold else FontWeight.Normal,
                                color = if (isNext) MaterialTheme.colorScheme.primary
                                else MaterialTheme.colorScheme.onSurfaceVariant,
                                modifier = Modifier.width(52.dp),
                            )
                            Column(Modifier.weight(1f)) {
                                Text(
                                    sl.unitName.ifBlank { sl.unitId },
                                    fontWeight = if (isNext) FontWeight.Bold else FontWeight.Normal,
                                    style = MaterialTheme.typography.bodyMedium,
                                )
                                sl.room.takeIf { it.isNotBlank() }?.let {
                                    Text(it, style = MaterialTheme.typography.labelSmall,
                                        color = MaterialTheme.colorScheme.onSurfaceVariant)
                                }
                            }
                            if (isNext) {
                                Text("NEXT", fontSize = 9.sp, fontWeight = FontWeight.ExtraBold,
                                    color = MaterialTheme.colorScheme.primary)
                            }
                        }
                    }
                }
            }
        }
    }
}

/**
 * Attendance against the exam-eligibility bar, in one line per state.
 *
 * A student cares about exactly one thing here: am I going to be allowed to sit the exam. So the
 * units below the threshold lead, with the number of sessions each still needs — the actionable
 * figure — rather than a percentage they have to interpret.
 */
@Composable
private fun EligibilityStrip(p: ProgressClient.Progress) {
    if (p.units.isEmpty()) return
    val below = p.units.filter { it.status.equals("INELIGIBLE", ignoreCase = true) }
    val overall = p.units.sumOf { it.held }.takeIf { it > 0 }
        ?.let { held -> p.units.sumOf { it.attended } * 100.0 / held }
    val ok = below.isEmpty()

    Surface(
        color = (if (ok) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error).copy(alpha = .09f),
        shape = MaterialTheme.shapes.medium,
        modifier = Modifier.fillMaxWidth().padding(bottom = 16.dp),
    ) {
        Column(Modifier.padding(14.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text("Exam eligibility", style = MaterialTheme.typography.labelLarge, fontWeight = FontWeight.Bold)
                    Text(
                        if (ok) "All ${p.units.size} of your units are above the attendance bar."
                        else "${below.size} of your ${p.units.size} units ${if (below.size == 1) "is" else "are"} below the bar.",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
                overall?.let {
                    Text(
                        "${it.toInt()}%", fontWeight = FontWeight.ExtraBold,
                        style = MaterialTheme.typography.titleMedium,
                        color = if (ok) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error,
                    )
                }
            }
            // Name the units at risk and what it takes to fix each. Capped so a student failing
            // everything gets a readable list rather than the whole roadmap again.
            below.take(4).forEach { u ->
                Spacer(Modifier.height(6.dp))
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(u.unitName.ifBlank { u.unitId }, Modifier.weight(1f),
                        style = MaterialTheme.typography.bodySmall)
                    Text(
                        u.deficit?.takeIf { it > 0 }?.let { "attend $it more" } ?: "${u.pct.toInt()}%",
                        style = MaterialTheme.typography.labelSmall, fontWeight = FontWeight.Bold,
                        color = MaterialTheme.colorScheme.error,
                    )
                }
            }
            if (below.size > 4) {
                Text("…and ${below.size - 4} more — see My attendance.",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(top = 6.dp))
            }
        }
    }
}

/** One unit on the student's roadmap. `dimmed` marks a unit outside the current year/semester. */
@Composable
private fun UnitRow(u: StudentHomeClient.Unit, dimmed: Boolean = false) {
    val alpha = if (dimmed) 0.55f else 1f
    Surface(
        color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = if (dimmed) 0.45f else 1f),
        shape = MaterialTheme.shapes.medium,
        modifier = Modifier.fillMaxWidth().padding(bottom = 6.dp),
    ) {
        Row(Modifier.padding(12.dp), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text(u.unitName.ifBlank { u.unitId }, fontWeight = FontWeight.SemiBold,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = alpha))
                Text(
                    listOfNotNull(u.unitId.takeIf { it.isNotBlank() },
                        u.lecturerName.takeIf { it.isNotBlank() }).joinToString(" · "),
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = alpha),
                )
            }
            if (u.year > 0) Text("Y${u.year}/S${u.semester}",
                style = MaterialTheme.typography.labelSmall, fontWeight = FontWeight.Bold,
                color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = alpha))
        }
    }
}

/** Profile — a whole page, so it shows the student's whole record rather than three lines. */
@Composable
private fun StudentProfile(reloadKey: Int, onOpenPortal: () -> Unit) {
    var showChangePw by remember { mutableStateOf(false) }
    if (showChangePw) ChangePasswordDialog(onClose = { showChangePw = false })

    var home by remember { mutableStateOf<StudentHomeClient.Home?>(null) }
    LaunchedEffect(reloadKey) { home = runCatching { StudentHomeClient().fetch() }.getOrNull() }

    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(24.dp)) {
        Text("Profile", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(14.dp))

        // Identity card.
        Surface(color = MaterialTheme.colorScheme.surfaceVariant, shape = MaterialTheme.shapes.large,
            modifier = Modifier.fillMaxWidth()) {
            Column(Modifier.padding(16.dp)) {
                Text(home?.fullName?.takeIf { it.isNotBlank() } ?: AppState.coordinatorName.orEmpty(),
                    style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
                AppState.role?.let {
                    Text(it, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
        }
        Spacer(Modifier.height(16.dp))

        ProfileField("Registration number", home?.studentId ?: AppState.studentId.orEmpty())
        ProfileField("Email", home?.email.orEmpty())
        ProfileField("Course", home?.course.orEmpty())
        ProfileField("Level of study", home?.level.orEmpty())
        ProfileField("Study session", home?.sessionType.orEmpty())
        ProfileField("Intake", home?.intake.orEmpty())
        ProfileField("Year of study", home?.year?.takeIf { it > 0 }?.let { "Year $it" }.orEmpty())
        ProfileField("Semester", home?.semester?.takeIf { it > 0 }?.let { "Semester $it" }.orEmpty())
        ProfileField("Academic year", home?.academicYear.orEmpty())
        ProfileField("Cohort", home?.cohort ?: AppState.cohortLabel.orEmpty())
        // This semester's units, not the whole programme roadmap that Home also lists.
        ProfileField("Units this semester", home?.units?.count { it.current }?.takeIf { it > 0 }?.toString().orEmpty())

        Spacer(Modifier.height(22.dp))
        Button(onClick = onOpenPortal, Modifier.fillMaxWidth()) { Text("🎓  Student portal") }
        Spacer(Modifier.height(8.dp))
        OutlinedButton(onClick = { showChangePw = true }, Modifier.fillMaxWidth()) { Text("🔑  Change password") }
        Spacer(Modifier.height(8.dp))
        SignOutButton()
        Spacer(Modifier.height(24.dp))
    }
}

/** One label/value row. Blank values are skipped rather than shown as an empty line. */
@Composable
private fun ProfileField(label: String, value: String) {
    if (value.isBlank()) return
    Column(Modifier.fillMaxWidth().padding(vertical = 6.dp)) {
        Text(label, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(value, style = MaterialTheme.typography.bodyMedium)
    }
    HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = .4f))
}
