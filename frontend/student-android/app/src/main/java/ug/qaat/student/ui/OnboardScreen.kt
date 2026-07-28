package ug.qaat.student.ui

import android.Manifest
import android.content.pm.PackageManager
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import kotlinx.coroutines.launch
import ug.qaat.student.net.OnboardClient
import ug.qaat.student.store.StudentStore

/**
 * One-time onboarding: scan the student's QR card → qr-login + my-qr → store the signed credential.
 * The only screen that needs the internet. After this the app opens straight to the one-tap ATTEND.
 */
@Composable
fun OnboardScreen(onDone: () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var hasCamera by remember {
        mutableStateOf(ContextCompat.checkSelfPermission(ctx, Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED)
    }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    val permLauncher = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { hasCamera = it }
    LaunchedEffect(Unit) { if (!hasCamera) permLauncher.launch(Manifest.permission.CAMERA) }

    Column(Modifier.fillMaxSize().padding(24.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        Spacer(Modifier.height(24.dp))
        Text("Set up QAAT Attend", style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold)
        Text("Scan your student QR card once. After this you just tap to attend — no internet needed in class.",
            style = MaterialTheme.typography.bodyMedium, textAlign = TextAlign.Center,
            color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(vertical = 12.dp))

        when {
            busy -> {
                Spacer(Modifier.height(40.dp)); CircularProgressIndicator()
                Text("Signing you in…", Modifier.padding(top = 12.dp))
            }
            !hasCamera -> {
                Text("Camera permission is needed to scan your QR card.", color = MaterialTheme.colorScheme.error, textAlign = TextAlign.Center)
                Button(onClick = { permLauncher.launch(Manifest.permission.CAMERA) }, Modifier.padding(top = 12.dp)) { Text("Allow camera") }
            }
            else -> {
                Surface(Modifier.fillMaxWidth().weight(1f).padding(top = 8.dp), shape = MaterialTheme.shapes.large) {
                    QrScanner(onResult = { scanned ->
                        if (busy) return@QrScanner
                        busy = true; error = null
                        scope.launch {
                            runCatching { OnboardClient().onboard(scanned) }
                                .onSuccess { r -> StudentStore.save(r.credential, r.studentId, r.studentId); onDone() }
                                .onFailure { error = it.message ?: "Onboarding failed"; busy = false }
                        }
                    }, modifier = Modifier.fillMaxSize())
                }
            }
        }
        error?.let {
            Text(it, color = MaterialTheme.colorScheme.error, textAlign = TextAlign.Center, modifier = Modifier.padding(top = 12.dp))
        }
    }
}
