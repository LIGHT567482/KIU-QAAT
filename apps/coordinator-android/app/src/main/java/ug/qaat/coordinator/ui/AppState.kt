package ug.qaat.coordinator.ui

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue

/**
 * Small observable app state shared between the foreground service and the screens.
 * The session-open flow sets currentSessionId/currentUnitId; the service updates roomCode.
 */
object AppState {
    // Auth (set after login; binding key feeds the Sealer — never logged).
    var token by mutableStateOf<String?>(null)
    var userId by mutableStateOf<String?>(null)
    var tenantId by mutableStateOf<String?>(null)
    var deviceBindingKey by mutableStateOf<String?>(null)
    // Light/dark preference (persisted), mirrors the PWA theme toggle.
    var darkTheme by mutableStateOf(false)

    // Coordinator identity for the top bar + profile popup + welcome message.
    var coordinatorName by mutableStateOf<String?>(null)
    var coordinatorTitle by mutableStateOf<String?>(null)
    var coordinatorRegNo by mutableStateOf<String?>(null)
    var coordinatorEmail by mutableStateOf<String?>(null)
    var role by mutableStateOf<String?>(null)
    // Cohort summary shown in the profile popup (moved out of the dashboard body).
    var cohortLabel by mutableStateOf<String?>(null)
    // Global refresh state driven by the top-nav ⟳ icon.
    var refreshing by mutableStateOf(false)
    var refreshTick by mutableStateOf(0)
    val loggedIn: Boolean get() = token != null

    /** Best display name: the credential's full name, else the email's local part. */
    val displayName: String get() = coordinatorName?.takeIf { it.isNotBlank() }
        ?: coordinatorEmail?.substringBefore("@")?.takeIf { it.isNotBlank() } ?: "Coordinator"

    // Daily manifest (config inherited from the cloud while online).
    var manifest by mutableStateOf<ug.qaat.coordinator.net.ManifestClient.Parsed?>(null)

    // Tenant branding (logo + colours), applied app-wide after login.
    var branding by mutableStateOf<ug.qaat.coordinator.net.BrandingClient.Branding?>(null)

    // Active session.
    var currentSessionId by mutableStateOf<String?>(null)
    var currentUnitId by mutableStateOf<String?>(null)
    var roomCode by mutableStateOf("------")      // STATIC student room code (does not rotate)
    var lecturerCode by mutableStateOf("------")  // ROTATING lecturer code (changes every 10s)
    var secondsLeft by mutableStateOf(10)
    var hotspotSsid by mutableStateOf<String?>(null)
    var hotspotPass by mutableStateOf<String?>(null)
    // Base URL of the in-room server on the hotspot (LocalOnlyHotspot gateway is 192.168.49.1).
    val inRoomBaseUrl: String get() = "http://192.168.49.1:8080"
}
