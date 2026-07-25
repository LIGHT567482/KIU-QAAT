// Root build file. Versions are pinned in each module. Open this folder in Android
// Studio (Giraffe+); it provides the Android SDK + a Gradle wrapper. The :engine and
// :crypto-core modules are plain Kotlin/JVM (already verified off-device) and are
// consumed by the :app Android module.
plugins {
    id("com.android.application") version "8.5.2" apply false
    kotlin("android") version "2.0.21" apply false
    kotlin("jvm") version "2.0.21" apply false
    id("org.jetbrains.kotlin.plugin.compose") version "2.0.0" apply false
}
