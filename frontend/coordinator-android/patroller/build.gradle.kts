import java.io.FileInputStream
import java.util.Properties

plugins {
    id("com.android.application")
    kotlin("android")
    id("com.google.devtools.ksp") version "2.0.21-1.0.25"
    id("org.jetbrains.kotlin.plugin.compose")
}

// Reuse the SAME release keystore as the coordinator app (both .gitignored); falls back to debug
// signing if absent so the build never breaks.
val keystorePropsFile = rootProject.file("keystore.properties")
val keystoreProps = Properties().apply {
    if (keystorePropsFile.exists()) FileInputStream(keystorePropsFile).use { load(it) }
}
val hasReleaseKeystore = keystorePropsFile.exists()

android {
    namespace = "ug.qaat.patroller"
    compileSdk = 34
    defaultConfig {
        applicationId = "ug.qaat.patroller"
        minSdk = 26
        targetSdk = 34
        versionCode = 1
        versionName = "1.0.0"
        // Same cloud backend as the coordinator app. The patroller app reuses the coordinator app's
        // proven TLS networking (Net.kt copied verbatim) so it installs & runs on every phone.
        val apiBase = (project.findProperty("qaat.apiBase") as String?) ?: "https://qaat-gateway.onrender.com"
        buildConfigField("String", "API_BASE", "\"$apiBase\"")
        val authWarm = (project.findProperty("qaat.authWarmUrl") as String?) ?: "https://qaat-auth.onrender.com/health"
        buildConfigField("String", "AUTH_WARM_URL", "\"$authWarm\"")
    }
    signingConfigs {
        create("release") {
            if (hasReleaseKeystore) {
                storeFile = rootProject.file(keystoreProps.getProperty("storeFile"))
                storePassword = keystoreProps.getProperty("storePassword")
                keyAlias = keystoreProps.getProperty("keyAlias")
                keyPassword = keystoreProps.getProperty("keyPassword")
            }
        }
    }
    buildTypes {
        getByName("release") {
            isMinifyEnabled = false
            isShrinkResources = false
            signingConfig = if (hasReleaseKeystore) signingConfigs.getByName("release")
                            else signingConfigs.getByName("debug")
        }
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
    val composeBom = platform("androidx.compose:compose-bom:2024.09.03")
    implementation(composeBom)
    implementation("androidx.compose.material3:material3")
    // Vector nav/action icons — the bars draw flat silhouettes, never emoji.
    implementation("androidx.compose.material:material-icons-core")
    implementation("androidx.activity:activity-compose:1.9.2")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.8.6")

    // Networking — Ktor + OkHttp engine (same as the coordinator app, for the copied Net.kt).
    implementation("io.ktor:ktor-client-okhttp:2.3.12")

    // Room (offline patrol manifest cache + queued patrol logs).
    implementation("androidx.room:room-runtime:2.6.1")
    implementation("androidx.room:room-ktx:2.6.1")
    ksp("androidx.room:room-compiler:2.6.1")

    // Encrypted-at-rest session store (mirrors the coordinator app; degrades to plain prefs).
    implementation("androidx.security:security-crypto:1.1.0-alpha06")
}
