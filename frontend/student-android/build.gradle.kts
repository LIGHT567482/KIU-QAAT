// Root build file for the QAAT student app. Mirrors the coordinator-android project so both
// build with the same Gradle/AGP/Kotlin toolchain. The student app is deliberately THIN: it
// onboards once online (holds the student's signed QR credential), then works fully offline —
// it talks only to the coordinator's in-room server over the hotspot LAN.
plugins {
    id("com.android.application") version "8.5.2" apply false
    kotlin("android") version "2.0.21" apply false
    id("org.jetbrains.kotlin.plugin.compose") version "2.0.0" apply false
}
