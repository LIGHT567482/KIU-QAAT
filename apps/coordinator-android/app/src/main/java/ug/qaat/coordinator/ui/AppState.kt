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
    val loggedIn: Boolean get() = token != null

    // Daily manifest (config inherited from the cloud while online).
    var manifest by mutableStateOf<ug.qaat.coordinator.net.ManifestClient.Parsed?>(null)

    // Tenant branding (logo + colours), applied app-wide after login.
    var branding by mutableStateOf<ug.qaat.coordinator.net.BrandingClient.Branding?>(null)

    // Active session.
    var currentSessionId by mutableStateOf<String?>(null)
    var currentUnitId by mutableStateOf<String?>(null)
    var roomCode by mutableStateOf("------")
    var secondsLeft by mutableStateOf(10)
    var hotspotSsid by mutableStateOf<String?>(null)
}
