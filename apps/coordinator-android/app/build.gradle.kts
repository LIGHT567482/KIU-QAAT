plugins {
    id("com.android.application")
    kotlin("android")
    id("com.google.devtools.ksp") version "2.0.21-1.0.25"
}

android {
    namespace = "ug.qaat.coordinator"
    compileSdk = 34
    defaultConfig {
        applicationId = "ug.qaat.coordinator"
        minSdk = 26          // LocalOnlyHotspot + java.time
        targetSdk = 34
        versionCode = 1
        versionName = "0.1.0-phaseA"
        // The cloud backend the app pulls its daily manifest from + syncs to.
        // Emulator → host = 10.0.2.2. For a real phone, set your laptop/server LAN IP or domain.
        buildConfigField("String", "API_BASE", "\"https://10.0.2.2:8443\"")
    }
    buildFeatures { compose = true; buildConfig = true }
    composeOptions { kotlinCompilerExtensionVersion = "1.5.15" }
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
    implementation("androidx.activity:activity-compose:1.9.2")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.8.6")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.8.6")
    implementation("androidx.navigation:navigation-compose:2.8.2")

    // Embedded HTTP server — Ktor on the hotspot interface (CIO engine).
    implementation("io.ktor:ktor-server-core:2.3.12")
    implementation("io.ktor:ktor-server-cio:2.3.12")
    implementation("io.ktor:ktor-server-content-negotiation:2.3.12")

    // Sync client (pull manifest, upload sealed package).
    implementation("io.ktor:ktor-client-cio:2.3.12")

    // Room + SQLCipher (encrypted at rest).
    implementation("androidx.room:room-runtime:2.6.1")
    implementation("androidx.room:room-ktx:2.6.1")
    ksp("androidx.room:room-compiler:2.6.1")
    implementation("net.zetetic:android-database-sqlcipher:4.5.4")
    implementation("androidx.sqlite:sqlite:2.4.0")

    // QR rendering (Wi-Fi join QR + lecturer gate QR).
    implementation("com.google.zxing:core:3.5.3")
}
