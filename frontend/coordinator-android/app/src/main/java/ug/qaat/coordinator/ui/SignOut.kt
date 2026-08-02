package ug.qaat.coordinator.ui

import android.content.Intent
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.ui.Modifier
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import ug.qaat.coordinator.di.Graph
import ug.qaat.coordinator.notify.AlertNotifier
import ug.qaat.coordinator.service.SessionService
import ug.qaat.coordinator.store.SessionStore

/**
 * Signing out, done once for every role.
 *
 * Sign-out is not "forget the token". This handset runs an in-room hotspot and an HTTP server,
 * caches a cohort's roster and its check-ins, and holds the device-binding key that is the ONLY
 * thing able to seal a closed session for upload. Dropping the token while any of that is live
 * used to leave the hub serving check-ins to a signed-out phone, and made pending attendance
 * permanently unuploadable — the key needed to seal it was gone.
 *
 * So sign-out has three tiers, in this order:
 *
 *  1. REFUSED while a session is open. The coordinator must End the session, which seals and
 *     uploads it. There is no "sign out and lose the room" path, because the students in it have
 *     already checked in.
 *  2. CONFIRMED when sealed sessions have not reached the server. They cannot survive sign-out —
 *     the binding key goes with it — so the count is named and the user chooses.
 *  3. Otherwise immediate.
 *
 * Then the teardown is total: service stopped, notifications cleared, credentials wiped
 * synchronously, cached cohort data dropped, and every scrap of in-memory state reset.
 */

/** Why sign-out cannot proceed at all right now, or null if it can. */
fun signOutBlocker(): String? =
    if (AppState.currentSessionId != null)
        "A session is still open. End it first — that seals the attendance and uploads it. " +
            "Signing out now would leave the room's check-ins stranded on this phone."
    else null

/**
 * The full teardown. Assumes the caller has already cleared the checks above; [SignOutButton] is
 * the supported way in.
 */
fun performSignOut() {
    // 1. The hub: stop the foreground service, which in onDestroy stops the Ktor server, unregisters
    //    the LAN advert and closes the app-owned hotspot. A signed-out phone must not still be
    //    answering check-ins on :8080.
    runCatching { Graph.appContext.stopService(Intent(Graph.appContext, SessionService::class.java)) }

    // 2. This account's pop-up notifications and the record of which alerts it had already seen.
    AlertNotifier.reset()

    // 3. Credentials, synchronously. clear().apply() returns before the write lands, so a process
    //    death in the next instant could resurrect the session on next launch.
    SessionStore.clearNow()

    // 4. Cached cohort data — roster, check-ins, session history, patrol round.
    runCatching { Graph.db.dao().clearAllForSignOut() }

    // 5. In-memory state. Identity, then the whole hub, so no field of the previous session can
    //    bleed into the next sign-in.
    AppState.token = null; AppState.userId = null; AppState.tenantId = null
    AppState.deviceBindingKey = null
    AppState.coordinatorName = null; AppState.coordinatorTitle = null
    AppState.coordinatorRegNo = null; AppState.coordinatorEmail = null; AppState.role = null
    AppState.studentId = null; AppState.staffId = null; AppState.org = null
    AppState.attendBlockUntil = 0L
    // Left set, this would drop the NEXT person straight onto the mandatory change-password
    // screen — or, worse, skip it for someone who does still owe a change.
    AppState.forcePasswordChange = false
    AppState.manifest = null; AppState.manifestError = null; AppState.cohortLabel = null
    AppState.sessionNotice = null
    AppState.currentSessionId = null; AppState.currentUnitId = null
    AppState.currentLecturerHasCode = false; AppState.lecturerStartedHere = false
    AppState.currentSessionCode = null
    AppState.hotspotSsid = null; AppState.hotspotPass = null; AppState.hotspotUp = false
    AppState.hotspotDiag = null
    AppState.serverReady = false; AppState.serverError = null; AppState.clientsReached = 0
    AppState.refreshing = false
    // Branding is the INSTITUTION's, not the user's, and there is only one institution — keeping it
    // means the login screen stays branded instead of flashing back to stock Material.
}

/**
 * The sign-out control every role screen uses. Owning the dialogs here is the point: the four
 * screens each had their own bare button calling straight through to the teardown, so whether you
 * were warned about unsynced attendance depended on which role you happened to be.
 */
@Composable
fun SignOutButton(modifier: Modifier = Modifier.fillMaxWidth(), label: String = "Sign out") {
    var blocked by remember { mutableStateOf<String?>(null) }
    var pending by remember { mutableStateOf(0) }
    var confirming by remember { mutableStateOf(false) }
    var checking by remember { mutableStateOf(false) }

    // Counting pending uploads touches Room, so it happens off the main thread once the button is
    // pressed — not on every recomposition of the profile screen.
    LaunchedEffect(checking) {
        if (!checking) return@LaunchedEffect
        val n = withContext(Dispatchers.IO) { runCatching { Graph.db.dao().pendingSyncCount() }.getOrDefault(0) }
        checking = false
        if (n > 0) { pending = n; confirming = true } else performSignOut()
    }

    OutlinedButton(
        modifier = modifier,
        onClick = {
            val why = signOutBlocker()
            if (why != null) blocked = why else checking = true
        },
    ) { Text(label, color = MaterialTheme.colorScheme.error) }

    blocked?.let { why ->
        AlertDialog(
            onDismissRequest = { blocked = null },
            title = { Text("End the session first") },
            text = { Text(why) },
            confirmButton = { TextButton(onClick = { blocked = null }) { Text("OK") } },
        )
    }

    if (confirming) {
        AlertDialog(
            onDismissRequest = { confirming = false },
            title = { Text("Attendance not uploaded") },
            text = {
                Text(
                    "$pending closed ${if (pending == 1) "session has" else "sessions have"} not reached " +
                        "the server yet. Signing out discards ${if (pending == 1) "it" else "them"} — the key " +
                        "that seals them belongs to this sign-in.\n\n" +
                        "Get online and let it sync first if you can."
                )
            },
            confirmButton = {
                TextButton(onClick = { confirming = false; performSignOut() }) {
                    Text("Sign out anyway", color = MaterialTheme.colorScheme.error)
                }
            },
            dismissButton = { TextButton(onClick = { confirming = false }) { Text("Stay signed in") } },
        )
    }
}
