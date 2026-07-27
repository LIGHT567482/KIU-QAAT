plugins {
    id("com.android.application")
    kotlin("android")
    id("com.google.devtools.ksp") version "2.0.21-1.0.25"
    id("org.jetbrains.kotlin.plugin.compose")
}

android {
    namespace = "ug.qaat.coordinator"
    compileSdk = 34
    defaultConfig {
        applicationId = "ug.qaat.coordinator"
        minSdk = 26          // LocalOnlyHotspot + java.time
        targetSdk = 34
        versionCode = 2
        versionName = "1.0.0"
        // DEFAULT backend URL the app starts with. The app works for BOTH your local server
        // AND the cloud from ONE build — it trusts the cloud's real CA cert AND the embedded
        // self-signed cert, and the URL is switchable at runtime (login "server" field /
        // Net.setBaseUrl). This is only the default; override it per build if you like:
        //   ./gradlew assembleRelease -Pqaat.apiBase=https://api.yourdomain.com
        val apiBase = (project.findProperty("qaat.apiBase") as String?) ?: "https://qaat-gateway.onrender.com"
        buildConfigField("String", "API_BASE", "\"$apiBase\"")
        // The auth-service's own health URL — pinged on the login screen to wake the sleeping
        // free-tier service BEFORE the user submits (the gateway proxies login to it). Best-effort;
        // a wrong/unreachable value just means no pre-warm, login still works via the gateway.
        val authWarm = (project.findProperty("qaat.authWarmUrl") as String?) ?: "https://qaat-auth.onrender.com/health"
        buildConfigField("String", "AUTH_WARM_URL", "\"$authWarm\"")
    }
    buildFeatures { compose = true; buildConfig = true }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions { jvmTarget = "17" }
    packaging { resources.excludes += setOf("META-INF/INDEX.LIST", "META-INF/io.netty.versions.properties") }
}

dependencies {
    implementation(project(":engine"))
    implementation(project(":crypto-core"))

    // Compose
    val composeBom = platform("androidx.compose:compose-bom:2024.09.03")
    implementation(composeBom)
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.material:material-icons-core")
    implementation("androidx.activity:activity-compose:1.9.2")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.8.6")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.8.6")
    implementation("androidx.navigation:navigation-compose:2.8.2")

    // Embedded HTTP server — Ktor on the hotspot interface (CIO engine).
    implementation("io.ktor:ktor-server-core:2.3.12")
    implementation("io.ktor:ktor-server-cio:2.3.12")
    implementation("io.ktor:ktor-server-content-negotiation:2.3.12")

    // Sync client (pull manifest, upload sealed package). OkHttp engine so we can pin
    // the app-embedded QAAT cert + accept any host — works on any phone at any LAN IP
    // without the user installing a certificate on the device.
    implementation("io.ktor:ktor-client-okhttp:2.3.12")

    // Room + SQLCipher (encrypted at rest).
    implementation("androidx.room:room-runtime:2.6.1")
    implementation("androidx.room:room-ktx:2.6.1")
    ksp("androidx.room:room-compiler:2.6.1")
    implementation("net.zetetic:android-database-sqlcipher:4.5.4")
    implementation("androidx.sqlite:sqlite:2.4.0")

    // QR rendering (Wi-Fi join QR + lecturer gate QR).
    implementation("com.google.zxing:core:3.5.3")

    // Encrypted-at-rest persistence of the coordinator's session + cached manifest, so
    // the app opens straight to work (no re-login) and runs fully offline.
    implementation("androidx.security:security-crypto:1.1.0-alpha06")

    // JVM unit tests (run headlessly): exercise the in-room check-in HTTP path end-to-end.
    testImplementation(kotlin("test"))
    testImplementation("junit:junit:4.13.2")
}
