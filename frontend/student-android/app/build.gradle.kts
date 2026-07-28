plugins {
    id("com.android.application")
    kotlin("android")
    id("org.jetbrains.kotlin.plugin.compose")
}

android {
    namespace = "ug.qaat.student"
    compileSdk = 34
    defaultConfig {
        applicationId = "ug.qaat.student"
        minSdk = 26          // NSD + java.time; matches the coordinator app
        targetSdk = 34
        versionCode = 1
        versionName = "1.0.0"
        // Default backend — used ONLY for the one-time online onboarding (qr-login + my-qr).
        // Override per build: ./gradlew assembleRelease -Pqaat.apiBase=https://api.yourdomain.com
        val apiBase = (project.findProperty("qaat.apiBase") as String?) ?: "https://qaat-gateway.onrender.com"
        buildConfigField("String", "API_BASE", "\"$apiBase\"")
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
    // Compose
    val composeBom = platform("androidx.compose:compose-bom:2024.09.03")
    implementation(composeBom)
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.material:material-icons-core")
    implementation("androidx.activity:activity-compose:1.9.2")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.8.6")

    // HTTP: OkHttp engine trusts the app-embedded cloud cert path the same way the coordinator app
    // does (onboarding hits the cloud); the in-room calls are plain http over the hotspot LAN.
    implementation("io.ktor:ktor-client-core:2.3.12")
    implementation("io.ktor:ktor-client-okhttp:2.3.12")

    // CameraX + ML Kit barcode scanning for the one-time "scan your QR card" onboarding.
    implementation("androidx.camera:camera-camera2:1.3.4")
    implementation("androidx.camera:camera-lifecycle:1.3.4")
    implementation("androidx.camera:camera-view:1.3.4")
    implementation("com.google.mlkit:barcode-scanning:17.3.0")

    // Encrypted-at-rest storage of the student's credential (so they never log in again).
    implementation("androidx.security:security-crypto:1.1.0-alpha06")

    testImplementation(kotlin("test"))
    testImplementation("junit:junit:4.13.2")
}
