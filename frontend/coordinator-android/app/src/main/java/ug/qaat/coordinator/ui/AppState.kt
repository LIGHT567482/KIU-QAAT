package ug.qaat.coordinator.ui

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue

/**
 * Small observable app state shared between the foreground service and the screens.
 * The session-open flow sets currentSessionId/currentUnitId; the service updates the lecturer code.
 */
object AppState {
    // The app's LocalOnlyHotspot gateway — fixed and known, the same on every coordinator phone,
    // so the projected check-in link always resolves to the coordinator's server with no detection.
    const val LOCAL_HOTSPOT_IP = "192.168.49.1"
    // The conventional gateway of a phone's OWN portable hotspot (Android's stock tether subnet).
    // Used as the serving IP in system-hotspot mode — a coordinator's own hotspot is NOT on the
    // LocalOnlyHotspot 192.168.49.x subnet, so serving on .49.1 there is unreachable ("can't be
    // reached"). detectApIp() refines it; a manual override still wins.
    const val SYSTEM_HOTSPOT_IP = "192.168.43.1"
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

    // Stack trace of the previous run's uncaught crash (if any), surfaced once on next
    // launch so a "silent close" can be read/screenshotted instead of vanishing.
    var lastCrash by mutableStateOf<String?>(null)

    /** Best display name: the credential's full name, else the email's local part. */
    val displayName: String get() = coordinatorName?.takeIf { it.isNotBlank() }
        ?: coordinatorEmail?.substringBefore("@")?.takeIf { it.isNotBlank() } ?: "Coordinator"

    // One-shot notice about the session lifecycle (auto-close fired, or "end the current one
    // first" when a second open is attempted). Shown on the attendance screens, then cleared.
    var sessionNotice by mutableStateOf<String?>(null)

    // Daily manifest (config inherited from the cloud while online).
    var manifest by mutableStateOf<ug.qaat.coordinator.net.ManifestClient.Parsed?>(null)
    // Why the last manifest fetch failed (HTTP code / network reason), so a failed
    // refresh explains itself instead of silently showing "hasn't loaded yet".
    var manifestError by mutableStateOf<String?>(null)

    // Tenant branding (logo + colours), applied app-wide after login.
    var branding by mutableStateOf<ug.qaat.coordinator.net.BrandingClient.Branding?>(null)

    // Active session.
    var currentSessionId by mutableStateOf<String?>(null)
    var currentUnitId by mutableStateOf<String?>(null)
    var lecturerCode by mutableStateOf("------")  // ROTATING lecturer code (changes every 10s)
    var secondsLeft by mutableStateOf(10)
    var hotspotSsid by mutableStateOf<String?>(null)   // the ACTIVE name students must join
    var hotspotPass by mutableStateOf<String?>(null)   // the ACTIVE password
    // True once the foreground service has built the in-room server AND it is actually listening
    // on :8080. The "Start taking attendance" button waits on this.
    var serverReady by mutableStateOf(false)
    // Human-readable reason the hub failed to come up (null when fine). Surfaced in the UI so a
    // startup failure is diagnosable on-device instead of vanishing into a notification.
    var serverError by mutableStateOf<String?>(null)
    // Hotspot mode. false = the app starts its OWN LocalOnlyHotspot at a FIXED, KNOWN gateway
    // (192.168.49.1) that the app controls — no per-phone IP guessing, same for every coordinator.
    // This is the shipping model (the system hotspot's IP is OEM-dependent and hidden from the app
    // on MIUI/HyperOS). Students join by scanning the projected "Connect to Wi-Fi" QR. true = the
    // coordinator's own system hotspot (kept as a fallback only).
    var useSystemHotspot by mutableStateOf(false)
    // Reachability self-test: how many client requests have reached the in-room server. The instant
    // a student's phone loads the check-in page this goes > 0 — objective proof that client→host
    // works on this hardware (vs. staying 0 if the phone blocks it).
    var clientsReached by mutableStateOf(0)
    // True once the app has found the phone's IP on the active hotspot (system-hotspot mode).
    var hotspotUp by mutableStateOf(false)
    // Diagnostic: the phone's private IPv4 addresses (iface=ip) when auto-detect is unsure, so the
    // coordinator can read the real hotspot gateway off-screen instead of guessing.
    var hotspotDiag by mutableStateOf<String?>(null)
    // Manual override of the hotspot gateway IP (persisted). On phones that hide their hotspot
    // interface from apps (e.g. MIUI/HyperOS), auto-detect can't see the gateway — so the
    // coordinator reads it off ANY joined student phone (its Wi-Fi "Gateway") and sets it here.
    // When set, it wins over auto-detect and is the IP student cards must be issued for.
    var manualHotspotIp by mutableStateOf<String?>(null)
    // The in-room server's IP on the hotspot interface. Defaults to the fixed LocalOnlyHotspot
    // gateway (192.168.49.1); detectApIp() confirms/overrides it once the hotspot is up.
    var inRoomIp by mutableStateOf(LOCAL_HOTSPOT_IP)
    val inRoomBaseUrl: String get() = "http://$inRoomIp:8080"

    // ── System-hotspot mode: the cohort-named Wi-Fi the coordinator sets up on their OWN phone.
    // Android forbids an app from setting the system hotspot's name/password, so the coordinator
    // types them into Android settings; we capture the SAME values here so the projected
    // "Connect to Wi-Fi" QR encodes them and students can scan-join. Persisted across sessions.
    var systemHotspotSsid by mutableStateOf<String?>(null)
    var systemHotspotPass by mutableStateOf<String?>(null)

    /** A Wi-Fi-friendly SSID suggested from the cohort label (spaces/·/symbols stripped, ≤28 chars),
     *  so every coordinator's hotspot is identifiable per-cohort in a shared room. */
    fun suggestedSsid(): String {
        val base = (cohortLabel ?: "").ifBlank { "QAAT-Room" }
        val clean = base.replace("·", "-").replace(Regex("\\s+"), "").replace(Regex("[^A-Za-z0-9-]"), "")
        return clean.trim('-').take(28).ifBlank { "QAAT-Room" }
    }
}
