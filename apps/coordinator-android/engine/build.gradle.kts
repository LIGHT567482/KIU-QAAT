// Pure Kotlin/JVM in-room engine (validation chain, session manager, room code).
// Verified off-device against real vectors + the live backend.
plugins { kotlin("jvm") }
kotlin { jvmToolchain(17) }
dependencies {
    implementation(project(":crypto-core"))
    testImplementation(kotlin("test"))
}
