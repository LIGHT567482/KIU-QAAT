package ug.qaat.coordinator.ui

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch
import ug.qaat.coordinator.net.PatrolClient

/**
 * The patroller's SECOND page — the one they land on after a successful sign-in, before the round.
 *
 * WHY ONLY THIS ROLE. A patrol tick is an accusation: it records that a named lecturer was or was
 * not teaching, and the QA reports weigh it against the coordinator's own log precisely because it
 * comes from an independent observer. The account password is the weak link in that chain — it is
 * what gets shared "just to help cover the rounds this week" — and once shared, anyone can mark any
 * lecturer absent. No other role can do damage of that shape from a borrowed password, so no other
 * role is asked for more.
 *
 * Together with the handset binding (migration 069) this is two independent facts:
 *
 *   • [DeviceGate] — is this the phone the account is registered on?
 *   • this screen  — is this the person the account belongs to?
 *
 * FIRST SIGN-IN sets the PIN; every sign-in after asks for it. It is verified SERVER-side, never
 * locally: a secret a stolen handset can check for itself is not a second factor, it is a delay.
 * That does mean the round cannot open offline — the trade is deliberate, and the message says so.
 */
@Composable
fun PatrolPinGate(content: @Composable () -> Unit) {
    val scope = rememberCoroutineScope()
    // Survives recomposition, NOT the process: closing the app re-asks. A patroller who puts the
    // phone down and comes back to a fresh process must prove themselves again.
    var unlocked by remember { mutableStateOf(false) }
    var state by remember { mutableStateOf<PatrolClient.PinState?>(null) }
    var loadFailed by remember { mutableStateOf(false) }

    suspend fun refresh() {
        val s = PatrolClient().pinState()
        loadFailed = s == null
        state = s
    }
    LaunchedEffect(Unit) { refresh() }

    when {
        unlocked -> content()

        loadFailed -> PinMessageScreen(
            title = "Can't check your PIN",
            body = "Patrol needs a connection to unlock. Find signal and try again — your saved ticks are safe.",
        ) { scope.launch { loadFailed = false; refresh() } }

        state == null -> Box(Modifier.fillMaxSize(), Alignment.Center) {
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                CircularProgressIndicator()
                Text("Checking your patrol account…", Modifier.padding(top = 12.dp),
                    color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }

        // Never set → the first-sign-in screen. This is the only place a PIN is chosen; an
        // administrator can clear it but can never set one, so nobody else ever knows it.
        !state!!.isSet -> SetPinScreen(onDone = { unlocked = true })

        else -> EnterPinScreen(
            initiallyLocked = state!!.locked,
            attemptsLeft = state!!.attemptsLeft,
            onUnlocked = { unlocked = true },
        )
    }
}

/** First sign-in: choose a PIN, twice. */
@Composable
private fun SetPinScreen(onDone: () -> Unit) {
    val scope = rememberCoroutineScope()
    var pin by remember { mutableStateOf("") }
    var confirm by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var err by remember { mutableStateOf<String?>(null) }

    val mismatch = confirm.isNotEmpty() && pin != confirm
    val ready = pin.length in 4..8 && pin == confirm && !busy

    PinScaffold(
        title = "Set your patrol PIN",
        blurb = "You'll enter this each time you open patrol, on top of your password. " +
            "It keeps your ticks yours — nobody who learns your password can file a patrol " +
            "record as you. Choose 4 to 8 digits you won't write down.",
    ) {
        PinField("New PIN", pin, { pin = it })
        Spacer(Modifier.height(10.dp))
        PinField("Confirm PIN", confirm, { confirm = it }, isError = mismatch)
        if (mismatch) {
            Text("The two PINs don't match.", color = MaterialTheme.colorScheme.error,
                style = MaterialTheme.typography.labelSmall, modifier = Modifier.padding(top = 4.dp))
        }
        err?.let {
            Spacer(Modifier.height(8.dp))
            Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.labelMedium)
        }
        Spacer(Modifier.height(18.dp))
        Button(
            enabled = ready, modifier = Modifier.fillMaxWidth(),
            onClick = {
                busy = true; err = null
                scope.launch {
                    val fail = PatrolClient().setPin(pin)
                    busy = false
                    if (fail == null) onDone() else err = fail
                }
            },
        ) { Text(if (busy) "Saving…" else "Save PIN and start patrol") }

        Spacer(Modifier.height(10.dp))
        Text(
            "Forgotten it later? An administrator can clear it so you can set a new one. " +
                "They can't see or choose it.",
            style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
            textAlign = TextAlign.Center,
        )
        Spacer(Modifier.height(20.dp))
        SignOutButton()
    }
}

/** Every sign-in after the first. */
@Composable
private fun EnterPinScreen(initiallyLocked: Boolean, attemptsLeft: Int, onUnlocked: () -> Unit) {
    val scope = rememberCoroutineScope()
    var pin by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var err by remember { mutableStateOf<String?>(null) }
    var left by remember { mutableStateOf(attemptsLeft) }
    var locked by remember { mutableStateOf(initiallyLocked) }

    if (locked) {
        PinMessageScreen(
            title = "Patrol is locked",
            body = "Too many wrong PINs. Wait a little and try again, or ask an administrator to " +
                "clear your PIN so you can set a new one.",
            retryLabel = "Try again",
            onRetry = { locked = false; err = null },
        )
        return
    }

    PinScaffold(
        title = "Enter your patrol PIN",
        blurb = "One more step before today's round.",
    ) {
        PinField("PIN", pin, { pin = it }, isError = err != null)
        err?.let {
            Spacer(Modifier.height(8.dp))
            Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.labelMedium)
            // Naming the remaining attempts is deliberate: a patroller mistyping in a corridor
            // should know how close they are to being locked out mid-round.
            if (left in 1..2) {
                Text("$left ${if (left == 1) "try" else "tries"} left before patrol locks.",
                    color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.labelSmall)
            }
        }
        Spacer(Modifier.height(18.dp))
        Button(
            enabled = pin.length >= 4 && !busy, modifier = Modifier.fillMaxWidth(),
            onClick = {
                busy = true; err = null
                scope.launch {
                    val res = PatrolClient().verifyPin(pin)
                    busy = false
                    when {
                        res.ok -> onUnlocked()
                        res.locked -> { locked = true; pin = "" }
                        else -> { err = res.message; left = res.attemptsLeft; pin = "" }
                    }
                }
            },
        ) { Text(if (busy) "Checking…" else "Open patrol") }

        Spacer(Modifier.height(20.dp))
        SignOutButton()
    }
}

/**
 * Change the PIN from the Profile tab. The CURRENT one is required — the round is already open at
 * this point, so without it a handset left unlocked for a minute would be enough to take the
 * account by simply replacing its second factor.
 */
@Composable
fun ChangePatrolPinDialog(onClose: () -> Unit) {
    val scope = rememberCoroutineScope()
    var current by remember { mutableStateOf("") }
    var next by remember { mutableStateOf("") }
    var confirm by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var err by remember { mutableStateOf<String?>(null) }
    var done by remember { mutableStateOf(false) }

    val mismatch = confirm.isNotEmpty() && next != confirm
    val ready = current.length >= 4 && next.length in 4..8 && next == confirm && !busy

    AlertDialog(
        onDismissRequest = onClose,
        title = { Text(if (done) "PIN changed" else "Change patrol PIN") },
        text = {
            if (done) {
                Text("Your new PIN applies the next time you open patrol.")
            } else {
                Column {
                    PinField("Current PIN", current, { current = it })
                    Spacer(Modifier.height(8.dp))
                    PinField("New PIN", next, { next = it })
                    Spacer(Modifier.height(8.dp))
                    PinField("Confirm new PIN", confirm, { confirm = it }, isError = mismatch)
                    if (mismatch) {
                        Text("The two PINs don't match.", color = MaterialTheme.colorScheme.error,
                            style = MaterialTheme.typography.labelSmall)
                    }
                    err?.let {
                        Spacer(Modifier.height(8.dp))
                        Text(it, color = MaterialTheme.colorScheme.error,
                            style = MaterialTheme.typography.labelMedium)
                    }
                }
            }
        },
        confirmButton = {
            if (done) {
                TextButton(onClick = onClose) { Text("Done") }
            } else {
                TextButton(
                    enabled = ready,
                    onClick = {
                        busy = true; err = null
                        scope.launch {
                            val fail = PatrolClient().setPin(next, current)
                            busy = false
                            if (fail == null) done = true else err = fail
                        }
                    },
                ) { Text(if (busy) "Saving…" else "Save") }
            }
        },
        dismissButton = { if (!done) TextButton(onClick = onClose) { Text("Cancel") } },
    )
}

// ── Shared chrome ────────────────────────────────────────────────────────────

@Composable
private fun PinScaffold(title: String, blurb: String, body: @Composable ColumnScope.() -> Unit) {
    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(
            Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(28.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center,
        ) {
            BrandLogo(AppState.branding, size = 56)
            Spacer(Modifier.height(16.dp))
            Text("🛡", fontSize = 30.sp)
            Spacer(Modifier.height(8.dp))
            Text(title, style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold,
                textAlign = TextAlign.Center)
            Spacer(Modifier.height(8.dp))
            Text(blurb, textAlign = TextAlign.Center, style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.height(22.dp))
            body()
        }
    }
}

@Composable
private fun PinField(label: String, value: String, onChange: (String) -> Unit, isError: Boolean = false) {
    OutlinedTextField(
        value = value,
        // Filtered at the source: the field cannot hold anything the server would reject, so a
        // patroller never gets "digits only" back from a round-trip.
        onValueChange = { onChange(it.filter(Char::isDigit).take(8)) },
        label = { Text(label) },
        singleLine = true,
        isError = isError,
        visualTransformation = PasswordVisualTransformation(),
        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.NumberPassword),
        modifier = Modifier.fillMaxWidth(),
    )
}

@Composable
private fun PinMessageScreen(
    title: String, body: String, retryLabel: String = "Retry", onRetry: () -> Unit,
) {
    PinScaffold(title = title, blurb = body) {
        Button(onClick = onRetry, modifier = Modifier.fillMaxWidth()) { Text(retryLabel) }
        Spacer(Modifier.height(20.dp))
        SignOutButton()
    }
}
